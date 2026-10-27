package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrWalletConflict      = errors.New("wallet source conflict")
	ErrMovementInvalid     = errors.New("wallet movement is incomplete")
)

// Wallet field reasons; the transport turns each into the copy its screens show.
const (
	ReasonWalletAmount = "wallet_amount"
	ReasonWalletType   = "wallet_type"
	ReasonWalletReason = "wallet_reason"
	ReasonWalletKey    = "wallet_key"
	ReasonWalletQuery  = "wallet_query"
)

const (
	// Decimal digits only, so `1e3`, `NaN`, `+5`, `.5`, `1.` and non-ASCII digits never parse.
	maxWalletWholeDigits = 10
	maxWalletAmountCents = int64(999999999999)
	defaultWalletLimit   = 50
	maxWalletLimit       = 100
	minIdempotencyKeyLen = 8
	maxIdempotencyKeyLen = 64
)

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// Movement is one signed wallet movement; the service signs it and the ledger records it once.
type Movement struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	Amount     string
	Kind       string
	ActorKind  string
	ActorID    *uuid.UUID
	Reason     string
	SourceType string
	SourceID   string
}

// AdjustmentInput is one staff adjustment as the caller typed it, before the service signs it.
type AdjustmentInput struct {
	Type           string
	Amount         string
	Reason         string
	IdempotencyKey string
}

type WalletService struct {
	db   *pgxpool.Pool
	repo *repository.WalletRepository
}

func NewWalletService(db *pgxpool.Pool, repo *repository.WalletRepository) *WalletService {
	return &WalletService{db: db, repo: repo}
}

// Apply is the one movement path: every credit and debit, now or in P13, comes through here.
func (s *WalletService) Apply(ctx context.Context, tx pgx.Tx, m Movement) (model.WalletEntry, bool, error) {
	amount, ok := parseAmount(m.Amount)
	if !ok {
		return model.WalletEntry{}, false, &InputError{Fields: map[string]string{"amount": ReasonWalletAmount}}
	}
	if m.TenantID == uuid.Nil || m.UserID == uuid.Nil || m.SourceType == "" || m.SourceID == "" ||
		!validKind(m.Kind) || !validActorKind(m.ActorKind) {
		return model.WalletEntry{}, false, ErrMovementInvalid
	}
	if m.Kind == model.KindAdjustment {
		if m.ActorID == nil || (m.ActorKind != model.ActorStaff && m.ActorKind != model.ActorPlatform) ||
			strings.TrimSpace(m.Reason) == "" {
			return model.WalletEntry{}, false, &InputError{Fields: map[string]string{"reason": ReasonWalletReason}}
		}
	}
	// The row lock is what makes seq order the movement order, so a balance chain can never fork.
	if _, err := s.repo.LockStudentTx(ctx, tx, m.TenantID, m.UserID); err != nil {
		return model.WalletEntry{}, false, walletError(err)
	}
	existing, err := s.repo.FindBySource(ctx, tx, m.TenantID, m.SourceType, m.SourceID)
	if err == nil {
		if existing.UserID != m.UserID || existing.Amount != amount {
			return model.WalletEntry{}, false, ErrWalletConflict
		}
		return existing, true, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return model.WalletEntry{}, false, err
	}
	balance, moved, err := s.repo.AdjustBalanceTx(ctx, tx, m.TenantID, m.UserID, amount)
	if err != nil {
		return model.WalletEntry{}, false, walletError(err)
	}
	if !moved {
		return model.WalletEntry{}, false, ErrInsufficientBalance
	}
	entry, inserted, err := s.repo.InsertEntryTx(ctx, tx, m.TenantID, model.WalletEntry{
		UserID: m.UserID, Amount: amount, BalanceAfter: balance, Kind: m.Kind,
		ActorKind: m.ActorKind, ActorID: m.ActorID, Reason: m.Reason,
		SourceType: m.SourceType, SourceID: m.SourceID,
	})
	if err != nil {
		return model.WalletEntry{}, false, walletError(err)
	}
	if !inserted {
		// The balance already moved in this transaction, so the caller must roll back rather than replay.
		return model.WalletEntry{}, false, ErrWalletConflict
	}
	return entry, false, nil
}

func (s *WalletService) Wallet(ctx context.Context, scope HostScope, principal model.Principal) (model.Wallet, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Wallet{}, err
	}
	wallet, err := s.repo.Wallet(ctx, tenantID, principal.UserID)
	if err != nil {
		return model.Wallet{}, walletError(err)
	}
	return wallet, nil
}

func (s *WalletService) Entries(ctx context.Context, scope HostScope, principal model.Principal, limit, offset int) ([]model.WalletEntry, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	return s.entries(ctx, tenantID, principal.UserID, limit, offset)
}

func (s *WalletService) StudentWallet(ctx context.Context, scope HostScope, principal model.Principal, studentID uuid.UUID) (model.Wallet, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Wallet{}, err
	}
	if _, err := s.repo.Student(ctx, tenantID, studentID); err != nil {
		return model.Wallet{}, walletError(err)
	}
	wallet, err := s.repo.Wallet(ctx, tenantID, studentID)
	if err != nil {
		return model.Wallet{}, walletError(err)
	}
	return wallet, nil
}

func (s *WalletService) StudentEntries(ctx context.Context, scope HostScope, principal model.Principal, studentID uuid.UUID, limit, offset int) ([]model.WalletEntry, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if _, err := s.repo.Student(ctx, tenantID, studentID); err != nil {
		return nil, 0, walletError(err)
	}
	return s.entries(ctx, tenantID, studentID, limit, offset)
}

