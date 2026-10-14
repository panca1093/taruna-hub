package handler

import (
	"log/slog"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	logger *slog.Logger
	health *HealthHandler
}

func New(logger *slog.Logger, health *HealthHandler) *Handler {
	return &Handler{logger: logger, health: health}
}

func (h *Handler) Register(e *echo.Echo, apiMiddleware ...echo.MiddlewareFunc) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)

	v1 := e.Group("/api/v1")
	v1.Use(apiMiddleware...)
	v1.GET("/tenant", h.Tenant)
}
