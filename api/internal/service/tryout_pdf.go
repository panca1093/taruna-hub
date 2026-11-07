package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/pdfassets"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

// ErrPDFUnavailable marks a converter outage (transport failure, timeout, 502/503/504), as opposed to a document
// the converter rejected.
var ErrPDFUnavailable = errors.New("pdf converter unavailable")

const (
	exportCodeRendererUnavailable = "renderer_unavailable"
	exportCodeTryoutNotFound      = "tryout_not_found"
	exportCodeTryoutChanged       = "tryout_changed"
	exportCodePDFTooLarge         = "pdf_too_large"

	pdfMaxQuestions = 1000
	pdfMaxHTMLBytes = 40 << 20
)

// PDFFile is one file of the self-contained document; the first is index.html.
type PDFFile struct {
	Name string
	Data []byte
}

// PDFConverter turns that document into a PDF. It must not read anything the files do not contain.
type PDFConverter interface {
	HTMLToPDF(ctx context.Context, files []PDFFile) ([]byte, error)
}

// TryoutPDFParams is what the create endpoint stores: the tryout and the definition revision the file describes.
type TryoutPDFParams struct {
	TryoutID uuid.UUID `json:"tryout_id"`
	Revision int       `json:"revision"`
}

type TryoutPDFRenderer struct {
	repo *repository.TryoutRepository
	conv PDFConverter
}

func NewTryoutPDFRenderer(repo *repository.TryoutRepository, conv PDFConverter) *TryoutPDFRenderer {
	return &TryoutPDFRenderer{repo: repo, conv: conv}
}

func (r *TryoutPDFRenderer) Render(ctx context.Context, job model.Export) (ExportArtifact, error) {
	var params TryoutPDFParams
	if err := json.Unmarshal(job.Params, &params); err != nil || params.TryoutID == uuid.Nil {
		return ExportArtifact{}, &ExportError{Code: exportCodeRender, Err: errors.New("invalid tryout pdf params")}
	}
	tryout, err := r.repo.PrintTryout(ctx, job.TenantID, params.TryoutID)
	if errors.Is(err, repository.ErrNotFound) {
		return ExportArtifact{}, &ExportError{Code: exportCodeTryoutNotFound, Err: err}
	}
	if err != nil {
		return ExportArtifact{}, err
	}
	if tryout.Revision != params.Revision {
		return ExportArtifact{}, &ExportError{Code: exportCodeTryoutChanged,
			Err: fmt.Errorf("tryout is at revision %d, the export was requested for %d", tryout.Revision, params.Revision)}
	}
	files, err := BuildTryoutPDFFiles(tryout, job.CutOffAt)
	if err != nil {
		return ExportArtifact{}, err
	}
	data, err := r.conv.HTMLToPDF(ctx, files)
	if errors.Is(err, ErrPDFUnavailable) {
		return ExportArtifact{}, &ExportError{Code: exportCodeRendererUnavailable, Err: err}
	}
	if err != nil {
		return ExportArtifact{}, &ExportError{Code: exportCodeRender, Err: err}
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return ExportArtifact{}, &ExportError{Code: exportCodeRender, Err: errors.New("converter did not return a pdf")}
	}
	return ExportArtifact{Data: data, ContentType: "application/pdf", FileName: fmt.Sprintf("%s-r%d.pdf", slugify(tryout.Title), tryout.Revision)}, nil
}

