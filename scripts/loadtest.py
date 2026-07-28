#!/usr/bin/env python3
"""img.li / imgli 可复用压测脚本（stdlib only）。

用法示例:
  # 读压（公网或源站）
  python3 scripts/loadtest.py read --base https://img.li --n 100 --c 20
  python3 scripts/loadtest.py read --base http://127.0.0.1:8686 --path /api/v1/config --n 200 --c 40

  # 写压（游客上传；注意生产游客限速，测前需临时提高）
  python3 scripts/loadtest.py write --base http://127.0.0.1:8686 --n 30 --c 5
  python3 scripts/loadtest.py write --base https://img.li --n 10 --c 2 --token "$TOKEN"

  # 综合读场景（home + config + 一张图）
  python3 scripts/loadtest.py suite --base https://img.li --image /i/zCtBfKWVsgaL.png
"""
from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import struct
import sys
import time
import urllib.error
import urllib.request
import zlib
from typing import Dict, List, Optional, Tuple


def _pct(sorted_lat: List[float], p: float) -> float:
    if not sorted_lat:
        return 0.0
    i = min(len(sorted_lat) - 1, int(round((p / 100.0) * (len(sorted_lat) - 1))))
    return sorted_lat[i]


def _report(name: str, url: str, n: int, c: int, wall: float, lat: List[float], codes: Dict[int, int]) -> None:
    lat = sorted(lat)
    errs = sum(v for k, v in codes.items() if k == 0 or k >= 400)
    ok = codes.get(200, 0) + codes.get(201, 0)
    print(f"\n=== {name} ===")
    print(f"url={url}")
    print(f"n={n} c={c} wall={wall:.2f}s rps={n / wall if wall else 0:.1f} ok={ok} err_or_4xx+={errs}")
    if lat:
        print(
            "latency_ms "
            f"p50={_pct(lat, 50) * 1000:.1f} "
            f"p95={_pct(lat, 95) * 1000:.1f} "
            f"p99={_pct(lat, 99) * 1000:.1f} "
            f"max={lat[-1] * 1000:.1f}"
        )
    print(f"codes={dict(sorted(codes.items()))}")


def http_get(url: str, timeout: float, headers: Optional[dict] = None) -> Tuple[float, int, int, Optional[str]]:
    t0 = time.perf_counter()
    h = {"User-Agent": "imgli-loadtest/1.0"}
    if headers:
        h.update(headers)
    try:
        req = urllib.request.Request(url, headers=h)
        with urllib.request.urlopen(req, timeout=timeout) as r:
            body = r.read()
            return time.perf_counter() - t0, int(r.status), len(body), None
    except urllib.error.HTTPError as e:
        try:
            e.read()
        except Exception:
            pass
        return time.perf_counter() - t0, int(e.code), 0, str(e.code)
    except Exception as e:
        return time.perf_counter() - t0, 0, 0, type(e).__name__


def run_get(name: str, url: str, n: int, c: int, timeout: float, headers: Optional[dict] = None) -> None:
    lat: List[float] = []
    codes: Dict[int, int] = {}
    t0 = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, c)) as ex:
        futs = [ex.submit(http_get, url, timeout, headers) for _ in range(n)]
        for f in concurrent.futures.as_completed(futs):
            dt, code, _size, _err = f.result()
            lat.append(dt)
            codes[code] = codes.get(code, 0) + 1
    _report(name, url, n, c, time.perf_counter() - t0, lat, codes)


def _chunk(tag: bytes, data: bytes) -> bytes:
    return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)


def make_png(seed: int, edge: int = 8) -> bytes:
    """生成唯一小 PNG（seed 改像素 → 不同 hash，走完整上传而非秒传）。"""
    raw = bytearray()
    for y in range(edge):
        raw.append(0)  # filter none
        for x in range(edge):
            v = (seed + x * 17 + y * 31) & 0xFF
            raw.extend((v, (v * 3) & 0xFF, (v * 7) & 0xFF, 255))
    ihdr = struct.pack(">IIBBBBB", edge, edge, 8, 6, 0, 0, 0)
    return (
        b"\x89PNG\r\n\x1a\n"
        + _chunk(b"IHDR", ihdr)
        + _chunk(b"IDAT", zlib.compress(bytes(raw), 9))
        + _chunk(b"IEND", b"")
    )


