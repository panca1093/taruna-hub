# Student Workspace Redesign Implementation Plan

**Goal:** Menerapkan workspace siswa full width dengan navigasi sederhana dan flow Tryout, Kecermatan, cart, pembayaran serta topup yang tetap mengikuti backend otoritatif.

**Architecture:** Reshape halaman Next yang sudah ada, reuse runtime hooks dan typed API. Prototype bundled hanya menentukan arah visual/journey; tidak menambah business policy. Deliver per paket/slice, pertahankan route lama, dan pisahkan kemampuan akun/commerce yang memerlukan kontrak baru.

**Tech Stack:** Go 1.26, Echo, pgx, PostgreSQL; Next.js 16, React 19, HeroUI v3, Tailwind v4, generated OpenAPI client.

**Spec:** [Student workspace handoff](HANDOFF.md)

## Global Constraints

- Dashboard, Tryout, Katalog adalah tiga navigasi utama; akun membuka halaman; saldo ringkas boleh popup, isi saldo wajib full page.
- Owner memilih rose hangat untuk auth, siswa dan admin/operator pada 31 Oktober 2026: ikuti [PALETTE-DECISION](PALETTE-DECISION.md). Selaraskan canvas/sidebar/kartu/teks/CTA pada kedua workspace; token/pola berubah bersama `globals.css`, komponen dan DESIGN. Rebrand marketing dan pembangunan seluruh modul admin tetap di luar scope otomatis.
- Gunakan typed client `web/lib/api/`, ID/EN dictionary, shared UI dan guide Next terpasang. PRD read-only. Kontrak baru OpenAPI-first.
- Scoring, auth, money, quota dan deadlines tetap server; tidak ada fake data, simulation control atau OTP di log/UI production.
- Pertahankan local full-width diff, fitur admin/public, katalog emoji/shape terbaru dan URL yang sudah dipakai.
- Satu product/order. Profil readonly sampai mutation policy/contract disepakati. D03 tetap provisional; tidak ada policy refund baru.
- Kerjakan satu paket sebelum paket berikutnya. Update PLAN per slice yang dibuktikan, jangan menandai seluruh P14/P16/P18 selesai dari UI demo.
- Commit/push/merge mengikuti instruksi owner, bukan otomatis dari checklist ini. Author user tanpa atribusi agent.

## Review Focus

1. Jawaban terkirim saat deadline/reload/offline: label saved hanya sesudah ACK, deadline tidak mundur (Task 2/3).
2. Attempt lama atau pembahasan ditahan: URL memilih session tepat dan tidak membocorkan keys (Task 2).
3. Payment create timeout/duplicate/late callback: entity identity sama, credit/debit/grant sekali dan held tidak memberi akses (Task 4/5).
4. Pengguna berganti akun atau produk/harga berubah: cart/data tidak bocor dan canonical API menentukan checkout (Task 1/4).
5. Shared shell pada 320/2560px, locale panjang dan keyboard: flow siswa tetap terbaca dan admin/public tetap berfungsi (Task 1/6).

---

## Baseline dan batas file

Jalankan prototype portabel sesuai spec. Semua path di bawah relatif root repo. File baru adalah tujuan yang diusulkan; sebelum menambah komponen cek shared UI existing. Jangan memecah runtime/service sekadar untuk kebutuhan visual.

| Paket / slice | File existing utama | File baru yang diperlukan |
|---|---|---|
| 1 / P03,P17 | `web/components/app-shell.tsx`, `web/lib/nav.ts`, `web/app/student/page.tsx`, `web/app/globals.css`, `requirement/DESIGN.md` | Komponen student shell/account menu hanya jika shared shell membutuhkan isolasi; dictionary sesuai pola repo |
| 2 / P16,P17,P19 | `web/app/student/tryouts/page.tsx`, `web/app/student/tryout/[id]/page.tsx`, `subtest/[subtestId]/page.tsx`, `result/page.tsx`, `web/lib/tryout-session.ts` | Tidak perlu backend baru untuk selected attempt |
| 3 / P18 | `web/app/student/kecermatan/[id]/page.tsx`, `attempt/page.tsx`, `result/page.tsx`, `web/components/kecermatan-item.tsx` | Reuse renderer dan runtime existing |
| 4 / P11,P13,P14 | `web/app/catalog/page.tsx`, `[id]/page.tsx`, `web/app/checkout/[id]/page.tsx`, `web/app/student/orders/page.tsx` | `web/lib/student-cart.ts`, `web/app/student/cart/page.tsx`, `web/app/student/orders/[id]/page.tsx` |
| 5 / P12,P15,P14 | `web/app/student/wallet/page.tsx`, account balance shortcut | `web/app/student/wallet/topup/page.tsx`, `web/app/student/wallet/topups/[id]/page.tsx` |
| 6 / P03,cross-cutting | `web/app/student/profile/page.tsx`, login/register/verify/recovery existing, locale/theme context | Contract work only after separate decision; no invented profile endpoint |

