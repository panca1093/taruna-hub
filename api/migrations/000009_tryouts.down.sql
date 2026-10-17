DROP TABLE tryout_questions;
DROP TABLE tryout_subtests;
DROP TABLE tryouts;

ALTER TABLE question_versions DROP CONSTRAINT question_versions_id_question_key;
