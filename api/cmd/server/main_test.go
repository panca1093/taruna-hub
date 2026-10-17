package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// testJWTSecret is what newEcho signs with here; a real deployment gets it from JWT_SECRET.
var testJWTSecret = []byte("p03a-server-test-secret-0123456789")

// testSealKey is the 32-byte NOTIFICATION_SEAL_KEY the signup and reset emails are sealed with here.
var testSealKey = []byte("p03b-server-test-seal-key-012345")

const wiringPassword = "wiring-test-password"

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
	student := seedUser(t, ctx, pool, tenants[0].ID, model.RoleStudent)
	admin := seedUser(t, ctx, pool, tenants[0].ID, model.RoleAdmin)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}
	rdb, err := infra.NewRedisClient(ctx, redisURL)
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	notifications, err := newNotifications(config.Config{Env: "development", DatabaseURL: dsn, NotificationSealKey: testSealKey}, logger, pool)
	if err != nil {
		t.Fatalf("wire notifications: %v", err)
	}

	for _, env := range []string{"development", "staging", "production"} {
		t.Run(env, func(t *testing.T) {
			baseDomain := "tarunaeduka.id"
			if env == "development" {
				baseDomain = "localhost"
			}
			// Outside development CORS is an explicit list, so the browser origins are named here.
			origins := []string{"http://localhost:3000"}
			if env != "development" {
				origins = []string{
					originOf(tenants[0].Slug + "." + baseDomain),
					originOf(tenants[1].Slug + "." + baseDomain),
				}
			}
			e := newEcho(config.Config{
				Env:         env,
				DatabaseURL: dsn,
				CORSOrigins: origins,
				BaseDomain:  baseDomain,
				JWTSecret:   testJWTSecret,
			}, logger, pool, rdb, notifications)

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

			host := tenants[0].Slug + "." + baseDomain

			// /me is registered but never public, and no auth response may be cached.
			rec := serveOn(e, "/api/v1/me", host)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("GET /api/v1/me without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			if code := envelopeOf(t, rec).Error.Code; code != httpx.CodeUnauthorized {
				t.Errorf("GET /api/v1/me without a token: error code = %q, want %q", code, httpx.CodeUnauthorized)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET /api/v1/me: Cache-Control = %q, want no-store", got)
			}

			// Curriculum is tenant staff only: no token is a rejection, an admin reads it everywhere.
			if rec := serveOn(e, "/api/v1/programs", host); rec.Code != http.StatusUnauthorized {
				t.Errorf("GET /api/v1/programs without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			adminLogin := postJSON(e, host, "/api/v1/auth/login", map[string]string{
				"identifier": *admin.Email, "password": wiringPassword,
			})
			if adminLogin.Code != http.StatusOK {
				t.Fatalf("POST /api/v1/auth/login as admin: status = %d body = %s, want %d", adminLogin.Code, adminLogin.Body.String(), http.StatusOK)
			}
			var adminTokens struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(adminLogin.Body.Bytes(), &adminTokens); err != nil || adminTokens.AccessToken == "" {
				t.Fatalf("admin login body %s carries no access token: %v", adminLogin.Body.String(), err)
			}
			if rec := getWithToken(e, host, "/api/v1/programs", adminTokens.AccessToken); rec.Code != http.StatusOK {
				t.Errorf("GET /api/v1/programs as admin: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
			} else if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET /api/v1/programs: Cache-Control = %q, want no-store", got)
			}

			// The question bank is staff content too: closed without a token, open to an admin.
			if rec := serveOn(e, "/api/v1/questions", host); rec.Code != http.StatusUnauthorized {
				t.Errorf("GET /api/v1/questions without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			if rec := getWithToken(e, host, "/api/v1/questions", adminTokens.AccessToken); rec.Code != http.StatusOK {
				t.Errorf("GET /api/v1/questions as admin: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
			} else if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET /api/v1/questions: Cache-Control = %q, want no-store", got)
			}

			// Import is wired the same way: the upload routes are closed, the template downloads.
			if rec := serveOn(e, "/api/v1/questions/import/template?format=xlsx", host); rec.Code != http.StatusUnauthorized {
				t.Errorf("GET import template without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			if rec := getWithToken(e, host, "/api/v1/questions/import/template?format=csv", adminTokens.AccessToken); rec.Code != http.StatusOK {
				t.Errorf("GET import template as admin: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
			} else if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET import template: Cache-Control = %q, want no-store", got)
			}
			if rec := serveOn(e, "/api/v1/questions/import/preview", host); rec.Code != http.StatusUnauthorized {
				t.Errorf("POST import preview without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}
			if rec := serveOn(e, "/api/v1/questions/import/commit", host); rec.Code != http.StatusUnauthorized {
				t.Errorf("POST import commit without a token: status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusUnauthorized)
			}

			login := postJSON(e, host, "/api/v1/auth/login", map[string]string{
				"identifier": *student.Email,
				"password":   wiringPassword,
			})
			if login.Code != http.StatusOK {
				t.Fatalf("POST /api/v1/auth/login: status = %d body = %s, want %d", login.Code, login.Body.String(), http.StatusOK)
			}
			if got := login.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("POST /api/v1/auth/login: Cache-Control = %q, want no-store", got)
			}
			var tokens struct {
				AccessToken string `json:"access_token"`
				TokenType   string `json:"token_type"`
				ExpiresIn   int    `json:"expires_in"`
				User        struct {
					ID    string  `json:"id"`
					Email *string `json:"email"`
					Role  string  `json:"role"`
				} `json:"user"`
			}
			if err := json.Unmarshal(login.Body.Bytes(), &tokens); err != nil {
				t.Fatalf("login body %s is not JSON: %v", login.Body.String(), err)
			}
			if tokens.TokenType != "Bearer" || tokens.ExpiresIn <= 0 || tokens.User.ID != student.ID.String() {
				t.Errorf("login body = %s, want a bearer token for user %s with a positive expires_in", login.Body.String(), student.ID)
			}
			if c := cookieNamed(login.Result().Cookies(), "refresh_token"); c == nil || c.Value == "" {
				t.Errorf("login set no refresh cookie: %v", login.Result().Header.Values("Set-Cookie"))
			}
			if strings.Contains(login.Body.String(), "refresh_token") {
				t.Errorf("login body mentions the refresh cookie: %s", login.Body.String())
			}

			authed := getWithToken(e, host, "/api/v1/me", tokens.AccessToken)
			if authed.Code != http.StatusOK {
				t.Fatalf("GET /api/v1/me with a token: status = %d body = %s, want %d", authed.Code, authed.Body.String(), http.StatusOK)
			}
			var me struct {
				ID    string  `json:"id"`
				Name  string  `json:"name"`
				Email *string `json:"email"`
				Phone *string `json:"phone"`
				Role  string  `json:"role"`
			}
			if err := json.Unmarshal(authed.Body.Bytes(), &me); err != nil {
				t.Fatalf("me body %s is not JSON: %v", authed.Body.String(), err)
			}
			if me.ID != student.ID.String() || me.Role != string(model.RoleStudent) || me.Email == nil || *me.Email != *student.Email {
				t.Errorf("me body = %s, want the student who logged in", authed.Body.String())
			}
			if strings.Contains(authed.Body.String(), "balance") {
				t.Errorf("me body carries a balance, which this slice must not publish: %s", authed.Body.String())
			}

			logout := postWithCookie(e, host, "/api/v1/auth/logout", cookieNamed(login.Result().Cookies(), "refresh_token"), originOf(host))
			if logout.Code != http.StatusNoContent {
				t.Errorf("POST /api/v1/auth/logout: status = %d body = %s, want %d", logout.Code, logout.Body.String(), http.StatusNoContent)
			}
			if got := logout.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("POST /api/v1/auth/logout: Cache-Control = %q, want no-store", got)
			}
			if after := getWithToken(e, host, "/api/v1/me", tokens.AccessToken); after.Code != http.StatusUnauthorized {
				t.Errorf("GET /api/v1/me after logout: status = %d body = %s, want %d", after.Code, after.Body.String(), http.StatusUnauthorized)
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
		// Sessions first: they reference the users this fixture owns.
		if _, err := pool.Exec(ctx, `DELETE FROM auth_sessions WHERE tenant_id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup fixture sessions: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup fixture users: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = ANY($1)`, ids); err != nil {
			t.Errorf("cleanup fixture tenants: %v", err)
		}
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

// seedUser gives the wiring test one account whose password it knows; seedTenants cleans it up.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, role model.Role) model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(wiringPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "wiring-" + uuid.NewString()[:8] + "@example.com"
	user, err := repository.NewUserRepository(pool).Create(ctx, model.User{
		TenantID: &tenantID, Name: "Wiring Student", Email: &email,
		PasswordHash: string(hash), Role: role,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
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

func postJSON(e *echo.Echo, host, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Host = host
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func postWithCookie(e *echo.Echo, host, path string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if origin != "" {
		req.Header.Set(echo.HeaderOrigin, origin)
	}
	req.Host = host
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func getWithToken(e *echo.Echo, host, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// originOf turns a request host into the page origin a browser would send: same hostname, web port.
func originOf(host string) string {
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return "http://" + host + ":3000"
}

func envelopeOf(t *testing.T, rec *httptest.ResponseRecorder) httpx.Envelope {
	t.Helper()
	var envelope httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body %s is not the error envelope: %v", rec.Body.String(), err)
	}
	return envelope
}
