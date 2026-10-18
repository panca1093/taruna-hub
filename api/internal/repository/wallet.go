package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// WalletFilter is already validated by the service; the repository only turns it into SQL.
type WalletFilter struct {
	Search string
	Limit  int
	Offset int
}

type WalletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{db: db}
}

// The actor's name is joined for staff rows only; a student must never see a staff email or phone.
const walletEntryColumns = `w.id, w.seq, w.tenant_id, w.user_id, w.amount::text,
	w.balance_after::text, w.kind, w.actor_kind, w.actor_id,
	CASE WHEN w.actor_kind = 'staff' THEN a.name END, w.reason, w.source_type, w.source_id, w.created_at`

const walletEntryFrom = ` FROM wallet_entries w
	LEFT JOIN users a ON a.id = w.actor_id AND a.tenant_id = w.tenant_id`

// LockStudentTx takes the projection's row lock, which is what serializes every movement of a wallet.
func (r *WalletRepository) LockStudentTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (string, error) {
	if tenantID == uuid.Nil {
		return "", ErrScopeRequired
	}
	var balance string
	err := tx.QueryRow(ctx,
		`SELECT balance::text FROM users WHERE tenant_id = $1 AND id = $2 AND role = 'student' FOR UPDATE`,
		tenantID, userID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock wallet: %w", err)
	}
	return balance, nil
}

// AdjustBalanceTx moves the projection only while it stays non-negative, and returns the new balance.
func (r *WalletRepository) AdjustBalanceTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, amount string) (string, bool, error) {
	if tenantID == uuid.Nil {
		return "", false, ErrScopeRequired
	}
	var balance string
	err := tx.QueryRow(ctx, `
		UPDATE users SET balance = balance + $3::numeric
		 WHERE tenant_id = $1 AND id = $2 AND balance + $3::numeric >= 0
		RETURNING balance::text`, tenantID, userID, amount).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("move wallet balance: %w", err)
	}
	return balance, true, nil
}

// FindBySource reads the row one business source already wrote, if any.
func (r *WalletRepository) FindBySource(ctx context.Context, q queryer, tenantID uuid.UUID, sourceType, sourceID string) (model.WalletEntry, error) {
	if tenantID == uuid.Nil {
		return model.WalletEntry{}, ErrScopeRequired
	}
	entry, err := scanWalletEntry(q.QueryRow(ctx, `SELECT `+walletEntryColumns+walletEntryFrom+
		` WHERE w.tenant_id = $1 AND w.source_type = $2 AND w.source_id = $3`,
		tenantID, sourceType, sourceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.WalletEntry{}, ErrNotFound
	}
	return entry, err
}

// InsertEntryTx never raises on a duplicate source: it reports the winner's row instead.
func (r *WalletRepository) InsertEntryTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, entry model.WalletEntry) (model.WalletEntry, bool, error) {
	if tenantID == uuid.Nil {
		return model.WalletEntry{}, false, ErrScopeRequired
	}
	created, err := scanWalletEntry(tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO wallet_entries (tenant_id, user_id, amount, balance_after, kind, actor_kind,
			        actor_id, reason, source_type, source_id)
			VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (tenant_id, source_type, source_id) DO NOTHING
			RETURNING *
		)
		SELECT `+walletEntryColumns+` FROM inserted w
		  LEFT JOIN users a ON a.id = w.actor_id AND a.tenant_id = w.tenant_id`,
		tenantID, entry.UserID, entry.Amount, entry.BalanceAfter, entry.Kind, entry.ActorKind,
		entry.ActorID, entry.Reason, entry.SourceType, entry.SourceID))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.WalletEntry{}, false, fmt.Errorf("insert wallet entry: %w", err)
	}
	existing, findErr := r.FindBySource(ctx, tx, tenantID, entry.SourceType, entry.SourceID)
	if findErr != nil {
		return model.WalletEntry{}, false, findErr
	}
	return existing, false, nil
}

// Wallet is the projection plus the two ledger totals, all computed in SQL numeric.
func (r *WalletRepository) Wallet(ctx context.Context, tenantID, userID uuid.UUID) (model.Wallet, error) {
	if tenantID == uuid.Nil {
		return model.Wallet{}, ErrScopeRequired
	}
	var out model.Wallet
	err := r.db.QueryRow(ctx, `
		SELECT u.balance::text,
		       COALESCE(sum(w.amount) FILTER (WHERE w.amount > 0), 0)::numeric(14,2)::text,
		       COALESCE(-sum(w.amount) FILTER (WHERE w.amount < 0), 0)::numeric(14,2)::text
		  FROM users u
		  LEFT JOIN wallet_entries w ON w.user_id = u.id AND w.tenant_id = u.tenant_id
		 WHERE u.tenant_id = $1 AND u.id = $2 AND u.role = 'student'
		 GROUP BY u.balance`, tenantID, userID).Scan(&out.Balance, &out.TotalCredits, &out.TotalDebits)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Wallet{}, ErrNotFound
	}
	if err != nil {
		return model.Wallet{}, fmt.Errorf("read wallet: %w", err)
	}
	return out, nil
}

// Entries lists the ledger newest first and counts every row, both scoped to the tenant and user.
func (r *WalletRepository) Entries(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.WalletEntry, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	const where = ` WHERE w.tenant_id = $1 AND w.user_id = $2`
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM wallet_entries w`+where, tenantID, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count wallet entries: %w", err)
	}
	rows, err := r.db.Query(ctx, `SELECT `+walletEntryColumns+walletEntryFrom+where+
		` ORDER BY w.seq DESC LIMIT $3 OFFSET $4`, tenantID, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list wallet entries: %w", err)
	}
	defer rows.Close()

	out := make([]model.WalletEntry, 0, limit)
	for rows.Next() {
		entry, err := scanWalletEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list wallet entries: %w", err)
	}
	return out, total, nil
}

