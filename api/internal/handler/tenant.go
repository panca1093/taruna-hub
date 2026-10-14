package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const hostScopeKey = "host_scope"

// tenantNotFound is one message for every host without a tenant, so a caller cannot tell them apart.
const tenantNotFound = "Tenant tidak ditemukan."

// ResolveHostScope selects the tenant from the request host; X-Forwarded-Host only behind a trusted proxy.
func ResolveHostScope(tenants *service.TenantService, trustForwardedHost bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			host := c.Request().Host
			if fwd := c.Request().Header.Get("X-Forwarded-Host"); trustForwardedHost && fwd != "" {
				host = strings.TrimSpace(fwd)
			}

			scope, err := tenants.ResolveHost(c.Request().Context(), host)
			if errors.Is(err, service.ErrTenantNotFound) {
				return httpx.NotFound(tenantNotFound)
			}
			if err != nil {
				return err
			}

			c.Set(hostScopeKey, scope)
			return next(c)
		}
	}
}

func hostScope(c echo.Context) (service.HostScope, bool) {
	s, ok := c.Get(hostScopeKey).(service.HostScope)
	return s, ok
}

// tenantResponse is an explicit DTO: the scope also carries the tenant ID and activity, which are not public.
type tenantResponse struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Tenant serves the public lookup the landing pages need; the platform host has no tenant to show.
func (h *Handler) Tenant(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok || scope.Platform {
		return httpx.NotFound(tenantNotFound)
	}
	return c.JSON(http.StatusOK, tenantResponse{Slug: scope.Tenant.Slug, Name: scope.Tenant.Name})
}
