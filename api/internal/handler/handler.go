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

// Register mounts the transport. apiMiddleware carries the middleware for the /api/v1
// group — P01 attaches host-to-tenant resolution there; P00 has no /api/v1 routes yet,
// so the group itself is not created.
func (h *Handler) Register(e *echo.Echo, apiMiddleware ...echo.MiddlewareFunc) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)
}