// Student reads one student of this tenant; another tenant's id or a staff role is simply absent.
func (r *WalletRepository) Student(ctx context.Context, tenantID, userID uuid.UUID) (model.WalletStudent, error) {
	if tenantID == uuid.Nil {
		return model.WalletStudent{}, ErrScopeRequired
	}
	student, err := scanWalletStudent(r.db.QueryRow(ctx, `
		SELECT id, name, email, phone, balance::text, created_at
		  FROM users WHERE tenant_id = $1 AND id = $2 AND role = 'student'`, tenantID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.WalletStudent{}, ErrNotFound
	}
	return student, err
}

// Students lists the tenant's students newest first; `%` and `_` in the search match themselves.
func (r *WalletRepository) Students(ctx context.Context, tenantID uuid.UUID, filter WalletFilter) ([]model.WalletStudent, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"tenant_id = $1", "role = 'student'"}
	args := []any{tenantID}
	if filter.Search != "" {
		args = append(args, likePattern(filter.Search))
		where = append(where, fmt.Sprintf(
			`(name ILIKE $%d ESCAPE '\' OR email ILIKE $%d ESCAPE '\' OR phone ILIKE $%d ESCAPE '\')`,
			len(args), len(args), len(args)))
	}
	from := " FROM users WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count students: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT id, name, email, phone, balance::text, created_at`+from+
		fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list students: %w", err)
	}
	defer rows.Close()

	out := make([]model.WalletStudent, 0, filter.Limit)
	for rows.Next() {
		student, err := scanWalletStudent(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, student)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list students: %w", err)
	}
	return out, total, nil
}

type walletScanner interface {
	Scan(dest ...any) error
}

func scanWalletEntry(row walletScanner) (model.WalletEntry, error) {
	var out model.WalletEntry
	var actorName *string
	var createdAt time.Time
	err := row.Scan(&out.ID, &out.Seq, &out.TenantID, &out.UserID, &out.Amount, &out.BalanceAfter,
		&out.Kind, &out.ActorKind, &out.ActorID, &actorName, &out.Reason, &out.SourceType,
		&out.SourceID, &createdAt)
	if err != nil {
		return model.WalletEntry{}, err
	}
	out.ActorName, out.CreatedAt = actorName, createdAt
	return out, nil
}

func scanWalletStudent(row walletScanner) (model.WalletStudent, error) {
	var out model.WalletStudent
	var createdAt time.Time
	if err := row.Scan(&out.ID, &out.Name, &out.Email, &out.Phone, &out.Balance, &createdAt); err != nil {
		return model.WalletStudent{}, err
	}
	out.CreatedAt = createdAt
	return out, nil
}
