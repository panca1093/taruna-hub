# Gateway orders and top-ups (P14, Midtrans Snap)

A student pays an order or a wallet top-up on the Midtrans payment page. Money and access move only when the
server has confirmed the payment with Midtrans; the browser coming back from the payment page proves nothing.
One platform-owned merchant account serves the deployment (D16). Money still moves only through
`WalletService.Apply`, access only through `OrderService.IssueEnrollmentsTx` (see [orders.md](orders.md),
[topups.md](topups.md)).

## Configuration

| Variable | Meaning |
|---|---|
| `MIDTRANS_SERVER_KEY` | Merchant server key. Unset: the gateway method is unavailable (`503` on `/payments/gateway`, checkout and top-up) and nothing else changes. |
| `MIDTRANS_IS_PRODUCTION` | `true` for the live API; default `false` (sandbox). |
| `MIDTRANS_NOTIFICATION_URL` | Optional. Sent as `X-Override-Notification` on each Snap call. |
| `APP_URL` | Existing. The Snap `callbacks.finish` is `APP_URL/student/orders` (orders) or `APP_URL/student/wallet` (top-ups). |

No client key is needed: the flow is a server-side Snap redirect. In the Midtrans dashboard set the payment
notification URL to `<api origin>/api/v1/payments/midtrans/notification` (unless `MIDTRANS_NOTIFICATION_URL` is used).

## Data (migration 000021)

    payment_attempts   id, tenant_id, user_id, provider, provider_order_id (UNIQUE, global), order_id | topup_id
                       (exactly one, each UNIQUE), amount, status, redirect_url, expires_at, last_checked_at, paid_at
    payment_events     attempt_id, source (notification|status), transaction_status, fraud_status, status_code,
                       gross_amount, provider_transaction_id, body_hash, raw jsonb, outcome
                       UNIQUE (attempt_id, source, body_hash)
    payment_exceptions id, attempt_id (UNIQUE), user_id, order_id | topup_id, kind, paid_amount, status
                       (held|credited|refunded), resolved_by, resolve_reason, resolved_at
    orders.payment_method now admits 'gateway'; topup_requests.method already did.

`provider_order_id` is `TE-` plus a random UUID without dashes, created in the same transaction as the order or
top-up. Attempt status: `pending`, `failed` (a deny the payer may still retry), `paid`, `cancelled` (expired or
cancelled), `held` (a success staff must review). `paid`, `cancelled` and `held` are final.

## Checkout and top-up

1. One transaction creates the pending order (or top-up) and its attempt, with the usual idempotency key, product
   checks, one-pending-per-product rule (P13) and top-up minimum (P15). A gateway order needs a whole-rupiah price
   (`422 payment_method` otherwise); a gateway top-up takes no proof.
2. After the commit, outside any lock, the server creates the Snap transaction and stores `redirect_url`. The web
   sends the student there.
3. Ambiguous create (timeout, network, 5xx): the order stays pending without a page and the answer is `502
   gateway_error`. Retrying with the same idempotency key reuses the same `provider_order_id`. If Midtrans says
   the id exists but has no transaction (the payer never saw that page), the order or top-up is cancelled and a
   fresh checkout starts clean. A definite refusal cancels it at once. So one business operation is never two
   payments.

## Notification (`POST /api/v1/payments/midtrans/notification`)

No session and no tenant host. In order:

1. Verify `SHA512(order_id + status_code + gross_amount + server_key)` in constant time. Wrong: `403`, nothing stored.
2. Look the stored reference up; the tenant and entity come from there. Unknown: `200` (the provider stops
   retrying), nothing stored.
3. Ask the Status API for the truth (outside any lock) and apply that, not the body. An unreachable Status API is a
   `5xx`, so Midtrans retries. If the notification and the Status API disagree on order id or amount, the
   notification is stored as evidence and nothing moves.
4. In one transaction lock the attempt, then the order or top-up, then the student, apply, and store the
   evidence.

