#!/usr/bin/env bash
# VIP img.li — lightweight health snapshot (disk / SQLite ops queues / process).
# Exit 0 = ok or warn; exit 1 = critical (disk full, baili down, huge backlog).
# Cron example: */15 * * * * /opt/baili/bin/health-check.sh >/dev/null 2>&1
set -euo pipefail

DATA_DIR="${DATA_DIR:-/opt/baili/data}"
DB="${DB:-$DATA_DIR/baili.db}"
BIN="${BIN:-/opt/baili/bin/baili}"
DISK_PATH="${DISK_PATH:-/opt}"
# thresholds
DISK_WARN_PCT="${DISK_WARN_PCT:-85}"
DISK_CRIT_PCT="${DISK_CRIT_PCT:-95}"
PENDING_WARN="${PENDING_WARN:-50}"
TASK_PENDING_WARN="${TASK_PENDING_WARN:-100}"
TASK_RUNNING_WARN="${TASK_RUNNING_WARN:-20}"

if [[ -w /var/log ]]; then
  LOG="${LOG:-/var/log/baili-health.log}"
else
  LOG="${LOG:-$DATA_DIR/health.log}"
fi

log() { echo "[$(date -Iseconds)] $*" | tee -a "$LOG" >&2; }

level=0 # 0 ok, 1 warn, 2 crit
note() {
  local sev="$1"; shift
  case "$sev" in
    warn) [[ $level -lt 1 ]] && level=1 ;;
    crit) level=2 ;;
  esac
  log "$sev: $*"
}

# --- process ---
if pgrep -f "$BIN serve" >/dev/null 2>&1; then
  log "ok: baili process up"
else
  note crit "baili process not running (pattern: $BIN serve)"
fi

# --- disk ---
if df -P "$DISK_PATH" >/dev/null 2>&1; then
  # POSIX df: use last line, capacity % field
  use_pct="$(df -P "$DISK_PATH" | awk 'END{gsub(/%/,"",$5); print $5}')"
  avail="$(df -hP "$DISK_PATH" | awk 'END{print $4}')"
  log "disk path=$DISK_PATH use=${use_pct}% avail=$avail"
  if [[ "${use_pct:-0}" -ge "$DISK_CRIT_PCT" ]]; then
    note crit "disk ${use_pct}% >= ${DISK_CRIT_PCT}%"
  elif [[ "${use_pct:-0}" -ge "$DISK_WARN_PCT" ]]; then
    note warn "disk ${use_pct}% >= ${DISK_WARN_PCT}%"
  fi
else
  note warn "df failed for $DISK_PATH"
fi

# --- sqlite metrics ---
if [[ ! -f "$DB" ]]; then
  note crit "db missing: $DB"
else
  db_bytes="$(wc -c <"$DB" | tr -d ' ')"
  log "db path=$DB bytes=$db_bytes"
  if ! command -v sqlite3 >/dev/null 2>&1; then
    note warn "sqlite3 not installed; skip queue metrics"
  else
    # sqlite3 default separator |
    IFS='|' read -r images pending rejected files tasks_p tasks_r <<<"$(sqlite3 "$DB" \
      "SELECT
        (SELECT count(*) FROM images WHERE deleted_at IS NULL),
        (SELECT count(*) FROM images WHERE deleted_at IS NULL AND status='pending'),
        (SELECT count(*) FROM images WHERE deleted_at IS NULL AND status='rejected'),
        (SELECT count(*) FROM files),
        (SELECT count(*) FROM tasks WHERE status='pending'),
        (SELECT count(*) FROM tasks WHERE status='running');")"
    log "metrics images=$images pending=$pending rejected=$rejected files=$files tasks_pending=$tasks_p tasks_running=$tasks_r"
    if [[ "${pending:-0}" -ge "$PENDING_WARN" ]]; then
      note warn "pending images ${pending} >= ${PENDING_WARN}"
    fi
    if [[ "${tasks_p:-0}" -ge "$TASK_PENDING_WARN" ]]; then
      note warn "tasks pending ${tasks_p} >= ${TASK_PENDING_WARN}"
    fi
    if [[ "${tasks_r:-0}" -ge "$TASK_RUNNING_WARN" ]]; then
      note warn "tasks running ${tasks_r} >= ${TASK_RUNNING_WARN} (possible stuck workers)"
    fi
  fi
fi

# --- localhost HTTP ---
if command -v curl >/dev/null 2>&1; then
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 http://127.0.0.1:8686/api/v1/config || echo 000)"
  log "http config status=$code"
  if [[ "$code" != "200" && "$code" != "429" ]]; then
    # 429 is rate limit from health spam; treat as alive
    note crit "local config HTTP $code (expect 200 or 429)"
  fi
fi

case "$level" in
  0) log "health ok"; exit 0 ;;
  1) log "health warn"; exit 0 ;;
  *) log "health critical"; exit 1 ;;
esac
