package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const testExportKind = model.ExportTryoutResultsCSV

type fakeRenderer struct {
	mu    sync.Mutex
	calls int
	err   error
	name  string
}

func (f *fakeRenderer) Render(context.Context, model.Export) (ExportArtifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return ExportArtifact{}, f.err
	}
	return ExportArtifact{Data: []byte("a,b\n1,2\n"), ContentType: "text/csv", FileName: f.name}, nil
}

type countingExportJobs struct {
	mu   sync.Mutex
	args []ExportJobArgs
}

func (c *countingExportJobs) InsertExportTx(_ context.Context, _ pgx.Tx, a ExportJobArgs) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.args = append(c.args, a)
	return nil
}

type exportHarness struct {
	walletHarness
	svc      *ExportService
	renderer *fakeRenderer
	store    *memStore
	jobs     *countingExportJobs
}

func newExportHarness(t *testing.T) exportHarness {
	t.Helper()
	h := exportHarness{walletHarness: newWalletHarness(t), renderer: &fakeRenderer{name: "hasil.csv"}, store: &memStore{}, jobs: &countingExportJobs{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil,
		bytes.Repeat([]byte{5}, SealKeyBytes), logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.svc = NewExportService(h.pool, repository.NewExportRepository(h.pool), h.store, h.jobs, notes, logger)
	h.svc.Register(testExportKind, h.renderer)
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{h.tenant.ID, h.other.ID}); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return h
}

func (h exportHarness) request(t *testing.T) model.Export {
	t.Helper()
	e, err := h.svc.Request(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), testExportKind, map[string]string{"tryout_id": "x"})
	if err != nil {
		t.Fatalf("request export: %v", err)
	}
	return e
}

