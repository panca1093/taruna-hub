# Results, ranking and controlled review (P17)

How completed attempts become ranks, reviews and staff results. Policy: ARCHITECTURE D04; contract: `api/openapi.yaml`
(`GET /my/tryout-sessions/{id}/result`, `.../review`, `GET /admin/results`, `GET /admin/tryouts/{id}/leaderboard`).
Code: `service/result.go`, `repository/result.go`, `handler/result.go`, migration `000015` (two indexes only).

## Ranking

A student's ranking value on a tryout is the mean of their completed-attempt scores (`score_sum / result_count` from
`tryout_score_totals`, kept by P16), rounded to 2 decimals. Students are ranked among the tenant's participants of that
tryout with `RANK()`: equal averages share a rank and the next rank skips (1, 2, 2, 4). Ties are judged on the rounded,
displayed average. The rank is computed from PostgreSQL on every read and never stored, so every attempt of one student
shows the same current rank and nothing can be stale.

Redis is not used. A sorted-set cache would need a version to know when it is stale; `sum(result_count)` of the tryout is
one (it rises by exactly one per completed attempt), with `ZCOUNT` for the rank and a rebuild from `Averages()` on
mismatch. It was left out because the PostgreSQL query is one indexed scan and the cache adds no correctness.

## Review and explanations

`GET .../review` serves only the owner's completed attempt, and only when `tryouts.explanation_release_at` is null or past.
Before that it answers `409 explanations_not_released` after reading nothing but the release time, so no key,
explanation or content leaves. `.../result` reports `explanations_released` and the release time. Question content has no
asset links, and a student still cannot download staff-uploaded question images.

## IRT prevalidation

For results of IRT-mode sessions, `irt_warning` is true when any question version in the attempt has fewer than 30
distinct students (`IRTMinStudents`) with a non-blank answer in a completed session. Retakes count once, blank answers and
running sessions do not count, and a new question version starts at 0. The count is per version across tryouts. It appears
on `GET /admin/results` only; weighted scoring is untouched.

## Staff

`GET /admin/results` (Admin, Operator) lists completed attempts with score, the student's current rank, subtests and
`irt_warning`, filterable by tryout and student name, paginated like orders. The leaderboard endpoint is staff-only too.

## Not here

CSV/XLSX export and dashboards (P21), the violation timeline in the result detail (P19), a student-facing leaderboard.
