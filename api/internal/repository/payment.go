package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// PaymentRepository persists attempts, provider evidence and exceptions. Every method takes the tenant scope
// except the two provider-reference lookups: a notification names only the provider order id, and the tenant is
// whatever that stored reference says (never the request).
type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

const paymentColumns = `id, tenant_id, user_id, provider_order_id, order_id, topup_id, amount::text, status,
	redirect_url, expires_at, last_checked_at, paid_at, created_at`

func scanPayment(row pgx.Row) (model.PaymentAttempt, error) {
	var a model.PaymentAttempt
	err := row.Scan(&a.ID, &a.TenantID, &a.UserID, &a.ProviderOrderID, &a.OrderID, &a.TopupID, &a.Amount, &a.Status,
		&a.RedirectURL, &a.ExpiresAt, &a.LastCheckedAt, &a.PaidAt, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAttempt{}, ErrNotFound
	}
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("scan payment attempt: %w", err)
	}
	return a, nil
}

func (r *PaymentRepository) InsertAttemptTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, a model.PaymentAttempt) (model.PaymentAttempt, error) {
	if tenantID == uuid.Nil {
		return model.PaymentAttempt{}, ErrScopeRequired
	}
	out, err := scanPayment(tx.QueryRow(ctx, `
		INSERT INTO payment_attempts (tenant_id, user_id, provider, provider_order_id, order_id, topup_id, amount, expires_at)
		VALUES ($1, $2, 'midtrans', $3, $4, $5, $6::numeric, $7)
		RETURNING `+paymentColumns, tenantID, a.UserID, a.ProviderOrderID, a.OrderID, a.TopupID, a.Amount, a.ExpiresAt))
	if err != nil {
		return model.PaymentAttempt{}, err
	}
	return out, nil
}

func (r *PaymentRepository) AttemptForOrder(ctx context.Context, tenantID, orderID uuid.UUID) (model.PaymentAttempt, error) {
	if tenantID == uuid.Nil {
		return model.PaymentAttempt{}, ErrScopeRequired
	}
	return scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payment_attempts WHERE tenant_id = $1 AND order_id = $2`, tenantID, orderID))
}

func (r *PaymentRepository) AttemptForTopup(ctx context.Context, tenantID, topupID uuid.UUID) (model.PaymentAttempt, error) {
	if tenantID == uuid.Nil {
		return model.PaymentAttempt{}, ErrScopeRequired
	}
	return scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payment_attempts WHERE tenant_id = $1 AND topup_id = $2`, tenantID, topupID))
}

// AttemptByProviderOrder resolves a notification's reference to the stored attempt, and with it the tenant.
func (r *PaymentRepository) AttemptByProviderOrder(ctx context.Context, providerOrderID string) (model.PaymentAttempt, error) {
	return scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payment_attempts WHERE provider_order_id = $1`, providerOrderID))
}

// LockAttemptTx takes the attempt row so concurrent notifications and reconciliations of one payment queue up.
func (r *PaymentRepository) LockAttemptTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.PaymentAttempt, error) {
	if tenantID == uuid.Nil {
		return model.PaymentAttempt{}, ErrScopeRequired
	}
	return scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payment_attempts WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id))
}

// SetRedirect also restarts the expiry clock: the provider counts it from when the page was created.
func (r *PaymentRepository) SetRedirect(ctx context.Context, tenantID, id uuid.UUID, redirectURL string, expiresAt time.Time) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := r.db.Exec(ctx, `UPDATE payment_attempts SET redirect_url = $3, expires_at = $4, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, redirectURL, expiresAt)
	return err
}

func (r *PaymentRepository) MarkChecked(ctx context.Context, tenantID, id uuid.UUID, at time.Time) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := r.db.Exec(ctx, `UPDATE payment_attempts SET last_checked_at = $3 WHERE tenant_id = $1 AND id = $2`, tenantID, id, at)
	return err
}

