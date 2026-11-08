# Production deployment

One Hostinger VPS (KVM 2, 8 GB) runs the whole stack from `deploy/compose.yml`. Cloudflare sits in
front for DNS, public TLS and caching. GitHub Actions only builds images into GHCR; the owner deploys by
running a script on the VPS.

```
browser ──https──▶ Cloudflare ──https + client cert──▶ Caddy :443 ─┬─ tarunaedu.com/api/*  → api:8081
                                                                    ├─ tarunaedu.com/*      → web:3000
                                                                    ├─ www.tarunaedu.com    → redirect
                                                                    └─ files.tarunaedu.com  → rustfs:9000
postgres, redis, rustfs, gotenberg, worker: compose network only, no published ports
```

The repository is public: never commit the VPS IP, SSH user or app path. GitHub holds no server
credentials; this page writes them as `<ssh-user>` and `<app-dir>`.

| File | Role |
|---|---|
| `deploy/compose.yml` | Production services. No seed: accounts come from `bootstrap` |
| `deploy/Caddyfile` | Reverse proxy, Cloudflare origin cert, Authenticated Origin Pulls |
| `deploy/.env.example` | Template for `<app-dir>/.env` on the VPS |
| `deploy/backup.sh` | Nightly Postgres dump + asset mirror to Cloudflare R2 |
| `deploy/deploy.sh` | Run on the VPS: pull images, replace stack files, migrate, restart, prune |
| `.github/workflows/images.yml` | Build and push GHCR images on manual run or `v*` tag; never touches the VPS |

The API reaches RustFS over plain HTTP inside the network, but presigned links must be
`https://files.tarunaedu.com`, so `S3_PUBLIC_USE_SSL=true` sets the scheme of those links on its own.

## 1. Cloudflare (once)

1. Add the site `tarunaedu.com` (Free plan), then set the domain's nameservers at the registrar to the two
   Cloudflare gives you. Wait until the zone shows **Active**.
2. **DNS**: `A` records for `@`, `www` and `files` pointing to the VPS IP, all **Proxied** (orange cloud).
3. **SSL/TLS → Overview**: mode **Full (strict)**. **Edge Certificates**: Always Use HTTPS on, minimum TLS 1.2.
4. **SSL/TLS → Origin Server → Create Certificate**: RSA, hostnames `tarunaedu.com` and `*.tarunaedu.com`,
   15 years. Save the certificate as `origin.pem` and the key as `origin-key.pem`; the key is shown only once.
5. Same page: turn **Authenticated Origin Pulls** on. Download Cloudflare's origin-pull CA from
   `https://developers.cloudflare.com/ssl/static/authenticated_origin_pull_ca.pem` and save it as
   `cloudflare-origin-pull-ca.pem`. Without this CA Caddy refuses every request, Cloudflare's included.
6. **Caching → Cache Rules**: one rule, *Bypass cache* when the URI path starts with `/api/` **or** the
   hostname is `files.tarunaedu.com`. Static Next.js assets keep Cloudflare's default caching.
7. Leave **Bot Fight Mode** off: on the Free plan it cannot exempt the Midtrans webhook.

## 2. VPS (once)

Ubuntu 24.04 on the KVM 2.

```bash
adduser <ssh-user> && usermod -aG sudo <ssh-user>   # then copy your SSH key to ~<ssh-user>/.ssh/authorized_keys
curl -fsSL https://get.docker.com | sh && usermod -aG docker <ssh-user>
ufw allow 22/tcp && ufw allow 443/tcp && ufw enable
sudo mkdir -p <app-dir>/certs <app-dir>/backups && sudo chown -R <ssh-user>: <app-dir>
```

Turn off SSH password login (`PasswordAuthentication no` in `/etc/ssh/sshd_config`). Docker publishes
ports past `ufw`, so also add the same 22/443 rules in **hPanel → VPS → Firewall**. Port 443 is still safe
when reached directly: without Cloudflare's client certificate the TLS handshake fails.

Put the three files from Cloudflare in `<app-dir>/certs/`, then:

