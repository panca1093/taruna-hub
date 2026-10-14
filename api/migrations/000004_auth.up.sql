-- P03a: server-side sessions behind the access token, plus account deactivation.
-- A refresh token is only ever stored as its SHA-256 (32 bytes), so a database dump mints nothing.

ALTER TABLE users ADD COLUMN is_active boolean NOT NULL DEFAULT true;

CREATE TABLE auth_sessions (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid        NOT NULL REFERENCES users (id),
    -- NULL means the platform scope, exactly as on users.
    tenant_id        uuid        REFERENCES tenants (id),
    refresh_hash     bytea       NOT NULL UNIQUE CHECK (octet_length(refresh_hash) = 32),
    created_at       timestamptz NOT NULL,
    last_activity_at timestamptz NOT NULL,
    revoked_at       timestamptz
);

-- Logout and idle expiry only ever filter live sessions of one user.
CREATE INDEX auth_sessions_user_live_idx ON auth_sessions (user_id) WHERE revoked_at IS NULL;
