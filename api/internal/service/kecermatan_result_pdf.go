package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const (
	exportCodeStudentResultNotFound = "student_result_not_found"

	kecPDFMaxAttempts = 100
)

// KecermatanResultPDFParams is what the create endpoint stores; the report describes the completed attempts at render time.
type KecermatanResultPDFParams struct {
	StudentID uuid.UUID `json:"student_id"`
	TestID    uuid.UUID `json:"test_id"`
}

// PreparePDF reads the pair through the caller's tenant, so a foreign or unknown id, or a student without a
// completed attempt, is the usual not-found.
func (s *KecermatanRuntimeService) PreparePDF(ctx context.Context, scope HostScope, principal model.Principal, studentID, testID uuid.UUID) (KecermatanResultPDFParams, error) {
	if _, err := s.StaffStudentResults(ctx, scope, principal, studentID, testID); err != nil {
		return KecermatanResultPDFParams{}, err
	}
	return KecermatanResultPDFParams{StudentID: studentID, TestID: testID}, nil
}

// KecermatanPDFAttempt is a completed attempt with the per-kolom counts the report prints. It has no field for
// a generated column, an answer or a key.
type KecermatanPDFAttempt struct {
	Attempt model.KecAttempt
	Counted bool
	Kolom   []model.KecKolom
}

type KecermatanPDFReport struct {
	TenantName string
	Student    KecStudentSummary
	Attempts   []KecermatanPDFAttempt
}

type KecermatanResultPDFRenderer struct {
	db   *pgxpool.Pool
	repo *repository.KecermatanRuntimeRepository
	conv PDFConverter
}

func NewKecermatanResultPDFRenderer(db *pgxpool.Pool, repo *repository.KecermatanRuntimeRepository, conv PDFConverter) *KecermatanResultPDFRenderer {
	return &KecermatanResultPDFRenderer{db: db, repo: repo, conv: conv}
}

func (r *KecermatanResultPDFRenderer) Render(ctx context.Context, job model.Export) (ExportArtifact, error) {
	var params KecermatanResultPDFParams
	if err := json.Unmarshal(job.Params, &params); err != nil || params.StudentID == uuid.Nil || params.TestID == uuid.Nil {
		return ExportArtifact{}, &ExportError{Code: exportCodeRender, Err: errors.New("invalid kecermatan result pdf params")}
	}
	results, err := loadKecStudentResults(ctx, r.repo, r.db, job.TenantID, params.StudentID, params.TestID)
	if errors.Is(err, ErrKecSessionNotFound) {
		return ExportArtifact{}, &ExportError{Code: exportCodeStudentResultNotFound, Err: err}
	}
	if err != nil {
		return ExportArtifact{}, err
	}
	if len(results.Attempts) > kecPDFMaxAttempts {
		return ExportArtifact{}, &ExportError{Code: exportCodePDFTooLarge,
			Err: fmt.Errorf("%d attempts exceed the %d limit", len(results.Attempts), kecPDFMaxAttempts)}
	}
	tenantName, err := r.repo.TenantName(ctx, job.TenantID)
	if err != nil {
		return ExportArtifact{}, err
	}
	report := KecermatanPDFReport{TenantName: tenantName, Student: results.KecStudentSummary}
	for i, a := range results.Attempts {
		kolom, err := r.repo.Kolom(ctx, r.db, job.TenantID, a.SessionID)
		if err != nil {
			return ExportArtifact{}, err
		}
		report.Attempts = append(report.Attempts, KecermatanPDFAttempt{Attempt: a, Counted: i < kecProgressWindow, Kolom: kolom})
	}
	files, err := BuildKecermatanResultPDFFiles(report, job.CutOffAt)
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
	return ExportArtifact{Data: data, ContentType: "application/pdf",
		FileName: fmt.Sprintf("kecermatan-%s-%s.pdf", slugify(report.Student.StudentName), slugify(report.Student.TestTitle))}, nil
}

