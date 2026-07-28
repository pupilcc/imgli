#!/usr/bin/env bash
# Run on 腾讯云轻量 OCR host as docker-capable user.
set -euo pipefail
VIP_IP="${VIP_IP:?set VIP_IP to the caller IP allowed to reach OCR (e.g. 203.0.113.10)}"
WORKDIR="${WORKDIR:-/var/tmp/imgli-ocr-ops}"
mkdir -p "$WORKDIR"

echo "=== preflight ==="
docker ps -a --filter name=baili-ocr --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'
curl -sS -m 3 http://127.0.0.1:3199/health || true
echo

if [[ ! -f "$WORKDIR/ops-patch-allow-ips.py" ]]; then
  echo "missing $WORKDIR/ops-patch-allow-ips.py" >&2
  exit 1
fi

docker cp baili-ocr:/app/server.py "$WORKDIR/server.py.orig"
python3 "$WORKDIR/ops-patch-allow-ips.py" "$WORKDIR/server.py.orig" "$WORKDIR/server.py"
grep -q ALLOW_IPS "$WORKDIR/server.py"

TOKEN=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' baili-ocr | sed -n 's/^TOKEN=//p')
test -n "$TOKEN"

docker rm -f baili-ocr-tmp 2>/dev/null || true
docker create --name baili-ocr-tmp baili-ocr:paddle-native >/dev/null
docker cp "$WORKDIR/server.py" baili-ocr-tmp:/app/server.py
docker commit baili-ocr-tmp baili-ocr:paddle-native-allowips >/dev/null
docker rm -f baili-ocr-tmp >/dev/null

TS=$(date +%Y%m%dT%H%M%SZ)
docker stop baili-ocr
docker rename baili-ocr "baili-ocr.bak.$TS"
# IMPORTANT: image may inherit a bad ENTRYPOINT from intermediate commit (e.g. sleep).
# Always force python3 + server.py.
docker run -d --name baili-ocr --restart unless-stopped --network host \
  --entrypoint python3 \
  -e TOKEN="$TOKEN" \
  -e PORT=3199 \
  -e HOST=0.0.0.0 \
  -e "ALLOW_IPS=${VIP_IP},127.0.0.1" \
  -e FLAGS_use_mkldnn=0 \
  -e FLAGS_use_onednn=0 \
  -e FLAGS_enable_pir_api=0 \
  -e OMP_NUM_THREADS=2 \
  -e OCR_LANG=ch \
  baili-ocr:paddle-native-allowips /app/server.py

for i in $(seq 1 40); do
  if curl -sS -m 2 http://127.0.0.1:3199/health 2>/dev/null | grep -q ok-ocr; then
    echo "cutover_ready_${i}"
    break
  fi
  sleep 3
done
echo "local=$(curl -sS -m 5 http://127.0.0.1:3199/health || echo FAIL)"
docker logs baili-ocr --tail 20 2>&1 | tail -25
docker ps --filter name=baili-ocr --format '{{.Names}} {{.Status}} {{.Image}}'
