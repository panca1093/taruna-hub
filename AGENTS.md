# AGENTS.md — Taruna Eduka Tryout Platform

Multi-tenant online tryout platform: students buy and take timed tryouts and metered Kecermatan tests, while staff manage content, finance, monitoring and reports. Monorepo with a Go API (`api/`) and a Next.js web app (`web/`).

## Sources of truth

Read the relevant document before changing behavior. Never silently contradict one.

| Concern | Source | Rule |
|---|---|---|
| Product behavior | `requirement/PRD.md` (v1.3) | Read-only for agents. Flag conflicts; don't edit the PRD to match code |
| System design | `requirement/ARCHITECTURE.md` | Tenancy, data ownership, runtime and job invariants |
| Delivery order | `requirement/PLAN.md` (slices P00–P23) | Work happens one slice at a time |
| Open policy | `requirement/RECONCILIATION.md`, decisions D01–D16 in ARCHITECTURE §9 | Don't invent policy for an unresolved decision; stop and ask |
| UI / UX | `requirement/DESIGN.md` | Tokens, components, patterns, copy and interaction rules |
| HTTP contract | `api/openapi.yaml` | The only wire contract. The frontend client is generated from it |

`requirement/RAB.md` and `rab-cloud.*` are client-facing cost documents. They are git-ignored and outside development scope.

## Repo map

```
api/                  Go 1.26 · Echo v4 · pgx v5 · raw SQL · golang-migrate
  cmd/server/         entrypoint
  internal/model      domain types
  internal/repository persistence only (SQL)
  internal/service    business rules
  internal/handler    HTTP transport, auth, validation, serialization
  internal/httpx      error envelope, middleware
  internal/infra      datastore wiring
  migrations/         numbered up/down SQL
  seed/dev.sql        dev fixtures; never a migration
  openapi.yaml        API contract
web/                  Next.js 16 · React 19 · HeroUI v3 · Tailwind v4
  app/                routes: public, /student, /admin
  components/         shared UI (ui/, admin/, marketing/)
  lib/api/            generated schema.ts + typed client: the data boundary
  lib/i18n/dict/      one dictionary file per feature (id + en)
  lib/mock-data.ts, demo-store.ts   mock layer, replaced slice by slice
requirement/          PRD, architecture, plan, reconciliation
docker-compose.yml    Postgres, Redis, MinIO
```

## Run locally

Default ports were already taken, so these are shifted: API **8081**, Postgres **5433**, Redis **6380**, MinIO **9002** (console 9003).

```bash
cd api && make reset     # containers + migrations + seed (idempotent)
cd api && make run       # API on :8081
cd web && npm run dev    # web on :3000
cd web && npm run gen:api   # after any openapi.yaml change
```

## Slice workflow

