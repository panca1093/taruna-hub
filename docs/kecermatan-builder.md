# Kecermatan builder and generator (P10)

A kecermatan test is a frozen generation configuration: structure, sources and a stored seed. The server
regenerates its content from that configuration; nothing generated is stored in this slice (P18 snapshots it
per attempt). Admin and Operator may manage tests (D02). Tenant-owned only (D17).

    kecermatan_tests          id, tenant_id, program_id, title, description, item_type, item_length,
                              options_per_soal, soal_per_kolom, kolom_count, kolom_duration_seconds,
                              builtin_sources[], cover_from/to, generation_seed, revision,
                              first_attempt_at, created_by, created_at, updated_at
    kecermatan_test_packages  (test_id, package_id) PK, tenant_id, revision, required_count
                              -- P09's seam; test_id now has a foreign key to kecermatan_tests

ARCHITECTURE lists a unified `tests` model; as with P08's `tryouts`, a standalone table was built instead.

## Rules

Checked on save, and a failing save stores nothing: `kolom_count` even 10-50, `soal_per_kolom` 50-100 (D09),
`item_length` 1-5, `options_per_soal` 3-8, `kolom_duration_seconds` 1-3600 (the upper bounds for options and
duration are choices made here, not in the PRD), `#rrggbb` covers. The same limits are CHECK constraints.

Sources by `item_type`: `char`/`number` built in, no package; `symbol`/`emoji`/`image` exactly one package
of that type; `shape` none (all seven built-in) or one shape package; `mix` any of the built-ins
`char|number|shape` plus any packages. The server derives `builtin_sources` for non-mix types.

Pool (FR-ADM-KEC-10): deduplicated over all sources, at least `options_per_soal x item_length`. Text kinds
dedupe by character, so a symbol `A` and the letter `A` are one element. Otherwise `422 fields.pool`.

## Pinning

Create and update lock every package row `FOR UPDATE` (in id order) before writing `kecermatan_test_packages`,
pin the package's CURRENT revision and write `required_count` (the full need for one source; for a mix, what
the other sources do not already cover). P09's delete and shrink guards then apply. Every save re-pins at the
current revision; between saves the test keeps rendering the revision it pinned. Deleting a test unpins.

## Lock

`first_attempt_at` is NULL until P18's admission stamps it under the test row lock (`LockTx` is that lock).
When set, `locked` is true and edit/delete answer 409. A stale `base_revision` also answers 409. Tests prove
this by stamping the column directly; there are no sessions yet.

## Generator (`service/kecermatan_generator.go`)

Pure function of `(seed, pool order, params)`; no clock, no global RNG. Per generated column a PCG stream is
seeded from `sha256(seed || column)`, and the bounded draw is local rejection sampling, so a standard-library
change cannot alter content (a frozen test vector guards the algorithm). A partial Fisher-Yates samples
`options x length` elements without replacement into the labelled reference items (A, B, ...): no element
repeats within or across items of a column. Each soal hides one uniformly chosen item and shuffles the rest.

Only `kolom_count / 2` columns are generated (`GenerateKecermatan`); `Slot(k)` maps kolom k and k + N/2 to the
same content, and `AnswerKeys()` returns one independent row per kolom (N rows). `GenerateKecermatanColumn`
builds a single column. `KecermatanService.Pool` rebuilds the pool from the pinned revisions; P18 should
snapshot `GenerateKecermatan` output at admission and decide how a per-attempt seed derives from the stored one.
Cost is bounded: 50 kolom x 100 soal takes a few milliseconds.

## Preview

`POST /kecermatan-tests/{id}/preview {kolom?, seed?}` generates one column and stores nothing; it works on a
locked test. Kolom in the second half return the first half's content. Without `seed` a fresh one is drawn,
so the stored seed never leaves the server; the response carries the seed used and the `missing` key, which is
staff-only. No student endpoint exists in this slice.

## Layers

| Layer | File |
|---|---|
| repository | `repository/kecermatan.go` |
| service | `service/kecermatan.go` (rules, pinning, preview), `kecermatan_generator.go` |
| handler | `handler/kecermatan.go` |
| web | `app/admin/kecermatan`, `components/admin/kecermatan-builder.tsx` |
