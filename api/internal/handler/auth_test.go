package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const authTestPassword = "correct-horse-battery-staple"

// authTestSecret is long enough for HMAC-SHA256 and shared with the service under test only.
var authTestSecret = []byte("p03a-handler-test-secret-0123456789")

// newAuthService is the real service over the fixture database; nothing here is stubbed.
func newAuthService(f tenantFixture, opts ...service.AuthOption) *service.AuthService {
	return service.NewAuthService(
		repository.NewUserRepository(f.pool),
		repository.NewSessionRepository(f.pool),
		repository.NewThrottleRepository(f.pool),
		authTestSecret,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		opts...,
	)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type authFixture struct {
	tenantFixture
	logs   *syncBuffer
	logger *slog.Logger
	secure bool
}

func newAuthFixture(t *testing.T, baseDomain string, secure bool) authFixture {
	t.Helper()
	buf := &syncBuffer{}
	return authFixture{
		tenantFixture: newTenantFixture(t, baseDomain),
		logs:          buf,
		logger:        slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		secure:        secure,
	}
}

// newUser adds an active account to the fixture's first tenant; the tenant cleanup removes it.
func (f tenantFixture) newUser(t *testing.T, role model.Role, password string) model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "auth-" + uuid.NewString()[:8] + "@example.com"
	user, err := repository.NewUserRepository(f.pool).Create(context.Background(), model.User{
		TenantID: &f.active[0].ID, Name: "Auth Probe", Email: &email,
		PasswordHash: string(hash), Role: role,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

// origin is the browser page origin for a tenant host: same hostname, the web dev port.
func (f tenantFixture) origin(slug string) string {
	return "http://" + slug + "." + f.baseDomain + ":3000"
}

// echo builds the real middleware chain, so CORS, the host scope and the loggers all take part.
func (f authFixture) echo(t *testing.T, opts ...service.AuthOption) *echo.Echo {
	t.Helper()
	logger := f.logger
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	e.Use(httpx.RequestID())
	e.Use(httpx.Recover(logger))
	e.Use(httpx.RequestLogger(logger))
	e.Use(httpx.CORS(devOrigins, true))
	auth := newAuthService(f.tenantFixture, opts...)
	New(logger, NewHealthHandler(f.pool), auth, newAccountService(t, f.tenantFixture, auth), AuthOptions{
		SecureCookies: f.secure,
		AllowedOrigin: httpx.OriginPolicy(devOrigins, true),
	}).Register(e, ResolveHostScope(f.tenants, false))
	return e
}

func postJSON(e *echo.Echo, host, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return serve(e, req)
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
	return serve(e, req)
}

func getWithToken(e *echo.Echo, host, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(e, req)
}

func serve(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
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

func setCookieHeader(rec *httptest.ResponseRecorder, name string) string {
	for _, h := range rec.Result().Header.Values("Set-Cookie") {
		if strings.HasPrefix(h, name+"=") {
			return h
		}
	}
	return ""
}

func decodeTokenResponse(t *testing.T, rec *httptest.ResponseRecorder) tokenResponse {
	t.Helper()
	var body tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a token response: %v", rec.Body.String(), err)
	}
	return body
}

func credentials(identifier string) map[string]string {
	return map[string]string{"identifier": identifier, "password": authTestPassword}
}

// TestRequireRole uses a probe route that exists only here, so the guard is the only thing tested.
func TestRequireRole(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	auth := newAuthService(f.tenantFixture)
	scope := service.HostScope{Tenant: f.active[0]}
	host := f.host(f.active[0].Slug)

	student := f.newUser(t, model.RoleStudent, authTestPassword)
	admin := f.newUser(t, model.RoleAdmin, authTestPassword)
	studentToken := loginToken(t, auth, scope, *student.Email, authTestPassword)
	adminToken := loginToken(t, auth, scope, *admin.Email, authTestPassword)

	e := f.echo(t)
	probe := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	// The host-scope middleware is listed explicitly because this route is outside the v1 group.
	e.GET("/api/v1/probe/admin", probe, ResolveHostScope(f.tenants, false), Authenticate(auth), RequireRole(model.RoleAdmin))

	cases := []struct {
		name  string
		token string
		want  int
	}{
		{"no principal", "", http.StatusUnauthorized},
		{"wrong role", studentToken, http.StatusForbidden},
		{"allowed role", adminToken, http.StatusOK},
	}
	for _, tc := range cases {
		rec := getWithToken(e, host, "/api/v1/probe/admin", tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d body = %s, want %d", tc.name, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

// The refresh cookie is the one place the token reaches the browser, so its attributes are the contract.
func TestRefreshCookieFlags(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secure bool
	}{
		{"development", false},
		{"outside development", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture(t, "localhost", tc.secure)
			e := f.echo(t)
			user := f.newUser(t, model.RoleStudent, authTestPassword)
			host := f.host(f.active[0].Slug)
			origin := f.origin(f.active[0].Slug)

			login := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil)
			if login.Code != http.StatusOK {
				t.Fatalf("login: status = %d body = %s, want %d", login.Code, login.Body.String(), http.StatusOK)
			}

			refresh := cookieNamed(login.Result().Cookies(), refreshCookieName)
			if refresh == nil {
				t.Fatalf("login set no %s cookie; Set-Cookie = %v", refreshCookieName, login.Result().Header.Values("Set-Cookie"))
			}
			if refresh.Value == "" {
				t.Error("the refresh cookie carries no token")
			}
			if !refresh.HttpOnly {
				t.Error("refresh cookie is not HttpOnly")
			}
			if refresh.Path != refreshCookiePath {
				t.Errorf("refresh cookie Path = %q, want %q", refresh.Path, refreshCookiePath)
			}
			if refresh.SameSite != http.SameSiteLaxMode {
				t.Errorf("refresh cookie SameSite = %v, want Lax", refresh.SameSite)
			}
			if refresh.Secure != tc.secure {
				t.Errorf("refresh cookie Secure = %v, want %v", refresh.Secure, tc.secure)
			}
			if refresh.Domain != "" {
				t.Errorf("refresh cookie Domain = %q, want host-only (empty)", refresh.Domain)
			}
			if refresh.MaxAge != refreshCookieMaxAgeSeconds {
				t.Errorf("refresh cookie Max-Age = %d, want %d", refresh.MaxAge, refreshCookieMaxAgeSeconds)
			}

			header := setCookieHeader(login, refreshCookieName)
			if strings.Contains(strings.ToLower(header), "domain=") {
				t.Errorf("Set-Cookie names a Domain: %s", header)
			}

			body := login.Body.String()
			if strings.Contains(body, refresh.Value) {
				t.Error("the refresh token appears in the login body")
			}
			if strings.Contains(body, refreshCookieName) {
				t.Errorf("the login body mentions %s: %s", refreshCookieName, body)
			}
			var tokens tokenResponse
			if err := json.Unmarshal([]byte(body), &tokens); err != nil {
				t.Fatalf("login body %s is not JSON: %v", body, err)
			}
			if tokens.AccessToken == "" || tokens.TokenType != "Bearer" || tokens.ExpiresIn <= 0 {
				t.Errorf("login body = %s, want a bearer token with a positive expires_in", body)
			}

			// Logout clears the cookie: Go renders MaxAge -1 as Max-Age=0, which deletes it.
			logout := postWithCookie(e, host, "/api/v1/auth/logout", refresh, origin)
			if logout.Code != http.StatusNoContent {
				t.Fatalf("logout: status = %d body = %s, want %d", logout.Code, logout.Body.String(), http.StatusNoContent)
			}
			clear := setCookieHeader(logout, refreshCookieName)
			if !strings.Contains(clear, "Max-Age=0") {
				t.Errorf("logout Set-Cookie = %q, want Max-Age=0", clear)
			}
			if !strings.Contains(clear, "Path="+refreshCookiePath) {
				t.Errorf("logout Set-Cookie = %q, want Path=%s", clear, refreshCookiePath)
			}
			if cleared := cookieNamed(logout.Result().Cookies(), refreshCookieName); cleared == nil || cleared.Value != "" {
				t.Errorf("logout Set-Cookie = %q, want an empty value", clear)
			}
		})
	}
}

// Cookie-only routes need an Origin that CORS accepts and that sits on this request's host.
func TestRefreshAndLogoutRequireAcceptedOrigin(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	user := f.newUser(t, model.RoleStudent, authTestPassword)
	host := f.host(f.active[0].Slug)
	sameHostOtherPort := f.origin(f.active[0].Slug)
	sibling := f.origin(f.active[1].Slug)

	login := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login: status = %d body = %s, want %d", login.Code, login.Body.String(), http.StatusOK)
	}
	refresh := cookieNamed(login.Result().Cookies(), refreshCookieName)

	for _, tc := range []struct {
		path string
		ok   int
	}{
		{"/api/v1/auth/refresh", http.StatusOK},
		{"/api/v1/auth/logout", http.StatusNoContent},
	} {
		rejected := []struct {
			name   string
			origin string
		}{
			{"no origin header", ""},
			{"an origin CORS rejects", "http://example.com"},
			{"a sibling tenant origin CORS accepts", sibling},
		}
		for _, c := range rejected {
			rec := postWithCookie(e, host, tc.path, refresh, c.origin)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s with %s: status = %d body = %s, want %d", tc.path, c.name, rec.Code, rec.Body.String(), http.StatusForbidden)
			}
		}

		rec := postWithCookie(e, host, tc.path, refresh, sameHostOtherPort)
		if rec.Code != tc.ok {
			t.Errorf("%s with the same-hostname dev origin: status = %d body = %s, want %d", tc.path, rec.Code, rec.Body.String(), tc.ok)
		}
	}
}

// Nothing sensitive may reach a log line, an error body or an unexpected body property.
func TestLogsAndErrorsAreRedacted(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	user := f.newUser(t, model.RoleStudent, authTestPassword)
	host := f.host(f.active[0].Slug)
	origin := f.origin(f.active[0].Slug)

	wrongPassword := postJSON(e, host, "/api/v1/auth/login", map[string]string{"identifier": *user.Email, "password": "not-the-password"}, nil)
	unknown := postJSON(e, host, "/api/v1/auth/login", map[string]string{"identifier": "ghost@example.com", "password": authTestPassword}, nil)
	malformed := postJSON(e, host, "/api/v1/auth/login", map[string]string{"identifier": *user.Email}, nil)

	login := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login: status = %d body = %s, want %d", login.Code, login.Body.String(), http.StatusOK)
	}
	refresh := cookieNamed(login.Result().Cookies(), refreshCookieName)
	if refresh == nil || refresh.Value == "" {
		t.Fatal("login set no refresh cookie")
	}
	access := decodeTokenResponse(t, login).AccessToken

	// /me comes before the logout, because logout is what revokes this very session.
	me := getWithToken(e, host, "/api/v1/me", access)
	if me.Code != http.StatusOK {
		t.Fatalf("me: status = %d body = %s, want %d", me.Code, me.Body.String(), http.StatusOK)
	}
	refreshed := postWithCookie(e, host, "/api/v1/auth/refresh", refresh, origin)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d body = %s, want %d", refreshed.Code, refreshed.Body.String(), http.StatusOK)
	}
	logout := postWithCookie(e, host, "/api/v1/auth/logout", refresh, origin)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d body = %s, want %d", logout.Code, logout.Body.String(), http.StatusNoContent)
	}
	badToken := getWithToken(e, host, "/api/v1/me", "not.a.jwt")

	refreshedToken := decodeTokenResponse(t, refreshed).AccessToken
	if refreshedToken == "" {
		t.Fatal("refresh returned an empty access token")
	}

	secretHash := sha256.Sum256([]byte(refresh.Value))
	secrets := map[string]string{
		"the password":           authTestPassword,
		"the refresh token":      refresh.Value,
		"the refresh token hash": hex.EncodeToString(secretHash[:]),
		"JWT_SECRET in base64":   base64.StdEncoding.EncodeToString(authTestSecret),
		"JWT_SECRET as text":     string(authTestSecret),
		"the access token":       access,
	}
	logs := f.logs.String()
	for name, secret := range secrets {
		if secret == "" {
			t.Fatalf("%s is empty, so this check would pass vacuously", name)
		}
		if strings.Contains(logs, secret) {
			t.Errorf("the captured logs contain %s", name)
		}
	}

	// Only the token responses carry an access token, and only in access_token.
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"wrong password": wrongPassword,
		"unknown user":   unknown,
		"missing field":  malformed,
		"logout":         logout,
		"me":             me,
		"bad token":      badToken,
	} {
		body := rec.Body.String()
		if strings.Contains(body, refresh.Value) {
			t.Errorf("%s response carries the refresh token: %s", name, body)
		}
		if strings.Contains(body, authTestPassword) {
			t.Errorf("%s response carries the password: %s", name, body)
		}
		if strings.Contains(body, access) || strings.Contains(body, refreshedToken) {
			t.Errorf("%s response carries an access token: %s", name, body)
		}
	}

	if !strings.Contains(login.Body.String(), `"access_token":"`+access+`"`) {
		t.Errorf("the login body does not carry the access token in access_token: %s", login.Body.String())
	}
	if got := strings.Count(refreshed.Body.String(), refreshedToken); got != 1 {
		t.Errorf("the refresh body carries its access token %d times, want exactly once in access_token: %s", got, refreshed.Body.String())
	}
	if strings.Contains(f.logs.String(), "ghost@example.com") {
		t.Error("the captured logs contain the identifier a caller typed")
	}
}

