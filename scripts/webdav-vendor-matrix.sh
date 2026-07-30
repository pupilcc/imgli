#!/usr/bin/env bash
# WebDAV surface probe against a self-hosted or existing endpoint.
# You do NOT need a SaaS signup per brand — Docker/OpenList/NAS is enough.
#
# Usage:
#   export IMGLI_TEST_WEBDAV_ENDPOINT='http://127.0.0.1:8080/dav'
#   export IMGLI_TEST_WEBDAV_USERNAME='user'   # optional
#   export IMGLI_TEST_WEBDAV_PASSWORD='pass'   # optional
#   ./scripts/webdav-vendor-matrix.sh
#
# Or load from file (optional):
#   VENDOR_ENV=~/.secrets/imgli-webdav.env ./scripts/webdav-vendor-matrix.sh
set -euo pipefail
ENV_FILE="${VENDOR_ENV:-}"
if [ -n "$ENV_FILE" ]; then
  [ -f "$ENV_FILE" ] || { echo "缺 $ENV_FILE" >&2; exit 2; }
  # shellcheck disable=SC1090
  set -a
  # shellcheck source=/dev/null
  . "$ENV_FILE"
  set +a
fi
: "${IMGLI_TEST_WEBDAV_ENDPOINT:?set IMGLI_TEST_WEBDAV_ENDPOINT (no userinfo in URL)}"
export IMGLI_TEST_WEBDAV_LIVE=1
cd "$(dirname "$0")/.."
echo "WebDAV matrix live → endpoint=${IMGLI_TEST_WEBDAV_ENDPOINT%%/*}://… (credentials not printed)"
exec go test ./internal/storage/webdav/ -run TestDriverSurfaceLive -v -count=1
