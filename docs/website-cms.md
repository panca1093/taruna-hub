# Website CMS (P11b)

An Admin edits the marketing content of the public website, saves a private draft, previews it and
publishes one whole-site snapshot. Visitors read only the published snapshot. This page is the
contract and the invariants; the wire shapes are in `api/openapi.yaml` and the plan is
[P11b](../requirement/PLAN.md#p11b).

**Status:** the contract is defined (OpenAPI, typed client `api.website.*`, `normalizeWebsiteContent` and
its tests) and the API is built: migration `000022_website_content`, the repository, `WebsiteService` and
the five endpoints. The portal editor (`/admin/website/*`), its review and the protected draft preview are built; the browser→API→database proof and the public `/programs/[slug]` routes are not done yet.

## Owner decisions

Recorded by Saifullah Panca, 2026-11-01.

| Gate | Decision |
|---|---|
| G1 permissions | Approved. Admin only for workbench read, save, preview, history and publish. Operator and Student are `403`, anonymous is `401`. |
| G2 publication boundary | Approved. One atomic whole-site snapshot; no per-page publish. |
| G3 content contract | Approved. Exactly the contract below. |
| G4 launch content | **Open.** The owner must approve the real copy and supply official program and contact data, or accept the visible "information pending" states. It blocks the first real publication and launch, not building or testing with fixtures. |

## Content contract

`WebsiteContent` is `schema_version: 1` plus `locales` with exactly `id` and `en`, both required. Each
locale has `home` and `programs`. `programs` has exactly three stable marketing slots: `tni`, `polri`,
`kedinasan`. They are not curriculum Program ids and have no add, delete, reorder or slug editor.
Unknown fields, locales and slots are refused (an unknown field is `validation_failed`, an unknown locale or slot `website_content_invalid`). A tenant, role, publisher or timestamp in a request body
is never accepted.

| Scope | Field | Max code points | Required |
|---|---|---|---|
| home | `hero_title` | 90 | yes |
| home | `hero_body` | 360 | yes |
| home | `program_cta` | 36 | yes |
| home | `practice_cta` | 36 | yes |
| each program | `title` | 80 | yes |
| each program | `summary` | 320 | yes |
| each program | `schedule`, `location`, `fees`, `contact` | 160 each | no |

- Text is trimmed at both ends before it is validated, diffed or stored. Length counts Unicode code
  points, not UTF-8 bytes and not JS UTF-16 units, so an emoji is one.
- A required text cannot be blank after trimming. An optional field is always a present string; empty
  means the information is pending, and absent or null is a validation error.
- Everything is plain text. There is no HTML, markdown, script or automatic linkification; React
  escapes it on render, and an HTML-looking value is stored as is and shown as text.
- `fees` is public informational text, never a checkout amount or price authority. Product names,
  prices, quota and sale status come only from the catalog.
- Fixed in code, not CMS data: CTA destinations (program CTA `/#programs`, practice CTA `/catalog`,
  login `/login`, register `/register`, program details `/programs/tni|polri|kedinasan`), FAQ, Cara
  Kerja, section labels, layout, branding and navigation. An unknown slot route is `404`. No
  program-interest CTA ships until a real inquiry channel is approved.
- Prototype camelCase keys map to these snake_case fields; `heroCta` became `practice_cta`.

`normalizeWebsiteContent` in `internal/service/website.go` does the trimming and validation and returns
every failing field at once as an `InputError` whose keys are dotted paths:
`schema_version`, `locales.<id|en>`, `locales.<l>.home.<field>`, `locales.<l>.programs.<slot>` and
`locales.<l>.programs.<slot>.<field>`. Reasons: `website_required`, `website_too_long`,
`website_schema_version`, `website_locale`, `website_program_slot`. The handler maps the error to
`422 website_content_invalid`.

## Endpoints

All under `/api/v1`; every response is `Cache-Control: no-store`.

| Method and path | Access | Request → response |
|---|---|---|
| `GET /website` | anonymous | → `PublishedWebsite`, or `404 website_not_published` |
| `GET /admin/website` | Admin | → `WebsiteWorkbench` |
| `PUT /admin/website` | Admin | `{expected_draft_revision, content}` → latest `WebsiteWorkbench` |
| `POST /admin/website/publish` | Admin | `{expected_draft_revision, expected_published_revision}` → `WebsitePublication` |
| `GET /admin/website/history?limit&cursor` | Admin | → `WebsitePublicationList`; `limit` 1–50, default 20 |

The public route resolves the one configured tenant exactly like `GET /tenant`. Anonymous on a protected
route is `401`; an authenticated non-Admin is `403`.

## DTOs and errors

- `PublishedWebsite`: `content`, `published_revision`, `published_at`. Nothing else: no actor id, draft,
  history, workbench or future content.
- `WebsiteWorkbench`: `draft`, `draft_revision`, `published_revision` (0 when nothing is published),
  `published` (content or null, for the diff), `updated_at`, `updated_by`. It always carries both
  revisions; the frontend compares revisions for concurrency, never timestamps.
- `WebsitePublication`: `published_revision`, `source_draft_revision`, `published_at`, `published_by`,
  `changed`.
- `WebsitePublicationList`: `items` newest first and `next_cursor` (null on the last page); the cursor
  is opaque and deterministic.
- Revisions are non-negative 64-bit integers kept within the JS safe-integer range; database increments
  are guarded against overflow.

| Status | `error.code` | When |
|---|---|---|
| 401 / 403 | `unauthorized` / `forbidden` | anonymous / authenticated non-Admin on a protected route |
| 404 | `website_not_published` | `GET /website` before the first publication |
| 409 | `website_revision_conflict` | a stale `expected_draft_revision`, a stale `expected_published_revision`, or a retry whose draft has moved |
| 413 | `payload_too_large` | body over 64 KiB (the schema's worst case fits below it) |
| 422 | `website_content_invalid` | content breaks the contract; `fields` maps each path to a reason |
| 422 | `validation_failed` | an unreadable body, trailing data, an unknown request field, a revision outside its range, or a malformed `limit` or `cursor` |

The three `website_*` codes are lowercase like every other domain code in the envelope enum.

## Persistence

`website_content`: one row per configured tenant with `schema_version`, `draft` JSONB, `draft_revision`,
nullable `published` JSONB, `published_revision`, `updated_at`/`updated_by` and `published_at`/`published_by`.
`website_publications` is append-only metadata (tenant_id, `published_revision` unique within the tenant,
`source_draft_revision`, actor, timestamp). It is an audit trail, not a rollback archive. There is no
pruning or delete API in v1, and the prototype's ten-row truncation is not copied.

The row is created lazily in a scoped transaction on the first workbench read: the draft is seeded with
reviewed default hero and CTA text and three descriptive slot titles and summaries (optional program
fields empty), `draft_revision` 1, `published_revision` 0, `published` null. Two first readers create one
row. A migration, seed or demo never publishes and never modifies existing content. The default ID/EN text
is included in the implementing PR for owner review.

Rules live in `WebsiteService`; the repository takes the tenant scope and only persists. Transactions
follow the existing pattern; the tenant comes from configuration and the actor from authenticated claims.
There are no platform-admin or cross-tenant features (D17).

## Save, publish, retry and concurrency

**Save**
- Lock the scoped row, then compare `expected_draft_revision` before any write. Validate the normalized
  complete content.
- Content equal to the stored draft is a no-op: same revision, time and actor.
- Changed content increments `draft_revision` exactly once.
- A stale save answers the conflict and does not overwrite newer edits.
- Saving never changes the published revision or content.

**Publish**
- Lock the same row. Compare both revisions, validate the draft, copy the entire draft into `published`
  and insert one history record, all in one transaction. No external call happens inside it.
- The first publication is revision 1; a later changed publication increments the published revision
  once. `draft_revision` is unchanged.
- A draft equal to the published content is a no-op: `changed=false`, no new history record.
- Lost-response retry: a request with the same `source_draft_revision` that was already published returns
  that recorded publication unchanged, even though its `expected_published_revision` is now stale. If
  the draft has moved on, the answer is the conflict. A retry never publishes a newer, unseen draft.
- A database failure rolls back, leaving the published content, revision and history unchanged.
- Two concurrent publishes of the same draft produce one history record.
- A stale publisher cannot publish someone else's latest edit.
- The audit actor comes from authenticated claims. Request logs carry no secrets, credentials or
  content bodies.

## Caching

`Cache-Control: no-store` on the public endpoint, the protected draft and history endpoints and the
preview pages. There is no global or cross-request draft cache and no CDN invalidation in this slice. A
visitor sees new content on the next load or refetch; there is no push to other browsers.

## Preview and privacy

- Preview is a protected Admin route (`/admin/website/preview`; `?program=tni|polri|kedinasan` only
  selects a view, it never grants access). It reads the saved draft through the authenticated API and
  stays Admin-guarded on a direct URL or reload, with a visible draft banner and a link back to the editor.
- The public `/website` route never accepts `cms=draft`. Draft text is never serialized into public
  HTML, public caches or public URLs. An auth failure on a fresh or reloaded preview shows the existing
  login or forbidden behavior and never falls back to anonymous draft access.
- Preview and the public page share one presentation component that takes typed content, a mode and the
  real catalog data. There is no public preview or share link.
- The editor starts clean after the server load; typing updates the local preview without API writes.
  Save is explicit, "Simpan & preview" saves first when dirty and opens the preview only on success, and
  a plain preview shows the last saved draft. Unsaved text is never labelled saved and inputs survive
  errors. On a conflict the local edits stay, the user is told a newer version exists and may reload only
  after confirming the discard; there is no silent merge. Dirty navigation warns. One user action must
  not trigger a duplicate save or publish, and an unknown network outcome is followed by a refetch of the
  workbench and history before any retry. Publish is disabled until the draft is saved and differs from
  the published content (the API enforces every rule regardless), and the review checkbox is an
  acknowledgement, not authorization.

## Public data states

CMS missing (`404`) or a network or server error shows the reviewed static hero and CTA from the landing
dictionary plus a visible "program information unavailable" state, with a retry, and never draft content.
Empty optional program fields show the dictionary "Menunggu informasi resmi" label with no enabled
inquiry CTA. Program routes with no published snapshot are unavailable or `404`. The catalog loads,
empties and fails independently, and a CMS failure never breaks login or catalog navigation.

## Acceptance (browser, API, database)

API tests:
- Admin only; anonymous `401`, Student and Operator `403`; the scoped tenant is required.
- Strict body: unknown key, locale or slot, every limit at the limit and +1, trim, multibyte, and 64 KiB.
- Concurrent first reads create one row; save no-op and conflict; the public DTO is published-only.
- Publish: atomic failure, duplicate and concurrent publish, lost-response retry, unseen-draft conflict.
- History: ordering, cursor pagination and ownership; stored text stays inert.

Browser, API and database evidence:
- Admin saves a new headline → the private preview matches the saved revision → anonymous public is still
  old → the review shows ID and EN and every program diff → publish → a new public load matches the
  committed revision and there is exactly one history row.
- Two editors conflict and the local edits are preserved.
- Anonymous, Student and Operator are denied on the API and on the preview route.
- A failed publish keeps the old public version.
- A fresh session or reload, a no-op save and publish, an unknown program slug and the catalog states.
- Widths 320, 390, 768, 1024 and 1440 px; light and dark; ID and EN; keyboard and focus; contrast;
  reduced motion; no horizontal page overflow.

Prototype screenshots are visual-only and are not API or database proof. Nothing is marked done on
intent; the plan boxes are ticked only for scenarios a test or demo proved.

## Not included

Lead inbox, real enrollment, program payments, contact-message submission, media library, SEO editor,
arbitrary HTML or URLs or scripts, drag-and-drop sections, scheduled publish, rollback, public preview
links, CMS export, tenant switcher, and any change to exam, scoring or finance behavior.
