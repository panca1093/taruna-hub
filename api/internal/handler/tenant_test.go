package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type tenantFixture struct {
	pool        *pgxpool.Pool
	redis       *redis.Client
	tenants     *service.TenantService
	active      [2]model.Tenant
	inactive    model.Tenant
	missingSlug string
}

// tenantMux stands in for separate deployments, each configured with one tenant; the request Host only picks the deployment.
type tenantMux map[string]http.Handler

func (m tenantMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, ok := m[r.Host]
	if !ok {
		http.NotFound(w, r)
		return
	}
	h.ServeHTTP(w, r)
}

func (f tenantFixture) deployments(build func(slug string) *echo.Echo) http.Handler {
	m := tenantMux{}
	for _, slug := range []string{f.active[0].Slug, f.active[1].Slug, f.inactive.Slug, f.missingSlug} {
		m[f.host(slug)] = build(slug)
	}
	return m
}

// Router-level only; TestRoutesAcrossEnvironments covers the real assembly.
func newTenantEcho(f tenantFixture, slug string) *echo.Echo {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	New(logger, NewHealthHandler(nil), newAuthService(f), nil, newCurriculumService(f), newQuestionService(f), newTryoutService(f), newProductService(f), newWalletService(f), newAssetService(f, nil, time.Minute), AuthOptions{
		AllowedOrigin: httpx.OriginPolicy(devOrigins, true),
	}).Register(e, ResolveHostScope(f.tenants, slug))
	return e
}

// devOrigins is what the development CORS policy accepts; these tests never change it.
var devOrigins = []string{"http://localhost:3000"}

func newTenantFixture(t *testing.T) tenantFixture {
	t.Helper()
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
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	t.Cleanup(func() { _ = rdb.Close() })

	f := tenantFixture{
		pool:        pool,
		redis:       rdb,
		tenants:     service.NewTenantService(repository.NewTenantRepository(pool)),
		missingSlug: "nope-" + uuid.NewString()[:8],
	}

	var ids []uuid.UUID
	t.Cleanup(func() {
		// Curriculum, question and tryout rows carry the tenant, so they go before the tenant itself.
		for _, table := range []string{"assets", "product_tryouts", "products", "tryout_questions", "tryout_subtests", "tryouts", "question_versions", "questions", "chapters", "subjects", "programs", "notification_logs"} {
			if _, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE tenant_id = ANY($1)`, ids); err != nil {
				t.Errorf("cleanup %s of the fixture tenants: %v", table, err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM auth_sessions WHERE tenant_id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup sessions of the fixture tenants: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup users of the fixture tenants: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup fixture tenants: %v", err)
		}
	})

	slugs := []string{"a-" + uuid.NewString()[:8], "b-" + uuid.NewString()[:8], "c-" + uuid.NewString()[:8]}
	for i, slug := range slugs {
		tenant, err := f.tenants.Provision(ctx, slug, "Handler test "+slug)
		if err != nil {
			t.Fatalf("provision %s: %v", slug, err)
		}
		ids = append(ids, tenant.ID)
		switch i {
		case 0, 1:
			f.active[i] = tenant
		default:
			if _, err := pool.Exec(ctx, `UPDATE tenants SET is_active = false WHERE id = $1`, tenant.ID); err != nil {
				t.Fatalf("deactivate %s: %v", slug, err)
			}
			tenant.IsActive = false
			f.inactive = tenant
		}
	}
	return f
}

func (f tenantFixture) host(slug string) string { return slug + ".test" }

func getTenant(e http.Handler, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant", nil)
	req.Host = host
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func tenantBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return body
}

func errorEnvelope(t *testing.T, rec *httptest.ResponseRecorder) httpx.Envelope {
	t.Helper()
	var envelope httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return envelope
}

func TestTenantAnswersTheConfiguredTenant(t *testing.T) {
	f := newTenantFixture(t)
	e := f.deployments(func(slug string) *echo.Echo { return newTenantEcho(f, slug) })

	var bodies []map[string]any
	for _, tenant := range f.active {
		rec := getTenant(e, f.host(tenant.Slug))
		if rec.Code != http.StatusOK {
			t.Fatalf("tenant %s: status = %d body = %s, want %d", tenant.Slug, rec.Code, rec.Body.String(), http.StatusOK)
		}
		body := tenantBody(t, rec)
		if len(body) != 2 || body["slug"] != tenant.Slug || body["name"] != tenant.Name {
			t.Errorf("tenant %s: body = %s, want exactly {slug: %s, name: %s}", tenant.Slug, rec.Body.String(), tenant.Slug, tenant.Name)
		}
		bodies = append(bodies, body)
	}
	if reflect.DeepEqual(bodies[0], bodies[1]) {
		t.Errorf("both deployments answered %v; the configured slug must select the tenant", bodies[0])
	}
}

func TestMissingOrInactiveConfiguredTenantFailsClosed(t *testing.T) {
	f := newTenantFixture(t)
	e := f.deployments(func(slug string) *echo.Echo { return newTenantEcho(f, slug) })

	var reference httpx.Envelope
	for i, slug := range []string{f.missingSlug, f.inactive.Slug} {
		for _, path := range []string{"/api/v1/tenant", "/api/v1/me", "/api/v1/catalog/products"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Host = f.host(slug)
			rec := serve(e, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s on %s: status = %d body = %s, want %d", path, slug, rec.Code, rec.Body.String(), http.StatusNotFound)
				continue
			}
			got := errorEnvelope(t, rec)
			if got.Error.Code != httpx.CodeNotFound {
				t.Errorf("%s on %s: code = %q, want %q", path, slug, got.Error.Code, httpx.CodeNotFound)
			}
			if i == 0 && path == "/api/v1/tenant" {
				reference = got
			}
			// request_id is per request, so only code and message can be compared.
			if got.Error.Code != reference.Error.Code || got.Error.Message != reference.Error.Message {
				t.Errorf("%s on %s: %s; a missing and an inactive tenant must answer alike", path, slug, rec.Body.String())
			}
		}
	}
}

func TestTenantStopsServingADeactivatedTenant(t *testing.T) {
	f := newTenantFixture(t)
	e := f.deployments(func(slug string) *echo.Echo { return newTenantEcho(f, slug) })
	tenant := f.active[1]
	if rec := getTenant(e, f.host(tenant.Slug)); rec.Code != http.StatusOK {
		t.Fatalf("tenant %s: status = %d body = %s, want %d", tenant.Slug, rec.Code, rec.Body.String(), http.StatusOK)
	}

	if _, err := f.pool.Exec(context.Background(), `UPDATE tenants SET is_active = false WHERE id = $1`, tenant.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if rec := getTenant(e, f.host(tenant.Slug)); rec.Code != http.StatusNotFound {
		t.Errorf("deactivated tenant: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusNotFound)
	}
}

// /me exists now, but it is never public: no token is a rejection, not a tenant lookup.
func TestMeRequiresAuthentication(t *testing.T) {
	f := newTenantFixture(t)
	e := newTenantEcho(f, f.active[0].Slug)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/v1/me: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
	}
	if code := errorEnvelope(t, rec).Error.Code; code != httpx.CodeUnauthorized {
		t.Errorf("GET /api/v1/me: error code = %q, want %q", code, httpx.CodeUnauthorized)
	}
}
