package service

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	// ErrGatewayUnavailable means no merchant keys are configured, so the gateway method is not offered.
	ErrGatewayUnavailable = errors.New("payment gateway is not configured")
	// ErrGatewayAmbiguous means a create call may or may not have reached the provider; retry with the same key.
	ErrGatewayAmbiguous = errors.New("payment gateway did not answer")
	// ErrGatewayDuplicate, ErrGatewayRejected and ErrGatewayNotFound are the provider's definite answers.
	ErrGatewayDuplicate = errors.New("provider order id already exists")
	ErrGatewayRejected  = errors.New("provider rejected the transaction")
	ErrGatewayNotFound  = errors.New("provider has no such transaction")
	// ErrPaymentCreateFailed means the payment could not be opened and the order or top-up was cancelled.
	ErrPaymentCreateFailed = errors.New("payment could not be created")

	ErrPaymentSignature   = errors.New("payment notification signature is invalid")
	ErrPaymentUnknown     = errors.New("payment notification names no known reference")
	ErrExceptionNotFound  = errors.New("payment exception not found")
	ErrExceptionForbidden = errors.New("payment exception action not allowed for this role")
	ErrExceptionState     = errors.New("payment exception was resolved the other way")
)

const ReasonExceptionReason = "exception_reason"

const (
	gatewayExpiryMinutes = 1440
	reconcileInterval    = 10 * time.Second
	maxItemNameLen       = 50
)

// ProviderStatus is the provider's view of one transaction, from a notification body or the Status API.
type ProviderStatus struct {
	OrderID           string
	TransactionID     string
	StatusCode        string
	GrossAmount       string
	TransactionStatus string
	FraudStatus       string
	Raw               []byte
}

// Notification is a webhook body with the signature it carried.
type Notification struct {
	ProviderStatus
	SignatureKey string
}

// SnapRequest is everything the provider needs to open a payment page; Amount is canonical whole rupiah.
type SnapRequest struct {
	ProviderOrderID string
	Amount          string
	ItemName        string
	CustomerName    string
	Email           string
	Phone           string
	FinishURL       string
	ExpiryMinutes   int
}

// PaymentGateway is Midtrans Snap behind an interface so tests never touch the network.
type PaymentGateway interface {
	Configured() bool
	// CreateTransaction returns the redirect URL; the errors above classify a failure.
	CreateTransaction(ctx context.Context, req SnapRequest) (string, error)
	Status(ctx context.Context, providerOrderID string) (ProviderStatus, error)
	VerifySignature(n Notification) bool
}

// MidtransSignature is SHA512(order_id + status_code + gross_amount + server_key), hex.
func MidtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

// SignatureMatches compares in constant time.
func SignatureMatches(want, got string) bool {
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(want)), []byte(strings.ToLower(got))) == 1
}

type PaymentOption func(*PaymentService)

func WithPaymentClock(now func() time.Time) PaymentOption {
	return func(s *PaymentService) { s.now = now }
}

// PaymentService owns attempts and provider evidence. Money and access still move only through
// WalletService.Apply and the order service's single issuance path; this service decides when.
type PaymentService struct {
	db      *pgxpool.Pool
	repo    *repository.PaymentRepository
	orders  *OrderService
	topups  *TopupService
	wallets *WalletService
	gateway PaymentGateway
	appURL  string
	now     func() time.Time
}

