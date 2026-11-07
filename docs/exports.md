# Exports (shared foundation for P20 and P21)

Durable artifacts rendered by a background job: the tryout PDF (P20) and the tryout results CSV/XLSX (P21). This
page covers the shared parts. Each kind's content, params and create endpoint belong to its own slice.

## Data model

Table `exports` (migration 000023), one row per request.

| Column | Meaning |
|---|---|
| `kind` | `tryout_pdf`, `tryout_results_csv` or `tryout_results_xlsx` (CHECK) |
| `params` | JSON the owning slice defined; the renderer reads it, nothing else |
| `status` | `queued`, `running`, `ready`, `failed` |
| `requested_by` | staff user, composite FK with `tenant_id` |
| `cut_off_at` | `now()` at request time; a late render still describes this moment |
| `object_key`, `content_type`, `file_name`, `size_bytes` | set when `ready` (CHECK: ready needs key, name, type) |
| `error_code`, `attempts` | last failure code (empty on success), attempts started |

The object lives at `<tenant_id>/export/<id>` in the same bucket as assets. `object_key` is never in an API body.

## Lifecycle

1. `ExportService.Request` inserts the row (`queued`) and the River job (queue `exports`, 3 attempts) in one
   transaction.
2. The worker calls `Run(tenantID, exportID, attempt, maxAttempts)`: claim (`running`, `attempts+1`), call the
   registered renderer, upload, then in one transaction mark `ready` and enqueue the `export-ready` email to the
   requester (event key `export-ready:<id>`, so it is sent once).
3. A finished (`ready` or `failed`) or unknown export is a no-op, so a restarted worker never renders twice.

## Retries

A failed attempt stores `error_code` and returns the row to `queued`; `Run` returns the error so River retries
(its default backoff). On the last attempt (`attempt >= maxAttempts`) the row becomes `failed` and `Run` returns
nil. A renderer may return `*service.ExportError{Code, Err}` to choose the code the admin sees; any other error is
`render_failed`. Other codes: `upload_failed`, `storage_unavailable`, `no_renderer`, `internal_error`. A missing
renderer is retried like any failure, so a rolling deploy where the worker is older than the API still recovers.

A cancelled or timed-out attempt on the last try (`attempt >= maxAttempts`) also ends the row as `failed` with
`render_timeout`, written on a context that survives the cancellation. If the worker dies outright, the periodic
reconcile in the worker calls `FailStale`, which marks every `running` export untouched for longer than
`ExportJobTimeout x ExportMaxAttempts + ExportStaleMargin` as `failed` with `worker_lost`, across tenants. `queued`
rows are never swept: River still holds their job, so a long worker outage or a backlog only delays them.

## Authorization

Admin and Operator only (D02, role-only). Student is `403`. Every call re-checks the principal against the
configured tenant; another tenant's id, an unknown id and a malformed id are all `404`. `export_not_ready` (`409`)
answers a download link for anything but `ready`. The link is a 60-second presigned GET that saves the file under
`file_name` (`Content-Disposition`), issued on every call and never stored. Renderers never receive or fetch a URL.

## Endpoints

- `GET /api/v1/exports?kind=&limit=` newest first, default 20, max 50.
- `GET /api/v1/exports/{id}`
- `POST /api/v1/exports/{id}/download-link` returns `{url, expires_at}`.

## Adding a kind

Renderer, in `api/cmd/worker/exports.go`:

```go
func registerExportRenderers(svc *service.ExportService, deps exportDeps) {
	svc.Register(model.ExportTryoutPDF, newTryoutPDFRenderer(deps.pool, deps.store, deps.logger))
}
```

`Render(ctx, job model.Export) (service.ExportArtifact, error)` decodes `job.Params`, reads data as of
`job.CutOffAt`, and returns bytes, content type and a file name (sanitized by the service).

Create endpoint, in the slice's handler: validate the params, then

```go
return h.acceptExport(c, model.ExportTryoutPDF, params) // 202 with the Export body
```

and register the route with `Authenticate(h.auth)` plus the staff guard. Frontend: render
`<ExportStatus kind="tryout_pdf" refreshKey={n} />` from `components/admin/export-status.tsx` and bump `refreshKey`
after the `202`; it polls every 3 seconds while any of the latest five is queued or running.
