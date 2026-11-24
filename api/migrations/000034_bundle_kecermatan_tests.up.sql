-- 2026-11-24: a bundle may sell kecermatan tests next to its tryouts; each purchase credits every member.

CREATE TABLE product_kecermatan_tests (
    product_id         uuid NOT NULL,
    kecermatan_test_id uuid NOT NULL,
    tenant_id          uuid NOT NULL,
    position           int  NOT NULL CHECK (position >= 1),
    CONSTRAINT product_kecermatan_tests_pkey PRIMARY KEY (product_id, kecermatan_test_id),
    -- Deferrable so one save can reorder the package inside a single transaction.
    CONSTRAINT product_kecermatan_tests_product_position_key UNIQUE (product_id, position) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT product_kecermatan_tests_product_tenant_fkey FOREIGN KEY (product_id, tenant_id)
        REFERENCES products (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT product_kecermatan_tests_test_tenant_fkey FOREIGN KEY (kecermatan_test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX product_kecermatan_tests_test_idx ON product_kecermatan_tests (kecermatan_test_id);

-- The checkout snapshot of a bundle's kecermatan tests, so a later product edit never changes what was bought.
CREATE TABLE order_kecermatan_items (
    order_id           uuid NOT NULL,
    tenant_id          uuid NOT NULL,
    kecermatan_test_id uuid NOT NULL,
    test_title         text NOT NULL,
    position           int  NOT NULL CHECK (position >= 1),
    CONSTRAINT order_kecermatan_items_pkey PRIMARY KEY (order_id, kecermatan_test_id),
    CONSTRAINT order_kecermatan_items_order_tenant_fkey FOREIGN KEY (order_id, tenant_id)
        REFERENCES orders (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT order_kecermatan_items_test_tenant_fkey FOREIGN KEY (kecermatan_test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX order_kecermatan_items_test_idx ON order_kecermatan_items (kecermatan_test_id, tenant_id);
