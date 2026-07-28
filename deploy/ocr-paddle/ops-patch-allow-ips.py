#!/usr/bin/env python3
"""Patch an OCR sidecar server.py to honor ALLOW_IPS (ops helper)."""
from __future__ import annotations

import sys
from pathlib import Path


def patch(text: str) -> str:
    if "def _allow_ip" in text and "ALLOW_IPS" in text:
        return text

    host_line = 'HOST = os.environ.get("HOST", "0.0.0.0")\n'
    if host_line not in text:
        raise SystemExit("HOST line not found")
    text = text.replace(
        host_line,
        host_line
        + 'ALLOW_IPS = {x.strip() for x in os.environ.get("ALLOW_IPS", "").split(",") if x.strip()}\n',
        1,
    )

    helpers = """    def _client_ip(self) -> str:
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

"""
    marker = "    def do_GET(self):"
    if marker not in text:
        raise SystemExit("do_GET not found")
    if "def _allow_ip" not in text:
        text = text.replace(marker, helpers + marker, 1)

    get_old = "    def do_GET(self):\n        if self.path in"
    get_new = (
        "    def do_GET(self):\n"
        "        if not self._allow_ip():\n"
        '            print("deny GET ip=%s" % self._client_ip(), flush=True)\n'
        "            self._deny()\n"
        "            return\n"
        "        if self.path in"
    )
    if get_old not in text:
        raise SystemExit("do_GET body pattern not found")
    text = text.replace(get_old, get_new, 1)

    post_old = "    def do_POST(self):\n        if TOKEN:"
    post_new = (
        "    def do_POST(self):\n"
        "        if not self._allow_ip():\n"
        '            print("deny POST ip=%s" % self._client_ip(), flush=True)\n'
        "            self._deny()\n"
        "            return\n"
        "        if TOKEN:"
    )
    if post_old not in text:
        raise SystemExit("do_POST body pattern not found")
    text = text.replace(post_old, post_new, 1)

    for old, new in (
        (
            'print(f"ocr-paddle listening {HOST}:{PORT}", flush=True)',
            'print(f"ocr-paddle listening {HOST}:{PORT} allow_ips={sorted(ALLOW_IPS) or \'ANY\'}", flush=True)',
        ),
        (
            'print(f"ocr-rapid listening {HOST}:{PORT}", flush=True)',
            'print(f"ocr-rapid listening {HOST}:{PORT} allow_ips={sorted(ALLOW_IPS) or \'ANY\'}", flush=True)',
        ),
    ):
        if old in text:
            text = text.replace(old, new, 1)
            break
    return text


def main() -> None:
    src = Path(sys.argv[1])
    dst = Path(sys.argv[2] if len(sys.argv) > 2 else sys.argv[1])
    out = patch(src.read_text())
    dst.write_text(out)
    print(f"patched {src} -> {dst} bytes={len(out)}")


if __name__ == "__main__":
    main()
