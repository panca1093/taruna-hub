-- S0 baseline: only what the walking skeleton needs. Later slices add their own tables.

CREATE TABLE programs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text        NOT NULL UNIQUE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE user_role AS ENUM ('student', 'operator', 'admin');

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text        NOT NULL,
    email         text        UNIQUE,
    phone         text        UNIQUE,
    password_hash text        NOT NULL,
    role          user_role   NOT NULL DEFAULT 'student',
    balance       numeric(14, 2) NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    -- FR-AUTH-01/02 allow signup and login by email OR phone, so at least one must exist.
    CONSTRAINT users_email_or_phone CHECK (email IS NOT NULL OR phone IS NOT NULL)
);