func loginToken(t *testing.T, auth *service.AuthService, scope service.HostScope, identifier, password string) string {
	t.Helper()
	tokens, err := auth.Login(context.Background(), scope, identifier, password)
	if err != nil {
		t.Fatalf("login %s: %v", identifier, err)
	}
	return tokens.AccessToken
}

// Every failed credential attempt must look identical from outside and cost the same comparison.
func TestLoginErrorsAreIndistinguishable(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	tenantA, tenantB := f.active[0], f.active[1]

	active := f.newUser(t, model.RoleStudent, authTestPassword)
	inactive := f.newUser(t, model.RoleStudent, authTestPassword)
	if _, err := f.pool.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, inactive.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	// This account exists, but only in the other tenant.
	onlyA := f.newUser(t, model.RoleStudent, authTestPassword)

	var mu sync.Mutex
	compares := 0
	counted := func([]byte, []byte) error {
		mu.Lock()
		compares++
		mu.Unlock()
		return bcrypt.ErrMismatchedHashAndPassword
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return compares
	}

	cases := []struct {
		name       string
		tenant     model.Tenant
		identifier string
	}{
		{"an unknown identifier on an active host", tenantA, "ghost-" + uuid.NewString()[:8] + "@example.com"},
		{"the wrong password", tenantA, *active.Email},
		{"an inactive account", tenantA, *inactive.Email},
		{"an identifier that lives in another tenant", tenantB, *onlyA.Email},
	}

	reference := ""
	for _, tc := range cases {
		before := count()
		e := f.echo(t, service.WithAuthPasswordCompare(counted))
		rec := postJSON(e, f.host(tc.tenant.Slug), "/api/v1/auth/login", map[string]string{
			"identifier": tc.identifier,
			"password":   "never-the-right-password",
		}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d body = %s, want %d", tc.name, rec.Code, rec.Body.String(), http.StatusUnauthorized)
		}
		if got := count() - before; got != 1 {
			t.Errorf("%s: the password comparison ran %d times, want exactly 1", tc.name, got)
		}
		body := bodyWithoutRequestID(t, rec)
		if reference == "" {
			reference = body
			continue
		}
		if body != reference {
			t.Errorf("%s: body = %s, want the same body as every other rejection (%s)", tc.name, body, reference)
		}
	}
}

// bodyWithoutRequestID normalizes the only field a caller may see differ between two rejections.
func bodyWithoutRequestID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("body %s is not the error envelope: %v", rec.Body.String(), err)
	}
	if envelope.Error.RequestID == "" {
		t.Fatalf("body %s carries no request_id, so the comparison would not be modulo anything", rec.Body.String())
	}
	envelope.Error.RequestID = ""
	out, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("re-encode envelope: %v", err)
	}
	return string(out)
}
