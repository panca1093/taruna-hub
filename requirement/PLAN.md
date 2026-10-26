# Taruna Eduka Full-scope Delivery Plan

**Version:** 2.0 · **Date:** 2026-10-09 · **Status:** draft for review
**Basis:** [PRD v1.3](PRD.md), [architecture](ARCHITECTURE.md), [reconciliation/traceability](RECONCILIATION.md).
**Supersedes:** plan v1.0 at Git `4f2246a`; its July schedule and August go-live assumptions are historical.

**Goal:** Deliver all 161 numbered PRD requirements plus its unnumbered question, scoring and non-functional requirements through verifiable vertical slices.

**Architecture:** One host-aware Next.js application and one Go application deployed as API/worker. PostgreSQL owns money, access, answers/results and River jobs; Redis is derived state; MinIO/GCS stores authorized assets.

**Tech stack:** Existing Go 1.26.3 / Echo v4 / pgx v5 / raw SQL / golang-migrate; Next.js 16.2.9 / React 19 / HeroUI / Tailwind v4; PostgreSQL / Redis / River / MinIO-GCS / Midtrans. New integration versions are pinned during their owning slice after compatibility verification.

**For implementers:** This is a delivery plan at slice level, not a claim that each future function/wire schema is already specified. Before coding a slice, review its architecture constraints, resolve its listed decision gates, define its OpenAPI/migration contract and write a focused test-first implementation checklist. Native execution is the default; delegation, commits, pushes, deployment and production mutation require their applicable user authorization. No application implementation occurs as part of this document task.

## 1. Global constraints and completion rule

