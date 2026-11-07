# Student operations and the tenant revenue recap (P22)

Staff run accounts and manual grants for one tenant, and the Admin reads a revenue recap. Everything is
tenant-scoped (D17): there is no Platform Admin view and no cross-tenant report here.

## Who may do what (D02)

| Action | Student | Operator | Admin |
|---|---|---|---|
| List/read/create/edit/deactivate/reactivate a **student** account | 403 | yes | yes |
| Same on an **operator or admin** account (and `role=operator\|admin\|staff` lists) | 403 | 403 | yes |
| Per-student access list, manual grant | 403 | yes | yes |
| Revenue recap | 403 | 403 | yes |

The routes allow Admin and Operator; `service.mayManage` decides by the target's role, so the same rule
guards reads, edits and status changes. An id of another tenant, a Platform Admin or (on student routes) a
non-student is `404`. A role never changes through an edit.

## Accounts and deactivation (D14)

    GET  /api/v1/admin/users            role, active, q, limit, offset   (deactivated accounts stay listed)
    POST /api/v1/admin/users            name, email, phone?, password, role
    GET|PUT /api/v1/admin/users/{id}    PUT replaces name, email, phone (empty phone clears it)
    POST /api/v1/admin/users/{id}/deactivate | /reactivate    reason (required)

There is no delete. Deactivating flips `users.is_active`, revokes every live `auth_sessions` row in the same
transaction and audits it. Login then answers the same `401 unauthorized` as a wrong password (the existing
anti-enumeration behaviour, unchanged), a refresh is refused, and the old access token fails on its next
request. The row stays, so orders, ledger entries, enrollments, results and audit rows keep resolving.
Reactivating restores login with the old password. Repeating either call is a no-op that audits nothing, and
nobody can deactivate themselves. A changed email clears `email_verified_at` and any pending code.
Retention periods and RPO stay open under D14 and are not built here.

## Manual grants

    GET  /api/v1/students/{id}/enrollments      tryouts and kecermatan credits, with active and source_type
    POST /api/v1/students/{id}/grants           tryout_id | kecermatan_test_id + attempts, reason, idempotency_key

A grant goes through `OrderService.IssueEnrollmentsTx`, the same issuance a purchase uses, with
`source_type = 'staff'`, `source_id = idempotency_key`, `actor_kind = 'staff'`, the staff `actor_id` and the
reason, and no order. It writes one `enrollment_grants` row and opens only the named test. A tryout must be
published; kecermatan credits add to the student's single enrollment for that test.

The key is serialized with an advisory lock, then looked up: the same key and body answers `200` with the
original access, the same key with another student, test or credit answers `409`, and a tryout the student
already holds answers `409 already_owned`. The first call is `201`.

`active` in the access list is computed at read time: a published tryout inside its own window, or a
kecermatan test with credits left (`attempts_used < attempts_purchased`).

## Audit

`audit_events` (migration 000024) records `actor_id`, `actor_role`, `action`, `target_type`, `target_id`,
`reason`, a small `detail` object and `created_at`, tenant-scoped, written in the same transaction as the
action. Actions: `user.create`, `user.update` (only when a field changed), `user.deactivate`,
`user.reactivate`, `grant.manual` (target is the enrollment). Rows are only inserted; there is no update
or delete path. There is no read endpoint yet.

## Revenue recap

    GET /api/v1/admin/reports/revenue?from=YYYY-MM-DD&to=YYYY-MM-DD&limit&offset     (Admin only)

Confirmed orders (by `confirmed_at`) and approved top-ups (by `approved_at`) for an inclusive range of
Asia/Jakarta days, default the last 30 days, at most 366. One repeatable-read snapshot returns exact decimal
strings (`numeric(16,2)`): order count and total, by payment method, by product type, the top-up count and
total, and a paginated list of days with activity, newest first. A wallet order is funded by earlier top-ups,
so the two totals are different views and are never summed. It is a report: no settlement, no merchant
credentials.

## Screens

- `/admin/users` (Admin): staff accounts, status filter, search, create/edit, deactivate/reactivate with reason.
- `/admin/students` (Admin, Operator): the same account actions for students beside the wallet adjustment,
  plus an **Access** list and **Grant access** in the student drawer.
- `/admin/orders` (Admin): the recap sits above the order list, where the money already is.

## Not covered

Results of a deactivated student are protected by the same no-delete rule but are not exercised by a P22 test.
