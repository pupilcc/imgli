#!/usr/bin/env sh
# Public SPA smoke — catches "healthz green, frontend white-screen" class of outages.
# See docs/ops-deploy-checklist.md
#
# Usage:
#   ./scripts/ops-smoke-public.sh
#   ./scripts/ops-smoke-public.sh https://img.li
#   BASE_URL=https://img.li ./scripts/ops-smoke-public.sh
set -eu

BASE="${1:-${BASE_URL:-https://img.li}}"
BASE="${BASE%/}"

info() { printf 'smoke: %s\n' "$*"; }
die()  { printf 'smoke: FAIL: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "need $1"; }
need curl

info "base=$BASE"

code="$(curl -sfS -o /dev/null -w '%{http_code}' "$BASE/" || true)"
[ "$code" = "200" ] || die "GET / → HTTP $code (want 200)"

code="$(curl -sfS -o /dev/null -w '%{http_code}' "$BASE/healthz" || true)"
[ "$code" = "200" ] || die "GET /healthz → HTTP $code (want 200)"

html="$(curl -sfS "$BASE/")" || die "fetch / body"
echo "$html" | grep -q 'id="root"' || die 'HTML missing id="root"'

js="$(echo "$html" | sed -n 's/.*src="\(\/assets\/index-[^"]*\.js\)".*/\1/p' | head -1)"
[ -n "$js" ] || die "could not find /assets/index-*.js in HTML"
info "bundle=$js"
code="$(curl -sfS -o /dev/null -w '%{http_code}' "$BASE$js" || true)"
[ "$code" = "200" ] || die "GET $js → HTTP $code"

# Public config must parse; theme keys optional but response shape required
cfg="$(curl -sfS "$BASE/api/v1/config")" || die "GET /api/v1/config failed"
echo "$cfg" | python3 -c "
import sys, json
try:
    env = json.load(sys.stdin)
except Exception as e:
    raise SystemExit(f'config json: {e}')
if env.get('status') is not True:
    raise SystemExit(f'config status not true: {env!r}')
data = env.get('data') or {}
if not isinstance(data, dict):
    raise SystemExit('config data not object')
# locale-shaped fields must not be raw objects mistaken as React children later
ann = data.get('announcement') or {}
if isinstance(ann, dict):
    t = ann.get('text')
    if t is not None and not isinstance(t, (str, dict)):
        raise SystemExit(f'announcement.text bad type {type(t)}')
print('config ok; site_name=', repr(data.get('site_name',''))[:40])
print('theme_glass' in data and 'theme keys present' or 'theme keys absent (old binary?)')
" || die "config shape check failed"

info "PASS"