func (s *WalletService) Students(ctx context.Context, scope HostScope, principal model.Principal, filter repository.WalletFilter) ([]model.WalletStudent, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	filter, err = checkedWalletFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	students, total, err := s.repo.Students(ctx, tenantID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list students: %w", err)
	}
	return students, total, nil
}

// Adjust signs a staff credit or debit and commits it with its ledger row in one transaction.
func (s *WalletService) Adjust(ctx context.Context, scope HostScope, principal model.Principal, studentID uuid.UUID, in AdjustmentInput) (model.WalletEntry, bool, model.Wallet, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.WalletEntry{}, false, model.Wallet{}, err
	}
	signed, reason, key, err := checkedAdjustment(in)
	if err != nil {
		return model.WalletEntry{}, false, model.Wallet{}, err
	}
	actor := principal.UserID
	var entry model.WalletEntry
	var replayed bool
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		applied, again, err := s.Apply(ctx, tx, Movement{
			TenantID: tenantID, UserID: studentID, Amount: signed, Kind: model.KindAdjustment,
			ActorKind: model.ActorStaff, ActorID: &actor, Reason: reason,
			SourceType: model.KindAdjustment, SourceID: key,
		})
		if err != nil {
			return err
		}
		entry, replayed = applied, again
		return nil
	})
	if err != nil {
		return model.WalletEntry{}, false, model.Wallet{}, err
	}
	wallet, err := s.repo.Wallet(ctx, tenantID, studentID)
	if err != nil {
		return model.WalletEntry{}, false, model.Wallet{}, walletError(err)
	}
	return entry, replayed, wallet, nil
}

func (s *WalletService) entries(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.WalletEntry, int, error) {
	limit, offset, err := checkedWalletPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	entries, total, err := s.repo.Entries(ctx, tenantID, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list wallet entries: %w", err)
	}
	return entries, total, nil
}

// parseAmount accepts an optional '-', ASCII digits and at most two fraction digits, then normalizes.
func parseAmount(raw string) (string, bool) {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return "", false
	}
	negative := strings.HasPrefix(raw, "-")
	digits := raw
	if negative {
		digits = raw[1:]
	}
	whole, fraction, hasPoint := strings.Cut(digits, ".")
	if whole == "" || len(whole) > maxWalletWholeDigits || (hasPoint && (fraction == "" || len(fraction) > 2)) {
		return "", false
	}
	if strings.Contains(fraction, ".") {
		return "", false
	}
	for i := 0; i < len(digits); i++ {
		if c := digits[i]; c != '.' && (c < '0' || c > '9') {
			return "", false
		}
	}
	cents, ok := decimalCents(whole, fraction)
	if !ok || cents <= 0 || cents > maxWalletAmountCents {
		return "", false
	}
	out := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	if negative {
		return "-" + out, true
	}
	return out, true
}

func checkedAdjustment(in AdjustmentInput) (signed, reason, key string, err error) {
	fields := map[string]string{}
	if in.Type != "credit" && in.Type != "debit" {
		fields["type"] = ReasonWalletType
	}
	amount, ok := parseAmount(in.Amount)
	if !ok || strings.HasPrefix(amount, "-") {
		fields["amount"] = ReasonWalletAmount
	}
	reason = strings.TrimSpace(in.Reason)
	if reason == "" {
		fields["reason"] = ReasonWalletReason
	}
	key = strings.TrimSpace(in.IdempotencyKey)
	if !idempotencyKey.MatchString(key) {
		fields["idempotency_key"] = ReasonWalletKey
	}
	if len(fields) > 0 {
		return "", "", "", &InputError{Fields: fields}
	}
	if in.Type == "debit" {
		amount = "-" + amount
	}
	return amount, reason, key, nil
}

func checkedWalletPage(limit, offset int) (int, int, error) {
	fields := map[string]string{}
	if limit == 0 {
		limit = defaultWalletLimit
	}
	if limit < 1 || limit > maxWalletLimit {
		fields["limit"] = ReasonWalletQuery
	}
	if offset < 0 {
		fields["offset"] = ReasonWalletQuery
	}
	if len(fields) > 0 {
		return 0, 0, &InputError{Fields: fields}
	}
	return limit, offset, nil
}

func checkedWalletFilter(filter repository.WalletFilter) (repository.WalletFilter, error) {
	limit, offset, err := checkedWalletPage(filter.Limit, filter.Offset)
	if err != nil {
		return repository.WalletFilter{}, err
	}
	filter.Limit, filter.Offset = limit, offset
	if utf8.RuneCountInString(filter.Search) > 100 {
		return repository.WalletFilter{}, &InputError{Fields: map[string]string{"q": ReasonWalletQuery}}
	}
	return filter, nil
}

func validKind(kind string) bool {
	switch kind {
	case model.KindOpening, model.KindTopup, model.KindSpend, model.KindAdjustment:
		return true
	}
	return false
}

func validActorKind(actor string) bool {
	switch actor {
	case model.ActorSystem, model.ActorStudent, model.ActorProvider, model.ActorStaff, model.ActorPlatform:
		return true
	}
	return false
}

// walletTenant pins every query to the host's tenant and refuses a principal from another one.
func walletTenant(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	if scope.Tenant.ID == uuid.Nil || principal.TenantID == nil || *principal.TenantID != scope.Tenant.ID {
		return uuid.Nil, ErrTenantNotFound
	}
	return scope.Tenant.ID, nil
}

func walletError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrWalletNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *WalletService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
