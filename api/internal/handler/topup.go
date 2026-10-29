package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	topupNotFound    = "Permintaan top-up tidak ditemukan."
	topupStateRule   = "Permintaan top-up sudah diproses dan tidak bisa diubah."
	topupKeyConflict = "Kunci idempotensi sudah dipakai untuk permintaan top-up lain."
	topupAmountRule  = "Jumlah top-up harus rupiah penuh dan tidak kurang dari minimum top-up."
	topupKeyRule     = "Kunci idempotensi 8–64 karakter: huruf, angka, _ atau -."
	topupProofRule   = "Bukti transfer wajib dan harus berkas milikmu yang belum dipakai."
	topupRejectRule  = "Alasan penolakan wajib diisi, maksimal 500 karakter."
	topupQueryRule   = "Parameter tidak valid."
	topupMethodRule  = "Metode top-up harus manual atau gateway."
	topupMaxBody     = 8 << 10
)

type topupResponse struct {
	ID           uuid.UUID  `json:"id"`
	Code         string     `json:"code"`
	Status       string     `json:"status"`
	Method       string     `json:"method"`
	StudentID    uuid.UUID  `json:"student_id"`
	StudentName  string     `json:"student_name"`
	Amount       string     `json:"amount"`
	ProofAssetID *uuid.UUID `json:"proof_asset_id"`
	RejectReason string     `json:"reject_reason"`
	CreatedAt    time.Time  `json:"created_at"`
	ApprovedAt   *time.Time `json:"approved_at"`
	RejectedAt   *time.Time `json:"rejected_at"`
	RedirectURL  *string    `json:"redirect_url"`
}

type topupsResponse struct {
	Topups   []topupResponse `json:"topups"`
	Total    int             `json:"total"`
	MinTopup string          `json:"min_topup"`
}

type topupCreateRequest struct {
	Method         string  `json:"method"`
	Amount         string  `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
	ProofAssetID   *string `json:"proof_asset_id"`
}

type topupRejectRequest struct {
	Reason string `json:"reason"`
}

// CreateTopup answers 201 for a new request and 200 when the same idempotency key finds its earlier one.
func (h *Handler) CreateTopup(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, topupMaxBody)
	var req topupCreateRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input := service.CreateTopupInput{Method: req.Method, Amount: req.Amount, Key: req.IdempotencyKey}
	if req.ProofAssetID != nil && strings.TrimSpace(*req.ProofAssetID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ProofAssetID))
		if err != nil {
			return httpx.Validation(incomplete, map[string]string{"proof_asset_id": invalidFormat})
		}
		input.ProofAssetID = &id
	}
	topup, replayed, err := h.topups.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return topupError(err)
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	return c.JSON(status, topupResponseOf(topup))
}

func (h *Handler) MyTopups(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	topups, total, minimum, err := h.topups.Topups(c.Request().Context(), scope, principal, limit, offset)
	if err != nil {
		return topupError(err)
	}
	return c.JSON(http.StatusOK, topupsOf(topups, total, minimum))
}

// MyTopup and StaffTopup share one service read: the service scopes a student to their own requests.
func (h *Handler) MyTopup(c echo.Context) error { return h.topup(c) }

func (h *Handler) StaffTopup(c echo.Context) error { return h.topup(c) }

func (h *Handler) topup(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := topupID(c)
	if err != nil {
		return err
	}
	topup, err := h.topups.Topup(c.Request().Context(), scope, principal, id)
	if err != nil {
		return topupError(err)
	}
	return c.JSON(http.StatusOK, topupResponseOf(topup))
}

func (h *Handler) StaffTopups(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	topups, total, minimum, err := h.topups.StaffTopups(c.Request().Context(), scope, principal, service.TopupFilter{
		Status: c.QueryParam(orderStatusParam), Search: c.QueryParam("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return topupError(err)
	}
	return c.JSON(http.StatusOK, topupsOf(topups, total, minimum))
}

func (h *Handler) ApproveTopup(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := topupID(c)
	if err != nil {
		return err
	}
	topup, err := h.topups.Approve(c.Request().Context(), scope, principal, id)
	if err != nil {
		return topupError(err)
	}
	return c.JSON(http.StatusOK, topupResponseOf(topup))
}

func (h *Handler) RejectTopup(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := topupID(c)
	if err != nil {
		return err
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, topupMaxBody)
	var req topupRejectRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	topup, err := h.topups.Reject(c.Request().Context(), scope, principal, id, req.Reason)
	if err != nil {
		return topupError(err)
	}
	return c.JSON(http.StatusOK, topupResponseOf(topup))
}

// topupID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func topupID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(topupNotFound)
	}
	return id, nil
}

func topupResponseOf(t model.Topup) topupResponse {
	return topupResponse{
		ID: t.ID, Code: service.TopupCode(t.Number), Status: t.Status, Method: t.Method, StudentID: t.UserID,
		StudentName: t.StudentName, Amount: t.Amount, ProofAssetID: t.ProofAssetID, RejectReason: t.RejectReason,
		CreatedAt: t.CreatedAt, ApprovedAt: t.ApprovedAt, RejectedAt: t.RejectedAt, RedirectURL: t.RedirectURL,
	}
}

func topupsOf(topups []model.Topup, total int, minimum string) topupsResponse {
	body := topupsResponse{Topups: make([]topupResponse, 0, len(topups)), Total: total, MinTopup: minimum}
	for _, t := range topups {
		body.Topups = append(body.Topups, topupResponseOf(t))
	}
	return body
}

func topupError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = topupReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrTopupNotFound):
		return httpx.NotFound(topupNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrTopupForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrTopupState):
		return httpx.Conflict(topupStateRule)
	case errors.Is(err, service.ErrTopupKeyConflict):
		return httpx.Conflict(topupKeyConflict)
	}
	return gatewayError(err)
}

func topupReason(reason string) string {
	switch reason {
	case service.ReasonTopupAmount:
		return topupAmountRule
	case service.ReasonTopupKey:
		return topupKeyRule
	case service.ReasonTopupProof:
		return topupProofRule
	case service.ReasonTopupReason:
		return topupRejectRule
	case service.ReasonTopupQuery:
		return topupQueryRule
	case service.ReasonTopupMethod:
		return topupMethodRule
	}
	return invalidFormat
}