- Full PRD v1.3 scope remains in the plan. Incremental staging demos do not remove later requirements from release acceptance.
- Business rules belong in services; repositories require scope and perform persistence. Follow existing layering without creating packages for single-use helpers.
- Shared schema; tenant scope from validated host; JWT tenant must match; master is visible read-only, not a null-tenant authorization escape.
- Money/grants/critical jobs commit atomically; gateway side effects happen outside database locks; all retries preserve business identity.
- Unified tests/access/session roots; separate typed answers/results. Snapshot content and scoring at assignment/admission as specified by the architecture.
- Server deadlines, stable session randomization, exactly one result per completed session, distinct accepted/pending answers, no hidden grace policy.
- All three payment methods, self-service top-ups, item-package expansion, PDF and XLSX/CSV remain required; post-MVP features remain excluded.
- Current application baseline is `4f2246a`; P00 is partially implemented. Existing build/type/Go checks passed in the 2026-10-08 scan, lint had 16 errors/2 warnings, and browser/database integration was unverified. Those are dated observations, not fresh certification.
- A slice is complete only after a real browser/API/database demo, meaningful unit/integration cases, tenant/role adversarial checks, and its required provider/file-output verification pass.
- The PR that delivers a slice, or part of one, updates that slice here in the same PR: the **Status** line says `done`, `done, lean` or `in progress` and names what is deferred; each acceptance box the PR proves with a test or demo is ticked; each box left open gets one indented line saying why. A slice PR without this update is not finished.
- Every changed API is defined in `api/openapi.yaml`; regenerate client and enforce drift checks. Read installed Next.js documentation before frontend code changes.
- Go commands use `GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec` on this workstation and a writable cache. Run Next build and browser checks sequentially to protect `.next` state.
- Multi-tenancy is out of MVP ([D17](ARCHITECTURE.md#decision-register)). §1.1 overrides the tenant parts of every slice below.

### 1.1 MVP tenancy override (D17, 2026-10-28)

MVP is one platform with one configured tenant on one domain. Keep the `tenant_id` columns and the repository scope arguments as built, and keep passing the configured tenant through them. New slices add no cross-tenant adversarial cases, master-content logic or platform-admin surface; role and ownership adversarial cases still apply.

| Slice | Effect |
|---|---|
| P01 | Built with host resolution. Replaced by **P01b** below |
| **P01b** | New, size S: the API takes the tenant from configuration instead of the request host, and the web app runs on one domain without a tenant subdomain. Existing tenant-scoped tests stay |
| P04 | Shrinks to tenant settings and branding for the one tenant (including the FR-PAY-W-09 minimum top-up). Provisioning, pausing and support context are post-MVP |
| P05, P07, P09 | Tenant-owned content only. Master content, copy-to-own and platform master UI are post-MVP |
| P11 | One landing and catalog on one domain; no root/tenant host split |
| P17 | Ranking within the tenant is ranking across the platform |
| P22 | Tenant Admin financial view stays; Platform Admin cross-tenant recap is post-MVP |
| P23 | Host routing, two-tenant fixtures and cross-tenant adversarial checks are dropped from MVP release evidence |

## 2. Review focus

1. **Scope escape:** Host/token/guessed-ID/cache/job/asset mismatches must never disclose or mutate a foreign tenant; owned by every slice, especially P01/P04/P06.
2. **Repeated or uncertain payment:** Concurrent review, duplicated webhook and create-timeout must never duplicate a debit/credit/grant; P12–P15.
3. **Closed/offline/competing tabs:** Deadlines keep running, retries do not rewind answers or consume extra quota, and finalization happens once; P16/P18.
4. **Content changes:** Master/package/product edits cannot rewrite bought access, assigned questions or historical results; P07–P10/P13/P16/P18.
5. **Loss of derived/async state:** Redis loss and worker restart preserve authoritative ranking and queued critical work; P02/P17/P20/P21/P23.

## 3. Slice map and dependencies

Sizes are relative risk/work (M/L/XL), **not promised days or money**. A slice completion depends on its listed predecessors; independent work may overlap only with agreed interfaces. P15 precedes P14 in execution order.

| Slice | Capability | Size | Prerequisites |
|---|---|---|---|
| [P00](#p00) | Foundation health and integration baseline | M | None |
| [P01](#p01) | Tenant-safe bootstrap | M | P00 |
| [P02](#p02) | Durable jobs and delivery transport | M | P00 |
| [P03](#p03) | Real tenant accounts and sessions | L | P01, P02 |
| [P04](#p04) | Tenant onboarding, branding and support context | L | P03 |
| [P05](#p05) | Master and tenant curriculum | M | P04 |
| [P06](#p06) | Authorized asset upload and download | M | P03, P04 |
| [P07](#p07) | Question bank and imports | L | P05, P06 |
| [P08](#p08) | Tryout builder with frozen assignments | M | P07 |
| [P09](#p09) | Kecermatan item packages | L | P05, P06 |
| [P10](#p10) | Kecermatan builder and deterministic generator | L | P09 |
| [P11](#p11) | Catalog, products and tenant landing | M | P04, P08, P10 |
| [P12](#p12) | Wallet ledger and staff adjustments | M | P03, P04 |
| [P13](#p13) | Manual and wallet checkout with enrollment | L | P02, P06, P11, P12 |
| [P14](#p14) | Gateway orders and top-ups | L | P13, P15 |
| [P15](#p15) | Self-service manual top-up and review | M | P02, P06, P12 |
| [P16](#p16) | Tryout runtime and immediate basic result | XL | P08, P13 |
| [P17](#p17) | Tryout history, ranking and controlled review | L | P16 |
| [P18](#p18) | Kecermatan attempts, quota and full results | XL | P10, P13 |
| [P19](#p19) | Monitoring and violation timelines | M | P16, P17 |
| [P20](#p20) | Asynchronous tryout PDF | M | P02, P06, P08 |
| [P21](#p21) | Result dashboards and CSV/XLSX exports | M | P02, P06, P17, P18 |
| [P22](#p22) | Student operations and platform financial recap | M | P04, P13, P14, P17, P18 |
| [P23](#p23) | Full-scope certification and release | XL | P00, P01, P02, P03, P04, P05, P06, P07, P08, P09, P10, P11, P12, P13, P14, P15, P16, P17, P18, P19, P20, P21, P22 |

Representative prerequisite chains:

- Tryout: P00 → P01 → P03 → P04 → P05 → P07 → P08 → P11 → P13 → P16 → P17 → P23.
- Kecermatan: P04 → P05/P06 → P09 → P10 → P11 → P13 → P18 → P23.
- Gateway finance: P03 → P12 → P15; P11/P12 → P13; P13/P15 → P14 → P23.
- Durable notifications: P00 → P02 → P03, without a reverse dependency on authentication.

These are dependency chains, not a duration-based critical path. Staffing, detailed estimates and provider onboarding determine the calendar critical path.

## 4. Vertical delivery slices

For every slice, use this verification loop:

- [ ] Resolve listed business/contract gates and record the reviewed decision.
- [ ] Define only this slice's OpenAPI, migrations, DTOs and permission boundaries.
- [ ] Write reproducing/behavioral cases for its acceptance scenarios; confirm failure before implementation.
- [ ] Implement the minimum backend/frontend/integration change and remove that domain's production mock path.
- [ ] Verify scenarios with real persistence/providers/artifacts as required; document commands, fixture, results and limitations.
- [ ] Demonstrate from the browser and check predecessor/scope invariants; update slice status after evidence passes.

<a id="p00"></a>

### P00 — Foundation health and integration baseline

**Status:** done — the demo identity is gone in every APP_ENV, `/readyz` proves the API → PostgreSQL round trip (`/dev/api-check`), returned 401/500 are logged with their real status, and CI enforces build/vet/race, tsc/lint/build and generated-schema drift · **Size:** M · **Depends:** None

**Outcome:** A developer can start a fresh local stack and demonstrate a real scoped browser/API/PostgreSQL round trip.

**DB / backend / integration:** Existing API config/probes/error handling, initial schema, Compose, web API client, CI. No environment is deployed yet, so the initial migration may be rewritten until the first deployment.

**Files / frontend:** api/internal/config, handler, httpx; api/Makefile; docker-compose.yml; .github/workflows/ci.yml; web/lib/api; web/app/dev/api-check.

**Interfaces:** Produces consistent errors/request IDs, liveness/readiness, migration workflow, generated contract check. No demo identity exists; the first identity is P03 login.

**Acceptance scenarios:**

- [x] Fresh database migrations and local fixtures succeed; browser renders live API and database readiness from /readyz, not a mock.
- [x] No request can authenticate through a client-supplied header or fallback identity in any APP_ENV.
- [x] Returned 401/500 is logged as its actual status.
- [x] Go build/vet/race tests, TypeScript/build and lint are enforced in CI; generated schema drift fails.

**Decision / verification gate:** Direct browser/database verification; existing lint failures and no-running-container baseline are addressed here.

<a id="p01"></a>

### P01 — Tenant-safe bootstrap

**Status:** done — hosts resolve to tenants independently, platform and tenant accounts have separate unique indexes, and the first tenant and admin come from the bootstrap command only; no deployed data existed, so the ownership migration was folded into `000001_init` instead of mapping rows · **Size:** M · **Depends:** P00

**Outcome:** Two tenant hosts resolve independently and the initial platform/tenant identities can be provisioned without a public privilege-escalation path.

**DB / backend / integration:** Add tenants and role/null-tenant constraints; replace global identifier uniqueness with tenant indexes and platform partial indexes. Scoped repository entry points and host validation.

**Files / frontend:** api/migrations; api/internal/model/user.go; repository/user.go; service/user.go; handler/tenant.go; httpx; explicit development seed/bootstrap command.

**Interfaces:** Produces validated tenant scope, master/platform scope and initial account provisioning. P03 supplies interactive authentication; no open public admin-bootstrap endpoint.

**Acceptance scenarios:**

- [x] Tenant A and B can contain the same email/phone; duplicates inside A fail.
- [x] Unknown host, misleading suffix, reserved slug and spoofed forwarding header do not select a tenant.
- [x] Child/root repository probes cannot return tenant B resources under A scope.
- [x] Existing data is inventoried; any ownership migration uses an explicit mapping and preserves IDs.

**Decision / verification gate:** Bootstrap credentials remain local/secret; production provisioning is an explicit operational action.

<a id="p02"></a>

### P02 — Durable jobs and delivery transport

**Status:** done for email — River jobs enqueue in the caller's transaction, SMTP delivery retries up to six attempts, and unconfigured channels report unavailable; WhatsApp has no provider, so D16 stays open for it · **Size:** M · **Depends:** P00

**Outcome:** A committed business event survives a worker restart and reaches the configured email/WhatsApp provider.

**DB / backend / integration:** River schema/migrations, worker entry point, thin transaction-aware dispatch, notification logs, provider adapters, protected secret handling. Delivery transport precedes user-event templates to remove the former auth/notification cycle.

**Files / frontend:** api/cmd/worker; api/internal/infra; service notification/event handling; adapter notifier; api/migrations; Compose worker configuration.

**Interfaces:** Produces durable enqueue in caller transaction, channel delivery outcomes, retries/discards and redacted logs. Consumes database only; entity-specific templates are added with owning slices.

**Acceptance scenarios:**

- [x] Rollback business transaction leaves neither state nor job.
- [x] Kill worker after enqueue; restart sends the committed job; provider failure is visible and recoverable.
- [x] Duplicate event/channel identity is suppressed; ambiguous provider timeout is recorded without claiming exactly-once delivery.
- [x] Unconfigured WhatsApp is marked unavailable, never delivered; expired reset/OTP messages are not sent.

**Decision / verification gate:** D16 provider/secret decisions; bounded retry policy must be specified and tested before slice sign-off.

<a id="p03"></a>

### P03 — Real tenant accounts and sessions

**Status:** done — D02 is resolved (the Operator is allowed what the PRD leaves ambiguous); P03a sessions/login/role guard, P03b registration, email code, reset, rotation and throttling, and P03c web login, registration, recovery and session screens are done; the Operator guard on user and settings mutations has nothing to test until P04/P22 add those endpoints · **Size:** L · **Depends:** P01, P02

**Outcome:** Students and staff authenticate in their own tenant and recover accounts through the approved channel.

**DB / backend / integration:** JWT access/rotating refresh, server idle tracking, bcrypt, reset token single use, OTP if retained; tenant/role/resource guards. Define auth OpenAPI schemas first.

**Files / frontend:** api/internal/handler/me.go; service/user.go; repository/user.go; new auth files and migrations; web/lib/session-context.tsx; login/staff/register/forgot-password; require-role.

**Interfaces:** Consumes validated host scope and notifier; produces authenticated principal and audited session. Disable role-switching UI for integrated production.

**Acceptance scenarios:**

- [x] Register/login with email and phone, including identical identifiers in two tenants; foreign-tenant JWT is rejected.
- [x] Password/reset/refresh/OTP expiration and reuse tests include concurrent requests and brute-force limits.
- [ ] Operator cannot mutate programs/users/settings under the agreed D02 matrix; direct API is tested.
      Programs are tested under D02 (curriculum role tests); users and settings have no mutation endpoints until P04/P22.
- [ ] Reload/refresh/logout/idle expiry and active-exam policy behave as specified; secrets are absent from logs.
      Everything but the active-exam half is done and tested; D13 says an exam keeps its session alive, which P16 implements.

**Decision / verification gate:** D01 OTP and phone-only recovery; D02 privileges; D13 token/idle policy. Auth cannot be signed off with phone signup silently removed.

<a id="p04"></a>

### P04 — Tenant onboarding, branding and support context

**Status:** planned — not started. P05, P11 and P12 shipped lean without it, on tenants and admins created by the P01 bootstrap command; the platform onboarding UI, branding/settings and support context are all still here · **Size:** L · **Depends:** P03

**Outcome:** Platform Admin onboards a school, provisions its first Admin, and enters a tenant for audited troubleshooting.

**DB / backend / integration:** Platform tenant CRUD, slug validation/immutability, active state, first-admin transaction, branding/settings, scoped support session. Draft domain deployment contract from D06.

**Files / frontend:** Tenant/platform handlers/services/repositories and migrations; new web/app/platform pages; web/app/admin/settings; host-aware web layout.

**Interfaces:** Consumes platform authentication; produces active tenant configuration/support context. Root marketing and tenant routing remain one deployment.

**Acceptance scenarios:**

- [ ] Create school and first Admin together; retry does not duplicate either; slug cannot be renamed or use reserved names.
- [ ] Paused tenant is unavailable to tenant/public users but accessible to actual Platform Admin support.
- [ ] Support mutations retain platform actor and target tenant in audit, without impersonating a student.
- [ ] Tenant A branding/settings never appears on B across sequential requests or cache reuse.

**Decision / verification gate:** D06 host/first-slug/root content; D15 in-flight paused-tenant operations.

<a id="p05"></a>

### P05 — Master and tenant curriculum

**Status:** done, lean — tenant programs/subjects/chapters with role-scoped CRUD, tenant composite FKs, RESTRICT delete guards and the web programs page on the API; master content, copy-to-own and origin markers are deferred · **Size:** M · **Depends:** P04

**Outcome:** Staff choose authorized master/tenant programs, subjects and chapters; Admin maintains the tenant hierarchy.

**DB / backend / integration:** Curriculum ownership and references, master read-only/copy lineage, deletion guards and role-scoped CRUD.

**Files / frontend:** api curriculum model/repository/service/handler and migrations; web/app/admin/programs; platform master curriculum pages.

**Interfaces:** Produces master-plus-own curriculum selections with origin markers. Operator may read authorized dropdowns but cannot mutate programs.

**Acceptance scenarios:**

- [ ] Master visible in A/B, tenant A content invisible to B; tenant cannot mutate master.
      Tenant isolation is done and tested; master content is deferred.
- [ ] Admin copies master hierarchy into own content; later master changes do not change the copy.
      Deferred with master content.
- [x] Delete referenced program/subject/chapter is rejected clearly; cross-owner hierarchy links are rejected.
- [ ] Create chapter and use it in the editor without manual page reload.
      Not recorded as verified; needs a browser check.

**Decision / verification gate:** Includes FR-ADM-PRG and shared-content semantics; copy is explicit and never live synchronization.

<a id="p06"></a>

### P06 — Authorized asset upload and download

**Status:** done, lean — authenticated PNG/JPEG/WebP upload proved by decoding, staged→final with a transaction-aware finalize, 60-second presigned downloads re-authorized on every call, and a worker purge of abandoned staged uploads, all against MinIO; public branding assets, cover/export purposes and GCS staging verification are deferred · **Size:** M · **Depends:** P03, P04

**Outcome:** Users upload allowed images/proofs and only authorized users retrieve protected assets.

**DB / backend / integration:** MinIO/GCS adapter, staged/finalized object metadata, purpose/tenant/actor ownership, public branding versus private exam/proof/export access.

**Files / frontend:** api adapter storage and asset handlers/services/migrations; web upload controls used by builders and student orders.

**Interfaces:** Produces asset references and scoped upload/finalize/download operations, not permanently public URLs for private files.

**Acceptance scenarios:**

- [x] Validate actual image type and maximum 5MB; incorrect type/oversize files fail before attachment.
- [x] Tenant B cannot attach, finalize or download A assets; client path/purpose does not change owner. (Upload, finalize and download are tested; "attach" has no consumer until P07/P13 call `FinalizeTx`.)
- [ ] Private objects are not anonymously readable; authorized access expires and is not cached publicly. (Proved on MinIO: an unsigned GET is `403`, the 60 s link works and then stops, the response is `private, no-store`. Open until the same is shown against the real GCS bucket in staging, and until P04 decides which assets are public.)
- [x] Abandoned previews clean up safely without deleting referenced frozen assets.

**Decision / verification gate:** Storage credentials and final authorization must be verified in staging, not just mocked. **Open:** only MinIO has been exercised (no mocks); GCS HMAC interoperability, uniform bucket-level access and the staging credentials still need a run.

<a id="p07"></a>

### P07 — Question bank and imports

**Status:** P07a and P07b done — four question types with immutable versions, filters, the bank page and the confirmed XLSX/CSV/DOCX import; D08's default commit policy still awaits Panca, and master content, copy, images and per-question weights are deferred · **Size:** L · **Depends:** P05, P06

**Outcome:** Staff create four question types or import real documents and maintain tenant/master content safely.

**DB / backend / integration:** Question/options validation, immutable revisions, master copy, filtering, templates, actual XLSX/CSV/DOCX parsing and confirmed import commit.

**Files / frontend:** Question files and migrations; web/app/admin/questions; components/admin/question-editor, import-questions-modal and rich-text-editor; platform master bank.

**Interfaces:** Produces immutable question versions and authorized content selections. Uses curriculum and assets; option weights follow stable option IDs.

**Acceptance scenarios:**

- [ ] All four types gradeable; weighted-option mode only pg; absent points default to 1 and blank response earns no default credit.
      The four types are stored and validated; grading is proven only when P16 scores an attempt, and per-question weights are deferred.
- [ ] Tenant copy survives master edit; current assignments retain old question/scoring/explanation revision.
      Immutable versions are done and tested (P08 assigns frozen versions); the master/copy half is deferred.
- [ ] Upload malformed and valid rows, see real errors, confirm exact accepted set per D08; retry creates no duplicates.
      Built and tested on the default commit policy (preview, confirmed subset, idempotent retry); ticks once D08 is confirmed.
- [ ] LaTeX/images preview correctly; filters/IDs work; HTML and spreadsheet-formula input is handled safely.
      Filters, IDs, stored-verbatim HTML and formula neutralising are tested; images are deferred.

**Decision / verification gate:** D08 import commit policy; D04 difficulty weights. No hardcoded import preview counts as done.

<a id="p08"></a>

### P08 — Tryout builder with frozen assignments

**Status:** done — draft/published tryouts with ordered subtests, frozen question versions, stale-revision conflict, the first-attempt lock, the list and builder on the API, and a seed covering draft, published and locked; scoring stays a mode only (D04 open), participants/cover/student endpoints are deferred · **Size:** M · **Depends:** P07

**Outcome:** Staff assemble tenant tryouts with ordered subtests, dates, explanation release and shuffle settings.

**DB / backend / integration:** Unified tests plus tryout details/subtests/question assignments; immutable assigned revisions, validation and full edit/delete lock after first attempt.

**Files / frontend:** Test/tryout files and migrations; components/admin/tryout-builder and question-picker-drawer; web/app/admin/tryouts.

**Interfaces:** Produces a playable tryout definition and versions for P11/P16/P20. Both master and own questions may be assigned, another tenant content may not.

**Acceptance scenarios:**

- [x] Subtest/question order, durations, date/release fields and randomization flags survive reload.
- [x] Invalid date window or empty playable definition cannot be published.
- [ ] First session locks every editable field/delete; master update never changes assigned content.
      The lock is done and a question edit never changes assigned content; the master half awaits master content (P07a deferred it).
- [x] A tenant cannot assign another tenant question or mutate master through the builder.

**Decision / verification gate:** D04 scoring configuration ownership/normalization; original-order PDF input is preserved.

<a id="p09"></a>

### P09 — Kecermatan item packages

**Status:** planned · **Size:** L · **Depends:** P05, P06

**Outcome:** Staff manage authorized symbol/image/emoji/shape sources with real preview and safe imports.

**DB / backend / integration:** Packages and frozen revisions; CSV/TXT/XLSX symbol inputs, palette/paste, multi-image/ZIP staging, content-hash dedup, emoji catalog/assets, shapes, master copy.

**Files / frontend:** New package model/service/handlers/migrations; new web/app/admin/item-packages; platform master package view; shared file controls.

**Interfaces:** Produces deduplicated versioned source pools and ephemeral column preview. Include emoji in schema despite the PRD data-model omission.

**Acceptance scenarios:**

- [ ] ZIP traversal/symlink/nested archive/oversize/corrupt/duplicate files produce per-element reasons; preview changes no live package.
- [ ] Confirmation inserts accepted elements only; master mutation by tenant and foreign-tenant package use are rejected.
- [ ] Used package delete/shrink below any referencing test threshold fails; existing frozen assets remain renderable.
- [ ] Emoji is catalog-only and consistent across devices; pasted emoji in symbols fails with guidance; preview does not consume quota.

**Decision / verification gate:** D02 (resolved: the Operator may mutate packages); D09 bounded extraction/session size and emoji licenses.

<a id="p10"></a>

### P10 — Kecermatan builder and deterministic generator

**Status:** planned · **Size:** L · **Depends:** P09

**Outcome:** Staff build valid tests and preview the repeated-column mechanism before publication.

**DB / backend / integration:** Unified tests plus kecermatan details/package links; bounded server generator, source selection, globally unique elements per column and N/2 snapshot replay.

**Files / frontend:** Kecermatan definition/generator files and migrations; components/admin/kecermatan-builder; web/app/admin/kecermatan.

**Interfaces:** Produces frozen generation configuration for P11/P18. No student response exposes scoring keys/future snapshots.

**Acceptance scenarios:**

- [ ] Reject odd or fewer than 10 columns, fewer than 50 questions, item length outside 1–5, nonpositive duration and insufficient pool.
- [ ] Across each column reference set no element repeats within/across items or mix sources.
- [ ] Columns k and k+N/2 have identical references/questions/subset ordering, with separate answer keys per column identity.
- [ ] Reject edit/delete after a session exists; supported char/number/symbol/image/emoji/shape/mix sources render.

**Decision / verification gate:** D09 generator limits; generation policy is fixed by content snapshots, not a client RNG.

<a id="p11"></a>

### P11 — Catalog, products and tenant landing

**Status:** done, lean — individual and bundle products with exact-numeric prices, admin CRUD and an anonymous host-resolved catalog plus landing/catalog pages on the API; kecermatan products (no test entity yet), checkout, covers and the D06 platform landing are deferred · **Size:** M · **Depends:** P04, P08, P10

**Outcome:** Each school publishes all three product types and students see its real branded landing/catalog.

**DB / backend / integration:** Products/product_tests, active status, price/display fields, tenant-local filters and public landing statistics/featured products.

**Files / frontend:** Product/catalog services/handlers/migrations; web/app/page.tsx; catalog/detail; admin/products; platform landing routing.

**Interfaces:** Produces purchasable validated product IDs and test membership. Complete scope includes kecermatan products, not only a tryout-first interim demo.

**Acceptance scenarios:**

- [x] Individual exactly one tryout and bundle two to twenty in one program; the kecermatan half waits for P09's test entity.
- [x] Inactive product is hidden publicly; active products, filters and details contain only resolved tenant data.
- [x] Deactivate product and verify landing/catalog update without deploy; sequential A/B requests show correct branding/content.
- [ ] Root CTAs target approved hosts and the landing download copy matches D11 — both wait on D06/D11; the "1.000+ siswa" claim was replaced with the real visible-tryout count.

**Decision / verification gate:** D06 root featured-product ownership/domain routing; D11 landing download promise.

<a id="p12"></a>

### P12 — Wallet ledger and staff adjustments

**Status:** done, lean — an exact-numeric ledger behind one transaction-aware movement path, a non-negative balance projection enforced twice, admin and operator adjustments with a required reason and an idempotency key, the student wallet page and the admin students page on the API, and a seed whose ledger explains every balance; top-up requests, checkout spending (P13 calls the same path), minimums, refunds and the D12 denomination follow-ups are deferred · **Size:** M · **Depends:** P03, P04

**Outcome:** Students view exact running balances and authorized staff make audited adjustments.

**DB / backend / integration:** Exact numeric balance projection, signed immutable ledger, balance-after/source/actor metadata, row locking and adjustment validation.

**Files / frontend:** Wallet service/repository/handlers and migrations; web/app/student/wallet; admin student adjustment controls.

**Interfaces:** Produces one transaction-aware wallet movement path consumed by checkout/top-ups. Financial source uniqueness is independent of cache.

**Acceptance scenarios:**

- [x] Concurrent debits/credits serialize; ledger sum equals projection and every balance-after is consistent.
- [x] Adjustment requires a reason; actor kind/identity distinguish student, provider and staff/platform support.
- [x] Duplicate source/operation creates one entry; rollback leaves projection/ledger unchanged.
- [x] Student and staff lists cannot see another tenant/user wallet; migrated opening balances have explicit entries.

**Decision / verification gate:** D12 precision/rounding/negative balance; no float write path.

<a id="p13"></a>

### P13 — Manual and wallet checkout with enrollment

**Status:** planned · **Size:** L · **Depends:** P02, P06, P11, P12

**Outcome:** Students purchase through manual transfer or wallet and receive the correct access after confirmation.

**DB / backend / integration:** Order snapshots, proofs/review/cancellation, unified enrollment plus source grants, retry safety, actor audit and durable order/access notifications.

**Files / frontend:** Order/enrollment service/repositories/handlers/migrations; web/app/checkout; student/orders/tryouts; admin/orders.

**Interfaces:** Produces pending/confirmed/cancelled orders and idempotent access issuance for P14/P16/P18; repeat kecermatan purchase adds quota to same aggregate.

**Acceptance scenarios:**

- [ ] Insufficient wallet balance creates no debit/confirmed order/grant; mid-transaction failure rolls back all.
- [ ] Repeated checkout/review creates one debit/grant per source; manual cancel opens no access.
- [ ] Proof belongs to requesting student/tenant; staff sees/reviews only own tenant orders.
- [ ] Bundle grants every purchased test; repeat kecermatan purchase preserves history on same enrollment; correct email/WA events enqueue atomically.

**Decision / verification gate:** D05 expiry/credit aggregation and D12 money policy; external delivery success is verified through P02.

<a id="p14"></a>

### P14 — Gateway orders and top-ups

**Status:** planned · **Size:** L · **Depends:** P13, P15

**Outcome:** Verified Midtrans success confirms purchases or credits top-ups exactly once across tenants.

**DB / backend / integration:** Globally unique provider references, separate payment attempts/events, signed amount-checked notifications, ambiguous-create/status recovery and exception reconciliation.

**Files / frontend:** adapter Midtrans; payment/webhook service/repository/handlers/migrations; checkout gateway action/status; top-up gateway flow.

**Interfaces:** Consumes order/top-up models and wallet/grant services. Webhook is provider-authenticated and resolves tenant from stored reference.

**Acceptance scenarios:**

- [ ] Sandbox purchase and top-up succeed; return URL alone cannot grant access or credit.
- [ ] Replay notification three times and reorder pending/success events: one money movement/grant and no state downgrade.
- [ ] Wrong signature/reference/amount/fraud status does not confirm; ambiguous create timeout does not make a second payment.
- [ ] Late paid cancellation, rejected top-up, chargeback and paused-tenant events retain evidence and follow approved D07/D15 policy.

**Decision / verification gate:** D07 late/reversal cases; D15 paused operations; D16 one platform-owned merchant account.

<a id="p15"></a>

### P15 — Self-service manual top-up and review

**Status:** planned · **Size:** M · **Depends:** P02, P06, P12

**Outcome:** Students request a top-up and staff approve or reject it with a complete audit trail.

**DB / backend / integration:** Separate topup_requests, minimum tenant amount, proof ownership, reviewed actor/reason and idempotent credit.

**Files / frontend:** Top-up service/repository/handlers/migrations; student/wallet and orders history; new admin top-ups view.

**Interfaces:** Produces top-up entity/payment target for P14; top-ups never fake a product order or grant a test.

**Acceptance scenarios:**

- [ ] Below-minimum value fails server validation; valid request remains pending without credit.
- [ ] Two concurrent approvals create one credit; reject requires reason and creates none.
- [ ] Student sees own status/history and staff filters tenant requests; proof access obeys ownership.
- [ ] Gateway extension uses the same business credit invariant while keeping provider evidence distinct.

**Decision / verification gate:** D12 amount policy. Listed before P14 in execution order despite stable numeric slice labels.

<a id="p16"></a>

### P16 — Tryout runtime and immediate basic result

**Status:** planned · **Size:** XL · **Depends:** P08, P13

**Outcome:** Students complete repeatable tryouts with server timing, saved answers, stable shuffles and immediate persisted scores.

**DB / backend / integration:** Unified sessions/parts, versioned snapshots, answer sequencing/retry, deadline scan/finalization, active-session uniqueness, subtest transition and scoring core.

**Files / frontend:** Session/runtime/scoring files and migrations; web/lib/tryout-session; student/tryout overview/subtest/result; typed API client.

**Interfaces:** Produces exactly one completed result per session and transactional score sum/count for P17. Gateway readiness is a full-release gate, not needed to demo a wallet-funded runtime.

**Acceptance scenarios:**

- [ ] Concurrent start/resume consumes one active session/attempt number; reload retains snapshot and accepted answers.
- [ ] Background/closed tab and device clock changes cannot extend deadline; synchronized expiry meets ±5s under the agreed normal workload.
- [ ] Late/stale/reordered answer writes are rejected; submit retry is idempotent; direct access cannot return to a submitted subtest.
- [ ] Seeded shuffle fixtures preserve option points and session order; all four question types and weighted-option grading match hand calculations; score appears at completion.

**Decision / verification gate:** D04 score rules and D10 expiry/offline policy. Include server-accepted versus unsent browser-answer indicators.

<a id="p17"></a>

### P17 — Tryout history, ranking and controlled review

**Status:** planned · **Size:** L · **Depends:** P16

**Outcome:** Students review all attempts and tenant-local average-based rankings without premature explanation access.

**DB / backend / integration:** Result/history queries, revision-aware average leaderboard, Redis fallback/rebuild, IRT prevalidation and explanation release gating.

**Files / frontend:** Result/leaderboard services/repositories; web student result/history and admin results; Redis adapter.

**Interfaces:** Consumes persisted completed results; produces authoritative tenant/test ranking and staff filters/IRT indicators.

**Acceptance scenarios:**

- [ ] Scores 60/80/100 rank with average 80; no foreign-tenant participant enters ranking.
- [ ] Missing/stale Redis or lost cache event returns/rebuilds correct PostgreSQL average; tied scores follow D04.
- [ ] Before explanation release inspect raw API and asset links: no hidden key/explanation leaks; after release own completed attempt review succeeds.
- [ ] IRT threshold fixtures count approved distinct students/question revisions correctly; weighted scoring remains unchanged.

**Decision / verification gate:** D04 counts/ties/global weights; current rank is distinguished from any historical result rank.

<a id="p18"></a>

### P18 — Kecermatan attempts, quota and full results

**Status:** planned · **Size:** XL · **Depends:** P10, P13

**Outcome:** Students consume purchased credits once, complete repeated timed columns, and inspect confirmed composite results/history.

**DB / backend / integration:** Atomic quota admission, snapshot persistence/replay, server column/transition deadlines, typed answers/results, zero/wrong distinction and scoring-version capture.

**Files / frontend:** Kecermatan session/result files and migrations; web/lib/kecermatan-session; student kecermatan overview/attempt/result; admin kecermatan results.

**Interfaces:** Consumes generator/credits; produces per-column and total counts, speed/accuracy/endurance/final scores plus full review. No fullscreen/violations/ranking.

**Acceptance scenarios:**

- [ ] Concurrent/retried start consumes exactly one credit; zero quota redirects to purchase; resume does not consume again.
- [ ] Second half exactly replays first-half content while storing independent answers and does not announce repetition.
- [ ] Expiry records unanswered as kosong; closed-tab/resume and transition tests cannot reset timers.
- [ ] Client-approved N=10 and N>10 fixtures, zero-attempt/zero-first-half/overflow cases and rounding validate all four scores; detailed review reproduces the exact attempt.

**Decision / verification gate:** D03 scoring is a full-scope completion gate; D05 abandoned quota; D10 transitions. A provisional UI label alone is insufficient.

<a id="p19"></a>

### P19 — Monitoring and violation timelines

**Status:** planned · **Size:** M · **Depends:** P16, P17

**Outcome:** Staff see active tryouts and retained violation history through 30-second polling.

**DB / backend / integration:** Tenant-scoped active-session queries, fullscreen/screenshot event append and historical timeline links.

**Files / frontend:** Violation/monitoring service/repository/handlers; web/app/admin/monitoring; runtime warning overlay; admin/student result links as permitted.

**Interfaces:** Produces persisted tryout-only violation evidence. Monitoring reads database state; events only invalidate derived views.

**Acceptance scenarios:**

- [ ] Starting/completing session appears/disappears within a 30-second refresh; history remains after completion.
- [ ] Fullscreen exit/detectable screenshot shows warning and records timestamp while timer keeps running.
- [ ] Tenant A cannot list B active/finished sessions or timeline by guessed ID.
- [ ] Polling 50 active sessions meets ≤500ms p95 in the documented fixture; kecermatan has no violations or fullscreen enforcement.

**Decision / verification gate:** Fullscreen availability is browser-dependent; test supported desktop/mobile paths and explicitly describe best-effort behavior.

<a id="p20"></a>

### P20 — Asynchronous tryout PDF

**Status:** planned · **Size:** M · **Depends:** P02, P06, P08

**Outcome:** Authorized staff download a printable tryout with rendered math/images and no answers.

**DB / backend / integration:** question_exports, durable render job, original assigned revision/order, bounded renderer, private artifact/status/ready notification.

**Files / frontend:** PDF worker/artifact services/migrations; web tryout builder/list export control and job status/download.

**Interfaces:** Produces admin/operator-only PDF artifacts, isolated by tenant and original definition revision.

**Acceptance scenarios:**

- [ ] PDF has title, subtest name/duration, numbering, original option order, images/math and school/date footer.
- [ ] Inspect render input and output: no keys/explanations; shuffle settings do not change PDF order.
- [ ] Worker restart completes/retries one artifact without cross-tenant download or arbitrary URL/file fetch.
- [ ] Multi-page math/image fixture renders legibly; failure is visible; ready notification arrives.

**Decision / verification gate:** D11 scope/copy alignment. Student and kecermatan PDFs remain unapproved post-MVP/open requests.

<a id="p21"></a>

### P21 — Result dashboards and CSV/XLSX exports

**Status:** planned · **Size:** M · **Depends:** P02, P06, P17, P18

**Outcome:** Staff filter results and download real spreadsheet reports for their tenant.

**DB / backend / integration:** Snapshot/cut-off export requests, durable report worker, protected artifacts and scoped list/detail filters for both test types.

**Files / frontend:** Result export files/migrations; web/app/admin/results and kecermatan-results; shared artifact status view.

**Interfaces:** Produces CSV/XLSX tryout exports required by FR-ADM-RES-04 and complete kecermatan dashboard. Kecermatan spreadsheet export can be added only if separately requested.

**Acceptance scenarios:**

- [ ] Filter tryout/program returns expected rows and IRT flag; kecermatan shows counts/composites/final score/date/quota.
- [ ] CSV/XLSX use same tenant/report cut-off; XLSX numbers are numeric and text cannot execute formulas.
- [ ] Tenant B cannot request/read A report or reuse A download link endpoint; revoked authorization blocks new link issuance.
- [ ] Worker failure/retry leaves one logical report with visible status; large export does not block API request.

**Decision / verification gate:** No additional kecermatan-export feature is inferred from the tryout export requirement.

<a id="p22"></a>

### P22 — Student operations and platform financial recap

**Status:** planned · **Size:** M · **Depends:** P04, P13, P14, P17, P18

**Outcome:** Staff administer agreed student capabilities/grants and Platform Admin sees cross-tenant summaries for manual revenue sharing.

**DB / backend / integration:** Account lifecycle/deactivation, user CRUD, active enrollment view/manual grants, audit, tenant/platform aggregates.

**Files / frontend:** User/admin-student/platform-report services/handlers/migrations; web/admin/users, students and platform dashboard/report pages.

**Interfaces:** Consumes finance/runtime history; produces audited operations, tenant-only revenue reports, platform tenant counts/status and global transaction recap.

**Acceptance scenarios:**

- [ ] Approved Operator versus Admin CRUD matrix holds in UI and direct API; deletion/deactivation preserves financial/history links.
- [ ] Manual grant without order records actor/reason and opens only intended test; enrollment listing reflects exact active access.
- [ ] Tenant Admin sees own orders/revenue; Platform Admin sees counts/value per tenant/period without automated settlement.
- [ ] Support grant/user/financial action keeps actual platform actor; count/report queries are scoped and paginated.

**Decision / verification gate:** D02 (resolved: the Operator may run student operations); D14 deletion/retention. No per-tenant merchant credentials or settlement engine.

<a id="p23"></a>

### P23 — Full-scope certification and release

**Status:** planned · **Size:** XL · **Depends:** P00, P01, P02, P03, P04, P05, P06, P07, P08, P09, P10, P11, P12, P13, P14, P15, P16, P17, P18, P19, P20, P21, P22

**Outcome:** The full platform passes client UAT, isolation/financial/runtime checks and the named production-capacity/restore gates.

**DB / backend / integration:** Staging/production configuration, trusted host routing/TLS, resource sizing, provider verification, runbooks, backups, metrics/alerts, migration and rollback rehearsal.

**Files / frontend:** Deployment/operational docs and configuration; CI integration/browser/load checks; scoped defect fixes only.

**Interfaces:** Consumes every complete slice and resolved release decision. Produces signed-off evidence, actual deployment versions/resources and a newly agreed launch date.

**Acceptance scenarios:**

- [ ] Run every numbered FR acceptance plus question/scoring/NFR cases from RECONCILIATION; all full-scope release gates pass.
- [ ] 1,000 aggregate CCU workload with 10,000 registered-user dataset meets non-exam ≤500ms p95, question load ≤1s and deadline lag ±5s; publish workload/error thresholds/resources.
- [ ] Restore daily-backup fixture and private asset references into isolated environment; prove 30-day retention and approved RPO/RTO recovery steps.
- [ ] Test cross-tenant adversarial paths, real notification channels, merchant sandbox-to-production configuration, rate limits under legitimate exam traffic and incompatible-version rollback risks.

**Decision / verification gate:** No 3 August 2026 promise or old RAB is reused; provider stubs/provisional scoring/mock data cannot pass full-scope UAT.

## 5. Full-scope requirement coverage

[RECONCILIATION.md §4](RECONCILIATION.md#4-requirement-traceability) contains one row for each of the 161 FR IDs, each linked to an architecture section and owning slice. Its additional rows cover question types, all scoring modes, the PRD schema/event/cache conventions and all non-functional requirements. Multiple slices can jointly satisfy one requirement; no slice can claim a feature complete because its UI exists.

## 6. Integration acquisition and release dependencies

| Dependency | First needed | Completion evidence |
|---|---|---|
| Email/WhatsApp provider and templates | P02/P03 | Real staging delivery, failure/retry/expiry behavior and credential/log handling |
| Domain and trusted origin/TLS | P11 | One production domain with valid TLS (D17) |
| Storage credentials/private buckets | P06/P09/P20/P21 | Authorized uploads/downloads and inaccessible foreign/private objects |
| Platform-owned Midtrans merchant | P14 | Sandbox success/failure/duplicate/uncertain outcomes, then reviewed production configuration |
| Emoji catalog/assets/font license | P09 | Visual consistency and license evidence |
| Client grading fixtures/policy | P16/P17/P18 | Approved exact expected outputs and edge-case policy |
| Infrastructure budget/staffing | P23 scheduling | Reviewed resource/load results, assigned ownership, new calendar and cost |

Start provider/domain acquisition alongside foundation work. Exact providers, costs, package versions and machine sizes are not invented by this plan.

## 7. Milestones and release sign-off

| Milestone | Slices | Exit evidence |
|---|---|---|
| M1 Foundation | P00–P04, P01b | One configured tenant, real auth, durable real provider delivery |
| M2 Both content formats | P05–P10 | Master/copy/version invariants and deterministic generator fixtures |
| M3 All commerce paths | P11–P15 | Real catalog/landing, three payments, both top-up paths, ledger and grants |
| M4 Both runtimes/results | P16–P18 | Authoritative expiry/recovery and approved grading/ranking |
| M5 Complete operations | P19–P22 | Monitoring, PDF, spreadsheet reports, student grants and platform recap |
| M6 Full-scope production readiness | P23 | All FRs/NFRs, client UAT, restore/security/load evidence and operating ownership |

Milestones group sign-off; independent later-slice work can start when its actual predecessors pass. A catalog-only or wallet-only release does not count as full-scope completion. No new launch date or revised RAB is promised until scope decisions, staffing, provider onboarding and measured capacity are reviewed.

## 8. Decision gates and handoff

[ARCHITECTURE.md §9](ARCHITECTURE.md#9-risks--open-questions) owns D01–D17. The key early blockers are OTP/phone-only recovery, Operator capabilities, scoring ownership/normalization, domain ownership and resource bounds. Finance/session policies and confirmed kecermatan formulas gate their own slices; they do not prevent review of independent architecture sections.

This plan is ready for architecture/scope review, not automatic execution or production deployment. Preserve the documented decisions and evidence when turning each slice into its implementation checklist. The original plan is recoverable with `git show 4f2246a:requirement/PLAN.md`.
