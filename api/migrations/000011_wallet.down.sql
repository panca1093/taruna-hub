DROP TRIGGER IF EXISTS wallet_entries_no_truncate ON wallet_entries;
DROP TRIGGER IF EXISTS wallet_entries_immutable ON wallet_entries;
DROP TABLE IF EXISTS wallet_entries;
DROP FUNCTION IF EXISTS wallet_entries_reject_change();

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_balance_nonneg,
    DROP CONSTRAINT IF EXISTS users_id_tenant_key;
