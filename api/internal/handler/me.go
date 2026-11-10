package handler

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// Identity comes only from the principal, so neither body names a user.
type updateProfileRequest struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
	BirthDate string `json:"birth_date"`
}

type setAvatarRequest struct {
	Avatar json.RawMessage `json:"avatar"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Me publishes the authenticated principal the middleware rebuilt from the database.
func (h *Handler) Me(c echo.Context) error {
	principal, ok := PrincipalFrom(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	return c.JSON(http.StatusOK, newUserResponse(principal))
}

// UpdateMe changes the caller's own profile fields and answers with the user as /me would.
func (h *Handler) UpdateMe(c echo.Context) error {
	scope, ok := hostScope(c)
	principal, signedIn := PrincipalFrom(c)
	if !ok || !signedIn {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req updateProfileRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	updated, err := h.account.UpdateProfile(c.Request().Context(), scope, principal, service.ProfileInput{
		Name: req.Name, Phone: req.Phone, Gender: req.Gender, BirthDate: req.BirthDate,
	})
	if err != nil {
		return accountError(c, err)
	}
	return c.JSON(http.StatusOK, newUserResponse(updated))
}

// SetAvatar stores the caller's pick from the built-in avatars; null goes back to the automatic one.
func (h *Handler) SetAvatar(c echo.Context) error {
	principal, signedIn := PrincipalFrom(c)
	if !signedIn {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req setAvatarRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if len(req.Avatar) == 0 {
		return httpx.Validation(incomplete, map[string]string{"avatar": required})
	}
	var avatar *string
	if string(req.Avatar) != "null" {
		var key string
		if err := json.Unmarshal(req.Avatar, &key); err != nil {
			return httpx.Validation(incomplete, map[string]string{"avatar": invalidFormat})
		}
		avatar = &key
	}
	updated, err := h.account.SetAvatar(c.Request().Context(), principal, avatar)
	if err != nil {
		return accountError(c, err)
	}
	return c.JSON(http.StatusOK, newUserResponse(updated))
}

// ChangePassword keeps the caller signed in: a wrong current password is a field error, never a 401.
func (h *Handler) ChangePassword(c echo.Context) error {
	scope, ok := hostScope(c)
	principal, signedIn := PrincipalFrom(c)
	if !ok || !signedIn {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req changePasswordRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"current_password": req.CurrentPassword, "new_password": req.NewPassword}); err != nil {
		return err
	}
	if err := h.account.ChangePassword(c.Request().Context(), scope, principal, req.CurrentPassword, req.NewPassword); err != nil {
		return accountError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