func (h exportHarness) readyNotices(t *testing.T) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notification_logs WHERE tenant_id = $1 AND template = $2`, h.tenant.ID, TemplateExportReady).Scan(&n); err != nil {
		t.Fatalf("count notices: %v", err)
	}
	return n
}

func TestExportHappyPathReachesReadyWithOneNotice(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	e := h.request(t)
	if e.Status != model.ExportQueued || e.CutOffAt.IsZero() || len(h.jobs.args) != 1 || h.jobs.args[0].ExportID != e.ID {
		t.Fatalf("request = %+v, jobs = %+v; want a queued export with one job", e, h.jobs.args)
	}
	if err := h.svc.Run(ctx, h.tenant.ID, e.ID, 1, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := h.svc.Get(ctx, h.scope(h.tenant), h.principal(h.tenant, h.operator), e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.ExportReady || got.FileName == nil || *got.FileName != "hasil.csv" || got.SizeBytes == nil || *got.SizeBytes == 0 || got.FinishedAt == nil {
		t.Fatalf("export = %+v, want ready with a file", got)
	}
	if want := h.tenant.ID.String() + "/export/" + e.ID.String(); got.ObjectKey == nil || *got.ObjectKey != want {
		t.Errorf("object key = %v, want %s", got.ObjectKey, want)
	}
	if _, ok := h.store.objects[*got.ObjectKey]; !ok {
		t.Error("the artifact was not uploaded")
	}
	if n := h.readyNotices(t); n != 1 {
		t.Errorf("%d ready notices, want 1", n)
	}
	link, err := h.svc.DownloadLink(ctx, h.scope(h.tenant), h.principal(h.tenant, h.operator), e.ID)
	if err != nil || link.URL == "" || link.ExpiresAt.IsZero() {
		t.Errorf("download link = %+v, %v; want a signed link", link, err)
	}
}

func TestExportRendererErrorRetriesThenFails(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	h.renderer.err = &ExportError{Code: "tryout_missing", Err: errors.New("boom")}
	e := h.request(t)

	for attempt := 1; attempt < ExportMaxAttempts; attempt++ {
		if err := h.svc.Run(ctx, h.tenant.ID, e.ID, attempt, ExportMaxAttempts); err == nil {
			t.Fatalf("attempt %d returned nil, want the error so River retries", attempt)
		}
		got, _ := h.svc.Get(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID)
		if got.Status != model.ExportQueued || got.ErrorCode != "tryout_missing" || got.Attempts != attempt {
			t.Fatalf("after attempt %d: %+v, want queued with the code and the attempt counted", attempt, got)
		}
	}
	if err := h.svc.Run(ctx, h.tenant.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("final attempt = %v, want nil so River stops", err)
	}
	got, _ := h.svc.Get(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "tryout_missing" || got.FinishedAt == nil {
		t.Errorf("export = %+v, want failed with the code visible", got)
	}
	if n := h.readyNotices(t); n != 0 {
		t.Errorf("%d ready notices for a failed export, want 0", n)
	}
	if _, err := h.svc.DownloadLink(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID); !errors.Is(err, ErrExportNotReady) {
		t.Errorf("download of a failed export = %v, want ErrExportNotReady", err)
	}
}

func TestExportSecondRunOnFinishedIsNoOp(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	e := h.request(t)
	for i := 0; i < 2; i++ {
		if err := h.svc.Run(ctx, h.tenant.ID, e.ID, 1, ExportMaxAttempts); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if h.renderer.calls != 1 {
		t.Errorf("renderer called %d times, want 1", h.renderer.calls)
	}
	if n := h.readyNotices(t); n != 1 {
		t.Errorf("%d ready notices, want 1", n)
	}
	if err := h.svc.Run(ctx, h.tenant.ID, uuid.New(), 1, ExportMaxAttempts); err != nil {
		t.Errorf("run of an unknown export = %v, want a no-op", err)
	}
}

func TestExportMissingRendererFailsOnTheLastAttempt(t *testing.T) {
	h := newExportHarness(t)
	e, err := h.svc.Request(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), model.ExportTryoutPDF, nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := h.svc.Run(context.Background(), h.tenant.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ := h.svc.Get(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "no_renderer" {
		t.Errorf("export = %+v, want failed no_renderer", got)
	}
}

func TestExportRefusesStudentsAndUnknownKinds(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	e := h.request(t)
	student := h.principal(h.tenant, h.student)
	scope := h.scope(h.tenant)

	if _, err := h.svc.Request(ctx, scope, student, testExportKind, nil); !errors.Is(err, ErrExportForbidden) {
		t.Errorf("student request = %v, want forbidden", err)
	}
	if _, err := h.svc.Get(ctx, scope, student, e.ID); !errors.Is(err, ErrExportForbidden) {
		t.Errorf("student get = %v, want forbidden", err)
	}
	if _, err := h.svc.List(ctx, scope, student, "", 5); !errors.Is(err, ErrExportForbidden) {
		t.Errorf("student list = %v, want forbidden", err)
	}
	if _, err := h.svc.DownloadLink(ctx, scope, student, e.ID); !errors.Is(err, ErrExportForbidden) {
		t.Errorf("student download = %v, want forbidden", err)
	}
	wantInputField(t, func() error {
		_, err := h.svc.Request(ctx, scope, h.principal(h.tenant, h.admin), "nope", nil)
		return err
	}(), "kind")
}

func TestExportOtherTenantIsNotFound(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	e := h.request(t)
	otherAdmin := walletUser(t, repository.NewUserRepository(h.pool), h.other.ID, model.RoleAdmin, "Admin B")
	p := h.principal(h.other, otherAdmin)

	if _, err := h.svc.Get(ctx, h.scope(h.other), p, e.ID); !errors.Is(err, ErrExportNotFound) {
		t.Errorf("foreign get = %v, want not found", err)
	}
	if _, err := h.svc.DownloadLink(ctx, h.scope(h.other), p, e.ID); !errors.Is(err, ErrExportNotFound) {
		t.Errorf("foreign download = %v, want not found", err)
	}
	if _, err := h.svc.Get(ctx, h.scope(h.tenant), p, e.ID); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("a principal of another tenant under this scope = %v, want tenant not found", err)
	}
	list, err := h.svc.List(ctx, h.scope(h.other), p, "", 10)
	if err != nil || len(list) != 0 {
		t.Errorf("foreign list = %v, %v; want empty", list, err)
	}
}

func TestExportDownloadLinkNeedsReady(t *testing.T) {
	h := newExportHarness(t)
	e := h.request(t)
	if _, err := h.svc.DownloadLink(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID); !errors.Is(err, ErrExportNotReady) {
		t.Errorf("download of a queued export = %v, want ErrExportNotReady", err)
	}
}

func TestExportListIsNewestFirstAndFiltersKind(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	first, second := h.request(t), h.request(t)
	if _, err := h.svc.Request(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), model.ExportTryoutPDF, nil); err != nil {
		t.Fatalf("request pdf: %v", err)
	}
	list, err := h.svc.List(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), testExportKind, 1)
	if err != nil || len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("list = %v, %v; want only the newest csv %s", list, err, second.ID)
	}
	all, _ := h.svc.List(ctx, h.scope(h.tenant), h.principal(h.tenant, h.admin), testExportKind, 10)
	if len(all) != 2 || all[1].ID != first.ID {
		t.Errorf("list = %v, want two csv rows newest first", all)
	}
}

func TestSafeFileName(t *testing.T) {
	job := model.Export{ID: uuid.New(), Kind: testExportKind}
	cases := map[string]string{
		"hasil.csv":         "hasil.csv",
		`a/b\c"d;e.csv`:     "abcde.csv",
		"line\r\nbreak.pdf": "linebreak.pdf",
		"..":                testExportKind + "-" + job.ID.String(),
		"":                  testExportKind + "-" + job.ID.String(),
	}
	for in, want := range cases {
		if got := safeFileName(in, job); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

type cancellingRenderer struct{ cancel context.CancelFunc }

func (c cancellingRenderer) Render(ctx context.Context, _ model.Export) (ExportArtifact, error) {
	c.cancel()
	return ExportArtifact{}, ctx.Err()
}

func TestExportCancelledFinalAttemptEndsFailed(t *testing.T) {
	h := newExportHarness(t)
	e := h.request(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.svc.Register(testExportKind, cancellingRenderer{cancel: cancel})

	if err := h.svc.Run(ctx, h.tenant.ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err == nil {
		t.Fatal("cancelled run returned nil, want the context error")
	}
	got, _ := h.svc.Get(context.Background(), h.scope(h.tenant), h.principal(h.tenant, h.admin), e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != "render_timeout" || got.FinishedAt == nil {
		t.Errorf("export = %+v, want failed with render_timeout", got)
	}
}

func TestExportFailStaleLeavesFreshRowsAlone(t *testing.T) {
	h := newExportHarness(t)
	ctx := context.Background()
	old, fresh := h.request(t), h.request(t)
	for _, id := range []uuid.UUID{old.ID, fresh.ID} {
		if _, err := h.pool.Exec(ctx, `UPDATE exports SET status = 'running' WHERE id = $1`, id); err != nil {
			t.Fatalf("set running: %v", err)
		}
	}
	if _, err := h.pool.Exec(ctx, `UPDATE exports SET updated_at = now() - interval '2 hours' WHERE id = $1`, old.ID); err != nil {
		t.Fatalf("age row: %v", err)
	}

	n, err := h.svc.FailStale(ctx, time.Hour)
	if err != nil || n < 1 {
		t.Fatalf("FailStale = %d, %v; want at least the aged row", n, err)
	}
	p, s := h.principal(h.tenant, h.admin), h.scope(h.tenant)
	gotOld, _ := h.svc.Get(ctx, s, p, old.ID)
	gotFresh, _ := h.svc.Get(ctx, s, p, fresh.ID)
	if gotOld.Status != model.ExportFailed || gotOld.ErrorCode != "worker_lost" {
		t.Errorf("old = %+v, want failed worker_lost", gotOld)
	}
	if gotFresh.Status != model.ExportRunning {
		t.Errorf("fresh = %+v, want still running", gotFresh)
	}
}