// NewPaymentService wires itself into the order and top-up services, which need it for gateway checkout.
func NewPaymentService(db *pgxpool.Pool, repo *repository.PaymentRepository, orders *OrderService, topups *TopupService, wallets *WalletService, gateway PaymentGateway, appURL string, opts ...PaymentOption) *PaymentService {
	s := &PaymentService{db: db, repo: repo, orders: orders, topups: topups, wallets: wallets, gateway: gateway, appURL: appURL, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	orders.payments = s
	topups.payments = s
	return s
}

// Available reports whether the gateway method can be offered.
func (s *PaymentService) Available() bool {
	return s != nil && s.gateway != nil && s.gateway.Configured()
}

// GatewayMethod is the student-facing availability check.
func (s *PaymentService) GatewayMethod(scope HostScope, principal model.Principal) error {
	if _, err := walletTenant(scope, principal); err != nil {
		return err
	}
	if !s.Available() {
		return ErrGatewayUnavailable
	}
	return nil
}

func newProviderOrderID() string { return "TE-" + strings.ReplaceAll(uuid.NewString(), "-", "") }

// insertAttemptTx records the payment in the transaction that creates the order or top-up, so a business
// operation never exists without the provider identity its retries reuse.
func (s *PaymentService) insertAttemptTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, orderID, topupID *uuid.UUID, amount string) error {
	_, err := s.repo.InsertAttemptTx(ctx, tx, tenantID, model.PaymentAttempt{
		UserID: userID, ProviderOrderID: newProviderOrderID(), OrderID: orderID, TopupID: topupID, Amount: amount,
		ExpiresAt: s.now().Add(gatewayExpiryMinutes * time.Minute),
	})
	return err
}

// ensureRedirect opens the provider's payment page for an attempt that has none yet. It runs outside any
// transaction. A retry reuses the stored provider order id, so an ambiguous earlier create never becomes a second
// payment: the provider answers "duplicate" and, since the payer never saw that page, the attempt is cancelled.
func (s *PaymentService) ensureRedirect(ctx context.Context, a model.PaymentAttempt, item, finishPath string) error {
	if a.RedirectURL != nil || a.Status != model.AttemptPending {
		return nil
	}
	contact, err := s.repo.ContactOf(ctx, a.TenantID, a.UserID)
	if err != nil {
		return err
	}
	req := SnapRequest{
		ProviderOrderID: a.ProviderOrderID, Amount: a.Amount, ItemName: truncateRunes(item, maxItemNameLen),
		CustomerName: contact.Name, FinishURL: s.appURL + finishPath, ExpiryMinutes: gatewayExpiryMinutes,
	}
	if contact.Email != nil {
		req.Email = *contact.Email
	}
	if contact.Phone != nil {
		req.Phone = *contact.Phone
	}
	redirect, err := s.gateway.CreateTransaction(ctx, req)
	switch {
	case err == nil:
		return s.repo.SetRedirect(ctx, a.TenantID, a.ID, redirect)
	case errors.Is(err, ErrGatewayDuplicate):
		st, serr := s.gateway.Status(ctx, a.ProviderOrderID)
		switch {
		case serr == nil:
			return s.process(ctx, a, st, evidence{source: "status", st: st, raw: st.Raw})
		case errors.Is(serr, ErrGatewayNotFound):
			return s.abandon(ctx, a)
		}
		return fmt.Errorf("%w: %v", ErrGatewayAmbiguous, serr)
	case errors.Is(err, ErrGatewayRejected):
		return s.abandon(ctx, a)
	}
	return fmt.Errorf("%w: %v", ErrGatewayAmbiguous, err)
}

// abandon cancels the order or top-up whose payment page can never be opened.
func (s *PaymentService) abandon(ctx context.Context, a model.PaymentAttempt) error {
	if err := s.process(ctx, a, ProviderStatus{OrderID: a.ProviderOrderID, TransactionStatus: "cancel"}); err != nil {
		return err
	}
	return ErrPaymentCreateFailed
}

// EnsureOrderRedirect opens the payment page of a gateway order and returns the order as it now reads.
func (s *PaymentService) ensureOrderRedirect(ctx context.Context, order model.Order) (model.Order, error) {
	a, err := s.repo.AttemptForOrder(ctx, order.TenantID, order.ID)
	if err != nil {
		return order, err
	}
	err = s.ensureRedirect(ctx, a, order.ProductTitle, "/student/orders")
	reloaded, rerr := s.orders.repo.Find(ctx, order.TenantID, order.ID)
	if rerr != nil {
		return order, rerr
	}
	return reloaded, err
}

