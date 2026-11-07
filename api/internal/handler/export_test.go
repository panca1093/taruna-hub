package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type exportStubStore struct{}

func (exportStubStore) Put(context.Context, string, []byte, string) error { return nil }
func (exportStubStore) Delete(context.Context, string) error              { return nil }
func (exportStubStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

type exportStubJobs struct{}

func (exportStubJobs) InsertExportTx(context.Context, pgx.Tx, service.ExportJobArgs) error {
	return nil
}

type exportStubRenderer struct{}

func (exportStubRenderer) Render(context.Context, model.Export) (service.ExportArtifact, error) {
	return service.ExportArtifact{Data: []byte("x"), ContentType: "text/csv", FileName: "hasil.csv"}, nil
}

type exportHTTPFixture struct {
	walletHTTPFixture
	e   http.Handler
	svc *service.ExportService
}

func newExportHTTPFixture(t *testing.T) exportHTTPFixture {
	t.Helper()
	f := exportHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t)}
	notes, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil, handlerSealKey, f.logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	f.svc = service.NewExportService(f.pool, repository.NewExportRepository(f.pool), exportStubStore{}, exportStubJobs{}, notes, f.logger)
	f.svc.Register(model.ExportTryoutResultsCSV, exportStubRenderer{})
	auth := newAuthService(f.tenantFixture)
	f.e = f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(f.logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(f.logger))
		h := New(f.logger, NewHealthHandler(f.pool), auth, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, AuthOptions{}).
			WithExports(f.svc)
		h.Register(e, ResolveHostScope(f.tenants, slug))
		// Stands in for a kind's own create endpoint; it has no role guard so the service rule is what answers.
		e.POST("/api/v1/test-exports", func(c echo.Context) error {
			return h.acceptExport(c, c.QueryParam("kind"), map[string]string{"tryout_id": "t1"})
		}, ResolveHostScope(f.tenants, slug), Authenticate(h.auth))
		return e
	})
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.tenantA.ID, f.tenantB.ID}); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return f
}

func (f exportHTTPFixture) call(method, path, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, method, f.hostA, path, nil, token)
}

func decodeExport(t *testing.T, rec *httptest.ResponseRecorder) exportResponse {
	t.Helper()
	var body exportResponse
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode export: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestExportRoutesRequireStaff(t *testing.T) {
	f := newExportHTTPFixture(t)
	id := uuid.NewString()
	routes := [][2]string{
		{http.MethodGet, "/api/v1/exports"},
		{http.MethodGet, "/api/v1/exports/" + id},
		{http.MethodPost, "/api/v1/exports/" + id + "/download-link"},
	}
	for _, r := range routes {
		if rec := f.call(r[0], r[1], ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", r[0], r[1], rec.Code)
		}
		if rec := f.call(r[0], r[1], f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student %s %s = %d, want 403", r[0], r[1], rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, "/api/v1/exports", f.tokenOperatorA); rec.Code != http.StatusOK {
		t.Errorf("operator list = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if rec := f.call(http.MethodPost, "/api/v1/test-exports?kind=tryout_results_csv", f.tokenA); rec.Code != http.StatusForbidden {
		t.Errorf("student create = %d, want 403", rec.Code)
	}
}

func TestAcceptExportAnswers202ThenListGetAndDownload(t *testing.T) {
	f := newExportHTTPFixture(t)
	rec := f.call(http.MethodPost, "/api/v1/test-exports?kind=tryout_results_csv", f.tokenOperatorA)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create = %d, want 202 (%s)", rec.Code, rec.Body.String())
	}
	created := decodeExport(t, rec)
	if created.Status != model.ExportQueued || created.Kind != model.ExportTryoutResultsCSV || created.FileName != nil {
		t.Fatalf("created = %+v, want a queued csv without a file", created)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("object_key")) {
		t.Error("the DTO leaks object_key")
	}
	if rec := f.call(http.MethodPost, "/api/v1/test-exports?kind=bogus", f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown kind = %d, want 422", rec.Code)
	}

	path := "/api/v1/exports/" + created.ID.String()
	if rec := f.call(http.MethodPost, path+"/download-link", f.tokenAdminA); rec.Code != http.StatusConflict ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"export_not_ready"`)) {
		t.Errorf("download of a queued export = %d %s, want 409 export_not_ready", rec.Code, rec.Body.String())
	}

	if err := f.svc.Run(context.Background(), f.tenantA.ID, created.ID, 1, service.ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	rec = f.call(http.MethodGet, path, f.tokenAdminA)
	if got := decodeExport(t, rec); rec.Code != http.StatusOK || got.Status != model.ExportReady || got.FileName == nil || *got.FileName != "hasil.csv" {
		t.Errorf("get = %d %+v, want ready", rec.Code, got)
	}
	rec = f.call(http.MethodPost, path+"/download-link", f.tokenAdminA)
	var link struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &link); rec.Code != http.StatusOK || err != nil || link.URL == "" || link.ExpiresAt.IsZero() {
		t.Errorf("download link = %d %s, want url and expires_at", rec.Code, rec.Body.String())
	}
}

func TestExportForeignOrUnknownIDIsNotFound(t *testing.T) {
	f := newExportHTTPFixture(t)
	other, err := f.svc.Request(context.Background(), service.HostScope{Tenant: f.tenantB}, model.Principal{
		UserID: f.adminB.ID, TenantID: &f.tenantB.ID, Role: model.RoleAdmin,
	}, model.ExportTryoutResultsCSV, nil)
	if err != nil {
		t.Fatalf("request in tenant B: %v", err)
	}
	for _, id := range []string{other.ID.String(), uuid.NewString(), "not-a-uuid"} {
		if rec := f.call(http.MethodGet, "/api/v1/exports/"+id, f.tokenAdminA); rec.Code != http.StatusNotFound {
			t.Errorf("get %s = %d, want 404", id, rec.Code)
		}
		if rec := f.call(http.MethodPost, "/api/v1/exports/"+id+"/download-link", f.tokenAdminA); rec.Code != http.StatusNotFound {
			t.Errorf("download %s = %d, want 404", id, rec.Code)
		}
	}
}

func TestExportListOrderingAndLimit(t *testing.T) {
	f := newExportHTTPFixture(t)
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		rec := f.call(http.MethodPost, "/api/v1/test-exports?kind=tryout_results_csv", f.tokenAdminA)
		ids = append(ids, decodeExport(t, rec).ID)
	}
	f.call(http.MethodPost, "/api/v1/test-exports?kind=tryout_pdf", f.tokenAdminA)

	var list exportListResponse
	rec := f.call(http.MethodGet, "/api/v1/exports?kind=tryout_results_csv&limit=2", f.tokenOperatorA)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	if len(list.Exports) != 2 || list.Exports[0].ID != ids[2] || list.Exports[1].ID != ids[1] {
		t.Errorf("list = %+v, want the two newest csv rows, newest first", list.Exports)
	}
	for _, bad := range []string{"0", "51", "abc", "-1"} {
		if rec := f.call(http.MethodGet, "/api/v1/exports?limit="+bad, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("limit=%s = %d, want 422", bad, rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, "/api/v1/exports?kind=bogus", f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("kind=bogus = %d, want 422", rec.Code)
	}
	rec = f.call(http.MethodGet, "/api/v1/exports", f.tokenAdminA)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Exports) != 4 {
		t.Errorf("default list has %d rows, want 4", len(list.Exports))
	}
}
