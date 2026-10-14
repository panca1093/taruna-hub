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

## Current state (slices P00–P02)

`/healthz`, `/readyz` and `GET /api/v1/tenant`.

`/readyz` is the API → PostgreSQL round trip: it pings the database and answers
`503 {status, reason}` when the database is unreachable. `GET /api/v1/tenant` publishes the
active tenant the request host resolves to, or a `404` that is identical for an inactive,
unknown, reserved, malformed or platform host. No route authenticates yet — the first identity
arrives with P03 login.

## Notifications and durable jobs (P02)

Business events are written to `notification_logs` and a River job is queued **in the caller's
transaction**, so a rolled-back business transaction leaves neither a row nor a job, and a
committed one survives an API or worker crash. P03 and later slices only call
`NotificationService.Enqueue(ctx, tx, …)`; they never see SMTP or River.

```bash
go run ./cmd/worker     # the delivery worker; one process, one queue
```

### Migrations own the River schema

`migrations/000003_river.*` is the River schema, generated with
`go run github.com/riverqueue/river/cmd/river@v0.49.0 migrate-get --all --exclude-version 1 --schema river`.
**golang-migrate is the only owner of that schema — never run River's own migrator against this
database.** Version 001 is excluded, so there is no `river_migration` table. One statement was
folded into the generated SQL: `pending` is part of the initial `CREATE TYPE` instead of a later
`ALTER TYPE … ADD VALUE`, because golang-migrate runs a migration file inside one transaction and
PostgreSQL refuses to use an enum value that the same transaction added.

### Delivery

| Concern | Behaviour |
|---|---|
| Identity | one row per (tenant, event key, recipient, channel); a repeat is suppressed, not an error |
| Queue / attempts | queue `notifications`, 6 attempts per job |
| Retry schedule | after attempt n: 30s, 2m, 10m, 1h, 6h, then 6h |
| Claim lease | 2 minutes, longer than the 30s SMTP deadline |
| Terminal states | `sent`, `failed`, `expired`, `unavailable`; all immutable and all erase the sealed payload |
| Orphans | `Reconcile` runs at worker start and every 10 minutes, and retires queued rows whose River job is gone or finished |

**Delivery is at-least-once, never exactly-once.** If the provider accepts a message and the
process dies before the status write, the retry sends a second copy. A durable enqueue guarantees
a surviving request, not a single message at an external provider.

An unconfigured channel is recorded as `unavailable`, never as delivered. WhatsApp has no provider
yet, so its adapter reports itself unconfigured; email with `SMTP_HOST` unset does the same. Only
the `notifications` queue exists today — PDF, report and import jobs will get their own queues when
their slices arrive.

### Config

| Variable | Meaning |
|---|---|
| `SMTP_HOST` | relay host; unset means email is unavailable (legal in every environment) |
| `SMTP_PORT` | relay port, default `587` |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | optional AUTH credentials |
| `SMTP_FROM` | envelope and header sender; required once `SMTP_HOST` is set |
| `SMTP_TLS` | `starttls` (default), `tls`, or `none`; `none` only when `APP_ENV=development`, and never with `SMTP_USERNAME` unless `SMTP_HOST` is loopback |
| `NOTIFICATION_SEAL_KEY` | base64 of exactly 32 bytes; required only to send secret-bearing messages |

`SMTP_TLS=starttls` refuses to fall back to cleartext: a server that does not offer STARTTLS, or
offers an untrusted certificate, fails the send instead. Secrets (OTP, reset tokens) never appear
in `params`, in River args or in logs — they are sealed with AES-256-GCM under
`NOTIFICATION_SEAL_KEY` and erased the moment the row reaches a terminal state. Logs carry fixed
class text and a masked recipient, never provider output.
