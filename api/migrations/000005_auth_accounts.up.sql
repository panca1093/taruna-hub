-- P03b: one canonical form per identifier, email verification on the account itself (D01), and
-- the previous refresh hash for rotation. Throttles live in Redis; password resets are stateless.

-- Login matches identifiers exactly, so storage must already be canonical: a lowercased email and
-- an E.164 phone. Existing rows are rewritten first; the rewrite is not undone by the down step.
UPDATE users SET email = lower(btrim(email)) WHERE email IS NOT NULL;
UPDATE users SET phone = regexp_replace(phone, '[[:space:]().-]', '', 'g') WHERE phone IS NOT NULL;
UPDATE users SET phone = '+62' || substr(phone, 2) WHERE phone ~ '^0[0-9]+$';
UPDATE users SET phone = '+' || phone WHERE phone ~ '^62[0-9]+$';

ALTER TABLE users
    ADD CONSTRAINT users_email_canonical CHECK (email = lower(btrim(email))),
    ADD CONSTRAINT users_phone_e164 CHECK (phone ~ '^\+[1-9][0-9]{7,14}$'),
    -- D01: verification never blocks login; it is a flag the web app surfaces.
    ADD COLUMN email_verified_at timestamptz,
    -- The outstanding verification code, stored only as an HMAC; all three clear once verified.
    ADD COLUMN email_code_hash bytea CHECK (octet_length(email_code_hash) = 32),
    ADD COLUMN email_code_expires_at timestamptz,
    ADD COLUMN email_code_attempts int NOT NULL DEFAULT 0;

-- Accounts that existed before verification was a thing were provisioned by people, not signups.
UPDATE users SET email_verified_at = created_at WHERE email IS NOT NULL;

-- Rotation keeps the hash it replaced so a replayed old token can be told from an unknown one.
ALTER TABLE auth_sessions
    ADD COLUMN previous_refresh_hash bytea UNIQUE CHECK (octet_length(previous_refresh_hash) = 32),
    ADD COLUMN rotated_at timestamptz;
