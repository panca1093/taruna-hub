CREATE INDEX tryout_questions_question_idx ON tryout_questions (tenant_id, question_id);
CREATE INDEX tryout_session_questions_question_idx ON tryout_session_questions (tenant_id, question_id);
