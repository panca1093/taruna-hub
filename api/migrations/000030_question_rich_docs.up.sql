-- Question text becomes a rich document. The text columns stay as the plain-text projection (search, rollback);
-- a NULL document is read as the legacy text until the startup backfill fills it.
ALTER TABLE question_versions
    ADD COLUMN content_doc     jsonb,
    ADD COLUMN explanation_doc jsonb;
