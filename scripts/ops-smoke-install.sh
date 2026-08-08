#!/usr/bin/env sh
# Install-path smoke: prove a fresh install can start, serve SPA, register, upload.
# Covers what users actually do — not just "image builds".
#
# Usage:
#   ./scripts/ops-smoke-install.sh docker [IMAGE]
#   ./scripts/ops-smoke-install.sh binary [BINARY]
#   IMAGE=ghcr.io/yixian-huang/imgli:v0.9.6 ./scripts/ops-smoke-install.sh docker
#   BINARY=./imgli ./scripts/ops-smoke-install.sh binary
#
# Env:
#   PORT          listen port (default: free high port)
#   SKIP_BIND     if 1, docker mode skips bind-mount scenario (named volume only)
#   PULL          if 1 (default for remote images), docker pull first
set -eu

MODE="${1:-}"
ARG="${2:-}"

info() { printf 'install-smoke: %s\n' "$*"; }
die()  { printf 'install-smoke: FAIL: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "need $1"; }

need curl
need python3

# --- tiny 1x1 PNG ---
PNG_B64='iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII='

pick_port() {
  if [ -n "${PORT:-}" ]; then
    echo "$PORT"
    return
  fi
  # Prefer python for a free port
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

wait_health() {
  base="$1"
  i=0
  while [ "$i" -lt 45 ]; do
    if curl -sfS "$base/healthz" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

# HTTP checks + first-user register + multipart upload + fetch object
assert_user_journey() {
  base="$1"
  base="${base%/}"
  info "journey against $base"

  # SPA / public smoke (inline to avoid double dependency path issues)
  code="$(curl -sfS -o /dev/null -w '%{http_code}' "$base/" || true)"
  [ "$code" = "200" ] || die "GET / → HTTP $code"
  code="$(curl -sfS -o /dev/null -w '%{http_code}' "$base/healthz" || true)"
  [ "$code" = "200" ] || die "GET /healthz → HTTP $code"
  html="$(curl -sfS "$base/")" || die "fetch /"
  echo "$html" | grep -q 'id="root"' || die 'HTML missing id="root"'
  js="$(echo "$html" | sed -n 's/.*src="\(\/assets\/index-[^"]*\.js\)".*/\1/p' | head -1)"
  [ -n "$js" ] || die "no SPA bundle in HTML"
  code="$(curl -sfS -o /dev/null -w '%{http_code}' "$base$js" || true)"
  [ "$code" = "200" ] || die "GET $js → HTTP $code"

  cfg="$(curl -sfS "$base/api/v1/config")" || die "config"
  echo "$cfg" | python3 -c "
import sys, json
d=json.load(sys.stdin)
assert d.get('status') is True, d
assert isinstance(d.get('data'), dict), d
print('config ok site_name=', repr((d.get('data') or {}).get('site_name'))[:40])
" || die "config shape"

  # Cookie jar for session after first-user register (becomes admin)
  jar="$(mktemp)"
  png="$(mktemp)"
  # POSIX sh: no RETURN trap — clean up at end of function
  journey_cleanup() { rm -f "$jar" "$png" 2>/dev/null || true; }

  if ! reg="$(curl -sfS -c "$jar" -b "$jar" -X POST "$base/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d '{"username":"smokebot","email":"smokebot@example.com","password":"Smoke-pass-1"}')"; then
    journey_cleanup
    die "register first user failed (is data dir empty?)"
  fi
  if ! echo "$reg" | python3 -c "
import sys, json
d=json.load(sys.stdin)
assert d.get('status') is True, d
u=(d.get('data') or {})
assert u.get('username')=='smokebot', u
print('registered id=', u.get('id'))
"; then
    journey_cleanup
    die "register response"
  fi

  if ! echo "$PNG_B64" | base64 -d >"$png" 2>/dev/null; then
    echo "$PNG_B64" | base64 --decode >"$png" || { journey_cleanup; die "base64 decode png"; }
  fi

  # Session cookie upload (Origin omitted — OriginCheck allows missing Origin)
  if ! up="$(curl -sfS -c "$jar" -b "$jar" -X POST "$base/api/v1/upload" \
    -F "file=@${png};filename=smoke.png;type=image/png" \
    -F visibility=public)"; then
    journey_cleanup
    die "upload failed"
  fi
  if ! url="$(echo "$up" | python3 -c "
import sys, json
d=json.load(sys.stdin)
assert d.get('status') is True, d
data=d.get('data') or {}
links=data.get('links') or {}
url=links.get('url') or ''
assert url, data
print(url)
")"; then
    journey_cleanup
    die "upload response missing links.url"
  fi
  info "uploaded $url"

  # Resolve relative /i/... against base
  case "$url" in
    http*|//*) obj="$url" ;;
    /*) obj="$base$url" ;;
    *) obj="$base/$url" ;;
  esac
  code="$(curl -sfS -o /dev/null -w '%{http_code}' "$obj" || true)"
  if [ "$code" != "200" ]; then
    journey_cleanup
    die "GET object $obj → HTTP $code (want 200)"
  fi

  journey_cleanup
  info "journey PASS"
}

run_binary() {
  bin="${ARG:-${BINARY:-}}"
  [ -n "$bin" ] || die "binary mode needs path: $0 binary ./imgli"
  [ -x "$bin" ] || [ -f "$bin" ] || die "binary not found: $bin"
  chmod +x "$bin" 2>/dev/null || true

  port="$(pick_port)"
  data="$(mktemp -d)"
  log="$(mktemp)"
  base="http://127.0.0.1:${port}"
  info "binary=$bin port=$port data=$data"

  IMGLI_LISTEN=":${port}" \
  IMGLI_BASE_URL="$base" \
  IMGLI_DATA_DIR="$data" \
    "$bin" serve >"$log" 2>&1 &
  pid=$!
  cleanup() {
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    rm -rf "$data" "$log"
  }
  trap cleanup EXIT INT TERM

  if ! wait_health "$base"; then
    info "server log tail:"
    tail -n 40 "$log" >&2 || true
    die "binary did not become healthy"
  fi
  ver="$("$bin" version 2>/dev/null || true)"
  info "version=${ver:-unknown}"
  assert_user_journey "$base"
  info "binary smoke PASS"
}

run_docker() {
  need docker
  img="${ARG:-${IMAGE:-}}"
  [ -n "$img" ] || die "docker mode needs image: $0 docker ghcr.io/yixian-huang/imgli:vX.Y.Z"

  if [ "${PULL:-1}" = "1" ]; then
    case "$img" in
      */*|ghcr.io/*|*.*) info "pull $img"; docker pull "$img" ;;
      *) info "skip pull local-ish tag $img" ;;
    esac
  fi

  port="$(pick_port)"
  base="http://127.0.0.1:${port}"
  name="imgli-smoke-$$"
  data_bind="$(mktemp -d)"
  # Ensure bind dir is root-owned pattern on CI (already is); entrypoint must fix for uid 1000.
  info "docker image=$img port=$port"

  # --- scenario A: named volume (recommended path) ---
  vol="imgli-smoke-vol-$$"
  docker volume create "$vol" >/dev/null
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker run -d --name "$name" \
    -p "${port}:8686" \
    -e "IMGLI_BASE_URL=${base}" \
    -e "IMGLI_DATA_DIR=/data" \
    -e "VIPS_CONCURRENCY=1" \
    -v "${vol}:/data" \
    "$img" >/dev/null

  cleanup_docker() {
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker volume rm -f "$vol" >/dev/null 2>&1 || true
    rm -rf "$data_bind"
  }
  trap cleanup_docker EXIT INT TERM

  if ! wait_health "$base"; then
    docker logs "$name" 2>&1 | tail -n 50 >&2 || true
    die "docker (named volume) not healthy"
  fi
  # version inside container
  dver="$(docker exec "$name" imgli version 2>/dev/null || true)"
  info "container version=${dver:-unknown}"
  assert_user_journey "$base"
  info "docker named-volume smoke PASS"

  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm -f "$vol" >/dev/null 2>&1 || true
  trap - EXIT INT TERM

  # --- scenario B: bind mount (regression for OOM / permission) ---
  if [ "${SKIP_BIND:-0}" = "1" ]; then
    info "SKIP_BIND=1 — skip bind-mount scenario"
    info "docker smoke PASS"
    return 0
  fi

  port="$(pick_port)"
  base="http://127.0.0.1:${port}"
  name="imgli-smoke-bind-$$"
  data_bind="$(mktemp -d)"
  # Fresh empty bind path owned by runner (often root on Actions)
  info "docker bind-mount data=$data_bind port=$port"

  docker rm -f "$name" >/dev/null 2>&1 || true
  docker run -d --name "$name" \
    -p "${port}:8686" \
    -e "IMGLI_BASE_URL=${base}" \
    -e "IMGLI_DATA_DIR=/data" \
    -e "VIPS_CONCURRENCY=1" \
    -v "${data_bind}:/data" \
    "$img" >/dev/null

  cleanup_bind() {
    docker rm -f "$name" >/dev/null 2>&1 || true
    # entrypoint chown's /data to uid 1000; host runner may not list/rm the bind path
    if [ -n "${data_bind:-}" ] && [ -d "$data_bind" ]; then
      docker run --rm -v "${data_bind}:/data" alpine:3.20 sh -c 'rm -rf /data/* /data/.[!.]* 2>/dev/null; true' >/dev/null 2>&1 || true
      rmdir "$data_bind" 2>/dev/null || rm -rf "$data_bind" 2>/dev/null || true
    fi
  }
  trap cleanup_bind EXIT INT TERM

  if ! wait_health "$base"; then
    docker logs "$name" 2>&1 | tail -n 80 >&2 || true
    die "docker (bind mount) not healthy — check entrypoint chown / SQLite path"
  fi
  # SQLite file should exist after first request (check inside container — host may lack list perms after chown)
  assert_user_journey "$base"
  if ! docker exec "$name" sh -c 'test -f /data/imgli.db || test -f /data/imgli.db-wal'; then
    docker exec "$name" ls -la /data >&2 || true
    die "bind mount: expected imgli.db under /data after serve"
  fi
  info "docker bind-mount smoke PASS"
  info "docker smoke PASS"
}

case "$MODE" in
  docker) run_docker ;;
  binary) run_binary ;;
  *)
    cat >&2 <<'EOF'
usage:
  ./scripts/ops-smoke-install.sh docker [IMAGE]
  ./scripts/ops-smoke-install.sh binary [BINARY]

Examples:
  ./scripts/ops-smoke-install.sh docker ghcr.io/yixian-huang/imgli:v0.9.6
  ./scripts/ops-smoke-install.sh binary ./imgli
EOF
    exit 2
    ;;
esac
