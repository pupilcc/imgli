#!/usr/bin/env bash
# VIP img.li — lightweight health snapshot (disk / SQLite ops queues / process / memory).
# Exit 0 = ok or warn; exit 1 = critical (disk full, baili down, huge backlog, hung HTTP).
# Cron example:
#   */5 * * * * UNIT=baili /opt/baili/bin/health-check.sh >/dev/null 2>&1
# Auto-heal hung process (optional, after 2 consecutive criticals):
#   AUTO_RESTART=1 RESTART_UNIT=baili /opt/baili/bin/health-check.sh
set -euo pipefail

DATA_DIR="${DATA_DIR:-/opt/baili/data}"
DB="${DB:-$DATA_DIR/baili.db}"
BIN="${BIN:-/opt/baili/bin/imgli}"
# legacy binary name on img.li
if [[ ! -x "$BIN" && -x /opt/baili/bin/baili ]]; then
  BIN=/opt/baili/bin/baili
fi
DISK_PATH="${DISK_PATH:-/opt}"
UNIT="${UNIT:-baili}"
# thresholds
DISK_WARN_PCT="${DISK_WARN_PCT:-85}"
DISK_CRIT_PCT="${DISK_CRIT_PCT:-95}"
PENDING_WARN="${PENDING_WARN:-50}"
TASK_PENDING_WARN="${TASK_PENDING_WARN:-100}"
TASK_RUNNING_WARN="${TASK_RUNNING_WARN:-20}"
# memory: MemoryCurrent as fraction of MemoryHigh (systemd cgroup)
MEM_WARN_PCT="${MEM_WARN_PCT:-80}"
MEM_CRIT_PCT="${MEM_CRIT_PCT:-95}"
HTTP_TIMEOUT="${HTTP_TIMEOUT:-3}"
AUTO_RESTART="${AUTO_RESTART:-0}"
RESTART_UNIT="${RESTART_UNIT:-$UNIT}"
STATE_DIR="${STATE_DIR:-$DATA_DIR}"
CRIT_STREAK_FILE="${CRIT_STREAK_FILE:-$STATE_DIR/health-crit-streak}"

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
if pgrep -f "$BIN serve" >/dev/null 2>&1 || pgrep -f "/opt/baili/bin/imgli serve" >/dev/null 2>&1; then
  log "ok: imgli/baili process up"
else
  note crit "imgli process not running (pattern: $BIN serve)"
fi

# --- systemd memory (if unit present) ---
if command -v systemctl >/dev/null 2>&1 && systemctl cat "$UNIT" >/dev/null 2>&1; then
  cur="$(systemctl show "$UNIT" -p MemoryCurrent --value 2>/dev/null || echo 0)"
  high="$(systemctl show "$UNIT" -p MemoryHigh --value 2>/dev/null || echo infinity)"
  peak="$(systemctl show "$UNIT" -p MemoryPeak --value 2>/dev/null || echo 0)"
  log "memory unit=$UNIT current=${cur} peak=${peak} high=${high}"
  if [[ "$cur" =~ ^[0-9]+$ && "$high" =~ ^[0-9]+$ && "$high" -gt 0 ]]; then
    pct=$(( cur * 100 / high ))
    if [[ "$pct" -ge "$MEM_CRIT_PCT" ]]; then
      note crit "memory ${pct}% of MemoryHigh (cur=$cur high=$high)"
    elif [[ "$pct" -ge "$MEM_WARN_PCT" ]]; then
      note warn "memory ${pct}% of MemoryHigh (cur=$cur high=$high)"
    fi
  fi
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

# --- localhost HTTP (healthz preferred; config as fallback) ---
if command -v curl >/dev/null 2>&1; then
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$HTTP_TIMEOUT" http://127.0.0.1:8686/healthz || echo 000)"
  log "http healthz status=$code"
  if [[ "$code" != "200" ]]; then
    code2="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$HTTP_TIMEOUT" http://127.0.0.1:8686/api/v1/config || echo 000)"
    log "http config status=$code2"
    if [[ "$code2" != "200" && "$code2" != "429" ]]; then
      note crit "local HTTP healthz=$code config=$code2 (process hung or down)"
    fi
  fi
fi

case "$level" in
  0)
    rm -f "$CRIT_STREAK_FILE" 2>/dev/null || true
    log "health ok"; exit 0
    ;;
  1)
    rm -f "$CRIT_STREAK_FILE" 2>/dev/null || true
    log "health warn"; exit 0
    ;;
  *)
    streak=0
    if [[ -f "$CRIT_STREAK_FILE" ]]; then
      streak="$(cat "$CRIT_STREAK_FILE" 2>/dev/null || echo 0)"
    fi
    streak=$((streak + 1))
    echo "$streak" >"$CRIT_STREAK_FILE"
    log "health critical streak=$streak"
    if [[ "$AUTO_RESTART" == "1" && "$streak" -ge 2 ]]; then
      log "auto-restart: systemctl restart $RESTART_UNIT (streak=$streak)"
      if systemctl restart "$RESTART_UNIT"; then
        echo 0 >"$CRIT_STREAK_FILE"
        log "auto-restart: ok"
      else
        log "auto-restart: failed"
      fi
    fi
    exit 1
    ;;
esac
