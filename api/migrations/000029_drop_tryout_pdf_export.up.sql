-- Stored objects of the deleted rows stay in the bucket under <tenant>/export/<id>; the down migration restores the CHECK, not the rows.
-- The tryout PDF is produced in the admin's browser now; the server-side export kind is gone.
DELETE FROM exports WHERE kind = 'tryout_pdf';
ALTER TABLE exports DROP CONSTRAINT exports_kind_check;
ALTER TABLE exports ADD CONSTRAINT exports_kind_check CHECK (kind IN ('tryout_results_csv', 'tryout_results_xlsx'));
