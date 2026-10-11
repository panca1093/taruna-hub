package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func newTestEcho(users *service.UserService) *echo.Echo {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler(logger)
	New(logger, users, nil).Register(e, DevIdentity(users))
	return e
}

func getMe(e *echo.Echo, demoUser string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	if demoUser != "" {
		req.Header.Set("X-Demo-User", demoUser)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestMeResolvesDemoIdentityInDevelopment(t *testing.T) {
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

	email := "p00-" + uuid.NewString() + "@example.com"
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ('P00 Probe', $1, 'x')`, email); err != nil {
		t.Fatalf("insert probe user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email) })

	e := newTestEcho(service.NewUserService(repository.NewUserRepository(pool)))

	rec := getMe(e, email)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), email) {
		t.Fatalf("status = %d body = %s, want 200 with %s", rec.Code, rec.Body.String(), email)
	}

	rec = getMe(e, "nobody@example.com")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown demo user: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
