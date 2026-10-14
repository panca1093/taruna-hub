-- P02 durable notifications. One row per business event / recipient / channel identity.

CREATE TABLE notification_logs (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid        REFERENCES tenants (id),
    event_key     text        NOT NULL,
    channel       text        NOT NULL CHECK (channel IN ('email', 'whatsapp')),
    recipient     text        NOT NULL,
    template      text        NOT NULL,
    params        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    sealed_params bytea,
    expires_at    timestamptz,
    status        text        NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'sent', 'failed', 'expired', 'unavailable')),
    attempts      int         NOT NULL DEFAULT 0,
    last_outcome  text        CHECK (last_outcome IN ('ok', 'transient', 'permanent', 'uncertain')),
    last_error    text,
    claim_token   uuid,
    claimed_until timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    -- A row is unfinished exactly while it is queued, so a terminal row always has a finish time.
    CONSTRAINT notification_logs_finished CHECK ((status = 'queued') = (finished_at IS NULL)),
    -- Sealed bytes exist only while the message can still be delivered.
    CONSTRAINT notification_logs_sealed CHECK (status = 'queued' OR sealed_params IS NULL)
);

-- Identity is (tenant, event, recipient, channel), and the all-zero sentinel lets the null tenant take part.
CREATE UNIQUE INDEX notification_logs_identity_key ON notification_logs (
    (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid)), event_key, recipient, channel);

-- Reconcile scans queued rows older than a cutoff with no live claim.
CREATE INDEX notification_logs_reconcile_idx ON notification_logs (created_at, id)
    WHERE status = 'queued';