func (r *PaymentRepository) SetAttemptStatusTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, status string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := tx.Exec(ctx, `
		UPDATE payment_attempts SET status = $3, updated_at = now(),
		       paid_at = CASE WHEN $3 = 'paid' THEN now() ELSE paid_at END
		 WHERE tenant_id = $1 AND id = $2`, tenantID, id, status)
	if err != nil {
		return fmt.Errorf("move payment attempt: %w", err)
	}
	return nil
}

// InsertEventTx stores provider evidence; a byte-identical replay of the same source is stored once.
func (r *PaymentRepository) InsertEventTx(ctx context.Context, tx pgx.Tx, e model.PaymentEvent, outcome string) error {
	if e.TenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO payment_events (attempt_id, tenant_id, source, transaction_status, fraud_status, status_code,
		        gross_amount, provider_transaction_id, body_hash, raw, outcome)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (attempt_id, source, body_hash) DO NOTHING`,
		e.AttemptID, e.TenantID, e.Source, e.Status, e.FraudStatus, e.StatusCode, e.GrossAmount, e.TransactionID,
		e.BodyHash, e.Raw, outcome)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	return nil
}

// ConfirmOrderByProviderTx and the cancel/topup variants below move a row that is still pending; the caller
// holds its lock. They record no reviewer, because the provider, not a person, decided.
func (r *PaymentRepository) CancelOrderTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status = 'cancelled', cancelled_at = now(), cancel_reason = $3, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, tenantID, id, reason)
	if err != nil {
		return fmt.Errorf("cancel gateway order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PaymentRepository) ApproveTopupTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE topup_requests SET status = 'approved', approved_at = now(), updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, tenantID, id)
	if err != nil {
		return fmt.Errorf("approve gateway top-up: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PaymentRepository) RejectTopupTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE topup_requests SET status = 'rejected', rejected_at = now(), reject_reason = $3, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, tenantID, id, reason)
	if err != nil {
		return fmt.Errorf("reject gateway top-up: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ContactOf reads where a payer's gateway details and notifications go, without taking any lock.
func (r *PaymentRepository) ContactOf(ctx context.Context, tenantID, userID uuid.UUID) (model.Contact, error) {
	if tenantID == uuid.Nil {
		return model.Contact{}, ErrScopeRequired
	}
	var c model.Contact
	err := r.db.QueryRow(ctx, `SELECT name, email, phone FROM users WHERE tenant_id = $1 AND id = $2`, tenantID, userID).
		Scan(&c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Contact{}, ErrNotFound
	}
	return c, err
}

func (r *PaymentRepository) TenantName(ctx context.Context, tenantID uuid.UUID) (string, error) {
	var name string
	err := r.db.QueryRow(ctx, `SELECT name FROM tenants WHERE id = $1`, tenantID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

const exceptionColumns = `x.id, x.tenant_id, x.attempt_id, x.user_id, u.name, x.order_id, o.number, x.topup_id, t.number,
	x.kind, x.paid_amount::text, x.status, x.resolved_by, x.resolve_reason, x.resolved_at, x.created_at`

const exceptionFrom = ` FROM payment_exceptions x
	JOIN users u ON u.id = x.user_id AND u.tenant_id = x.tenant_id
	LEFT JOIN orders o ON o.id = x.order_id AND o.tenant_id = x.tenant_id
	LEFT JOIN topup_requests t ON t.id = x.topup_id AND t.tenant_id = x.tenant_id`

func scanException(row pgx.Row) (model.PaymentException, error) {
	var x model.PaymentException
	err := row.Scan(&x.ID, &x.TenantID, &x.AttemptID, &x.UserID, &x.StudentName, &x.OrderID, &x.OrderNumber, &x.TopupID,
		&x.TopupNumber, &x.Kind, &x.PaidAmount, &x.Status, &x.ResolvedBy, &x.ResolveReason, &x.ResolvedAt, &x.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentException{}, ErrNotFound
	}
	if err != nil {
		return model.PaymentException{}, fmt.Errorf("scan payment exception: %w", err)
	}
	return x, nil
}

// InsertExceptionTx holds a success for review; one attempt holds at most one.
func (r *PaymentRepository) InsertExceptionTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, a model.PaymentAttempt, kind, paidAmount string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO payment_exceptions (tenant_id, attempt_id, user_id, order_id, topup_id, kind, paid_amount)
		VALUES ($1, $2, $3, $4, $5, $6, $7::numeric)
		ON CONFLICT (attempt_id) DO NOTHING`,
		tenantID, a.ID, a.UserID, a.OrderID, a.TopupID, kind, paidAmount)
	if err != nil {
		return fmt.Errorf("insert payment exception: %w", err)
	}
	return nil
}

