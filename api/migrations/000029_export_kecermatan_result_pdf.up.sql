ALTER TABLE exports DROP CONSTRAINT exports_kind_check;
ALTER TABLE exports ADD CONSTRAINT exports_kind_check
    CHECK (kind IN ('tryout_pdf', 'tryout_results_csv', 'tryout_results_xlsx', 'kecermatan_result_pdf'));