1. Pick the slice in `PLAN.md`. Read its architecture constraints and check its decision gates. If a gate is unresolved, build only the independent parts and ask about the rest.
2. **Contract first.** Define paths, schemas, error codes, pagination and idempotency fields in `openapi.yaml`, then run `npm run gen:api`.
3. Write the migration with `make migrate-create NAME=...`, including a working down migration.
4. Test first: service rules get unit tests, repositories and money/session flows get Postgres integration tests, and every endpoint gets tenant/role adversarial cases.
5. Replace the slice's mock reads in `web/` with `lib/api` calls. Keep the UI states that already exist: loading, empty, error, saved.
6. A slice is done only after a real browser → API → database demo passes, along with the checks under [Definition of done](#definition-of-done).

## Backend rules

- **Layering:** `handler → service → repository → model`. Business rules live in `service`, never in `repository` or `handler`. Repositories require a tenant scope argument and only persist.
- **Compact over modular.** One file per domain in each layer. Don't create a package for a single-use helper.
- **Tenancy:** the tenant comes from the validated host, and the JWT tenant must match it. Never trust a tenant or role sent by the client. Master content is read-only to tenants, not a null-tenant escape hatch.
- **Money and access:** ledger writes, balance projection, grants and critical jobs commit in one transaction. Gateway calls happen outside DB locks. Every retry keeps the same business identity (idempotency key).
- **Exam runtime:** server deadlines are authoritative. Randomization is stable per session. A completed session yields exactly one result. Content and scoring are snapshotted at assignment/admission.
- **Never** put grading keys, correct answers or internal snapshots in student DTOs.
- **Errors** use the envelope `{error:{code,message,fields?,request_id?}}` via `internal/httpx`, and codes are declared in OpenAPI.
- PostgreSQL is authoritative. Redis is derived and must be rebuildable.
- `X-Demo-User` / `devIdentity` is dev-only and gets deleted when real auth lands (P03). Never extend it.

## Frontend rules

- **Next.js 16 has breaking changes.** Read the matching guide in `web/node_modules/next/dist/docs/` before writing Next code (see `web/AGENTS.md`).
- **Data:** all server data flows through `web/lib/api/`. Never hand-write types that `schema.ts` already provides.
- **No business decisions in the browser.** Payment, scoring, quota, authorization and deadlines come from the API. The client timer only displays the server deadline.
- **i18n:** every user-visible string goes through a `lib/i18n/dict/<feature>.ts` file with both `id` and `en`, where `en` is typed `typeof id`. Indonesian is the default. No hardcoded copy in components.
- Role switching and the demo session are mock-only. Production session state comes from authenticated claims.

## UI / UX consistency

`requirement/DESIGN.md` is the UI contract. Read it before any UI work, and follow it over your own taste or framework defaults.

- **Tokens only.** Colors, radius, fonts and shadows come from the CSS variables in `web/app/globals.css` (HeroUI semantic tokens plus `--accent-2`, `--ring-start`, `--ring-end`). No hex or oklch literals, and no arbitrary Tailwind values like `bg-[#...]` or `rounded-[7px]` in components. One documented exception: program category chips use Tailwind palette classes, and those live only in `components/status-badge.tsx` (DESIGN §2.3).
- **Reuse before creating.** Check `web/components/` first: `app-shell`, `data-table`, `empty-state`, `stat-card`, `status-badge`, `countdown-ring`, `latex`, `ui/form`, `ui/form-modal`, `ui/link-button`. When a pattern appears a second time, extract it into `components/` and document it in `requirement/DESIGN.md`.
- **HeroUI v3 is compound-component based.** Follow its documented nesting exactly. Two known traps:
  - `Switch.Control` must sit inside `Switch.Content`, or it renders but can't be clicked.
  - Drawer width goes on `Drawer.Dialog`, not `Drawer.Content`.
- **Data states:** specify every applicable state from DESIGN §7.5 before connecting an API.
- **Acceptance:** every UI change passes the DESIGN §10 review checklist (viewport widths, themes, locales, keyboard, contrast).
- **Math** renders through `components/latex.tsx` (KaTeX). Never use raw HTML injection for question content.
- **Design changes stay in sync.** A change to a token or shared pattern updates `globals.css`, the component and `requirement/DESIGN.md` in the same commit.

## Definition of done

Run these in order. Run the web build and the dev server one at a time, never together, so they don't clobber `.next`.

```bash
cd api && make build vet test                       # tests need `make up` + migrations
cd web && npm run gen:api && git diff --exit-code lib/api/schema.ts
cd web && npx tsc --noEmit && npm run lint && npm run build
```

For UI changes, also run the DESIGN §10 checklist in a browser. Report check results honestly. If something was skipped or failed, say so with the output.

## Conventions

- Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:` plus a short imperative summary. Mention the slice ID when relevant (`feat: P03 tenant login`).
- Secrets live only in `.env` files, which are git-ignored. `api/.env.example` is the template.
- Stay in the slice's scope. Flag nearby issues instead of fixing them uninvited.
- Comments explain *why*, not *what*. Default to no comment.
