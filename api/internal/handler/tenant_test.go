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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// The resolver must behave the same under a dev base domain and a production one.
var baseDomains = []string{"localhost", "tarunaeduka.id"}

type tenantFixture struct {
	pool       *pgxpool.Pool
	baseDomain string
	tenants    *service.TenantService
	active     [2]model.Tenant
	inactive   model.Tenant
}

func forEachBaseDomain(t *testing.T, run func(t *testing.T, f tenantFixture)) {
	t.Helper()
	for _, baseDomain := range baseDomains {
		t.Run(baseDomain, func(t *testing.T) { run(t, newTenantFixture(t, baseDomain)) })
	}
}

// Router-level only; TestRoutesAcrossEnvironments covers the real assembly.
func newTenantEcho(f tenantFixture, trustForwardedHost bool) *echo.Echo {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	New(logger, NewHealthHandler(nil), newAuthService(f), nil, AuthOptions{
		AllowedOrigin: httpx.OriginPolicy(devOrigins, true),
	}).Register(e, ResolveHostScope(f.tenants, trustForwardedHost))
	return e
}

// devOrigins is what the development CORS policy accepts; these tests never change it.
var devOrigins = []string{"http://localhost:3000"}

func newTenantFixture(t *testing.T, baseDomain string) tenantFixture {
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

	f := tenantFixture{
		pool:       pool,
		baseDomain: baseDomain,
		tenants:    service.NewTenantService(repository.NewTenantRepository(pool), baseDomain),
	}

	var ids []uuid.UUID
	t.Cleanup(func() {
		for _, table := range []string{"notification_logs", "auth_registrations", "auth_password_resets"} {
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

func (f tenantFixture) host(slug string) string { return slug + "." + f.baseDomain + ":8081" }

func getTenant(e *echo.Echo, host string, extra map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant", nil)
	req.Host = host
	for k, v := range extra {
		req.Header.Set(k, v)
	}
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

func TestTenantAnswersItsOwnTenantPerHost(t *testing.T) {
	forEachBaseDomain(t, func(t *testing.T, f tenantFixture) {
		e := newTenantEcho(f, false)

		var bodies []map[string]any
		for _, tenant := range f.active {
			rec := getTenant(e, f.host(tenant.Slug), nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("host %s: status = %d body = %s, want %d", tenant.Slug, rec.Code, rec.Body.String(), http.StatusOK)
			}
			body := tenantBody(t, rec)
			if len(body) != 2 || body["slug"] != tenant.Slug || body["name"] != tenant.Name {
				t.Errorf("host %s: body = %s, want exactly {slug: %s, name: %s}", tenant.Slug, rec.Body.String(), tenant.Slug, tenant.Name)
			}
			bodies = append(bodies, body)
		}
		if reflect.DeepEqual(bodies[0], bodies[1]) {
			t.Errorf("both hosts answered %v; the host must select the tenant", bodies[0])
		}
	})
}

func TestTenantHostsWithoutAnActiveTenantAnswerIdentical404(t *testing.T) {
	forEachBaseDomain(t, func(t *testing.T, f tenantFixture) {
		e := newTenantEcho(f, false)
		a := f.active[0]

		// The platform host is the reference: every hostless host must be indistinguishable from it.
		reference := errorEnvelope(t, getTenant(e, "admin."+f.baseDomain, nil))
		if reference.Error.Code != httpx.CodeNotFound {
			t.Fatalf("admin.%s: code = %q, want %q", f.baseDomain, reference.Error.Code, httpx.CodeNotFound)
		}

		cases := []struct{ name, host string }{
			{"unknown tenant", "nope-" + a.Slug + "." + f.baseDomain},
			{"base domain only", f.baseDomain},
			{"suffix appended", a.Slug + "." + f.baseDomain + ".evil.com"},
			{"hyphen instead of dot", a.Slug + "-" + f.baseDomain},
			{"extra label", "x." + a.Slug + "." + f.baseDomain},
			{"reserved www", "www." + f.baseDomain},
			{"reserved api", "api." + f.baseDomain},
			{"reserved admin", "admin." + f.baseDomain},
			{"reserved tryout", "tryout." + f.baseDomain},
			{"inactive tenant", f.inactive.Slug + "." + f.baseDomain},
			{"loopback", "127.0.0.1:8081"},
		}
		for _, tc := range cases {
			rec := getTenant(e, tc.host, nil)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s (%s): status = %d body = %s, want %d", tc.name, tc.host, rec.Code, rec.Body.String(), http.StatusNotFound)
				continue
			}
			got := errorEnvelope(t, rec)
			// request_id is per request, so only code and message can be compared.
			if got.Error.Code != reference.Error.Code || got.Error.Message != reference.Error.Message {
				t.Errorf("%s (%s): %s; want code %q message %q", tc.name, tc.host, rec.Body.String(), reference.Error.Code, reference.Error.Message)
			}
		}
	})
}

func TestTenantStopsServingADeactivatedTenant(t *testing.T) {
	forEachBaseDomain(t, func(t *testing.T, f tenantFixture) {
		e := newTenantEcho(f, false)
		tenant := f.active[1]
		if rec := getTenant(e, f.host(tenant.Slug), nil); rec.Code != http.StatusOK {
			t.Fatalf("host %s: status = %d body = %s, want %d", tenant.Slug, rec.Code, rec.Body.String(), http.StatusOK)
		}

		if _, err := f.pool.Exec(context.Background(), `UPDATE tenants SET is_active = false WHERE id = $1`, tenant.ID); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		if rec := getTenant(e, f.host(tenant.Slug), nil); rec.Code != http.StatusNotFound {
			t.Errorf("deactivated tenant: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusNotFound)
		}
	})
}

func TestTenantForwardedHostOnlyHonoredFromTrustedProxy(t *testing.T) {
	forEachBaseDomain(t, func(t *testing.T, f tenantFixture) {
		a, b := f.active[0], f.active[1]
		host := f.host(a.Slug)
		spoof := map[string]string{"X-Forwarded-Host": f.host(b.Slug)}

		rec := getTenant(newTenantEcho(f, false), host, spoof)
		if rec.Code != http.StatusOK || tenantBody(t, rec)["slug"] != a.Slug {
			t.Errorf("untrusted proxy: status %d body %s; want tenant %s (header ignored)", rec.Code, rec.Body.String(), a.Slug)
		}

		rec = getTenant(newTenantEcho(f, true), host, spoof)
		if body := tenantBody(t, rec); rec.Code != http.StatusOK || body["slug"] != b.Slug {
			t.Errorf("trusted proxy: status %d body %s; want tenant %s", rec.Code, rec.Body.String(), b.Slug)
		}

		chain := map[string]string{"X-Forwarded-Host": f.host(a.Slug) + ", " + f.host(b.Slug)}
		if rec := getTenant(newTenantEcho(f, true), host, chain); rec.Code != http.StatusNotFound {
			t.Errorf("forwarded chain: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusNotFound)
		}
	})
}

// /me exists now, but it is never public: no token is a rejection, not a tenant lookup.
func TestMeRequiresAuthenticationOnEveryHost(t *testing.T) {
	forEachBaseDomain(t, func(t *testing.T, f tenantFixture) {
		for _, host := range []string{f.host(f.active[0].Slug), "admin." + f.baseDomain} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			req.Host = host
			newTenantEcho(f, false).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("GET /api/v1/me on %s: status = %d body = %s, want %d", host, rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			if code := errorEnvelope(t, rec).Error.Code; code != httpx.CodeUnauthorized {
				t.Errorf("GET /api/v1/me on %s: error code = %q, want %q", host, code, httpx.CodeUnauthorized)
			}
		}
	})
}
