#!/usr/bin/env bash
# VIP img.li — online SQLite backup for /opt/baili/data/baili.db
# Style: set -euo pipefail; paths default to /opt/baili, override via env below.
set -euo pipefail

DATA_DIR="${DATA_DIR:-/opt/baili/data}"
DB="${DB:-$DATA_DIR/baili.db}"
BACKUP_DIR="${BACKUP_DIR:-$DATA_DIR/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
STAMP="$(date +%Y%m%d)"
DEST="$BACKUP_DIR/baili.backup-${STAMP}.db"

if [[ -w /var/log ]]; then
  LOG="${LOG:-/var/log/baili-backup.log}"
else
  LOG="${LOG:-$DATA_DIR/backup.log}"
fi

log() { echo "[$(date -Iseconds)] $*" | tee -a "$LOG" >&2; }

if [[ ! -f "$DB" ]]; then
  log "ERROR: db not found: $DB"
  exit 1
fi
if ! command -v sqlite3 >/dev/null 2>&1; then
  log "ERROR: sqlite3 not installed"
  exit 1
fi

mkdir -p "$BACKUP_DIR"
log "backup start db=$DB dest=$DEST"
sqlite3 "$DB" ".backup '$DEST'"
chmod 600 "$DEST" 2>/dev/null || true
SIZE="$(wc -c <"$DEST" | tr -d ' ')"
log "backup ok bytes=$SIZE path=$DEST"

# optional: dump moderation JSON snapshot (no secrets expansion beyond what is in DB)
if command -v python3 >/dev/null 2>&1; then
  SECRETS_DIR="${SECRETS_DIR:-/opt/baili/.secrets}"
  if [[ -d "$SECRETS_DIR" ]]; then
    SNAP="$SECRETS_DIR/moderation.backup-${STAMP}.json"
    python3 - <<PY
import sqlite3, pathlib
db = sqlite3.connect("$DB")
row = db.execute("SELECT value FROM settings WHERE key='moderation'").fetchone()
if row:
    p = pathlib.Path("$SNAP")
    p.write_text(row[0])
    p.chmod(0o600)
    print("moderation snapshot", p)
PY
  fi
fi

# retention
find "$BACKUP_DIR" -name 'baili.backup-*.db' -type f -mtime +"$RETENTION_DAYS" -print -delete 2>/dev/null | while read -r f; do
  log "retained-delete $f"
done || true

log "backup done"
echo "$DEST"
