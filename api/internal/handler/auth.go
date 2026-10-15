package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const principalKey = "principal"

// The cookie is the only place the refresh token travels to the browser, and it never reaches JS.
const (
	refreshCookieName = "refresh_token"
	refreshCookiePath = "/api/v1/auth"
	// D13 keeps the session to 3h idle; the cookie's own month is only an upper bound.
	refreshCookieMaxAgeSeconds = 30 * 24 * 60 * 60
)

const (
	invalidCredentials = "Kredensial tidak valid."
	sessionInvalid     = "Sesi tidak valid."
	originRejected     = "Origin tidak diizinkan."
	roleDenied         = "Akses ditolak."
)

// Scope comes from the host middleware, so a token minted for another tenant never passes here.
func Authenticate(auth *service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			scope, ok := hostScope(c)
			if !ok {
				return httpx.NotFound(tenantNotFound)
			}
			raw, ok := bearerToken(c)
			if !ok {
				return httpx.Unauthorized(sessionInvalid)
			}
			principal, err := auth.Authenticate(c.Request().Context(), scope, raw)
			if err != nil {
				return httpx.Unauthorized(sessionInvalid)
			}
			c.Set(principalKey, principal)
			return next(c)
		}
	}
}

// RequireRole authorizes by role only: it never reads the token again and never widens the scope.
func RequireRole(roles ...model.Role) echo.MiddlewareFunc {
	allowed := make(map[model.Role]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			principal, ok := PrincipalFrom(c)
			if !ok {
				return httpx.Unauthorized(sessionInvalid)
			}
			if !allowed[principal.Role] {
				return httpx.Forbidden(roleDenied)
			}
			return next(c)
		}
	}
}

// PrincipalFrom is how a handler reads the authenticated caller; every guard shares this one key.
func PrincipalFrom(c echo.Context) (model.Principal, bool) {
	principal, ok := c.Get(principalKey).(model.Principal)
	return principal, ok
}

func bearerToken(c echo.Context) (string, bool) {
	const prefix = "Bearer "
	header := c.Request().Header.Get(echo.HeaderAuthorization)
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// LoginRequest is the credential body. FR-AUTH-02: one field matches email or phone.
type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// userResponse publishes only what a session needs; balance and the ledger stay out of this slice.
type userResponse struct {
	ID    uuid.UUID  `json:"id"`
	Name  string     `json:"name"`
	Email *string    `json:"email"`
	Phone *string    `json:"phone"`
	Role  model.Role `json:"role"`
	// EmailVerified drives the web app's "verify your email" alert; it never blocks anything.
	EmailVerified bool `json:"email_verified"`
}

type tokenResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        userResponse `json:"user"`
}

// Login verifies the credentials against the request host's scope and opens a session.
func (h *Handler) Login(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}

	var req loginRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	fields := map[string]string{}
	if strings.TrimSpace(req.Identifier) == "" {
		fields["identifier"] = "Wajib diisi."
	}
	if req.Password == "" {
		fields["password"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		return httpx.Validation("Isi permintaan belum lengkap.", fields)
	}

	tokens, err := h.auth.Login(c.Request().Context(), scope, req.Identifier, req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return httpx.Unauthorized(invalidCredentials)
	}
	if err != nil {
		return rateLimited(c, err)
	}

	h.setRefreshCookie(c, tokens.RefreshToken)
	return c.JSON(http.StatusOK, newTokenResponse(tokens))
}

// Refresh issues a new access token and rotates the cookie. The refresh token is never echoed in a body.
func (h *Handler) Refresh(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}

	tokens, err := h.auth.Refresh(c.Request().Context(), scope, h.refreshCookie(c))
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			// This token can never work again, so stop the browser from sending it.
			h.clearRefreshCookie(c)
			return httpx.Unauthorized(sessionInvalid)
		}
		return err
	}
	// Empty means a racing refresh already rotated, and its response carries the cookie to keep.
	if tokens.RefreshToken != "" {
		h.setRefreshCookie(c, tokens.RefreshToken)
	}
	return c.JSON(http.StatusOK, newTokenResponse(tokens))
}

// Always 204 and always clears the cookie, so a retry or a stale token is not a client error.
func (h *Handler) Logout(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	if err := h.auth.Logout(c.Request().Context(), scope, h.refreshCookie(c)); err != nil {
		h.logger.Error("logout could not revoke the session",
			slog.String("request_id", httpx.RequestIDFrom(c)),
			slog.String("error", err.Error()),
		)
	}
	h.clearRefreshCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// CORS alone still runs the handler for a rejected origin, and a sibling tenant's origin passes CORS, so the host must match too.
func (h *Handler) requireAcceptedOrigin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		origin := c.Request().Header.Get(echo.HeaderOrigin)
		if origin == "" || h.allowedOrigin == nil || !h.allowedOrigin(origin) || !sameHostname(origin, c) {
			return httpx.Forbidden(originRejected)
		}
		return next(c)
	}
}

// bindStrict refuses unknown fields, so a misspelled credential field is an error and not a default.
func bindStrict(c echo.Context, dst any) error {
	dec := json.NewDecoder(c.Request().Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return httpx.Validation("Isi permintaan tidak dapat dibaca.", map[string]string{"body": "Format permintaan tidak sesuai."})
	}
	return nil
}

func newUserResponse(p model.Principal) userResponse {
	return userResponse{
		ID: p.UserID, Name: p.Name, Email: p.Email, Phone: p.Phone, Role: p.Role,
		EmailVerified: p.EmailVerified,
	}
}

func newTokenResponse(t service.Tokens) tokenResponse {
	return tokenResponse{
		AccessToken: t.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   t.ExpiresIn,
		User:        newUserResponse(t.Principal),
	}
}

func (h *Handler) refreshCookie(c echo.Context) string {
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *Handler) setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(h.refreshCookieFor(token, refreshCookieMaxAgeSeconds))
}

// clearRefreshCookie uses MaxAge -1, which Go renders as Max-Age=0; MaxAge 0 does not delete.
func (h *Handler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(h.refreshCookieFor("", -1))
}

func (h *Handler) refreshCookieFor(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     refreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies,
	}
}

// Ports are ignored so web on :3000 can call the API on :8081 in development.
func sameHostname(origin string, c echo.Context) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host, ok := requestHost(c)
	if !ok {
		return false
	}
	return parsed.Hostname() == hostnameOnly(host)
}

func hostnameOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
