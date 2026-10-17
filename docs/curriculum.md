# Curriculum: programs, subjects and chapters (P05)

Programs belong to a tenant. A program has subjects, a subject has chapters, and that hierarchy is
what later slices read for the "mata pelajaran" and "bab materi" dropdowns (FR-ADM-PRG-02). One
program is admin-only to change (FR-ADM-PRG-01); an Operator reads the tree, a Student does not.

    programs   id, tenant_id, code, name, description, position, created_at
                 UNIQUE (tenant_id, code)          -- the same code may exist in another tenant
    subjects   id, tenant_id, program_id, name, position, created_at
    chapters   id, tenant_id, subject_id, name, position, created_at

`position` is the stored order: programs append, subjects and chapters follow the order of the array
the client sends. The wire form is `Program {id, code, name, description, subjects: [{id, name,
chapters: [{id, name}]}]}`.

## What this slice deliberately does not have

Master content (`tenant_id NULL`), copy-to-own and origin markers are deferred (FR-TEN-09..13); there
is exactly one real tenant today, so `tenant_id` is `NOT NULL` and dropping a master scope later is a
non-destructive change. There is no capability matrix either: the guard is by role until D02 is
settled. `web/lib/mock-data.ts` keeps feeding the question editor, catalog and product pages until
their own slices.

## Why the keys are composite and every delete is RESTRICT

Both children carry a composite foreign key, `(program_id, tenant_id) -> programs (id, tenant_id)`, and
the parents carry `UNIQUE (id, tenant_id)` for it to point at. A subject row can therefore never name a
program of another tenant: the pair simply does not exist, so PostgreSQL answers `23503` instead of
silently accepting a cross-tenant link. The repository filters on `tenant_id` as well; the constraint
is the second line of defence behind the scope argument.

`ON DELETE RESTRICT` is deliberate. Questions and tryouts will reference chapters from P07 onwards, and
a database that refuses to delete a referenced row is the only place that rule can be enforced without
a race. Deleting a program is therefore explicit, inside one transaction: chapters, then subjects,
then the program row. A restrict violation from any step rolls the whole transaction back and answers
`409`, so a half-deleted tree never exists.

## Replace, not patch

`PUT /api/v1/programs/{id}` takes the complete tree. In one transaction, with the program row locked
`FOR UPDATE`, items that carry an `id` are updated, items without one are created, and items the body
no longer names are deleted. Positions are rewritten from the array order, and the ids of kept items
never change — the web page keeps them in its form so "add a chapter" does not recreate the program.

An `id` that belongs to another program (of this tenant or another) answers `422` with its path in
`fields`, for example `subjects[1].id`, and nothing is written. A chapter may move between subjects of
its own program, but not out of it. A program id from another tenant answers `404`, never `403`, so an
id does not reveal that it exists elsewhere.

## Rules, and where they live

Code is trimmed and uppercased, 2–12 characters of `[A-Z0-9_-]`, unique per tenant (`409` with
`fields.code` when taken). Names are trimmed, 1–100 characters, and unique among their siblings without
regard to case (`subjects[1].name`, `subjects[0].chapters[2].name` when not). A description is at most
500 characters, a program at most 50 subjects, a subject at most 200 chapters. Uniqueness of the names
is also a unique index on `lower(name)` inside the parent, so a race cannot slip a duplicate past the
service.

Business rules live in `internal/service/curriculum.go`; the repository only persists - including the
replace diff, which is given an already validated tree. Every route answers `Cache-Control: no-store`,
and the tree is always the tenant of the request host, because a token minted for another host never
authenticates here.

## Tryout count column

The page used to show a "Tryout" count read from the mock tryouts, which would be a misleading `0` for
a real program. It is removed here and returns with P08, when tryouts actually reference a program.
