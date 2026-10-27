package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

var handlerSealKey = bytes.Repeat([]byte{9}, service.SealKeyBytes)

// noJobs stands in for River; the notification row is what these tests read the code and link from.
type noJobs struct{}

func (noJobs) InsertNotificationTx(context.Context, pgx.Tx, service.NotificationJobArgs) error {
	return nil
}

func newAccountService(t *testing.T, f tenantFixture, auth *service.AuthService) *service.AccountService {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifications, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil, handlerSealKey, logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	return service.NewAccountService(service.AccountDeps{
		DB:            f.pool,
		Users:         repository.NewUserRepository(f.pool),
		Sessions:      repository.NewSessionRepository(f.pool),
		Throttles:     repository.NewThrottleRepository(f.redis),
		Notifications: notifications,
		Auth:          auth,
		Links:         service.WebLinks{BaseURL: devOrigins[0]},
		Secret:        authTestSecret,
		Logger:        logger,
	}, service.WithAccountPasswordCost(bcrypt.MinCost))
}

// secretParam opens the newest notification of a template, the way the worker would.
func (f tenantFixture) secretParam(t *testing.T, recipient, template, key string) string {
	t.Helper()
	var id uuid.UUID
	var sealed []byte
	if err := f.pool.QueryRow(context.Background(),
		`SELECT id, sealed_params FROM notification_logs WHERE recipient = $1 AND template = $2
		  ORDER BY created_at DESC, id LIMIT 1`, recipient, template).Scan(&id, &sealed); err != nil {
		t.Fatalf("read %s for %s: %v", template, recipient, err)
	}
	sealer, err := service.NewSealer(handlerSealKey)
	if err != nil {
		t.Fatalf("sealer: %v", err)
	}
	plain, err := sealer.Open(sealed, id[:])
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var secret map[string]string
	if err := json.Unmarshal(plain, &secret); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return secret[key]
}

func wantError(t *testing.T, name string, rec *httptest.ResponseRecorder, status int, code httpx.Code, fields ...string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("%s: status = %d body = %s, want %d", name, rec.Code, rec.Body.String(), status)
		return
	}
	env := errorEnvelope(t, rec)
	if env.Error.Code != code {
		t.Errorf("%s: code = %s, want %s", name, env.Error.Code, code)
	}
	for _, field := range fields {
		if env.Error.Fields[field] == "" {
			t.Errorf("%s: no message for field %s in %v", name, field, env.Error.Fields)
		}
	}
}

