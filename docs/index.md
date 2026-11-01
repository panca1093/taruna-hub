# Implementation notes

How the code behaves and why, one file per area. These describe what is built; the requirements and
decisions behind them stay in `requirement/` (PRD, ARCHITECTURE, PLAN), and the HTTP contract is
`api/openapi.yaml`. How to run things lives in each app's own README.

| Area | Slices | File |
|---|---|---|
| Tenancy: the one configured tenant | P01, P01b | [tenancy.md](tenancy.md) |
| Notifications and durable jobs | P02 | [notifications.md](notifications.md) |
| Authentication, sessions, registration and recovery | P03a, P03b | [auth.md](auth.md) |
| Curriculum: programs, subjects and chapters | P05 | [curriculum.md](curriculum.md) |
| Question bank: four types in immutable versions | P07a | [questions.md](questions.md) |
| Tryouts: drafts, publishing and frozen assignments | P08 | [tryouts.md](tryouts.md) |
| Products and the public catalog | P11 | [products.md](products.md) |
| Website CMS: bilingual marketing content, draft, preview and whole-site publish | P11b | [website-cms.md](website-cms.md) |
| Wallet ledger, balance projection and staff adjustments | P12 | [wallet.md](wallet.md) |
| Assets: private image uploads and short-lived downloads | P06 | [assets.md](assets.md) |
| Orders, checkout and enrollments | P13 | [orders.md](orders.md) |
| Self-service manual top-up and review | P15 | [topups.md](topups.md) |
| Tryout runtime: sessions, deadlines, scoring and the result | P16 | [tryout-runtime.md](tryout-runtime.md) |
| Kecermatan item packages: symbol, image, emoji and shape pools in frozen revisions | P09 | [item-packages.md](item-packages.md) |
| Kecermatan attempts: credits, server-timed kolom, answers, versioned scoring and results | P18 | [kecermatan-runtime.md](kecermatan-runtime.md) |

## Design handoff — proposal only

[Student workspace redesign, 31 October 2026](handoffs/2026-10-31-student-workspace/HANDOFF.md) includes a portable prototype, missing-flow audit, implementation plan and acceptance criteria. This is a proposal package; its demo flows are not evidence that the redesign or payment integrations are implemented.

The handoff also includes an [admin/operator design proposal](handoffs/2026-10-31-student-workspace/ADMIN-PROPOSAL.md), separate from the student implementation plan. A separate local admin prototype now demonstrates three review journeys; production admin screens have not been rebuilt.

The owner selected a [shared warm rose palette](handoffs/2026-10-31-student-workspace/PALETTE-DECISION.md) for student and admin workspaces on 31 October 2026. This replaces the previous blue default; existing blue admin previews remain historical flow references pending restyling.
