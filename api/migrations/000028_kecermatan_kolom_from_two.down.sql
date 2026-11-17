-- NOT VALID: tests and sessions created with fewer than 10 kolom stay, only new rows are held to the old range.
ALTER TABLE kecermatan_sessions
    DROP CONSTRAINT kecermatan_sessions_kolom_count_check,
    ADD CONSTRAINT kecermatan_sessions_kolom_count_check CHECK (kolom_count BETWEEN 10 AND 50 AND kolom_count % 2 = 0) NOT VALID;

ALTER TABLE kecermatan_tests
    DROP CONSTRAINT kecermatan_tests_kolom_count_check,
    ADD CONSTRAINT kecermatan_tests_kolom_count_check CHECK (kolom_count BETWEEN 10 AND 50 AND kolom_count % 2 = 0) NOT VALID;
