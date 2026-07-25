package handler

import (
	"log/slog"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type Handler struct {
	cfg    config.Config
	logger *slog.Logger
	users  *service.UserService
	health *HealthHandler
}

func New(cfg config.Config, logger *slog.Logger, users *service.UserService, health *HealthHandler) *Handler {
	return &Handler{cfg: cfg, logger: logger, users: users, health: health}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.health.Live)
	e.GET("/readyz", h.health.Ready)

	v1 := e.Group("/api/v1")
	v1.Use(devIdentity(h.users))
	v1.GET("/me", h.Me)
}
