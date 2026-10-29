# Self-service manual top-up (P15)

A student asks for a wallet credit by bank transfer, staff check the proof and approve or reject. A top-up
never creates an order and never grants a test. Money moves only through the P12 wallet path
(`WalletService.Apply`); the request is the evidence, the ledger entry is the money.

    topup_requests   id, number (identity), tenant_id, user_id, amount, method, status, idempotency_key,
                     proof_asset_id, reject_reason, reviewed_by, approved_at, rejected_at, created_at, updated_at
                       UNIQUE (tenant_id, user_id, idempotency_key)
                       UNIQUE (proof_asset_id) WHERE NOT NULL
                       CHECK amount >= 1 AND amount = trunc(amount)
                       CHECK (method = 'manual') = (proof_asset_id IS NOT NULL)
                       FOREIGN KEY (user_id, tenant_id) -> users, (tenant_id, proof_asset_id) -> assets
    tenant_settings  min_topup numeric(14,2) NOT NULL DEFAULT 10000, whole rupiah >= 1

The code is `TUP-` plus the number padded to six digits. `method` already admits `gateway`: P14 reuses the
status machine and the credit invariant and keeps its provider evidence (payment attempts, events) in its own
tables. Only `manual` is created today.

## Amount (D12)

Whole rupiah, no upper bound, at least the tenant minimum. The server decides: `amount` is digits with an
optional `.0`/`.00`; a fraction, sign, exponent, zero or malformed value is `422` on `amount`, as is a value
below the minimum. "No upper bound" is limited in practice by the ledger column (`numeric(14,2)`, the wallet
path caps one movement below 10 billion). The minimum is stored in `tenant_settings.min_topup` (default
Rp10.000 when a tenant never saved one) and edited by Admin only through `PUT /api/v1/admin/settings`, with the
scoring weights, in one upsert. The student list response carries `min_topup` so the page can show it.

## Create (`POST /api/v1/topups`)

Student only. In one transaction the server locks the student's `users` row (the same lock the wallet uses),
looks the `idempotency_key` up (same key, amount and proof replays the original with `200`; anything else is
`409`), checks the amount against the minimum, finalizes the proof with `AssetService.FinalizeTx` (a missing,
foreign, wrongly typed or reused proof is one `422` on `proof_asset_id`) and inserts a `pending` request. A
proof that already backs an order is refused too. Nothing is credited.

## Review

`POST /api/v1/admin/topups/{id}/approve` and `/reject` (Admin and Operator). One transaction: lock the
request row (`FOR UPDATE`), act only if it is `pending`, then

- approve: mark `approved` with the reviewer, call `Apply` (`kind topup`, actor `staff` with the reviewer id,
  source `topup`/`<request id>`), enqueue the email and WhatsApp notifications;
- reject: mark `rejected` with the reviewer and the required reason (1-500 characters), enqueue notifications.

Two concurrent approvals serialize on the request lock: the second finds `approved` and returns it unchanged,
so the wallet is credited once; the ledger's unique source key is the second guard. Repeating a review on a
request already in that state answers `200` and changes nothing; the opposite review is `409`. Any failure,
including the job insert, rolls back the status, the credit and the notifications together.

## Visibility

A student sees only their own requests (`GET /topups`, `/topups/{id}`; another student's id is `404`). Staff list
every request with a `status` filter and `q` (code or student name). The proof is read through
`GET /api/v1/assets/{id}/url`: the uploader and staff may read it, anyone else gets `404`. The approved credit
shows on the wallet ledger as a `topup` entry whose `source_id` is the request id, which the wallet page maps to
the request code.

## Web

- Student `/student/wallet`: a "Top up saldo" button on the balance card opens a modal (amount, destination
  account from `/payments/bank-transfer`, proof upload) and a "Permintaan top up" list shows status and
  rejection reason. The proof upload and the retry key follow checkout: one key per amount/proof choice.
- Admin `/admin/topups` (Admin and Operator): the Orders page pattern, with a status filter and a detail modal
  with the proof, Approve, and Reject with a reason.
- Admin `/admin/settings`: the minimum top-up field next to the scoring weights.

## Tests

Service (Postgres): amount/key/proof validation, the minimum setting, pending credits nothing and creates no
order or enrollment, idempotent create, proof reuse, approve once with actor and ledger source, reject reason,
state transitions, concurrent approvals credit once, rollback on a failed notification, consistent balance
chain, visibility. Repository: scope arguments, tenant isolation, proof uniqueness, table constraints.
Handler: roles and ownership, proof access, validation, the saved minimum, the review endpoints and the wallet
ledger reference.

## Not covered

- Gateway top-ups (P14).
- No limit on how many requests a student may keep pending; staff see them all.
- Create does not refuse when the bank account is not configured (checkout answers `503`); the page shows
  "unavailable" because the account lookup fails.
- A proof already used by an order is refused in SQL, but there is no dedicated test for that branch.
