# Tryouts: drafts, publishing and frozen assignments (P08)

Staff assemble a tenant tryout from the question bank: ordered subtests with durations, an active
window, a scoring mode, explanation release and the two shuffle flags. A tryout is a **draft** or
**published**; only publishing requires a playable definition.

    tryouts              id, tenant_id, program_id, title, description, scoring_mode, status,
                         active_from, active_until, explanation_release_at,
                         randomize_questions, randomize_options, revision, first_attempt_at,
                         created_by, created_at, updated_at
                           UNIQUE (id, tenant_id)
    tryout_subtests      id, tryout_id, tenant_id, name, duration_seconds, position
                           UNIQUE (id, tenant_id)
                           UNIQUE (tryout_id, position) DEFERRABLE INITIALLY DEFERRED
    tryout_questions     subtest_id, tryout_id, tenant_id, position, question_id,
                         question_version_id
                           PRIMARY KEY (subtest_id, position) DEFERRABLE INITIALLY DEFERRED
                           UNIQUE (tryout_id, question_id)

`scoring_mode` is the whole of D04 for now: `weighted` or `irt`, never a weight, a formula or a
fitted IRT model.

## Why an assignment stores a version, not a question

`tryout_questions.question_version_id` points at a P07a `question_versions` row, which is immutable.
Assigning a question picks its **current** version at that moment; from then on the tryout reads that
row and nothing else, so editing the question — or editing it several times — cannot change what the
tryout contains. The read model exposes `question_id, version_id, version, number, type, difficulty,
content, position` and deliberately never `options`, `correct` or `explanation`: the answer key stays
on the staff question API and this endpoint is what P16 will build sessions from.

Two new constraints make the freeze structural rather than a habit. `question_versions` gains
`UNIQUE (id, question_id)`, and `tryout_questions` carries `FOREIGN KEY (question_version_id,
question_id)` onto it, so a version row can never be assigned to a different question. The tenant
pair, `FOREIGN KEY (question_version_id, tenant_id) -> question_versions (id, tenant_id)`, keeps
another tenant's version out even if a query filter were ever wrong.

The consequences the tests prove: deleting a question a tryout uses answers `409` through the
existing P07a mapping (the version FK now blocks it), and deleting a program a tryout uses answers
`409` through the P05 mapping. Only the assignment goes; the question and its versions stay.

## Saving: replace-all, one transaction, one revision

`POST /api/v1/tryouts` creates revision 1 as a draft. `PUT /api/v1/tryouts/{id}` takes the complete
definition and a `base_revision`, and in ONE transaction with the tryout row locked `FOR UPDATE`:

- a different `base_revision` answers `409` (nothing written), so two editors cannot silently
  overwrite each other;
- the old subtests and assignments are deleted and the new ones inserted in array order, so positions
  always start at 1 and are gap-free;
- `revision` and `updated_at` move forward once.

For every assignment: a `version_id` equal to the version this tryout already assigned to that
question is kept (which is how a re-save never re-freezes); an absent `version_id` takes the
question's current version; any other `version_id` — an arbitrary id, another question's version,
another tryout's version, another tenant's version — answers `422` on
`subtests[i].questions[j].version_id`.

## The lock after the first attempt

`tryouts.first_attempt_at` is stamped by **P16's admission transaction, under the same tryout row
lock**. P08 only reads it. Once it is set, every `PUT`, `DELETE`, `publish` and `unpublish` answers
`409` with "Tryout sudah dikerjakan siswa dan tidak bisa diubah." and writes nothing. Because the
admission stamp and the builder save both take `SELECT ... FOR UPDATE` on the same row, whichever
commits second sees the other's commit: a save that was already waiting cannot become a lost update.

Tests stamp the column with SQL, since sessions do not exist yet.

What P16's admission transaction must do, in this order:

1. `SELECT ... FOR UPDATE` the tryout row, then **re-check** `status = 'published'` and that the
   active window is open: an unpublish or a shortened window can commit just before the lock.
2. Read the subtests and assignments **after** taking the lock, in the same transaction. Subtest ids
   change on every save, so a definition read earlier may no longer exist.
3. Stamp with `UPDATE ... SET first_attempt_at = now() WHERE first_attempt_at IS NULL`, so only the
   first admission sets it.

`publish` and `unpublish` do not bump `revision`; only a save does.

## Rules, and where they live

`internal/service/tryout.go` owns every rule; the repository only persists. Title 1–200 characters,
description at most 2000, `scoring_mode` one of the two, program must exist in the tenant,
`active_until` required, `active_from` optional and strictly earlier. At most 30 subtests, 200
questions per subtest and 500 per tryout; subtest names 1–200 characters and durations 60–28800
seconds. A question of another tenant answers `422` on its path and never reveals that it exists
elsewhere; a question repeated in one tryout answers `422` on its second path.

Draft saves may be empty and may carry a past `active_until`. Publishing — and every save of an
already published tryout — requires at least one subtest, at least one question per subtest, and
`active_until` after now; the clock is injected (`WithTryoutClock`) so tests move time instead of
sleeping. Publishing twice is idempotent `200`, and unpublish returns the draft.

## Wire and guards

`Tryout` carries `locked` (derived from `first_attempt_at`), `subtest_count`, `question_count` and,
apart from the list, `subtests` with their assignments. List responses carry counts only so the page
stays small; `GET /api/v1/tryouts/{id}` returns the definition. The list filters by `program_id`,
`status` and a case-insensitive `q` over the title where `%` and `_` match themselves, newest first,
`limit` 50 by default and 100 at most.

Every route is Admin + Operator (D02 is still open, so the guard is by role), answers `Cache-Control:
no-store`, and an id of another tenant is `404`, never `403`. Unknown JSON fields are refused by
`bindStrict`.

## What this slice deliberately does not do

No student endpoint, no sessions or attempts (P16), no master content or copy (P07a/P11), no cover
image or gradient (P06), no PDF (P20), no scoring or weights (D04), no question-version "upgrade"
action, and subtest ids are not stable across saves — a replace legitimately recreates them.

The tryout list lost the prototype's **participants** column, because nothing counts students until
P16, and the question picker lost its **weight** label, because D04 is unresolved. The cover color
swatch is gone too: the API stores no cover field, so keeping an unsaved control would be a lie.