func (s *PaymentService) ensureTopupRedirect(ctx context.Context, topup model.Topup) (model.Topup, error) {
	a, err := s.repo.AttemptForTopup(ctx, topup.TenantID, topup.ID)
	if err != nil {
		return topup, err
	}
	err = s.ensureRedirect(ctx, a, "Top up saldo "+TopupCode(topup.Number), "/student/wallet")
	reloaded, rerr := s.topups.repo.Find(ctx, topup.TenantID, topup.ID)
	if rerr != nil {
		return topup, rerr
	}
	return reloaded, err
}

// reconcileOrder and reconcileTopup refresh a pending gateway payment when its owner reads it. They never
// fail the read: the notification path stays the primary one.
func (s *PaymentService) reconcileOrder(ctx context.Context, order model.Order) {
	if order.PaymentMethod != model.PaymentGateway || order.Status != model.OrderPending || !s.Available() {
		return
	}
	if a, err := s.repo.AttemptForOrder(ctx, order.TenantID, order.ID); err == nil {
		_ = s.reconcile(ctx, a)
	}
}

func (s *PaymentService) reconcileTopup(ctx context.Context, topup model.Topup) {
	if topup.Method != model.TopupGateway || topup.Status != model.TopupPending || !s.Available() {
		return
	}
	if a, err := s.repo.AttemptForTopup(ctx, topup.TenantID, topup.ID); err == nil {
		_ = s.reconcile(ctx, a)
	}
}

func (s *PaymentService) reconcile(ctx context.Context, a model.PaymentAttempt) error {
	if a.Status != model.AttemptPending && a.Status != model.AttemptFailed {
		return nil
	}
	if a.LastCheckedAt != nil && s.now().Sub(*a.LastCheckedAt) < reconcileInterval {
		return nil
	}
	if err := s.repo.MarkChecked(ctx, a.TenantID, a.ID, s.now()); err != nil {
		return err
	}
	st, err := s.gateway.Status(ctx, a.ProviderOrderID)
	if errors.Is(err, ErrGatewayNotFound) {
		if s.now().Before(a.ExpiresAt) {
			return nil
		}
		expired := ProviderStatus{OrderID: a.ProviderOrderID, TransactionStatus: "expire"}
		return s.process(ctx, a, expired)
	}
	if err != nil {
		return err
	}
	return s.process(ctx, a, st, evidence{source: "status", st: st, raw: st.Raw})
}

// HandleNotification is the webhook. The signature gates everything, the tenant and entity come from the stored
// reference, and the money moves only on what the Status API confirms. An error means "retry"; the two sentinels
// are answered by the transport.
func (s *PaymentService) HandleNotification(ctx context.Context, n Notification, raw []byte) error {
	if !s.Available() {
		return ErrGatewayUnavailable
	}
	if n.OrderID == "" || !s.gateway.VerifySignature(n) {
		return ErrPaymentSignature
	}
	a, err := s.repo.AttemptByProviderOrder(ctx, n.OrderID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrPaymentUnknown
	}
	if err != nil {
		return err
	}
	st, err := s.gateway.Status(ctx, a.ProviderOrderID)
	if err != nil {
		return fmt.Errorf("confirm notification with the provider: %w", err)
	}
	notified := evidence{source: "notification", st: n.ProviderStatus, raw: raw}
	if st.OrderID != a.ProviderOrderID || !sameAmount(st.GrossAmount, n.GrossAmount) {
		return s.process(ctx, a, ProviderStatus{}, notified)
	}
	return s.process(ctx, a, st, notified, evidence{source: "status", st: st, raw: st.Raw})
}

func sameAmount(a, b string) bool {
	x, okx := parseAmount(a)
	y, oky := parseAmount(b)
	return okx && oky && x == y
}

