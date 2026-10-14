# API — Taruna Eduka Tryout Platform

Go backend. Layering follows PRD §7: `model → repository → service → handler`, with
`infra` for datastore wiring and `httpx` for HTTP concerns.

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

## Tenancy (P01)

The tenant comes from the request host, never from the client: `<slug>.<BASE_DOMAIN>` selects a
tenant, `admin.<BASE_DOMAIN>` selects the platform, and every other host answers `404`.
`X-Forwarded-Host` is ignored unless `TRUST_FORWARDED_HOST=true`, which is only safe behind a
proxy that strips the client's copy. `BASE_DOMAIN` is required outside development.

In development `BASE_DOMAIN` defaults to `localhost` and browsers resolve `*.localhost` to
loopback, so after `make reset`:

```bash
curl http://taruna.localhost:8081/api/v1/tenant     # {"slug":"taruna","name":"Taruna Eduka"}
curl http://sekolah-b.localhost:8081/api/v1/tenant  # the other seed tenant, on its own host
```

Leave `NEXT_PUBLIC_API_URL` unset locally: the client then calls the API on the page's own host,
so `http://taruna.localhost:3000` talks to `http://taruna.localhost:8081`.

The first tenant and admins are created with the bootstrap command; there is no HTTP endpoint
for it. The password comes from the environment so it stays out of shell history:

```bash
go run ./cmd/bootstrap tenant -slug taruna -name "Taruna Eduka"
BOOTSTRAP_PASSWORD=... go run ./cmd/bootstrap admin -tenant taruna -name "Siti" -email siti@example.com
BOOTSTRAP_PASSWORD=... go run ./cmd/bootstrap admin -platform -name "Root" -email root@example.com
```

## Current state (slices P00–P01)

`/healthz`, `/readyz` and `GET /api/v1/tenant`.

`/readyz` is the API → PostgreSQL round trip: it pings the database and answers
`503 {status, reason}` when the database is unreachable. `GET /api/v1/tenant` publishes the
active tenant the request host resolves to, or a `404` that is identical for an inactive,
unknown, reserved, malformed or platform host. No route authenticates yet — the first identity
arrives with P03 login.
