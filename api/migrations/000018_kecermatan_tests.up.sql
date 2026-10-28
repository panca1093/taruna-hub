-- P10: tenant kecermatan tests. The row is the frozen generation configuration: structure, sources and a
-- stored seed, so the server can regenerate the same N/2 columns from it. first_attempt_at is stamped by
-- P18's admission transaction under the test row lock; from then on every edit and delete is refused.

CREATE TABLE kecermatan_tests (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             uuid        NOT NULL REFERENCES tenants (id),
    program_id            uuid        NOT NULL,
    title                 text        NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    description           text        NOT NULL DEFAULT '',
    item_type             text        NOT NULL
        CONSTRAINT kecermatan_tests_item_type_check
        CHECK (item_type IN ('char', 'number', 'symbol', 'emoji', 'image', 'shape', 'mix')),
    item_length           int         NOT NULL CHECK (item_length BETWEEN 1 AND 5),
    options_per_soal      int         NOT NULL CHECK (options_per_soal BETWEEN 3 AND 8),
    soal_per_kolom        int         NOT NULL CHECK (soal_per_kolom BETWEEN 50 AND 100),
    kolom_count           int         NOT NULL CHECK (kolom_count BETWEEN 10 AND 50 AND kolom_count % 2 = 0),
    kolom_duration_seconds int        NOT NULL CHECK (kolom_duration_seconds BETWEEN 1 AND 3600),
    -- Built-in sources (char, number, shape); only `mix` may name more than the type itself implies.
    builtin_sources       text[]      NOT NULL DEFAULT '{}'
        CHECK (builtin_sources <@ ARRAY['char', 'number', 'shape']),
    cover_from            text        NOT NULL CHECK (cover_from ~ '^#[0-9a-fA-F]{6}$'),
    cover_to              text        NOT NULL CHECK (cover_to ~ '^#[0-9a-fA-F]{6}$'),
    -- 16 random bytes in hex, drawn once at creation and never changed: the generator is a pure
    -- function of (seed, configuration, pinned package revisions).
    generation_seed       text        NOT NULL CHECK (generation_seed ~ '^[0-9a-f]{32}$'),
    revision              int         NOT NULL DEFAULT 1 CHECK (revision >= 1),
    -- NULL means nothing was ever attempted; P18 sets it inside its own transaction.
    first_attempt_at      timestamptz NULL,
    created_by            uuid        NOT NULL REFERENCES users (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT kecermatan_tests_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT kecermatan_tests_program_tenant_fkey FOREIGN KEY (program_id, tenant_id)
        REFERENCES programs (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX kecermatan_tests_tenant_created_idx ON kecermatan_tests (tenant_id, created_at DESC);

-- P09 left test_id without a foreign key; the pin now belongs to a real test of the same tenant.
ALTER TABLE kecermatan_test_packages
    ADD CONSTRAINT kecermatan_test_packages_test_fkey FOREIGN KEY (test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT;
