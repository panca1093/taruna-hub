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
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	walletNotFound    = "Dompet siswa tidak ditemukan."
	walletInsuff      = "Saldo tidak cukup."
	walletAmountRule  = "Jumlah harus angka lebih dari 0 dengan maksimal 2 desimal."
	walletTypeRule    = "Tipe harus credit atau debit."
	walletReasonRule  = "Alasan wajib diisi."
	walletKeyRule     = "Kunci idempotensi 8–64 karakter: huruf, angka, _ atau -."
	walletKeyConflict = "Kunci idempotensi sudah dipakai untuk permintaan lain."
	walletQueryRule   = "Batas 1–100."
)

type walletResponse struct {
	Balance      string `json:"balance"`
	TotalCredits string `json:"total_credits"`
	TotalDebits  string `json:"total_debits"`
}

type walletEntryResponse struct {
	ID           uuid.UUID `json:"id"`
	Seq          int64     `json:"seq"`
	Kind         string    `json:"kind"`
	Amount       string    `json:"amount"`
	BalanceAfter string    `json:"balance_after"`
	ActorKind    string    `json:"actor_kind"`
	ActorName    *string   `json:"actor_name"`
	Reason       string    `json:"reason"`
	SourceType   string    `json:"source_type"`
	SourceID     string    `json:"source_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type walletEntriesResponse struct {
	Entries []walletEntryResponse `json:"entries"`
	Total   int                   `json:"total"`
}

type walletStudentResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	Balance   string    `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type walletStudentsResponse struct {
	Students []walletStudentResponse `json:"students"`
	Total    int                     `json:"total"`
}

type adjustmentRequest struct {
	Type           string `json:"type"`
	Amount         string `json:"amount"`
	Reason         string `json:"reason"`
	IdempotencyKey string `json:"idempotency_key"`
}

type adjustmentResponse struct {
	Entry   walletEntryResponse `json:"entry"`
	Balance string              `json:"balance"`
}

// Wallet answers the signed-in student's own projection and ledger totals.
func (h *Handler) Wallet(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	wallet, err := h.wallets.Wallet(c.Request().Context(), scope, principal)
	if err != nil {
		return walletError(err)
	}
	return c.JSON(http.StatusOK, walletResponseOf(wallet))
}

func (h *Handler) WalletEntries(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := walletPageOf(c)
	if err != nil {
		return err
	}
	entries, total, err := h.wallets.Entries(c.Request().Context(), scope, principal, limit, offset)
	if err != nil {
		return walletError(err)
	}
	return c.JSON(http.StatusOK, walletEntriesOf(entries, total))
}

func (h *Handler) ListStudents(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := walletPageOf(c)
	if err != nil {
		return err
	}
	students, total, err := h.wallets.Students(c.Request().Context(), scope, principal, repository.WalletFilter{
		Search: c.QueryParam("q"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return walletError(err)
	}
	body := walletStudentsResponse{Students: make([]walletStudentResponse, 0, len(students)), Total: total}
	for _, student := range students {
		body.Students = append(body.Students, walletStudentResponse{
			ID: student.ID, Name: student.Name, Email: student.Email, Phone: student.Phone,
			Balance: student.Balance, CreatedAt: student.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StudentWallet(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := walletStudentID(c)
	if err != nil {
		return err
	}
	wallet, err := h.wallets.StudentWallet(c.Request().Context(), scope, principal, id)
	if err != nil {
		return walletError(err)
	}
	return c.JSON(http.StatusOK, walletResponseOf(wallet))
}

func (h *Handler) StudentWalletEntries(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := walletStudentID(c)
	if err != nil {
		return err
	}
	limit, offset, err := walletPageOf(c)
	if err != nil {
		return err
	}
	entries, total, err := h.wallets.StudentEntries(c.Request().Context(), scope, principal, id, limit, offset)
	if err != nil {
		return walletError(err)
	}
	return c.JSON(http.StatusOK, walletEntriesOf(entries, total))
}

// The body is capped before it is mapped, so an oversized adjustment is one error, not thousands.
const maxAdjustmentBody = 8 << 10

func bindAdjustment(c echo.Context, dst any) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxAdjustmentBody)
	return bindStrict(c, dst)
}

func (h *Handler) AdjustStudentWallet(c echo.Context) error {
	scope, principal, ok := walletCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := walletStudentID(c)
	if err != nil {
		return err
	}
	var req adjustmentRequest
	if err := bindAdjustment(c, &req); err != nil {
		return err
	}
	entry, replayed, wallet, err := h.wallets.Adjust(c.Request().Context(), scope, principal, id, service.AdjustmentInput{
		Type: req.Type, Amount: req.Amount, Reason: req.Reason, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return walletError(err)
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	return c.JSON(status, adjustmentResponse{Entry: walletEntryOf(entry), Balance: wallet.Balance})
}

func walletCaller(c echo.Context) (service.HostScope, model.Principal, bool) {
	scope, scoped := hostScope(c)
	principal, signedIn := PrincipalFrom(c)
	return scope, principal, scoped && signedIn
}

// walletStudentID is a 404 for anything that is not a uuid, never a 400 that would confirm a shape.
func walletStudentID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, httpx.NotFound(walletNotFound)
	}
	return id, nil
}

func walletPageOf(c echo.Context) (int, int, error) {
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

func walletResponseOf(wallet model.Wallet) walletResponse {
	return walletResponse{
		Balance: wallet.Balance, TotalCredits: wallet.TotalCredits, TotalDebits: wallet.TotalDebits,
	}
}

func walletEntryOf(entry model.WalletEntry) walletEntryResponse {
	return walletEntryResponse{
		ID: entry.ID, Seq: entry.Seq, Kind: entry.Kind, Amount: entry.Amount,
		BalanceAfter: entry.BalanceAfter, ActorKind: entry.ActorKind, ActorName: entry.ActorName,
		Reason: entry.Reason, SourceType: entry.SourceType, SourceID: entry.SourceID,
		CreatedAt: entry.CreatedAt,
	}
}

func walletEntriesOf(entries []model.WalletEntry, total int) walletEntriesResponse {
	body := walletEntriesResponse{Entries: make([]walletEntryResponse, 0, len(entries)), Total: total}
	for _, entry := range entries {
		body.Entries = append(body.Entries, walletEntryOf(entry))
	}
	return body
}

func walletError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = walletReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrWalletNotFound):
		return httpx.NotFound(walletNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrInsufficientBalance):
		return httpx.Validation(walletInsuff, map[string]string{"amount": walletInsuff})
	case errors.Is(err, service.ErrWalletConflict):
		return httpx.Conflict(walletKeyConflict)
	}
	return err
}

func walletReason(reason string) string {
	switch reason {
	case service.ReasonWalletAmount:
		return walletAmountRule
	case service.ReasonWalletType:
		return walletTypeRule
	case service.ReasonWalletReason:
		return walletReasonRule
	case service.ReasonWalletKey:
		return walletKeyRule
	case service.ReasonWalletQuery:
		return walletQueryRule
	}
	return invalidFormat
}
