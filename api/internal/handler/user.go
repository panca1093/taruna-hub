package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	userNotFound     = "Akun tidak ditemukan."
	userSelfRule     = "Anda tidak dapat menonaktifkan akun sendiri."
	userRoleRule     = "Peran harus student, operator atau admin."
	userReasonRule   = "Alasan wajib diisi, maksimal 500 karakter."
	userKeyRule      = "Kunci idempotensi 8–64 karakter: huruf, angka, _ atau -."
	userTargetRule   = "Pilih tepat satu: tryout_id atau kecermatan_test_id."
	userAttemptsRule = "Jumlah percobaan 1–1000, hanya untuk tes kecermatan."
	userKeyConflict  = "Kunci idempotensi sudah dipakai untuk pemberian akses lain."
	grantTargetGone  = "Tryout (terbit) atau tes kecermatan tidak ditemukan."
	grantHeld        = "Siswa sudah memiliki tryout ini."
	maxUserBody      = 8 << 10
)

type adminUserResponse struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Email         *string    `json:"email"`
	Phone         *string    `json:"phone"`
	Role          model.Role `json:"role"`
	IsActive      bool       `json:"is_active"`
	Balance       string     `json:"balance"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
}

type adminUserListResponse struct {
	Users []adminUserResponse `json:"users"`
	Total int                 `json:"total"`
}

type createUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type accountReasonRequest struct {
	Reason string `json:"reason"`
}

type grantRequest struct {
	TryoutID         *uuid.UUID `json:"tryout_id"`
	KecermatanTestID *uuid.UUID `json:"kecermatan_test_id"`
	Attempts         int        `json:"attempts"`
	Reason           string     `json:"reason"`
	IdempotencyKey   string     `json:"idempotency_key"`
}

type studentAccessResponse struct {
	EnrollmentID      uuid.UUID  `json:"enrollment_id"`
	Kind              string     `json:"kind"`
	TestID            uuid.UUID  `json:"test_id"`
	Title             string     `json:"title"`
	ActiveFrom        *time.Time `json:"active_from"`
	ActiveUntil       *time.Time `json:"active_until"`
	AttemptsPurchased *int       `json:"attempts_purchased"`
	AttemptsUsed      int        `json:"attempts_used"`
	Active            bool       `json:"active"`
	SourceType        string     `json:"source_type"`
	CreatedAt         time.Time  `json:"created_at"`
}

type studentAccessListResponse struct {
	Access []studentAccessResponse `json:"access"`
	Total  int                     `json:"total"`
}

func adminUserOf(u model.User) adminUserResponse {
	return adminUserResponse{
		ID: u.ID, Name: u.Name, Email: u.Email, Phone: u.Phone, Role: u.Role, IsActive: u.IsActive,
		Balance: u.Balance, EmailVerified: u.EmailVerifiedAt != nil, CreatedAt: u.CreatedAt,
	}
}

func studentAccessOf(a model.StudentAccess) studentAccessResponse {
	return studentAccessResponse{
		EnrollmentID: a.EnrollmentID, Kind: a.Kind, TestID: a.TestID, Title: a.Title, ActiveFrom: a.ActiveFrom,
		ActiveUntil: a.ActiveUntil, AttemptsPurchased: a.AttemptsPurchased, AttemptsUsed: a.AttemptsUsed,
		Active: a.Active, SourceType: a.SourceType, CreatedAt: a.CreatedAt,
	}
}

func (h *Handler) ListUsers(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := walletPageOf(c)
	if err != nil {
		return err
	}
	in := service.UserListInput{Role: c.QueryParam("role"), Search: c.QueryParam("q"), Limit: limit, Offset: offset}
	if raw := c.QueryParam("active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			return httpx.Validation(incomplete, map[string]string{"active": invalidFormat})
		}
		in.Active = &active
	}
	users, total, err := h.userAdmin.List(c.Request().Context(), scope, principal, in)
	if err != nil {
		return userAdminError(err)
	}
	body := adminUserListResponse{Users: make([]adminUserResponse, 0, len(users)), Total: total}
	for _, u := range users {
		body.Users = append(body.Users, adminUserOf(u))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GetUser(c echo.Context) error {
	scope, principal, id, ok, err := userCaller(c)
	if !ok {
		return err
	}
	u, gerr := h.userAdmin.Get(c.Request().Context(), scope, principal, id)
	if gerr != nil {
		return userAdminError(gerr)
	}
	return c.JSON(http.StatusOK, adminUserOf(u))
}

func (h *Handler) CreateUser(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req createUserRequest
	if err := bindUser(c, &req); err != nil {
		return err
	}
	u, err := h.userAdmin.Create(c.Request().Context(), scope, principal, service.CreateUserInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone, Password: req.Password, Role: req.Role,
	})
	if err != nil {
		return userAdminError(err)
	}
	return c.JSON(http.StatusCreated, adminUserOf(u))
}

func (h *Handler) UpdateUser(c echo.Context) error {
	scope, principal, id, ok, err := userCaller(c)
	if !ok {
		return err
	}
	var req updateUserRequest
	if err := bindUser(c, &req); err != nil {
		return err
	}
	u, uerr := h.userAdmin.Update(c.Request().Context(), scope, principal, id, service.UpdateUserInput{
		Name: req.Name, Email: req.Email, Phone: req.Phone,
	})
	if uerr != nil {
		return userAdminError(uerr)
	}
	return c.JSON(http.StatusOK, adminUserOf(u))
}

func (h *Handler) DeactivateUser(c echo.Context) error {
	return h.setUserActive(c, false)
}

func (h *Handler) ReactivateUser(c echo.Context) error {
	return h.setUserActive(c, true)
}

func (h *Handler) setUserActive(c echo.Context, active bool) error {
	scope, principal, id, ok, err := userCaller(c)
	if !ok {
		return err
	}
	var req accountReasonRequest
	if err := bindUser(c, &req); err != nil {
		return err
	}
	var u model.User
	var serr error
	if active {
		u, serr = h.userAdmin.Reactivate(c.Request().Context(), scope, principal, id, req.Reason)
	} else {
		u, serr = h.userAdmin.Deactivate(c.Request().Context(), scope, principal, id, req.Reason)
	}
	if serr != nil {
		return userAdminError(serr)
	}
	return c.JSON(http.StatusOK, adminUserOf(u))
}

func (h *Handler) StudentAccess(c echo.Context) error {
	scope, principal, id, ok, err := userCaller(c)
	if !ok {
		return err
	}
	limit, offset, perr := walletPageOf(c)
	if perr != nil {
		return perr
	}
	rows, total, aerr := h.userAdmin.Access(c.Request().Context(), scope, principal, id, limit, offset)
	if aerr != nil {
		return userAdminError(aerr)
	}
	body := studentAccessListResponse{Access: make([]studentAccessResponse, 0, len(rows)), Total: total}
	for _, a := range rows {
		body.Access = append(body.Access, studentAccessOf(a))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) GrantStudentAccess(c echo.Context) error {
	scope, principal, id, ok, err := userCaller(c)
	if !ok {
		return err
	}
	var req grantRequest
	if err := bindUser(c, &req); err != nil {
		return err
	}
	result, gerr := h.userAdmin.Grant(c.Request().Context(), scope, principal, id, service.GrantInput{
		TryoutID: req.TryoutID, KecermatanTestID: req.KecermatanTestID, Attempts: req.Attempts,
		Reason: req.Reason, IdempotencyKey: req.IdempotencyKey,
	})
	if gerr != nil {
		return userAdminError(gerr)
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	return c.JSON(status, studentAccessOf(result.Access))
}

// userCaller resolves the caller and the :id; a malformed id is a 404, never a 400 that would confirm a shape.
func userCaller(c echo.Context) (service.HostScope, model.Principal, uuid.UUID, bool, error) {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return scope, principal, uuid.Nil, false, httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return scope, principal, uuid.Nil, false, httpx.NotFound(userNotFound)
	}
	return scope, principal, id, true, nil
}

func bindUser(c echo.Context, dst any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxUserBody)
	return bindStrict(c, dst)
}

func userAdminError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for name, reason := range input.Fields {
			fields[name] = userReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		return httpx.NotFound(userNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrUserForbidden), errors.Is(err, service.ErrReportForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrSelfDeactivation):
		return httpx.Forbidden(userSelfRule)
	case errors.Is(err, service.ErrEmailTaken):
		return httpx.Conflict(alreadyRegistered).WithFields(map[string]string{"email": emailTaken})
	case errors.Is(err, service.ErrPhoneTaken):
		return httpx.Conflict(alreadyRegistered).WithFields(map[string]string{"phone": phoneTaken})
	case errors.Is(err, service.ErrGrantKeyConflict):
		return httpx.Conflict(userKeyConflict)
	case errors.Is(err, service.ErrGrantTarget):
		return httpx.NotFound(grantTargetGone)
	case errors.Is(err, service.ErrAlreadyOwned):
		return httpx.New(http.StatusConflict, httpx.CodeAlreadyOwned, grantHeld)
	}
	return err
}

func userReason(reason string) string {
	switch reason {
	case service.ReasonPassword:
		return passwordRule
	case service.ReasonUserRole:
		return userRoleRule
	case service.ReasonUserReason:
		return userReasonRule
	case service.ReasonUserKey:
		return userKeyRule
	case service.ReasonUserTarget:
		return userTargetRule
	case service.ReasonUserAttempt:
		return userAttemptsRule
	case service.ReasonWalletQuery:
		return walletQueryRule
	case service.ReasonReportRange:
		return reportRangeRule
	}
	return invalidFormat
}

// WithStudentOps attaches the account/grant and revenue recap services, for the same reason as WithTopups.
func (h *Handler) WithStudentOps(users *service.UserAdminService, reports *service.ReportService) *Handler {
	h.userAdmin, h.reports = users, reports
	return h
}

// registerStudentOps wires P22. Operator and Admin both reach the account and grant routes (D02); the service
// narrows an Operator to student accounts. The recap is Admin-only.
func (h *Handler) registerStudentOps(v1 *echo.Group) {
	staff := RequireRole(model.RoleAdmin, model.RoleOperator)
	v1.GET("/admin/users", h.ListUsers, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.POST("/admin/users", h.CreateUser, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.GET("/admin/users/:id", h.GetUser, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.PUT("/admin/users/:id", h.UpdateUser, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.POST("/admin/users/:id/deactivate", h.DeactivateUser, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.POST("/admin/users/:id/reactivate", h.ReactivateUser, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.GET("/students/:id/enrollments", h.StudentAccess, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.POST("/students/:id/grants", h.GrantStudentAccess, httpx.NoStore(), Authenticate(h.auth), staff)
	v1.GET("/admin/reports/revenue", h.RevenueRecap, httpx.NoStore(), Authenticate(h.auth), RequireRole(model.RoleAdmin))
}
