# Assets: private image uploads and short-lived downloads (P06)

A user uploads an image, the API proves it is one, stores the bytes in S3-compatible storage and keeps
only metadata. Nobody reads an object except through a link the API signed a moment ago, after checking
tenant and role again. MinIO stands in locally; production is GCS through its HMAC (S3 interoperability)
keys, so the adapter speaks plain S3 and nothing is GCS-specific.

    assets   id, tenant_id, purpose, object_key UNIQUE, content_type, size_bytes, status,
             uploaded_by, created_at, finalized_at
               UNIQUE (tenant_id, id)
               FOREIGN KEY (uploaded_by, tenant_id) REFERENCES users (id, tenant_id)
               CHECK size_bytes BETWEEN 1 AND 5242880, CHECK (status = 'final') = (finalized_at IS NOT NULL)

`purpose` is `question_image` or `payment_proof`; `status` is `staged` or `final`. The row stores the
object key, never a URL: a signed link is minted per request and lives 60 seconds.

## Layers

| Layer | File | Holds |
|---|---|---|
| repository | `repository/asset.go` | SQL only; every method takes the tenant, except the purge scan |
| service | `service/asset.go` | every rule, and the small `ObjectStore` interface it needs |
| infra | `infra/storage.go` | the minio-go implementation: `Put`, `Delete`, `PresignGet`, `EnsureBucket` |
| handler | `handler/asset.go` | multipart parsing, the body cap, error mapping |

## The object key

`<tenant_id>/<purpose>/<asset_id>`, built in `AssetService.Upload` from the validated host's tenant, a
purpose the service checked against a closed list, and an id the server generated. The client's file
name, path, content type and any extra form field (`tenant_id`, `object_key`, `uploaded_by`, `id`,
`status`) are never read. `uploaded_by` is the caller from the token.

## Validation (before any byte is stored)

The handler caps the body at 5 MiB plus 64 KiB of multipart framing (`413` in the envelope when it
overflows); the service then refuses a file over exactly 5 MiB the same way. The type comes from the bytes:

- PNG and JPEG are recognized by magic number, then decoded in full with the standard library, so a
  truncated or bit-flipped file is `corrupt`. The size is read first and anything over 4096 x 4096 is
  refused before a pixel is allocated. At most two full decodes run at once; further uploads wait for
  a slot (or their request context).
- WebP is recognized by `RIFF…WEBP`; `golang.org/x/image` is not in the module graph, so the container
  and the first chunk (`VP8 `, `VP8L`, `VP8X`) are read by hand for the dimensions and for basic
  structure. Its pixel data is **not** decoded, so a WebP with valid headers and corrupt pixels is
  accepted: the Go standard library has no WebP decoder.
- Everything else — SVG, HTML, PDF, GIF, a renamed text file, an empty file — is `422` on `file`.

The stored `content_type` is the detected one. Rejections write nothing: no object and no row.

## Wire

| Route | Who | Answers |
|---|---|---|
| `POST /api/v1/assets` | any tenant user; `payment_proof` student/admin/operator, `question_image` admin/operator | `201` `{id, purpose, content_type, size_bytes, status}`, always `staged` |
| `GET /api/v1/assets/{id}/url` | the uploader, or admin/operator of the tenant for a `final` asset | `200` `{url, expires_at}`, `Cache-Control: private, no-store` |

Every route is `no-store` and behind `Authenticate`. A purpose the role may not upload is `403`; an
unknown purpose is `422` on `purpose`. With `S3_ENDPOINT` unset every route answers `503`
`service_unavailable` in the envelope and nothing else changes.

## Authorization

A foreign tenant's id, an unknown id, a malformed id and an asset the caller may not touch all answer
the **same `404`**, never `403` (the convention from the wallet), so ids cannot be probed. A staged
asset is readable only by its uploader — an admin cannot download it until it is final.
A student therefore reads only their own uploads; `DownloadURL` allows only the uploader or staff, so
a student cannot yet read a final `question_image`. That access is deferred to the slice that serves
questions to students (P16).

Authorization runs on every `url` call, but a link already issued works for anyone who holds it until it
expires; that is why the TTL is 60 seconds and the response is never cached.

## Finalize

There is no HTTP finalize route: an uploader could otherwise make its own upload permanent and skip the
purge. A slice that attaches an asset to its own row calls
`AssetService.FinalizeTx(ctx, tx, scope, principal, id)` inside its transaction, so the attachment and
the finalize commit or roll back together. Finalizing a `final` asset returns it unchanged and keeps the
first `finalized_at`.

## Cleanup

The worker's 10-minute loop calls `PurgeStaged(ctx, 24h)` when `S3_ENDPOINT` is set. Per candidate it
takes `SELECT … FOR UPDATE SKIP LOCKED` on a row that is still `staged` and old enough, deletes the
**object first**, then the row, then commits. A failed object delete rolls back and leaves a retryable
row; a concurrent finalize either wins before the lock (the row is no longer a candidate) or waits and
finds the row gone. A `final` row is never selected, because frozen content may point at it. The row
lock is held across one object delete, which is the price of that guarantee. At most 500 rows per tick.
`Upload` inserts the staged row before it writes the object, so a crash between the two leaves a row the
purge reclaims (deleting a missing object succeeds); a failed `Put` removes the row again.

## Configuration and startup

`S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_USE_SSL`, `S3_REGION` (see
`api/README.md`). The server calls `EnsureBucket` at startup: it **creates** the bucket only when
`APP_ENV=development` and otherwise fails startup if the bucket is missing. No bucket policy is ever
set, so the bucket stays private.

## Tests

Storage is real MinIO, never a mock; the tests skip when `S3_ENDPOINT` is unset and CI starts MinIO.
They cover: wrong type with the right extension, SVG, 5 MiB exactly and + 1 byte, a corrupt and a
truncated PNG, oversize dimensions, no storage write for any rejected file, tenant B uploading with A's
token, finalizing (through `FinalizeTx`) and downloading A's asset (`404`, state unchanged), client path/purpose/owner fields
ignored, a student uploading `question_image`, another student and a staff member reading a staged
proof, `FinalizeTx` twice, rolled back and committed, and refused for another tenant or student, an unsigned GET (`403`), a tampered
signature, an expired link (a 1-second TTL through the service constructor), purge keeping final and
fresh rows and surviving a failing object delete, and `503` with no storage.

## What this slice deliberately does not do

No upload or preview controls in the web app, no public branding assets (P04), no cover, export or
payment-proof screens beyond the two purposes (P13/P15/P21), no signed-URL CDN, no image resizing or
re-encoding (bytes after a PNG's `IEND` are stored as sent), no virus scanning, no per-tenant quota, and
no GCS run: **GCS interoperability and staging credentials are unverified** and the PLAN gate stays open.
A fully decoded 4096 x 4096 image can take tens of MB while it is checked; the two-slot decode limit
bounds that per process.
