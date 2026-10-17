# Question bank: four types in immutable versions (P07a)

Staff write the four question types of PRD §5.5 — `pg` (one correct answer), `pg_multi` (two or more,
all-or-nothing), `isian` (short text, exact or case-insensitive) and `benar_salah` — against the
curriculum of P05. A Student never reaches the bank at all: every answer key stays on the staff API.

    questions          id, seq, tenant_id, current_version, created_by, created_at, updated_at
                         seq    GENERATED ALWAYS AS IDENTITY UNIQUE   -- the number the list shows
    question_versions  id, question_id, tenant_id, version, program_id, subject_id, chapter_id,
                       type, content, difficulty, explanation, option_weighted, case_sensitive,
                       options jsonb, correct jsonb, created_by, created_at
                         UNIQUE (question_id, version)

The wire form is `Question {id, number, version, version_id, program_id, subject_id, chapter_id,
type, content, difficulty, explanation, option_weighted, case_sensitive, options: [{id, label,
points?}], correct: [string], created_at, updated_at}`. `number` is the `seq`; `version_id` is what
later slices reference when they assign a question to a tryout.

## Why every save is a new row

A tryout must keep the content it was built with (ARCHITECTURE §4.3), so an edit never rewrites what
a test already used. The head row owns identity and ordering; every save appends a version and moves
`current_version` to it, so the row P08 pointed at stays byte-identical forever. Versions are gapless
because nothing ever deletes or renumbers them, and a question can only be deleted when no version of
it is referenced — the database answers `23503` and the delete rolls back as a `409`.

The composite keys do the tenancy: `(question_id, tenant_id)`, `(program_id, tenant_id)` and
`(subject_id, tenant_id)`, and `(chapter_id, tenant_id)` for the optional chapter (MATCH SIMPLE, so a
NULL chapter needs no lookup). A version can therefore never name another tenant's program, subject
or chapter, even if a repository filter were ever wrong. All of them are `ON DELETE RESTRICT`, which
is what makes a chapter still used by a question refuse to be deleted from the curriculum page.

What the database cannot express is checked in the service: the subject must belong to the program and
the chapter to the subject.

## PUT: base version, no-op, and option ids

`PUT /api/v1/questions/{id}` carries `base_version`, the version the editor opened. In one
transaction with the head row locked `FOR UPDATE`:

- a `base_version` that is no longer current answers `409` ("Soal sudah diubah orang lain, muat
  ulang.") and writes nothing, so two editors cannot silently overwrite each other;
- a payload whose normalized fields equal the current version answers `200` with the same version and
  writes no row — saving without changing anything is not a change;
- anything else appends version `current_version + 1`.

Option ids are the server's: an option that arrives with an id belonging to this question keeps it, an
option without an id becomes a new one, and an id of another question answers `422`. That is what lets
a stable option id survive a save (P08 and the scoring of P16 depend on it). A brand-new question has
no ids yet, so the client may send a placeholder (`a`, `new-4`) in `options[].id` and name it in
`correct`; the server replaces both with the real ids. `benar_salah` is the exception: its options are
always the server's `benar`/`salah`, and sending different ones answers `422`.

## Rules, and where they live

`content` is trimmed to 1–20000 characters and `explanation` to at most 20000; both are stored
verbatim as text, because markdown and LaTeX render in the browser and no HTML is ever interpreted.
A markdown image whose URL starts with `data:` or `javascript:` is refused ("Gambar belum didukung"):
uploads and assets are deferred (P06), so the editor hides the image button instead of offering
something the server cannot store.

`pg` takes 2–8 options with unique labels; normally exactly one correct id, and with `option_weighted`
none at all — every option is valid and may carry 0–100 points with at most 2 decimals. An option
without points stays absent in the stored JSON, so scoring (P16) can default it to 1. `pg_multi` takes
2–8 options and at least two correct ids and never carries weights. `isian` takes no options and 1–10
accepted answers, unique under the question's case rule; `case_sensitive` is legal only there.
`benar_salah` takes exactly one of `benar`/`salah`.

The rules live in `internal/service/question.go`; the repository only persists, including the
`INSERT` that appends a version. The lock is taken on the head row in a statement of its own before
the version row is read: after a competing save commits, a single joined statement holding
`FOR UPDATE` would re-check the join against its original snapshot and miss the version that just
appeared.

## Not in this slice

Import in any document format (P07b), master content and copy-to-own (FR-TEN-09..13), images, and any
per-question or per-difficulty weight (D04 is still open; weights are tenant-level and belong to
P08/P16). There is no capability matrix either (D02): the guard is by role until it is settled, and
both Admin and Operator write tenant content — Operator is content, per `decisions.md`. The page
therefore lost its "Bobot" column and the editor no longer derives a weight from difficulty; both
return with P08.

The question picker drawer of the tryout builder and the import modal still run on the prototype data,
because they belong to P08 and P07b.
