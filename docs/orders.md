# Orders, checkout and enrollments (P13)

A student buys a product with the wallet or by bank transfer. Either way the server writes an order that
snapshots what was bought, and a confirmed order opens the tryouts it contains. Money moves only through the
P12 wallet path; access is issued by one shared function.

    orders        id, number (identity), tenant_id, user_id, product_id, product_type, product_title, price,
                  payment_method, status, idempotency_key, proof_asset_id, cancel_reason, reviewed_by,
                  confirmed_at, cancelled_at, created_at, updated_at
                    UNIQUE (tenant_id, user_id, idempotency_key)
                    UNIQUE (proof_asset_id) WHERE NOT NULL
                    UNIQUE (tenant_id, user_id, product_id) WHERE status = 'pending'
                    FOREIGN KEY (user_id, tenant_id) -> users, (tenant_id, proof_asset_id) -> assets
    order_items   order_id, tenant_id, tryout_id, tryout_title, position      PRIMARY KEY (order_id, tryout_id)
    enrollments   id, tenant_id, user_id, tryout_id, active_until, attempts_purchased, attempts_used, created_at
                    UNIQUE (tenant_id, user_id, tryout_id)
    enrollment_grants   enrollment_id, order_id, source_type, source_id, actor_kind, actor_id, reason, attempts
                    UNIQUE (tenant_id, source_type, source_id, enrollment_id)

`product_id` has no foreign key on purpose: the title, price and tryouts on the order are what was bought, so
editing or deleting the product later changes nothing. `price` is `numeric(14,2)` and travels as a canonical
decimal string like every other amount (D12); the client sends only `expected_price`, the price its page showed. The order code is
`ORD-` plus the identity number padded to six digits.

## Checkout (`POST /api/v1/orders`)

Student only. In one PostgreSQL transaction the server:

1. locks the student's `users` row, which serializes that student's checkouts and wallet movements;
2. looks the `idempotency_key` up. The same key for the same product, method and proof returns the original
   order (`200`); the same key for anything else is `409`;
3. reads the product through `ProductRepository.SellableTx`, which reuses the catalog's visibility predicate, so
   checkout and catalog cannot disagree: active, inside `active_from`..`active_until`, every tryout published
   and still open. Anything else is `404`;
4. if `expected_price` differs from the product's price, `409 price_changed` and nothing is written (a replay in
   step 2 is answered before this check);
5. for an `individual` product, refuses with `409 already_owned` if the student holds that tryout with its
   window still open (D05b). A `bundle` is always allowed;
6. for `manual`, finalizes the proof with `AssetService.FinalizeTx`. A missing, foreign, wrongly typed or reused
   proof is one `422` on `proof_asset_id`; the unique index on `proof_asset_id` makes a proof back one order;
7. inserts the pending order and its items. The partial unique index `orders_pending_product_key` allows one
   pending order per student and product, any method: the loser gets `409 order_pending`. A wallet order is
   confirmed in the same transaction, so it never stays pending;
8. for `wallet`, calls `WalletService.Apply` (`spend`, `order`/`<order id>`), confirms the order, issues the
   enrollments and enqueues the notifications. Too little balance is `422 insufficient_balance` and the whole
   transaction rolls back, as does any later failure, including the job insert.

A manual order stays `pending` with no access until staff review it. The gateway method is P14; asking for it is
`422 payment_method`.

## Review (`/api/v1/admin/orders`)

Admin and Operator (D02, role guard). `GET` lists with `status`, `q` and paging; `GET /{id}` reads one; the proof
is read with `GET /api/v1/assets/{proof_asset_id}/url`, which re-authorizes each call. `POST /{id}/confirm` and
`POST /{id}/cancel` (reason, 1-500 characters) take the order row lock, so concurrent or repeated reviews queue
and exactly one transition happens:

| Current | Confirm | Cancel |
|---|---|---|
| pending | confirmed, access issued, notified | cancelled, reason stored, no access, notified |
| confirmed | `200`, nothing changes | `409` |
| cancelled | `409` | `200`, first reason kept |

A confirmed order never goes back to pending; the status columns have check constraints that tie each status to
its timestamp and a cancelled order to a non-blank reason.

## Enrollments

`OrderService.IssueEnrollmentsTx` is the one place access is granted; wallet and manual confirmation call it now,
and the gateway (P14) and staff grants (P22) will call it with their own source. For each tryout it inserts the
user's single `enrollments` row and a grant naming the source; a tryout the user already holds is left untouched
(D05b), and the unique grant key stops a repeated source from granting twice. A grant without an order
(`source_type = 'staff'`) must carry an actor and a reason, enforced by a check.

D05a: a tryout's access window is the tryout's own `active_from`/`active_until`, so `enrollments.active_until`
stays `NULL` for tryouts and `GET /api/v1/enrollments` reads the window live from the tryout. The column and
`attempts_purchased`/`attempts_used` hold a kecermatan test's credits. P18 made `tryout_id` nullable beside a
`kecermatan_test_id`, with a CHECK that exactly one is set and a second unique key per user and kecermatan test: a
kecermatan order (no items; it snapshots the test and `included_attempts`) adds credit to that one row through
`IssueEnrollmentsTx`. See [kecermatan-runtime.md](kecermatan-runtime.md).

## Product sale window

`products.active_from` (nullable, admin-set) with `active_until` bounds only when a product can be bought. It must
be before `active_until` when both are set (`422 active_from`). The admin product form has one date field for it.

## Notifications

`order-confirmed` and `order-cancelled` are registered with the other templates. Each is enqueued in the same
transaction as the transition, once per channel the student has (email and WhatsApp), with the event key
`<template>:<order id>`, so a repeated review enqueues nothing new. Delivery success is P02's concern.

## Bank account

`BANK_NAME`, `BANK_ACCOUNT_NUMBER` and `BANK_ACCOUNT_HOLDER` (see `api/.env.example`) are the single tenant's
transfer destination, served to students at `GET /api/v1/payments/bank-transfer`. Unset, that route and manual
checkout answer `503`.

## The web

`app/checkout/[id]` reads the public product, the wallet balance and the bank account from the API and offers
manual and wallet payment (the Midtrans option is hidden until P14). One idempotency key is kept per purchase
attempt and the proof is uploaded once per chosen file. `app/student/orders` and `app/student/tryouts` read the
student's own orders and enrollments; `app/admin/orders` lists, filters, opens the proof, confirms, and cancels
with a reason.

## Not done here

Gateway orders (P14), top-up requests, refunds, the tryout runtime that
consumes an enrollment (P16), and a pending-order de-duplication for the same student and product.
