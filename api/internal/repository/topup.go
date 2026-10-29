package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// TopupFilter is already validated by the service; the repository only turns it into SQL.
type TopupFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

type TopupRepository struct {
	db *pgxpool.Pool
}

func NewTopupRepository(db *pgxpool.Pool) *TopupRepository {
	return &TopupRepository{db: db}
}

const topupColumns = `t.id, t.number, t.tenant_id, t.user_id, u.name, t.amount::text, t.method, t.status,
	t.idempotency_key, t.proof_asset_id, t.reject_reason, t.reviewed_by, t.approved_at, t.rejected_at,
	t.created_at, t.updated_at`

const topupFrom = ` FROM topup_requests t JOIN users u ON u.id = t.user_id AND u.tenant_id = t.tenant_id`

// MinTopup is the tenant's minimum, or the D12 default when it never saved one.
func (r *TopupRepository) MinTopup(ctx context.Context, q queryer, tenantID uuid.UUID) (string, error) {
	return minTopup(ctx, q, tenantID)
}

// LockStudentTx serializes one student's top-up creations on the same row the wallet uses, and returns where
// the notifications go.
func (r *TopupRepository) LockStudentTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (model.Contact, error) {
	if tenantID == uuid.Nil {
		return model.Contact{}, ErrScopeRequired
	}
	var c model.Contact
	err := tx.QueryRow(ctx,
		`SELECT name, email, phone FROM users WHERE tenant_id = $1 AND id = $2 AND role = 'student' FOR UPDATE`,
		tenantID, userID).Scan(&c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Contact{}, ErrNotFound
	}
	if err != nil {
		return model.Contact{}, fmt.Errorf("lock student: %w", err)
	}
	return c, nil
}

// ContactOf reads a student's contact without locking the wallet row; the review path locks the request first.
func (r *TopupRepository) ContactOf(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (model.Contact, error) {
	if tenantID == uuid.Nil {
		return model.Contact{}, ErrScopeRequired
	}
	var c model.Contact
	err := tx.QueryRow(ctx,
		`SELECT name, email, phone FROM users WHERE tenant_id = $1 AND id = $2 AND role = 'student'`,
		tenantID, userID).Scan(&c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Contact{}, ErrNotFound
	}
	if err != nil {
		return model.Contact{}, fmt.Errorf("read student contact: %w", err)
	}
	return c, nil
}

func (r *TopupRepository) FindByKeyTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, key string) (model.Topup, error) {
	if tenantID == uuid.Nil {
		return model.Topup{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `t.tenant_id = $1 AND t.user_id = $2 AND t.idempotency_key = $3`, false, tenantID, userID, key)
}

func (r *TopupRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Topup, error) {
	if tenantID == uuid.Nil {
		return model.Topup{}, ErrScopeRequired
	}
	return r.find(ctx, r.db, `t.tenant_id = $1 AND t.id = $2`, false, tenantID, id)
}

// LockTx takes the request row, so concurrent reviews of one request queue behind each other.
func (r *TopupRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Topup, error) {
	if tenantID == uuid.Nil {
		return model.Topup{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `t.tenant_id = $1 AND t.id = $2`, true, tenantID, id)
}

func (r *TopupRepository) FindTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Topup, error) {
	if tenantID == uuid.Nil {
		return model.Topup{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `t.tenant_id = $1 AND t.id = $2`, false, tenantID, id)
}

// InsertTx refuses a proof that already backs an order, so one upload never proves two payments.
func (r *TopupRepository) InsertTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, t model.Topup) (model.Topup, error) {
	if tenantID == uuid.Nil {
		return model.Topup{}, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO topup_requests (tenant_id, user_id, amount, method, status, idempotency_key, proof_asset_id)
		SELECT $1::uuid, $2::uuid, $3::numeric, $4::text, 'pending', $5::text, $6::uuid
		 WHERE NOT EXISTS (SELECT 1 FROM orders WHERE proof_asset_id = $6::uuid)
		RETURNING id`,
		tenantID, t.UserID, t.Amount, t.Method, t.Key, t.ProofAssetID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Topup{}, ErrProofTaken
	}
	if err != nil {
		return model.Topup{}, mapTopupError("insert top-up", err)
	}
	return r.FindTx(ctx, tx, tenantID, id)
}

// ApproveTx and RejectTx only move a row that is still pending; the caller already holds its lock.
func (r *TopupRepository) ApproveTx(ctx context.Context, tx pgx.Tx, tenantID, id, reviewer uuid.UUID) error {
	return r.transition(ctx, tx, tenantID, id, `
		UPDATE topup_requests SET status = 'approved', approved_at = now(), reviewed_by = $3, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, reviewer)
}

func (r *TopupRepository) RejectTx(ctx context.Context, tx pgx.Tx, tenantID, id, reviewer uuid.UUID, reason string) error {
	return r.transition(ctx, tx, tenantID, id, `
		UPDATE topup_requests SET status = 'rejected', rejected_at = now(), reviewed_by = $3, reject_reason = $4,
		       updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, reviewer, reason)
}

func (r *TopupRepository) transition(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, sql string, args ...any) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, sql, append([]any{tenantID, id}, args...)...)
	if err != nil {
		return mapTopupError("move top-up", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TopupRepository) ListForUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.Topup, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	return r.list(ctx, []string{"t.tenant_id = $1", "t.user_id = $2"}, []any{tenantID, userID}, limit, offset)
}

func (r *TopupRepository) List(ctx context.Context, tenantID uuid.UUID, filter TopupFilter) ([]model.Topup, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"t.tenant_id = $1"}
	args := []any{tenantID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("t.status = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, likePattern(filter.Search))
		where = append(where, fmt.Sprintf(
			`('TUP-' || lpad(t.number::text, 6, '0') ILIKE $%[1]d ESCAPE '\' OR u.name ILIKE $%[1]d ESCAPE '\')`, len(args)))
	}
	return r.list(ctx, where, args, filter.Limit, filter.Offset)
}

func (r *TopupRepository) list(ctx context.Context, where []string, args []any, limit, offset int) ([]model.Topup, int, error) {
	from := topupFrom + ` WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count top-ups: %w", err)
	}
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, `SELECT `+topupColumns+from+
		fmt.Sprintf(` ORDER BY t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list top-ups: %w", err)
	}
	defer rows.Close()
	out := make([]model.Topup, 0, limit)
	for rows.Next() {
		t, err := scanTopup(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list top-ups: %w", err)
	}
	return out, total, nil
}

func (r *TopupRepository) find(ctx context.Context, q queryer, where string, forUpdate bool, args ...any) (model.Topup, error) {
	sql := `SELECT ` + topupColumns + topupFrom + ` WHERE ` + where
	if forUpdate {
		// Only the request row is locked; the student name comes from a plain join.
		sql += ` FOR UPDATE OF t`
	}
	return scanTopup(q.QueryRow(ctx, sql, args...))
}

func scanTopup(row pgx.Row) (model.Topup, error) {
	var t model.Topup
	err := row.Scan(&t.ID, &t.Number, &t.TenantID, &t.UserID, &t.StudentName, &t.Amount, &t.Method, &t.Status,
		&t.Key, &t.ProofAssetID, &t.RejectReason, &t.ReviewedBy, &t.ApprovedAt, &t.RejectedAt,
		&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Topup{}, ErrNotFound
	}
	if err != nil {
		return model.Topup{}, fmt.Errorf("scan top-up: %w", err)
	}
	return t, nil
}

func mapTopupError(what string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "topup_requests_proof_key" {
		return ErrProofTaken
	}
	return fmt.Errorf("%s: %w", what, err)
}
