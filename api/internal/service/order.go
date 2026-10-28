package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrOrderNotFound            = errors.New("order not found")
	ErrOrderForbidden           = errors.New("order action not allowed for this role")
	ErrOrderState               = errors.New("order is no longer pending")
	ErrOrderKeyConflict         = errors.New("idempotency key belongs to a different order")
	ErrProductUnavailable       = errors.New("product is not for sale")
	ErrAlreadyOwned             = errors.New("tryout already held")
	ErrBankTransferUnavailable  = errors.New("bank transfer account is not configured")
	ErrEnrollmentSourceRequired = errors.New("enrollment grant needs a source")
)

// Order field reasons; the transport turns each into the copy its screens show.
const (
	ReasonOrderProduct = "order_product"
	ReasonOrderMethod  = "order_method"
	ReasonOrderKey     = "order_key"
	ReasonOrderProof   = "order_proof"
	ReasonOrderReason  = "order_reason"
	ReasonOrderQuery   = "order_query"
)

const (
	TemplateOrderConfirmed = "order-confirmed"
	TemplateOrderCancelled = "order-cancelled"

	maxCancelReasonLen = 500
	defaultOrderLimit  = 50
	maxOrderLimit      = 100
)

// BankAccount is the single tenant's transfer destination, read from server configuration.
type BankAccount struct {
	Bank   string
	Number string
	Holder string
}

func (b BankAccount) Configured() bool { return b.Bank != "" && b.Number != "" && b.Holder != "" }

type CheckoutInput struct {
	ProductID    uuid.UUID
	Method       string
	Key          string
	ProofAssetID *uuid.UUID
}

type OrderFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// IssueInput is one business source asking for access. Order carries the order for an order source.
type IssueInput struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	TryoutIDs  []uuid.UUID
	SourceType string
	SourceID   string
	OrderID    *uuid.UUID
	ActorKind  string
	ActorID    *uuid.UUID
	Reason     string
}

type OrderOption func(*OrderService)

// WithOrderClock replaces the clock; the sale window and held access are time-based, so tests move time.
func WithOrderClock(now func() time.Time) OrderOption {
	return func(s *OrderService) { s.now = now }
}

type OrderService struct {
	db       *pgxpool.Pool
	repo     *repository.OrderRepository
	products *repository.ProductRepository
	wallets  *WalletService
	assets   *AssetService
	notes    *NotificationService
	bank     BankAccount
	now      func() time.Time
}