// D01 over HTTP: signup answers like a login, and the email is verified later by the signed-in user.
func TestSignUpOverHTTP(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	host := f.host(f.active[0].Slug)
	email := "signup-" + uuid.NewString()[:8] + "@example.com"
	signup := map[string]string{"name": "Budi", "email": strings.ToUpper(email), "phone": "0812-" + strconv.Itoa(10000000+int(uuid.New().ID()%89999999)), "password": authTestPassword}

	rec := postJSON(e, host, "/api/v1/auth/register", signup, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("register Cache-Control = %q, want no-store", got)
	}
	if c := cookieNamed(rec.Result().Cookies(), refreshCookieName); c == nil || c.Value == "" || !c.HttpOnly {
		t.Error("register set no HttpOnly refresh cookie")
	}
	tokens := decodeTokenResponse(t, rec)
	if tokens.User.Role != model.RoleStudent || tokens.User.Email == nil || *tokens.User.Email != email || tokens.User.EmailVerified {
		t.Errorf("registered user = %+v, want an unverified student with %s", tokens.User, email)
	}
	bearer := map[string]string{echo.HeaderAuthorization: "Bearer " + tokens.AccessToken}
	code := f.secretParam(t, email, service.TemplateEmailCode, "code")

	// Unverified is signed in: /me works and says so, which is what the web app's alert reads.
	me := getWithToken(e, host, "/api/v1/me", tokens.AccessToken)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"email_verified":false`) {
		t.Errorf("me before verifying: status = %d body = %s, want 200 and email_verified false", me.Code, me.Body.String())
	}

	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	wantError(t, "wrong code", postJSON(e, host, "/api/v1/me/email/verify", map[string]string{"code": wrong}, bearer),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "code")
	wantError(t, "no token", postJSON(e, host, "/api/v1/me/email/verify", map[string]string{"code": code}, nil),
		http.StatusUnauthorized, httpx.CodeUnauthorized)
	wantError(t, "resend inside the cooldown", postJSON(e, host, "/api/v1/me/email/resend", nil, bearer),
		http.StatusTooManyRequests, httpx.CodeRateLimited)

	if rec := postJSON(e, host, "/api/v1/me/email/verify", map[string]string{"code": code}, bearer); rec.Code != http.StatusNoContent {
		t.Fatalf("verify: status = %d body = %s, want 204", rec.Code, rec.Body.String())
	}
	if me := getWithToken(e, host, "/api/v1/me", tokens.AccessToken); !strings.Contains(me.Body.String(), `"email_verified":true`) {
		t.Errorf("me after verifying = %s, want email_verified true", me.Body.String())
	}
	wantError(t, "resend after verifying", postJSON(e, host, "/api/v1/me/email/resend", nil, bearer), http.StatusConflict, httpx.CodeConflict)

	wantError(t, "email taken", postJSON(e, host, "/api/v1/auth/register", signup, nil), http.StatusConflict, httpx.CodeConflict, "email")
	wantError(t, "missing fields", postJSON(e, host, "/api/v1/auth/register", map[string]string{"phone": "0812"}, nil),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "name", "email", "password")
	wantError(t, "bad formats", postJSON(e, host, "/api/v1/auth/register",
		map[string]string{"name": "X", "email": "nope", "phone": "12", "password": "short"}, nil),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "email", "phone", "password")
	wantError(t, "unknown field", postJSON(e, host, "/api/v1/auth/register",
		map[string]string{"name": "X", "email": email, "password": authTestPassword, "role": "admin"}, nil),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "body")

	if strings.Contains(f.logs.String(), code) {
		t.Error("the verification code reached a log")
	}
	if strings.Contains(rec.Body.String(), code) {
		t.Error("the verification code reached a response")
	}
}

func TestPasswordResetOverHTTP(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	slug := f.active[0].Slug
	host := f.host(slug)
	user := f.newUser(t, model.RoleStudent, authTestPassword)

	known := postJSON(e, host, "/api/v1/auth/password/forgot", map[string]string{"email": *user.Email}, nil)
	unknown := postJSON(e, host, "/api/v1/auth/password/forgot", map[string]string{"email": "ghost-" + uuid.NewString()[:8] + "@example.com"}, nil)
	for name, rec := range map[string]*httptest.ResponseRecorder{"known": known, "unknown": unknown} {
		if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
			t.Errorf("forgot for a %s email: status = %d body = %q, want 202 and no body", name, rec.Code, rec.Body.String())
		}
	}

	link := f.secretParam(t, *user.Email, service.TemplatePasswordReset, "reset_url")
	prefix := devOrigins[0] + "/reset-password#token="
	if !strings.HasPrefix(link, prefix) {
		t.Fatalf("reset link %q, want it under %q", link, prefix)
	}
	token := strings.TrimPrefix(link, prefix)

	wantError(t, "bad token", postJSON(e, host, "/api/v1/auth/password/reset", map[string]string{"token": "nope", "password": "brand-new-horse"}, nil),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "token")
	wantError(t, "short password", postJSON(e, host, "/api/v1/auth/password/reset", map[string]string{"token": token, "password": "short"}, nil),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "password")

	if rec := postJSON(e, host, "/api/v1/auth/password/reset", map[string]string{"token": token, "password": "brand-new-horse"}, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: status = %d body = %s, want 204", rec.Code, rec.Body.String())
	}
	if rec := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("the old password: status = %d, want 401", rec.Code)
	}
	if rec := postJSON(e, host, "/api/v1/auth/login", map[string]string{"identifier": *user.Email, "password": "brand-new-horse"}, nil); rec.Code != http.StatusOK {
		t.Errorf("the new password: status = %d, want 200", rec.Code)
	}
	if strings.Contains(f.logs.String(), token) {
		t.Error("the reset token reached a log")
	}
}

func TestThrottledLoginAnswers429WithRetryAfter(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	host := f.host(f.active[0].Slug)
	user := f.newUser(t, model.RoleStudent, authTestPassword)

	wrong := map[string]string{"identifier": *user.Email, "password": "not-the-password"}
	for i := range 10 {
		if rec := postJSON(e, host, "/api/v1/auth/login", wrong, nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong attempt %d: status = %d, want 401", i+1, rec.Code)
		}
	}
	rec := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil)
	wantError(t, "past the limit", rec, http.StatusTooManyRequests, httpx.CodeRateLimited)
	seconds, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || seconds < 1 || seconds > 15*60 {
		t.Errorf("Retry-After = %q, want whole seconds within the 15-minute window", rec.Header().Get("Retry-After"))
	}
}

// The browser sees rotation as a new cookie; a racing tab inside the grace window keeps the one it has.
func TestRefreshRotatesTheCookie(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	slug := f.active[0].Slug
	host, origin := f.host(slug), f.origin(slug)
	user := f.newUser(t, model.RoleStudent, authTestPassword)

	login := postJSON(e, host, "/api/v1/auth/login", credentials(*user.Email), nil)
	first := cookieNamed(login.Result().Cookies(), refreshCookieName)
	if first == nil {
		t.Fatal("login set no refresh cookie")
	}

	rotated := postWithCookie(e, host, "/api/v1/auth/refresh", first, origin)
	if rotated.Code != http.StatusOK {
		t.Fatalf("refresh: status = %d body = %s", rotated.Code, rotated.Body.String())
	}
	second := cookieNamed(rotated.Result().Cookies(), refreshCookieName)
	if second == nil || second.Value == "" || second.Value == first.Value {
		t.Fatal("refresh did not set a new refresh cookie")
	}

	racing := postWithCookie(e, host, "/api/v1/auth/refresh", first, origin)
	if racing.Code != http.StatusOK {
		t.Fatalf("a racing refresh with the replaced cookie: status = %d body = %s", racing.Code, racing.Body.String())
	}
	if h := setCookieHeader(racing, refreshCookieName); h != "" {
		t.Errorf("a grace refresh touched the cookie: %s", h)
	}

	next := postWithCookie(e, host, "/api/v1/auth/refresh", second, origin)
	if c := cookieNamed(next.Result().Cookies(), refreshCookieName); next.Code != http.StatusOK || c == nil || c.Value == second.Value {
		t.Errorf("refresh with the current cookie: status = %d, want 200 and another rotation", next.Code)
	}
}
