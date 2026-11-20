package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func kecReportFixture(attempts int) KecermatanPDFReport {
	score := func(v float64) *float64 { return &v }
	number := func(v int) *int { return &v }
	completed := time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC)
	report := KecermatanPDFReport{
		TenantName: "Bimbel <Taruna>",
		Student: KecStudentSummary{
			KecStudentRow: model.KecStudentRow{StudentName: `Budi <script>alert(1)</script> & "Co"`, TestTitle: "Tes <b>Cermat</b>", ProgramName: "Polri & TNI", ProgramCode: "PO<1>"},
			Progress:      &KecProgress{AttemptsCounted: 5, NilaiGabungan: 71.5, Kecepatan: 60, Ketelitian: 80.25, Ketahanan: 70},
		},
	}
	for i := attempts; i >= 1; i-- {
		at := completed.AddDate(0, 0, i)
		report.Attempts = append(report.Attempts, KecermatanPDFAttempt{
			Attempt: model.KecAttempt{AttemptNumber: i, CompletedAt: &at, NilaiAkhir: score(50 + float64(i)), Kecepatan: score(60), Ketelitian: score(80.25), Ketahanan: score(70)},
			Counted: attempts-i < kecProgressWindow,
			Kolom: []model.KecKolom{
				{Number: 1, Benar: number(30), Salah: number(10), Kosong: number(10)},
				{Number: 2, Benar: number(20), Salah: number(5), Kosong: number(25)},
			},
		})
	}
	return report
}

