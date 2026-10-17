-- P07a: the tenant question bank. One head row per question carries identity and ordering; every
-- save writes a new immutable version row, so a tryout that referenced a version keeps reading the
-- content it was built with. Master content (tenant_id NULL), images and weights are deferred.

-- Needed by the chapter composite foreign key below: the pair is what makes a cross-tenant chapter
-- reference fail instead of silently matching an id from another tenant.
ALTER TABLE chapters ADD CONSTRAINT chapters_id_tenant_key UNIQUE (id, tenant_id);

CREATE TABLE questions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The human ID the list shows (FR-ADM-SOA-06); installation-wide, so numbers may skip.
    seq             bigint      GENERATED ALWAYS AS IDENTITY UNIQUE,
    tenant_id       uuid        NOT NULL REFERENCES tenants (id),
    current_version int         NOT NULL CHECK (current_version >= 1),
    created_by      uuid        NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT questions_id_tenant_key UNIQUE (id, tenant_id)
);

CREATE INDEX questions_tenant_seq_idx ON questions (tenant_id, seq DESC);

CREATE TABLE question_versions (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id     uuid        NOT NULL,
    tenant_id       uuid        NOT NULL,
    version         int         NOT NULL CHECK (version >= 1),
    program_id      uuid        NOT NULL,
    subject_id      uuid        NOT NULL,
    chapter_id      uuid        NULL,
    type            text        NOT NULL CHECK (type IN ('pg', 'pg_multi', 'isian', 'benar_salah')),
    content         text        NOT NULL,
    difficulty      text        NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard')),
    explanation     text        NOT NULL DEFAULT '',
    option_weighted boolean     NOT NULL DEFAULT false,
    case_sensitive  boolean     NOT NULL DEFAULT false,
    -- [{id,label,points?}]; ids are assigned by the service and stay stable across versions.
    options         jsonb       NOT NULL DEFAULT '[]',
    -- Option ids for the choice types, accepted answers for `isian`.
    correct         jsonb       NOT NULL,
    created_by      uuid        NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    -- P08 references a version with a composite key, the same way the curriculum does.
    CONSTRAINT question_versions_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT question_versions_question_version_key UNIQUE (question_id, version),
    CONSTRAINT question_versions_question_tenant_fkey FOREIGN KEY (question_id, tenant_id)
        REFERENCES questions (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT question_versions_program_tenant_fkey FOREIGN KEY (program_id, tenant_id)
        REFERENCES programs (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT question_versions_subject_tenant_fkey FOREIGN KEY (subject_id, tenant_id)
        REFERENCES subjects (id, tenant_id) ON DELETE RESTRICT,
    -- MATCH SIMPLE: a NULL chapter_id satisfies the key without a lookup.
    CONSTRAINT question_versions_chapter_tenant_fkey FOREIGN KEY (chapter_id, tenant_id)
        REFERENCES chapters (id, tenant_id) ON DELETE RESTRICT
);
