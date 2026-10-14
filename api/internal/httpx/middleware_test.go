package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRequestLoggerRecordsReturnedErrorStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"unauthorized", Unauthorized("Sesi tidak ditemukan."), http.StatusUnauthorized},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			e := echo.New()
			e.HTTPErrorHandler = ErrorHandler(logger)
			e.Use(RequestLogger(logger))
			e.GET("/x", func(echo.Context) error { return tc.err })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

			if rec.Code != tc.want {
				t.Fatalf("response status = %d, want %d", rec.Code, tc.want)
			}

			var logged int
			dec := json.NewDecoder(&buf)
			for dec.More() {
				var line map[string]any
				if err := dec.Decode(&line); err != nil {
					t.Fatalf("decode log line: %v", err)
				}
				if line["msg"] == "request" {
					logged = int(line["status"].(float64))
				}
			}
			if logged != tc.want {
				t.Errorf("logged status = %d, want %d", logged, tc.want)
			}
		})
	}
}

// The demo identity is gone; no environment may advertise the header it used.
func TestCORSAllowHeadersNeverIncludeDemoHeader(t *testing.T) {
	cases := []struct {
		name string
		dev  bool
	}{
		{"development", true},
		{"non-development", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := preflightAllowHeaders(t, tc.dev, "http://localhost:3000")
			if strings.Contains(allowed, "X-Demo-User") {
				t.Errorf("Access-Control-Allow-Headers = %q, must not allow X-Demo-User", allowed)
			}
		})
	}
}

func TestCORSOriginPolicyPerEnvironment(t *testing.T) {
	cases := []struct {
		name  string
		dev   bool
		allow []string
		deny  []string
	}{
		{
			name:  "development accepts any localhost port",
			dev:   true,
			allow: []string{"http://localhost:3000", "http://localhost:4123", "http://127.0.0.1:3000"},
			deny:  []string{"http://example.com", "http://localhost.evil.com"},
		},
		{
			name:  "non-development accepts only the configured origins",
			dev:   false,
			allow: []string{"http://localhost:3000"},
			deny:  []string{"http://localhost:4123", "http://127.0.0.1:3000", "http://example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, origin := range tc.allow {
				if got := preflightAllowOrigin(t, tc.dev, origin); got != origin {
					t.Errorf("preflight Origin %s: Access-Control-Allow-Origin = %q, want %q", origin, got, origin)
				}
				if got := simpleAllowOrigin(t, tc.dev, origin); got != origin {
					t.Errorf("GET Origin %s: Access-Control-Allow-Origin = %q, want %q", origin, got, origin)
				}
			}
			for _, origin := range tc.deny {
				if got := preflightAllowOrigin(t, tc.dev, origin); got != "" {
					t.Errorf("preflight Origin %s: Access-Control-Allow-Origin = %q, want empty", origin, got)
				}
				if got := simpleAllowOrigin(t, tc.dev, origin); got != "" {
					t.Errorf("GET Origin %s: Access-Control-Allow-Origin = %q, want empty", origin, got)
				}
			}
		})
	}
}

func corsEcho(dev bool) *echo.Echo {
	e := echo.New()
	e.Use(CORS([]string{"http://localhost:3000"}, dev))
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	return e
}

func preflightAllowHeaders(t *testing.T, dev bool, origin string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "X-Demo-User")
	rec := httptest.NewRecorder()
	corsEcho(dev).ServeHTTP(rec, req)
	return rec.Header().Get("Access-Control-Allow-Headers")
}

func preflightAllowOrigin(t *testing.T, dev bool, origin string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	corsEcho(dev).ServeHTTP(rec, req)
	return rec.Header().Get("Access-Control-Allow-Origin")
}

func simpleAllowOrigin(t *testing.T, dev bool, origin string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	corsEcho(dev).ServeHTTP(rec, req)
	return rec.Header().Get("Access-Control-Allow-Origin")
}
