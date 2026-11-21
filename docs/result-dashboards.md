# Result dashboards (P21)

Staff filter completed results. The server-side CSV/XLSX export planned for P21 was removed on 2026-11-21: results
are downloaded as PDFs of the per-student result page, built in the browser (see
[RECONCILIATION](../requirement/RECONCILIATION.md) section 2). There is no export endpoint, job or table (migration
000031); files stored by the old exports stay in the bucket under `<tenant>/export/<id>` and are not purged.

## Dashboards

- `GET /api/v1/admin/results` (tryouts): one row per student and tryout, only for students with at least one
  completed attempt. Filters `tryout_id`, `program_id`, `q` (case-insensitive name substring, `%` and `_` literal, at
  most 100 characters), `limit`, `offset`; `total` counts student rows. Each row has `attempts_completed`,
  `average_score`, `latest_score`, the student's current rank and participant count (D04: average of all attempts,
  ties share a rank and the next skips, the same numbers as the leaderboard and the student's own result page),
  `irt_warning` (true when any IRT-mode attempt of the student holds a question version answered by fewer than 30
  distinct students), the avatar and `last_completed_at`. Order: most recent activity, name, ids; with `tryout_id`, rank comes first (ranks of different tryouts are not comparable).
- `GET /api/v1/admin/results/students/{student_id}/tryouts/{tryout_id}`: the student's `ranking` on the tryout and
  every completed attempt, newest first, each with its score, `irt_warning` and per-subtest scores. A student or
  tryout outside the tenant, or a student without a completed attempt, is 404 ("Hasil siswa tidak ditemukan.").
- `GET /api/v1/admin/kecermatan-results`: filters `test_id`, `program_id`. Each row has benar/salah/kosong totals,
  the three composites, `nilai_akhir`, `completed_at` and `remaining_attempts` (the student's unused credits on the
  test now, from the enrollment row, not at completion time). No leaderboard.

Admin and Operator only; every query is scoped to the tenant.

There is no kecermatan spreadsheet export; the kecermatan page keeps its client-side CSV of the rows on screen.
