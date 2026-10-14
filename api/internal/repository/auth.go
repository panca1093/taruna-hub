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

const sessionColumns = `id, user_id, tenant_id, refresh_hash, created_at, last_activity_at, revoked_at`

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

func (r *SessionRepository) FindByRefreshHash(ctx context.Context, hash []byte) (model.Session, error) {
	row := r.db.QueryRow(ctx, `SELECT `+sessionColumns+` FROM auth_sessions WHERE refresh_hash = $1`, hash)
	return scanSession(row)
}

// Scoped by tenant so a token stolen for another tenant can never end a session here.
func (r *SessionRepository) RevokeByRefreshHash(ctx context.Context, hash []byte, tenantID *uuid.UUID, at time.Time) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = $3
		  WHERE refresh_hash = $1 AND tenant_id IS NOT DISTINCT FROM $2 AND revoked_at IS NULL`,
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
	err := row.Scan(&s.ID, &s.UserID, &s.TenantID, &s.RefreshHash, &s.CreatedAt, &s.LastActivityAt, &s.RevokedAt)
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