func NewOrderService(db *pgxpool.Pool, repo *repository.OrderRepository, products *repository.ProductRepository, wallets *WalletService, assets *AssetService, notes *NotificationService, bank BankAccount, opts ...OrderOption) *OrderService {
	s := &OrderService{db: db, repo: repo, products: products, wallets: wallets, assets: assets, notes: notes, bank: bank, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// OrderCode is the human-facing code of an order number.
func OrderCode(number int64) string { return fmt.Sprintf("ORD-%06d", number) }

// Checkout creates the order. A wallet payment is debited, confirmed and granted in the same transaction;
// a manual one waits as pending with its proof attached. The bool reports a replay of the same key.
func (s *OrderService) Checkout(ctx context.Context, scope HostScope, principal model.Principal, in CheckoutInput) (model.Order, bool, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Order{}, false, err
	}
	if principal.Role != model.RoleStudent {
		return model.Order{}, false, ErrOrderForbidden
	}
	if err := checkedCheckout(in); err != nil {
		return model.Order{}, false, err
	}
	if in.Method == model.PaymentManual && !s.bank.Configured() {
		return model.Order{}, false, ErrBankTransferUnavailable
	}

	var out model.Order
	var replayed bool
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		contact, err := s.repo.LockStudentTx(ctx, tx, tenantID, principal.UserID)
		if err != nil {
			return orderError(err)
		}
		existing, err := s.repo.FindByKeyTx(ctx, tx, tenantID, principal.UserID, in.Key)
		if err == nil {
			if existing.ProductID != in.ProductID || existing.PaymentMethod != in.Method || !sameProof(existing.ProofAssetID, in.ProofAssetID) {
				return ErrOrderKeyConflict
			}
			out, replayed = existing, true
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		product, err := s.products.SellableTx(ctx, tx, tenantID, in.ProductID, s.now())
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrProductUnavailable
			}
			return orderError(err)
		}
		items := make([]model.OrderItem, 0, len(product.Tryouts))
		tryoutIDs := make([]uuid.UUID, 0, len(product.Tryouts))
		for _, t := range product.Tryouts {
			items = append(items, model.OrderItem{TryoutID: t.ID, Title: t.Title})
			tryoutIDs = append(tryoutIDs, t.ID)
		}
		if product.Type == model.ProductIndividual {
			held, err := s.repo.HeldTryouts(ctx, tx, tenantID, principal.UserID, tryoutIDs, s.now())
			if err != nil {
				return orderError(err)
			}
			if len(held) > 0 {
				return ErrAlreadyOwned
			}
		}
		if in.ProofAssetID != nil {
			asset, err := s.assets.FinalizeTx(ctx, tx, scope, principal, *in.ProofAssetID)
			if err != nil {
				if errors.Is(err, ErrAssetNotFound) {
					return proofInvalid()
				}
				return err
			}
			if asset.Purpose != model.PurposePaymentProof {
				return proofInvalid()
			}
		}

		order, err := s.repo.InsertTx(ctx, tx, tenantID, model.Order{
			UserID: principal.UserID, ProductID: product.ID, ProductType: product.Type,
			ProductTitle: product.Title, Price: product.Price, PaymentMethod: in.Method,
			Key: in.Key, ProofAssetID: in.ProofAssetID, Items: items,
		})
		if err != nil {
			return orderError(err)
		}
		if in.Method == model.PaymentWallet {
			order, err = s.payWithWallet(ctx, tx, scope, principal, order, contact)
			if err != nil {
				return err
			}
		}
		out = order
		return nil
	})
	if err != nil {
		return model.Order{}, false, err
	}
	return out, replayed, nil
}

// payWithWallet debits through the one wallet path, then confirms, grants and notifies in the same transaction.
func (s *OrderService) payWithWallet(ctx context.Context, tx pgx.Tx, scope HostScope, principal model.Principal, order model.Order, contact model.Contact) (model.Order, error) {
	actor := principal.UserID
	if _, _, err := s.wallets.Apply(ctx, tx, Movement{
		TenantID: order.TenantID, UserID: order.UserID, Amount: "-" + order.Price, Kind: model.KindSpend,
		ActorKind: model.ActorStudent, ActorID: &actor, Reason: "Pembelian " + order.ProductTitle,
		SourceType: model.GrantSourceOrder, SourceID: order.ID.String(),
	}); err != nil {
		return model.Order{}, err
	}
	if err := s.repo.ConfirmTx(ctx, tx, order.TenantID, order.ID, nil); err != nil {
		return model.Order{}, orderError(err)
	}
	if err := s.grantOrder(ctx, tx, order, model.ActorStudent, &actor); err != nil {
		return model.Order{}, err
	}
	confirmed, err := s.repo.FindTx(ctx, tx, order.TenantID, order.ID)
	if err != nil {
		return model.Order{}, orderError(err)
	}
	if err := s.notify(ctx, tx, scope, confirmed, contact, TemplateOrderConfirmed, ""); err != nil {
		return model.Order{}, err
	}
	return confirmed, nil
}

// Confirm accepts a pending order's payment. Repeating it on a confirmed order changes nothing.
func (s *OrderService) Confirm(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Order, error) {
	return s.review(ctx, scope, principal, id, model.OrderConfirmed, "")
}

// Cancel refuses a pending order with a reason and opens no access. Repeating it changes nothing.
func (s *OrderService) Cancel(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, reason string) (model.Order, error) {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 1 || n > maxCancelReasonLen {
		return model.Order{}, &InputError{Fields: map[string]string{"reason": ReasonOrderReason}}
	}
	return s.review(ctx, scope, principal, id, model.OrderCancelled, reason)
}

