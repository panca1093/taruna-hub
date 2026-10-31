# Kecermatan item packages (P09)

A package is a named pool of elements a kecermatan test draws its reference items from. Four types:
`symbol` (typed or pasted text, a built-in palette, or a CSV/TXT/XLSX file), `image` (many files or a ZIP),
`emoji` (picked from a built-in catalog of 345) and `shape` (17 built-in flat and 7 solid shapes). Tenant-owned only:
master packages and copy-to-own are out of the MVP (D17). Admin and Operator may change packages (D02).

    item_packages           id, tenant_id, type, name, current_revision, created_by, created_at, updated_at
                              UNIQUE (tenant_id, lower(btrim(name)))
    item_package_revisions  (package_id, revision) PK, element_count        -- frozen: UPDATE is refused
    item_package_elements   (package_id, revision, element_id) PK, position, kind, key, value, label, asset_id
                              UNIQUE (package_id, revision, key)           -- frozen: UPDATE is refused
    kecermatan_test_packages  (test_id, package_id) PK, tenant_id, revision, required_count
                              -- the seam P10 plugs into; both foreign keys are ON DELETE RESTRICT

## Revisions

Every content change (an accepted commit, an element removal) writes the next full revision: a copy of the
previous elements plus or minus the change. `element_id` is stable across revisions. A rename writes no
revision. A test (P10) pins a `(package, revision)` through `kecermatan_test_packages`, so it keeps rendering
exactly what it was built from; `GET /item-packages/{id}?revision=N` reads any frozen revision. Image assets
of an older revision stay `final` and downloadable. Deleting a package deletes its revisions but not its
assets: orphaned `item_image` assets are not garbage-collected yet.

`key` is the dedup identity: the symbol itself, the emoji or shape id, or the image's SHA-256. Names of
images are labels for the dashboard only.

## Upload: preview, then commit

Both routes take the same multipart parts and the server judges them itself; the browser never sends
verdicts. `elements/preview` writes nothing (no row, no object, no revision). `elements/commit` judges again
and writes the accepted elements as ONE new revision in one transaction, under a row lock on the package.
It is idempotent: re-sending finds everything already present, reports `added: 0` and writes no revision.
Image objects are stored as staged assets first and finalized inside the revision's transaction, so a failed
commit leaves only staged rows the existing purge job deletes.

Per-element reasons: `duplicate`, `duplicate_in_upload`, `emoji_in_symbol`, `symbol_multi`, `symbol_invalid`,
`image_type`, `image_too_large`, `image_corrupt`, `image_dimensions`, `path_traversal`, `symlink`,
`nested_archive`, `not_in_catalog`, `package_full`. A ZIP that breaks an archive-wide limit is rejected as one
element: `archive_too_large`, `archive_too_many_files`, `archive_extracted_too_large`, `archive_corrupt`.

Limits (D09): ZIP at most 10 MB, 200 files, 50 MB extracted (declared sizes are summed first, then the bytes
actually inflated are counted); images at most 5 MB each, 4096 x 4096, PNG/JPEG/WebP only (SVG is an
`image_type`). Not in D09 and chosen here: 500 elements per package, 1000 symbols and 5 MB per symbol file,
64 MB request body.

## Symbols and emoji

A symbol is exactly one character. A token that contains an emoji (any pictographic block, the BMP
emoji-presentation symbols, variation selector, ZWJ, keycap, tag characters) is `emoji_in_symbol`, and the
message points at the Emoji package. Arrows, card suits and geometric shapes stay legal.

The add dialog filters the emoji catalog by its Indonesian name. Shapes come back from the catalog with a
`group` (`flat` or `solid`) and the dialog lists the two groups separately. A shape test without a shape package
still draws only from the first seven flat shapes (`defaultShapeCount`): the pool is resolved at every admission,
so growing it would change tests that are already locked. The newer shapes are reached through a shape package.

Emoji are catalog-only: single-codepoint, no skin tones, flags or ZWJ sequences. An element stores the hex
codepoint (`1f34e`) and is drawn from `web/public/emoji/<id>.svg`, a bundled Twemoji asset, so it looks the
same on every device and nothing is fetched at runtime. Twemoji graphics are CC-BY 4.0; the license text is
`web/public/emoji/LICENSE-GRAPHICS.txt` and `GET /item-packages/catalog` returns `emoji_attribution`, which
the UI must show wherever the catalog is used. A test keeps `emojiCatalog` and the bundled files in step.

## Guards for P10

`Delete` and `RemoveElement` lock the package row. A pinned package is refused with `package_in_use` (delete)
or `package_too_small` (removal that would leave fewer elements than the largest `required_count`). The
foreign keys refuse a raw delete of a pinned package or revision too. P10 must insert into
`kecermatan_test_packages` in a transaction that first takes `FOR SHARE`/`FOR UPDATE` on the package row, add
the foreign key on `test_id`, and check the pool against `options_per_soal x item_length` (FR-ADM-KEC-10).

## Sample column

`POST /item-packages/{id}/sample-column` draws `soal_per_kolom` questions from the current elements, each
`options_per_soal` items of `item_length` distinct elements. It stores nothing and is not metered. It is a
pool preview, not the P10 generator: no seed, no reference item, no per-session stability.

## Layers

| Layer | File |
|---|---|
| repository | `repository/item_package.go` |
| service | `service/item_package.go` (rules), `item_package_input.go` (parsing, ZIP, classification), `item_package_catalog.go` (emoji, shapes, palette) |
| handler | `handler/item_package.go` |
