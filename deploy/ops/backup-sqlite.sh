#!/usr/bin/env bash
# Online SQLite backup via sqlite3 .backup (safe while imgli is running).
#
# Defaults keep VIP legacy paths (/opt/baili, baili.db). Generic install:
#   DATA_DIR=/data DB=/data/imgli.db BACKUP_DIR=/data/backups ./backup-sqlite.sh
#
# See docs/backup.md for restore and Postgres.
set -euo pipefail

DATA_DIR="${DATA_DIR:-/opt/baili/data}"
DB="${DB:-$DATA_DIR/baili.db}"
BACKUP_DIR="${BACKUP_DIR:-$DATA_DIR/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"
STAMP="$(date +%Y%m%d)"
# File name prefix: imgli.backup-* or baili.backup-* both cleaned by retention below
NAME_PREFIX="${NAME_PREFIX:-imgli}"
# Legacy VIP hosts often use baili.* names
if [[ "$(basename "$DB")" == baili.db ]]; then
  NAME_PREFIX="${NAME_PREFIX:-baili}"
fi
DEST="$BACKUP_DIR/${NAME_PREFIX}.backup-${STAMP}.db"

if [[ -w /var/log ]]; then
  LOG="${LOG:-/var/log/${NAME_PREFIX}-backup.log}"
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

# retention (legacy baili.* and imgli.*)
find "$BACKUP_DIR" \( -name 'baili.backup-*.db' -o -name 'imgli.backup-*.db' \) \
  -type f -mtime +"$RETENTION_DAYS" -print -delete 2>/dev/null | while read -r f; do
  log "retained-delete $f"
done || true

log "backup done"
echo "$DEST"
