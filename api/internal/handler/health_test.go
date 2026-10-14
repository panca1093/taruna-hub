package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
)

// testPool is the shared connect-or-skip gate: the readiness proof is only meaningful
// against a real database, so these tests must run rather than silently pass.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := infra.NewPostgresPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func readyzEcho(pool *pgxpool.Pool) *echo.Echo {
	e := echo.New()
	h := New(slog.New(slog.NewTextHandler(os.Stderr, nil)), NewHealthHandler(pool))
	h.Register(e)
	return e
}

func getReadyz(e *echo.Echo) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	return rec
}

func TestReadyzReportsReachableDatabase(t *testing.T) {
	rec := getReadyz(readyzEcho(testPool(t)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not JSON: %v", rec.Body.String(), err)
	}
	if body["status"] != "ready" {
		t.Errorf("body = %s, want status ready", rec.Body.String())
	}
}

func TestReadyzReportsUnavailableDatabase(t *testing.T) {
	pool := testPool(t)
	pool.Close()

	rec := getReadyz(readyzEcho(pool))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not JSON: %v", rec.Body.String(), err)
	}
	if body["status"] != "unavailable" || body["reason"] != "database unreachable" {
		t.Errorf("body = %s, want status unavailable with reason database unreachable", rec.Body.String())
	}
}
