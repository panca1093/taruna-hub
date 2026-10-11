package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestCORSPreflightDemoHeader(t *testing.T) {
	cases := []struct {
		name string
		dev  bool
		want bool
	}{
		{"development allows X-Demo-User", true, true},
		{"non-development rejects X-Demo-User", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(CORS([]string{"http://localhost:3000"}, tc.dev))
			e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			req := httptest.NewRequest(http.MethodOptions, "/x", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			req.Header.Set("Access-Control-Request-Method", http.MethodGet)
			req.Header.Set("Access-Control-Request-Headers", "X-Demo-User")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			allowed := rec.Header().Get("Access-Control-Allow-Headers")
			got := bytes.Contains([]byte(allowed), []byte("X-Demo-User"))
			if got != tc.want {
				t.Errorf("Access-Control-Allow-Headers = %q, X-Demo-User allowed = %v, want %v", allowed, got, tc.want)
			}
		})
	}
}
