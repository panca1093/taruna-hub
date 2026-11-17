# Kecermatan attempts, credits and results (P18)

How a student buys attempts, takes a kecermatan test on a server timeline and gets authoritative counts plus
composite scores. Policy lives in ARCHITECTURE §4.7 and decisions D03, D05, D10; the wire contract is
`api/openapi.yaml` (tag `kecermatan-runtime`). Code: `service/kecermatan_runtime.go`, `service/kecermatan_score.go`,
`repository/kecermatan_runtime.go`, `handler/kecermatan_runtime.go`, migration `000019`. No fullscreen, violations
or ranking exist for kecermatan.

## Products and credits

A product of type `kecermatan` names one test of its own program and `included_attempts` (1-1000); it attaches no
tryouts. It is sold while it is active, inside its sale window and its test exists (`publicVisible` in the product
repository). The order snapshots `kecermatan_test_id` and `included_attempts`, and a foreign key stops a test with an
order from being deleted.

Wallet checkout and manual confirmation grant credits through the one issuance path, `IssueEnrollmentsTx`. The
student has one `enrollments` row per kecermatan test (`kecermatan_test_id` set, `tryout_id` null; a CHECK makes the
two exclusive). `attempts_purchased` is the credit total, `attempts_used` the completed attempts, and a CHECK keeps
`attempts_used <= attempts_purchased`. The grant row's unique source key makes a retried order add nothing twice; a
second purchase adds to the same row (no separate entitlement) and credits never expire. `GET /enrollments` stays
tryout-only; kecermatan holdings are `GET /my/kecermatan-tests`.

## Locks

Every path that touches credits or an attempt takes the **user row (`FOR NO KEY UPDATE`), then the enrollment row, then
the session row** (`withSession`, `Start`): the same user-then-enrollment order checkout and confirmation use, so a
start and a purchase of one student cannot wait on each other (tested by racing them). A payment adding credits and a completion consuming one therefore serialize on one row
(proved by a concurrent test), and `kecermatan_sessions_active_key` allows one attempt in progress per student and test.
Admission takes the test row `FOR SHARE`, or `FOR UPDATE` for the very first attempt, which stamps `first_attempt_at`
and locks the test against edits (P10).

## Start, snapshot and replay

`POST /my/kecermatan-tests/{id}/sessions` returns the attempt in progress (`200`: a retry, a second tab or a resume,
nothing consumed) or, with at least one credit, creates one (`201`) and starts kolom 1 at once. With no credit left it
answers `409 kecermatan_no_credit`, which the web turns into a redirect to the purchase. A credit is needed to start
but is **consumed only when the attempt completes**, in the same transaction as its result (D05).

The attempt seed is `HMAC-SHA256(key = stored test seed, message = 16 fresh random bytes)` truncated to 16 bytes
(`deriveKecermatanAttemptSeed`): bound to the test, different for every attempt, and computable by nobody from the
stored seed alone. The seed and the generated N/2 columns (reference sets, shown subsets, answer key) are frozen in
`kecermatan_sessions`; kolom k and k + N/2 share a column and answer separately. Regenerating from the stored seed and
the pinned pool equals the snapshot (tested). Nothing announces the repetition.

A student response carries only the running kolom: its reference set, the labels **shown** per soal and the answers
stored so far. The missing label, any seed and every other kolom stay on the server; the handler tests walk the raw
JSON for those keys during an attempt. The detailed review exists only for a completed attempt of its owner.

## Server timeline (D10)

`kecermatan_session_kolom` has one row per kolom (all created at admission). Kolom 1 starts at admission; kolom k
ends at `started_at + kolom_duration_seconds`; then a **pause of 5 seconds** (`KecermatanPauseSeconds`, the mockup's
"Lanjut" countdown) follows, and kolom k+1 starts at the earlier of the student's `POST .../continue` and the end of
the pause. When settled late, an automatic start is recorded at the pause's end, not at "now", so a closed tab or a
slow worker never shifts a timer. The last kolom's deadline completes the attempt (`completed_at` is that deadline).
There is no early submit, as in the mockup.

`settleLocked` is the one routine that applies passed instants. It runs under both locks from every mutating call,
from reads and from the worker. Ending a kolom writes its benar/salah/kosong; anything not answered or skipped is
kosong. `kecermatan_sessions.due_at` holds the next instant (a deadline or an automatic start) and is what the
worker scans: `cmd/worker` calls `KecermatanRuntimeService.FinalizeDue` every 2 seconds beside the tryout one, so an
abandoned attempt is finalized, scored and its credit consumed without the student returning.

## Answers

`PUT /my/kecermatan-sessions/{id}/kolom/{k}/soal/{n}` with `{answer: "A".."H" | null}` (the field must be present;
`null` is a skip). Answers are write-once: the same answer again is a harmless retry, another one is
`409 kecermatan_answer_locked`. Only the running kolom accepts them, before its server deadline
(`409 kecermatan_kolom_not_running`).

## Scoring (D03)

`ScoreKecermatan(version, counts)` is a pure function that accepts versions 1 and 2 and routes to `scoreV1` or
`scoreV2`; `kecermatan_results.scoring_version` records the version. N must be even and at least 2 for every version.
**Version 2 is the client's revised formula and the default** (`KecermatanScoringVersion = 2`): every new student
attempt is scored with it. **Version 1 is the owner's earlier provisional formula (2026-10-28), kept unchanged** and
reachable through `ScoreKecermatan(1, ...)` and `RecomputeScores`. This does not declare the D03 gate closed, and the
A and B bounds review (see below) is a separate piece of work. No UI text explains either formula.

