# Question bank: four types in immutable versions, with import (P07a, P07b)

Staff write the four question types of PRD §5.5 — `pg` (one correct answer), `pg_multi` (two or more,
all-or-nothing), `isian` (short text, exact or case-insensitive) and `benar_salah` — against the
curriculum of P05. A Student never reaches the bank at all: every answer key stays on the staff API.

    questions          id, seq, tenant_id, current_version, created_by, created_at, updated_at
                         seq    GENERATED ALWAYS AS IDENTITY UNIQUE   -- the number the list shows
    question_versions  id, question_id, tenant_id, version, program_id, subject_id, chapter_id,
                       type, content, difficulty, explanation, option_weighted, case_sensitive,
                       options jsonb, correct jsonb, created_by, created_at,
                       content_doc jsonb, explanation_doc jsonb      -- 000030; a `doc` key per option
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

`content`, `explanation` and every option `label` are rich documents (`RichDoc`: Tiptap/ProseMirror JSON,
see `api/internal/model/richdoc.go`), never HTML. `internal/service/richdoc.go` checks them against an
allow-list (paragraph, lists, table, `shape`, `image`; inline text with bold/italic/underline, `hardBreak`,
`math`), drops every attribute it does not know, and stores the normalised document. Limits: 2000 nodes,
depth 12, 100 KB; a table is at most 30 x 12 cells. `content` needs a visible element and at most 20000
characters of text, an option label the same with 2000, the explanation may be the empty document
`{"type":"doc","content":[]}`. A bad document answers `422` on its field with one of `doc_invalid`,
`doc_node`, `doc_mark`, `doc_attr`, `doc_too_large` or `doc_asset`.

Storage: `question_versions.content_doc` and `explanation_doc` (jsonb) and a `doc` key on every `options`
element hold the documents. The text columns `content`, `explanation` and the option `label` hold the **legacy
text projection** `model.DocLegacyText`, written on every save: formulas as `$latex$`, `**bold**`, `*italic*`,
bullet items as `- item` lines, ordered items as `1. item` lines, a legacy URL image as an `![alt](src)` line and
a table row as its cells joined by ` | `. Shapes and uploaded images have no text form and are omitted;
underline is dropped. The `q` search runs `ILIKE` on `content`, so words in a table cell, next to a formula or
in a list are found. Limits, emptiness and the duplicate-label check use the separate plain `model.DocText`
(formulas as bare LaTeX, no markers). The columns are also the rollback path, and what that path keeps is stated
plainly: after `000030` down, text, formulas, bold/italic and simple lists survive as the old renderer reads
them, while **tables are flattened to ` | ` lines, and shapes and uploaded images are lost** (a down migration
cannot bring them back, and a question that depended on them reads differently). Underline is lost too.

Reads, layering and legacy rows. The repository only selects and updates raw columns: a row whose document
column is NULL comes back as `model.RichDoc{Legacy: text}` with no `Type` (an option without `doc` the same, from its
label). The **service** converts, in one place (`resolveDoc` in `service/richdoc.go`), for every reader: question
list/get/create/update, the tryout assignment read (`TryoutService.Get/Create/Update/Publish/Unpublish`), the print
read, the student runtime (`RuntimeService.Subtest`) and the review (`ResultService.Review`). The conversion is
`model.DocFromLegacyText` followed by the same `normalizeDocWith` as client input (trimmed LaTeX, an alt cut to
200 characters, trailing empty paragraphs dropped, equal neighbouring text merged), so re-saving a legacy question
the editor did not change passes validation and creates no version. Text that cannot be made valid (more than 2000
nodes, say) is shown as converted and refused with the usual reason when saved.

What the conversion guarantees is the behaviour of the old client renderer (`web/components/latex.tsx` on `main`
before the rich-document change), and `model/richdoc_test.go` pins it case by case: a line is split on `$...$` first
and only the pieces between formulas get `**bold**` / `*italic*` (so `*Jika $x=2$ maka*` keeps its stars); a run of
two or more consecutive list lines is one list of the first line's kind even when bullet and numbered lines mix, and a
lone list line stays a literal paragraph; `
` and `` are line breaks and never reach a text node; a formula whose
LaTeX is blank (`$ $`) stays literal text. One known difference: an old `![alt](data:...)` line rendered as an image,
but the document model refuses `data:` URLs, so it reads as literal text.

At API startup `QuestionService.BackfillDocs` writes the converted documents into rows that still have none. It takes
`pg_try_advisory_lock` on a dedicated connection: a second replica that cannot get the lock logs it, skips the
backfill and serves (reads fall back), and the lock is always released on the same connection (a connection whose
unlock fails is closed). Rows are read in keyset order (`content_doc IS NULL AND id > last ORDER BY id`, batches of
500, one transaction per batch), so a row the converter refuses (for example `options` that is not an array) is logged
with its version id, left as text and never read twice in a run. The scan walks the primary key once per start; there
is no partial index because a start-up pass over a table of thousands of versions is cheap and the index would only
matter at millions. The backfill rewrites immutable version rows on purpose, adding only the document columns and the
option `doc` keys; the text columns are untouched. "Nothing changed, no new version" compares normalised documents.

Images: an `image` node names a `question_image` asset by `attrs.asset_id`. In the same transaction as the
version insert, every asset id of the content, explanation and options must be an asset of this tenant with
that purpose, and is finalized (`AssetService.FinalizePurposeTx`). A `staged` image may be attached only by the
staff member who uploaded it; one that is already `final` by any staff member of the tenant. An unknown,
foreign-tenant, wrong-purpose or someone-else's-staged id is the same `doc_asset` on the field that holds it, never
a `500`, and it never says which. `attrs.src` (an `http(s)` URL) is refused on create (`doc_attr`) and accepted on
update only when the identical `src` is already in the question's current version (content, explanation or an
option), so legacy URL images survive a re-save and no new external image can be introduced; spreadsheet import
and the legacy conversion are the only paths that produce one. A student may read a `final` `question_image` of the
tenant through `GET /assets/{id}/url` or `GET /assets/{id}/content`; see [assets.md](assets.md) for how open that is.

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
a formula cell's own text wins over its cached result. The template keeps its text columns: on preview and
commit each `content`, `explanation` and option cell goes through `DocFromLegacyText` (`$...$` math,
`**bold**`, `*italic*`, `- `/`1. ` lists, `![alt](http...)` image lines) and then the same validation as the
create endpoint, so HTML is data. A `data:`/`javascript:` image line in a cell is refused (`question_image`
reason), and the preview `content` stays the plain cell text.

The two upload routes carry no rate limit: the only throttle in the codebase is the auth service's
Redis counter inside login/forgot/resend, not a reusable middleware, and no import budget is defined.

## Not in this slice

Master content and copy-to-own (FR-TEN-09..13), and any per-question or per-difficulty weight
(D04 is still open; weights are tenant-level and belong to P08/P16). There is no capability matrix
either (D02): the guard is by role, and both Admin and Operator write tenant
content — Operator is content, per `decisions.md`. The page therefore lost its "Bobot" column and the
editor no longer derives a weight from difficulty; both return with P08.

The question picker drawer of the tryout builder still runs on the prototype data, because it belongs
to P08; the import modal is real from P07b.