func TestBuildKecermatanResultPDFFilesSectionsAndEscaping(t *testing.T) {
	files, err := BuildKecermatanResultPDFFiles(kecReportFixture(6), time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(files) != 2 || files[0].Name != "index.html" || files[1].Name != "footer.html" {
		t.Fatalf("files = %v, want index.html and footer.html", files)
	}
	index, footer := string(files[0].Data), string(files[1].Data)
	for _, want := range []string{
		"Budi &lt;script&gt;alert(1)&lt;/script&gt; &amp; &#34;Co&#34;", "Tes &lt;b&gt;Cermat&lt;/b&gt;", "Polri &amp; TNI", "PO&lt;1&gt;",
		"Nilai gabungan", "71,50", "Kecepatan", "Ketelitian", "Ketahanan", "80,25",
		"Nilai per percobaan", "<svg", "Percobaan", "Tanggal", "Nilai akhir", "Masuk nilai gabungan",
		"Percobaan 6", "Percobaan 1", "<th>Kolom</th>", "<th class=\"n\">Benar</th>", "<th class=\"n\">Salah</th>", "<th class=\"n\">Kosong</th>",
		`<tr class="sum"><td>Total</td><td class="n">50</td><td class="n">15</td><td class="n">35</td><td class="n">100</td></tr>`,
		"9 November 2026", "window.__mathDone=true",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	if strings.Count(index, "<td>Ya</td>") != 5 || strings.Count(index, "<td>Tidak</td>") != 1 {
		t.Errorf("counted flags: %d yes, %d no, want 5 and 1 (only the latest five count)", strings.Count(index, "<td>Ya</td>"), strings.Count(index, "<td>Tidak</td>"))
	}
	if strings.Index(index, "Percobaan 6") > strings.Index(index, "Percobaan 1</h2>") {
		t.Error("per-attempt sections must follow the newest-first order")
	}
	if strings.Count(index, "<script") != 1 || strings.Contains(footer, "<script") {
		t.Error("the only script allowed is the fixed ready flag; a student-controlled <script> reached the document")
	}
	for _, raw := range []string{"<b>Cermat", "Bimbel <Taruna>"} {
		if strings.Contains(index, raw) || strings.Contains(footer, raw) {
			t.Errorf("unescaped %q reached the document", raw)
		}
	}
	if !strings.Contains(footer, "Bimbel &lt;Taruna&gt;") || !strings.Contains(footer, "9 November 2026") {
		t.Errorf("footer = %s, want the escaped tenant name and the Jakarta date", footer)
	}
	for _, f := range files {
		for _, scheme := range []string{"http://", "https://", "//cdn", "ftp://", "src=", "<link", "@import"} {
			if bytes.Contains(f.Data, []byte(scheme)) {
				t.Errorf("%s contains %q, the document must be self-contained", f.Name, scheme)
			}
		}
	}
}

func TestBuildKecermatanResultPDFFilesRefusesTooManyAttempts(t *testing.T) {
	var exportErr *ExportError
	if _, err := BuildKecermatanResultPDFFiles(kecReportFixture(kecPDFMaxAttempts+1), time.Now()); !errors.As(err, &exportErr) || exportErr.Code != "pdf_too_large" {
		t.Errorf("err = %v, want pdf_too_large", err)
	}
}

type kecPDFHarness struct {
	kecHarness
	export *ExportService
	jobs   *countingExportJobs
	conv   *fakeConverter
	store  *memStore
}

func newKecPDFHarness(t *testing.T) kecPDFHarness {
	t.Helper()
	h := kecPDFHarness{kecHarness: newKecHarness(t), conv: &fakeConverter{}, store: &memStore{}, jobs: &countingExportJobs{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil, bytes.Repeat([]byte{5}, SealKeyBytes), logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.export = NewExportService(h.pool, repository.NewExportRepository(h.pool), h.store, h.jobs, notes, logger)
	h.export.Register(model.ExportKecermatanResultPDF, NewKecermatanResultPDFRenderer(h.pool, h.repo, h.conv))
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{h.t.q.a.ID, h.t.q.b.ID}); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return h
}

func (h kecPDFHarness) request(t *testing.T, tenant model.Tenant, studentID, testID uuid.UUID) model.Export {
	t.Helper()
	e, err := h.export.Request(context.Background(), h.scope(tenant), h.t.principal(tenant, model.RoleAdmin), model.ExportKecermatanResultPDF,
		KecermatanResultPDFParams{StudentID: studentID, TestID: testID})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return e
}

func TestKecermatanResultPDFWorkerRendersTheStudentsReport(t *testing.T) {
	h := newKecPDFHarness(t)
	ctx := context.Background()
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.named(t, tenant, test, "Siti <Aminah>", 3)
	h.finish(t, tenant, p, test, 12)
	h.finish(t, tenant, p, test, 33)
	admin := h.t.principal(tenant, model.RoleAdmin)

	params, err := h.rt.PreparePDF(ctx, h.scope(tenant), admin, user.ID, test.ID)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	e, err := h.export.Request(ctx, h.scope(tenant), admin, model.ExportKecermatanResultPDF, params)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var stored map[string]string
	if err := json.Unmarshal(e.Params, &stored); err != nil || len(stored) != 2 || stored["student_id"] != user.ID.String() || stored["test_id"] != test.ID.String() {
		t.Errorf("queued params = %s, want exactly student_id and test_id", e.Params)
	}
	if e.Kind != model.ExportKecermatanResultPDF || e.Status != model.ExportQueued || len(h.jobs.args) != 1 {
		t.Errorf("export = %s/%s with %d jobs, want a queued kecermatan_result_pdf and one job", e.Kind, e.Status, len(h.jobs.args))
	}

	if err := h.export.Run(ctx, tenant.ID, e.ID, 1, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := h.export.Get(ctx, h.scope(tenant), h.t.principal(tenant, model.RoleOperator), e.ID)
	if err != nil || got.Status != model.ExportReady || got.ContentType == nil || *got.ContentType != "application/pdf" ||
		got.FileName == nil || !strings.HasPrefix(*got.FileName, "kecermatan-siti-aminah-") || !strings.HasSuffix(*got.FileName, ".pdf") {
		t.Fatalf("export = %+v, %v; want a ready pdf named after the student and test", got, err)
	}
	index := string(h.conv.files[0].Data)
	for _, want := range []string{"Siti &lt;Aminah&gt;", test.Title, "Percobaan 2", "Percobaan 1", "Nilai gabungan", "<svg"} {
		if !strings.Contains(index, want) {
			t.Errorf("rendered index.html lacks %q", want)
		}
	}
	if strings.Count(index, `<tr class="sum">`) != 2 || strings.Count(index, "<td>Ya</td>") != 2 {
		t.Errorf("want a total row and a counted flag per attempt, got %d and %d", strings.Count(index, `<tr class="sum">`), strings.Count(index, "<td>Ya</td>"))
	}
	for _, secret := range []string{test.Seed, `"missing"`, "generation", "answer"} {
		if secret != "" && strings.Contains(strings.ToLower(joined(h.conv.files)), strings.ToLower(secret)) {
			t.Errorf("render input contains %q", secret)
		}
	}
}

func TestKecermatanResultPDFConverterFailures(t *testing.T) {
	h := newKecPDFHarness(t)
	ctx := context.Background()
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	user, p := h.named(t, tenant, test, "Siswa PDF", 1)
	h.finish(t, tenant, p, test, 5)
	e := h.request(t, tenant, user.ID, test.ID)

	h.conv.err = errors.Join(ErrPDFUnavailable, errors.New("connection refused"))
	for attempt := 1; attempt < ExportMaxAttempts; attempt++ {
		if err := h.export.Run(ctx, tenant.ID, e.ID, attempt, ExportMaxAttempts); err == nil {
			t.Fatalf("attempt %d returned nil, want an error so the job is retried", attempt)
		}
	}
	if err := h.export.Run(ctx, tenant.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("last attempt returned %v, want nil once it is marked failed", err)
	}
	got, _ := h.export.Get(ctx, h.scope(tenant), h.t.principal(tenant, model.RoleAdmin), e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "renderer_unavailable" {
		t.Errorf("outage export = %s/%q, want failed/renderer_unavailable", got.Status, got.ErrorCode)
	}

	rejected := h.request(t, tenant, user.ID, test.ID)
	h.conv.err = errors.New("gotenberg answered 500: boom")
	if err := h.export.Run(ctx, tenant.ID, rejected.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ = h.export.Get(ctx, h.scope(tenant), h.t.principal(tenant, model.RoleAdmin), rejected.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "render_failed" {
		t.Errorf("rejected export = %s/%q, want failed/render_failed", got.Status, got.ErrorCode)
	}
}

func TestKecermatanResultPDFOtherTenantAndMissingResultFail(t *testing.T) {
	h := newKecPDFHarness(t)
	ctx := context.Background()
	tenant, other := h.t.q.a, h.t.q.b
	test := h.testIn(t, tenant)
	user, p := h.named(t, tenant, test, "Siswa Tenant A", 1)
	h.finish(t, tenant, p, test, 5)
	_, idleP := h.named(t, tenant, test, "Belum Selesai", 1)
	h.start(t, tenant, idleP, test)

	for name, c := range map[string]struct {
		tenant            model.Tenant
		studentID, testID uuid.UUID
	}{
		"another tenant's export": {other, user.ID, test.ID},
		"no completed attempt":    {tenant, idleP.UserID, test.ID},
		"unknown student":         {tenant, uuid.New(), test.ID},
	} {
		e := h.request(t, c.tenant, c.studentID, c.testID)
		if err := h.export.Run(ctx, c.tenant.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
			t.Fatalf("%s: run: %v", name, err)
		}
		got, _ := h.export.Get(ctx, h.scope(c.tenant), h.t.principal(c.tenant, model.RoleAdmin), e.ID)
		if got.Status != model.ExportFailed || got.ErrorCode != "student_result_not_found" {
			t.Errorf("%s = %s/%q, want failed/student_result_not_found", name, got.Status, got.ErrorCode)
		}
	}
	if h.conv.calls != 0 {
		t.Errorf("converter called %d times, want none for results that are not there", h.conv.calls)
	}
	if _, err := h.export.Request(ctx, h.scope(tenant), p, model.ExportKecermatanResultPDF, KecermatanResultPDFParams{StudentID: user.ID, TestID: test.ID}); !errors.Is(err, ErrExportForbidden) {
		t.Errorf("student requests the export: err = %v, want forbidden", err)
	}
}
