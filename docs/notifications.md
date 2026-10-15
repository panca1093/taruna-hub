# Notifications and durable jobs (P02)

Business events are written to `notification_logs` and a River job is queued **in the caller's
transaction**, so a rolled-back business transaction leaves neither a row nor a job, and a
committed one survives an API or worker crash. P03 and later slices only call
`NotificationService.Enqueue(ctx, tx, …)`; they never see SMTP or River.

## Migrations own the River schema

`migrations/000003_river.*` is the River schema, generated with
`go run github.com/riverqueue/river/cmd/river@v0.49.0 migrate-get --all --exclude-version 1 --schema river`.
**golang-migrate is the only owner of that schema — never run River's own migrator against this
database.** Version 001 is excluded, so there is no `river_migration` table. One statement was
folded into the generated SQL: `pending` is part of the initial `CREATE TYPE` instead of a later
`ALTER TYPE … ADD VALUE`, because golang-migrate runs a migration file inside one transaction and
PostgreSQL refuses to use an enum value that the same transaction added.

## Delivery

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

## Transport security

`SMTP_TLS=starttls` refuses to fall back to cleartext: a server that does not offer STARTTLS, or
offers an untrusted certificate, fails the send instead. Secrets (OTP, reset tokens) never appear
in `params`, in River args or in logs — they are sealed with AES-256-GCM under
`NOTIFICATION_SEAL_KEY` and erased the moment the row reaches a terminal state. Logs carry fixed
class text and a masked recipient, never provider output.
