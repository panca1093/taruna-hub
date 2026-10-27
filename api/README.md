# API — Taruna Eduka Tryout Platform

Go backend. Layering follows PRD §7: `model → repository → service → handler`, with
`infra` for datastore wiring and `httpx` for HTTP concerns.

This file is how to run it. How it behaves is in [`docs/`](../docs/index.md) at the repo root.

## Run it

```bash
make reset   # containers + schema + dev fixtures
make run     # server on :8081
```

`make reset` is idempotent — safe to re-run whenever local state drifts.

## Ports

Every default was already taken on the original dev machine, so these are shifted.
Change them in `docker-compose.yml` and `.env` if they clash on yours.

| Service | Host port | Note |
|---|---|---|
| API | 8081 | 8080 is OrbStack |
| Postgres | 5433 | |
| Redis | 6380 | |
| MinIO | 9002 (console 9003) | |

## The API contract

`openapi.yaml` is the source of truth. The frontend's typed client is generated from it:

```bash
cd ../web && npm run gen:api
```

CI fails if the committed client is out of date, so a contract change cannot land
without the frontend types moving with it.

## Migrations

```bash
make migrate-up
make migrate-down                    # rolls back one step
make migrate-create NAME=add_orders  # new pair of .sql files
```

Fixtures live in `seed/dev.sql` and are deliberately **not** a migration — production
must never run them.

## Worker

```bash
go run ./cmd/worker     # delivers queued notifications (email); one process, one queue
```

## Local tenant

`TENANT_SLUG` names the one tenant the API serves; `.env.example` sets the seeded `taruna`. After
`make reset`, run the API with it and use plain `localhost`:

```bash
TENANT_SLUG=taruna make run
curl http://localhost:8081/api/v1/tenant     # {"slug":"taruna","name":"Taruna Eduka"}
```

Leave `NEXT_PUBLIC_API_URL` unset locally: the client then calls the API on the page's own hostname,
so `http://localhost:3000` talks to `http://localhost:8081`. `sekolah-b` is a second seed tenant that
only the repository tests use.

## Bootstrap

The first tenant and admins are created from the command line, never over HTTP. The password comes
from the environment so it stays out of shell history:

```bash
go run ./cmd/bootstrap tenant -slug taruna -name "Taruna Eduka"
BOOTSTRAP_PASSWORD=... go run ./cmd/bootstrap admin -tenant taruna -name "Siti" -email siti@example.com
BOOTSTRAP_PASSWORD=... go run ./cmd/bootstrap admin -platform -name "Root" -email root@example.com
```

## Configuration

`.env.example` is the template. Generate secrets with `openssl rand -base64 32`.

| Variable | Required | Meaning |
|---|---|---|
| `DATABASE_URL` | always | PostgreSQL |
| `REDIS_URL` | outside development | Redis; development defaults to `redis://localhost:6380/0` |
| `JWT_SECRET` | always | Base64, at least 32 bytes |
| `NOTIFICATION_SEAL_KEY` | always | Base64, exactly 32 bytes; replacing it voids undelivered secret messages |
| `TENANT_SLUG` | always | The one tenant this deployment serves; `bootstrap` needs it set too |
| `CORS_ORIGINS` | no | Comma-separated; default `http://localhost:3000` |
| `APP_URL` | outside development | Web origin that links in emails point at, `https` outside development; development defaults to `http://localhost:3000` |
| `SMTP_HOST` | no | Unset means email is unavailable |
| `SMTP_PORT` | no | Default `587` |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | no | Optional AUTH credentials |
| `SMTP_FROM` | with `SMTP_HOST` | Sender address |
| `SMTP_TLS` | no | `starttls` (default), `tls`, or `none` (development only, and with credentials only against a loopback host) |
| `S3_ENDPOINT` | no | S3-compatible host:port (MinIO, or `storage.googleapis.com` with HMAC keys). Unset means the asset routes answer `503` |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` | with `S3_ENDPOINT` | Credentials and the private bucket |
| `S3_USE_SSL` | no | `true` (default) outside development, `false` in it |
| `S3_REGION` | no | Only when the server needs one |
