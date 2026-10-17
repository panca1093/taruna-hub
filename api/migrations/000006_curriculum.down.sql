-- Rows are tenant-scoped now, so the restored global UNIQUE (code) needs an empty table again.

DROP TABLE chapters;
DROP TABLE subjects;

DELETE FROM programs;

ALTER TABLE programs
    DROP CONSTRAINT programs_id_tenant_key,
    DROP CONSTRAINT programs_tenant_code_key,
    DROP COLUMN position,
    DROP COLUMN description,
    DROP COLUMN tenant_id,
    ADD CONSTRAINT programs_code_key UNIQUE (code);
