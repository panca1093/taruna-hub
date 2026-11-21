# Tryout PDF (P20)

An Admin or Operator downloads a tryout as a printable PDF: questions only, no keys, no explanations. Since the owner
decision of 2026-11-21 the PDF is generated in the admin's browser, the same way the per-student result PDFs are
(`web/lib/capture-pdf.ts`). There is no server-side PDF, no Gotenberg sidecar, no export job, no ready email and no
stored file. FR-ADM-TRY-12 (asynchronous PDF with notification and link) is superseded; see
[RECONCILIATION](../requirement/RECONCILIATION.md) section 2. Accepted trade-offs: the text is an image (not selectable
or searchable), the admin waits in the tab while it renders, and there is no history of generated PDFs.

## Flow

1. The builder's "Unduh PDF" button (enabled once the form has no unsaved changes) calls
   `GET /api/v1/tryouts/{id}/print` (Admin/Operator; Student `403`, anonymous `401`, foreign or unknown id `404`).
2. `web/components/admin/tryout-print.tsx` mounts an off-screen, `inert`, light-theme layout from that data, measures
   every block at the page's content width and flows the blocks into A4 pages (794 x 1123 CSS px).
3. `savePagesAsPdf` (`web/lib/capture-pdf.ts`) captures the pages one after another with `html-to-image` (JPEG, 2x) and
   writes one PDF with one A4 page per image. The button shows "Halaman x dari N" while it runs; failure shows the
   error with a retry button. The file is named `<title-slug>.pdf`.

## What goes into the PDF

- Order is what the admin stored: subtests by position, questions by position, options in stored order. The randomize
  flags never apply. Question numbers restart at 1 in each subtest, like the exam runtime.
- The tryout title, each subtest's name, duration and question count, the stem and option labels through
  `LatexText` (math, bold/italic, lists, images), options lettered A, B, C. `isian` gets an answer line;
  `benar_salah` shows its two stored options. The footer on every page is the tenant name, the print date
  (Asia/Jakarta) and "Halaman n / N".
- **Keys cannot reach the browser.** `TryoutRepository.PrintTryout` selects type, stem and option labels only (labels
  are pulled out of the options JSON in SQL, so option ids and points are not read either) into `model.PrintTryout`,
  which has no field for a key, a correct option, an explanation or a point value. The handler test asserts the body
  has none.
- The tryout is read through each assignment's frozen question version, in one read-only repeatable-read transaction.
  The tryout and assignment API does not carry options, which is why this read exists.

## Pagination

A question (stem and options) is never split across pages. A subtest heading, and the title before the first heading,
stay on the page of the block after them. A block taller than a page goes alone on its own page, scaled down to fit.
`paginate` in `tryout-print.tsx` is a pure function of measured heights.

## Test

```bash
cd api && go test ./internal/handler -run TryoutPrint -count=1   # needs DATABASE_URL and REDIS_URL
```

The visual result (fonts, KaTeX glyphs, page breaks) is checked in a browser from the tryout builder.