Each task also changes its `web/lib/i18n/dict/<feature>.ts`, relevant `docs/<domain>.md`, DESIGN patterns and PLAN acceptance only as warranted. Final exact component extraction is implementer judgment; no generic redesign framework needed.

## Task 1 — Shell dan dashboard konsisten (G11,G12)

**Consumes:** Auth session/role existing; `api.studentDashboard()`; server wallet and enrollment/runtime summaries. **Produces:** Same student shell on login/register, stable three-route nav and account destinations; admin shell retains existing contract.

- [ ] Refresh `git status`, HEAD/main and diff. Record divergence; preserve the two full-width edits or reconcile equivalent upstream changes. Inspect patch before applying. Create isolated execution branch/worktree without losing dirty work.
- [ ] Read source-of-truth UI/auth docs and matching installed Next guide. Confirm owner-authorized package boundary; record the owner-selected warm rose palette from PALETTE-DECISION and the remaining non-palette gates from spec.
- [ ] Record before-change browser cases: same viewport after login/register, verified/unverified, loading/error/new/active, average score exactly 0 vs null, 1920/2560px content bounds. This establishes which layout failures actually exist.
- [ ] Reshape `app-shell.tsx`/`nav.ts` for student nav Dashboard→`/student`, Tryout→`/student/tryouts`, Katalog→`/catalog`. Put account destinations in keyboard-accessible dropdown; keep logout/session behavior. Mobile has the same three destinations. Do not remove admin entries globally.
- [ ] Map shared workspace semantic tokens to the existing warm rose reference in PALETTE-DECISION. Align auth/student/admin shell surfaces and states together, including the dark plum sidebar; retain role/navigation behavior and marketing scope. Compare same-viewport screenshots across both workspaces in light/dark; existing blue admin screenshots are historical flow evidence. Update globals/components/DESIGN together.
- [ ] Update dashboard layout using trusted summaries and full available width. Keep data-driven empty/unverified states and invalidation after purchase/result; no fabricated normal/Kecermatan combined metrics. Update tokens/pattern docs and ID/EN strings together.
- [ ] Verify login and register render identical component geometry for equivalent data; data differs legitimately. Verify 320/390/768/1024/1440/1920/2560px, expanded/collapsed rail, both locales/themes, keyboard menu focus/Escape and reduced motion. Switch accounts: no old balances/identity/results survive. Check admin/public shell behavior regression; expect the approved rose restyling of the admin shell, with its role/navigation intact.
- [ ] Run web typecheck/lint and required build sequentially with dev server stopped. Review diff and record evidence/slice status before proceeding.

## Task 2 — Tryout normal sampai history/review (G01,G03)

**Consumes:** Existing `api.runtime` methods and `useServerClock`/`useAnswerSync`. **Produces:** Detail/readiness/runtime/full-page submit/transition/result/history with exact session identity, no scoring or deadline changes.

- [ ] Inspect runtime docs, current hooks and `api/internal/service/runtime_test.go`, `runtime_score_test.go`, `result_test.go`. Establish fixtures for two completed attempts with different scores, one active subtest and held/released explanations. Use existing dev/test provisioning, not production identities.
- [ ] Reproduce history limitation: select older session A when B is latest; current result page loads B. Define acceptance: `?attempt=A` calls `api.runtime.result(A)`/review A, foreign session is denied, no query retains legacy latest behavior. If automated coverage infrastructure is present, encode this regression there; otherwise keep explicit reproducible browser evidence. Do not add a test framework solely for layout.
- [ ] Implement session selection in `web/app/student/tryout/[id]/result/page.tsx` and history links in overview/hub. Handle unavailable/foreign/incomplete attempt with honest error/navigation; backend remains authorization source.
- [ ] Reshape overview, per-subtest route and result using current runtime. Four question types, LaTex/image assets, navigation, start/retake/resume and progress remain intact. Content confirmation and next-subtest transition use full-page states; required fullscreen/security guards stay enforced.
- [ ] Keep answer pending/ACK/retry/stale_answer behavior. Never mark saved optimistically; retain unsent recovery. Disable duplicate submit while resolving but reconcile unknown responses through same session. Preserve start/submit semantics and server clock/extension behavior.
- [ ] Verify answer then reload, network loss then reconnect before/after deadline, duplicate submit, expired access, two tabs, timeout with blanks, locked previous subtest, fullscreen denied/exit and staff extension. No clock reset, double result or silent loss labeled saved. Run existing runtime/result tests if behavior changed.
- [ ] Verify A/B historical selection, score 0, held review (including 409 release race), released review and ranking tie display. No key/answer appears in payload/UI before permitted. Verify back/deep link/reload and responsive exam at all widths from Task 1. Record real browser→API→DB completion, not demo score.

