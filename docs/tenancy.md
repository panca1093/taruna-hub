# Tenancy (P01, P01b)

MVP serves one tenant (D17). `TENANT_SLUG` names it, and the host, the headers and the client never
choose it. A middleware loads that tenant for every request and answers `404 not_found` on every
route when it is missing or inactive, so a misconfigured deployment serves nothing. `TENANT_SLUG` is
required everywhere; development uses the seeded `taruna`.

`GET /api/v1/tenant` publishes the configured tenant's slug and name. `/readyz` is the API →
PostgreSQL round trip: it pings the database and answers `503 {status, reason}` when the database is
unreachable.

`tenant_id` columns, the `tenants` table and the repositories' tenant scope stay, so every query is
still scoped to the configured tenant. No request has a platform scope: a null-tenant account is never
found by login, and a session or token that does not name the configured tenant is rejected, so a
platform account cannot act inside the tenant.

Links in emails point at `APP_URL`, never at a request header. The first tenant and its admins are
created with the bootstrap command (see `api/README.md`); there is no HTTP endpoint for it.
