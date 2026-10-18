-- P12: wallet_entries is the signed ledger and users.balance its projection (D12, exact numeric).

ALTER TABLE users
    ADD CONSTRAINT users_balance_nonneg CHECK (balance >= 0),
    -- Makes (id, tenant_id) referenceable, so a ledger row can never point at another tenant's user.
    ADD CONSTRAINT users_id_tenant_key UNIQUE (id, tenant_id);

CREATE TABLE wallet_entries (
    id            uuid           PRIMARY KEY DEFAULT gen_random_uuid(),
    -- A total order for balance chains: seq order is the order the money moved.
    seq           bigint         GENERATED ALWAYS AS IDENTITY UNIQUE,
    tenant_id     uuid           NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id       uuid           NOT NULL,
    amount        numeric(14, 2) NOT NULL
        CONSTRAINT wallet_entries_amount_nonzero CHECK (amount <> 0),
    balance_after numeric(14, 2) NOT NULL
        CONSTRAINT wallet_entries_balance_after_nonneg CHECK (balance_after >= 0),
    kind          text           NOT NULL
        CONSTRAINT wallet_entries_kind_check CHECK (kind IN ('opening', 'topup', 'spend', 'adjustment')),
    actor_kind    text           NOT NULL
        CONSTRAINT wallet_entries_actor_kind_check
        CHECK (actor_kind IN ('system', 'student', 'provider', 'staff', 'platform')),
    actor_id      uuid           REFERENCES users (id) ON DELETE RESTRICT,
    reason        text           NOT NULL DEFAULT '',
    source_type   text           NOT NULL,
    source_id     text           NOT NULL,
    created_at    timestamptz    NOT NULL DEFAULT now(),
    -- One financial operation moves money once, whatever produced the retry.
    CONSTRAINT wallet_entries_source_key UNIQUE (tenant_id, source_type, source_id),
    CONSTRAINT wallet_entries_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT wallet_entries_adjustment_actor CHECK (
        kind <> 'adjustment'
        OR (actor_kind IN ('staff', 'platform') AND btrim(reason) <> '' AND actor_id IS NOT NULL))
);

CREATE INDEX wallet_entries_user_seq_idx ON wallet_entries (user_id, seq DESC);

CREATE FUNCTION wallet_entries_reject_change() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'wallet_entries is immutable' USING ERRCODE = '23514';
END;
$$;

CREATE TRIGGER wallet_entries_immutable
    BEFORE UPDATE OR DELETE ON wallet_entries
    FOR EACH ROW EXECUTE FUNCTION wallet_entries_reject_change();

-- A row trigger never sees TRUNCATE, which would erase the whole ledger.
CREATE TRIGGER wallet_entries_no_truncate
    BEFORE TRUNCATE ON wallet_entries
    FOR EACH STATEMENT EXECUTE FUNCTION wallet_entries_reject_change();

-- A migrated balance is explained by one opening row, so the ledger sum equals the projection at once.
INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind, reason,
        source_type, source_id)
SELECT u.tenant_id, u.id, u.balance, u.balance, 'opening', 'system', '', 'opening', u.id::text
FROM users u
WHERE u.tenant_id IS NOT NULL
  AND u.balance <> 0
  AND NOT EXISTS (
      SELECT 1 FROM wallet_entries e
       WHERE e.tenant_id = u.tenant_id
         AND e.source_type = 'opening'
         AND e.source_id = u.id::text);
