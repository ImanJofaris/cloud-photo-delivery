#!/usr/bin/env bash
# Restore a Cloud Photo Delivery backup produced by scripts/backup.sh.
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/db ./scripts/restore.sh backups/cpd-20260916T000000Z.dump
#
# The restore is destructive: it drops and recreates the objects contained in
# the dump (--clean --if-exists). Stop the API and worker first.
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${1:?usage: restore.sh <backup-file>}"
file="$1"

if [ ! -f "$file" ]; then
	echo "backup not found: $file" >&2
	exit 1
fi

pg_restore --clean --if-exists --no-owner --no-acl --dbname "$DATABASE_URL" "$file"

echo "restore complete: $file"
echo "run migrations next: goose -dir migrations postgres \"\$DATABASE_URL\" up"
