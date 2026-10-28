-- P19: tryout violation evidence. Append-only rows the student's browser reports (fullscreen exit, screenshot
-- shortcut); the server stamps the time. Evidence only: nothing here submits, penalizes or scores.
CREATE TABLE tryout_session_violations (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Breaks ties between events the clock cannot tell apart, so a timeline order is stable.
    seq         bigint      GENERATED ALWAYS AS IDENTITY,
    tenant_id   uuid        NOT NULL,
    session_id  uuid        NOT NULL,
    subtest_id  uuid        NOT NULL,
    type        text        NOT NULL CHECK (type IN ('exit_fullscreen', 'screenshot_attempt')),
    -- The browser's retry identity: a resend of the same event never adds a second row.
    event_id    uuid        NOT NULL,
    occurred_at timestamptz NOT NULL,
    CONSTRAINT tryout_session_violations_event_key UNIQUE (session_id, event_id),
    CONSTRAINT tryout_session_violations_session_fkey FOREIGN KEY (session_id, tenant_id)
        REFERENCES tryout_sessions (id, tenant_id) ON DELETE RESTRICT,
    CONSTRAINT tryout_session_violations_part_fkey FOREIGN KEY (session_id, subtest_id)
        REFERENCES tryout_session_parts (session_id, subtest_id) ON DELETE RESTRICT
);

-- The per-session timeline and the per-row count in the polling query.
CREATE INDEX tryout_session_violations_timeline_idx ON tryout_session_violations (tenant_id, session_id, occurred_at, seq);

-- The monitoring poll: in-progress sessions of one tenant, newest first.
CREATE INDEX tryout_sessions_monitor_idx ON tryout_sessions (tenant_id, started_at DESC, id) WHERE status = 'in_progress';
