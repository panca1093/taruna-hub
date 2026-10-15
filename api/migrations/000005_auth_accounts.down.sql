-- Identifier canonicalization is a data rewrite and stays; only its constraints are dropped.
DROP TABLE auth_throttles;

ALTER TABLE auth_sessions
    DROP COLUMN rotated_at,
    DROP COLUMN previous_refresh_hash;

DROP TABLE auth_password_resets;
DROP TABLE auth_registrations;

ALTER TABLE users
    DROP CONSTRAINT users_phone_e164,
    DROP CONSTRAINT users_email_canonical;
