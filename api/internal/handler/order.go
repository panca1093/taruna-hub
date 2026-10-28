package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	orderNotFound     = "Pesanan tidak ditemukan."
	orderProductGone  = "Produk tidak tersedia untuk dibeli."
	orderAlreadyOwned = "Kamu sudah memiliki tryout ini dan aksesnya masih aktif."
	orderInsufficient = "Saldo tidak mencukupi untuk paket ini."
	orderPriceChanged = "Harga paket berubah. Periksa harga terbaru lalu coba lagi."
	orderPending      = "Kamu masih punya pesanan untuk paket ini yang menunggu konfirmasi."
	orderPriceRule    = "Harga yang ditampilkan wajib diisi sebagai angka desimal, maksimal dua digit di belakang koma."
	orderStateRule    = "Pesanan sudah diproses dan tidak bisa diubah."
	orderKeyConflict  = "Kunci idempotensi sudah dipakai untuk pesanan lain."
	orderBankMissing  = "Transfer manual belum tersedia."
	orderProductRule  = "Produk wajib dipilih."
	orderMethodRule   = "Metode pembayaran harus manual atau wallet."
	orderKeyRule      = "Kunci idempotensi 8–64 karakter: huruf, angka, _ atau -."
	orderProofRule    = "Bukti transfer wajib untuk transfer manual dan harus berkas milikmu yang belum dipakai."
	orderCancelRule   = "Alasan pembatalan wajib diisi, maksimal 500 karakter."
	orderQueryRule    = "Parameter tidak valid."
	orderMaxBody      = 8 << 10
	orderStatusParam  = "status"
)

type orderItemResponse struct {
	TryoutID uuid.UUID `json:"tryout_id"`
	Title    string    `json:"title"`
}

type orderResponse struct {
	ID            uuid.UUID           `json:"id"`
	Code          string              `json:"code"`
	Status        string              `json:"status"`
	PaymentMethod string              `json:"payment_method"`
	StudentID     uuid.UUID           `json:"student_id"`
	StudentName   string              `json:"student_name"`
	ProductID     uuid.UUID           `json:"product_id"`
	ProductType   string              `json:"product_type"`
	ProductTitle  string              `json:"product_title"`
	Price         string              `json:"price"`
	Items         []orderItemResponse `json:"items"`
	ProofAssetID  *uuid.UUID          `json:"proof_asset_id"`
	CancelReason  string              `json:"cancel_reason"`
	CreatedAt     time.Time           `json:"created_at"`
	ConfirmedAt   *time.Time          `json:"confirmed_at"`
	CancelledAt   *time.Time          `json:"cancelled_at"`
}

type ordersResponse struct {
	Orders []orderResponse `json:"orders"`
	Total  int             `json:"total"`
}

type checkoutRequest struct {
	ProductID      string  `json:"product_id"`
	PaymentMethod  string  `json:"payment_method"`
	IdempotencyKey string  `json:"idempotency_key"`
	ProofAssetID   *string `json:"proof_asset_id"`
	ExpectedPrice  string  `json:"expected_price"`
}

type cancelRequest struct {
	Reason string `json:"reason"`
}

type enrollmentResponse struct {
	ID           uuid.UUID  `json:"id"`
	TryoutID     uuid.UUID  `json:"tryout_id"`
	TryoutTitle  string     `json:"tryout_title"`
	ProgramCode  string     `json:"program_code"`
	ProgramName  string     `json:"program_name"`
	SubtestCount int        `json:"subtest_count"`
	ActiveFrom   *time.Time `json:"active_from"`
	ActiveUntil  time.Time  `json:"active_until"`
	CreatedAt    time.Time  `json:"created_at"`
}

type enrollmentsResponse struct {
	Enrollments []enrollmentResponse `json:"enrollments"`
}

type bankTransferResponse struct {
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

// Checkout answers 201 for a new order and 200 when the same idempotency key finds its earlier one.
func (h *Handler) Checkout(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, orderMaxBody)
	var req checkoutRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input, err := checkoutInputOf(req)
	if err != nil {
		return err
	}
	order, replayed, err := h.orders.Checkout(c.Request().Context(), scope, principal, input)
	if err != nil {
		return orderError(err)
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	return c.JSON(status, orderResponseOf(order))
}

func (h *Handler) MyOrders(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	orders, total, err := h.orders.Orders(c.Request().Context(), scope, principal, limit, offset)
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, ordersOf(orders, total))
}

// MyOrder and StaffOrder share one service read: the service scopes a student to their own orders.
func (h *Handler) MyOrder(c echo.Context) error { return h.order(c) }

func (h *Handler) StaffOrder(c echo.Context) error { return h.order(c) }

func (h *Handler) order(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := orderID(c)
	if err != nil {
		return err
	}
	order, err := h.orders.Order(c.Request().Context(), scope, principal, id)
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, orderResponseOf(order))
}

