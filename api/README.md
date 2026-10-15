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

## Current state (slices P00–P03a)

`/healthz`, `/readyz`, `GET /api/v1/tenant`, the auth routes and `GET /api/v1/me`.

`/readyz` is the API → PostgreSQL round trip: it pings the database and answers
`503 {status, reason}` when the database is unreachable. `GET /api/v1/tenant` publishes the
active tenant the request host resolves to, or a `404` that is identical for an inactive,
unknown, reserved, malformed or platform host. `GET /api/v1/me` is the first route that
authenticates; see [Authentication and sessions](#authentication-and-sessions-p03a).

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
| `NOTIFICATION_SEAL_KEY` | base64 of exactly 32 bytes; **required in every environment** since P03b, because signup codes and reset links are sealed |

`SMTP_TLS=starttls` refuses to fall back to cleartext: a server that does not offer STARTTLS, or
offers an untrusted certificate, fails the send instead. Secrets (OTP, reset tokens) never appear
in `params`, in River args or in logs — they are sealed with AES-256-GCM under
`NOTIFICATION_SEAL_KEY` and erased the moment the row reaches a terminal state. Logs carry fixed
class text and a masked recipient, never provider output.

## Authentication and sessions (P03a, P03b)

`POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout` and
`GET /api/v1/me` (P03a), plus registration, the email code and password reset (P03b). This is the
API half of P03; the web screens arrive with P03c.

### What a request proves

Every `/api/v1/*` request resolves a tenant from its host first, and the token has to agree with it.
`login` reads the identifier only inside that tenant, so the same email in two tenants is two
different people. `Authenticate` then rebuilds the caller from the access token **plus** the session
row and the user row, so a revoked session, a changed role or a deactivated account invalidates an
old token on the very next request. The token's own claims are never trusted for the role or the
tenant on their own.

| Concern | Value |
|---|---|
| Access token | HS256 JWT, 15 minutes, issuer `taruna-api`, audience `taruna-web` |
| Refresh token | 32 random bytes, base64url, stored only as its SHA-256; rotated on every refresh |
| Refresh cookie | `refresh_token`; `Path=/api/v1/auth`; `HttpOnly`; `SameSite=Lax`; `Secure` outside development; `Max-Age` 30 days |
| Idle lifetime | 3 hours (D13), enforced server-side on every authenticated request |

Two rules are easy to get wrong, and both are deliberate:

- **Refreshing is not activity.** It issues a new access token but does not move `last_activity_at`,
  so reloading a page cannot keep a session alive past the idle deadline. Only an authenticated
  request extends it, and at most once a minute per session.
- **`refresh` and `logout` require an `Origin`** that is both accepted by the CORS policy and on the
  host this request resolved to. The CORS middleware still runs the handler for a rejected origin, so
  those two routes — the ones that act on a cookie alone — check it themselves. The dev case of web
  on `:3000` calling the API on `:8081` under the same hostname stays allowed.

**Every refresh rotates the token.** The replaced token keeps working for 30 seconds, so two tabs
that refresh at once both get an access token; the one that lost the race sets no cookie, because the
winner already did. After those 30 seconds a replaced token can only be a copy, so presenting it
revokes the whole session and the current token dies with it.

Logout always answers `204` and always clears the cookie, and revocation takes effect immediately.
Login failures are indistinguishable: an unknown identifier, an inactive account and a wrong password
share one message, and an unknown account still pays for a bcrypt comparison so timing does not
single it out either.

### Identifiers

Emails are stored trimmed and lowercased, phones as E.164 (`0812…`, `62812…` and `+62 812-…` all
become `+62812…`), and check constraints on `users` refuse anything else. Login, registration and
recovery normalize the input the same way, so every spelling reaches the same account.

### Registration and recovery (P03b, D01)

| Route | Answer |
|---|---|
| `POST /auth/register` | `202` and a 6-digit code by email; nothing is an account yet |
| `POST /auth/register/resend` | `202` and a fresh code; the same `202` and no email for an unknown address |
| `POST /auth/register/verify` | `200`, a token body and the cookie — the account exists from here |
| `POST /auth/password/forgot` | `202` and no body for every well-formed email |
| `POST /auth/password/reset` | `204`; the password changes and every session of the account is revoked |

D01 is settled: the code goes by email over the P02 SMTP transport, email is required at signup and a
phone is optional, so recovery is always an email link. Registration exists only on tenant hosts.

A pending signup lives in `auth_registrations`, not in `users`, so an unverified address never
blocks anyone: signing up again replaces it. The code is stored as an HMAC keyed from `JWT_SECRET`,
works for 10 minutes and 5 guesses, and the guesses are counted under a row lock, so a burst of
concurrent requests buys no extra tries. Two racing verifications of the right code create one
account.

A reset link is `<scheme>://<slug>.<BASE_DOMAIN>[:port]/reset-password#token=…`, built from
configuration only — never from a request header, or a forged `Origin` could put an attacker's host
into a genuine email. The token sits in the fragment so it never reaches a server log or a `Referer`.
It is single use (one conditional `UPDATE`), lasts 30 minutes, and a newer request retires older links.

Codes and links travel only as sealed notification params with an expiry equal to their own, so a
message that would arrive dead is expired instead of sent.

### Throttles

Fixed-window counters in `auth_throttles`, keyed by a SHA-256 of rule, host scope and normalized
identifier. Attempts are counted before any password check in one atomic statement, so concurrent
requests cannot exceed the budget. A refusal is `429 too_many_requests` with `Retry-After`.

| Rule | Budget |
|---|---|
| Login, per identifier and host | 10 attempts / 15 minutes; a success or a password reset clears it |
| Signup codes, per email and tenant | 5 / hour across register and resend, and 60 seconds between resends |
| Forgot password, per email and host | 3 / 15 minutes, counted for unknown emails too |

A login lockout keyed on the identifier lets someone who knows an address lock its owner out for 15
minutes; the reset link still works and clears it.

### Not here yet

No capability matrix — the guard is by role only (D02) — no absolute session cap, nothing per-IP
(that needs a trusted-proxy client-IP decision), and no sweep of expired pending signups, used reset
tokens or stale throttle rows: they are small and harmless, and a maintenance job can arrive with the
next worker change. `GET /api/v1/me` carries identity only: balance and the ledger are not part of it.

### Config

| Variable | Meaning |
|---|---|
| `JWT_SECRET` | Base64 of at least 32 bytes; **required in every environment**. `openssl rand -base64 32` |
| `NOTIFICATION_SEAL_KEY` | Base64 of exactly 32 bytes; **required in every environment** (see Notifications) |
| `PUBLIC_WEB_SCHEME` | Scheme of links in emails; default `https`, and `http` is allowed only in development (its default there) |
| `PUBLIC_WEB_PORT` | Port of links in emails; default `3000` in development, none elsewhere |