def http_upload(
    url: str,
    png: bytes,
    timeout: float,
    token: Optional[str],
    visibility: str,
) -> Tuple[float, int, Optional[str], Optional[str]]:
    boundary = f"----imgli{time.time_ns()}"
    filename = f"lt-{time.time_ns()}.png"
    parts = [
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
        f"Content-Type: image/png\r\n\r\n".encode(),
        png,
        f"\r\n--{boundary}\r\n"
        f'Content-Disposition: form-data; name="visibility"\r\n\r\n'
        f"{visibility}\r\n"
        f"--{boundary}--\r\n".encode(),
    ]
    body = b"".join(parts)
    headers = {
        "User-Agent": "imgli-loadtest/1.0",
        "Content-Type": f"multipart/form-data; boundary={boundary}",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    t0 = time.perf_counter()
    try:
        req = urllib.request.Request(url, data=body, method="POST", headers=headers)
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read()
            key = None
            try:
                key = json.loads(raw.decode()).get("data", {}).get("key")
            except Exception:
                pass
            return time.perf_counter() - t0, int(r.status), key, None
    except urllib.error.HTTPError as e:
        msg = e.read()[:200].decode("utf-8", "replace")
        return time.perf_counter() - t0, int(e.code), None, msg
    except Exception as e:
        return time.perf_counter() - t0, 0, None, type(e).__name__


def run_write(
    base: str,
    n: int,
    c: int,
    timeout: float,
    token: Optional[str],
    visibility: str,
    edge: int,
) -> List[str]:
    url = base.rstrip("/") + "/api/v1/upload"
    lat: List[float] = []
    codes: Dict[int, int] = {}
    keys: List[str] = []
    errors: List[str] = []
    t0 = time.perf_counter()

    def one(i: int):
        return http_upload(url, make_png(i + int(time.time()) % 100000, edge=edge), timeout, token, visibility)

    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, c)) as ex:
        futs = [ex.submit(one, i) for i in range(n)]
        for f in concurrent.futures.as_completed(futs):
            dt, code, key, err = f.result()
            lat.append(dt)
            codes[code] = codes.get(code, 0) + 1
            if key:
                keys.append(key)
            if err and len(errors) < 10:
                errors.append(f"{code}: {err[:120]}")
    _report("write_upload", url, n, c, time.perf_counter() - t0, lat, codes)
    if errors:
        print("sample_errors:")
        for e in errors:
            print(" ", e)
    if keys:
        print(f"uploaded_keys={len(keys)} first={keys[0]} last={keys[-1]}")
    return keys


def cmd_read(args: argparse.Namespace) -> None:
    path = args.path if args.path.startswith("/") else "/" + args.path
    run_get(args.name or path, args.base.rstrip("/") + path, args.n, args.c, args.timeout)


def cmd_write(args: argparse.Namespace) -> None:
    token = args.token or os.environ.get("IMGLI_TOKEN")
    run_write(args.base, args.n, args.c, args.timeout, token, args.visibility, args.edge)


def cmd_suite(args: argparse.Namespace) -> None:
    base = args.base.rstrip("/")
    run_get("home", base + "/", args.n, args.c, args.timeout)
    run_get("config", base + "/api/v1/config", args.n, min(args.c, 20), args.timeout)
    if args.image:
        img = args.image if args.image.startswith("/") else "/" + args.image
        run_get("image", base + img, args.n, args.c, args.timeout)


def main() -> int:
    p = argparse.ArgumentParser(description="imgli load test (read/write/suite)")
    sub = p.add_subparsers(dest="cmd", required=True)

    pr = sub.add_parser("read", help="GET 压测")
    pr.add_argument("--base", default="https://img.li")
    pr.add_argument("--path", default="/")
    pr.add_argument("--name", default="")
    pr.add_argument("--n", type=int, default=100)
    pr.add_argument("--c", type=int, default=20)
    pr.add_argument("--timeout", type=float, default=30)
    pr.set_defaults(func=cmd_read)

    pw = sub.add_parser("write", help="POST /api/v1/upload 写压")
    pw.add_argument("--base", default="https://img.li")
    pw.add_argument("--n", type=int, default=20)
    pw.add_argument("--c", type=int, default=4)
    pw.add_argument("--timeout", type=float, default=60)
    pw.add_argument("--token", default="", help="Bearer token；空=游客")
    pw.add_argument("--visibility", default="public", choices=["public", "private"])
    pw.add_argument("--edge", type=int, default=8, help="生成 PNG 边长像素")
    pw.set_defaults(func=cmd_write)

    ps = sub.add_parser("suite", help="常用读场景套件")
    ps.add_argument("--base", default="https://img.li")
    ps.add_argument("--image", default="", help="如 /i/xxx.png")
    ps.add_argument("--n", type=int, default=50)
    ps.add_argument("--c", type=int, default=10)
    ps.add_argument("--timeout", type=float, default=30)
    ps.set_defaults(func=cmd_suite)

    args = p.parse_args()
    args.func(args)
    return 0


if __name__ == "__main__":
    sys.exit(main())
