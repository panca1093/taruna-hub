package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	gatewayMissing      = "Pembayaran Midtrans belum tersedia."
	gatewayFailed       = "Midtrans tidak dapat dihubungi. Coba lagi dengan permintaan yang sama; jangan membuat pesanan baru."
	gatewayCancelled    = "Pembayaran tidak dapat dibuka dan dibatalkan. Silakan coba lagi."
	exceptionNotFound   = "Tinjauan pembayaran tidak ditemukan."
	exceptionStateRule  = "Tinjauan pembayaran sudah diselesaikan dengan cara lain."
	exceptionReasonRule = "Alasan wajib diisi, maksimal 500 karakter."
	notificationMaxBody = 64 << 10
)

// notificationBody is the part of Midtrans' notification this service reads; everything else stays in the evidence.
type notificationBody struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	TransactionID     string `json:"transaction_id"`
}

type paymentExceptionRef struct {
	ID   uuid.UUID `json:"id"`
	Kind string    `json:"kind"`
}

type paymentExceptionResponse struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	StudentID   uuid.UUID  `json:"student_id"`
	StudentName string     `json:"student_name"`
	OrderID     *uuid.UUID `json:"order_id"`
	OrderCode   *string    `json:"order_code"`
	TopupID     *uuid.UUID `json:"topup_id"`
	TopupCode   *string    `json:"topup_code"`
	PaidAmount  string     `json:"paid_amount"`
	Reason      string     `json:"reason"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

type paymentExceptionsResponse struct {
	Exceptions []paymentExceptionResponse `json:"exceptions"`
	Total      int                        `json:"total"`
}

type gatewayMethodResponse struct {
	Provider string `json:"provider"`
}

// MidtransNotification is provider-authenticated: no session, no tenant host. The signature is the only credential,
// and the tenant and entity come from the stored reference. Unknown references are answered 200 so the provider
// stops retrying; a failure to confirm with the provider is a 5xx so it retries.
func (h *Handler) MidtransNotification(c echo.Context) error {
	if h.payments == nil {
		return httpx.Unavailable(gatewayMissing)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, notificationMaxBody)
	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return httpx.BadRequest(incomplete)
	}
	var body notificationBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return httpx.BadRequest(incomplete)
	}
	err = h.payments.HandleNotification(c.Request().Context(), service.Notification{
		ProviderStatus: service.ProviderStatus{
			OrderID: body.OrderID, TransactionID: body.TransactionID, StatusCode: body.StatusCode,
			GrossAmount: body.GrossAmount, TransactionStatus: body.TransactionStatus, FraudStatus: body.FraudStatus,
		},
		SignatureKey: body.SignatureKey,
	}, raw)
	switch {
	case err == nil, errors.Is(err, service.ErrPaymentUnknown):
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, service.ErrPaymentSignature):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrGatewayUnavailable):
		return httpx.Unavailable(gatewayMissing)
	}
	return err
}

// GatewayMethod tells the checkout and wallet screens whether Midtrans is offered.
func (h *Handler) GatewayMethod(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	if err := h.payments.GatewayMethod(scope, principal); err != nil {
		return gatewayError(err)
	}
	return c.JSON(http.StatusOK, gatewayMethodResponse{Provider: model.ProviderMidtrans})
}

func (h *Handler) PaymentExceptions(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	list, total, err := h.payments.Exceptions(c.Request().Context(), scope, principal, c.QueryParam(orderStatusParam), limit, offset)
	if err != nil {
		return gatewayError(err)
	}
	body := paymentExceptionsResponse{Exceptions: make([]paymentExceptionResponse, 0, len(list)), Total: total}
	for _, x := range list {
		body.Exceptions = append(body.Exceptions, paymentExceptionOf(x))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) CreditPaymentException(c echo.Context) error {
	return h.resolveException(c, h.payments.Credit)
}

func (h *Handler) RefundPaymentException(c echo.Context) error {
	return h.resolveException(c, h.payments.MarkRefunded)
}

type resolveFunc func(ctx context.Context, scope service.HostScope, principal model.Principal, id uuid.UUID, reason string) (model.PaymentException, error)

func (h *Handler) resolveException(c echo.Context, resolve resolveFunc) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(exceptionNotFound)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, orderMaxBody)
	var req cancelRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	x, err := resolve(c.Request().Context(), scope, principal, id, req.Reason)
	if err != nil {
		return gatewayError(err)
	}
	return c.JSON(http.StatusOK, paymentExceptionOf(x))
}

func paymentExceptionOf(x model.PaymentException) paymentExceptionResponse {
	out := paymentExceptionResponse{
		ID: x.ID, Kind: x.Kind, Status: x.Status, StudentID: x.UserID, StudentName: x.StudentName,
		OrderID: x.OrderID, TopupID: x.TopupID, PaidAmount: x.PaidAmount, Reason: x.ResolveReason,
		CreatedAt: x.CreatedAt, ResolvedAt: x.ResolvedAt,
	}
	if x.OrderNumber != nil {
		code := service.OrderCode(*x.OrderNumber)
		out.OrderCode = &code
	}
	if x.TopupNumber != nil {
		code := service.TopupCode(*x.TopupNumber)
		out.TopupCode = &code
	}
	return out
}

// gatewayError maps the payment errors both checkout and top-up can return; anything else passes through.
func gatewayError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			if reason == service.ReasonExceptionReason {
				fields[path] = exceptionReasonRule
			} else {
				fields[path] = orderReason(reason)
			}
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrGatewayUnavailable):
		return httpx.Unavailable(gatewayMissing)
	case errors.Is(err, service.ErrGatewayAmbiguous):
		return httpx.BadGateway(gatewayFailed)
	case errors.Is(err, service.ErrPaymentCreateFailed):
		return httpx.BadGateway(gatewayCancelled)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrExceptionNotFound):
		return httpx.NotFound(exceptionNotFound)
	case errors.Is(err, service.ErrExceptionForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrExceptionState):
		return httpx.Conflict(exceptionStateRule)
	}
	return err
}
