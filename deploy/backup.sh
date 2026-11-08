#!/usr/bin/env bash
# Nightly off-site backup to Cloudflare R2: a dated Postgres dump plus a sync of the asset bucket.
# The sync deletes what the app purged (payment proofs, expired exports), so it protects against
# losing the VPS, not against deleting an object by mistake.
set -euo pipefail
cd "$(dirname "$0")"
dc() { docker compose --env-file .env --env-file .image-tag "$@"; }
. ./.env

stamp=$(date +%Y%m%d-%H%M%S)
mkdir -p backups
dc exec -T postgres pg_dump -U tryout -d tryout -Fc > "backups/db-$stamp.dump"

dc run --rm backup copyto "/backups/db-$stamp.dump" "r2:$R2_BUCKET/db/db-$stamp.dump"
dc run --rm backup sync storage:tryout-assets "r2:$R2_BUCKET/assets"

find backups -name 'db-*.dump' -mtime +7 -delete
echo "backup $stamp ok"
