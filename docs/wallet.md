# Wallet: the ledger, the projection and staff adjustments (P12)

One number says what a student can spend — `users.balance` — and one table explains it, row by row,
forever. `wallet_entries` is the **ledger**: every movement is one immutable signed row that carries
the balance just after it. `users.balance` is the **projection**, kept equal to the ledger sum by the
one movement path below.

    wallet_entries   id, seq, tenant_id, user_id, amount, balance_after, kind, actor_kind, actor_id,
                     reason, source_type, source_id, created_at
                       UNIQUE (tenant_id, source_type, source_id)
                       FOREIGN KEY (user_id, tenant_id) REFERENCES users (id, tenant_id)
                       FOREIGN KEY (actor_id) REFERENCES users (id)

`seq` is an identity column, so it is a total order: listed by `seq`, each row's `balance_after` is
the previous `balance_after` plus its own `amount`. `kind` is `opening`, `topup`, `spend` or
`adjustment`; `actor_kind` is `system`, `student`, `provider`, `staff` or `platform`. A nullable
`actor_id` alone could not tell a student from a gateway from a staff member, so the kind and the
actor kind are stored beside it (FR-PAY-W-08).

## Money (D12)

Balance and every amount are `numeric(14,2)` — **exactly two decimal digits** — and on the wire money
is a canonical decimal **string**: `"50000.00"`, `"-25000.00"`. No `float64` appears in Go, no
`::float` in SQL, and the web only formats the string for display (`lib/format-money.ts`). The one
parser accepts an optional `-`, ASCII digits and at most two fraction digits, so `0.01`, `1`, `1.5`
and `9999999999.99` are accepted (normalized to `0.01`, `1.00`, `1.50`), while `0`, `0.00`, `-0`,
`1e3`, `NaN`, `+5`, `1.234`, `1.`, `.5`, `" 5"`, `"5 "`, `""`, `10000000000.00` and `١٢٣` are not.
The arithmetic that decides anything happens in SQL `numeric`; the service only re-reads back what
it wrote.

**A balance never goes below zero.** A movement that would is *rejected* — `422` on `amount`, nothing
written — never clamped and never partially applied. Two independent layers enforce it: a
`CHECK (balance >= 0)` on `users`, and the `balance + $amt >= 0` predicate in the conditional
`UPDATE` that moves the projection. The second one is what makes a concurrent debit fail cleanly
instead of hitting the constraint.

## The one movement function

`WalletService.Apply(ctx, tx, Movement)` is the only way money moves, and it runs **inside the
caller's transaction** so a checkout (P13) can compose it with an order and grant. In order:

1. validate and normalize the amount;
2. `SELECT balance FROM users WHERE id = $1 AND tenant_id = $2 AND role = 'student' FOR UPDATE` —
   the row lock is what serializes one wallet's movements, and it is what makes `seq` order the
   movement order;
3. an entry with the same `(tenant_id, source_type, source_id)` already exists: return it, write
   nothing (`replayed`), and refuse with a conflict if it moved another amount or another user;
4. `UPDATE users SET balance = balance + $amt WHERE ... AND balance + $amt >= 0 RETURNING balance` —
   zero rows after the lock means insufficient funds;
5. insert the row with `balance_after` = the returned balance, `ON CONFLICT ... DO NOTHING`; a
   caller that lost a unique race comes back with the winner's row, never a `500`.

What P13 must do: call `Apply` once per financial operation inside its own transaction, with one
`source_type`/`source_id` pair that identifies that operation (for example `order`/`<order id>`), and
roll back if it returns ANY error, including `ErrWalletConflict`: that error can come back after the
balance already moved inside the caller's transaction, so committing it would split the balance from the
ledger. Nothing else may write `users.balance`.

## Immutable by trigger

A `BEFORE UPDATE OR DELETE` row trigger and a `BEFORE TRUNCATE` statement trigger on `wallet_entries` raise an exception, and no row is ever
cascaded away: the ledger references `users` with RESTRICT semantics. A correction is a new
adjustment, not an edit. A deleted student would take their history with them, so that is a deliberate
conflict with whatever P22 decides about account deletion (D14).

Tests delete their own ledger rows by switching `session_replication_role` to `replica` on one cleanup
connection; the production trigger is never weakened. The triggers guard against accidental SQL, not
against the table owner or a superuser, so the production application role must own neither
`wallet_entries` nor the database.

## Opening balances

A migrated `users.balance` would otherwise have no history, so the migration writes one `opening` row
per user with a non-zero balance (`amount = balance_after = balance`, `actor_kind = 'system'`,
`source_id = <user id>`), and the seed does the same for the users it gives a balance. From then on
the ledger sum equals the projection, and a mismatch is a bug that can be found by asking the
database. The dev seed also gives taruna's Naila a top-up, a spend and an admin's adjustment, each
anchored to the row before it, and re-running it changes nothing.

## Wire and guards

| Route | Who | Answers |
|---|---|---|
| `GET /api/v1/wallet` | student | balance plus the ledger's credit and debit totals |
| `GET /api/v1/wallet/entries` | student | newest first by `seq`, 50 per page, 100 at most |
| `GET /api/v1/students` | admin, operator | the tenant's students with balances, `q` over name/email/phone |
| `GET /api/v1/students/{id}/wallet` | admin, operator | that student's wallet |
| `GET /api/v1/students/{id}/wallet/entries` | admin, operator | that student's ledger |
| `POST /api/v1/students/{id}/wallet/adjustments` | admin only | `201` a new entry, `200` a replay |

`total_credits` and `total_debits` are summed in SQL, debits as the magnitude of the negative rows.
The listing joins the actor's display name **for staff rows only**, so a student never sees a staff
email or phone. `q` is case-insensitive and `%` or `_` in it match themselves; an id of another tenant,
or of somebody who is not a student, answers `404` with the same message, never `403`. Every route
answers `Cache-Control: no-store`; unknown JSON fields are refused by `bindStrict`, and the
adjustment body is capped at 8 KiB before it is read.

An adjustment needs a `type` (`credit` or `debit`), a positive unsigned `amount` the service signs,
a non-blank `reason`, and an `idempotency_key` of 8–64 characters from `[A-Za-z0-9_-]`. The key is the
row's `source_id`, so the same key and the same body replay (`200`, no new row) while the same key
with another amount, type or student is a `409`. Insufficient funds answers `422` on `amount` with
"Saldo tidak cukup.".

## The web

`app/student/wallet` reads `api.wallet.get` and `api.wallet.entries` and shows the balance, the two
totals, and the ledger with its running balance, type label, signed amount, reference and note, with
loading, empty, error and load-more states. `app/admin/students` lists real accounts with server-side
search; the wallet button (admin only) opens the adjustment dialog, which shows the current balance,
requires an amount and a reason, shows the server's field errors inline, and refreshes the row's
balance on success. Each submit attempt generates one idempotency key and a retry after a network
error reuses it, so a lost response cannot credit twice.

The prototype's **program badge and enrollments columns, the grant-tryout action and the manual
top-up with proof upload are gone**: a student has no program, and enrollments, orders and top-up
requests arrive with P13…P15. The add/edit/delete student controls went too — accounts come from
registration, and student operations are P22.

## What this slice deliberately does not do

No self-service top-up, gateway or manual transfer and no approval queue (P13/P14/P15), no spending
at checkout (P13 — it calls `Apply`), no minimum top-up setting (FR-PAY-W-09), no refunds, exports,
notifications or rate limits, and no capability matrix: D02 is still open, so every guard here is by
role. D12's remaining questions (denomination and rounding for percentages and refunds) are untouched.
