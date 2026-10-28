-- P16: tryout runtime. A session freezes the question versions, the scoring weights and one permutation at
-- attempt start (ARCHITECTURE 4.6); its parts carry the server-side deadlines; one result row per session.

-- D10: read live, never frozen, so Admin can change it while an exam runs.
ALTER TABLE tryouts
    ADD COLUMN grace_seconds int NOT NULL DEFAULT 0 CHECK (grace_seconds BETWEEN 0 AND 3600);

-- D04: one row per tenant; no row means the default weights 1/2/3.
CREATE TABLE tenant_settings (
    tenant_id     uuid         PRIMARY KEY REFERENCES tenants (id) ON DELETE RESTRICT,
    weight_easy   numeric(8,2) NOT NULL DEFAULT 1 CHECK (weight_easy > 0),
    weight_medium numeric(8,2) NOT NULL DEFAULT 2 CHECK (weight_medium > 0),
    weight_hard   numeric(8,2) NOT NULL DEFAULT 3 CHECK (weight_hard > 0),
    updated_by    uuid         NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_at    timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE tryout_sessions (
    id              uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid         NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id         uuid         NOT NULL,
    tryout_id       uuid         NOT NULL,
    attempt_number  int          NOT NULL CHECK (attempt_number >= 1),
    status          text         NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed')),
    scoring_mode    text         NOT NULL CHECK (scoring_mode IN ('weighted', 'irt')),
    scoring_version int          NOT NULL CHECK (scoring_version >= 1),
    weight_easy     numeric(8,2) NOT NULL,
    weight_medium   numeric(8,2) NOT NULL,
    weight_hard     numeric(8,2) NOT NULL,
    -- The permutation of questions and options derives from this, so a resume rebuilds nothing.
    seed            bigint       NOT NULL,
    started_at      timestamptz  NOT NULL,
    completed_at    timestamptz  NULL,
    CONSTRAINT tryout_sessions_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT tryout_sessions_attempt_key UNIQUE (tenant_id, user_id, tryout_id, attempt_number),
    CONSTRAINT tryout_sessions_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_sessions_tryout_tenant_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_sessions_completed_check CHECK ((status = 'completed') = (completed_at IS NOT NULL))
);

-- Concurrent starts race on this: the loser fails and re-reads the winner's session.
CREATE UNIQUE INDEX tryout_sessions_active_key ON tryout_sessions (tenant_id, user_id, tryout_id)
    WHERE status = 'in_progress';
CREATE INDEX tryout_sessions_user_idx ON tryout_sessions (tenant_id, user_id, started_at DESC, id DESC);
CREATE INDEX tryout_sessions_open_idx ON tryout_sessions (tryout_id) WHERE status = 'in_progress';

CREATE TABLE tryout_session_parts (
    session_id       uuid          NOT NULL,
    tenant_id        uuid          NOT NULL,
    subtest_id       uuid          NOT NULL,
    position         int           NOT NULL CHECK (position >= 1),
    name             text          NOT NULL,
    duration_seconds int           NOT NULL CHECK (duration_seconds > 0),
    status           text          NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'submitted')),
    started_at       timestamptz   NULL,
    -- min(started_at + duration, tryout active_until); the extension and the tryout's grace are added live.
    deadline_at      timestamptz   NULL,
    -- D10: staff extension of this subtest only, granted while it runs; every grant is also an audit row below.
    extra_seconds    int           NOT NULL DEFAULT 0 CHECK (extra_seconds BETWEEN 0 AND 28800),
    submitted_at     timestamptz   NULL,
    submit_reason    text          NULL CHECK (submit_reason IN ('student', 'expired', 'unstarted')),
    question_count   int           NOT NULL CHECK (question_count >= 0),
    earned_points    numeric(14,4) NULL,
    max_points       numeric(14,4) NULL,
    correct_count    int           NULL,
    CONSTRAINT tryout_session_parts_pkey PRIMARY KEY (session_id, subtest_id),
    CONSTRAINT tryout_session_parts_position_key UNIQUE (session_id, position),
    CONSTRAINT tryout_session_parts_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES tryout_sessions (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_session_parts_subtest_fkey FOREIGN KEY (subtest_id, tenant_id)
        REFERENCES tryout_subtests (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_session_parts_state_check CHECK (
        (status = 'pending' AND started_at IS NULL AND deadline_at IS NULL AND submitted_at IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND deadline_at IS NOT NULL AND submitted_at IS NULL)
        OR (status = 'submitted' AND submitted_at IS NOT NULL AND submit_reason IS NOT NULL
            AND earned_points IS NOT NULL AND max_points IS NOT NULL AND correct_count IS NOT NULL))
);

-- The deadline worker's due scan only ever looks at running parts.
CREATE INDEX tryout_session_parts_due_idx ON tryout_session_parts (deadline_at) WHERE status = 'running';

CREATE TABLE tryout_session_questions (
    session_id          uuid  NOT NULL,
    tenant_id           uuid  NOT NULL,
    subtest_id          uuid  NOT NULL,
    -- 1-based position in the order this student sees, already permuted when randomize_questions is on.
    position            int   NOT NULL CHECK (position >= 1),
    question_id         uuid  NOT NULL,
    question_version_id uuid  NOT NULL,
    -- Option ids in the order this student sees them; the ids never change, so points survive a shuffle.
    option_order        jsonb NOT NULL DEFAULT '[]',
    CONSTRAINT tryout_session_questions_pkey PRIMARY KEY (session_id, question_id),
    CONSTRAINT tryout_session_questions_position_key UNIQUE (session_id, subtest_id, position),
    CONSTRAINT tryout_session_questions_part_fkey FOREIGN KEY (session_id, subtest_id)
        REFERENCES tryout_session_parts (session_id, subtest_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_session_questions_version_fkey FOREIGN KEY (question_version_id, question_id)
        REFERENCES question_versions (id, question_id) ON DELETE RESTRICT
);

CREATE TABLE tryout_session_answers (
    session_id  uuid        NOT NULL,
    tenant_id   uuid        NOT NULL,
    question_id uuid        NOT NULL,
    answer      jsonb       NOT NULL DEFAULT '[]',
    -- Server-assigned and monotonic per answer; moves only when the stored answer changes.
    revision    int         NOT NULL CHECK (revision >= 1),
    -- The client's own per-question counter: the retry identity and the guard against reordered writes.
    client_seq  bigint      NOT NULL CHECK (client_seq >= 1),
    updated_at  timestamptz NOT NULL,
    CONSTRAINT tryout_session_answers_pkey PRIMARY KEY (session_id, question_id),
    CONSTRAINT tryout_session_answers_question_fkey FOREIGN KEY (session_id, question_id)
        REFERENCES tryout_session_questions (session_id, question_id) ON DELETE RESTRICT
);

-- The audit trail of D10's extension of the running subtest: append-only, one row per grant.
CREATE TABLE tryout_session_extensions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL,
    session_id uuid        NOT NULL,
    subtest_id uuid        NOT NULL,
    seconds    int         NOT NULL CHECK (seconds BETWEEN 1 AND 14400),
    reason     text        NOT NULL CHECK (btrim(reason) <> ''),
    granted_by uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tryout_session_extensions_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES tryout_sessions (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_session_extensions_part_fkey FOREIGN KEY (session_id, subtest_id)
        REFERENCES tryout_session_parts (session_id, subtest_id) ON DELETE RESTRICT
);

CREATE INDEX tryout_session_extensions_session_idx ON tryout_session_extensions (session_id, created_at);

-- Exactly one row per session: the primary key is what makes finalizing twice harmless.
CREATE TABLE tryout_results (
    session_id      uuid          PRIMARY KEY,
    tenant_id       uuid          NOT NULL,
    user_id         uuid          NOT NULL,
    tryout_id       uuid          NOT NULL,
    attempt_number  int           NOT NULL,
    earned_points   numeric(14,4) NOT NULL,
    max_points      numeric(14,4) NOT NULL,
    -- (earned / max) * 100 rounded to 2 decimals (D04); 0 when the tryout has no scorable points.
    score           numeric(5,2)  NOT NULL CHECK (score BETWEEN 0 AND 100),
    scoring_mode    text          NOT NULL,
    scoring_version int           NOT NULL,
    completed_at    timestamptz   NOT NULL,
    CONSTRAINT tryout_results_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES tryout_sessions (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX tryout_results_user_idx ON tryout_results (tenant_id, user_id, tryout_id, completed_at DESC);

-- P17 ranks on score_sum / result_count; it is maintained in the transaction that inserts the result.
CREATE TABLE tryout_score_totals (
    tenant_id    uuid          NOT NULL,
    user_id      uuid          NOT NULL,
    tryout_id    uuid          NOT NULL,
    score_sum    numeric(14,2) NOT NULL,
    result_count int           NOT NULL CHECK (result_count >= 1),
    updated_at   timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT tryout_score_totals_pkey PRIMARY KEY (tenant_id, user_id, tryout_id),
    CONSTRAINT tryout_score_totals_user_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_score_totals_tryout_fkey FOREIGN KEY (tryout_id, tenant_id)
        REFERENCES tryouts (id, tenant_id) ON DELETE RESTRICT
);
