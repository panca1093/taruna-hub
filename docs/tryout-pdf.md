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

`paginate` in `tryout-print.tsx` is a pure function of measured heights; the page body is 794 x 1123 CSS px minus the
margins and footer. Rules, in the order the code applies them:

1. A question (stem and options) is never split across pages.
2. A group is the title and subtest headings ("keep" blocks) plus the block after them. If the group fits a page, it
   moves to a fresh page when the current page cannot hold all of it, so a heading is never the last thing on a page.
3. A block that exactly fills the remaining height fits (`used + height <= body`); the next block starts a new page.
4. If a group is taller than a page, it gets its own page: the keep blocks at natural size, and the last block scaled
   down to the height that remains. A lone oversize question is the same case with no keep blocks.
5. Every subtest after the first starts on a new page (owner decision 2026-11-21); the first subtest stays under the
   title on page 1. No blank page is emitted when the break arrives on a page that is already empty.
6. An empty subtest's heading has no question to stay with: it sits alone on its own page, except an empty first
   subtest, which stays under the title on page 1.

Checked by reading the code and running `paginate` on sample heights: several subtests (each later one opens a page),
empty subtest in the middle and at the end (alone on its page), heading plus oversize question (rule 4, no orphan
heading), title plus heading plus oversize first question (rule 4), exact-fit page then a break (rule 3, no blank
page), one subtest and no subtests (unchanged).

## Images

Before blocks are measured, and again after the pages are laid out, `inlineImages` (`web/lib/capture-pdf.ts`) waits for
every `<img>` to load or fail, fetches its source and swaps in a `data:` URL, so `html-to-image` never has to fetch
anything (the multi-page capture runs with `cacheBust: false`). An image that fails to load or fetch is replaced by a
small bordered box with its alt text (or "Gambar tidak dapat dimuat"), never a blank gap. Cross-origin images need CORS
headers to be fetchable; otherwise they print as the alt-text box.

## Test

```bash
cd api && go test ./internal/handler -run TryoutPrint -count=1   # needs DATABASE_URL and REDIS_URL
```

The visual result (fonts, KaTeX glyphs, page breaks) is checked in a browser from the tryout builder.
