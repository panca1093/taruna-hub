-- P11: products; money is exact numeric (D12) and a product is public only while it and its tryouts are live.

CREATE TABLE products (
    id             uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid          NOT NULL REFERENCES tenants (id),
    type           text          NOT NULL CHECK (type IN ('individual', 'bundle')),
    title          text          NOT NULL,
    description    text          NOT NULL DEFAULT '',
    program_id     uuid          NOT NULL,
    price          numeric(14,2) NOT NULL CHECK (price > 0),
    original_price numeric(14,2) NULL CHECK (original_price IS NULL OR original_price >= price),
    is_active      boolean       NOT NULL DEFAULT false,
    active_until   timestamptz   NULL,
    created_by     uuid          NOT NULL REFERENCES users (id),
    created_at     timestamptz   NOT NULL DEFAULT now(),
    updated_at     timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT products_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT products_program_tenant_fkey FOREIGN KEY (program_id, tenant_id)
        REFERENCES programs (id, tenant_id) ON DELETE RESTRICT
);

-- Title uniqueness is case-insensitive per tenant, matching what the service reports as 422 title.
CREATE UNIQUE INDEX products_tenant_title_key ON products (tenant_id, lower(title));
CREATE INDEX products_tenant_created_idx ON products (tenant_id, created_at DESC);

CREATE TABLE product_tryouts (
    product_id uuid NOT NULL,
    tryout_id  uuid NOT NULL,
    tenant_id  uuid NOT NULL,
    position   int  NOT NULL CHECK (position >= 1),
    CONSTRAINT product_tryouts_pkey PRIMARY KEY (product_id, tryout_id),
    -- Deferrable so one save can reorder the package inside a single transaction.
    CONSTRAINT product_tryouts_product_position_key UNIQUE (product_id, position) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT product_tryouts_product_tenant_fkey FOREIGN KEY (product_id, tenant_id)
        REFERENCES products (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT product_tryouts_tryout_tenant_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX product_tryouts_tryout_idx ON product_tryouts (tryout_id);
