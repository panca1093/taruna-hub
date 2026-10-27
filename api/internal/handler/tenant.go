package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const hostScopeKey = "host_scope"

const tenantNotFound = "Tenant tidak ditemukan."

// ResolveHostScope loads the configured tenant for every request and fails closed when it is missing or inactive.
func ResolveHostScope(tenants *service.TenantService, slug string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			scope, err := tenants.Active(c.Request().Context(), slug)
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

// Tenant serves the public lookup the landing pages need.
func (h *Handler) Tenant(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	return c.JSON(http.StatusOK, tenantResponse{Slug: scope.Tenant.Slug, Name: scope.Tenant.Name})
}
