# Question bank: four types in immutable versions, with import (P07a, P07b)

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

## Import (P07b)

`POST /api/v1/questions/import/preview` and `/commit` take the file itself: the client never sends
parsed rows, and `commit` re-parses and re-validates the same bytes before writing. Preview writes
nothing — a test counts the bank before and after. Both routes are Admin/Operator only, answer
`Cache-Control: no-store`, and take their tenant from the host like the rest of the bank.

`GET /api/v1/questions/import/template?format=xlsx|csv|docx` returns the header plus two example rows
(one `pg`, one `isian`) named after this tenant's first program, subject and chapter, so a fresh
template can be filled in and imported unchanged. Excel, CSV and Word (.docx, first table) share the
same columns: `type`, `program`, `subject`, `chapter` (optional), `difficulty`, `content`,
`explanation` (optional), `option_a`..`option_h`, `correct` and `case_sensitive` (isian only). Header
names are case-insensitive, column order is free, extra columns are ignored, and at most 500 data rows
are accepted. `difficulty` also takes `mudah`/`sedang`/`sulit`; `correct` holds option letters (`B`,
`A,C`), `|`-separated accepted answers for `isian`, and `benar`/`salah` for `benar_salah`.

Each row becomes the same `QuestionInput` the editor posts and goes through `scalarOf` and
`validatedOptions` — the P07a rules, not a copy of them. A failing row reports the shared reason
against the spreadsheet column it came from (`program`, `option_b`, `correct`), which is what the
preview table shows. Program, subject and chapter names are resolved once per file inside the caller's
tenant tree, never per row.

D08 (default proposal, awaiting Panca): the preview lists every row with its real result and the user
confirms the accepted set — all valid rows by default. `commit` writes exactly that set in ONE
transaction; a selected row that is invalid, a row number that does not exist or a repeated one
answers `422` on `rows` and writes nothing. Unselected rows are ignored even when they are invalid.

Idempotency needs no staging table. Migration `000008` adds `questions.import_key`, set to
`<first 32 hex of sha256(file)>:<row number>`, with a partial unique index on
`(tenant_id, import_key)`; the insert is `ON CONFLICT ... DO NOTHING`. A retry, a subset followed by
the rest, and two commits running at once therefore yield one question per (file, row), and the later
attempt reports `already_imported`. The column is not part of the question wire type.

The file is data, not a program. At most 5 MB (`http.MaxBytesReader`) and 500 data rows; the format is
proved from the content rather than the extension (`.xlsx`/`.docx` must start with `PK\x03\x04` and
carry `xl/workbook.xml` or `word/document.xml`; `.csv` must be UTF-8 without NUL bytes, BOM stripped);
decompression is bounded by a 50 MB unzip and 20 MB worksheet budget in excelize, and the DOCX reader
takes only `word/document.xml` through a 20 MB `LimitedReader` with `encoding/xml`, which never
resolves external entities. A formula stays text: `=1+1` is stored as `=1+1` and never calculated, and
a formula cell's own text wins over its cached result. Content is stored verbatim, so HTML is data and
`data:`/`javascript:` image URLs are still refused by the shared rule.

The two upload routes carry no rate limit: the only throttle in the codebase is the auth service's
Redis counter inside login/forgot/resend, not a reusable middleware, and no import budget is defined.

## Not in this slice

Master content and copy-to-own (FR-TEN-09..13), images, and any per-question or per-difficulty weight
(D04 is still open; weights are tenant-level and belong to P08/P16). There is no capability matrix
either (D02): the guard is by role, and both Admin and Operator write tenant
content — Operator is content, per `decisions.md`. The page therefore lost its "Bobot" column and the
editor no longer derives a weight from difficulty; both return with P08.

The question picker drawer of the tryout builder still runs on the prototype data, because it belongs
to P08; the import modal is real from P07b.
