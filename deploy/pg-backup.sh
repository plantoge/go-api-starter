#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/backups/app-db"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
FILE="$BACKUP_DIR/appdb-$TIMESTAMP.dump"

pg_dump -Fc -h "${DB_HOST:-localhost}" -U "${DB_USER:-app}" "${DB_NAME:-appdb}" > "$FILE"

# Keep 35 days locally; the real retention policy lives wherever this gets
# copied to off-server.
find "$BACKUP_DIR" -name "appdb-*.dump" -mtime +35 -delete

echo "backup written: $FILE"
echo "REMINDER: this backup is only useful if it is copied off this server"
echo "          (rsync/rclone to remote storage) and restore has been tested."