type evidence struct {
	source string
	st     ProviderStatus
	raw    []byte
}

// process applies one verified provider status to an attempt in a single transaction and stores the evidence.
// A zero status is evidence that changed nothing.
func (s *PaymentService) process(ctx context.Context, a model.PaymentAttempt, st ProviderStatus, events ...evidence) error {
	name, err := s.repo.TenantName(ctx, a.TenantID)
	if err != nil {
		return err
	}
	scope := HostScope{Tenant: model.Tenant{ID: a.TenantID, Name: name}}
	return s.orders.inTx(ctx, func(tx pgx.Tx) error {
		attempt, err := s.repo.LockAttemptTx(ctx, tx, a.TenantID, a.ID)
		if err != nil {
			return err
		}
		outcome := "ignored"
		if st.TransactionStatus != "" {
			if outcome, err = s.apply(ctx, tx, scope, attempt, st); err != nil {
				return err
			}
		}
		for _, e := range events {
			sum := sha256.Sum256(e.raw)
			if err := s.repo.InsertEventTx(ctx, tx, model.PaymentEvent{
				AttemptID: attempt.ID, TenantID: attempt.TenantID, Source: e.source, Status: e.st.TransactionStatus,
				FraudStatus: e.st.FraudStatus, StatusCode: e.st.StatusCode, GrossAmount: e.st.GrossAmount,
				TransactionID: e.st.TransactionID, BodyHash: hex.EncodeToString(sum[:]), Raw: rawJSON(e.raw),
			}, outcome); err != nil {
				return err
			}
		}
		return nil
	})
}

func rawJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

const (
	kindSuccess  = "success"
	kindPending  = "pending"
	kindFailed   = "failed"
	kindCancel   = "cancelled"
	kindEvidence = "evidence"
)

func classifyProvider(st ProviderStatus) string {
	if st.FraudStatus == "deny" {
		return kindFailed
	}
	switch st.TransactionStatus {
	case "settlement":
		if st.FraudStatus == "challenge" {
			return kindPending
		}
		return kindSuccess
	case "capture":
		if st.FraudStatus == "accept" {
			return kindSuccess
		}
		return kindPending
	case "pending", "authorize":
		return kindPending
	case "deny", "failure":
		return kindFailed
	case "cancel", "expire":
		return kindCancel
	case "refund", "partial_refund", "chargeback", "partial_chargeback":
		return kindEvidence
	}
	return ""
}

// apply never downgrades: paid, held and cancelled are final, and only a success can leave failed.
func (s *PaymentService) apply(ctx context.Context, tx pgx.Tx, scope HostScope, a model.PaymentAttempt, st ProviderStatus) (string, error) {
	open := a.Status == model.AttemptPending || a.Status == model.AttemptFailed
	switch classifyProvider(st) {
	case kindSuccess:
		return s.applySuccess(ctx, tx, scope, a, st)
	case kindFailed:
		if a.Status == model.AttemptPending {
			return "applied", s.repo.SetAttemptStatusTx(ctx, tx, a.TenantID, a.ID, model.AttemptFailed)
		}
	case kindCancel:
		if open {
			return "applied", s.cancelTx(ctx, tx, scope, a)
		}
	case kindEvidence:
		return "evidence", nil
	}
	return "ignored", nil
}

const gatewayCancelReason = "Pembayaran Midtrans kedaluwarsa atau dibatalkan."

