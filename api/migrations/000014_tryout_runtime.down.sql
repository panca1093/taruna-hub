DROP TABLE IF EXISTS tryout_score_totals;
DROP TABLE IF EXISTS tryout_results;
DROP TABLE IF EXISTS tryout_session_extensions;
DROP TABLE IF EXISTS tryout_session_answers;
DROP TABLE IF EXISTS tryout_session_questions;
DROP TABLE IF EXISTS tryout_session_parts;
DROP TABLE IF EXISTS tryout_sessions;
DROP TABLE IF EXISTS tenant_settings;
ALTER TABLE tryouts DROP COLUMN IF EXISTS grace_seconds;