```bash
cp deploy/.env.example <app-dir>/.env   # copy from the repo, fill every empty value
chmod 600 <app-dir>/.env <app-dir>/certs/origin-key.pem
```

## 3. Release and deploy

Nothing pushes to the VPS. Deploying is `./deploy.sh <sha>` run on the server by the owner.

**Bootstrap (once)**, in `<app-dir>` after §2:

```bash
curl -fsSLo deploy.sh https://raw.githubusercontent.com/panca1093/taruna-hub/main/deploy/deploy.sh && chmod +x deploy.sh
```

**GHCR access.** If the `taruna-hub-api` and `taruna-hub-web` packages are public, nothing is needed. If
private, run `docker login ghcr.io` once on the VPS with a classic personal access token scoped
`read:packages` only.

**Release.** Run **Actions → Images → Run workflow** on `main` after CI is green, or push a `v*` tag. Both
push `taruna-hub-api` and `taruna-hub-web` tagged with the full commit SHA; a `v*` tag adds the tag name too.
Get the SHA from the run page or `git rev-parse origin/main`.

**Deploy.**

```bash
ssh <ssh-user>@<vps>
cd <app-dir> && ./deploy.sh <40-char sha>
```

The script downloads the repo at that SHA and pulls its images first, so a SHA without images fails with
the running stack untouched. Then it replaces `compose.yml`, `Caddyfile`, `backup.sh`, `deploy.sh` and
`migrations/`, writes `.image-tag`, restarts, waits for `/readyz`, and keeps the last 3 SHA-tagged images.
It prints the previous SHA as the rollback value.

**Rollback.** `./deploy.sh <previous sha>`. Migrations are not rolled back; they run while the old
api/worker still serve, so each must stay backward-compatible with the previous release.

Manual commands on the VPS always pass both env files:

```bash
cd <app-dir> && alias dc='docker compose --env-file .env --env-file .image-tag'
dc ps
dc logs -f api worker
```

### First deploy only: tenant and admin

There is no seed in production; `bootstrap` is the only way to create the tenant and the first admin.

```bash
dc run --rm api bootstrap tenant -slug taruna -name "Taruna Eduka"
dc run --rm -e BOOTSTRAP_PASSWORD='…' api bootstrap admin -tenant taruna -name "…" -email "…"
```

## 4. Backups

Create an R2 bucket `taruna-backup` and an R2 API token with Object Read & Write on it only, then fill the
`R2_*` values in `.env`. Add a lifecycle rule on the bucket deleting `db/` objects after 30 days.

```bash
crontab -e   # as <ssh-user>:
15 2 * * * <app-dir>/backup.sh >> <app-dir>/backups/backup.log 2>&1
```

Each run uploads `db/db-<stamp>.dump` and mirrors the asset bucket to `assets/`. The mirror deletes what
the app purged, so it covers losing the VPS, not deleting an object by mistake. Local dumps are kept 7 days.

Restore, after a fresh stack is up:

```bash
dc exec -T postgres pg_restore -U tryout -d tryout --clean --if-exists < backups/db-<stamp>.dump
dc run --rm backup sync r2:<r2-bucket>/assets storage:tryout-assets   # <r2-bucket> = R2_BUCKET in .env
```

Try a restore once before going live; an untested backup is not one.

## Later

- **Email (Brevo)**: verify `tarunaedu.com` in Brevo, add its SPF/DKIM/DMARC records in Cloudflare as
  **DNS only** (grey cloud), fill `SMTP_*` in `.env`, then `dc up -d api worker`. Until then signup codes
  and password resets are unavailable.
- **Midtrans**: set the notification URL to `https://tarunaedu.com/api/v1/payments/midtrans/notification`.

## Limits to remember

- Cloudflare Free rejects request bodies over **100 MB** and cuts responses after **100 s**. Caddy uses the
  same 100 MB cap so the two agree.
- RustFS is pinned at 1.0.4 (a young project; bump it deliberately and test uploads first). Storage is
  S3-compatible, so moving to R2 later is an env change (`S3_ENDPOINT`, keys, bucket) plus copying the objects.
- Everything lives on one disk; the R2 backups are the only copy elsewhere.