// BuildTryoutPDFFiles is the whole render input: index.html, footer.html and the embedded KaTeX build. Subtests,
// questions and options come out in stored order; numbering restarts at 1 in every subtest, like the exam runtime.
func BuildTryoutPDFFiles(t model.PrintTryout, printedAt time.Time) ([]PDFFile, error) {
	total := 0
	for _, s := range t.Subtests {
		total += len(s.Questions)
	}
	if total > pdfMaxQuestions {
		return nil, &ExportError{Code: exportCodePDFTooLarge, Err: fmt.Errorf("%d questions exceed the %d limit", total, pdfMaxQuestions)}
	}
	script, style, err := pdfassets.KaTeX()
	if err != nil {
		return nil, fmt.Errorf("load katex: %w", err)
	}

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="id"><head><meta charset="utf-8"><title>`)
	b.WriteString(html.EscapeString(t.Title))
	b.WriteString(`</title><link rel="stylesheet" href="katex.min.css"><style>` + pdfCSS + `</style></head><body>`)
	b.WriteString(`<h1>` + html.EscapeString(t.Title) + `</h1>`)
	for i, s := range t.Subtests {
		cls := "subtest"
		if i > 0 {
			cls += " break"
		}
		fmt.Fprintf(&b, `<section class="%s"><h2>%s</h2><p class="meta">Durasi %s &middot; %d soal</p>`,
			cls, html.EscapeString(s.Name), pdfDuration(s.DurationSeconds), len(s.Questions))
		for n, q := range s.Questions {
			fmt.Fprintf(&b, `<div class="q"><div class="no">%d.</div><div class="body"><div class="stem">`, n+1)
			writeBlocks(&b, q.Content)
			b.WriteString(`</div>`)
			if len(q.Options) > 0 {
				b.WriteString(`<ol class="opts">`)
				for k, label := range q.Options {
					fmt.Fprintf(&b, `<li><span class="letter">%s.</span><span>%s</span></li>`, optionLetter(k), renderInline(label))
				}
				b.WriteString(`</ol>`)
			} else {
				b.WriteString(`<div class="blank"></div>`)
			}
			b.WriteString(`</div></div>`)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`<script src="katex.min.js"></script><script>` + pdfScript + `</script></body></html>`)
	if b.Len() > pdfMaxHTMLBytes {
		return nil, &ExportError{Code: exportCodePDFTooLarge, Err: fmt.Errorf("document is %d bytes, over the %d limit", b.Len(), pdfMaxHTMLBytes)}
	}

	footer := `<!doctype html><html><head><meta charset="utf-8"><style>` + pdfFooterCSS + `</style></head><body><div class="row"><span>` +
		html.EscapeString(t.TenantName) + ` &middot; ` + pdfDate(printedAt) +
		`</span><span>Halaman <span class="pageNumber"></span> / <span class="totalPages"></span></span></div></body></html>`
	return []PDFFile{
		{Name: "index.html", Data: []byte(b.String())},
		{Name: "footer.html", Data: []byte(footer)},
		{Name: "katex.min.css", Data: style},
		{Name: "katex.min.js", Data: script},
	}, nil
}

const pdfScript = `document.querySelectorAll('.m').forEach(function(el){katex.render(el.dataset.tex,el,{throwOnError:false,output:'html'});});window.__mathDone=true;`

const pdfCSS = `
body{font-family:"Noto Sans","Liberation Sans",Arial,sans-serif;font-size:11pt;line-height:1.45;color:#111;margin:0}
h1{font-size:18pt;margin:0 0 4pt}
h2{font-size:14pt;margin:0 0 2pt}
.meta{margin:0 0 12pt;color:#444;font-size:10pt}
.break{break-before:page}
.q{display:flex;gap:8pt;margin:0 0 12pt;break-inside:avoid}
.no{min-width:22pt;font-weight:700}
.body{flex:1;min-width:0}
.stem p,.stem div{margin:0}
.stem img{display:block;max-width:100%;max-height:7cm;margin:4pt 0}
.opts{list-style:none;margin:4pt 0 0;padding:0}
.opts li{display:flex;gap:6pt;margin:0 0 2pt}
.letter{min-width:14pt}
.blank{margin-top:6pt;border-bottom:1px dotted #777;height:16pt}
ul,ol.list{margin:2pt 0;padding-left:16pt}
`

const pdfFooterCSS = `body{margin:0;font:8pt "Noto Sans","Liberation Sans",Arial,sans-serif;color:#555}.row{display:flex;justify-content:space-between;box-sizing:border-box;width:100%;padding:0 0.6in}`

var (
	pdfImageLine = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)$`)
	pdfDataImage = regexp.MustCompile(`^data:image/(?:png|jpeg|gif|webp);base64,[A-Za-z0-9+/=]+$`)
	pdfBullet    = regexp.MustCompile(`^-\s+(.*)$`)
	pdfNumbered  = regexp.MustCompile(`^\d+\.\s+(.*)$`)
	pdfMath      = regexp.MustCompile(`\$[^$]+\$`)
	pdfBold      = regexp.MustCompile(`\*\*([^*\s](?:[^*]*[^*\s])?)\*\*`)
	pdfItalic    = regexp.MustCompile(`\*([^*\s](?:[^*]*[^*\s])?)\*`)
)

// writeBlocks mirrors web/components/latex.tsx: image lines, two-or-more-line lists, and inline math/bold/italic.
// An image whose source is not an inline image data URI is dropped to its alt text, so no URL reaches Chromium.
func writeBlocks(b *strings.Builder, content string) {
	var list []string
	ordered := false
	flush := func() {
		if len(list) == 0 {
			return
		}
		tag := "ul"
		if ordered {
			tag = `ol class="list"`
		}
		b.WriteString("<" + tag + ">")
		for _, item := range list {
			b.WriteString("<li>" + renderInline(item) + "</li>")
		}
		b.WriteString("</" + strings.Fields(tag)[0] + ">")
		list = nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := pdfImageLine.FindStringSubmatch(line); m != nil {
			flush()
			if pdfDataImage.MatchString(m[2]) {
				b.WriteString(`<img src="` + m[2] + `" alt="` + html.EscapeString(m[1]) + `">`)
			} else if m[1] != "" {
				b.WriteString(`<div>` + html.EscapeString(m[1]) + `</div>`)
			}
			continue
		}
		bullet, numbered := pdfBullet.FindStringSubmatch(line), pdfNumbered.FindStringSubmatch(line)
		var next []string
		if i+1 < len(lines) {
			next = append(pdfBullet.FindStringSubmatch(lines[i+1]), pdfNumbered.FindStringSubmatch(lines[i+1])...)
		}
		if item := firstGroup(bullet, numbered); item != "" && (len(list) > 0 || len(next) > 0) {
			if len(list) == 0 {
				ordered = bullet == nil
			}
			list = append(list, item)
			continue
		}
		flush()
		if line != "" {
			b.WriteString(`<div>` + renderInline(line) + `</div>`)
		}
	}
	flush()
}

func firstGroup(a, b []string) string {
	if a != nil {
		return a[1]
	}
	if b != nil {
		return b[1]
	}
	return ""
}

// renderInline escapes everything; a formula survives only as an attribute the page script typesets.
func renderInline(text string) string {
	var out strings.Builder
	last := 0
	emit := func(plain string) {
		plain = html.EscapeString(plain)
		plain = pdfBold.ReplaceAllString(plain, "<strong>$1</strong>")
		out.WriteString(pdfItalic.ReplaceAllString(plain, "<em>$1</em>"))
	}
	for _, loc := range pdfMath.FindAllStringIndex(text, -1) {
		emit(text[last:loc[0]])
		out.WriteString(`<span class="m" data-tex="` + html.EscapeString(text[loc[0]+1:loc[1]-1]) + `"></span>`)
		last = loc[1]
	}
	emit(text[last:])
	return out.String()
}

func optionLetter(i int) string {
	if i < 26 {
		return string(rune('A' + i))
	}
	return fmt.Sprint(i + 1)
}

func pdfDuration(seconds int) string {
	minutes := (seconds + 59) / 60
	if minutes >= 60 && minutes%60 == 0 {
		return fmt.Sprintf("%d jam", minutes/60)
	}
	if minutes > 60 {
		return fmt.Sprintf("%d jam %d menit", minutes/60, minutes%60)
	}
	return fmt.Sprintf("%d menit", minutes)
}

var indonesianMonths = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func pdfDate(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	t = t.In(loc)
	return fmt.Sprintf("%d %s %d", t.Day(), indonesianMonths[t.Month()-1], t.Year())
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if slug == "" {
		return "tryout"
	}
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	return slug
}
