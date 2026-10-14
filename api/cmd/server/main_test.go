package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/infra"
)

// Routes are asserted through newEcho, so this covers the middleware chain the server
// actually runs in every environment, not a hand-built approximation of it.
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	for _, env := range []string{"development", "staging", "production"} {
		t.Run(env, func(t *testing.T) {
			e := newEcho(config.Config{
				Env:         env,
				DatabaseURL: dsn,
				CORSOrigins: []string{"http://localhost:3000"},
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

			rec := serve(e, "/api/v1/me")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET /api/v1/me: status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			var envelope httpx.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("GET /api/v1/me: body %s is not the error envelope: %v", rec.Body.String(), err)
			}
			if envelope.Error.Code != httpx.CodeNotFound {
				t.Errorf("GET /api/v1/me: error code = %q, want %q", envelope.Error.Code, httpx.CodeNotFound)
			}
		})
	}
}

func serve(e *echo.Echo, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