## Task 3 — Kecermatan runtime dan hasil (G02)

**Consumes:** `api.kecermatanRuntime`, existing item renderer and server phases/credits. **Produces:** Equally discoverable hub tab, readiness, responsive column UI, recovery, result/review/history for exact attempt.

- [ ] Read Kecermatan runtime/item-package docs, D03 and existing `kecermatan_runtime_test.go`, `kecermatan_score_test.go`, `kecermatan_order_test.go`. Prepare fixtures for zero credits, existing active session, char/image/emoji/solid-shape/mix and a non-demo N/M/option count.
- [ ] Capture baseline start/resume/answer/pause behavior and bounded retry. Define failing or missing UI cases: more than five choices clipped; retry exhaustion falsely appears saved; reload pause restarts time. Add a focused automated regression only if runtime logic is changed, using existing harness.
- [ ] Reshape hub/detail/readiness/attempt/result files, keeping API-driven references/options, media renderer and configured item length. Do not reuse normal fullscreen/violation controls or hardcode A–E/10×50.
- [ ] Preserve write-once answer/skip, server-driven column/pause, five-second transition with early continue, retry/refetch on stale/locked response and completion navigation. Make unresolved send state honest; no local deadline override, early finish or prototype accelerate-time control.
- [ ] Verify zero quota blocks server start; repeated start returns active session; duplicate answer cannot overwrite; retry after column expiry reconciles; reload/tab closure in column and pause preserves server timeline. Worker completion yields one result and one consumed credit. Repeat purchase adds credits to the same enrollment.
- [ ] Verify counts/result/version from server, selected attempt history, review column/pagination and all media types at mobile/desktop. Record DB-backed credit/result identity; note D03 remains open, even when provisional score matches fixture.

## Task 4 — Katalog, cart, checkout dan order status (G04,G05,G06,G08)

**Consumes:** `api.catalog`, `api.orders`, `api.assets.uploadProof`, generated body/status types. **Produces:** Account-scoped draft cart and one-product checkout with reopenable status at `/student/orders/:id`.

- [ ] Read products/orders/payments-gateway/assets docs and schemas. Inspect existing `order_test.go`, `payment_test.go`, `payment_concurrent_test.go`, `payment_review_test.go`; preserve semantics, not prototype status names.
- [ ] Define cart cases before implementation: account A draft invisible to B; paying item X leaves Y; removed/changed-price/unavailable product cannot use stale cached price. Store only draft product IDs under authenticated user scope; canonical product is fetched again. No guest-cart merge policy invented.
- [ ] Implement draft helper/page and signed-in catalog affordance; public catalog remains accessible. Cart chooses one product then `/checkout/:productId`; individual/bundle/Kecermatan product shapes use API. No combined-order endpoint or discount/tax logic.
- [ ] Move checkout/detail/status layout to full pages. For manual, bank instructions + upload proof come before `orders.checkout`; supply server-issued proof ID. Wallet and gateway retain current body/idempotency contracts. Same logical retry reuses key/payload; new intentional transaction gets a new identity only after reconciliation.
- [ ] Add reopenable order status page; list links to exact order ID. Reuse current polling rules; continue existing `redirect_url` rather than create duplicate pending order. Status refresh after gateway return is server-authoritative. Map order pending/confirmed/cancelled separately from payment exception; do not invent `Order.status=failed/review`.
- [ ] Verify changed price, already-owned individual, partially-owned bundle, sale window, order_pending, insufficient balance, unconfigured bank/gateway, rejected upload and create timeout. Duplicate callbacks/requests cannot duplicate debit/grant. Existing pending order can be continued after reload; cancel/late success held grants nothing. Provider failure need not mean cancelled business order.
- [ ] Run wallet/manual/browser→API→DB purchase for individual/bundle/Kecermatan, then open runtime/access from confirmed status. Check cart item removal and same-account dashboard refresh. Real Midtrans sandbox remains a separate P14 gate; record explicitly if unavailable.

## Task 5 — Saldo popup, full-page topup dan transaksi (G05,G06,G07)

**Consumes:** `api.wallet`, `api.topups`, proof upload, same payment infrastructure. **Produces:** Quick balance popup, full topup create/status pages and ledger navigation; no content stack of modals.

