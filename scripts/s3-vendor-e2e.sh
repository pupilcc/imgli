#!/usr/bin/env bash
# B-③ app 层 e2e:验证 app 管线接厂商驱动全链(上传落桶/回源字节一致/Range 206/缩略图/物理删除清桶)。
# 用法: scripts/s3-vendor-e2e.sh <vendor>;凭据同 matrix runner(~/.secrets/imgli-s3-vendors.env)。
set -euo pipefail
VENDOR="${1:?用法: $0 <vendor>}"
ENV_FILE="${VENDOR_ENV:-$HOME/.secrets/imgli-s3-vendors.env}"
[ -f "$ENV_FILE" ] || { echo "缺 $ENV_FILE" >&2; exit 2; }
# 只读入为 shell 变量,不整体 export(app/curl/python 子进程无需继承全部厂商凭据)
. "$ENV_FILE"
P=$(printf '%s' "$VENDOR" | tr '[:lower:]' '[:upper:]')
V() { eval "printf '%s' \"\${${P}_$1:-}\""; }
[ -n "$(V ENDPOINT)" ] || { echo "${P}_ENDPOINT 未配置" >&2; exit 2; }

ROOT=$(cd "$(dirname "$0")/.." && pwd)
DATA=$(mktemp -d /tmp/b3-e2e-XXXXXX)
BIN="$DATA/imgli"
PORT=18734; BASE="http://127.0.0.1:$PORT"; JAR="$DATA/jar"
go build -C "$ROOT" -o "$BIN" ./cmd/imgli

# 预置桶:判据测试「谁建谁删」可能已把自建桶带走;app 的 TestPolicy/上传需要桶在
"$ROOT/scripts/s3-vendor-matrix.sh" "$VENDOR" TestEnsureBucketLive >/dev/null
echo "桶预置 ✓"

IMGLI_LISTEN="127.0.0.1:$PORT" IMGLI_BASE_URL="$BASE" IMGLI_DATA_DIR="$DATA" \
IMGLI_RATE_LIMIT_MULT=100 "$BIN" serve &>"$DATA/serve.log" & SRV=$!
trap 'kill $SRV 2>/dev/null; rm -rf "$DATA"' EXIT
for i in $(seq 1 50); do curl -sf "$BASE/api/v1/config" >/dev/null && break; sleep 0.2; done

# 首注册用户即 admin
curl -sf -X POST "$BASE/api/v1/auth/register" -H "Origin: $BASE" -H 'Content-Type: application/json' \
  -c "$JAR" -d '{"username":"b3admin","email":"b3@x.io","password":"b3secret12"}' >/dev/null
curl -sf -X POST "$BASE/api/v1/auth/login" -H "Origin: $BASE" -H 'Content-Type: application/json' \
  -c "$JAR" -d '{"account":"b3admin","password":"b3secret12"}' >/dev/null
echo "注册/登录 ✓"

# 建厂商策略(config 是 JSON 字符串字段,故整体二次编码)
CFG=$(printf '{"endpoint":"%s","region":"%s","bucket":"%s","access_key_id":"%s","secret_access_key":"%s","path_style":"%s","prefix":"%s"}' \
  "$(V ENDPOINT)" "$(V REGION)" "$(V BUCKET)" "$(V AK)" "$(V SK)" "$(V PATHSTYLE)" "$(V PREFIX)")
