-- P06: assets holds object metadata only; the bytes live in S3-compatible storage under object_key.

CREATE TABLE assets (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    purpose      text        NOT NULL
        CONSTRAINT assets_purpose_check CHECK (purpose IN ('question_image', 'payment_proof')),
    -- Server-generated as <tenant_id>/<purpose>/<id>; nothing from the client reaches it.
    object_key   text        NOT NULL CONSTRAINT assets_object_key_key UNIQUE,
    content_type text        NOT NULL,
    size_bytes   bigint      NOT NULL
        CONSTRAINT assets_size_check CHECK (size_bytes BETWEEN 1 AND 5242880),
    status       text        NOT NULL DEFAULT 'staged'
        CONSTRAINT assets_status_check CHECK (status IN ('staged', 'final')),
    uploaded_by  uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    finalized_at timestamptz,
    CONSTRAINT assets_tenant_id_key UNIQUE (tenant_id, id),
    CONSTRAINT assets_uploader_tenant_fkey FOREIGN KEY (uploaded_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT assets_finalized_check CHECK ((status = 'final') = (finalized_at IS NOT NULL))
);

-- The purge scans only staged rows by age.
CREATE INDEX assets_staged_created_idx ON assets (created_at) WHERE status = 'staged';
