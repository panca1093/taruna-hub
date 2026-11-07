package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func newResultExportFixture(t *testing.T) runtimeHTTPFixture {
	t.Helper()
	f := newRuntimeHTTPFixture(t)
	notes, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil, handlerSealKey, f.logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	exports := service.NewExportService(f.pool, repository.NewExportRepository(f.pool), exportStubStore{}, exportStubJobs{}, notes, f.logger)
	rt := service.NewRuntimeService(f.pool, repository.NewRuntimeRepository(f.pool), service.WithRuntimeClock(f.clock.Now))
	auth := newAuthService(f.tenantFixture)
	f.e = f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(f.logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(f.logger))
		New(f.logger, NewHealthHandler(f.pool), auth, nil, nil, nil, nil, nil, nil, nil, nil, rt, newResultService(f.tenantFixture, rt), nil, nil, nil, AuthOptions{}).
			WithExports(exports).Register(e, ResolveHostScope(f.tenants, slug))
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

func TestResultExportEndpoint(t *testing.T) {
	f := newResultExportFixture(t)
	const path = "/api/v1/results/exports"

	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		for format, kind := range map[string]string{"csv": model.ExportTryoutResultsCSV, "xlsx": model.ExportTryoutResultsXLSX} {
			body := map[string]any{"format": format, "tryout_id": f.tryout.ID.String(), "program_id": f.tryout.ProgramID.String()}
			rec := f.call(http.MethodPost, path, body, token)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("%s %s = %d (%s), want 202", name, format, rec.Code, rec.Body.String())
			}
			got := decodeInto[exportResponse](t, rec)
			if got.Kind != kind || got.Status != model.ExportQueued {
				t.Errorf("%s %s export = %+v, want a queued %s", name, format, got, kind)
			}
			var params map[string]string
			if err := json.Unmarshal(got.Params, &params); err != nil || params["tryout_id"] != f.tryout.ID.String() || params["program_id"] != f.tryout.ProgramID.String() {
				t.Errorf("params = %s, want the tryout and program", got.Params)
			}
		}
	}
	if rec := f.call(http.MethodPost, path, map[string]any{"format": "csv"}, f.tokenAdminA); rec.Code != http.StatusAccepted {
		t.Errorf("unfiltered export = %d, want 202", rec.Code)
	}

	if rec := f.call(http.MethodPost, path, map[string]any{"format": "csv"}, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d, want 401", rec.Code)
	}
	if rec := f.call(http.MethodPost, path, map[string]any{"format": "csv"}, f.tokenA); rec.Code != http.StatusForbidden {
		t.Errorf("student = %d, want 403", rec.Code)
	}

	unknown := uuid.NewString()
	if rec := f.call(http.MethodPost, path, map[string]any{"format": "csv", "tryout_id": unknown}, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown tryout = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodPost, path, map[string]any{"format": "xlsx", "program_id": unknown}, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown program = %d, want 404", rec.Code)
	}
	// Tenant B's host cannot export tenant A's tryout.
	if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostB, path, map[string]any{"format": "csv", "tryout_id": f.tryout.ID.String()}, f.tokenAdmB); rec.Code != http.StatusNotFound {
		t.Errorf("foreign tenant's tryout = %d, want 404", rec.Code)
	}

	for name, body := range map[string]any{
		"bad format":     map[string]any{"format": "pdf"},
		"missing format": map[string]any{},
		"unknown field":  map[string]any{"format": "csv", "kind": "tryout_pdf"},
		"bad uuid":       map[string]any{"format": "csv", "tryout_id": "nope"},
	} {
		if rec := f.call(http.MethodPost, path, body, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d (%s), want 422", name, rec.Code, rec.Body.String())
		}
	}
	for _, query := range []string{"?program_id=nope"} {
		if rec := f.call(http.MethodGet, "/api/v1/admin/results"+query, nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("results%s = %d, want 422", query, rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, "/api/v1/admin/results?program_id="+f.tryout.ProgramID.String(), nil, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Errorf("results by program = %d, want 200", rec.Code)
	}
}
