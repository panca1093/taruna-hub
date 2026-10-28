# Tryout runtime (P16)

How a student attempts a tryout and gets an immediate basic result. Policy lives in ARCHITECTURE §4.6 and
decisions D04, D10, D13; the wire contract is `api/openapi.yaml` (tag `runtime`). Code: `service/runtime.go`,
`service/runtime_score.go`, `repository/runtime.go`, `handler/runtime.go`, migration `000014`.

## Data

| Table | Holds |
|---|---|
| `tryout_sessions` | One attempt: number, status, frozen scoring weights, scoring version, permutation seed, summed staff extension |
| `tryout_session_parts` | One row per subtest: status (`pending`/`running`/`submitted`), start, absolute deadline, why it ended, raw earned/max points |
| `tryout_session_questions` | The snapshot: question version, display position, option order. Content comes from the immutable version |
| `tryout_session_answers` | Accepted answer, server `revision`, client `client_seq` |
| `tryout_session_extensions` | Append-only audit of staff extensions (actor, reason, seconds) |
| `tryout_results` | Exactly one row per session (primary key `session_id`) |
| `tryout_score_totals` | Per student and tryout `score_sum` and `result_count`, for P17's average |
| `tenant_settings` | D04 weights; no row means 1/2/3 |

`tryouts.grace_seconds` (default 0) is read live. A partial unique index allows one `in_progress` session per student
and tryout; a partial index on running part deadlines feeds the worker scan.

## Start and snapshot

`POST /my/tryouts/{id}/sessions` locks the student's enrollment row, so concurrent starts serialize: the first
creates the session, the rest find it. Without an in-progress session it takes the tryout row lock (exclusive only
for the very first attempt, which stamps `first_attempt_at`; shared afterwards), checks published and the window
(`active_from <= now < active_until`) and writes the snapshot: question versions the tryout froze, the weights
current at that moment, and a permutation from a random seed. Questions shuffle within their subtest when
`randomize_questions` is on; `pg` and `pg_multi` options shuffle when `randomize_options` is on. Option ids never
change, so points follow the option. Each attempt draws its own seed.

## Deadlines (D10)

A part's stored deadline is `min(start + duration, tryout.active_until)`, set once when the student presses start.
Three live values are added on top:

- the session's staff extension `extra_seconds`; the student's countdown shows `deadline + extra`;
- the tryout's `grace_seconds`; hidden from the countdown.

An answer is admitted while `now < deadline + extra + grace`, using server time. The extension applies to every
subtest of that session (a literal reading of "per-session"); it never reopens a part that already settled.

`settleLocked` is the one routine that applies passed deadlines. It runs under the session row lock from every
mutating call, from reads that find something due, and from the worker. It expires running parts past their
cutoff (`submit_reason = expired`, `submitted_at` = the deadline), then finalizes the session when every part is
submitted, or when `active_until` has passed and no part is still inside its grace. Parts never started then score
0 over their full maximum (`unstarted`). Between subtests nothing runs: the next part starts only on the student's
press, and a session waiting there is finalized at `active_until`.

The worker (`cmd/worker`, `finalizeDeadlines`) ticks every 2 seconds and calls `FinalizeDue`: PostgreSQL decides
what is due (running parts past cutoff, plus in-progress sessions with no running part past `active_until`), each
session settles in its own transaction, and one failure does not stop the batch. It is safe to run several workers
and to run it twice: a second pass finds nothing. A late worker never lets a late answer in, because the answer path
settles first.

## Answers

`PUT /my/tryout-sessions/{id}/answers/{questionId}` with `{seq, answer}`. `seq` is the browser's per-question
counter. Equal `seq` and equal answer is an idempotent retry; lower `seq`, or equal `seq` with another answer, is
`409 stale_answer` and never writes; higher wins. `revision` moves only when the stored answer changes. The
browser (`lib/tryout-session.ts`) keeps one request in flight per question and resends the latest, shows
saved versus pending per question, keeps unsent answers in localStorage only to resend them, and never shows a
local answer as saved before the server acknowledged it.

## Scoring (D04)

Per question the earned points are the difficulty weight (frozen into the session) when correct: `pg` and
`benar_salah` by the single correct id, `pg_multi` all-or-nothing on the exact set, `isian` after trimming,
collapsing whitespace and lower-casing. `option_weighted` `pg` questions score the chosen option's points (unset
counts as 1) and their maximum is the best option. An `isian` question that the author marked `case_sensitive`
keeps case. Parts store raw earned and maximum points; the result stores totals, the 0-100 score
`round(earned / max * 100, 2)` and `scoring_version`. IRT mode scores the same way. Students see only the
score, per-subtest scores and counts, never raw points, keys or explanations.

Finalization writes the parts, the result, the running totals and the completion in one transaction, so the
completion response always carries the score and the totals move once.

## Web

The countdown is server time at the last sync plus monotonic elapsed time, re-synced on focus and every 2 minutes.
The re-sync is an authenticated request, and so is every answer, so an exam keeps its login session alive (D13);
the server deadline still decides everything. Fullscreen and screenshot warnings stay local; persisting them is P19.
The scoring weights are edited on `/admin/settings`; the grace field is in the tryout builder and, once a tryout is
locked, saves through `PUT /tryouts/{id}/grace`.

## Not here

Ranking, explanations and `irt_warning` on the result page (P17), the monitoring UI for extensions and violations
(P19), Kecermatan (P18).
