package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

// Me publishes the authenticated principal the middleware rebuilt from the database.
func (h *Handler) Me(c echo.Context) error {
	principal, ok := PrincipalFrom(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	return c.JSON(http.StatusOK, newUserResponse(principal))
}
