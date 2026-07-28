#!/usr/bin/env python3
"""imgli OCR sidecar — paddlepaddle 3.0 + paddleocr 2.9 (predict-stable on EPYC)."""
from __future__ import annotations
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io, json, os, threading

for _k, _v in (
    ("FLAGS_use_mkldnn", "0"),
    ("FLAGS_use_onednn", "0"),
    ("FLAGS_enable_pir_api", "0"),
    ("OMP_NUM_THREADS", "2"),
):
    os.environ.setdefault(_k, _v)

TOKEN = os.environ.get("TOKEN", "").strip()
PORT = int(os.environ.get("PORT", "3199"))
HOST = os.environ.get("HOST", "0.0.0.0")
MAX = 20 * 1024 * 1024
LANG = os.environ.get("OCR_LANG", "ch")
USE_ORIENT = os.environ.get("OCR_USE_ANGLE_CLS", "1") not in ("0", "false", "False")
# Comma-separated IPs. Empty = allow all (dev). Production: VIP egress + loopback.
# Example: ALLOW_IPS=203.0.113.10,127.0.0.1
ALLOW_IPS = {x.strip() for x in os.environ.get("ALLOW_IPS", "").split(",") if x.strip()}
_ocr = None
_lock = threading.Lock()

def get_ocr():
    global _ocr
    with _lock:
        if _ocr is None:
            import paddle
            try:
                paddle.set_flags({"FLAGS_use_mkldnn": False})
            except Exception as e:
                print("set_flags", e, flush=True)
            from paddleocr import PaddleOCR
            _ocr = PaddleOCR(
                use_angle_cls=USE_ORIENT,
                lang=LANG,
                use_gpu=False,
                enable_mkldnn=False,
                show_log=False,
            )
            print("paddleocr2 ready lang=%s" % LANG, flush=True)
        return _ocr

def ocr_bytes(data: bytes) -> str:
    from PIL import Image
    import numpy as np
    img = Image.open(io.BytesIO(data)).convert("RGB")
    arr = np.array(img)
    ocr = get_ocr()
    result = ocr.ocr(arr, cls=USE_ORIENT)
    lines = []
    if result:
        for block in result:
            if not block:
                continue
            for item in block:
                if item and len(item) >= 2 and item[1]:
                    text = item[1][0] if isinstance(item[1], (list, tuple)) else str(item[1])
                    if text and str(text).strip():
                        lines.append(str(text).strip())
    return "\n".join(lines)

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[ocr-paddle]", fmt % args, flush=True)

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
            self.send_response(200); self.end_headers(); self.wfile.write(b"ok-ocr"); return
        self.send_response(404); self.end_headers()
    def do_POST(self):
        if not self._allow_ip():
            print("deny POST ip=%s" % self._client_ip(), flush=True)
            self._deny()
            return
        if TOKEN:
            auth = self.headers.get("Authorization", "")
            if auth != f"Bearer {TOKEN}":
                self.send_response(401); self.end_headers()
                self.wfile.write(b'{"error":"unauthorized"}'); return
        try:
            raw = self._read_body()
        except Exception as e:
            print("read", e, flush=True)
            self.send_response(400); self.end_headers(); self.wfile.write(b'{"error":"bad body"}'); return
        if not raw:
            self.send_response(400); self.end_headers(); self.wfile.write(b'{"error":"empty"}'); return
        try:
            text = ocr_bytes(raw)
            data = json.dumps({"text": text}, ensure_ascii=False).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers(); self.wfile.write(data)
        except Exception as e:
            print("ocr", type(e).__name__, e, flush=True)
            self.send_response(500); self.end_headers(); self.wfile.write(b'{"error":"ocr_failed"}')

if __name__ == "__main__":
    print("warming paddleocr2...", flush=True)
    get_ocr()
    print(
        f"ocr-paddle listening {HOST}:{PORT} allow_ips={sorted(ALLOW_IPS) or 'ANY'}",
        flush=True,
    )
    ThreadingHTTPServer((HOST, PORT), H).serve_forever()
