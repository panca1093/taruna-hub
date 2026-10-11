package httpx

import (
	"log/slog"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDFrom(c echo.Context) string {
	if id, ok := c.Get(echo.HeaderXRequestID).(string); ok && id != "" {
		return id
	}
	return c.Response().Header().Get(echo.HeaderXRequestID)
}

// RequestLogger emits one structured line per request, carrying the request id so a
// client-reported failure can be traced back to a single log entry. It renders a returned
// error itself so the logged status is the one the client actually receives.
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			if err := next(c); err != nil {
				c.Error(err)
			}
			logger.Info("request",
				slog.String("request_id", RequestIDFrom(c)),
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.Int("status", c.Response().Status),
				slog.Duration("took", time.Since(start)),
			)
			return nil
		}
	}
}

func RequestID() echo.MiddlewareFunc {
	return middleware.RequestIDWithConfig(middleware.RequestIDConfig{TargetHeader: RequestIDHeader})
}

func Recover(logger *slog.Logger) echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			logger.Error("panic recovered",
				slog.String("request_id", RequestIDFrom(c)),
				slog.String("path", c.Request().URL.Path),
				slog.String("error", err.Error()),
				slog.String("stack", string(stack)),
			)
			return err
		},
	})
}

// CORS locks responses to the configured origins. In development it accepts any
// localhost port instead, because the Next.js dev server falls back to a random port
// whenever 3000 is taken — pinning one port there just produces confusing CORS failures.
// Development also allows the X-Demo-User header, or its preflight would be rejected.
func CORS(origins []string, development bool) echo.MiddlewareFunc {
	cfg := middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", RequestIDHeader},
		AllowCredentials: true,
		MaxAge:           300,
	}
	if development {
		cfg.AllowHeaders = append(cfg.AllowHeaders, "X-Demo-User")
		cfg.AllowOriginFunc = func(origin string) (bool, error) {
			return strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:"), nil
		}
	}
	return middleware.CORSWithConfig(cfg)
}

// NoStore marks exam-content responses uncacheable. PRD §7.2: per-session shuffled
// content must never be shared via a CDN cache, or every student gets one order.
func NoStore() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	}
}
