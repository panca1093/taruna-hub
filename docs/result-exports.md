# Result dashboards and CSV/XLSX exports (P21)

Staff filter completed results and request a spreadsheet of the tryout results. The job, artifact, retry and
download rules are the shared foundation in [exports.md](exports.md); this page covers what P21 adds.

## Dashboards

- `GET /api/v1/admin/results` (tryouts): filters `tryout_id`, `program_id`, `q`, `limit`, `offset`. Each row has the
  total score, the student's current rank and participant count (D04: average of all attempts, ties share a rank and
  the next skips), `irt_warning` (IRT-mode attempts only; a question version answered by fewer than 30 distinct
  students), per-subtest scores and `completed_at`.
- `GET /api/v1/admin/kecermatan-results`: filters `test_id`, `program_id`. Each row has benar/salah/kosong totals,
  the three composites, `nilai_akhir`, `completed_at` and `remaining_attempts` (the student's unused credits on the
  test now, from the enrollment row, not at completion time). No leaderboard.

Admin and Operator only; every query is scoped to the tenant.

## Tryout results export

`POST /api/v1/results/exports` with `{format: "csv"|"xlsx", tryout_id?, program_id?}`. Unknown fields and any other
format are `422`; a tryout or program that is not the tenant's is `404`; Student `403`, anonymous `401`. The answer
is `202` with the `Export` (kind `tryout_results_csv` or `tryout_results_xlsx`, params `{tryout_id?, program_id?}`).
The web page (`/admin/results`) shows both formats' latest exports through `ExportStatus`.

There is no kecermatan spreadsheet export: the requirement names only the tryout export (PLAN P21 gate). The
kecermatan page keeps its client-side CSV of the rows on screen.

### Cut-off

The renderer reads `tryout_results` completed at or before the export's `cut_off_at`, and ranks over that same set,
so a CSV and an XLSX requested at the same moment hold the same rows and a later completion is in neither. The IRT
flag is computed from completed sessions at render time, not the cut-off.

### Columns

| Column | Type in XLSX |
|---|---|
| Nama siswa, Tryout, Program (code) | text |
| Percobaan ke | number |
| Skor total, Rata-rata | number (0-100, 2 decimals) |
| Peringkat, Jumlah peserta | number |
| Peringatan IRT | text, `Ya` or `Tidak` |
| Selesai pada | text, ISO 8601 in `+07:00` (Asia/Jakarta), the same in CSV |

Rows are ordered by tryout title, rank, student and completion. Per-subtest scores and contact details are not
exported: the PRD (FR-ADM-RES-04) asks only for the results.

### Formats

- CSV: UTF-8 with a BOM, CRLF line ends, RFC 4180 quoting.
- XLSX: one sheet `Hasil`; numbers are numeric cells, text is written as an explicit string cell.
- Formula injection: every text cell that starts with `=`, `+`, `-`, `@`, tab or CR is prefixed with `'` in both
  formats. Numeric columns are never text.
- File names: `hasil-tryout-<cut-off date in Asia/Jakarta>.csv` / `.xlsx`.

### Limits and failure

The report is built in memory in the worker, never in the API request. More than 50,000 rows fails the export with
`error_code` `too_many_rows` (narrow the filter); because the foundation retries every failure, that failure is
retried to the last attempt before it shows as failed. Other failures use the foundation's retry rules.
