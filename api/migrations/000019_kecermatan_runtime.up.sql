-- P18: kecermatan products, credits on the enrollment row, per-attempt snapshots, server timers and results.

-- A kecermatan product sells attempt credits for one test; it has no product_tryouts rows.
ALTER TABLE products
    DROP CONSTRAINT products_type_check,
    ADD CONSTRAINT products_type_check CHECK (type IN ('individual', 'bundle', 'kecermatan')),
    ADD COLUMN kecermatan_test_id uuid NULL,
    ADD COLUMN included_attempts  int  NULL CHECK (included_attempts IS NULL OR included_attempts BETWEEN 1 AND 1000),
    ADD CONSTRAINT products_kecermatan_check CHECK (
        (type = 'kecermatan' AND kecermatan_test_id IS NOT NULL AND included_attempts IS NOT NULL)
        OR (type <> 'kecermatan' AND kecermatan_test_id IS NULL AND included_attempts IS NULL)),
    ADD CONSTRAINT products_kecermatan_test_fkey FOREIGN KEY (kecermatan_test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT;

CREATE INDEX products_kecermatan_test_idx ON products (tenant_id, kecermatan_test_id) WHERE kecermatan_test_id IS NOT NULL;

-- The order snapshot keeps what was bought; the foreign key stops a test with an order from being deleted.
ALTER TABLE orders
    DROP CONSTRAINT orders_product_type_check,
    ADD CONSTRAINT orders_product_type_check CHECK (product_type IN ('individual', 'bundle', 'kecermatan')),
    ADD COLUMN kecermatan_test_id uuid NULL,
    ADD COLUMN included_attempts  int  NULL CHECK (included_attempts IS NULL OR included_attempts >= 1),
    ADD CONSTRAINT orders_kecermatan_check CHECK (
        (product_type = 'kecermatan' AND kecermatan_test_id IS NOT NULL AND included_attempts IS NOT NULL)
        OR (product_type <> 'kecermatan' AND kecermatan_test_id IS NULL AND included_attempts IS NULL)),
    ADD CONSTRAINT orders_kecermatan_test_fkey FOREIGN KEY (kecermatan_test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT;

-- One enrollment row per user and test, tryout or kecermatan. On a kecermatan row attempts_purchased is the
-- credit total; every purchase adds to it, and a completed attempt adds one to attempts_used (D05).
ALTER TABLE enrollments
    ALTER COLUMN tryout_id DROP NOT NULL,
    ADD COLUMN kecermatan_test_id uuid NULL,
    ADD CONSTRAINT enrollments_kind_check CHECK ((tryout_id IS NULL) <> (kecermatan_test_id IS NULL)),
    ADD CONSTRAINT enrollments_credit_check CHECK (
        (kecermatan_test_id IS NULL AND attempts_purchased IS NULL)
        OR (kecermatan_test_id IS NOT NULL AND attempts_purchased IS NOT NULL AND attempts_used <= attempts_purchased)),
    ADD CONSTRAINT enrollments_kecermatan_holder_key UNIQUE (tenant_id, user_id, kecermatan_test_id),
    ADD CONSTRAINT enrollments_kecermatan_test_fkey FOREIGN KEY (kecermatan_test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT;

CREATE INDEX enrollments_kecermatan_idx ON enrollments (kecermatan_test_id, tenant_id) WHERE kecermatan_test_id IS NOT NULL;

-- One attempt. The per-attempt seed and the generated N/2 columns are frozen at admission, so the test can no
-- longer change what this student sees; the stored test seed never reaches a response.
CREATE TABLE kecermatan_sessions (
    id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              uuid        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id                uuid        NOT NULL,
    test_id                uuid        NOT NULL,
    attempt_number         int         NOT NULL CHECK (attempt_number >= 1),
    status                 text        NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed')),
    seed                   text        NOT NULL CHECK (seed ~ '^[0-9a-f]{32}$'),
    test_revision          int         NOT NULL,
    kolom_count            int         NOT NULL CHECK (kolom_count BETWEEN 10 AND 50 AND kolom_count % 2 = 0),
    soal_per_kolom         int         NOT NULL CHECK (soal_per_kolom BETWEEN 50 AND 100),
    options_per_soal       int         NOT NULL CHECK (options_per_soal BETWEEN 3 AND 8),
    kolom_duration_seconds int         NOT NULL CHECK (kolom_duration_seconds BETWEEN 1 AND 3600),
    pause_seconds          int         NOT NULL CHECK (pause_seconds BETWEEN 0 AND 60),
    -- The generated N/2 columns: reference sets, shown subsets and the answer key. Read by the server only,
    -- and never sent to a student whole.
    columns                jsonb       NOT NULL,
    -- The next instant something happens on the timeline (a running kolom's deadline or an automatic start);
    -- it is what the deadline worker scans.
    due_at                 timestamptz NULL,
    started_at             timestamptz NOT NULL,
    completed_at           timestamptz NULL,
    CONSTRAINT kecermatan_sessions_id_tenant_key UNIQUE (id, tenant_id),
    CONSTRAINT kecermatan_sessions_attempt_key UNIQUE (tenant_id, user_id, test_id, attempt_number),
    CONSTRAINT kecermatan_sessions_user_tenant_fkey FOREIGN KEY (user_id, tenant_id)
        REFERENCES users (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT kecermatan_sessions_test_tenant_fkey FOREIGN KEY (test_id, tenant_id)
        REFERENCES kecermatan_tests (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT kecermatan_sessions_state_check CHECK (
        (status = 'in_progress' AND completed_at IS NULL AND due_at IS NOT NULL)
        OR (status = 'completed' AND completed_at IS NOT NULL AND due_at IS NULL))
);

-- Concurrent or retried starts race on this: the loser re-reads the winner's attempt.
CREATE UNIQUE INDEX kecermatan_sessions_active_key ON kecermatan_sessions (tenant_id, user_id, test_id)
    WHERE status = 'in_progress';
CREATE INDEX kecermatan_sessions_user_idx ON kecermatan_sessions (tenant_id, user_id, started_at DESC, id DESC);
CREATE INDEX kecermatan_sessions_due_idx ON kecermatan_sessions (due_at) WHERE status = 'in_progress';

-- One row per kolom, all created at admission. Counts are written once when the kolom ends.
CREATE TABLE kecermatan_session_kolom (
    session_id   uuid        NOT NULL,
    tenant_id    uuid        NOT NULL,
    kolom_number int         NOT NULL CHECK (kolom_number >= 1),
    status       text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ended')),
    started_at   timestamptz NULL,
    deadline_at  timestamptz NULL,
    benar        int         NULL CHECK (benar >= 0),
    salah        int         NULL CHECK (salah >= 0),
    kosong       int         NULL CHECK (kosong >= 0),
    CONSTRAINT kecermatan_session_kolom_pkey PRIMARY KEY (session_id, kolom_number),
    CONSTRAINT kecermatan_session_kolom_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES kecermatan_sessions (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT kecermatan_session_kolom_state_check CHECK (
        (status = 'pending' AND started_at IS NULL AND deadline_at IS NULL AND benar IS NULL)
        OR (status = 'running' AND started_at IS NOT NULL AND deadline_at IS NOT NULL AND benar IS NULL)
        OR (status = 'ended' AND started_at IS NOT NULL AND deadline_at IS NOT NULL
            AND benar IS NOT NULL AND salah IS NOT NULL AND kosong IS NOT NULL))
);

-- One row per answered or skipped soal; a missing row is kosong. answer NULL is an explicit skip.
CREATE TABLE kecermatan_session_answers (
    session_id   uuid        NOT NULL,
    tenant_id    uuid        NOT NULL,
    kolom_number int         NOT NULL,
    soal_number  int         NOT NULL CHECK (soal_number >= 1),
    answer       text        NULL CHECK (answer IS NULL OR answer ~ '^[A-H]$'),
    answered_at  timestamptz NOT NULL,
    CONSTRAINT kecermatan_session_answers_pkey PRIMARY KEY (session_id, kolom_number, soal_number),
    CONSTRAINT kecermatan_session_answers_kolom_fkey FOREIGN KEY (session_id, kolom_number)
        REFERENCES kecermatan_session_kolom (session_id, kolom_number) ON DELETE RESTRICT
);

-- Exactly one row per attempt. The counts are authoritative and never change; the composite scores are a
-- function of the counts under scoring_version, so a revised formula can recompute them (D03).
CREATE TABLE kecermatan_results (
    session_id        uuid          PRIMARY KEY,
    tenant_id         uuid          NOT NULL,
    user_id           uuid          NOT NULL,
    test_id           uuid          NOT NULL,
    attempt_number    int           NOT NULL,
    benar_total       int           NOT NULL CHECK (benar_total >= 0),
    salah_total       int           NOT NULL CHECK (salah_total >= 0),
    kosong_total      int           NOT NULL CHECK (kosong_total >= 0),
    kecepatan         numeric(5,2)  NOT NULL CHECK (kecepatan BETWEEN 0 AND 100),
    ketahanan         numeric(5,2)  NOT NULL CHECK (ketahanan BETWEEN 0 AND 100),
    ketelitian        numeric(5,2)  NOT NULL CHECK (ketelitian BETWEEN 0 AND 100),
    nilai_akhir       numeric(5,2)  NOT NULL CHECK (nilai_akhir BETWEEN 0 AND 100),
    kecepatan_raw     numeric(14,4) NOT NULL,
    ketahanan_raw     numeric(14,4) NOT NULL,
    ketelitian_raw    numeric(14,4) NOT NULL,
    scoring_version   int           NOT NULL CHECK (scoring_version >= 1),
    completed_at      timestamptz   NOT NULL,
    CONSTRAINT kecermatan_results_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES kecermatan_sessions (id, tenant_id) ON DELETE RESTRICT
);

CREATE INDEX kecermatan_results_test_idx ON kecermatan_results (tenant_id, test_id, completed_at DESC);
CREATE INDEX kecermatan_results_user_idx ON kecermatan_results (tenant_id, user_id, test_id, completed_at DESC);

CREATE FUNCTION kecermatan_results_counts_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.benar_total <> OLD.benar_total OR NEW.salah_total <> OLD.salah_total
       OR NEW.kosong_total <> OLD.kosong_total OR NEW.session_id <> OLD.session_id THEN
        RAISE EXCEPTION 'kecermatan result counts are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER kecermatan_results_counts_immutable BEFORE UPDATE ON kecermatan_results
    FOR EACH ROW EXECUTE FUNCTION kecermatan_results_counts_immutable();
