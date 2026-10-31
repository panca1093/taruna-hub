# Execution ledger — 2026-10-31-auth-dashboard.md

Base: c2879da. Branch: codex/auth-dashboard-redesign. Source checkout was clean; current exam-layout commit was preserved. No push, merge or commit has been made.

- Task 1 implemented: OpenAPI summary, typed client/schema, raw scoped repository read, repeatable-read service aggregation, authenticated student route.
- Watched summary unit test fail on missing `summarizeDashboard`, then pass after implementation.
- Focused Go checks passed: TestDashboardSummaryEmptyAndRetakes and TestStudentDashboardIdentityRolesAndPersistedResults against local Postgres/Redis (empty, retakes/zero, exact balance, same-tenant student identity, role/anonymous guard, real completed result, no-store).
- Task 2 implemented: approved AuthLayout, password input, student/staff entries, real registration -> verify, skip/resend, reminder, recovery layout, hidden public staff links and existing bookmark redirect.
- Task 3 implemented: real dashboard loading/failure/empty/populated modules and scoped compact StatCard; recent results limited to latest per tryout to match the existing result destination; data reads include kecermatan links.
- Typecheck/lint preliminary runs passed. Full API build/vet/race tests and web production build are in progress. Browser demo and fresh reviewer are in progress.
- Design/auth docs updated; slice status explicitly says verification is pending. No acceptance box was ticked without evidence.

Implementation calls: keep grading and money authoritative on the server; add only a read-only summary contract. Reuse a clean feature branch rather than disturb the existing exam branch. Execute directly under the owner's explicit implementation approval; no second design approval is needed.

## Implementation verification and review (before publication)

- Fresh reviewer (read-only) found two material issues. Both fixed: exclude null `tryout_id` kecermatan enrollments; verification page admits supported staff roles and returns to the role's home.
- Watched `TestStudentDashboardExcludesKecermatan` fail (owned count 1) before filtering, then pass. Related handler/service checks passed with real local database services.
- Browser found invisible account-menu focus; added token focus ring and named mobile dialog. Rechecked: Tab focuses the named account trigger with a nonzero ring; Escape restores focus to the mobile menu button.
- Final web typecheck/lint/native production build passed; final Docker production web build also passed. Schema regeneration reproduced the reviewed schema byte for byte.
- API build/vet passed natively. Native full suite first competed with the running dev worker's shared notification queue and timed out; the isolated macOS run then hit the existing `127.0.0.2` SMTP bind restriction. Linux rerun initially lacked the web emoji assets; rerun with `web/public` mounted read-only passed `go test ./... -race -count=1` for all packages. No unrelated test/product source was changed to bypass these environment failures.
- Complete passing Linux suite used disposable DB `tryout_ui_01a18de5_20261031`, which was dropped after success. Local API/web were rebuilt and restarted; postgres/redis/minio/worker stayed running.
- Browser/API/database demo passed with the new `Siswa UI` account: name/email/optional-phone signup, real wrong-code error, refresh cookie recovery, skip/reminder, known code fixture -> persisted verified state, zero balance/counts, one temporary real enrollment -> count 1/populated module, then scoped removal of that temporary enrollment. No application password, secret, seed account or wallet was reset.
- Student login through `/portal` still routes to `/student` from server claims; logout and anonymous protected-route redirect pass. `/login/staff` redirects to `/portal`; public internal links count is zero.
- Auth/recovery, verification and dashboard were checked at 320/390/768/1024/1440px. Indonesian/light and English/dark layouts had no document overflow; ring/caption/cards did not overlap; dashboard module gaps measured 24px. Mobile drawer keyboard dismissal/name and password visibility passed.
- Local SMTP is unconfigured: no real-inbox delivery claim. Code fixtures affect only this session's local QA account. Documented seed passwords were rejected by the current local DB; those passwords were preserved and a current staff login is not claimed as browser-tested. Reduced-motion handling is present in CSS and the ring hook; no OS preference was changed.
- No commit, push or merge performed. Changes remain reviewable on codex/auth-dashboard-redesign.

## PR preparation

- Owner subsequently authorized commit, push and a PR against main. The original exam-layout commit remains on `feat/larger-exam-type`; the published auth/dashboard branch excludes that independent change.
- Before publication, schema regeneration, web typecheck, lint and production build were rerun successfully. API build/vet/race checks use a separate disposable database and the final source mounted read-only in Linux.
- Publication checks passed: Linux `go build ./...`, `go vet ./...`, and `go test ./... -race -count=1` for every API package (service 119.084s). The first retry was interrupted when the shared local stack stopped; the successful retry used dedicated temporary Postgres/Redis containers instead. Those temporary containers are removed after verification. The interrupted disposable DB `tryout_pr_01a18de5_20261031` remains in the stopped local Postgres volume; no application rows were changed.
- Unrelated exam edits appeared in the original checkout during publication. That checkout and its edits were left intact; only the reviewed auth/dashboard commit was cherry-picked onto current main in `/private/tmp/bimbel-auth-pr-01a18de5`, branch `codex/auth-dashboard-pr`. No exam files are in the PR.
