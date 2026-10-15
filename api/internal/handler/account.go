package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	required          = "Wajib diisi."
	incomplete        = "Isi permintaan belum lengkap."
	invalidFormat     = "Format tidak valid."
	passwordRule      = "Kata sandi 8–72 karakter."
	registrationGone  = "Pendaftaran tidak tersedia di alamat ini."
	emailTaken        = "Email sudah terdaftar."
	phoneTaken        = "Nomor HP sudah terdaftar."
	alreadyRegistered = "Akun sudah terdaftar."
	codeInvalid       = "Kode salah atau sudah kedaluwarsa."
	resetInvalid      = "Tautan tidak valid atau sudah kedaluwarsa."
	tooManyRequests   = "Terlalu banyak percobaan. Coba lagi nanti."
)

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type emailRequest struct {
	Email string `json:"email"`
}

type verifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type resetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type codeSentResponse struct {
	Email       string `json:"email"`
	ExpiresIn   int    `json:"expires_in"`
	ResendAfter int    `json:"resend_after"`
}

// SignUp starts a registration; the account exists only once the emailed code is verified.
func (h *Handler) SignUp(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	var req registerRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"name": req.Name, "email": req.Email, "password": req.Password}); err != nil {
		return err
	}
	sent, err := h.account.Register(c.Request().Context(), scope, service.RegisterInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone, Password: req.Password,
	})
	if err != nil {
		return accountError(c, err)
	}
	return c.JSON(http.StatusAccepted, newCodeSent(sent))
}

func (h *Handler) ResendCode(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	var req emailRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"email": req.Email}); err != nil {
		return err
	}
	sent, err := h.account.ResendCode(c.Request().Context(), scope, req.Email)
	if err != nil {
		return accountError(c, err)
	}
	return c.JSON(http.StatusAccepted, newCodeSent(sent))
}

// VerifyCode creates the account and answers exactly like a login, cookie included.
func (h *Handler) VerifyCode(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	var req verifyRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"email": req.Email, "code": req.Code}); err != nil {
		return err
	}
	tokens, err := h.account.VerifyCode(c.Request().Context(), scope, req.Email, strings.TrimSpace(req.Code))
	if err != nil {
		return accountError(c, err)
	}
	h.setRefreshCookie(c, tokens.RefreshToken)
	return c.JSON(http.StatusOK, newTokenResponse(tokens))
}

// ForgotPassword is 202 for every well-formed email, so the answer says nothing about who exists.
func (h *Handler) ForgotPassword(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	var req emailRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"email": req.Email}); err != nil {
		return err
	}
	if err := h.account.ForgotPassword(c.Request().Context(), scope, req.Email); err != nil {
		return accountError(c, err)
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) ResetPassword(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(tenantNotFound)
	}
	var req resetRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	if err := requireFields(map[string]string{"token": req.Token, "password": req.Password}); err != nil {
		return err
	}
	if err := h.account.ResetPassword(c.Request().Context(), scope, req.Token, req.Password); err != nil {
		return accountError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func requireFields(values map[string]string) error {
	fields := map[string]string{}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			fields[name] = required
		}
	}
	if len(fields) > 0 {
		return httpx.Validation(incomplete, fields)
	}
	return nil
}

func accountError(c echo.Context, err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for name, reason := range input.Fields {
			fields[name] = invalidFormat
			if reason == service.ReasonPassword {
				fields[name] = passwordRule
			}
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrRegistrationClosed):
		return httpx.NotFound(registrationGone)
	case errors.Is(err, service.ErrEmailTaken):
		return httpx.Conflict(alreadyRegistered).WithFields(map[string]string{"email": emailTaken})
	case errors.Is(err, service.ErrPhoneTaken):
		return httpx.Conflict(alreadyRegistered).WithFields(map[string]string{"phone": phoneTaken})
	case errors.Is(err, service.ErrInvalidCode):
		return httpx.Validation(codeInvalid, map[string]string{"code": codeInvalid})
	case errors.Is(err, service.ErrInvalidResetToken):
		return httpx.Validation(resetInvalid, map[string]string{"token": resetInvalid})
	}
	return rateLimited(c, err)
}

// rateLimited turns a throttle refusal into 429 with Retry-After and passes every other error through.
func rateLimited(c echo.Context, err error) error {
	var limited *service.ThrottleError
	if !errors.As(err, &limited) {
		return err
	}
	seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Response().Header().Set("Retry-After", strconv.Itoa(seconds))
	return httpx.TooManyRequests(tooManyRequests)
}

func newCodeSent(s service.CodeSent) codeSentResponse {
	return codeSentResponse{
		Email:       s.Email,
		ExpiresIn:   int(s.ExpiresIn.Seconds()),
		ResendAfter: int(s.ResendAfter.Seconds()),
	}
}
