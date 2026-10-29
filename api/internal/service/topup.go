package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrTopupNotFound    = errors.New("top-up request not found")
	ErrTopupForbidden   = errors.New("top-up action not allowed for this role")
	ErrTopupState       = errors.New("top-up request is no longer pending")
	ErrTopupKeyConflict = errors.New("idempotency key belongs to a different top-up request")
)

// Top-up field reasons; the transport turns each into the copy its screens show.
const (
	ReasonTopupAmount = "topup_amount"
	ReasonTopupKey    = "topup_key"
	ReasonTopupProof  = "topup_proof"
	ReasonTopupReason = "topup_reason"
	ReasonTopupQuery  = "topup_query"
	ReasonTopupMethod = "topup_method"
)

const (
	TemplateTopupApproved = "topup-approved"
	TemplateTopupRejected = "topup-rejected"

	maxRejectReasonLen = 500
	defaultTopupLimit  = 50
	maxTopupLimit      = 100
)

type CreateTopupInput struct {
	// Method is manual (default) or gateway; a gateway top-up has no proof.
	Method       string
	Amount       string
	Key          string
	ProofAssetID *uuid.UUID
}

type TopupFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// TopupService owns the request lifecycle; the money itself only ever moves through WalletService.Apply.
type TopupService struct {
	db      *pgxpool.Pool
	repo    *repository.TopupRepository
	wallets *WalletService
	assets  *AssetService
	notes   *NotificationService
	// payments is set by NewPaymentService; nil means no gateway.
	payments *PaymentService
}

func NewTopupService(db *pgxpool.Pool, repo *repository.TopupRepository, wallets *WalletService, assets *AssetService, notes *NotificationService) *TopupService {
	return &TopupService{db: db, repo: repo, wallets: wallets, assets: assets, notes: notes}
}

// TopupCode is the human-facing code of a request number.
func TopupCode(number int64) string { return fmt.Sprintf("TUP-%06d", number) }

// Create records a pending request with its proof; nothing is credited until staff approve it. The bool
// reports a replay of the same key, which writes nothing.
func (s *TopupService) Create(ctx context.Context, scope HostScope, principal model.Principal, in CreateTopupInput) (model.Topup, bool, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Topup{}, false, err
	}
	if principal.Role != model.RoleStudent {
		return model.Topup{}, false, ErrTopupForbidden
	}
	if in.Method == "" {
		in.Method = model.TopupManual
	}
	amount, err := checkedTopup(in)
	if err != nil {
		return model.Topup{}, false, err
	}
	if in.Method == model.TopupGateway && !s.payments.Available() {
		return model.Topup{}, false, ErrGatewayUnavailable
	}

	var out model.Topup
	var replayed bool
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.LockStudentTx(ctx, tx, tenantID, principal.UserID); err != nil {
			return topupError(err)
		}
		existing, err := s.repo.FindByKeyTx(ctx, tx, tenantID, principal.UserID, in.Key)
		if err == nil {
			if existing.Amount != amount || existing.Method != in.Method || !sameProof(existing.ProofAssetID, in.ProofAssetID) {
				return ErrTopupKeyConflict
			}
			out, replayed = existing, true
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}

		minimum, err := s.repo.MinTopup(ctx, tx, tenantID)
		if err != nil {
			return topupError(err)
		}
		if moneyCents(amount) < moneyCents(minimum) {
			return &InputError{Fields: map[string]string{"amount": ReasonTopupAmount}}
		}
		if in.ProofAssetID != nil {
			asset, err := s.assets.FinalizeTx(ctx, tx, scope, principal, *in.ProofAssetID)
			if err != nil {
				if errors.Is(err, ErrAssetNotFound) {
					return topupProofInvalid()
				}
				return err
			}
			if asset.Purpose != model.PurposePaymentProof {
				return topupProofInvalid()
			}
		}
		out, err = s.repo.InsertTx(ctx, tx, tenantID, model.Topup{
			UserID: principal.UserID, Amount: amount, Method: in.Method, Key: in.Key, ProofAssetID: in.ProofAssetID,
		})
		if err != nil {
			return topupError(err)
		}
		if in.Method == model.TopupGateway {
			return s.payments.insertAttemptTx(ctx, tx, tenantID, principal.UserID, nil, &out.ID, amount)
		}
		return nil
	})
	if err != nil {
		return model.Topup{}, false, err
	}
	if out.Method == model.TopupGateway && out.Status == model.TopupPending {
		opened, err := s.payments.ensureTopupRedirect(ctx, out)
		if err != nil {
			return model.Topup{}, false, err
		}
		out = opened
	}
	return out, replayed, nil
}

