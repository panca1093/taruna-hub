# Implementation notes

How the code behaves and why, one file per area. These describe what is built; the requirements and
decisions behind them stay in `requirement/` (PRD, ARCHITECTURE, PLAN), and the HTTP contract is
`api/openapi.yaml`. How to run things lives in each app's own README.

| Area | Slices | File |
|---|---|---|
| Tenancy from the request host | P01 | [tenancy.md](tenancy.md) |
| Notifications and durable jobs | P02 | [notifications.md](notifications.md) |
| Authentication, sessions, registration and recovery | P03a, P03b | [auth.md](auth.md) |
| Curriculum: programs, subjects and chapters | P05 | [curriculum.md](curriculum.md) |
| Question bank: four types in immutable versions | P07a | [questions.md](questions.md) |
| Tryouts: drafts, publishing and frozen assignments | P08 | [tryouts.md](tryouts.md) |
| Products and the public catalog | P11 | [products.md](products.md) |