- [ ] Inspect wallet/topups docs, current page and `topup_test.go`, `topup_proof_test.go`, `wallet_test.go`. Capture existing `min_topup`, exact numeric formatting and rejection reason handling.
- [ ] Define acceptance with a changed Admin minimum: amount below current minimum is refused, whole-rupiah strings stay exact, fractional/malformed/zero input rejected. Pending leaves balance unchanged; approved credits once; rejected shows reason; wallet is not a topup payment method.
- [ ] Keep quick balance in shell popup; Isi saldo closes it and navigates to `/student/wallet/topup`. Preserve `/student/wallet` as ledger/transaction destination. Create exact topup status route `/student/wallet/topups/:id` and link history rows there.
- [ ] Move create flow from modal into page, retaining manual proof-before-create, gateway availability, minimum from server and idempotency identity. Reuse API money helpers and server statuses; expose continue payment/refresh/cancel only when contract permits.
- [ ] Verify popup→page has no lingering dialog; Back/reload opens same transaction. Exercise manual upload failure/pending/approved/rejected, gateway pending/unknown/return/held and duplicated create/confirmation. Balance credit once with traceable ledger source; no optimistic cash or grants from return URL.
- [ ] Verify API-backed balance refresh in header/dashboard after approval and pagination/error/loading in ledger/history. Run relevant existing money/integration tests if changed, and record browser→API→DB evidence. Do not claim provider sandbox from local fake adapter.

## Task 6 — Akun, entry flow dan final acceptance (G09,G10,G11,G12)

**Consumes:** Existing auth/session/verify/recovery, locale/theme contexts and Tasks 1–5. **Produces:** Honest account pages, safe entry intent and documented end-to-end evidence, with unresolved product gates retained.

- [ ] Inspect `web/app/student/profile/page.tsx`: save/password currently toast-only, programs mock. Remove false success affordance from redesigned baseline; show server profile readonly and link to existing recovery/verification. Do not call an endpoint absent from OpenAPI. List writable fields/policy as deferred for owner; if separately approved, write a dedicated contract-first plan rather than expand this task silently.
- [ ] Trace anonymous catalog→checkout→login/register and verify optional-email route. Preserve existing safe return if implemented; otherwise retain only allowlisted internal student/product intent through auth. Test external/scheme-relative redirect rejected and staff `/portal` unaffected. Never persist OTP or credentials in cart/return state.
- [ ] Add remaining ID/EN copy using existing dictionaries, real loading/empty/error/retry/partial states and same-user invalidation. Check new/active/unverified/expired accounts and network errors; fake zero/profile saves/simulation controls must not remain.
- [ ] Execute spec acceptance across keyboard/focus/back/deep links, locale, themes, reduced motion, zoom, contrast and viewport 320–2560px. Review full user journeys, normal and Kecermatan runtimes, popup balance only, plus existing admin/public shell. Record limits where accessibility or provider testing remains incomplete.
- [ ] Stop dev server before Next build. Run repo Definition of done in order (commands below); resolve relevant failures, record unrelated/environment blockers precisely, and do not report skipped checks as passing.
- [ ] Update domain docs, DESIGN and PLAN status/acceptance boxes for only proven work. Review actual diff for API schema drift, secrets, grading keys, demo controls and unrelated changes. Provide owner with changed scope, verification evidence and remaining gates before any requested shipping step.

## Required verification commands

From the stated directories, one phase at a time. Read local Makefile/environment setup before integration tests; no destructive data reset against non-dev databases.

```bash
# api/
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec
make build vet test
```

```bash
# web/
npm run gen:api
git diff --exit-code lib/api/schema.ts
npx tsc --noEmit
npm run lint
npm run build
```

Use real dev infrastructure and fixtures for browser→API→DB demos; frontend-only simulation cannot close acceptance. Never run build alongside dev server against the same `.next`. Existing Go tests are regressions, not substitutes for the new browser geometry/journey evidence.

## Plan self-review and completion record

Coverage: G01/G03→Task2, G02→Task3, G04/G05/G06/G08→Task4, G05/G06/G07→Task5, G09/G10→Task6, G11/G12→Task1+6. Each Review Focus item has explicit acceptance in its owning task. No additional API endpoints assumed except a separately approved profile mutation package. No provider or D03 sign-off invented. Palette-family choice is owner-authorized warm rose; visual implementation/contrast acceptance is still pending, and admin module scope remains separate.

Executor records per task: baseline HEAD, scope, checks with outputs, browser/API/DB evidence, files changed, accepted/deferred gates and commit/PR if separately requested. Stop at the authorized package boundary; incomplete gates remain visible.
