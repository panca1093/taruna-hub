package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const (
	tinyPNG        = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	noteMarker     = "PEMBAHASAN-RAHASIA-88"
	pointsMarker   = 77.25
	evilImageURL   = "https://evil.example/leak.png"
	pdfTestTitle   = "Tryout Matematika & Fisika"
	pdfTestTenant  = "Bimbel <Taruna>"
	pdfTestSubtest = "Subtes Penalaran"
)

type fakeConverter struct {
	mu    sync.Mutex
	calls int
	err   error
	files []PDFFile
}

func (f *fakeConverter) HTMLToPDF(_ context.Context, files []PDFFile) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.files = files
	if f.err != nil {
		return nil, f.err
	}
	return []byte("%PDF-1.7 fake"), nil
}

func printFixture() model.PrintTryout {
	return model.PrintTryout{
		TenantName: pdfTestTenant, Title: pdfTestTitle, Revision: 3,
		Subtests: []model.PrintSubtest{
			{Name: pdfTestSubtest, DurationSeconds: 5400, Questions: []model.PrintQuestion{
				{Content: "Soal Z pertama $x^2 + 1$\n![gambar](" + tinyPNG + ")\n![luar](" + evilImageURL + ")", Options: []string{"Zeta", "Alfa", "Mu"}},
				{Content: "Soal A kedua <script>alert(1)</script>", Options: nil},
			}},
			{Name: "Subtes Kedua", DurationSeconds: 1800, Questions: []model.PrintQuestion{
				{Content: "Soal ketiga", Options: []string{"Benar", "Salah"}},
			}},
		},
	}
}

func joined(files []PDFFile) string {
	var b strings.Builder
	for _, f := range files {
		b.Write(f.Data)
	}
	return b.String()
}

