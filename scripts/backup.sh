#!/usr/bin/env bash
# Backup the Cloud Photo Delivery database with pg_dump (custom format).
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/db ./scripts/backup.sh
#
# Environment:
#   DATABASE_URL             required, connection string
#   BACKUP_DIR               output directory (default: ./backups)
#   BACKUP_RETENTION_DAYS    delete local dumps older than N days (default: 14)
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$BACKUP_DIR/cpd-$stamp.dump"

pg_dump --format=custom --no-owner --no-acl --file="$file" "$DATABASE_URL"

# A backup is only a backup if it is readable.
pg_restore --list "$file" >/dev/null

find "$BACKUP_DIR" -type f -name 'cpd-*.dump' -mtime "+${RETENTION_DAYS}" -delete

echo "backup written: $file"