func (s *OrderService) review(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, target, reason string) (model.Order, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Order{}, err
	}
	if !isStaff(principal.Role) {
		return model.Order{}, ErrOrderForbidden
	}
	var out model.Order
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		order, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return orderError(err)
		}
		if order.Status != model.OrderPending {
			if order.Status != target {
				return ErrOrderState
			}
			out = order
			return nil
		}
		contact, err := s.repo.Contact(ctx, tx, tenantID, order.UserID)
		if err != nil {
			return orderError(err)
		}
		reviewer := principal.UserID
		template := TemplateOrderConfirmed
		if target == model.OrderConfirmed {
			if err := s.repo.ConfirmTx(ctx, tx, tenantID, id, &reviewer); err != nil {
				return orderError(err)
			}
			if err := s.grantOrder(ctx, tx, order, model.ActorStaff, &reviewer); err != nil {
				return err
			}
		} else {
			template = TemplateOrderCancelled
			if err := s.repo.CancelTx(ctx, tx, tenantID, id, reviewer, reason); err != nil {
				return orderError(err)
			}
		}
		out, err = s.repo.FindTx(ctx, tx, tenantID, id)
		if err != nil {
			return orderError(err)
		}
		return s.notify(ctx, tx, scope, out, contact, template, reason)
	})
	if err != nil {
		return model.Order{}, err
	}
	return out, nil
}

// grantOrder opens every tryout of the order's snapshot; the order id is the business source.
func (s *OrderService) grantOrder(ctx context.Context, tx pgx.Tx, order model.Order, actorKind string, actor *uuid.UUID) error {
	ids := make([]uuid.UUID, 0, len(order.Items))
	for _, item := range order.Items {
		ids = append(ids, item.TryoutID)
	}
	orderID := order.ID
	_, err := s.IssueEnrollmentsTx(ctx, tx, IssueInput{
		TenantID: order.TenantID, UserID: order.UserID, TryoutIDs: ids,
		SourceType: model.GrantSourceOrder, SourceID: order.ID.String(), OrderID: &orderID,
		ActorKind: actorKind, ActorID: actor,
	})
	return err
}

// IssueEnrollmentsTx is the one place access is granted: wallet and manual checkout use it now, the gateway
// and staff grants later. A tryout the user already holds is left untouched (D05b), and the source key makes
// a repeated call grant nothing twice. It returns the tryouts this call opened.
func (s *OrderService) IssueEnrollmentsTx(ctx context.Context, tx pgx.Tx, in IssueInput) ([]uuid.UUID, error) {
	if in.SourceType == "" || in.SourceID == "" {
		return nil, ErrEnrollmentSourceRequired
	}
	var opened []uuid.UUID
	for _, tryoutID := range in.TryoutIDs {
		enrollmentID, created, err := s.repo.InsertEnrollmentTx(ctx, tx, in.TenantID, in.UserID, tryoutID)
		if err != nil {
			return nil, err
		}
		if !created {
			continue
		}
		granted, err := s.repo.InsertGrantTx(ctx, tx, in.TenantID, model.EnrollmentGrant{
			EnrollmentID: enrollmentID, OrderID: in.OrderID, SourceType: in.SourceType, SourceID: in.SourceID,
			ActorKind: in.ActorKind, ActorID: in.ActorID, Reason: in.Reason,
		})
		if err != nil {
			return nil, err
		}
		if granted {
			opened = append(opened, tryoutID)
		}
	}
	return opened, nil
}

