#!/usr/bin/env python3
"""imgli OCR sidecar — RapidOCR (PP-OCR ONNX models).

Drop-in for Tesseract/PaddleOCR sidecar protocol:
  POST /  Authorization: Bearer <token>  body=raw image bytes
  200 {"text":"..."}
  GET /health -> ok-ocr

Why RapidOCR: paddlepaddle 2.6 CPU wheels segfault on import on some
newer host CPUs (e.g. Zen5 KVM). RapidOCR runs the same PP-OCR family
models via onnxruntime, which is portable.
"""
from __future__ import annotations

from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
import threading

TOKEN = os.environ.get("TOKEN", "").strip()
PORT = int(os.environ.get("PORT", "3199"))
HOST = os.environ.get("HOST", "0.0.0.0")
MAX = 20 * 1024 * 1024
# Comma-separated IPs. Empty = allow all (dev). Production: VIP egress + loopback.
ALLOW_IPS = {x.strip() for x in os.environ.get("ALLOW_IPS", "").split(",") if x.strip()}

_engine = None
_lock = threading.Lock()


def get_engine():
    global _engine
    with _lock:
        if _engine is None:
            from rapidocr_onnxruntime import RapidOCR

            # Defaults: Chinese+English PP-OCR det/rec/cls ONNX
            _engine = RapidOCR()
            print("rapidocr ready", flush=True)
        return _engine


def ocr_bytes(data: bytes) -> str:
    from PIL import Image
    import numpy as np

    img = Image.open(io.BytesIO(data))
    if img.mode not in ("RGB", "L"):
        img = img.convert("RGB")
    if img.mode == "L":
        img = img.convert("RGB")
    arr = np.array(img)
    result, _elapse = get_engine()(arr)
    if not result:
        return ""
    lines = []
    for item in result:
        # item: [box, text, score]
        try:
            text = item[1]
            if text:
                lines.append(str(text))
        except (IndexError, TypeError):
            continue
    return "\n".join(lines)


class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[ocr-rapid]", fmt % args, flush=True)

    def _client_ip(self) -> str:
        return self.client_address[0]

    def _allow_ip(self) -> bool:
        if not ALLOW_IPS:
            return True
        ip = self._client_ip()
        return ip in ALLOW_IPS or ip in ("127.0.0.1", "::1")

    def _deny(self, code: int = 403) -> None:
        self.send_response(code)
        self.end_headers()
        self.wfile.write(b'{"error":"forbidden"}')

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
        if not self._allow_ip():
            print("deny GET ip=%s" % self._client_ip(), flush=True)
            self._deny()
            return
        if self.path in ("/", "/health"):
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok-ocr")
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if not self._allow_ip():
            print("deny POST ip=%s" % self._client_ip(), flush=True)
            self._deny()
            return
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
    print("warming rapidocr...", flush=True)
    get_engine()
    print(
        f"ocr-rapid listening {HOST}:{PORT} allow_ips={sorted(ALLOW_IPS) or 'ANY'}",
        flush=True,
    )
    ThreadingHTTPServer((HOST, PORT), H).serve_forever()
