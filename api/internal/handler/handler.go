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

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)
}