func (h *Handler) StaffOrders(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	orders, total, err := h.orders.StaffOrders(c.Request().Context(), scope, principal, service.OrderFilter{
		Status: c.QueryParam(orderStatusParam), Search: c.QueryParam("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, ordersOf(orders, total))
}

func (h *Handler) ConfirmOrder(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := orderID(c)
	if err != nil {
		return err
	}
	order, err := h.orders.Confirm(c.Request().Context(), scope, principal, id)
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, orderResponseOf(order))
}

func (h *Handler) CancelOrder(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := orderID(c)
	if err != nil {
		return err
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, orderMaxBody)
	var req cancelRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	order, err := h.orders.Cancel(c.Request().Context(), scope, principal, id, req.Reason)
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, orderResponseOf(order))
}

func (h *Handler) MyEnrollments(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	enrollments, err := h.orders.Enrollments(c.Request().Context(), scope, principal)
	if err != nil {
		return orderError(err)
	}
	body := enrollmentsResponse{Enrollments: make([]enrollmentResponse, 0, len(enrollments))}
	for _, e := range enrollments {
		body.Enrollments = append(body.Enrollments, enrollmentResponse{
			ID: e.ID, TryoutID: e.TryoutID, TryoutTitle: e.TryoutTitle, ProgramCode: e.ProgramCode,
			ProgramName: e.ProgramName, SubtestCount: e.SubtestCount, ActiveFrom: e.TryoutActiveFrom,
			ActiveUntil: e.TryoutActiveUntil, CreatedAt: e.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) BankTransfer(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	bank, err := h.orders.BankTransfer(scope, principal)
	if err != nil {
		return orderError(err)
	}
	return c.JSON(http.StatusOK, bankTransferResponse{
		BankName: bank.Bank, AccountNumber: bank.Number, AccountHolder: bank.Holder,
	})
}

// orderID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func orderID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(orderNotFound)
	}
	return id, nil
}

func orderPageOf(c echo.Context) (int, int, error) {
	fields := map[string]string{}
	limit, offset := 0, 0
	for name, target := range map[string]*int{"limit": &limit, "offset": &offset} {
		raw := c.QueryParam(name)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			fields[name] = invalidFormat
			continue
		}
		*target = value
	}
	if len(fields) > 0 {
		return 0, 0, httpx.Validation(incomplete, fields)
	}
	return limit, offset, nil
}

func checkoutInputOf(req checkoutRequest) (service.CheckoutInput, error) {
	fields := map[string]string{}
	input := service.CheckoutInput{Method: req.PaymentMethod, Key: req.IdempotencyKey, ExpectedPrice: req.ExpectedPrice}
	if id, err := uuid.Parse(req.ProductID); err != nil {
		fields["product_id"] = invalidFormat
	} else {
		input.ProductID = id
	}
	if req.ProofAssetID != nil && strings.TrimSpace(*req.ProofAssetID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ProofAssetID))
		if err != nil {
			fields["proof_asset_id"] = invalidFormat
		} else {
			input.ProofAssetID = &id
		}
	}
	if len(fields) > 0 {
		return service.CheckoutInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

func orderResponseOf(o model.Order) orderResponse {
	out := orderResponse{
		ID: o.ID, Code: service.OrderCode(o.Number), Status: o.Status, PaymentMethod: o.PaymentMethod,
		StudentID: o.UserID, StudentName: o.StudentName, ProductID: o.ProductID, ProductType: o.ProductType,
		ProductTitle: o.ProductTitle, Price: o.Price, Items: make([]orderItemResponse, 0, len(o.Items)),
		ProofAssetID: o.ProofAssetID, CancelReason: o.CancelReason, CreatedAt: o.CreatedAt,
		ConfirmedAt: o.ConfirmedAt, CancelledAt: o.CancelledAt,
	}
	for _, item := range o.Items {
		out.Items = append(out.Items, orderItemResponse{TryoutID: item.TryoutID, Title: item.Title})
	}
	return out
}

func ordersOf(orders []model.Order, total int) ordersResponse {
	body := ordersResponse{Orders: make([]orderResponse, 0, len(orders)), Total: total}
	for _, o := range orders {
		body.Orders = append(body.Orders, orderResponseOf(o))
	}
	return body
}

func orderError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = orderReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrOrderNotFound):
		return httpx.NotFound(orderNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrOrderForbidden):
		return httpx.Forbidden(roleDenied)
	case errors.Is(err, service.ErrProductUnavailable):
		return httpx.NotFound(orderProductGone)
	case errors.Is(err, service.ErrAlreadyOwned):
		return httpx.New(http.StatusConflict, httpx.CodeAlreadyOwned, orderAlreadyOwned)
	case errors.Is(err, service.ErrPriceChanged):
		return httpx.New(http.StatusConflict, httpx.CodePriceChanged, orderPriceChanged)
	case errors.Is(err, service.ErrOrderPending):
		return httpx.New(http.StatusConflict, httpx.CodeOrderPending, orderPending)
	case errors.Is(err, service.ErrInsufficientBalance):
		return httpx.New(http.StatusUnprocessableEntity, httpx.CodeInsufficientBalance, orderInsufficient)
	case errors.Is(err, service.ErrOrderState):
		return httpx.Conflict(orderStateRule)
	case errors.Is(err, service.ErrOrderKeyConflict):
		return httpx.Conflict(orderKeyConflict)
	case errors.Is(err, service.ErrBankTransferUnavailable):
		return httpx.Unavailable(orderBankMissing)
	}
	return err
}

func orderReason(reason string) string {
	switch reason {
	case service.ReasonOrderProduct:
		return orderProductRule
	case service.ReasonOrderMethod:
		return orderMethodRule
	case service.ReasonOrderKey:
		return orderKeyRule
	case service.ReasonOrderProof:
		return orderProofRule
	case service.ReasonOrderPrice:
		return orderPriceRule
	case service.ReasonOrderReason:
		return orderCancelRule
	case service.ReasonOrderQuery:
		return orderQueryRule
	}
	return invalidFormat
}