// notify queues the student's email and WhatsApp messages in the caller's transaction.
func (s *OrderService) notify(ctx context.Context, tx pgx.Tx, scope HostScope, order model.Order, contact model.Contact, template, reason string) error {
	tenantID := order.TenantID
	params := map[string]string{
		"tenant_name": scope.Tenant.Name, "name": contact.Name, "code": OrderCode(order.Number),
		"product": order.ProductTitle,
	}
	if reason != "" {
		params["reason"] = reason
	}
	for _, target := range []struct {
		channel   model.Channel
		recipient *string
	}{{model.ChannelEmail, contact.Email}, {model.ChannelWhatsApp, contact.Phone}} {
		if target.recipient == nil || strings.TrimSpace(*target.recipient) == "" {
			continue
		}
		if err := s.notes.Enqueue(ctx, tx, NewNotification{
			Scope: Scope{TenantID: &tenantID}, EventKey: template + ":" + order.ID.String(),
			Channel: string(target.channel), Recipient: *target.recipient, Template: template, Params: params,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Orders lists the signed-in student's own orders; nobody else's are reachable from here.
func (s *OrderService) Orders(ctx context.Context, scope HostScope, principal model.Principal, limit, offset int) ([]model.Order, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if principal.Role != model.RoleStudent {
		return nil, 0, ErrOrderForbidden
	}
	limit, offset, err = checkedOrderPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.ListForUser(ctx, tenantID, principal.UserID, limit, offset)
}

// Order answers the same not-found for another student's order as for a missing one.
func (s *OrderService) Order(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Order, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Order{}, err
	}
	order, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Order{}, orderError(err)
	}
	if principal.Role == model.RoleStudent && order.UserID != principal.UserID {
		return model.Order{}, ErrOrderNotFound
	}
	if principal.Role != model.RoleStudent && !isStaff(principal.Role) {
		return model.Order{}, ErrOrderForbidden
	}
	return order, nil
}

// StaffOrders lists every order of the tenant for review.
func (s *OrderService) StaffOrders(ctx context.Context, scope HostScope, principal model.Principal, filter OrderFilter) ([]model.Order, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if !isStaff(principal.Role) {
		return nil, 0, ErrOrderForbidden
	}
	fields := map[string]string{}
	filter.Limit, filter.Offset, err = checkedOrderPage(filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, err
	}
	switch filter.Status {
	case "", model.OrderPending, model.OrderConfirmed, model.OrderCancelled:
	default:
		fields["status"] = ReasonOrderQuery
	}
	if utf8.RuneCountInString(filter.Search) > 100 {
		fields["q"] = ReasonOrderQuery
	}
	if len(fields) > 0 {
		return nil, 0, &InputError{Fields: fields}
	}
	return s.repo.List(ctx, tenantID, repository.OrderFilter{
		Status: filter.Status, Search: filter.Search, Limit: filter.Limit, Offset: filter.Offset,
	})
}

// Enrollments is "Paket Belajar Saya": what the signed-in student holds.
func (s *OrderService) Enrollments(ctx context.Context, scope HostScope, principal model.Principal) ([]model.Enrollment, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, err
	}
	if principal.Role != model.RoleStudent {
		return nil, ErrOrderForbidden
	}
	return s.repo.Enrollments(ctx, tenantID, principal.UserID)
}

// BankTransfer is where a manual payment goes; it is the same for every student of the tenant.
func (s *OrderService) BankTransfer(scope HostScope, principal model.Principal) (BankAccount, error) {
	if _, err := walletTenant(scope, principal); err != nil {
		return BankAccount{}, err
	}
	if !s.bank.Configured() {
		return BankAccount{}, ErrBankTransferUnavailable
	}
	return s.bank, nil
}

func checkedCheckout(in CheckoutInput) error {
	fields := map[string]string{}
	if in.ProductID == uuid.Nil {
		fields["product_id"] = ReasonOrderProduct
	}
	switch in.Method {
	case model.PaymentManual:
		if in.ProofAssetID == nil {
			fields["proof_asset_id"] = ReasonOrderProof
		}
	case model.PaymentWallet:
		if in.ProofAssetID != nil {
			fields["proof_asset_id"] = ReasonOrderProof
		}
	default:
		fields["payment_method"] = ReasonOrderMethod
	}
	if !idempotencyKey.MatchString(in.Key) {
		fields["idempotency_key"] = ReasonOrderKey
	}
	if len(fields) > 0 {
		return &InputError{Fields: fields}
	}
	return nil
}

func checkedOrderPage(limit, offset int) (int, int, error) {
	fields := map[string]string{}
	if limit == 0 {
		limit = defaultOrderLimit
	}
	if limit < 1 || limit > maxOrderLimit {
		fields["limit"] = ReasonOrderQuery
	}
	if offset < 0 {
		fields["offset"] = ReasonOrderQuery
	}
	if len(fields) > 0 {
		return 0, 0, &InputError{Fields: fields}
	}
	return limit, offset, nil
}

func sameProof(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// proofInvalid answers a missing, foreign, wrongly-typed or already used proof the same way.
func proofInvalid() error {
	return &InputError{Fields: map[string]string{"proof_asset_id": ReasonOrderProof}}
}

func orderError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrOrderNotFound
	case errors.Is(err, repository.ErrProofTaken):
		return proofInvalid()
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *OrderService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
