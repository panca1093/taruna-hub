# Products and the public catalog (P11)

A product is what a school sells: one tryout (`individual`) or a package of two to twenty
(`bundle`). Products are tenant content — an Admin or an Operator writes them — and each tenant's
catalog is served from its own host, anonymously.

    products          id, tenant_id, type, title, description, program_id, price, original_price,
                      is_active, active_until, created_by, created_at, updated_at
                        UNIQUE (id, tenant_id)
                        UNIQUE (tenant_id, lower(title))          -- as an index
    product_tryouts   product_id, tryout_id, tenant_id, position
                        PRIMARY KEY (product_id, tryout_id)
                        UNIQUE (product_id, position) DEFERRABLE INITIALLY DEFERRED

Both children reach their parent through the tenant pair (`REFERENCES ... (id, tenant_id)`), so an
id from another tenant fails in SQL even if a query filter were ever wrong. Deleting a tryout a
product includes answers `409` through the existing P08 mapping, and deleting a program a product
uses answers `409` through the P05 mapping; the tests prove both. Deleting a product removes its
links and the row in one transaction.

## Money (D12)

`price` and `original_price` are `numeric(14,2)` and travel as canonical decimal strings — `"150000.00"`
— from the database to the browser. No `float64` appears in Go, no `::float` in SQL, and the web only
formats the string for display (`lib/format-money.ts`) and sends back the string it was given or one
built from what the user typed. The service parses a plain decimal with at most two fraction digits and
no sign, whitespace or exponent: `1e3`, `NaN`, `-1`, `1.234`, `" 5"` and `""` are all `422 price`;
`"1"` becomes `"1.00"` and `"99.9"` becomes `"99.90"`. A price must be greater than zero and at most
`9999999999.99`; `original_price`, when present, is at least `price`.

## What makes a product public

A product is listed and readable publicly only when all of these hold at request time:

1. `is_active` is true;
2. its own `active_until` is null or still in the future;
3. every attached tryout is `published`;
4. every attached tryout's own `active_until` is still in the future.

The predicate lives in one SQL fragment in `internal/repository/product.go` and is used by the list,
the detail and the summary, with `now` passed in from the service's injected clock (`WithProductClock`)
so tests move time instead of sleeping. A tryout that is unpublished after the product was saved is
not blocked — P08 owns publishing — the product simply stops being visible on the next request, with
no restart or deploy. Restoring the tryout restores the product.

## Rules, and where they live

`internal/service/product.go` owns every rule; repository methods take the tenant scope and only
persist, returning `ErrScopeRequired` for `uuid.Nil`. Title is trimmed to 1–200 characters and unique
case-insensitively per tenant; description is at most 4000; `program_id` must exist in the tenant. Each
tryout id must exist in the tenant, be `published` and belong to the product's program; a duplicate, a
draft, another tenant's id and a missing id all answer the same `422` on `tryout_ids[i]`, so existence
never leaks. `individual` needs exactly one tryout, `bundle` two to twenty. Setting `is_active` re-reads
the written links inside the transaction, so a concurrent unpublish cannot slip through, and concurrent
saves hold the product row lock, so the stored package is always exactly one caller's set.

`created_by` is the signed-in principal.

## Wire and guards

Admin (`GET/POST /api/v1/products`, `GET/PUT/DELETE /api/v1/products/{id}`) is Admin + Operator only —
D02 is still open, so the guard is by role; anonymous answers `401`, a Student `403`, and another
tenant's id `404`, never `403`. Responses carry `Cache-Control: no-store`, unknown JSON fields are
refused by `bindStrict`, and the body is capped at 64 KiB before it is mapped, so an oversized package
is one error rather than tens of thousands of field errors. `PUT` replaces every editable field and the
whole `tryout_ids` array.

The public side (`GET /api/v1/catalog/products`, `/api/v1/catalog/products/{id}`,
`/api/v1/catalog/summary`) is unauthenticated and resolves its tenant from the host exactly like
`GET /api/v1/tenant`: the platform host and unknown hosts answer the same `404`. Tenant ids come from
that scope and never from the client, and the body is display data only — title, description, program
code and name, the price strings, `discount_percent` computed on the server from integer cents, the
window and per-tryout counts. No answer keys, question content, `created_by`, revision, author or other
tenant appears in a list, a detail, a summary or an error body.

The catalog answers `Cache-Control: no-store`. A 30-second shared cache would need `Vary: Host`, but a
deployment that trusts `X-Forwarded-Host` resolves the tenant from that header instead, so two hosts
can share one `Host` value; per-host caching is not provably safe today, so it was not enabled.

There is no new rate limiter: the existing Redis throttle is built for authenticated rules with a
named identifier, and a per-IP budget needs the trusted-proxy client-IP decision that `docs/auth.md`
already lists as pending for auth.

## The web

`app/admin/products` is the staff table: real list, create, edit, delete, "Muat lebih banyak", inline
server field errors, toasts, and a tryout picker fed by `GET /api/v1/tryouts?status=published&program_id=`.
The type choice offers only `individual` and `bundle`, because there is no kecermatan test entity yet.

`app/catalog` and `app/catalog/[id]` read the public API only: program, type and title filters, loading,
empty and error states, and prices formatted from the string. The "Beli" button is still a link into
checkout, which stays P13; it says so instead of pretending a purchase happened.

`app/page.tsx` takes its featured products (first four by newest) and its two numeric stats from the
public catalog. The unverifiable "1.000+ siswa" claim is gone: the third figure is now the tenant's real
number of visible tryouts. That is a copy change on the landing for Panca to review.

## What this slice deliberately does not do

No kecermatan products (P09/P10 own the test entity), no checkout, orders, payments or wallet spending
(P13/P14), no cover images (P06 deferred, so the prototype's gradient swatch is a token now), no
master/copy, no platform (root) landing or featured ownership (D06), no recommendation or ranking
engine, and no search beyond a case-insensitive title substring with `%` and `_` matching themselves.
