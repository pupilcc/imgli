#!/usr/bin/env bash
# S4 operator probe: anonymous GET against a private-surface object URL must not
# return 200. Use after deploying S3/CDN to catch world-readable buckets.
#
# Usage:
#   ./scripts/probe-private-object-anon.sh 'https://bucket.example/private/path/to/object.png'
#   OBJECT_URL='https://...' ./scripts/probe-private-object-anon.sh
#
# Exit 0 = looks locked down (403/401/404 or network deny without body success).
# Exit 1 = anonymous 2xx (leak) or bad usage.
set -euo pipefail

URL="${1:-${OBJECT_URL:-}}"
if [ -z "$URL" ]; then
	echo "usage: $0 <https://.../private/...object>" >&2
	exit 2
fi

case "$URL" in
	http://*|https://*) ;;
	*)
		echo "probe: URL must be http(s)" >&2
		exit 2
		;;
esac

# No auth headers — pure anonymous.
code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 -L "$URL" || echo '000')"

echo "probe: GET $URL → HTTP $code"

case "$code" in
	200|206)
		echo "FAIL: anonymous read succeeded — bucket/CDN likely world-readable for private keys (S4)." >&2
		echo "Fix: disable public ACL / anonymous GetObject on private/* ; do not put private/* on a public CDN origin." >&2
		exit 1
		;;
	401|403|404|405)
		echo "OK: anonymous access denied or not found ($code)."
		exit 0
		;;
	000)
		echo "WARN: request failed (timeout/TLS/DNS). Cannot confirm lock-down; check URL and network." >&2
		exit 2
		;;
	*)
		echo "OK-ish: non-success status $code (treat as locked unless you expected 200)."
		exit 0
		;;
esac
