# PRD / Plan Reconciliation and Requirement Traceability

**Date:** 2026-10-09 · **Status:** complete coverage draft; business decisions remain open
**Sources:** [PRD v1.3](PRD.md), original plan v1.0 at Git `4f2246a`, current implementation at the same head.
**Outputs:** [System architecture](ARCHITECTURE.md), [delivery plan v2.0](PLAN.md).

## 1. Scope and precedence

The user authorized steps 1–3 for the **full scope**, not a reduced first release. All 161 numbered FR definitions and the unnumbered requirements of PRD §§5.5, 5.6, 6, 7 and 8 are covered. Full coverage means a design and owning verification slice exist; it does not mean implementation or unresolved policy is complete.

The PRD defines required product behavior. The old plan's explicit recorded owner choices remain evidence (notably OTP) and are surfaced where they conflict. The architecture proposes implementation details; it cannot silently change product policy. Keep PRD unchanged and record owner resolutions against D01–D17 before executing dependent work.

The source PRD's post-MVP roadmap remains outside this release: predictive analytics, automated revenue-share settlement, tenant-specific merchant credentials and student PDF downloads. Kecermatan PDF is an unapproved open question. One owner decision defers MVP scope: [D17](ARCHITECTURE.md#decision-register) moves multi-tenancy (FR-TEN-01..19, FR-AUTH-06..08) to post-MVP. Their rows below keep the post-MVP design; the MVP effect per slice is in [PLAN §1.1](PLAN.md#11-mvp-tenancy-override-d17-2026-10-28).

## 2. Reconciliation findings

| Finding | Evidence / consequence | Treatment |
|---|---|---|
| Plan targets PRD v1.2 | Header, old 14-slice schedule and cost predate v1.3 | Replace with full-scope P00–P23; no old launch/cost promise |
| Backend is no longer zero | Current API/schema/CI exists, although only walking skeleton | Mark P00 partial; keep real integration verification incomplete |
| Historical UI gap claims are stale | Demo store, wallet screen and shuffle type fields exist now | Preserve useful UI; use current code baseline instead of repeating zero-percent claims |
| Tenant model missing in old plan/code | FR-TEN-01..19 and AUTH-06..08 require it everywhere | Tenancy moves before real identity/content/finance; include jobs/cache/assets/support. **Superseded by D17:** multi-tenancy is post-MVP; the schema keeps `tenant_id` |
| OTP conflict | PRD §7 says no OTP; old plan records owner approval | Retain planned OTP and gate exact policy/channel under D01 |
| Auth/notification circular dependency | Old S1 depends on S2 while S2 depends on S1 | P02 transport depends on P00; P03 consumes it; domain templates come with domain slices |
| Operator privileges contradict | PRD §4 denies user administration; FR-ADM-STU-01 grants CRUD; package section says both Operator and Admin | Explicit D02 capability matrix; no silent privilege expansion |
| Self-service top-ups omitted | Old S6 covers W01..03, new PRD extends W04..09 | P12 ledger, P15 manual requests/review, P14 gateway credits and minimum setting |
| Wallet source inconsistent | Old plan says derive from ledger; PRD stores users.balance and balance_after | Ledger authoritative audit plus atomic locked projection; D12 precision/negative policy |
| PDF missing | FR-ADM-TRY-08..13 not in old S8 | P20 covers async original-order, math/images, no keys, tenant/date footer and ready link |
| Kecermatan under-scoped | Old S12 stops at KEC14 / ADM-KEC05 | P09 packages, P10 builder/generator, P18 runtime/results cover all 19 +24 requirements |
| Emoji schema inconsistent | ADM-KEC15/24 supports emoji; §8 package enum omits it | Include system-ID emoji packages/assets in target model; no free-text emoji |
| Kecermatan secrecy AC impossible | Old S12 forbids reference set in response, but student needs it to solve | Show current authorized reference/subset; withhold missing label, future snapshots and correctness |
| Kecermatan formula inconsistent | N>10 repeated halves differ from first/last five; division by zero and >100 possible | D03 confirms formula/range/rounding; no unapproved clamp or restriction to N=10 |
| Repeated purchase audit ambiguous | FR-KEC08 says no separate transaction; purchase still needs financial history | Separate payment/order records, aggregate credits on same enrollment with unique source grants |
| Master edit would alter exams | FR-TEN12 requires no propagation to copied/assigned content | Immutable assigned revisions and session snapshots; master updates create new versions |
| Question import AC contradictory | Old S7 mixes valid-row import and no partial import | D08 defines confirmed accepted set versus entire-file rejection; selected commit atomic |
| Landing/deployment domain conflict | Old split-deploy/tryout host contradicts tenant slug apps | One host-aware build proposed; root/platform/admin/first-tenant DNS details are D06 |
| Landing overpromises PDF | FR-LAND03 targets students; PDFs explicitly staff-only | D11 copy/scope decision; no student downloads added automatically |
| IRT and ranking terminology misleading | IRT is weighted prevalidation; ranking must use all completed attempts | Versioned raw scoring and tenant-local averages; D04 confirms counting/ties/weight ownership |
| Redis event loss could leave incorrect rank | PRD permits lossy in-memory updates | PostgreSQL authoritative averages plus revision-aware rebuild/fallback |
| Expiry and offline policy incomplete | Test/enrollment/part windows and closed-tab transitions conflict at boundaries | D10 settles precedence; server deadlines remain authoritative |
| Provider placeholders could look complete | Old plan proposes WhatsApp no-op to unblock work | Unconfigured state explicit; real channel delivery required for full-scope release |
| CI omits lint and test coverage is narrow | Current lint fails; only error-handler tests exist | P00 enforces lint; per-slice behavioral/integration/browser verification |
| Calendar/budget is obsolete | Original August deadline is historical; v1.3 itself calls for repricing | Relative sizing only; new calendar/cost depends on staffing/provider decisions and certification |
| Student profile self-edit (2026-10-31) | PRD §4 gives students no profile-write FR; the workspace handoff gate G09 left it open | Owner approved student self-edit of name and phone only; email stays admin-managed; password change needs the current password and revokes other sessions. `PATCH /api/v1/me` is student-only. PRD unchanged |

