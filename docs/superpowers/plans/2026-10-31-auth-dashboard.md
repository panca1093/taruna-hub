# Account Flow and Student Dashboard Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this approved plan in the current session.

**Goal:** Implement the approved student/staff auth and registration flow through a dashboard that displays authoritative account data.

**Architecture:** Reuse the session context, typed API client, shared form controls and AppShell. Add a read-only student summary to the existing runtime layers; keep presentation in Next pages and shared auth components.

**Tech Stack:** Go 1.26, Echo v4, PostgreSQL, Next.js 16.2.9, React 19, HeroUI v3, Tailwind v4.

**Spec:** `docs/superpowers/specs/2026-10-31-auth-dashboard-design.md`

## Global Constraints

- D01: email verification never blocks login; name/email/optional phone registration is retained.
- Data crosses `web/lib/api`; OpenAPI is the wire contract. No grading keys, snapshots, demo identities or browser-generated scores.
- Semantic tokens, existing fonts, bilingual copy, shared controls and reduced-motion support.
- Preserve the existing exam-layout branch. Publication was initially out of scope; the owner subsequently authorized commit, push and a PR, without merging.

## Review Focus

- Refreshing verification with a valid cookie must recover the session before deciding where to send the user.
- Wrong-role logins must follow authenticated claims rather than grant the entry page's requested role.
- An unavailable dashboard must never show false zero counts or balance.
- Retakes must not double count distinct completed tryouts; zero is a real score, while no results is null.
- Long translations, narrow viewports, focus and short desktop heights must not cause ring/text overlap or hide the primary action.

### Task 1: Authoritative dashboard summary

**Files:** `api/openapi.yaml`, existing runtime model/repository/service/handler files, focused dashboard tests, `web/lib/api/client.ts` and generated schema.

**Interfaces:** `GET /api/v1/my/dashboard` returns owned/completed tryout counts, nullable average score, and an exact decimal balance, scoped to the authenticated student. Existing enrollments/history endpoints supply module rows.

- [x] Define contract and generate schema.
- [x] Add failing tests for empty summary, zero/retake averages, same-platform student isolation, anonymous/staff rejection.
- [x] Implement minimal scoped reads and service aggregation; run focused Go tests with real Postgres/Redis.

### Task 2: Approved auth presentation and verification flow

**Files:** shared `web/components/auth-*`, auth dictionary/pages, `/portal`, `/verify-email`, role redirects and public footer.

**Interfaces:** existing `useSession.login/register/reloadUser`, `api.verifyEmail/resendEmailCode`; students go to `/student`, staff to `/admin`, registration goes to `/verify-email`.

- [x] Reuse shared form controls and build the separated-ring auth layout with staff reversal and coordinated entrance animation.
- [x] Implement real registration/verification/skip/resend states, password visibility and autofill semantics.
- [x] Replace public staff links with direct `/portal` entry; redirect old staff address and staff guard/logout destinations.
- [x] Verify typechecking and auth behavior against the API.

### Task 3: Dashboard spacing, real activity and delivery checks

**Files:** student dashboard and dictionary, small opt-in AppShell/StatCard presentation props as needed, verification reminder, DESIGN.md, PLAN.md and docs/auth.md.

**Interfaces:** generated dashboard summary, enrollment/history DTOs and exact money formatter. Existing shell navigation, result and tryout routes remain destinations.

- [x] Implement loading/error/retry/empty/populated states and approved module spacing using existing components.
- [x] Preserve precise result-session navigation and real activity for existing accounts.
- [x] Run required API build/vet/race tests, schema drift check, web typecheck/lint/build in order.
- [x] Run browser → API → database demo and viewport/theme/locale/keyboard checks; update slice acceptance status only with evidence.
- [x] Perform a whole-change review, resolve material findings, and report remaining limitations. Publish only after owner authorization; never merge without a separate request.