func (s *PaymentService) cancelTx(ctx context.Context, tx pgx.Tx, scope HostScope, a model.PaymentAttempt) error {
	if a.OrderID != nil {
		order, err := s.orders.repo.LockTx(ctx, tx, a.TenantID, *a.OrderID)
		if err != nil {
			return err
		}
		if order.Status == model.OrderPending {
			contact, err := s.orders.repo.LockStudentTx(ctx, tx, a.TenantID, order.UserID)
			if err != nil {
				return err
			}
			if err := s.repo.CancelOrderTx(ctx, tx, a.TenantID, order.ID, gatewayCancelReason); err != nil {
				return err
			}
			cancelled, err := s.orders.repo.FindTx(ctx, tx, a.TenantID, order.ID)
			if err != nil {
				return err
			}
			if err := s.orders.notify(ctx, tx, scope, cancelled, contact, TemplateOrderCancelled, gatewayCancelReason); err != nil {
				return err
			}
		}
	} else {
		topup, err := s.topups.repo.LockTx(ctx, tx, a.TenantID, *a.TopupID)
		if err != nil {
			return err
		}
		if topup.Status == model.TopupPending {
			contact, err := s.topups.repo.ContactOf(ctx, tx, a.TenantID, topup.UserID)
			if err != nil {
				return err
			}
			if err := s.repo.RejectTopupTx(ctx, tx, a.TenantID, topup.ID, gatewayCancelReason); err != nil {
				return err
			}
			rejected, err := s.topups.repo.FindTx(ctx, tx, a.TenantID, topup.ID)
			if err != nil {
				return err
			}
			if err := s.topups.notify(ctx, tx, scope, rejected, contact, TemplateTopupRejected, gatewayCancelReason); err != nil {
				return err
			}
		}
	}
	return s.repo.SetAttemptStatusTx(ctx, tx, a.TenantID, a.ID, model.AttemptCancelled)
}

func (s *PaymentService) applySuccess(ctx context.Context, tx pgx.Tx, scope HostScope, a model.PaymentAttempt, st ProviderStatus) (string, error) {
	if a.Status == model.AttemptPaid || a.Status == model.AttemptHeld {
		return "ignored", nil
	}
	paid, ok := parseAmount(st.GrossAmount)
	if !ok {
		return "ignored", nil
	}
	exception := ""
	if paid != a.Amount {
		exception = model.ExceptionAmountMismatch
	}
	if a.OrderID != nil {
		order, err := s.orders.repo.LockTx(ctx, tx, a.TenantID, *a.OrderID)
		if err != nil {
			return "", err
		}
		if exception == "" {
			switch order.Status {
			case model.OrderPending:
				contact, err := s.orders.repo.LockStudentTx(ctx, tx, a.TenantID, order.UserID)
				if err != nil {
					return "", err
				}
				if err := s.orders.repo.ConfirmTx(ctx, tx, a.TenantID, order.ID, nil); err != nil {
					return "", err
				}
				if err := s.orders.grantOrder(ctx, tx, order, model.ActorProvider, nil); err != nil {
					return "", err
				}
				confirmed, err := s.orders.repo.FindTx(ctx, tx, a.TenantID, order.ID)
				if err != nil {
					return "", err
				}
				if err := s.orders.notify(ctx, tx, scope, confirmed, contact, TemplateOrderConfirmed, ""); err != nil {
					return "", err
				}
				return "applied", s.repo.SetAttemptStatusTx(ctx, tx, a.TenantID, a.ID, model.AttemptPaid)
			case model.OrderCancelled:
				exception = model.ExceptionLateSuccess
			default:
				exception = model.ExceptionUnexpectedState
			}
		}
	} else {
		topup, err := s.topups.repo.LockTx(ctx, tx, a.TenantID, *a.TopupID)
		if err != nil {
			return "", err
		}
		if exception == "" {
			switch topup.Status {
			case model.TopupPending:
				contact, err := s.topups.repo.ContactOf(ctx, tx, a.TenantID, topup.UserID)
				if err != nil {
					return "", err
				}
				if err := s.repo.ApproveTopupTx(ctx, tx, a.TenantID, topup.ID); err != nil {
					return "", err
				}
				if _, _, err := s.wallets.Apply(ctx, tx, Movement{
					TenantID: a.TenantID, UserID: topup.UserID, Amount: topup.Amount, Kind: model.KindTopup,
					ActorKind: model.ActorProvider, Reason: "Top-up " + TopupCode(topup.Number) + " (Midtrans)",
					SourceType: model.WalletSourceTopup, SourceID: topup.ID.String(),
				}); err != nil {
					return "", err
				}
				approved, err := s.topups.repo.FindTx(ctx, tx, a.TenantID, topup.ID)
				if err != nil {
					return "", err
				}
				if err := s.topups.notify(ctx, tx, scope, approved, contact, TemplateTopupApproved, ""); err != nil {
					return "", err
				}
				return "applied", s.repo.SetAttemptStatusTx(ctx, tx, a.TenantID, a.ID, model.AttemptPaid)
			case model.TopupRejected:
				exception = model.ExceptionLateSuccess
			default:
				exception = model.ExceptionUnexpectedState
			}
		}
	}
	if err := s.repo.InsertExceptionTx(ctx, tx, a.TenantID, a, exception, paid); err != nil {
		return "", err
	}
	return "held", s.repo.SetAttemptStatusTx(ctx, tx, a.TenantID, a.ID, model.AttemptHeld)
}

