#!/usr/bin/env python3
"""imgli OCR sidecar — PaddleOCR 3.x + paddlepaddle 3.x.

Protocol matches moderation.OCRKeywordsChecker:
  POST /  Authorization: Bearer <token>  body=raw image bytes
  200 {"text":"..."}
  GET /health -> ok-ocr

Notes:
- paddlepaddle 2.6.x segfaults on import on some Zen5/KVM hosts; use 3.3+.
- paddleocr 3.x uses use_textline_orientation (not use_angle_cls) and
  drops show_log / enable_mkldnn kwargs.
"""
from __future__ import annotations

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
import threading

# Zen5/KVM: disable oneDNN/PIR executor paths that raise
# ConvertPirAttribute2RuntimeAttribute NotImplementedError at predict time.
for _k, _v in (
    ("FLAGS_use_mkldnn", "0"),
    ("FLAGS_onednn", "0"),
    ("FLAGS_enable_pir_api", "0"),
    ("FLAGS_enable_pir_in_executor", "0"),
    ("FLAGS_enable_new_ir_in_executor", "0"),
    ("OMP_NUM_THREADS", "2"),
):
    os.environ.setdefault(_k, _v)

TOKEN = os.environ.get("TOKEN", "").strip()
PORT = int(os.environ.get("PORT", "3199"))
HOST = os.environ.get("HOST", "0.0.0.0")
MAX = 20 * 1024 * 1024
LANG = os.environ.get("OCR_LANG", "ch")
USE_ORIENT = os.environ.get("OCR_USE_ANGLE_CLS", "1") not in ("0", "false", "False")

_ocr = None
_lock = threading.Lock()


def get_ocr():
    global _ocr
    with _lock:
        if _ocr is None:
            # Must set before model load; env FLAGS alone may not stick on 3.3.1.
            import paddle

            try:
                paddle.set_flags(
                    {
                        "FLAGS_use_mkldnn": False,
                        "FLAGS_use_onednn": False,
                    }
                )
            except Exception as e:
                print("paddle.set_flags warn", e, flush=True)

            from paddleocr import PaddleOCR

            try:
                # Doc preproc models are heavier and not needed for image-host text overlays.
                _ocr = PaddleOCR(
                    lang=LANG,
                    use_textline_orientation=USE_ORIENT,
                    use_doc_orientation_classify=False,
                    use_doc_unwarping=False,
                )
            except TypeError:
                # older paddleocr fallback
                _ocr = PaddleOCR(
                    use_angle_cls=USE_ORIENT,
                    lang=LANG,
                    use_gpu=False,
                    enable_mkldnn=False,
                )
            print(
                "paddleocr ready lang=%s orient=%s" % (LANG, USE_ORIENT),
                flush=True,
            )
        return _ocr


def _extract_lines(result) -> list[str]:
    """Normalize paddleocr 2.x / 3.x result shapes into text lines."""
    lines: list[str] = []
    if result is None:
        return lines

    # 3.x predict often returns list[dict] or OCRResult-like
    if isinstance(result, dict):
        # common keys: rec_texts / texts
        for key in ("rec_texts", "texts", "text"):
            if key in result:
                val = result[key]
                if isinstance(val, str):
                    if val.strip():
                        lines.append(val)
                elif isinstance(val, (list, tuple)):
                    for t in val:
                        if t:
                            lines.append(str(t))
                return lines

    if not isinstance(result, (list, tuple)):
        # try attribute access (OCRResult)
        for key in ("rec_texts", "texts"):
            val = getattr(result, key, None)
            if val is not None:
                return _extract_lines({key: val})
        return lines

    if not result:
        return lines

    first = result[0]
    # 3.x: list of page dicts
    if isinstance(first, dict):
        for page in result:
            lines.extend(_extract_lines(page))
        return lines

    # 2.x: result[0] = [[box, (text, conf)], ...]
    page = first if isinstance(first, list) else result
    if not page:
        return lines
    for item in page:
        try:
            if item is None:
                continue
            if isinstance(item, dict):
                lines.extend(_extract_lines(item))
                continue
            text = item[1][0] if isinstance(item[1], (list, tuple)) else item[1]
            if text:
                lines.append(str(text))
        except (IndexError, TypeError, KeyError):
            continue
    return lines


def ocr_bytes(data: bytes) -> str:
    import numpy as np
    from PIL import Image

    img = Image.open(io.BytesIO(data))
    if img.mode not in ("RGB", "L"):
        img = img.convert("RGB")
    if img.mode == "L":
        img = img.convert("RGB")
    arr = np.array(img)
    ocr = get_ocr()
    result = None
    # prefer predict (3.x), then ocr (2.x)
    if hasattr(ocr, "predict"):
        try:
            result = ocr.predict(arr)
        except Exception as e:
            print("predict fail", type(e).__name__, e, flush=True)
    if result is None and hasattr(ocr, "ocr"):
        try:
            result = ocr.ocr(arr)
        except TypeError:
            result = ocr.ocr(arr, cls=USE_ORIENT)
        except Exception as e:
            print("ocr fail", type(e).__name__, e, flush=True)
            raise
    lines = _extract_lines(result)
    return "\n".join(lines)


class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[ocr-paddle]", fmt % args, flush=True)

    def _read_body(self) -> bytes:
        te = (self.headers.get("Transfer-Encoding") or "").lower()
        if "chunked" in te:
            parts, total = [], 0
            while True:
                line = self.rfile.readline()
                if not line:
                    break
                size = int(line.split(b";", 1)[0].strip(), 16)
                if size == 0:
                    while True:
                        t = self.rfile.readline()
                        if t in (b"\r\n", b"\n", b""):
                            break
                    break
                if total + size > MAX:
                    raise ValueError("too large")
                chunk = self.rfile.read(size)
                parts.append(chunk)
                total += len(chunk)
                self.rfile.read(2)
            return b"".join(parts)
        cl = self.headers.get("Content-Length")
        if cl is None:
            raise ValueError("need Content-Length or chunked")
        n = int(cl)
        if n < 0 or n > MAX:
            raise ValueError("bad length")
        return self.rfile.read(n)

    def do_GET(self):
        if self.path in ("/", "/health"):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok-ocr")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if TOKEN:
            auth = self.headers.get("Authorization", "")
            if auth != f"Bearer {TOKEN}":
                self.send_response(401)
                self.end_headers()
                self.wfile.write(b'{"error":"unauthorized"}')
                return
        try:
            raw = self._read_body()
        except Exception as e:
            print("read", e, flush=True)
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b'{"error":"bad body"}')
            return
        if not raw:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b'{"error":"empty"}')
            return
        try:
            text = ocr_bytes(raw)
            data = json.dumps({"text": text}, ensure_ascii=False).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            print("ocr", type(e).__name__, e, flush=True)
            self.send_response(500)
            self.end_headers()
            self.wfile.write(b'{"error":"ocr_failed"}')


if __name__ == "__main__":
    print("warming paddleocr...", flush=True)
    try:
        get_ocr()
    except Exception as e:
        print("warm failed", e, flush=True)
        raise
    print(f"ocr-paddle listening {HOST}:{PORT}", flush=True)
    ThreadingHTTPServer((HOST, PORT), H).serve_forever()
