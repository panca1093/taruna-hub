-- P13: product sale window, order snapshots and the enrollments a confirmed order opens (D05).

ALTER TABLE products
    ADD COLUMN active_from timestamptz NULL,
    ADD CONSTRAINT products_window_check CHECK (active_from IS NULL OR active_until IS NULL OR active_from < active_until);

CREATE TABLE orders (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The human-facing order code is derived from this, so it is short and ordered, never guessed from an id.
    number          bigint        GENERATED ALWAYS AS IDENTITY UNIQUE,
    tenant_id       uuid          NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id         uuid          NOT NULL,
    -- No foreign key: the snapshot below is what was bought, so deleting or editing the product changes nothing here.
    product_id      uuid          NOT NULL,
    product_type    text          NOT NULL CHECK (product_type IN ('individual', 'bundle')),
    product_title   text          NOT NULL,
    price           numeric(14,2) NOT NULL CHECK (price > 0),
    payment_method  text          NOT NULL CHECK (payment_method IN ('manual', 'wallet')),
    status          text          NOT NULL CHECK (status IN ('pending', 'confirmed', 'cancelled')),
    -- A client-chosen operation identity: a retried checkout finds its order instead of making another.
    idempotency_key text          NOT NULL,
    proof_asset_id  uuid          NULL,
    cancel_reason   text          NOT NULL DEFAULT '',
    reviewed_by     uuid          NULL REFERENCES users (id) ON DELETE RESTRICT,
    confirmed_at    timestamptz   NULL,
    cancelled_at    timestamptz   NULL,
    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT orders_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT orders_idempotency_key UNIQUE (tenant_id, user_id, idempotency_key),
    CONSTRAINT orders_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT orders_proof_tenant_fkey FOREIGN KEY (tenant_id, proof_asset_id)
        REFERENCES assets (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT orders_confirmed_check CHECK ((status = 'confirmed') = (confirmed_at IS NOT NULL)),
    CONSTRAINT orders_cancelled_check CHECK (
        (status = 'cancelled') = (cancelled_at IS NOT NULL)
        AND (status <> 'cancelled' OR btrim(cancel_reason) <> '')),
    -- A transfer is reviewed against its proof; a wallet payment has nothing to prove.
    CONSTRAINT orders_proof_check CHECK ((payment_method = 'manual') = (proof_asset_id IS NOT NULL))
);

-- One upload backs one order, so a proof cannot be replayed on a second purchase.
CREATE UNIQUE INDEX orders_proof_key ON orders (proof_asset_id) WHERE proof_asset_id IS NOT NULL;
CREATE INDEX orders_tenant_status_idx ON orders (tenant_id, status, created_at DESC, id DESC);
CREATE INDEX orders_user_idx ON orders (tenant_id, user_id, created_at DESC, id DESC);

CREATE TABLE order_items (
    order_id     uuid NOT NULL,
    tenant_id    uuid NOT NULL,
    tryout_id    uuid NOT NULL,
    tryout_title text NOT NULL,
    position     int  NOT NULL CHECK (position >= 1),
    CONSTRAINT order_items_pkey PRIMARY KEY (order_id, tryout_id),
    CONSTRAINT order_items_order_tenant_fkey FOREIGN KEY (order_id, tenant_id)
        REFERENCES orders (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT order_items_tryout_tenant_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT
);

-- One row per user and tryout. A tryout's access window is the tryout's own (D05a), so active_until stays
-- NULL for it; a kecermatan purchase (P18) adds attempts_purchased to the same row instead of a new one.
CREATE TABLE enrollments (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id            uuid        NOT NULL,
    tryout_id          uuid        NOT NULL,
    active_until       timestamptz NULL,
    attempts_purchased int         NULL CHECK (attempts_purchased IS NULL OR attempts_purchased > 0),
    attempts_used      int         NOT NULL DEFAULT 0 CHECK (attempts_used >= 0),
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT enrollments_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT enrollments_holder_key UNIQUE (tenant_id, user_id, tryout_id),
    CONSTRAINT enrollments_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT enrollments_tryout_tenant_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX enrollments_user_idx ON enrollments (tenant_id, user_id, created_at DESC);

-- Why an enrollment exists. The unique key is the last guard against a retry granting the same source twice.
CREATE TABLE enrollment_grants (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid        NOT NULL,
    enrollment_id uuid        NOT NULL,
    order_id      uuid        NULL,
    source_type   text        NOT NULL CHECK (source_type IN ('order', 'staff')),
    source_id     text        NOT NULL,
    actor_kind    text        NOT NULL CHECK (actor_kind IN ('system', 'student', 'provider', 'staff', 'platform')),
    actor_id      uuid        NULL REFERENCES users (id) ON DELETE RESTRICT,
    reason        text        NOT NULL DEFAULT '',
    attempts      int         NULL CHECK (attempts IS NULL OR attempts > 0),
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT enrollment_grants_source_key UNIQUE (tenant_id, source_type, source_id, enrollment_id),
    CONSTRAINT enrollment_grants_enrollment_fkey FOREIGN KEY (enrollment_id, tenant_id)
        REFERENCES enrollments (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT enrollment_grants_order_fkey FOREIGN KEY (order_id, tenant_id)
        REFERENCES orders (id, tenant_id) ON DELETE RESTRICT,
    -- A grant without an order must say who gave it and why.
    CONSTRAINT enrollment_grants_staff_check CHECK (
        source_type <> 'staff' OR (actor_id IS NOT NULL AND btrim(reason) <> ''))
);
