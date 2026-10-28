-- P17: ranking reads every student's totals of one tryout, and the IRT count reads every session that held one
-- question version. Neither has a leading column on an existing index.
CREATE INDEX tryout_score_totals_tryout_idx ON tryout_score_totals (tenant_id, tryout_id);
CREATE INDEX tryout_session_questions_version_idx ON tryout_session_questions (question_version_id);
