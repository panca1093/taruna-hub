-- P03b: one canonical form per identifier, pending registrations behind an email code (D01),
-- single-use password resets, refresh rotation and fixed-window auth throttles.

-- Login matches identifiers exactly, so storage must already be canonical: a lowercased email and
-- an E.164 phone. Existing rows are rewritten first; the rewrite is not undone by the down step.
UPDATE users SET email = lower(btrim(email)) WHERE email IS NOT NULL;
UPDATE users SET phone = regexp_replace(phone, '[[:space:]().-]', '', 'g') WHERE phone IS NOT NULL;
UPDATE users SET phone = '+62' || substr(phone, 2) WHERE phone ~ '^0[0-9]+$';
UPDATE users SET phone = '+' || phone WHERE phone ~ '^62[0-9]+$';

ALTER TABLE users
    ADD CONSTRAINT users_email_canonical CHECK (email = lower(btrim(email))),
    ADD CONSTRAINT users_phone_e164 CHECK (phone ~ '^\+[1-9][0-9]{7,14}$');

-- A signup is not an account until its email code is verified, so it never occupies users.
-- One pending row per (tenant, email): signing up again replaces the code and the details.
CREATE TABLE auth_registrations (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid        NOT NULL REFERENCES tenants (id),
    email         text        NOT NULL CHECK (email = lower(btrim(email))),
    phone         text        CHECK (phone ~ '^\+[1-9][0-9]{7,14}$'),
    name          text        NOT NULL,
    password_hash text        NOT NULL,
    -- code_id changes with every code sent; it is the notification identity and the HMAC context.
    code_id       uuid        NOT NULL,
    code_hash     bytea       NOT NULL CHECK (octet_length(code_hash) = 32),
    attempts      int         NOT NULL DEFAULT 0,
    expires_at    timestamptz NOT NULL,
    sent_at       timestamptz NOT NULL,
    created_at    timestamptz NOT NULL,
    CONSTRAINT auth_registrations_tenant_email_key UNIQUE (tenant_id, email)
);

-- A reset token is stored only as its SHA-256; consumed_at makes it single use.
CREATE TABLE auth_password_resets (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tenant_id   uuid        REFERENCES tenants (id),
    token_hash  bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL
);

CREATE INDEX auth_password_resets_user_live_idx ON auth_password_resets (user_id) WHERE consumed_at IS NULL;

-- Rotation keeps the hash it replaced so a replayed old token can be told from an unknown one.
ALTER TABLE auth_sessions
    ADD COLUMN previous_refresh_hash bytea UNIQUE CHECK (octet_length(previous_refresh_hash) = 32),
    ADD COLUMN rotated_at timestamptz;

-- One counter per throttle key; the key is a SHA-256 so no identifier is stored in clear.
CREATE TABLE auth_throttles (
    key_hash     bytea       PRIMARY KEY CHECK (octet_length(key_hash) = 32),
    window_start timestamptz NOT NULL,
    hits         int         NOT NULL
);
