#!/usr/bin/env bash
# Online SQLite backup + uploads archive. Keep 14 days by default.
set -euo pipefail

DB="${PIONUS_DB:-data/pionus.db}"
UPLOADS="${PIONUS_UPLOADS:-uploads}"
DEST="${BACKUP_DIR:-backups}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
STAMP="$(date +%Y%m%d-%H%M%S)"

if [[ ! -f "$DB" ]]; then
  echo "database not found: $DB" >&2
  exit 1
fi
if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required for a consistent backup" >&2
  exit 1
fi

mkdir -p "$DEST"
sqlite3 "$DB" ".backup '$DEST/pionus-$STAMP.db'"
if [[ -d "$UPLOADS" ]]; then
  tar -C "$UPLOADS" -czf "$DEST/uploads-$STAMP.tgz" .
fi
chmod 600 "$DEST"/*-"$STAMP".* 2>/dev/null || true
find "$DEST" -type f -mtime +"$KEEP_DAYS" -delete

echo "backup written to $DEST"
