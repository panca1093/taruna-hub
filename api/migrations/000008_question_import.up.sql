-- P07b: idempotent import without a staging table. The key is the file digest plus the row number,
-- so re-uploading the same file can never create a second copy of a row. The partial unique index is
-- what the commit's ON CONFLICT names; hand-written questions leave the column NULL.

ALTER TABLE questions ADD COLUMN import_key text;

CREATE UNIQUE INDEX questions_import_key_uniq ON questions (tenant_id, import_key)
    WHERE import_key IS NOT NULL;
