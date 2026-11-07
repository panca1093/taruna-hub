package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func (f authFixture) pdfEcho(t *testing.T, configured bool) http.Handler {
	t.Helper()
	notes, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil, handlerSealKey, f.logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	exports := service.NewExportService(f.pool, repository.NewExportRepository(f.pool), exportStubStore{}, exportStubJobs{}, notes, f.logger)
	auth := newAuthService(f.tenantFixture)
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.active[0].ID, f.active[1].ID}); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(f.logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(f.logger))
		New(f.logger, NewHealthHandler(f.pool), auth, nil, nil, nil, newTryoutService(f.tenantFixture), nil, nil, nil, nil, nil, nil, nil, nil, nil, AuthOptions{}).
			WithExports(exports).WithPDF(configured).Register(e, ResolveHostScope(f.tenants, slug))
		return e
	})
}

func TestTryoutPDFRequest(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.pdfEcho(t, true)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA := f.host(tenantA.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	operatorA := curriculumToken(t, f, tenantA, model.RoleOperator)
	studentA := curriculumToken(t, f, tenantA, model.RoleStudent)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	content := newAuthFixtureContent(t, f, hostA, adminA)
	path := "/api/v1/tryouts/" + content.ID.String() + "/pdf-exports"

	for name, token := range map[string]string{"admin": adminA, "operator": operatorA} {
		rec := sendTryoutJSON(e, http.MethodPost, hostA, path, nil, token)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("%s request = %d %s, want 202", name, rec.Code, rec.Body.String())
		}
		var body exportResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var params service.TryoutPDFParams
		if err := json.Unmarshal(body.Params, &params); err != nil ||
			body.Kind != model.ExportTryoutPDF || body.Status != model.ExportQueued ||
			params.TryoutID != content.ID || params.Revision != content.Revision {
			t.Errorf("%s export = %+v params %+v, want a queued tryout_pdf for revision %d", name, body, params, content.Revision)
		}
	}
	if rec := sendTryoutJSON(e, http.MethodPost, hostA, path, nil, studentA); rec.Code != http.StatusForbidden {
		t.Errorf("student = %d, want 403", rec.Code)
	}
	if rec := sendTryoutJSON(e, http.MethodPost, hostA, path, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d, want 401", rec.Code)
	}
	hostB := f.host(tenantB.Slug)
	for _, id := range []string{content.ID.String(), uuid.NewString(), "not-a-uuid"} {
		if rec := sendTryoutJSON(e, http.MethodPost, hostB, "/api/v1/tryouts/"+id+"/pdf-exports", nil, adminB); rec.Code != http.StatusNotFound {
			t.Errorf("tenant B on %s = %d, want 404", id, rec.Code)
		}
	}
}

func TestTryoutPDFUnconfiguredIs503(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.pdfEcho(t, false)
	tenantA := f.active[0]
	hostA := f.host(tenantA.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	content := newAuthFixtureContent(t, f, hostA, adminA)

	rec := sendTryoutJSON(e, http.MethodPost, hostA, "/api/v1/tryouts/"+content.ID.String()+"/pdf-exports", nil, adminA)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"pdf_unavailable"`) {
		t.Errorf("unconfigured = %d %s, want 503 pdf_unavailable", rec.Code, rec.Body.String())
	}
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM exports WHERE tenant_id = $1`, tenantA.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d exports enqueued (%v), want none", n, err)
	}
}

// newAuthFixtureContent creates one tryout with one question through the real routes.
func newAuthFixtureContent(t *testing.T, f authFixture, host, adminToken string) tryoutResponse {
	t.Helper()
	plain := f.echo(t)
	program := questionCurriculum(t, plain, host, adminToken)
	question := createTryoutQuestion(t, plain, host, adminToken, program)
	rec := sendTryoutJSON(plain, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), adminToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tryout: %d %s", rec.Code, rec.Body.String())
	}
	return decodeTryout(t, rec)
}