// Exceptions lists held (or resolved) payment exceptions for staff.
func (s *PaymentService) Exceptions(ctx context.Context, scope HostScope, principal model.Principal, status string, limit, offset int) ([]model.PaymentException, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if !isStaff(principal.Role) {
		return nil, 0, ErrExceptionForbidden
	}
	limit, offset, err = checkedOrderPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	switch status {
	case "", model.ExceptionHeld, model.ExceptionCredited, model.ExceptionRefunded:
	default:
		return nil, 0, &InputError{Fields: map[string]string{"status": ReasonOrderQuery}}
	}
	return s.repo.ListExceptions(ctx, tenantID, status, limit, offset)
}

// Credit moves the held payment into the student's wallet, once. Repeating it changes nothing.
func (s *PaymentService) Credit(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, reason string) (model.PaymentException, error) {
	return s.resolve(ctx, scope, principal, id, reason, model.ExceptionCredited)
}

// MarkRefunded records that staff returned the money outside the system, with who and why.
func (s *PaymentService) MarkRefunded(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, reason string) (model.PaymentException, error) {
	return s.resolve(ctx, scope, principal, id, reason, model.ExceptionRefunded)
}

func (s *PaymentService) resolve(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, reason, target string) (model.PaymentException, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.PaymentException{}, err
	}
	if !isStaff(principal.Role) {
		return model.PaymentException{}, ErrExceptionForbidden
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 1 || n > maxCancelReasonLen {
		return model.PaymentException{}, &InputError{Fields: map[string]string{"reason": ReasonExceptionReason}}
	}
	var out model.PaymentException
	err = s.orders.inTx(ctx, func(tx pgx.Tx) error {
		x, err := s.repo.LockExceptionTx(ctx, tx, tenantID, id)
		if errors.Is(err, repository.ErrNotFound) {
			return ErrExceptionNotFound
		}
		if err != nil {
			return err
		}
		if x.Status != model.ExceptionHeld {
			if x.Status != target {
				return ErrExceptionState
			}
			out = x
			return nil
		}
		by := principal.UserID
		if target == model.ExceptionCredited {
			if _, _, err := s.wallets.Apply(ctx, tx, Movement{
				TenantID: tenantID, UserID: x.UserID, Amount: x.PaidAmount, Kind: model.KindTopup,
				ActorKind: model.ActorStaff, ActorID: &by, Reason: "Kredit pembayaran Midtrans: " + reason,
				SourceType: model.WalletSourceException, SourceID: x.ID.String(),
			}); err != nil {
				return err
			}
		}
		if err := s.repo.ResolveExceptionTx(ctx, tx, tenantID, id, target, by, reason); err != nil {
			return err
		}
		out, err = s.repo.FindExceptionTx(ctx, tx, tenantID, id)
		return err
	})
	if err != nil {
		return model.PaymentException{}, err
	}
	return out, nil
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
