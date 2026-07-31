#!/usr/bin/env bash
# B-③ 厂商矩阵 runner:把 <VENDOR>_* 映射为 IMGLI_TEST_S3_* 后跑 live 判据。
# 用法: scripts/s3-vendor-matrix.sh <minio|qiniu|cos|oss|upyun|r2> [go test -run 正则]
# 凭据文件: ~/.secrets/imgli-s3-vendors.env(VENDOR_ENV 可覆盖)。凭据不回显。
set -euo pipefail
VENDOR="${1:?用法: $0 <minio|qiniu|cos|oss|upyun|r2> [-run 正则]}"
RUN="${2:-TestDriverSurfaceLive|TestPresignGetLive}"
ENV_FILE="${VENDOR_ENV:-$HOME/.secrets/imgli-s3-vendors.env}"
[ -f "$ENV_FILE" ] || { echo "缺 $ENV_FILE(模板: scripts/imgli-s3-vendors.env.example)" >&2; exit 2; }
# 只读入为 shell 变量,不整体 export——否则全部厂商的 AK/SK 会泄给每个子进程(codex 评审)
. "$ENV_FILE"
P=$(printf '%s' "$VENDOR" | tr '[:lower:]' '[:upper:]')
for k in ENDPOINT REGION AK SK BUCKET PATHSTYLE PREFIX PRESIGN_DOMAIN; do
  unset "IMGLI_TEST_S3_${k}" || true   # 防继承调用环境旧值:空字段沿用旧账号/桶会连错资源
  v=$(eval "printf '%s' \"\${${P}_${k}:-}\"")
  [ -n "$v" ] && export "IMGLI_TEST_S3_${k}=${v}"
done
for k in ENDPOINT REGION AK SK BUCKET; do
  eval "[ -n \"\${IMGLI_TEST_S3_${k}:-}\" ]" || { echo "${P}_${k} 未配置" >&2; exit 2; }
done
export IMGLI_TEST_S3_LIVE=1
cd "$(dirname "$0")/.."
exec go test ./internal/storage/s3/ -run "$RUN" -v -count=1
