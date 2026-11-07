-- P20/P21: durable export artifacts. One row per requested PDF or CSV/XLSX; the River job renders it and the
-- object lives under <tenant_id>/export/<id>. The cut-off is captured at request time so a late render still
-- describes the data the requester saw.
CREATE TABLE exports (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    kind         text        NOT NULL
        CONSTRAINT exports_kind_check CHECK (kind IN ('tryout_pdf', 'tryout_results_csv', 'tryout_results_xlsx')),
    params       jsonb       NOT NULL,
    status       text        NOT NULL
        CONSTRAINT exports_status_check CHECK (status IN ('queued', 'running', 'ready', 'failed')),
    requested_by uuid        NOT NULL,
    cut_off_at   timestamptz NOT NULL,
    object_key   text,
    content_type text,
    file_name    text,
    size_bytes   bigint,
    error_code   text        NOT NULL DEFAULT '',
    attempts     integer     NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    CONSTRAINT exports_tenant_id_key UNIQUE (id, tenant_id),
    CONSTRAINT exports_requested_by_fkey FOREIGN KEY (requested_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT exports_ready_check CHECK (
        status <> 'ready' OR (object_key IS NOT NULL AND file_name IS NOT NULL AND content_type IS NOT NULL))
);

CREATE INDEX exports_tenant_kind_created_idx ON exports (tenant_id, kind, created_at DESC, id DESC);