func TestBuildTryoutPDFFilesKeepsStoredOrderAndNumbering(t *testing.T) {
	files, err := BuildTryoutPDFFiles(printFixture(), time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	index := string(files[0].Data)
	order := []string{pdfTestTitle[:15], pdfTestSubtest, "Soal Z pertama", "Soal A kedua", "Subtes Kedua", "Soal ketiga"}
	last := -1
	for _, want := range order {
		at := strings.Index(index, want)
		if at < 0 || at < last {
			t.Fatalf("%q at %d after %d; subtests, questions must keep stored order", want, at, last)
		}
		last = at
	}
	if z, a, m := strings.Index(index, "Zeta"), strings.Index(index, "Alfa"), strings.Index(index, "Mu<"); !(z < a && a < m) {
		t.Errorf("options at %d,%d,%d, want stored order Zeta, Alfa, Mu", z, a, m)
	}
	for _, want := range []string{"Durasi 1 jam 30 menit", "Durasi 30 menit", "2 soal", `<div class="no">1.</div>`, `<div class="no">2.</div>`,
		"A.", "C.", `data-tex="x^2 + 1"`, `src="` + tinyPNG + `"`, "&lt;script&gt;alert(1)&lt;/script&gt;", "Tryout Matematika &amp; Fisika"} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if strings.Count(index, `<div class="no">1.</div>`) != 2 {
		t.Error("numbering must restart at 1 in each subtest")
	}
	if strings.Contains(index, "<script>alert") || strings.Contains(index, evilImageURL) || strings.Contains(index, "evil.example") {
		t.Error("content reached the page unescaped or an external image URL survived")
	}
	footer := string(files[1].Data)
	if !strings.Contains(footer, "Bimbel &lt;Taruna&gt;") || !strings.Contains(footer, "9 November 2026") {
		t.Errorf("footer = %s, want escaped tenant name and the Jakarta date", footer)
	}
}

func TestBuildTryoutPDFFilesHasNoNetworkURL(t *testing.T) {
	files, err := BuildTryoutPDFFiles(printFixture(), time.Now())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	names := []string{}
	for _, f := range files {
		names = append(names, f.Name)
		for _, scheme := range []string{"http://", "https://", "//cdn", "ftp://"} {
			if bytes.Contains(f.Data, []byte(scheme)) {
				t.Errorf("%s contains %q", f.Name, scheme)
			}
		}
	}
	if strings.Join(names, ",") != "index.html,footer.html,katex.min.css,katex.min.js" {
		t.Errorf("files = %v, want exactly index, footer and the embedded KaTeX", names)
	}
	if !bytes.Contains(files[2].Data, []byte("data:font/woff2;base64,")) || bytes.Contains(files[2].Data, []byte("url(fonts/")) {
		t.Error("katex.min.css must carry its fonts inline")
	}
}

func TestBuildTryoutPDFFilesRefusesTooManyQuestions(t *testing.T) {
	big := model.PrintTryout{Title: "x", Subtests: []model.PrintSubtest{{Name: "s", DurationSeconds: 60, Questions: make([]model.PrintQuestion, pdfMaxQuestions+1)}}}
	var exportErr *ExportError
	if _, err := BuildTryoutPDFFiles(big, time.Now()); !errors.As(err, &exportErr) || exportErr.Code != "pdf_too_large" {
		t.Errorf("err = %v, want pdf_too_large", err)
	}
}

type pdfHarness struct {
	tryoutHarness
	export *ExportService
	conv   *fakeConverter
	store  *memStore
}

func newPDFHarness(t *testing.T) pdfHarness {
	t.Helper()
	h := pdfHarness{tryoutHarness: newTryoutHarness(t), conv: &fakeConverter{}, store: &memStore{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil, bytes.Repeat([]byte{5}, SealKeyBytes), logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.export = NewExportService(h.pool, repository.NewExportRepository(h.pool), h.store, &countingExportJobs{}, notes, logger)
	h.export.Register(model.ExportTryoutPDF, NewTryoutPDFRenderer(repository.NewTryoutRepository(h.pool), h.conv))
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{h.q.a.ID, h.q.b.ID}); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return h
}

// keyedQuestion carries a distinctive explanation, option points and (unweighted) a correct option id, so a leak is
// a plain substring check. The returned ids are every identifier the key could travel as.
func (h pdfHarness) keyedQuestion(t *testing.T, weighted bool, content string, labels ...string) (model.Question, []string) {
	t.Helper()
	input := h.q.pgInput(h.q.a)
	input.Content = content
	input.Explanation = noteMarker
	input.OptionWeighted = weighted
	input.Options, input.Correct = nil, nil
	for i, label := range labels {
		option := QuestionOptionInput{Label: label}
		if weighted {
			points := 1.0
			if i == 1 {
				points = pointsMarker
			}
			option.Points = &points
		}
		input.Options = append(input.Options, option)
	}
	if !weighted {
		input.Options[0].ID = "keyed"
		input.Correct = []string{"keyed"}
	}
	q := h.q.create(t, h.q.a, input)
	ids := []string{}
	for _, o := range q.Options {
		ids = append(ids, o.ID)
	}
	return q, append(ids, q.Correct...)
}

func (h pdfHarness) request(t *testing.T, tenant model.Tenant, tryoutID uuid.UUID, revision int) model.Export {
	t.Helper()
	e, err := h.export.Request(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), model.ExportTryoutPDF,
		TryoutPDFParams{TryoutID: tryoutID, Revision: revision})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return e
}

func (h pdfHarness) shuffledTryout(t *testing.T) (model.Tryout, []string) {
	t.Helper()
	q1, ids1 := h.keyedQuestion(t, false, "Soal Zulu $y$", "Zeta", "Alfa", "Mu")
	q2, ids2 := h.keyedQuestion(t, true, "Soal Alfa", "Satu", "Dua")
	q3, ids3 := h.keyedQuestion(t, false, "Soal Mike", "Tiga", "Empat")
	input := h.tryoutInput(h.q.a, subtestInput("Subtes Zulu", 3600, q2, q1), subtestInput("Subtes Alfa", 600, q3))
	input.RandomizeQuestions, input.RandomizeOptions = true, true
	return h.create(t, h.q.a, input), append(append(ids1, ids2...), ids3...)
}

func TestTryoutPDFWorkerRendersStoredOrderWithoutKeys(t *testing.T) {
	h := newPDFHarness(t)
	ctx := context.Background()
	tryout, keyIDs := h.shuffledTryout(t)
	e := h.request(t, h.q.a, tryout.ID, tryout.Revision)

	if err := h.export.Run(ctx, h.q.a.ID, e.ID, 1, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := h.export.Get(ctx, h.scope(h.q.a), h.principal(h.q.a, model.RoleOperator), e.ID)
	if err != nil || got.Status != model.ExportReady || got.ContentType == nil || *got.ContentType != "application/pdf" ||
		got.FileName == nil || !strings.HasSuffix(*got.FileName, "-r1.pdf") {
		t.Fatalf("export = %+v, %v; want a ready pdf named <slug>-r1.pdf", got, err)
	}
	if _, ok := h.store.objects[*got.ObjectKey]; !ok {
		t.Error("the artifact was not stored")
	}

	all, index := joined(h.conv.files), string(h.conv.files[0].Data)
	for _, secret := range append([]string{noteMarker, "77.25", "keyed"}, keyIDs...) {
		if strings.Contains(all, secret) {
			t.Errorf("render input contains %q", secret)
		}
	}
	if strings.Contains(all, "http://") || strings.Contains(all, "https://") {
		t.Error("render input contains a URL")
	}
	last := -1
	for _, want := range []string{"Subtes Zulu", "Soal Alfa", "Soal Zulu", "Subtes Alfa", "Soal Mike"} {
		at := strings.Index(index, want)
		if at < 0 || at < last {
			t.Fatalf("%q at %d after %d: shuffle flags must not change the printed order", want, at, last)
		}
		last = at
	}
	if z, a := strings.Index(index, "Zeta"), strings.Index(index, "Alfa</span>"); z < 0 || a < z {
		t.Errorf("option order broken: Zeta at %d, Alfa at %d", z, a)
	}

	if err := h.export.Run(ctx, h.q.a.ID, e.ID, 1, ExportMaxAttempts); err != nil || h.conv.calls != 1 {
		t.Errorf("second run: err = %v, converter calls = %d; a ready export must be a no-op", err, h.conv.calls)
	}
}

func TestTryoutPDFConverterFailureRetriesThenFailsVisibly(t *testing.T) {
	h := newPDFHarness(t)
	ctx := context.Background()
	tryout, _ := h.shuffledTryout(t)
	e := h.request(t, h.q.a, tryout.ID, tryout.Revision)

	h.conv.err = errors.New("gotenberg answered 500: boom")
	for attempt := 1; attempt < ExportMaxAttempts; attempt++ {
		if err := h.export.Run(ctx, h.q.a.ID, e.ID, attempt, ExportMaxAttempts); err == nil {
			t.Fatalf("attempt %d returned nil, want an error so the job is retried", attempt)
		}
	}
	h.conv.err = errors.Join(ErrPDFUnavailable, errors.New("connection refused"))
	if err := h.export.Run(ctx, h.q.a.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("last attempt returned %v, want nil once it is marked failed", err)
	}
	got, _ := h.export.Get(ctx, h.scope(h.q.a), h.principal(h.q.a, model.RoleAdmin), e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "renderer_unavailable" {
		t.Errorf("export = %s/%q, want failed/renderer_unavailable", got.Status, got.ErrorCode)
	}
}

func TestTryoutPDFOtherTenantTryoutAndStaleRevisionFail(t *testing.T) {
	h := newPDFHarness(t)
	ctx := context.Background()
	tryout, _ := h.shuffledTryout(t)

	foreign := h.request(t, h.q.b, tryout.ID, tryout.Revision)
	if err := h.export.Run(ctx, h.q.b.ID, foreign.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ := h.export.Get(ctx, h.scope(h.q.b), h.principal(h.q.b, model.RoleAdmin), foreign.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "tryout_not_found" || h.conv.calls != 0 {
		t.Errorf("foreign export = %s/%q with %d converter calls, want failed/tryout_not_found and no render", got.Status, got.ErrorCode, h.conv.calls)
	}

	stale := h.request(t, h.q.a, tryout.ID, tryout.Revision+1)
	if err := h.export.Run(ctx, h.q.a.ID, stale.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ = h.export.Get(ctx, h.scope(h.q.a), h.principal(h.q.a, model.RoleAdmin), stale.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "tryout_changed" {
		t.Errorf("stale export = %s/%q, want failed/tryout_changed", got.Status, got.ErrorCode)
	}
}
