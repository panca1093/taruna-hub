# Production deployment

One Hostinger VPS (KVM 2, 8 GB) runs the whole stack from `deploy/compose.yml`. Cloudflare sits in
front for DNS, public TLS and caching. Images are built by GitHub Actions and pulled from GHCR.

```
browser ──https──▶ Cloudflare ──https + client cert──▶ Caddy :443 ─┬─ tarunaedu.com/api/*  → api:8081
                                                                    ├─ tarunaedu.com/*      → web:3000
                                                                    ├─ www.tarunaedu.com    → redirect
                                                                    └─ files.tarunaedu.com  → rustfs:9000
postgres, redis, rustfs, gotenberg, worker: compose network only, no published ports
```

| File | Role |
|---|---|
| `deploy/compose.yml` | Production services. No seed: accounts come from `bootstrap` |
| `deploy/Caddyfile` | Reverse proxy, Cloudflare origin cert, Authenticated Origin Pulls |
| `deploy/.env.example` | Template for `/opt/taruna/.env` on the VPS |
| `deploy/backup.sh` | Nightly Postgres dump + asset mirror to Cloudflare R2 |
| `.github/workflows/deploy.yml` | Manual deploy: build → push GHCR → copy stack files → restart |

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
adduser deploy && usermod -aG sudo deploy        # then copy your SSH key to ~deploy/.ssh/authorized_keys
curl -fsSL https://get.docker.com | sh && usermod -aG docker deploy
ufw allow 22/tcp && ufw allow 443/tcp && ufw enable
sudo mkdir -p /opt/taruna/certs && sudo chown -R deploy: /opt/taruna
```

Turn off SSH password login (`PasswordAuthentication no` in `/etc/ssh/sshd_config`). Docker publishes
ports past `ufw`, so also add the same 22/443 rules in **hPanel → VPS → Firewall**. Port 443 is still safe
when reached directly: without Cloudflare's client certificate the TLS handshake fails.

Put the three files from Cloudflare in `/opt/taruna/certs/`, then:

```bash
cp deploy/.env.example /opt/taruna/.env   # copy from the repo, fill every empty value
chmod 600 /opt/taruna/.env /opt/taruna/certs/origin-key.pem
```

## 3. GitHub (once)

Repository **Settings → Environments → New environment** `production`, with secrets:

| Secret | Value |
|---|---|
| `VPS_HOST` | VPS IP |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | A private key made only for deploys; its public half in `~deploy/.ssh/authorized_keys` |

The workflow logs the VPS in to GHCR with the run's own short-lived token, so no personal access token is
stored on the server.

## 4. Deploy

**Actions → Deploy → Run workflow** on `main`, after CI is green. It builds `taruna-hub-api` and
`taruna-hub-web` tagged with the commit SHA, copies `compose.yml`, `Caddyfile`, `backup.sh` and
`migrations/` to `/opt/taruna`, runs migrations, restarts and waits for `/readyz`. The deployed tag is
in `/opt/taruna/.image-tag`; to roll back, run the workflow on the older commit.

Manual commands on the VPS always pass both env files:

```bash
cd /opt/taruna && alias dc='docker compose --env-file .env --env-file .image-tag'
dc ps
dc logs -f api worker
```

### First deploy only: tenant and admin

There is no seed in production; `bootstrap` is the only way to create the tenant and the first admin.

```bash
dc run --rm api bootstrap tenant -slug taruna -name "Taruna Eduka"
dc run --rm -e BOOTSTRAP_PASSWORD='…' api bootstrap admin -tenant taruna -name "…" -email "…"
```

## 5. Backups

Create an R2 bucket `taruna-backup` and an R2 API token with Object Read & Write on it only, then fill the
`R2_*` values in `.env`. Add a lifecycle rule on the bucket deleting `db/` objects after 30 days.

```bash
crontab -e   # as deploy:
15 2 * * * /opt/taruna/backup.sh >> /opt/taruna/backups/backup.log 2>&1
```

Each run uploads `db/db-<stamp>.dump` and mirrors the asset bucket to `assets/`. The mirror deletes what
the app purged, so it covers losing the VPS, not deleting an object by mistake. Local dumps are kept 7 days.

Restore, after a fresh stack is up:

```bash
dc exec -T postgres pg_restore -U tryout -d tryout --clean --if-exists < backups/db-<stamp>.dump
dc run --rm backup sync r2:$R2_BUCKET/assets storage:tryout-assets   # assets; R2_BUCKET from .env
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
