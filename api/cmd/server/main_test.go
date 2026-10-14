package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// Goes through newEcho so it covers the chain the server really runs in every environment.
func TestRoutesAcrossEnvironments(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := infra.NewPostgresPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tenants := seedTenants(t, ctx, pool)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	for _, env := range []string{"development", "staging", "production"} {
		t.Run(env, func(t *testing.T) {
			baseDomain := "tarunaeduka.id"
			if env == "development" {
				baseDomain = "localhost"
			}
			e := newEcho(config.Config{
				Env:         env,
				DatabaseURL: dsn,
				CORSOrigins: []string{"http://localhost:3000"},
				BaseDomain:  baseDomain,
			}, logger, pool)

			for _, tc := range []struct {
				path string
				want int
			}{
				{"/healthz", http.StatusOK},
				{"/readyz", http.StatusOK},
			} {
				if rec := serve(e, tc.path); rec.Code != tc.want {
					t.Errorf("GET %s: status = %d, want %d", tc.path, rec.Code, tc.want)
				}
			}

			for _, tenant := range tenants[:2] {
				host := tenant.Slug + "." + baseDomain
				rec := serveOn(e, "/api/v1/tenant", host)
				if rec.Code != http.StatusOK {
					t.Errorf("GET /api/v1/tenant on %s: status = %d body = %s, want %d", host, rec.Code, rec.Body.String(), http.StatusOK)
					continue
				}
				var body struct {
					Slug string `json:"slug"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("GET /api/v1/tenant on %s: body %s is not JSON: %v", host, rec.Body.String(), err)
				}
				if body.Slug != tenant.Slug || body.Name != tenant.Name {
					t.Errorf("GET /api/v1/tenant on %s: body = %s, want slug %s name %s", host, rec.Body.String(), tenant.Slug, tenant.Name)
				}
			}

			for _, host := range []string{
				tenants[2].Slug + "." + baseDomain,
				"nope-" + tenants[0].Slug + "." + baseDomain,
				"admin." + baseDomain,
				tenants[0].Slug + "." + baseDomain + ".evil.com",
				baseDomain,
			} {
				rec := serveOn(e, "/api/v1/tenant", host)
				if rec.Code != http.StatusNotFound {
					t.Errorf("GET /api/v1/tenant on %s: status = %d body = %s, want %d", host, rec.Code, rec.Body.String(), http.StatusNotFound)
					continue
				}
				if code := envelopeOf(t, rec).Error.Code; code != httpx.CodeNotFound {
					t.Errorf("GET /api/v1/tenant on %s: error code = %q, want %q", host, code, httpx.CodeNotFound)
				}
			}

			rec := serveOn(e, "/api/v1/me", tenants[0].Slug+"."+baseDomain)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET /api/v1/me: status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if code := envelopeOf(t, rec).Error.Code; code != httpx.CodeNotFound {
				t.Errorf("GET /api/v1/me: error code = %q, want %q", code, httpx.CodeNotFound)
			}
		})
	}
}

// seedTenants returns two active tenants and one inactive, deleted again when the test ends.
func seedTenants(t *testing.T, ctx context.Context, pool *pgxpool.Pool) []model.Tenant {
	t.Helper()
	tenants := service.NewTenantService(repository.NewTenantRepository(pool), "localhost")

	var ids []uuid.UUID
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = ANY($1)`, ids)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = ANY($1)`, ids)
	})

	var out []model.Tenant
	for _, slug := range []string{"a-" + uuid.NewString()[:8], "b-" + uuid.NewString()[:8], "c-" + uuid.NewString()[:8]} {
		tenant, err := tenants.Provision(ctx, slug, "Wiring test "+slug)
		if err != nil {
			t.Fatalf("provision %s: %v", slug, err)
		}
		ids = append(ids, tenant.ID)
		out = append(out, tenant)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET is_active = false WHERE id = $1`, out[2].ID); err != nil {
		t.Fatalf("deactivate %s: %v", out[2].Slug, err)
	}
	return out
}

func serve(e *echo.Echo, path string) *httptest.ResponseRecorder {
	return serveOn(e, path, "")
}

func serveOn(e *echo.Echo, path, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if host != "" {
		req.Host = host
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func envelopeOf(t *testing.T, rec *httptest.ResponseRecorder) httpx.Envelope {
	t.Helper()
	var envelope httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body %s is not the error envelope: %v", rec.Body.String(), err)
	}
	return envelope
}
