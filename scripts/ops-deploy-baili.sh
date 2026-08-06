#!/usr/bin/env bash
# Deploy a GitHub Release binary onto the img.li host layout (/opt/baili + systemd baili).
#
# Run ON the production host (or via npc):
#   ./scripts/ops-deploy-baili.sh v0.9.5
#   IMGLI_VERSION=v0.9.5 ./scripts/ops-deploy-baili.sh
#
# Via NoPanel from a workstation (example):
#   npc server exec command "VIP Cloud" --timeout 180 -- \
#     "curl -fsSL https://raw.githubusercontent.com/yixian-huang/imgli/main/scripts/ops-deploy-baili.sh | bash -s -- v0.9.5"
#
# Env:
#   IMGLI_VERSION     default: latest GitHub release tag
#   IMGLI_REPO        default: yixian-huang/imgli
#   IMGLI_BIN_DIR     default: /opt/baili/bin
#   IMGLI_RELEASES    default: /opt/baili/releases
#   IMGLI_UNIT        default: baili
#   IMGLI_PUBLIC_URL  if set, run scripts/ops-smoke-public.sh after restart
#   IMGLI_HEALTH_URL  default: http://127.0.0.1:8686/healthz
set -euo pipefail

REPO="${IMGLI_REPO:-yixian-huang/imgli}"
BIN_DIR="${IMGLI_BIN_DIR:-/opt/baili/bin}"
REL_DIR="${IMGLI_RELEASES:-/opt/baili/releases}"
UNIT="${IMGLI_UNIT:-baili}"
HEALTH_URL="${IMGLI_HEALTH_URL:-http://127.0.0.1:8686/healthz}"
VERSION="${1:-${IMGLI_VERSION:-}}"

info() { printf 'deploy-baili: %s\n' "$*"; }
die()  { printf 'deploy-baili: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "need $1"; }
need curl
need tar
need install
need systemctl

if [ -z "$VERSION" ]; then
  info "resolving latest release…"
  loc="$(curl -fsSLI "https://github.com/${REPO}/releases/latest" | tr -d '\r' | awk -F': ' 'tolower($1)=="location"{print $2; exit}')"
  [ -n "$loc" ] || die "could not resolve latest release"
  VERSION="${loc##*/}"
fi
case "$VERSION" in
  v*) VER_NUM="${VERSION#v}" ;;
  *)  VER_NUM="$VERSION"; VERSION="v${VERSION}" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch="x86_64" ;;
  aarch64|arm64) goarch="arm64" ;;
  *) die "unsupported arch: $arch" ;;
esac

asset="imgli_${VER_NUM}_Linux_${goarch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
info "download $url"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/imgli-deploy.XXXXXX")"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT INT HUP TERM

curl -fsSL "$url" -o "${tmpdir}/${asset}" || die "download failed (is the Release published?)"
tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"
[ -x "${tmpdir}/imgli" ] || die "archive missing executable imgli"

mkdir -p "$BIN_DIR" "$REL_DIR"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
if [ -e "${BIN_DIR}/imgli" ]; then
  cp -a "${BIN_DIR}/imgli" "${BIN_DIR}/imgli.bak.${ts}"
  info "backed up previous binary → imgli.bak.${ts}"
fi

install -m 755 "${tmpdir}/imgli" "${BIN_DIR}/imgli"
cp -a "${BIN_DIR}/imgli" "${REL_DIR}/imgli-${VERSION}-${ts}"
# legacy symlink used on img.li
if [ -L "${BIN_DIR}/baili" ] || [ ! -e "${BIN_DIR}/baili" ]; then
  ln -sfn imgli "${BIN_DIR}/baili"
fi

info "restart ${UNIT}"
systemctl restart "$UNIT"
sleep 1
systemctl is-active "$UNIT" >/dev/null || die "unit ${UNIT} not active"

# wait healthz
ok=0
for i in $(seq 1 30); do
  if curl -sfS "$HEALTH_URL" >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done
[ "$ok" = "1" ] || die "healthz not ready: $HEALTH_URL"

ver_out="$("${BIN_DIR}/imgli" version 2>/dev/null || true)"
info "installed version: ${ver_out:-unknown}"
info "active unit: $(systemctl is-active "$UNIT")"

if [ -n "${IMGLI_PUBLIC_URL:-}" ]; then
  script_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
  if [ -x "${script_dir}/ops-smoke-public.sh" ]; then
    "${script_dir}/ops-smoke-public.sh" "${IMGLI_PUBLIC_URL}"
  else
    info "skip public smoke (ops-smoke-public.sh missing)"
  fi
fi

info "PASS deploy ${VERSION}"
