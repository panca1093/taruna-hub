#!/usr/bin/env bash
# Run on the VPS from <app-dir>: ./deploy.sh <40-char commit sha> (docs/deployment.md).
set -euo pipefail
sha=${1:-}
[[ $sha =~ ^[0-9a-f]{40}$ ]] || { echo "usage: ./deploy.sh <40-char commit sha>" >&2; exit 2; }
dc() { docker compose --env-file .env --env-file .image-tag "$@"; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://codeload.github.com/panca1093/taruna-hub/tar.gz/$sha" | tar -xz -C "$tmp" --strip-components=1

# Pull before touching live files: a missing image fails here and leaves the running stack as it is.
IMAGE_TAG=$sha docker compose -f "$tmp/deploy/compose.yml" --project-directory . --env-file .env pull --quiet

prev=none
if [ -f .image-tag ]; then prev=$(sed -n 's/^IMAGE_TAG=//p' .image-tag); fi
echo "rollback value: ./deploy.sh $prev"

# mv gives each file a new inode, so replacing deploy.sh while it runs is safe.
mv "$tmp/deploy/compose.yml" "$tmp/deploy/Caddyfile" "$tmp/deploy/backup.sh" "$tmp/deploy/deploy.sh" .
chmod +x backup.sh deploy.sh
rm -rf migrations
mv "$tmp/api/migrations" migrations
echo "IMAGE_TAG=$sha" > .image-tag

if ! dc up -d --remove-orphans; then dc logs --tail 100 api migrate; exit 1; fi
for i in $(seq 1 60); do
  dc exec -T api wget -qO- http://localhost:8081/readyz >/dev/null 2>&1 && break
  [ "$i" = 60 ] && { dc logs --tail 100 api migrate; exit 1; }
  sleep 2
done

for repo in ghcr.io/panca1093/taruna-hub-api ghcr.io/panca1093/taruna-hub-web; do
  docker image ls "$repo" --format '{{.Tag}}' | { grep -E '^[0-9a-f]{40}$' || true; } | { grep -vx "$sha" || true; } | tail -n +3 \
    | xargs -r -I{} docker image rm "$repo:{}"
done
docker image prune -f

echo "deployed $sha (previous: $prev)"