func (r *PaymentRepository) LockExceptionTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.PaymentException, error) {
	if tenantID == uuid.Nil {
		return model.PaymentException{}, ErrScopeRequired
	}
	return scanException(tx.QueryRow(ctx, `SELECT `+exceptionColumns+exceptionFrom+
		` WHERE x.tenant_id = $1 AND x.id = $2 FOR UPDATE OF x`, tenantID, id))
}

func (r *PaymentRepository) FindExceptionTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.PaymentException, error) {
	if tenantID == uuid.Nil {
		return model.PaymentException{}, ErrScopeRequired
	}
	return scanException(tx.QueryRow(ctx, `SELECT `+exceptionColumns+exceptionFrom+` WHERE x.tenant_id = $1 AND x.id = $2`, tenantID, id))
}

func (r *PaymentRepository) FindException(ctx context.Context, tenantID, id uuid.UUID) (model.PaymentException, error) {
	if tenantID == uuid.Nil {
		return model.PaymentException{}, ErrScopeRequired
	}
	return scanException(r.db.QueryRow(ctx, `SELECT `+exceptionColumns+exceptionFrom+` WHERE x.tenant_id = $1 AND x.id = $2`, tenantID, id))
}

func (r *PaymentRepository) ResolveExceptionTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, status string, by uuid.UUID, reason string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `
		UPDATE payment_exceptions SET status = $3, resolved_by = $4, resolve_reason = $5, resolved_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'held'`, tenantID, id, status, by, reason)
	if err != nil {
		return fmt.Errorf("resolve payment exception: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PaymentRepository) ListExceptions(ctx context.Context, tenantID uuid.UUID, status string, limit, offset int) ([]model.PaymentException, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM payment_exceptions WHERE tenant_id = $1 AND ($2::text = '' OR status = $2)`,
		tenantID, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payment exceptions: %w", err)
	}
	rows, err := r.db.Query(ctx, `SELECT `+exceptionColumns+exceptionFrom+
		` WHERE x.tenant_id = $1 AND ($2::text = '' OR x.status = $2) ORDER BY x.created_at DESC, x.id DESC LIMIT $3 OFFSET $4`,
		tenantID, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list payment exceptions: %w", err)
	}
	defer rows.Close()
	out := []model.PaymentException{}
	for rows.Next() {
		x, err := scanException(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// AttemptForTarget re-reads an attempt by id.
func (r *PaymentRepository) AttemptForTarget(ctx context.Context, a model.PaymentAttempt) (model.PaymentAttempt, error) {
	if a.TenantID == uuid.Nil {
		return model.PaymentAttempt{}, ErrScopeRequired
	}
	return scanPayment(r.db.QueryRow(ctx, `SELECT `+paymentColumns+` FROM payment_attempts WHERE tenant_id = $1 AND id = $2`, a.TenantID, a.ID))
}

// ExpiredOpenAttempts lists open attempts past their expiry, least recently checked first so a transient failure
// cannot starve the rest. It spans tenants because the worker sweeps the deployment, then acts per attempt's tenant.
func (r *PaymentRepository) ExpiredOpenAttempts(ctx context.Context, now time.Time, limit int) ([]model.PaymentAttempt, error) {
	rows, err := r.db.Query(ctx, `SELECT `+paymentColumns+` FROM payment_attempts
		 WHERE status IN ('pending', 'failed') AND expires_at < $1
		 ORDER BY last_checked_at NULLS FIRST, expires_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired attempts: %w", err)
	}
	defer rows.Close()
	out := []model.PaymentAttempt{}
	for rows.Next() {
		a, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
