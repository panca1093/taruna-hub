package handler

import (
	"log/slog"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// AuthOptions are the two transport decisions the auth routes need; every rule lives in the service.
type AuthOptions struct {
	// SecureCookies marks the refresh cookie Secure outside development.
	SecureCookies bool
	// AllowedOrigin is the CORS decision; refresh and logout require it in addition to the request host.
	AllowedOrigin func(origin string) bool
}

type Handler struct {
	logger        *slog.Logger
	health        *HealthHandler
	auth          *service.AuthService
	secureCookies bool
	allowedOrigin func(origin string) bool
}

func New(logger *slog.Logger, health *HealthHandler, auth *service.AuthService, opts AuthOptions) *Handler {
	return &Handler{
		logger:        logger,
		health:        health,
		auth:          auth,
		secureCookies: opts.SecureCookies,
		allowedOrigin: opts.AllowedOrigin,
	}
}

func (h *Handler) Register(e *echo.Echo, apiMiddleware ...echo.MiddlewareFunc) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)

	v1 := e.Group("/api/v1")
	v1.Use(apiMiddleware...)
	v1.GET("/tenant", h.Tenant)

	// NoStore is first on every auth route so a rejection carries it too.
	v1.POST("/auth/login", h.Login, httpx.NoStore())
	v1.POST("/auth/refresh", h.Refresh, httpx.NoStore(), h.requireAcceptedOrigin)
	v1.POST("/auth/logout", h.Logout, httpx.NoStore(), h.requireAcceptedOrigin)
	v1.GET("/me", h.Me, httpx.NoStore(), Authenticate(h.auth))
}