// BuildKecermatanResultPDFFiles is the whole render input: a self-contained index.html and the footer. Every
// string that came from a student or a staff member is escaped; the numbers are formatted here.
func BuildKecermatanResultPDFFiles(r KecermatanPDFReport, printedAt time.Time) ([]PDFFile, error) {
	if len(r.Attempts) > kecPDFMaxAttempts {
		return nil, &ExportError{Code: exportCodePDFTooLarge, Err: fmt.Errorf("%d attempts exceed the %d limit", len(r.Attempts), kecPDFMaxAttempts)}
	}
	s := r.Student
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="id"><head><meta charset="utf-8"><title>` + esc("Hasil Kecermatan "+s.StudentName) +
		`</title><style>` + kecPDFCSS + `</style></head><body>`)
	b.WriteString(`<h1>` + esc(s.StudentName) + `</h1>`)
	b.WriteString(`<p class="meta">` + esc(s.TestTitle) + ` &middot; ` + esc(s.ProgramName) + ` (` + esc(s.ProgramCode) + `)</p>`)

	if p := s.Progress; p != nil {
		fmt.Fprintf(&b, `<h2>Ringkasan</h2><div class="cards"><div class="card hero"><span>Nilai gabungan</span><strong>%s</strong><small>rata-rata %d percobaan terakhir</small></div>`+
			`<div class="card"><span>Kecepatan</span><strong>%s</strong></div>`+
			`<div class="card"><span>Ketelitian</span><strong>%s</strong></div>`+
			`<div class="card"><span>Ketahanan</span><strong>%s</strong></div></div>`,
			kecNum(p.NilaiGabungan), p.AttemptsCounted, kecNum(p.Kecepatan), kecNum(p.Ketelitian), kecNum(p.Ketahanan))
	}

	b.WriteString(`<h2>Nilai per percobaan</h2>`)
	b.WriteString(kecChart(r.Attempts))
	b.WriteString(`<table><thead><tr><th>Percobaan</th><th>Tanggal</th><th class="n">Nilai akhir</th><th class="n">Kecepatan</th>` +
		`<th class="n">Ketelitian</th><th class="n">Ketahanan</th><th>Masuk nilai gabungan</th></tr></thead><tbody>`)
	for _, a := range r.Attempts {
		counted := "Tidak"
		if a.Counted {
			counted = "Ya"
		}
		fmt.Fprintf(&b, `<tr><td>%d</td><td>%s</td><td class="n">%s</td><td class="n">%s</td><td class="n">%s</td><td class="n">%s</td><td>%s</td></tr>`,
			a.Attempt.AttemptNumber, kecDate(a.Attempt.CompletedAt), kecNumPtr(a.Attempt.NilaiAkhir), kecNumPtr(a.Attempt.Kecepatan),
			kecNumPtr(a.Attempt.Ketelitian), kecNumPtr(a.Attempt.Ketahanan), counted)
	}
	b.WriteString(`</tbody></table>`)

	for _, a := range r.Attempts {
		fmt.Fprintf(&b, `<section class="attempt"><h2>Percobaan %d</h2><p class="meta">%s</p>`, a.Attempt.AttemptNumber, kecDate(a.Attempt.CompletedAt))
		b.WriteString(`<table><thead><tr><th>Kolom</th><th class="n">Benar</th><th class="n">Salah</th><th class="n">Kosong</th><th class="n">Total</th></tr></thead><tbody>`)
		var benar, salah, kosong int
		for _, k := range a.Kolom {
			kb, ks, kk := intOf(k.Benar), intOf(k.Salah), intOf(k.Kosong)
			benar, salah, kosong = benar+kb, salah+ks, kosong+kk
			fmt.Fprintf(&b, `<tr><td>%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td></tr>`, k.Number, kb, ks, kk, kb+ks+kk)
		}
		fmt.Fprintf(&b, `<tr class="sum"><td>Total</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td><td class="n">%d</td></tr></tbody></table></section>`,
			benar, salah, kosong, benar+salah+kosong)
	}
	b.WriteString(`<script>window.__mathDone=true;</script></body></html>`)
	if b.Len() > pdfMaxHTMLBytes {
		return nil, &ExportError{Code: exportCodePDFTooLarge, Err: fmt.Errorf("document is %d bytes, over the %d limit", b.Len(), pdfMaxHTMLBytes)}
	}

	footer := `<!doctype html><html><head><meta charset="utf-8"><style>` + pdfFooterCSS + `</style></head><body><div class="row"><span>` +
		esc(r.TenantName) + ` &middot; ` + pdfDate(printedAt) +
		`</span><span>Halaman <span class="pageNumber"></span> / <span class="totalPages"></span></span></div></body></html>`
	return []PDFFile{{Name: "index.html", Data: []byte(b.String())}, {Name: "footer.html", Data: []byte(footer)}}, nil
}

func kecNum(v float64) string { return strings.Replace(fmt.Sprintf("%.2f", v), ".", ",", 1) }

func kecNumPtr(v *float64) string {
	if v == nil {
		return "-"
	}
	return kecNum(*v)
}

func kecDate(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return pdfDate(*t)
}

// kecChart draws the nilai of each attempt, oldest on the left, as inline SVG with no external reference.
func kecChart(attempts []KecermatanPDFAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	const width, height, top, bottom = 520.0, 150.0, 14.0, 22.0
	ceiling := 100.0
	for _, a := range attempts {
		if a.Attempt.NilaiAkhir != nil && *a.Attempt.NilaiAkhir > ceiling {
			ceiling = *a.Attempt.NilaiAkhir
		}
	}
	slot := width / float64(len(attempts))
	barWidth := min(slot*0.6, 48)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" role="img" aria-label="Nilai akhir per percobaan">`, width, height, width, height)
	fmt.Fprintf(&b, `<line x1="0" y1="%.1f" x2="%.0f" y2="%.1f" stroke="#999" stroke-width="1"/>`, height-bottom, width, height-bottom)
	for i := range attempts {
		a := attempts[len(attempts)-1-i]
		value := 0.0
		if a.Attempt.NilaiAkhir != nil {
			value = *a.Attempt.NilaiAkhir
		}
		barHeight := (height - top - bottom) * value / ceiling
		x := slot*float64(i) + (slot-barWidth)/2
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#2563eb"/>`, x, height-bottom-barHeight, barWidth, barHeight)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="9" text-anchor="middle">%s</text>`, x+barWidth/2, height-bottom-barHeight-3, kecNum(value))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="9" text-anchor="middle">#%d</text>`, x+barWidth/2, height-8, a.Attempt.AttemptNumber)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

const kecPDFCSS = `
body{font-family:"Noto Sans","Liberation Sans",Arial,sans-serif;font-size:10.5pt;line-height:1.4;color:#111;margin:0}
h1{font-size:18pt;margin:0 0 2pt}
h2{font-size:13pt;margin:16pt 0 4pt}
.meta{margin:0 0 8pt;color:#444;font-size:10pt}
.cards{display:flex;gap:8pt;margin:4pt 0 8pt}
.card{flex:1;border:1px solid #bbb;border-radius:4pt;padding:6pt 8pt}
.card span{display:block;color:#444;font-size:9pt}
.card strong{font-size:16pt}
.card small{display:block;color:#555;font-size:8pt}
.hero{border-color:#2563eb}
table{width:100%;border-collapse:collapse;margin:4pt 0}
th,td{border-bottom:1px solid #ddd;padding:3pt 5pt;text-align:left}
th{background:#f3f4f6;font-size:9pt}
.n{text-align:right}
.sum td{font-weight:700;border-top:1px solid #888}
.attempt{break-inside:avoid}
.chart{display:block;margin:4pt 0 8pt;max-width:100%}
`
