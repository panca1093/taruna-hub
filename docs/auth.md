# Authentication and sessions (P03a, P03b)

`POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout` and
`GET /api/v1/me` (P03a), plus registration, the email code and password reset (P03b). This is the
API half of P03; the web screens arrive with P03c.

## What a request proves

Every `/api/v1/*` request resolves a tenant from its host first, and the token has to agree with it.
`login` reads the identifier only inside that tenant, so the same email in two tenants is two
different people. `Authenticate` then rebuilds the caller from the access token **plus** the session
row and the user row, so a revoked session, a changed role or a deactivated account invalidates an
old token on the very next request. The token's own claims are never trusted for the role or the
tenant on their own.

| Concern | Value |
|---|---|
| Access token | HS256 JWT, 15 minutes, issuer `taruna-api`, audience `taruna-web` |
| Refresh token | 32 random bytes, base64url, stored only as its SHA-256; rotated on every refresh |
| Refresh cookie | `refresh_token`; `Path=/api/v1/auth`; `HttpOnly`; `SameSite=Lax`; `Secure` outside development; `Max-Age` 30 days |
| Idle lifetime | 3 hours (D13), enforced server-side on every authenticated request |

Two rules are easy to get wrong, and both are deliberate:

- **Refreshing is not activity.** It issues a new access token but does not move `last_activity_at`,
  so reloading a page cannot keep a session alive past the idle deadline. Only an authenticated
  request extends it, and at most once a minute per session.
- **`refresh` and `logout` require an `Origin`** that is both accepted by the CORS policy and on the
  host this request resolved to. The CORS middleware still runs the handler for a rejected origin, so
  those two routes — the ones that act on a cookie alone — check it themselves. The dev case of web
  on `:3000` calling the API on `:8081` under the same hostname stays allowed.

**Every refresh rotates the token.** The replaced token keeps working for 30 seconds, so two tabs
that refresh at once both get an access token; the one that lost the race sets no cookie, because the
winner already did. After those 30 seconds a replaced token can only be a copy, so presenting it
revokes the whole session and the current token dies with it.

Logout always answers `204` and always clears the cookie, and revocation takes effect immediately.
Login failures are indistinguishable: an unknown identifier, an inactive account and a wrong password
share one message, and an unknown account still pays for a bcrypt comparison so timing does not
single it out either.

## Identifiers

Emails are stored trimmed and lowercased, phones as E.164 (`0812…`, `62812…` and `+62 812-…` all
become `+62812…`), and check constraints on `users` refuse anything else. Login, registration and
recovery normalize the input the same way, so every spelling reaches the same account.

## Registration and recovery (P03b, D01)

| Route | Answer |
|---|---|
| `POST /auth/register` | `201`, a token body and the cookie — the account exists and is signed in; a code is emailed |
| `POST /me/email/verify` | `204` once the signed-in user enters the emailed code |
| `POST /me/email/resend` | `202` and a fresh code; `409` if already verified |
| `POST /auth/password/forgot` | `202` and no body for every well-formed email |
| `POST /auth/password/reset` | `204`; the password changes, every session is revoked and the email counts as verified |

D01 is settled: the code goes by email over the P02 SMTP transport, email is required at signup and a
phone is optional, so recovery is always an email link. Registration exists only on tenant hosts.

**Verification never blocks login.** Signup creates the `users` row straight away and signs it in;
`email_verified_at` stays empty until the code is entered, and `user.email_verified` on every token
body and `/me` lets the web app show an alert. Because the account is real from the start, its email
and phone are taken from the start too. If someone signs up with an address that is not theirs, the
real owner uses "forgot password": the link reaches the real inbox, and completing it sets their
password, ends the squatter's sessions and verifies the email.

The code lives on the account row as an HMAC keyed from `JWT_SECRET` (`email_code_hash`,
`email_code_expires_at`, `email_code_attempts`), works for 10 minutes and 5 guesses, and the guesses
are counted under a row lock, so a burst of concurrent requests buys no extra tries.

A reset link is `<APP_URL>/reset-password#token=…`, built from
configuration only — never from a request header, or a forged `Origin` could put an attacker's host
into a genuine email. The token sits in the fragment so it never reaches a server log or a `Referer`.
It is stateless: the user id and expiry, signed with an HMAC over the **current password hash**. No
table stores it. Once the password changes, through this link or any other, every outstanding link
stops verifying; the change itself is one conditional `UPDATE`, so two racing submits cannot both win.
Links last 30 minutes, and asking twice leaves both valid until one of them is used.

Codes and links travel only as sealed notification params with an expiry equal to their own, so a
message that would arrive dead is expired instead of sent.

## Self-service profile and password

| Route | Answer |
|---|---|
| `PATCH /me` | `200` and the updated user; body `{name, phone}` |
| `POST /me/password` | `204`; body `{current_password, new_password}` |

Both take any authenticated principal and name no user: the account is always the caller's, and
unknown body fields are refused with `422 fields.body`. A student changes only the name and the
phone; the email is read-only and changes stay with an admin.

