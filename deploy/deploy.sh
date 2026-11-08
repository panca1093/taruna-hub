#!/usr/bin/env bash
# Run on the VPS from <app-dir>: ./deploy.sh <version tag, e.g. v1.0.0> (docs/deployment.md).
set -euo pipefail
tag=${1:-}
[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || { echo "usage: ./deploy.sh <version tag, e.g. v1.0.0>" >&2; exit 2; }
dc() { docker compose --env-file .env --env-file .image-tag "$@"; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://codeload.github.com/panca1093/taruna-hub/tar.gz/refs/tags/$tag" | tar -xz -C "$tmp" --strip-components=1

# Pull before touching live files: a tag without images fails here and leaves the running stack as it is.
IMAGE_TAG=$tag docker compose -f "$tmp/deploy/compose.yml" --project-directory . --env-file .env pull --quiet

prev=none
if [ -f .image-tag ]; then prev=$(sed -n 's/^IMAGE_TAG=//p' .image-tag); fi
echo "rollback value: ./deploy.sh $prev"

# mv gives each file a new inode, so replacing deploy.sh while it runs is safe.
mv "$tmp/deploy/compose.yml" "$tmp/deploy/Caddyfile" "$tmp/deploy/backup.sh" "$tmp/deploy/deploy.sh" .
chmod +x backup.sh deploy.sh
rm -rf migrations
mv "$tmp/api/migrations" migrations
echo "IMAGE_TAG=$tag" > .image-tag

if ! dc up -d --remove-orphans; then dc logs --tail 100 api migrate; exit 1; fi
for i in $(seq 1 60); do
  dc exec -T api wget -qO- http://localhost:8081/readyz >/dev/null 2>&1 && break
  [ "$i" = 60 ] && { dc logs --tail 100 api migrate; exit 1; }
  sleep 2
done

# Keeps the current and the 2 newest other versions; SHA-tagged images are never deployable, so they go.
for repo in ghcr.io/panca1093/taruna-hub-api ghcr.io/panca1093/taruna-hub-web; do
  tags=$(docker image ls "$repo" --format '{{.Tag}}')
  {
    printf '%s\n' "$tags" | { grep -E '^[0-9a-f]{40}$' || true; }
    printf '%s\n' "$tags" | { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+' || true; } | { grep -vx "$tag" || true; } | sort -rV | tail -n +3
  } | xargs -r -I{} docker image rm "$repo:{}"
done
docker image prune -f

# A version tag can be moved; the digest is the record of what actually runs.
docker image inspect --format '{{index .RepoDigests 0}}' "ghcr.io/panca1093/taruna-hub-api:$tag"
echo "deployed $tag (previous: $prev)"
