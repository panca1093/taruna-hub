package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const currentUserKey = "current_user"

// DevIdentity is a placeholder for the JWT middleware that S1 introduces. It resolves
// the caller from X-Demo-User (email or phone) so the walking skeleton can prove a real
// database round-trip before auth exists.
//
// TODO: remove in S1 — this trusts a client-supplied header and must never reach production.
func DevIdentity(users *service.UserService) echo.MiddlewareFunc {
	const devUserHeader = "X-Demo-User"
	const fallbackUser = "naila@example.com"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			identifier := c.Request().Header.Get(devUserHeader)
			if identifier == "" {
				identifier = fallbackUser
			}

			user, err := users.ByIdentifier(c.Request().Context(), identifier)
			if errors.Is(err, service.ErrUserNotFound) {
				return httpx.Unauthorized("Pengguna tidak dikenali.")
			}
			if err != nil {
				return err
			}

			c.Set(currentUserKey, user)
			return next(c)
		}
	}
}

// NoIdentity keeps /api/v1 closed outside development until P03 provides real sessions.
func NoIdentity(echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httpx.Unauthorized("Autentikasi diperlukan.")
	}
}

func currentUser(c echo.Context) (model.User, bool) {
	u, ok := c.Get(currentUserKey).(model.User)
	return u, ok
}

type meResponse struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Email   *string `json:"email"`
	Phone   *string `json:"phone"`
	Role    string  `json:"role"`
	Balance float64 `json:"balance"`
}

func (h *Handler) Me(c echo.Context) error {
	user, ok := currentUser(c)
	if !ok {
		return httpx.Unauthorized("Sesi tidak ditemukan.")
	}

	return c.JSON(http.StatusOK, meResponse{
		ID:      user.ID.String(),
		Name:    user.Name,
		Email:   user.Email,
		Phone:   user.Phone,
		Role:    string(user.Role),
		Balance: user.Balance,
	})
}
