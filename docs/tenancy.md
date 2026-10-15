# Tenancy (P01)

The tenant comes from the request host, never from the client: `<slug>.<BASE_DOMAIN>` selects a
tenant, `admin.<BASE_DOMAIN>` selects the platform, and every other host answers `404`.
`X-Forwarded-Host` is ignored unless `TRUST_FORWARDED_HOST=true`, which is only safe behind a
proxy that strips the client's copy. `BASE_DOMAIN` is required outside development.

`GET /api/v1/tenant` publishes the active tenant the request host resolves to, or a `404` that is
identical for an inactive, unknown, reserved, malformed or platform host. `/readyz` is the API →
PostgreSQL round trip: it pings the database and answers `503 {status, reason}` when the database is
unreachable.

The first tenant and its admins are created with the bootstrap command (see `api/README.md`); there is
no HTTP endpoint for it.