BODY=$(printf '%s' "$CFG" | python3 -c 'import json,sys; print(json.dumps({"name":"b3-e2e","driver":"s3","config":sys.stdin.read()}))')
# 请求体走 stdin(--data-binary @-):secret_access_key 不进 curl argv,防同机 ps 泄漏
PID=$(printf '%s' "$BODY" | curl -sf -X POST "$BASE/api/v1/admin/policies" -H "Origin: $BASE" -H 'Content-Type: application/json' -b "$JAR" \
  --data-binary @- | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')
echo "策略 id=$PID"

# admin 测试连接(写/读/删探针)——失败必须中止(set -e),不许静默跳过
TESTRES=$(curl -s -X POST "$BASE/api/v1/admin/policies/$PID/test" -H "Origin: $BASE" -b "$JAR")
printf '%s' "$TESTRES" | python3 -c 'import json,sys; r=json.load(sys.stdin); assert r.get("status") is True, r' \
  || { echo "测试连接失败: $TESTRES" >&2; exit 1; }
echo "测试连接 ✓"

# 默认组放行 + 用户偏好指向该策略
curl -sf -X PATCH "$BASE/api/v1/admin/groups/1" -H "Origin: $BASE" -H 'Content-Type: application/json' -b "$JAR" \
  -d "{\"allowed_policy_ids\":[1,$PID]}" >/dev/null
curl -sf -X PATCH "$BASE/api/v1/user/preferences" -H "Origin: $BASE" -H 'Content-Type: application/json' -b "$JAR" \
  -d "{\"default_policy_id\":$PID}" >/dev/null
echo "组放行+偏好指向 ✓"

# 上传确定性 PNG(每次运行内容随机差异,避免与历史测试图秒传撞车)
python3 - "$DATA/probe.png" <<'PYEOF'
import os, struct, sys, zlib
w = h = 64
seed = int.from_bytes(os.urandom(2), "big")
raw = b"".join(b"\x00" + bytes((x*3 + y*7 + seed) % 256 for x in range(w*3)) for y in range(h))
def chunk(t, d): return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t+d))
open(sys.argv[1], "wb").write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))
PYEOF
KEY=$(curl -sf -X POST "$BASE/api/v1/upload" -H "Origin: $BASE" -b "$JAR" -F "file=@$DATA/probe.png" \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["key"])')
echo "上传 key=$KEY"
OBJ=$(sqlite3 "$DATA/imgli.db" "SELECT f.path FROM files f JOIN images i ON i.file_id=f.id WHERE i.key='$KEY'")
echo "对象键=$OBJ"

# 流式回源:字节一致 + Range 206(cmp/curl 失败即中止)
curl -sf "$BASE/i/$KEY.png" -o "$DATA/back.png"
cmp "$DATA/probe.png" "$DATA/back.png"
echo "回源字节一致 ✓"
RC=$(curl -s -o /dev/null -w '%{http_code}' -r 100-199 "$BASE/i/$KEY.png")
[ "$RC" = 206 ] || { echo "Range 返回 $RC ≠ 206" >&2; exit 1; }
echo "Range 206 ✓"
curl -sf "$BASE/t/$KEY.png" -o /dev/null
echo "缩略图 ✓"

# 删除 → 清回收站 → 等物理删除任务 → 桶清验证
curl -sf -X DELETE "$BASE/api/v1/images/$KEY" -H "Origin: $BASE" -b "$JAR" >/dev/null
curl -sf -X DELETE "$BASE/api/v1/trash/$KEY" -H "Origin: $BASE" -b "$JAR" >/dev/null
for i in $(seq 1 30); do
  N=$(sqlite3 "$DATA/imgli.db" "SELECT COUNT(*) FROM tasks WHERE status NOT IN ('done')")
  [ "$N" = 0 ] && break; sleep 1
done
# 轮询超时不许静默放行:任务未清零说明物理删除可能根本没执行(codex 评审)
[ "$N" = 0 ] || { echo "后台任务未清($N 个未完成),物理删除链路未闭合" >&2; sqlite3 "$DATA/imgli.db" "SELECT id,type,status,last_error FROM tasks WHERE status NOT IN ('done')" >&2; exit 1; }
IMGLI_TEST_S3_EXPECT_ABSENT_KEY="$OBJ" "$ROOT/scripts/s3-vendor-matrix.sh" "$VENDOR" TestKeyAbsentLive
echo "=== $VENDOR e2e 全链 ✓ ==="
