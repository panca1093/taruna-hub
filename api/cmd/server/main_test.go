package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/handler"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

// Staging must be treated like production: the demo identity trusts a client header.
func TestAPIIdentityRejectsDemoIdentityOutsideDevelopment(t *testing.T) {
	for _, env := range []string{"production", "staging"} {
		t.Run(env, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
			e := echo.New()
			e.HTTPErrorHandler = httpx.ErrorHandler(logger)
			handler.New(logger, nil, nil).Register(e, apiIdentity(config.Config{Env: env}, nil))

			for _, header := range []string{"", "naila@example.com"} {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
				if header != "" {
					req.Header.Set("X-Demo-User", header)
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("X-Demo-User=%q: status = %d, want %d", header, rec.Code, http.StatusUnauthorized)
				}
			}
		})
	}
}
