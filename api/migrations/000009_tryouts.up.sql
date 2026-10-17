-- P08: tenant tryouts with frozen question versions. An assignment points at an immutable
-- question_versions row, so editing a question never changes a tryout that already used it.
-- first_attempt_at is stamped by P16's admission transaction under the tryout row lock; from then
-- on every edit, delete, publish and unpublish is refused.

-- The pair lets tryout_questions prove that a version row belongs to the question it names.
ALTER TABLE question_versions ADD CONSTRAINT question_versions_id_question_key UNIQUE (id, question_id);

CREATE TABLE tryouts (
    id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              uuid        NOT NULL REFERENCES tenants (id),
    program_id             uuid        NOT NULL,
    title                  text        NOT NULL,
    description            text        NOT NULL DEFAULT '',
    -- D04 is open: only the choice is stored here, never a weight or formula.
    scoring_mode           text        NOT NULL CHECK (scoring_mode IN ('weighted', 'irt')),
    status                 text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    active_from            timestamptz NULL,
    active_until           timestamptz NOT NULL,
    explanation_release_at timestamptz NULL,
    randomize_questions    boolean     NOT NULL DEFAULT false,
    randomize_options      boolean     NOT NULL DEFAULT false,
    revision               int         NOT NULL DEFAULT 1 CHECK (revision >= 1),
    -- NULL means nothing was ever attempted; P16 sets it inside its own transaction.
    first_attempt_at       timestamptz NULL,
    created_by             uuid        NOT NULL REFERENCES users (id),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tryouts_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT tryouts_program_tenant_fkey FOREIGN KEY (program_id, tenant_id)
        REFERENCES programs (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryouts_window_check CHECK (active_from IS NULL OR active_from < active_until)
);

CREATE INDEX tryouts_tenant_created_idx ON tryouts (tenant_id, created_at DESC);

CREATE TABLE tryout_subtests (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tryout_id        uuid        NOT NULL,
    tenant_id        uuid        NOT NULL,
    name             text        NOT NULL,
    duration_seconds int         NOT NULL CHECK (duration_seconds BETWEEN 60 AND 28800),
    position         int         NOT NULL CHECK (position >= 1),
    CONSTRAINT tryout_subtests_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT tryout_subtests_id_tryout_key UNIQUE (id, tryout_id),
    -- Deferrable so one save can reorder or swap positions inside a single transaction.
    CONSTRAINT tryout_subtests_tryout_position_key UNIQUE (tryout_id, position) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT tryout_subtests_tryout_tenant_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT
);

CREATE TABLE tryout_questions (
    subtest_id         uuid NOT NULL,
    tryout_id          uuid NOT NULL,
    tenant_id          uuid NOT NULL,
    position           int  NOT NULL CHECK (position >= 1),
    question_id        uuid NOT NULL,
    question_version_id uuid NOT NULL,
    CONSTRAINT tryout_questions_pkey PRIMARY KEY (subtest_id, position) DEFERRABLE INITIALLY DEFERRED,
    -- A question appears at most once per tryout, across every subtest.
    CONSTRAINT tryout_questions_tryout_question_key UNIQUE (tryout_id, question_id),
    CONSTRAINT tryout_questions_subtest_tenant_fkey FOREIGN KEY (subtest_id, tenant_id)
        REFERENCES tryout_subtests (id, tenant_id) ON DELETE RESTRICT,
    -- tryout_id is denormalized for the per-tryout uniqueness above, so it must match the subtest's own.
    CONSTRAINT tryout_questions_subtest_tryout_fkey FOREIGN KEY (subtest_id, tryout_id)
        REFERENCES tryout_subtests (id, tryout_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_questions_version_tenant_fkey FOREIGN KEY (question_version_id, tenant_id)
        REFERENCES question_versions (id, tenant_id) ON DELETE RESTRICT,
    -- The frozen version must be a version of the question named beside it.
    CONSTRAINT tryout_questions_version_question_fkey FOREIGN KEY (question_version_id, question_id)
        REFERENCES question_versions (id, question_id) ON DELETE RESTRICT
);

CREATE INDEX tryout_questions_tryout_idx ON tryout_questions (tryout_id);
