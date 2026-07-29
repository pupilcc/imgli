#!/usr/bin/env sh
# Install the latest imgli release binary from GitHub.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/yixian-huang/imgli/main/scripts/install.sh | sh
#   curl -fsSL ... | sh -s -- v0.2.0          # pin a tag
#   PREFIX=$HOME/bin curl -fsSL ... | sh      # install directory (default: ~/.local/bin)
#
# Requires: curl, tar (zip+unzip on Windows via Git Bash/MSYS is best-effort).
set -eu

REPO="${IMGLI_REPO:-yixian-huang/imgli}"
BINARY="imgli"
PREFIX="${PREFIX:-${IMGLI_INSTALL_DIR:-$HOME/.local/bin}}"
VERSION="${1:-${IMGLI_VERSION:-}}"

info() { printf 'imgli-install: %s\n' "$*"; }
err()  { printf 'imgli-install: %s\n' "$*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || err "need '$1' in PATH"
}

need curl
need tar
need mktemp

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
	linux)  goos="Linux" ;;
	darwin) goos="Darwin" ;;
	msys*|mingw*|cygwin*) goos="Windows" ;;
	*) err "unsupported OS: $(uname -s) (download from https://github.com/${REPO}/releases)" ;;
esac

case "$arch" in
	x86_64|amd64) goarch="x86_64" ;;
	aarch64|arm64) goarch="arm64" ;;
	*) err "unsupported arch: $arch" ;;
esac

if [ -z "$VERSION" ]; then
	info "resolving latest release…"
	# Prefer the redirect Location header (no API rate limit / no jq).
	loc="$(curl -fsSLI "https://github.com/${REPO}/releases/latest" | tr -d '\r' | awk -F': ' 'tolower($1)=="location"{print $2; exit}')"
	[ -n "$loc" ] || err "could not resolve latest release URL"
	VERSION="${loc##*/}"
fi

# Tag is vX.Y.Z; archive name uses Version without leading v (GoReleaser).
case "$VERSION" in
	v*) ver_num="${VERSION#v}" ;;
	*)  ver_num="$VERSION"; VERSION="v${VERSION}" ;;
esac

ext="tar.gz"
if [ "$goos" = "Windows" ]; then
	ext="zip"
	need unzip
fi

asset="${BINARY}_${ver_num}_${goos}_${goarch}.${ext}"
base="https://github.com/${REPO}/releases/download/${VERSION}"
url="${base}/${asset}"
sum_url="${base}/checksums.txt"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/imgli-install.XXXXXX")"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT INT HUP TERM

info "downloading ${asset} (${VERSION})…"
if ! curl -fsSL "$url" -o "${tmpdir}/${asset}"; then
	err "download failed: ${url}
Is there a published GitHub Release with assets for ${VERSION}?
See https://github.com/${REPO}/releases"
fi

if curl -fsSL "$sum_url" -o "${tmpdir}/checksums.txt" 2>/dev/null; then
	info "verifying checksum…"
	# checksums.txt lines: "<sha256>  <filename>" (GoReleaser)
	expected="$(awk -v f="$asset" '$2==f {print $1; exit}' "${tmpdir}/checksums.txt")"
	if [ -n "$expected" ]; then
		if command -v sha256sum >/dev/null 2>&1; then
			got="$(sha256sum "${tmpdir}/${asset}" | awk '{print $1}')"
		elif command -v shasum >/dev/null 2>&1; then
			got="$(shasum -a 256 "${tmpdir}/${asset}" | awk '{print $1}')"
		else
			info "no sha256sum/shasum; skip checksum verify"
			got=""
		fi
		if [ -n "$got" ] && [ "$got" != "$expected" ]; then
			err "checksum mismatch for ${asset}
  expected ${expected}
  got      ${got}"
		fi
	else
		info "asset not listed in checksums.txt; skip verify"
	fi
else
	info "checksums.txt not found; skip verify"
fi

info "extracting…"
case "$ext" in
	tar.gz) tar -xzf "${tmpdir}/${asset}" -C "$tmpdir" ;;
	zip)    unzip -q "${tmpdir}/${asset}" -d "$tmpdir" ;;
esac

bin_path="${tmpdir}/${BINARY}"
if [ "$goos" = "Windows" ]; then
	bin_path="${tmpdir}/${BINARY}.exe"
fi
[ -f "$bin_path" ] || err "binary not found in archive (expected ${BINARY})"

mkdir -p "$PREFIX"
dest="${PREFIX}/${BINARY}"
if [ "$goos" = "Windows" ]; then
	dest="${PREFIX}/${BINARY}.exe"
fi

# Prefer install(1) when available; fall back to cp + chmod.
if command -v install >/dev/null 2>&1; then
	install -m 755 "$bin_path" "$dest"
else
	cp "$bin_path" "$dest"
	chmod 755 "$dest"
fi

info "installed ${dest}"
if ! "$dest" version >/dev/null 2>&1; then
	info "warning: could not run '${dest} version' (still installed)"
else
	info "version: $("$dest" version)"
fi

case ":$PATH:" in
	*":${PREFIX}:"*) ;;
	*)
		info "note: ${PREFIX} is not on PATH; add it, e.g.:"
		info "  export PATH=\"${PREFIX}:\$PATH\""
		;;
esac

info "quick start:"
info "  ${BINARY} serve"
info "  # → http://localhost:8686  (first registered user becomes admin)"