// Approve accepts a pending request and credits the wallet once. Repeating it on an approved request changes nothing.
func (s *TopupService) Approve(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Topup, error) {
	return s.review(ctx, scope, principal, id, model.TopupApproved, "")
}

// Reject refuses a pending request with a reason and credits nothing. Repeating it changes nothing.
func (s *TopupService) Reject(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, reason string) (model.Topup, error) {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 1 || n > maxRejectReasonLen {
		return model.Topup{}, &InputError{Fields: map[string]string{"reason": ReasonTopupReason}}
	}
	return s.review(ctx, scope, principal, id, model.TopupRejected, reason)
}

func (s *TopupService) review(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, target, reason string) (model.Topup, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Topup{}, err
	}
	if !isStaff(principal.Role) {
		return model.Topup{}, ErrTopupForbidden
	}
	var out model.Topup
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		request, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return topupError(err)
		}
		if request.Status != model.TopupPending {
			if request.Status != target {
				return ErrTopupState
			}
			out = request
			return nil
		}
		contact, err := s.repo.ContactOf(ctx, tx, tenantID, request.UserID)
		if err != nil {
			return topupError(err)
		}
		reviewer := principal.UserID
		template := TemplateTopupApproved
		if target == model.TopupApproved {
			if err := s.repo.ApproveTx(ctx, tx, tenantID, id, reviewer); err != nil {
				return topupError(err)
			}
			if _, _, err := s.wallets.Apply(ctx, tx, Movement{
				TenantID: tenantID, UserID: request.UserID, Amount: request.Amount, Kind: model.KindTopup,
				ActorKind: model.ActorStaff, ActorID: &reviewer, Reason: "Top-up " + TopupCode(request.Number),
				SourceType: model.WalletSourceTopup, SourceID: request.ID.String(),
			}); err != nil {
				return err
			}
		} else {
			template = TemplateTopupRejected
			if err := s.repo.RejectTx(ctx, tx, tenantID, id, reviewer, reason); err != nil {
				return topupError(err)
			}
		}
		out, err = s.repo.FindTx(ctx, tx, tenantID, id)
		if err != nil {
			return topupError(err)
		}
		return s.notify(ctx, tx, scope, out, contact, template, reason)
	})
	if err != nil {
		return model.Topup{}, err
	}
	return out, nil
}

// notify queues the student's email and WhatsApp messages in the review's transaction.
func (s *TopupService) notify(ctx context.Context, tx pgx.Tx, scope HostScope, request model.Topup, contact model.Contact, template, reason string) error {
	tenantID := request.TenantID
	params := map[string]string{
		"tenant_name": scope.Tenant.Name, "name": contact.Name, "code": TopupCode(request.Number),
		"amount": rupiah(request.Amount),
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
			Scope: Scope{TenantID: &tenantID}, EventKey: template + ":" + request.ID.String(),
			Channel: string(target.channel), Recipient: *target.recipient, Template: template, Params: params,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Topups lists the signed-in student's own requests with the current minimum; nobody else's are reachable here.
func (s *TopupService) Topups(ctx context.Context, scope HostScope, principal model.Principal, limit, offset int) ([]model.Topup, int, string, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, "", err
	}
	if principal.Role != model.RoleStudent {
		return nil, 0, "", ErrTopupForbidden
	}
	limit, offset, err = checkedTopupPage(limit, offset)
	if err != nil {
		return nil, 0, "", err
	}
	topups, total, err := s.repo.ListForUser(ctx, tenantID, principal.UserID, limit, offset)
	if err != nil {
		return nil, 0, "", topupError(err)
	}
	minimum, err := s.repo.MinTopup(ctx, s.db, tenantID)
	return topups, total, minimum, topupError(err)
}

// Topup answers the same not-found for another student's request as for a missing one.
func (s *TopupService) Topup(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Topup, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.Topup{}, err
	}
	request, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Topup{}, topupError(err)
	}
	if principal.Role == model.RoleStudent && request.UserID != principal.UserID {
		return model.Topup{}, ErrTopupNotFound
	}
	if principal.Role != model.RoleStudent && !isStaff(principal.Role) {
		return model.Topup{}, ErrTopupForbidden
	}
	if request.Method == model.TopupGateway && request.Status == model.TopupPending && s.payments != nil {
		s.payments.reconcileTopup(ctx, request)
		if fresh, err := s.repo.Find(ctx, tenantID, id); err == nil {
			request = fresh
		}
	}
	return request, nil
}

