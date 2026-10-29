-- P15: self-service top-up requests (D12) and the tenant's minimum amount.

-- D12: whole rupiah, default Rp10.000; Admin changes it with the other tenant settings.
ALTER TABLE tenant_settings
    ADD COLUMN min_topup numeric(14,2) NOT NULL DEFAULT 10000
        CONSTRAINT tenant_settings_min_topup_check CHECK (min_topup >= 1 AND min_topup = trunc(min_topup));

-- A request is the evidence; the wallet ledger (source 'topup', source_id = id) is the money. Gateway top-ups
-- (P14) reuse the status and the credit invariant, and add their provider evidence in their own tables.
CREATE TABLE topup_requests (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    number          bigint        GENERATED ALWAYS AS IDENTITY UNIQUE,
    tenant_id       uuid          NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id         uuid          NOT NULL,
    amount          numeric(14,2) NOT NULL CHECK (amount >= 1 AND amount = trunc(amount)),
    method          text          NOT NULL CHECK (method IN ('manual', 'gateway')),
    status          text          NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    -- A client-chosen operation identity: a retried request finds its row instead of making another.
    idempotency_key text          NOT NULL,
    proof_asset_id  uuid          NULL,
    reject_reason   text          NOT NULL DEFAULT '',
    reviewed_by     uuid          NULL REFERENCES users (id) ON DELETE RESTRICT,
    approved_at     timestamptz   NULL,
    rejected_at     timestamptz   NULL,
    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT topup_requests_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT topup_requests_idempotency_key UNIQUE (tenant_id, user_id, idempotency_key),
    CONSTRAINT topup_requests_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT topup_requests_proof_tenant_fkey FOREIGN KEY (tenant_id, proof_asset_id)
        REFERENCES assets (tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT topup_requests_approved_check CHECK ((status = 'approved') = (approved_at IS NOT NULL)),
    CONSTRAINT topup_requests_rejected_check CHECK (
        (status = 'rejected') = (rejected_at IS NOT NULL)
        AND (status <> 'rejected' OR btrim(reject_reason) <> '')),
    -- A transfer is reviewed against its proof; a gateway payment is evidenced by the provider instead.
    CONSTRAINT topup_requests_proof_check CHECK ((method = 'manual') = (proof_asset_id IS NOT NULL))
);

-- One upload backs one request, so a proof cannot be replayed on a second top-up.
CREATE UNIQUE INDEX topup_requests_proof_key ON topup_requests (proof_asset_id) WHERE proof_asset_id IS NOT NULL;
CREATE INDEX topup_requests_tenant_status_idx ON topup_requests (tenant_id, status, created_at DESC, id DESC);
CREATE INDEX topup_requests_user_idx ON topup_requests (tenant_id, user_id, created_at DESC, id DESC);