`name` is trimmed and 1 to 100 characters. `phone` is optional and goes through the same
`NormalizePhone` as register. Uniqueness is left to the `(tenant, phone)` unique index, so saving the
phone the account already holds is fine and a phone another account holds is `409` with
`fields.phone`. An empty phone clears it only when the account has an email (`users_email_or_phone`
would otherwise reject it), and that case is `422 fields.phone` rather than a database error. The
web app refreshes the header name through `reloadUser` after a save.

The password change needs the current password. A wrong one is `422 fields.current_password`, never
`401`, so the web app does not mistake it for an expired session. `new_password` follows the register
and reset rule. The hash is swapped with a compare-and-set on the hash that was verified, and in the
same transaction every other live session of the user gets `revoked_at`; the caller's own session
(`principal.SessionID`) stays. Their access tokens die on the next request and their refresh tokens
are refused, because the middleware and refresh both read the session row. Unlike a reset, a change
does not mark the email verified.

Wrong guesses are throttled with the login budget (10 / 15 minutes) under their own key, per user, so
guessing against a stolen session cannot lock the owner's login. A success clears it.

## Throttles

Fixed-window counters in Redis (`REDIS_URL`), keyed by a SHA-256 of rule, host scope and normalized
identifier plus the window number. The window comes from the app clock, so tests move it. Attempts are
counted before any password check with one atomic `INCR`, so concurrent requests cannot exceed the
budget. A refusal is `429 too_many_requests` with `Retry-After`. Losing Redis only resets the counters;
while it is unreachable, login, signup and forgot-password answer `500` rather than run unthrottled.

| Rule | Budget |
|---|---|
| Login, per identifier and host | 10 attempts / 15 minutes; a success or a password reset clears it |
| Email codes, per email and tenant | 5 / hour across register and resend, and 60 seconds between resends |
| Forgot password, per email and host | 3 / 15 minutes, counted for unknown emails too |
| Password change, per user | 10 attempts / 15 minutes (login's size, separate key); a success clears it |

A login lockout keyed on the identifier lets someone who knows an address lock its owner out for 15
minutes; the reset link still works and clears it.

## The web app (P03c)

`web/lib/api/client.ts` holds the access token **in memory only**; the refresh cookie is `HttpOnly`
and never readable from JS. A protected call that gets a `401` refreshes once and retries once,
and concurrent callers share one refresh. If that refresh also fails the session is dropped and
`RequireRole` sends the visitor to `/login` (or `/portal` for staff-only pages).
Only a `401`/`403` from the refresh on page load means "not signed in"; a network error or `5xx`
shows a retry screen instead, because it says nothing about the session. Logout clears the local
session only after the server confirmed it, so a failed logout leaves the user signed in and says so.

`SessionProvider` (`web/lib/session-context.tsx`) asks `/auth/refresh` on every full page load, so a
reload keeps the user signed in, and calls `GET /me` when the route changes, at most once a minute.
That call is what keeps the 3-hour idle deadline moving: a refresh alone does not count as
activity, so a tab that makes no API call for 3 hours is signed out by the server. There is no
local auto-logout timer and no role switcher; the role is whatever the server says.

| Route | Does |
|---|---|
| `/login`, `/portal` | `POST /auth/login`, then `/student` or `/admin` by the role the server returns; staff enter the portal directly |
| `/login/staff` | Redirects to `/portal` for existing bookmarks |
| `/register` | `POST /auth/register` (name, email, optional phone, password), then `/verify-email`; server field errors show under the field |
| `/verify-email` | Authenticated account code entry and resend through the existing API; success/skip returns students to `/student` and staff to `/admin` |
| `/forgot-password` | `POST /auth/password/forgot`; always the same confirmation |
| `/reset-password#token=…` | reads the token from the fragment, `POST /auth/password/reset`, then asks the user to log in |

An unverified email shows a non-blocking app-shell reminder linking to `/verify-email`. The code form
handles wrong/expired codes, resend throttles and API errors. A full page load restores the refresh
session before the role guard decides whether verification is available. An already verified user
returns to the dashboard. The public landing and student login do not expose internal portal links.

The student overview reads `/my/dashboard`, enrollments, attempt history and owned kecermatan tests
through the typed client. Its summary is a read-only repeatable-read database snapshot: owned tryout
count, distinct tryouts with completed results, the mean of all persisted attempt scores (including
retakes and zero, rounded to two decimals), and the exact decimal wallet projection. No completed
results means a null average. Kecermatan is excluded from tryout metrics and remains visible among
owned-content links. Recent result cards show only the latest completed attempt per tryout so their
existing result-page destination matches the displayed attempt. Loading and failure never fabricate
zero values; a retry reloads the required data. This page no longer reads the mock layer.

## Not here yet

No capability matrix — the guard is by role only (D02: the Operator is allowed what the PRD leaves ambiguous) — no absolute session cap, nothing per-IP
(that needs a trusted-proxy client-IP decision), and no policy yet on what an unverified email may
not do (for example checkout); verification is only a flag today. `GET /api/v1/me` carries identity only: balance and the ledger are not part of it.
