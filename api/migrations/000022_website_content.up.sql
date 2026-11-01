-- P11b: the marketing website content. One row per tenant holds the private draft and the last published
-- whole-site snapshot; website_publications is the append-only audit trail of publications.
CREATE TABLE website_content (
    tenant_id          uuid        PRIMARY KEY REFERENCES tenants (id) ON DELETE RESTRICT,
    schema_version     integer     NOT NULL CHECK (schema_version = 1),
    draft              jsonb       NOT NULL,
    draft_revision     bigint      NOT NULL CHECK (draft_revision BETWEEN 1 AND 9007199254740991),
    published          jsonb       NULL,
    published_revision bigint      NOT NULL DEFAULT 0 CHECK (published_revision BETWEEN 0 AND 9007199254740991),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    updated_by         uuid        NULL,
    published_at       timestamptz NULL,
    published_by       uuid        NULL,
    CONSTRAINT website_content_published_check CHECK ((published IS NULL) = (published_revision = 0)),
    CONSTRAINT website_content_updated_by_fkey FOREIGN KEY (updated_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT website_content_published_by_fkey FOREIGN KEY (published_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT
);

CREATE TABLE website_publications (
    tenant_id             uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    published_revision    bigint      NOT NULL CHECK (published_revision BETWEEN 1 AND 9007199254740991),
    source_draft_revision bigint      NOT NULL CHECK (source_draft_revision BETWEEN 1 AND 9007199254740991),
    published_by          uuid        NOT NULL,
    published_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, published_revision),
    CONSTRAINT website_publications_source_key UNIQUE (tenant_id, source_draft_revision),
    CONSTRAINT website_publications_published_by_fkey FOREIGN KEY (published_by, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT
);
