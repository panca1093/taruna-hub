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

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

const sessionColumns = `id, user_id, tenant_id, refresh_hash, created_at, last_activity_at, revoked_at, previous_refresh_hash, rotated_at`

func (r *SessionRepository) Create(ctx context.Context, s model.Session) (model.Session, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO auth_sessions (user_id, tenant_id, refresh_hash, created_at, last_activity_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING `+sessionColumns,
		s.UserID, s.TenantID, s.RefreshHash, s.CreatedAt, s.LastActivityAt)
	return scanSession(row)
}

// FindByID is unscoped on purpose: the caller already holds a signed session id and compares tenants itself.
func (r *SessionRepository) FindByID(ctx context.Context, id uuid.UUID) (model.Session, error) {
	row := r.db.QueryRow(ctx, `SELECT `+sessionColumns+` FROM auth_sessions WHERE id = $1`, id)
	return scanSession(row)
}

// FindByRefreshHash matches the current token or the one the last rotation replaced; the caller tells them apart.
func (r *SessionRepository) FindByRefreshHash(ctx context.Context, hash []byte) (model.Session, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM auth_sessions WHERE refresh_hash = $1 OR previous_refresh_hash = $1`, hash)
	return scanSession(row)
}

// Rotate swaps in a new token only if current is still the live one, so two racing refreshes rotate once.
func (r *SessionRepository) Rotate(ctx context.Context, id uuid.UUID, current, next []byte, at time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE auth_sessions
		    SET previous_refresh_hash = refresh_hash, refresh_hash = $3, rotated_at = $4
		  WHERE id = $1 AND refresh_hash = $2 AND revoked_at IS NULL`,
		id, current, next, at)
	if err != nil {
		return false, fmt.Errorf("rotate session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	if _, err := r.db.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, at); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeAllForUserTx ends every live session of one account inside the caller's transaction.
func (r *SessionRepository) RevokeAllForUserTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, at time.Time) error {
	if _, err := tx.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, at); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

// Scoped by tenant so a token stolen for another tenant can never end a session here.
func (r *SessionRepository) RevokeByRefreshHash(ctx context.Context, hash []byte, tenantID *uuid.UUID, at time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = $3
		  WHERE (refresh_hash = $1 OR previous_refresh_hash = $1)
		    AND tenant_id IS NOT DISTINCT FROM $2 AND revoked_at IS NULL`,
		hash, tenantID, at)
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// One conditional write, so a burst of requests costs at most one update per window.
func (r *SessionRepository) Touch(ctx context.Context, id uuid.UUID, staleBefore, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE auth_sessions SET last_activity_at = $3 WHERE id = $1 AND last_activity_at < $2`,
		id, staleBefore, at)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func scanSession(row scannable) (model.Session, error) {
	var s model.Session
	err := row.Scan(&s.ID, &s.UserID, &s.TenantID, &s.RefreshHash, &s.CreatedAt, &s.LastActivityAt, &s.RevokedAt,
		&s.PreviousRefreshHash, &s.RotatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Session{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.Session{}, ErrDuplicate
	}
	if err != nil {
		return model.Session{}, fmt.Errorf("scan session: %w", err)
	}
	return s, nil
}
