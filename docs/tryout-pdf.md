# Tryout PDF (P20)

An Admin or Operator downloads a tryout as a printable PDF: questions only, no keys, no explanations. Student and
kecermatan PDFs do not exist (D11). Built on the shared export jobs in [exports.md](exports.md).

## Flow

1. `POST /api/v1/tryouts/{id}/pdf-exports` (Admin/Operator; Student `403`, anonymous `401`, foreign or unknown id
   `404`) reads the tryout through the caller's tenant and answers `202` with an `Export` of kind `tryout_pdf`.
   Its `params` are `{tryout_id, revision}`: the definition revision the file describes. Drafts export too.
2. The worker (`api/cmd/worker/exports.go`) runs `service.TryoutPDFRenderer`: load the print model, build a
   self-contained HTML document, send it to Gotenberg, store the PDF, email the requester. Retries, `failed` state,
   the ready notification and the download link are the foundation's.
3. The builder page shows the status list (`<ExportStatus kind="tryout_pdf" />`). It lists the latest five PDFs of
   the tenant, not only this tryout's; the file name `<title-slug>-r<revision>.pdf` tells them apart.

`GOTENBERG_URL` unset on the API answers `503` with code `pdf_unavailable` and queues nothing. Unset on the worker
leaves the kind unregistered, so a queued job fails with `no_renderer` after its retries.

## What goes into the PDF

- Order is what the admin stored: subtests by position, questions by position, options in stored order. The
  randomize flags never apply. Question numbers restart at 1 in each subtest, like the exam runtime; each subtest
  starts on a new page.
- The tryout title, each subtest's name, duration and question count, the stem (inline `$...$` math, `**bold**`,
  `*italic*`, lists, image lines, as `web/components/latex.tsx`), the option labels lettered A, B, C. A question
  without options (isian) gets one answer line. The footer on every page is the tenant name, the print date
  (Asia/Jakarta, taken from the export's cut-off) and page x / y.
- **Keys cannot reach it.** `TryoutRepository.PrintTryout` selects the stem and option labels only (labels are
  pulled out of the options JSON in SQL, so option ids and points are not read either) into `model.PrintTryout`,
  which has no field for a key, a correct option, an explanation or a point value.
- The tryout is rendered through each assignment's frozen question version. If its revision no longer equals
  `params.revision` the job fails with `tryout_changed` instead of describing something else.

## Rendering

Gotenberg 8.34.0 (Chromium) as a sidecar, called at `/forms/chromium/convert/html` with exactly four files:
`index.html`, `footer.html`, `katex.min.css` and `katex.min.js`. `api/internal/pdfassets` embeds KaTeX 0.17.0
(MIT, licence beside it) and its woff2 fonts; the stylesheet is rewritten at start-up so the fonts are `data:` URIs,
and the namespace URIs in the script are written with escaped slashes. No file contains `http://` or `https://`
(a test enforces it). Images are `![alt](data:image/...;base64,...)` lines already in the question text; any other
image source is dropped to its alt text. All question text is HTML-escaped; a formula survives only as a
`data-tex` attribute.

Math: the page script calls `katex.render` on each `.m` element and then sets `window.__mathDone = true`.
Gotenberg waits for `waitForExpression=window.__mathDone === true` and `failOnConsoleExceptions=true`, so a
broken script fails the job rather than printing raw LaTeX.

Text uses the sidecar's fonts (Noto Sans, then Liberation Sans). Licence validation of any other bundled font
remains the open P20/P09 item of ARCHITECTURE §Security.

## Limits and failure codes

| Guard | Value |
|---|---|
| Questions per tryout | 1000 (`pdf_too_large`) |
| Input HTML | 40 MB (`pdf_too_large`) |
| One conversion | 90 s client timeout; Gotenberg `--api-timeout=60s` |
| Response | 64 MB cap |
| Sidecar | 1.5 GB memory limit, queue of 10, Chromium restarted every 50 conversions |

`error_code` on a failed export: `tryout_not_found` (also another tenant's id; nothing is rendered),
`tryout_changed`, `pdf_too_large`, `renderer_unavailable` (connect error, timeout, 502/503/504),
`render_failed` (any other converter answer or a non-PDF body), plus the foundation's codes.

## Lockdown

`docker-compose.yml` starts Chromium with `--chromium-allow-list=^file:///tmp/.*` (where Gotenberg writes the
uploaded files), a matching deny-list, and `--chromium-deny-private-ips --chromium-deny-public-ips`. Verified:
a page referencing `http://example.com/...` and `file:///etc/passwd` renders without either. JavaScript stays
enabled because KaTeX needs it; the only script is the embedded build and the one-line page script.

## Run and test

```bash
docker compose up -d gotenberg                      # host port 3020
GOTENBERG_URL=http://localhost:3020 PDF_SMOKE_OUT=/tmp/p20.pdf \
  go test ./internal/infra -run RendersTryoutPDF -v  # real render, checked with pdftotext when installed
```

Native runs set `GOTENBERG_URL=http://localhost:3020` (see `api/.env.example`); compose sets
`http://gotenberg:3000` for the api and worker.