## 3. Original slice migration

| Original | Revised ownership |
|---|---|
| S0 Skeleton | P00 foundation, P01 tenancy, frontend data seam with each integrated domain |
| S1 Accounts | P03 identity; P04 first Admin/support; P22 staff account operations |
| S2 Notifications | P02 transport; event templates attached to owning slices |
| S3 Programs | P05 master/tenant curriculum |
| S4 Landing | P04 branding/routing and P11 real landing/catalog |
| S5 Products | P11 all three product types and catalog |
| S6 Commerce | P12 ledger, P13 wallet/manual orders/access, P14 gateway, P15 manual top-ups |
| S7 Questions | P06 assets, P07 bank/imports/immutable revisions |
| S8 Tryout builder | P08 definition/assignments; P20 PDF |
| S9 Tryout runtime | P16 runtime/basic score; P19 deterrence/timeline |
| S10 Results | P17 review/ranking; P21 spreadsheet reports |
| S11 Monitoring | P19 monitoring |
| S12 Kecermatan | P09 packages, P10 definitions/generator, P18 runtime/results; P11/P13/P14 commerce |
| S13 Release | P23 full-scope certification |
| New platform scope | P01/P04 tenancy/support; P05/P07/P09 master content; P22 global recap |

## 4. Requirement traceability

Each row is a distinct **definition** from PRD §5, not a loose ID reference in prose. The concise label does not replace the source's detailed behavior; read its full definition and any continuation bullets (especially item uploads). Owning slices supply concrete acceptance scenarios. A dash under decisions means no separately identified business-policy gate, not that testing is optional.

