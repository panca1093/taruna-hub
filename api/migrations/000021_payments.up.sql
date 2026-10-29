-- P14: Midtrans gateway orders and top-ups (D07, D16). One platform-owned merchant account serves the deployment.

ALTER TABLE orders DROP CONSTRAINT orders_payment_method_check;
ALTER TABLE orders ADD CONSTRAINT orders_payment_method_check CHECK (payment_method IN ('manual', 'wallet', 'gateway'));

-- One attempt per gateway order or top-up. provider_order_id is what Midtrans knows; it is globally unique and a
-- retry of an ambiguous create reuses it, so one business operation can never become two payments.
CREATE TABLE payment_attempts (
    id                uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid          NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id           uuid          NOT NULL,
    provider          text          NOT NULL CHECK (provider = 'midtrans'),
    provider_order_id text          NOT NULL CONSTRAINT payment_attempts_provider_order_key UNIQUE,
    order_id          uuid          NULL,
    topup_id          uuid          NULL,
    amount            numeric(14,2) NOT NULL CHECK (amount >= 1),
    -- pending: waiting for the payer; failed: a deny that may still be retried by the payer; paid: money applied;
    -- cancelled: expired or cancelled at the provider; held: a success that needs staff review (D07).
    status            text          NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'failed', 'paid', 'cancelled', 'held')),
    redirect_url      text          NULL,
    expires_at        timestamptz   NOT NULL,
    last_checked_at   timestamptz   NULL,
    paid_at           timestamptz   NULL,
    created_at        timestamptz   NOT NULL DEFAULT now(),
    updated_at        timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT payment_attempts_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT payment_attempts_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_attempts_order_fkey FOREIGN KEY (order_id, tenant_id)
        REFERENCES orders (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_attempts_topup_fkey FOREIGN KEY (topup_id, tenant_id)
        REFERENCES topup_requests (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_attempts_target_check CHECK ((order_id IS NULL) <> (topup_id IS NULL))
);
CREATE UNIQUE INDEX payment_attempts_order_key ON payment_attempts (order_id) WHERE order_id IS NOT NULL;
CREATE UNIQUE INDEX payment_attempts_topup_key ON payment_attempts (topup_id) WHERE topup_id IS NOT NULL;
CREATE INDEX payment_attempts_open_idx ON payment_attempts (expires_at) WHERE status IN ('pending', 'failed');

-- Raw provider evidence, kept even when it changed nothing. A byte-identical replay is stored once.
CREATE TABLE payment_events (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id            uuid        NOT NULL,
    tenant_id             uuid        NOT NULL,
    source                text        NOT NULL CHECK (source IN ('notification', 'status')),
    transaction_status    text        NOT NULL,
    fraud_status          text        NOT NULL DEFAULT '',
    status_code           text        NOT NULL DEFAULT '',
    gross_amount          text        NOT NULL DEFAULT '',
    provider_transaction_id text      NOT NULL DEFAULT '',
    body_hash             text        NOT NULL,
    raw                   jsonb       NOT NULL,
    -- What the event did: applied, ignored, held (exception), evidence (refund/chargeback).
    outcome               text        NOT NULL DEFAULT 'ignored' CHECK (outcome IN ('applied', 'ignored', 'held', 'evidence')),
    received_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payment_events_attempt_fkey FOREIGN KEY (attempt_id, tenant_id)
        REFERENCES payment_attempts (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_events_dedupe_key UNIQUE (attempt_id, source, body_hash)
);

-- A success the system refused to apply by itself. Staff credit the wallet once or mark it refunded outside.
CREATE TABLE payment_exceptions (
    id           uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid          NOT NULL,
    attempt_id   uuid          NOT NULL,
    user_id      uuid          NOT NULL,
    order_id     uuid          NULL,
    topup_id     uuid          NULL,
    kind         text          NOT NULL CHECK (kind IN ('late_success', 'amount_mismatch', 'unexpected_state')),
    paid_amount  numeric(14,2) NOT NULL CHECK (paid_amount >= 1),
    status       text          NOT NULL DEFAULT 'held' CHECK (status IN ('held', 'credited', 'refunded')),
    resolved_by  uuid          NULL REFERENCES users (id) ON DELETE RESTRICT,
    resolve_reason text        NOT NULL DEFAULT '',
    resolved_at  timestamptz   NULL,
    created_at   timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT payment_exceptions_attempt_key UNIQUE (attempt_id),
    CONSTRAINT payment_exceptions_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT payment_exceptions_attempt_fkey FOREIGN KEY (attempt_id, tenant_id)
        REFERENCES payment_attempts (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_exceptions_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT payment_exceptions_resolved_check CHECK (
        (status = 'held') = (resolved_at IS NULL)
        AND (status = 'held' OR (resolved_by IS NOT NULL AND btrim(resolve_reason) <> '')))
);
CREATE INDEX payment_exceptions_tenant_status_idx ON payment_exceptions (tenant_id, status, created_at DESC);
