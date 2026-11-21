-- Question text becomes a rich document. The text columns stay as the legacy-text projection (search, rollback):
-- formulas as $latex$, **bold**, *italic*, lists as "- " / "1. " lines, tables as " | " rows. Shapes and uploaded
-- images have no text form, so a down migration loses them (and flattens tables); text and simple lists survive.
-- A NULL document is read as the legacy text, converted like the old renderer showed it, until the startup backfill.
ALTER TABLE question_versions
    ADD COLUMN content_doc     jsonb,
    ADD COLUMN explanation_doc jsonb;
