-- Identifier canonicalization is a data rewrite and stays; only its constraints are dropped.
ALTER TABLE auth_sessions
    DROP COLUMN rotated_at,
    DROP COLUMN previous_refresh_hash;

ALTER TABLE users
    DROP COLUMN email_code_attempts,
    DROP COLUMN email_code_expires_at,
    DROP COLUMN email_code_hash,
    DROP COLUMN email_verified_at,
    DROP CONSTRAINT users_phone_e164,
    DROP CONSTRAINT users_email_canonical;