| Requirement | Required capability | Architecture | Delivery | Policy gate |
|---|---|---|---|---|
| FR-AUTH-01 | Email or phone registration | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P03](PLAN.md#p03) | D01 |
| FR-AUTH-02 | Email or phone login | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P03](PLAN.md#p03) | D01 |
| FR-AUTH-03 | Email password recovery | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P03](PLAN.md#p03) | D01 |
| FR-AUTH-04 | JWT and idle logout | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P03](PLAN.md#p03) | D13 |
| FR-AUTH-05 | Admin user account management | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P03](PLAN.md#p03) | D02 |
| FR-AUTH-06 | Tenant determined by subdomain | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P01](PLAN.md#p01), [P03](PLAN.md#p03) | — |
| FR-AUTH-07 | Identifier uniqueness within tenant | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P01](PLAN.md#p01), [P03](PLAN.md#p03) | — |
| FR-AUTH-08 | JWT and host tenant must match | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P01](PLAN.md#p01), [P03](PLAN.md#p03) | — |
| FR-CAT-01 | Tenant-only active catalog | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | D06 |
| FR-CAT-02 | Filter by program | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-CAT-03 | Individual, bundle and metered products | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-CAT-04 | Product detail and pricing/window | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-CAT-05 | Purchase action enters checkout | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-PAY-A-01 | Midtrans payment action | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P14](PLAN.md#p14) | D07, D15, D16 |
| FR-PAY-A-02 | Verified webhook confirms and grants | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P14](PLAN.md#p14) | D07, D15, D16 |
| FR-PAY-A-03 | Successful-payment email and WhatsApp | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P14](PLAN.md#p14), [P02](PLAN.md#p02) | D07, D15, D16 |
| FR-PAY-C-01 | Pending manual purchase | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | — |
| FR-PAY-C-02 | Student payment-proof upload | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13), [P06](PLAN.md#p06) | — |
| FR-PAY-C-03 | Staff confirmation grants access | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | — |
| FR-PAY-C-04 | Staff cancellation | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | D07 |
| FR-PAY-W-01 | Three top-up sources | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P12](PLAN.md#p12), [P14](PLAN.md#p14), [P15](PLAN.md#p15) | D12 |
| FR-PAY-W-02 | Wallet payment confirms and debits | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | D12 |
| FR-PAY-W-03 | Running-balance ledger and references | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P12](PLAN.md#p12) | D12 |
| FR-PAY-W-04 | Gateway self top-up | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P14](PLAN.md#p14) | D07, D12 |
| FR-PAY-W-05 | Manual self top-up and rejection | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P15](PLAN.md#p15) | D12 |
| FR-PAY-W-06 | Staff signed adjustment with reason | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P12](PLAN.md#p12) | D12 |
| FR-PAY-W-07 | Manual top-up staff queue and proofs | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P15](PLAN.md#p15) | D12 |
| FR-PAY-W-08 | Money movement actor audit | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P12](PLAN.md#p12) | D12 |
| FR-PAY-W-09 | Per-tenant minimum top-up | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P04](PLAN.md#p04), [P15](PLAN.md#p15) | D12 |
| FR-PAY-01 | Own order history | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | — |
| FR-PAY-02 | Access until enrollment deadline | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13), [P16](PLAN.md#p16) | D05, D10 |
| FR-TRY-01 | Owned tryout list | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-02 | Subtest overview and durations | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-03 | Start first subtest timer | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | D10 |
| FR-TRY-04 | Free question navigation within subtest | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-05 | No return to submitted subtest | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-06 | Deadline auto-submit | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | D10 |
| FR-TRY-07 | Immediate result on completion | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-08 | Repeat attempts and history | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16), [P17](PLAN.md#p17) | — |
| FR-TRY-09 | Resume without pausing deadline | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | D10 |
| FR-TRY-10 | Fullscreen entry | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P19](PLAN.md#p19) | — |
| FR-TRY-11 | Fullscreen exit warning and log | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P19](PLAN.md#p19) | — |
| FR-TRY-12 | Active date-window enforcement | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | D10 |
| FR-TRY-13 | Best-effort screenshot warning and log | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P19](PLAN.md#p19) | — |
| FR-TRY-14 | Stable per-attempt question permutation | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-TRY-15 | Stable options with content-bound points | [§4.6](ARCHITECTURE.md#46-tryout-runtime) | [P16](PLAN.md#p16) | — |
| FR-RES-01 | Total/subtest scores and rank | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P16](PLAN.md#p16), [P17](PLAN.md#p17) | D04 |
| FR-RES-02 | Complete attempt history | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17) | — |
| FR-RES-03 | Server-gated explanation release | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17) | — |
| FR-RES-04 | Internal tenant ranking | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17) | D04 |
| FR-RES-05 | Average of all attempts leaderboard | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17) | D04 |
| FR-NOT-01 | Order confirmation email by payment state | [§4.9](ARCHITECTURE.md#49-durable-jobs-and-notifications) | [P02](PLAN.md#p02), [P13](PLAN.md#p13), [P14](PLAN.md#p14) | D16 |
| FR-NOT-02 | Access-open email | [§4.9](ARCHITECTURE.md#49-durable-jobs-and-notifications) | [P02](PLAN.md#p02), [P13](PLAN.md#p13), [P14](PLAN.md#p14) | D16 |
| FR-NOT-03 | Password reset email | [§4.9](ARCHITECTURE.md#49-durable-jobs-and-notifications) | [P02](PLAN.md#p02), [P03](PLAN.md#p03) | D16 |
| FR-NOT-04 | Order/access WhatsApp | [§4.9](ARCHITECTURE.md#49-durable-jobs-and-notifications) | [P02](PLAN.md#p02), [P13](PLAN.md#p13), [P14](PLAN.md#p14) | D16 |
| FR-ADM-PRG-01 | Admin-only program CRUD | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P05](PLAN.md#p05) | — |
| FR-ADM-PRG-02 | Subject/chapter hierarchy | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P05](PLAN.md#p05) | — |
| FR-ADM-SOA-01 | Typed content/options/chapter/image editor | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | D04 |
| FR-ADM-SOA-02 | Live math rendering | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | — |
| FR-ADM-SOA-03 | Program/type/difficulty filters | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | — |
| FR-ADM-SOA-04 | XLSX/CSV/DOCX import and templates | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | D08 |
| FR-ADM-SOA-05 | Rendered question preview | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | — |
| FR-ADM-SOA-06 | Visible unique question ID | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07) | — |
| FR-ADM-TRY-01 | Definition, program, score and date window | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | D04 |
| FR-ADM-TRY-02 | Ordered subtests and durations | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-03 | Ordered question assignment | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-04 | Edit/delete locked after first attempt | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-05 | Explanation release control | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-06 | Question randomization setting | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-07 | Option randomization setting | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08) | — |
| FR-ADM-TRY-08 | Staff-only whole-tryout PDF | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | D11 |
| FR-ADM-TRY-09 | Original-order PDF contents | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | — |
| FR-ADM-TRY-10 | No keys or explanations in PDF | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | — |
| FR-ADM-TRY-11 | Rendered math in PDF | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | — |
| FR-ADM-TRY-12 | Async PDF and ready notification/link | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | — |
| FR-ADM-TRY-13 | Tenant identity and print-date footer | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P20](PLAN.md#p20) | — |
| FR-ADM-PRD-01 | Individual product definition | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-ADM-PRD-02 | Bundle product definition | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-ADM-PRD-03 | Catalog activation toggle | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-ADM-ORD-01 | Order lists and status filter | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | — |
| FR-ADM-ORD-02 | Confirm/cancel pending order | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | D07 |
| FR-ADM-ORD-03 | Authorized payment-proof review | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P13](PLAN.md#p13) | — |
| FR-ADM-STU-01 | Student CRUD with reconciled privileges | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P22](PLAN.md#p22), [P03](PLAN.md#p03) | D02, D14 |
| FR-ADM-STU-02 | Active enrollment view | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P22](PLAN.md#p22), [P13](PLAN.md#p13) | D05 |
| FR-ADM-STU-03 | Audited manual grant without order | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P22](PLAN.md#p22), [P13](PLAN.md#p13) | D05 |
| FR-ADM-RES-01 | Per-tryout student score/rank/time | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17), [P21](PLAN.md#p21) | D04 |
| FR-ADM-RES-02 | Test/program result filters | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17), [P21](PLAN.md#p21) | — |
| FR-ADM-RES-03 | IRT prevalidation indicator | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17), [P21](PLAN.md#p21) | D04 |
| FR-ADM-RES-04 | Excel and CSV exports | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P21](PLAN.md#p21) | — |
| FR-MON-01 | Active tryout student/subtest overview | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P19](PLAN.md#p19) | — |
| FR-MON-02 | Thirty-second polling | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P19](PLAN.md#p19) | — |
| FR-MON-03 | Session violation timeline/count | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P19](PLAN.md#p19) | — |
| FR-MON-04 | Completed-session violation history | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P19](PLAN.md#p19) | — |
| FR-KEC-01 | Shared timer per column | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D10 |
| FR-KEC-02 | Stable reference set per column | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10), [P18](PLAN.md#p18) | — |
| FR-KEC-03 | Exactly one missing item with shuffled subset | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10), [P18](PLAN.md#p18) | — |
| FR-KEC-04 | Unanswered is kosong | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D10 |
| FR-KEC-05 | Continue and automatic transition | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D10 |
| FR-KEC-06 | No fullscreen, violations or ranking | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | — |
| FR-KEC-15 | Even column count at least ten | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10) | — |
| FR-KEC-16 | Identical second-half content replay | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10), [P18](PLAN.md#p18) | — |
| FR-KEC-17 | Half generation, separate column answers | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10), [P18](PLAN.md#p18) | — |
| FR-KEC-18 | No repetition disclosure in student UI | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | — |
| FR-KEC-19 | Endurance compares repeated material | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D03 |
| FR-KEC-07 | Purchased attempt credits | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P11](PLAN.md#p11), [P13](PLAN.md#p13) | — |
| FR-KEC-08 | Consume credit and add purchases to same access | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P13](PLAN.md#p13), [P18](PLAN.md#p18) | D05 |
| FR-KEC-09 | Same three payment methods | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P13](PLAN.md#p13), [P14](PLAN.md#p14) | — |
| FR-KEC-10 | History and remaining quota | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | — |
| FR-KEC-11 | Per-column and total counts | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | — |
| FR-KEC-12 | Speed/endurance/accuracy 0–100 scores | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D03 |
| FR-KEC-13 | Detailed attempt review | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | — |
| FR-KEC-14 | Weighted final score 30/45/25 | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18) | D03 |
| FR-ADM-KEC-01 | Kecermatan definition builder | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10) | — |
| FR-ADM-KEC-02 | Priced metered product builder | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P11](PLAN.md#p11) | — |
| FR-ADM-KEC-03 | Edit/delete locked after attempts | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10) | — |
| FR-ADM-KEC-04 | Full count/composite/final/quota dashboard | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18), [P21](PLAN.md#p21) | D03 |
| FR-ADM-KEC-05 | Test/program filters | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P18](PLAN.md#p18), [P21](PLAN.md#p21) | — |
| FR-ADM-KEC-06 | Configuration minimums and defaults | [§4.7](ARCHITECTURE.md#47-kecermatan-runtime-and-scoring) | [P10](PLAN.md#p10) | D09 |
| FR-ADM-KEC-07 | Seven supported item modes | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09), [P10](PLAN.md#p10) | D09 |
| FR-ADM-KEC-08 | Image package and uniform image display | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P06](PLAN.md#p06), [P09](PLAN.md#p09) | D09 |
| FR-ADM-KEC-09 | No duplicated elements within/across items | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P10](PLAN.md#p10) | D09 |
| FR-ADM-KEC-10 | Enough unique elements for configuration | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P10](PLAN.md#p10) | D09 |
| FR-ADM-KEC-11 | Named item-package CRUD | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-12 | Master and tenant package ownership | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-13 | Matching source-package selection | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-14 | Referenced package deletion/shrink guard | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-15 | One package page and element grid/count | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-16 | Paste/palette/CSV/TXT/XLSX symbols | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-17 | Multiple image and safe ZIP upload | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D09 |
| FR-ADM-KEC-18 | Accepted/rejected preview before commit | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-19 | Symbol and content-hash image deduplication | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-20 | Safe edits and dashboard-only filenames | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-21 | Ephemeral generated column preview | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-22 | Platform master creation and tenant copy | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D02 |
| FR-ADM-KEC-23 | Deduplicated mixed source pools | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09), [P10](PLAN.md#p10) | D09 |
| FR-ADM-KEC-24 | System-selected consistent bundled emoji | [§4.4](ARCHITECTURE.md#44-assets-and-item-packages) | [P09](PLAN.md#p09) | D09 |
| FR-LAND-01 | Tenant hero, registration/catalog and statistics | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | D06 |
| FR-LAND-02 | Featured program links | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-LAND-03 | Accurate platform feature copy | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | D11 |
| FR-LAND-04 | Real tenant featured products | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | D06 |
| FR-LAND-05 | Three-step purchase/exam/result flow | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | — |
| FR-LAND-06 | Closing CTA/footer/staff portal | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P11](PLAN.md#p11) | D06 |
| FR-TEN-01 | Tenant provisioning | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P04](PLAN.md#p04) | — |
| FR-TEN-02 | Immutable unique reserved-safe slug | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P01](PLAN.md#p01), [P04](PLAN.md#p04) | D06 |
| FR-TEN-03 | Paused tenant gate with platform exception | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P04](PLAN.md#p04) | D15 |
| FR-TEN-04 | Per-tenant branding | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P04](PLAN.md#p04), [P11](PLAN.md#p11) | — |
| FR-TEN-05 | First tenant Admin provisioning | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P04](PLAN.md#p04) | — |
| FR-TEN-06 | Operational data tenant isolation | [§4.2](ARCHITECTURE.md#42-data-ownership-and-schema) | [P01](PLAN.md#p01), [P23](PLAN.md#p23) | — |
| FR-TEN-07 | Tenant-only ranking | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P17](PLAN.md#p17) | — |
| FR-TEN-08 | Scoped repository default | [§4.2](ARCHITECTURE.md#42-data-ownership-and-schema) | [P01](PLAN.md#p01), [P23](PLAN.md#p23) | — |
| FR-TEN-09 | Hybrid master/tenant content | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P05](PLAN.md#p05), [P07](PLAN.md#p07), [P09](PLAN.md#p09) | — |
| FR-TEN-10 | Combined authorized builder selection | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P08](PLAN.md#p08), [P10](PLAN.md#p10) | — |
| FR-TEN-11 | Read-only master and duplicate-to-own | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P05](PLAN.md#p05), [P07](PLAN.md#p07), [P09](PLAN.md#p09) | — |
| FR-TEN-12 | No master update propagation | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P07](PLAN.md#p07), [P08](PLAN.md#p08) | — |
| FR-TEN-13 | Tests/products always tenant-owned | [§4.2](ARCHITECTURE.md#42-data-ownership-and-schema) | [P08](PLAN.md#p08), [P10](PLAN.md#p10), [P11](PLAN.md#p11) | — |
| FR-TEN-14 | One platform merchant and bank account | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P14](PLAN.md#p14) | — |
| FR-TEN-15 | Tenant financial view and platform recap | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P22](PLAN.md#p22) | — |
| FR-TEN-16 | Revenue-share settlement outside system | [§4.5](ARCHITECTURE.md#45-catalog-purchases-wallet-and-grants) | [P22](PLAN.md#p22) | — |
| FR-TEN-17 | Platform tenant/statistics/status dashboard | [§4.8](ARCHITECTURE.md#48-tryout-scoring-results-monitoring-and-reporting) | [P22](PLAN.md#p22) | — |
| FR-TEN-18 | Audited platform support context | [§4.1](ARCHITECTURE.md#41-tenant-and-identity) | [P04](PLAN.md#p04), [P22](PLAN.md#p22) | — |
| FR-TEN-19 | Shared master-management UI | [§4.3](ARCHITECTURE.md#43-curriculum-question-bank-and-immutable-content) | [P05](PLAN.md#p05), [P07](PLAN.md#p07), [P09](PLAN.md#p09) | — |

## 5. Requirements without FR IDs

| Source | Required behavior / verification | Architecture | Delivery |
|---|---|---|---|
| §5.5 question types | PG single correct, multi all-or-nothing, exact/case-insensitive text, true/false; images and math | §4.3, §4.6 | P07, P16 |
| §5.6 difficulty weights | Correct answers sum difficulty weights with explicit ownership/config version | §4.3, §4.8; D04 | P08, P16, P17 |
| §5.6 option weights | PG-only points follow option identity; omitted point=1; blank response earns none | §4.3, §4.6 | P07, P16 |
| §5.6 IRT | Weighted scoring plus <30-student warning, not an estimated IRT model | §4.8; D04 | P16, P17 |
| §6.1 latency | Non-exam p95 ≤500ms; question load ≤1s; expiry auto-submit ±5s | §4.6, §7 | P16, P18, P19, P23 |
| §6.2 availability/recovery | 99.5% uptime; daily backup; 30-day retention; executed restore proof | §6, §7 | P23 |
| §6.3 security | HTTPS, bcrypt, JWT expiry/refresh, rate limits, upload limits, tenant isolation regression | §4.1, §4.4, §8 | P01, P03, P06, P09, P23 |
| §6.4 usability | Mobile-first, always-visible timer, irreversible-submit warning | §4.6, §4.7, §4.10 | P11, P16, P18, P23 |
| §6.5 scale | 10,000 registered and 1,000 aggregate CCU, stateless API scaling, shared schema | §3, §4.2, §10 | P01, P23 |
| §7 stack and layering | Go/Echo/pgx/raw SQL; Next/React/Tailwind; storage/cache/jobs/merchant boundary | §3, §5 | P00, P02, P06, P14, P23 |
| §7.1 transactional events | Critical jobs commit with business state; derived events may be lost and recover | §4.8, §4.9 | P02, P13, P14, P17, P20, P21 |
| §7.2 edge/cache | Wildcard tenancy, self-hosted compute, only safe public static caching; exam no-store | §3, §8 | P04, P11, P16, P23 |
| §8 model unification | Unified test/enrollment/session roots and product links; typed answer/result tables | §4.2 | P01, P08, P10, P13, P16, P18 |
| §8 constraints/versioning | Tenant uniqueness, platform null handling, repeated-column snapshots and ledger actor/balance-after | §4.1–4.7 | P01, P07, P09, P12, P18 |
| §10 open questions | Client scoring approval, new budget/calendar, download promises and routing | §9 D03/D06/D11/D16 | Owning slices plus P23 |

## 6. Verification and remaining decisions

The documentation verification checks exact set equality with the PRD's 161 FR definitions, unique traceability rows, valid slice IDs/links, all prerequisite edges and absence of dependency cycles, the 11-section TRD structure, local links, Markdown fences, Mermaid syntax, and unchanged PRD/application files. It does not prove product behavior, performance, payment correctness, live provider delivery or production readiness.

Verified on 2026-10-09:

| Check | Result |
|---|---|
| Exact PRD FR coverage and duplicate detection | 161/161 mapped exactly once |
| Slice references and dependency traversal | 24 slices; no missing predecessor or cycle |
| Architecture structure and policy references | 11 sections; D01–D16 resolve |
| Local links/anchors and Markdown fences | Pass |
| Actual Mermaid rendering | All four diagrams rendered with installed Mermaid 11.15.0 |
| Whitespace and change scope | Pass; only the three requested documentation artifacts changed/added |

The policy register is deliberately open for owner review; this is not approval to publish a finalized TRD or execute dependent contracts. Rendering proves diagram syntax, not implementation correctness.

All identified conflicts have a documented design treatment or an explicit D01–D16 owner/gate in [the architecture decision register](ARCHITECTURE.md#9-risks--open-questions). This completes full-scope translation as a reviewable draft. Business approval of those decisions and future per-slice wire-contract review remain necessary before the affected implementation. No production data was changed, no deployment was performed and no commit/push was made.
