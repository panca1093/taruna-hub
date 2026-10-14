-- P00+tenancy baseline: programs, tenants and tenanted users. Rewritable until the first deployment.

CREATE TABLE programs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text        NOT NULL UNIQUE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tenants (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text        NOT NULL UNIQUE
        CONSTRAINT tenants_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$'),
    name       text        NOT NULL,
    is_active  boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE user_role AS ENUM ('student', 'operator', 'admin', 'platform_admin');

CREATE TABLE users (
    id            uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid           REFERENCES tenants (id),
    name          text           NOT NULL,
    email         text,
    phone         text,
    password_hash text           NOT NULL,
    role          user_role      NOT NULL DEFAULT 'student',
    balance       numeric(14, 2) NOT NULL DEFAULT 0,
    created_at    timestamptz    NOT NULL DEFAULT now(),
    -- FR-AUTH-01/02 allow signup and login by email OR phone, so at least one must exist.
    CONSTRAINT users_email_or_phone CHECK (email IS NOT NULL OR phone IS NOT NULL),
    -- A platform admin is the only role outside a tenant, and no tenant role may be tenantless.
    CONSTRAINT users_tenant_role CHECK ((role = 'platform_admin') = (tenant_id IS NULL)),
    -- FR-AUTH-07: the same email or phone may register in several tenants.
    CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email),
    CONSTRAINT users_tenant_phone_key UNIQUE (tenant_id, phone)
);

-- A NULL tenant_id is never equal to another in a UNIQUE constraint, so platform accounts need their own.
CREATE UNIQUE INDEX users_platform_email_key ON users (email) WHERE tenant_id IS NULL;
CREATE UNIQUE INDEX users_platform_phone_key ON users (phone) WHERE tenant_id IS NULL;