// StaffTopups lists every request of the tenant for review.
func (s *TopupService) StaffTopups(ctx context.Context, scope HostScope, principal model.Principal, filter TopupFilter) ([]model.Topup, int, string, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, "", err
	}
	if !isStaff(principal.Role) {
		return nil, 0, "", ErrTopupForbidden
	}
	fields := map[string]string{}
	filter.Limit, filter.Offset, err = checkedTopupPage(filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, "", err
	}
	switch filter.Status {
	case "", model.TopupPending, model.TopupApproved, model.TopupRejected:
	default:
		fields["status"] = ReasonTopupQuery
	}
	if utf8.RuneCountInString(filter.Search) > 100 {
		fields["q"] = ReasonTopupQuery
	}
	if len(fields) > 0 {
		return nil, 0, "", &InputError{Fields: fields}
	}
	topups, total, err := s.repo.List(ctx, tenantID, repository.TopupFilter{
		Status: filter.Status, Search: filter.Search, Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, "", topupError(err)
	}
	minimum, err := s.repo.MinTopup(ctx, s.db, tenantID)
	return topups, total, minimum, topupError(err)
}

// parseWholeRupiah accepts digits with an optional ".00"/".0" and refuses any real fraction, sign or exponent.
func parseWholeRupiah(raw string) (string, bool) {
	if strings.HasPrefix(raw, "-") {
		return "", false
	}
	amount, ok := parseAmount(raw)
	if !ok || !strings.HasSuffix(amount, ".00") {
		return "", false
	}
	return amount, true
}

// checkedTopup validates the shape of a request; the minimum needs the database and is checked in Create.
func checkedTopup(in CreateTopupInput) (string, error) {
	fields := map[string]string{}
	amount, ok := parseWholeRupiah(in.Amount)
	if !ok {
		fields["amount"] = ReasonTopupAmount
	}
	if !idempotencyKey.MatchString(in.Key) {
		fields["idempotency_key"] = ReasonTopupKey
	}
	switch in.Method {
	case model.TopupManual:
		if in.ProofAssetID == nil {
			fields["proof_asset_id"] = ReasonTopupProof
		}
	case model.TopupGateway:
		if in.ProofAssetID != nil {
			fields["proof_asset_id"] = ReasonTopupProof
		}
	default:
		fields["method"] = ReasonTopupMethod
	}
	if len(fields) > 0 {
		return "", &InputError{Fields: fields}
	}
	return amount, nil
}

func checkedTopupPage(limit, offset int) (int, int, error) {
	fields := map[string]string{}
	if limit == 0 {
		limit = defaultTopupLimit
	}
	if limit < 1 || limit > maxTopupLimit {
		fields["limit"] = ReasonTopupQuery
	}
	if offset < 0 {
		fields["offset"] = ReasonTopupQuery
	}
	if len(fields) > 0 {
		return 0, 0, &InputError{Fields: fields}
	}
	return limit, offset, nil
}

// rupiah formats a canonical whole-rupiah amount for a message, e.g. "50000.00" becomes "Rp50.000".
func rupiah(amount string) string {
	whole, _, _ := strings.Cut(amount, ".")
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return "Rp" + b.String()
}

// topupProofInvalid answers a missing, foreign, wrongly-typed or already used proof the same way.
func topupProofInvalid() error {
	return &InputError{Fields: map[string]string{"proof_asset_id": ReasonTopupProof}}
}

func topupError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrTopupNotFound
	case errors.Is(err, repository.ErrProofTaken):
		return topupProofInvalid()
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *TopupService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