| Provider status | Effect |
|---|---|
| `settlement`; `capture` with `fraud_status=accept` | Success (below) |
| `pending`, `authorize`, `capture` + `challenge` | Nothing |
| `deny`, `failure`, any `fraud_status=deny` | Attempt `failed` (order stays pending; a later success still counts) |
| `cancel`, `expire` | Attempt `cancelled`; a pending order is cancelled (reason shown, `order-cancelled` notice) or a pending top-up rejected |
| `refund`, `partial_refund`, `chargeback`, `partial_chargeback` | Evidence only |

Success on a pending order: confirm it (no reviewer), issue the enrollment or kecermatan credits through the one
issuance path, send the `order-confirmed` email and WhatsApp, attempt `paid`. Success on a pending top-up: approve it
and credit the wallet via `Apply` (`topup`, source `topup`/`<request id>`, actor `provider`), send `topup-approved`.
A gateway order moves no wallet money.

Replays and reordering are harmless: a final attempt ignores everything except more evidence, identical bodies are
stored once, and because the Status API is the authority a stale `pending` can never undo a settled payment. Eight
concurrent identical notifications grant once (tested).

## Held payments (D07)

A success that cannot be applied cleanly is stored as a `payment_exception` and the attempt becomes `held`. Nothing
reopens access and nothing is refunded automatically:

- `late_success`: the order was cancelled or expired (or the top-up rejected).
- `amount_mismatch`: the Status API's `gross_amount` differs from the stored amount.
- `unexpected_state`: the order was confirmed another way (for example a staff confirm) before the payment.

Admin and Operator resolve it, each with a required reason (1-500 characters) recorded with the actor:

- `POST /api/v1/admin/payment-exceptions/{id}/credit` credits the paid amount to the wallet once (`topup`, source
  `payment_exception`/`<id>`, actor staff). It does not reopen access. Concurrent or repeated calls change nothing
  more.
- `POST .../refund` records that the money went back outside the system. It sends nothing to Midtrans.
- Credit after refund (or the reverse) is `409`. `GET /api/v1/admin/payment-exceptions?status=held` lists them; the
  admin Orders page shows a "Perlu tinjauan" badge and the two actions in the order detail. A held top-up exception
  has no screen yet (see below).

## Expiry and reading the status

Snap transactions expire after 24 hours. Midtrans normally sends `expire`. As a safety net, reading a pending
gateway order or top-up (`GET /orders/{id}`, `GET /topups/{id}`) asks the Status API at most once every 10 seconds:
a settled payment is applied, and a transaction Midtrans does not know about, past its 24-hour expiry, is treated as
expired. The student orders page and the wallet page poll these reads every 5 seconds while a gateway payment is
pending. There is no background sweeper; an abandoned pending order that nobody reads and Midtrans never reports
stays pending (and keeps the one-pending-per-product rule) until it is read. Add a River sweep before launch if
that matters.

## Tests

Service (Postgres, fake gateway): signature, unknown reference, unconfirmed claim, status outage, fraud deny then
settlement, wrong amount, replay x3, reorder, concurrent x8, refund and chargeback evidence, expiry and repurchase,
late success held, staff credit once under concurrency, mark refunded, ambiguous create (both outcomes), definite
refusal, unconfigured gateway, price rule, gateway top-up, reconcile and throttle. Repository: scope, global
reference uniqueness, one attempt per target, event and exception de-duplication. Infra: the real adapter against a
local fake server (signature, Snap request, error classification, the 404-in-a-200 status quirk). Handler: webhook
without auth, role and ownership checks, the exception flow over HTTP, gateway top-up. Not run: live sandbox.

## Not covered

- Live Midtrans sandbox and production calls (no keys in the build environment).
- A "continue payment" action for a pending order after the student closes the payment page (the redirect URL is in
  the API; the screens have no button for it).
- A screen for held top-up exceptions (the API lists and resolves them); notice to the student when staff credit.
- Background sweep of abandoned pending gateway orders (see above).
- Paused-tenant behavior (D15, deferred by D17); partial refunds and chargebacks beyond stored evidence.
