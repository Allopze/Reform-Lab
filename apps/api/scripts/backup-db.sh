#!/usr/bin/env bash
# backup-db.sh — Online SQLite backup with integrity verification.
# Uses SQLite's .backup command (safe with WAL mode) to create
# timestamped copies of the production database.
#
# Usage:
#   ./scripts/backup-db.sh              # uses defaults
#   DATABASE_PATH=/data/reform.db BACKUP_DIR=/backups KEEP=14 ./scripts/backup-db.sh
#
# Environment variables:
#   DATABASE_PATH   path to the live database (default: ./data/reform.db)
#   BACKUP_DIR      directory to store backups  (default: ./data/backups)
#   KEEP            number of recent backups to retain (default: 7)

set -euo pipefail

DB="${DATABASE_PATH:-./data/reform.db}"
BACKUP_DIR="${BACKUP_DIR:-./data/backups}"
KEEP="${KEEP:-7}"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="${BACKUP_DIR}/reform-${TIMESTAMP}.db"

# ── Preflight ───────────────────────────────────────────────────────

if [ ! -f "$DB" ]; then
  echo "ERROR: database not found at $DB" >&2
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "ERROR: sqlite3 is not installed" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"

# ── Backup ──────────────────────────────────────────────────────────

echo "Backing up $DB → $DEST ..."
sqlite3 "$DB" ".backup '${DEST}'"

# ── Verify ──────────────────────────────────────────────────────────

RESULT="$(sqlite3 "$DEST" "PRAGMA integrity_check;" 2>&1)"
if [ "$RESULT" != "ok" ]; then
  echo "ERROR: integrity check failed on backup: $RESULT" >&2
  rm -f "$DEST"
  exit 2
fi

SIZE="$(stat -c%s "$DEST" 2>/dev/null || stat -f%z "$DEST" 2>/dev/null)"
echo "Backup OK — ${SIZE} bytes, integrity verified."

# ── Rotate ──────────────────────────────────────────────────────────

TOTAL="$(find "$BACKUP_DIR" -maxdepth 1 -name 'reform-*.db' -type f | wc -l)"
if [ "$TOTAL" -gt "$KEEP" ]; then
  REMOVE=$((TOTAL - KEEP))
  find "$BACKUP_DIR" -maxdepth 1 -name 'reform-*.db' -type f -printf '%T+ %p\n' \
    | sort | head -n "$REMOVE" | awk '{print $2}' \
    | xargs rm -f
  echo "Rotated: removed $REMOVE old backup(s), keeping $KEEP."
fi