### Version 2 (default)

Per kolom, benar/salah/kosong are the counts; dikerjakan = benar + salah and all soal = benar + salah + kosong,
summed over kolom for the totals.

    p          = dikerjakan / all soal * 100
    Kecepatan  = 80                          if p >= 74
               = 60 + (p - 30) * 20 / 44     if 30 <= p < 74
               = 2 * p                       if p < 30
    A          = 72 + (mean Benar of kolom N/2+1..N - mean Benar of kolom 1..N/2) * 1.6
    B          = 80 - 1.5 * SD(dikerjakan per kolom)         (sample SD, n-1)
    Ketahanan  = 0.6 A + 0.4 B
    Ketelitian = (1 - Salah / dikerjakan) * 81
    Nilai Akhir = 0.30 Kecepatan + 0.30 Ketahanan + 0.40 Ketelitian

The client document writes the lowest branch as `2 × nilai mentah × 100` for a nilai mentah of 30% or less, with nilai
mentah as a fraction; on p that is `2 * p`, and it meets the middle branch at 60 when p = 30. A zero denominator (all soal = 0 or dikerjakan = 0) makes Kecepatan or Ketelitian 0. Each component
is clamped to 0-100 and rounded to two decimals (half away from zero), the same helpers as v1; Nilai Akhir weights the
clamped, rounded components and is rounded again. The unclamped values are stored in `*_raw`. The intermediates p, A,
SD and B are not rounded. Only Ketahanan can leave 0-100 (Kecepatan is at most 80, Ketelitian at most 81).

An attempt with nothing answered follows the formulas with no special case: Kecepatan 0, Ketelitian 0, A 72, SD 0,
B 80, Ketahanan 75.2 and Nilai Akhir 22.56. Whether A or B can leave a sensible range (B negative, A very large) is
not reviewed here and is out of scope for this change.

Ketahanan is unstable on short tests, with no special case: at N = 2 it compares one kolom with one kolom and the SD
is taken over two values, so one kolom's difference moves the score a lot. The constants are also in raw soal counts,
so the same relative performance scores a different Ketahanan above 50 soal per kolom. Both are open client questions
(PRD §10 question 1).

### Version 1 (kept)

    Kecepatan  = ((Benar + Salah) / all soal - 0.05) * 100
    Ketahanan  = (avg Benar of kolom N/2+1..N / avg Benar of kolom 1..N/2 - 0.05) * 100
    Ketelitian = Benar / (Benar + Salah) * 100
    Nilai Akhir = 0.30 Kecepatan + 0.45 Ketelitian + 0.25 Ketahanan

A zero denominator makes that score 0. Each score is clamped to 0-100 and rounded to two decimals (half away from
zero); Nilai Akhir weights those clamped, rounded values and is rounded again, so it is reproducible from the stored
numbers. At N=10 Ketahanan equals the client's "last 5 vs first 5". The unclamped values are stored too
(`*_raw`). Per-kolom and total benar/salah/kosong are authoritative: a trigger refuses to change them.

`KecermatanRuntimeService.RecomputeScores(ctx, tenantID, version)` accepts version 1 or 2 (anything else fails with
the unknown-scoring-version error before any write) and rewrites only the composites and the version of every result
of the tenant from the stored counts. One call uses one version; it cannot mix them. An operator runs it with
`bootstrap rescore-kecermatan -tenant <slug>` (`-version` defaults to the current one); there is no HTTP trigger. Run
it once after deploying a new default version, so the history and the progress averages do not mix versions.

## Results and history

| Route | Who |
|---|---|
| `GET /my/kecermatan-tests`, `/{id}` | student: credits, in-progress attempt, history, a product to buy more from |
| `GET /my/kecermatan-sessions/{id}` | owner: phase `kolom` / `pause` / `completed` |
| `GET /my/kecermatan-sessions/{id}/result`, `/review` | owner, completed only (`409 kecermatan_result_not_ready` before) |
| `GET /admin/kecermatan-results`, `/{id}` | Admin and Operator: completed attempts, per-kolom counts |

The review returns the N/2 generated columns and, per kolom, the column it replayed and its stored answers, so an
attempt is reproduced exactly.

The overview's `progress` is the student's standing on the test: `nilai_gabungan` is the plain average of
`nilai_akhir` over the five most recent completed attempts (fewer when fewer exist), `nilai_gabungan_previous` the
same average before the latest attempt, and `kecepatan` / `ketahanan` / `ketelitian` the averages of those same
attempts, all rounded to two decimals. It is computed on read from the stored results and is `null` before the
first completion. The student result page shows this summary and each attempt's per-kolom counts; it no longer
shows the review, though the `/review` route still exists.

## Known limits

Image items: `GET /assets/{id}/url` also serves a student an `item_image` asset when their own attempt shows it: in an
attempt in progress only the running kolom's reference elements, in a completed one any element of the snapshot
(the review). Emoji and shapes are bundled static assets and need no authorization. The admin CSV
button exports the rows on screen; the bulk export is P21. The browser-to-API-to-database demo was not run.
