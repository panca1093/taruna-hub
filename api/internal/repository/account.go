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

// AccountRepository persists the steps before and around an account: pending signups and resets.
type AccountRepository struct {
	db *pgxpool.Pool
}

func NewAccountRepository(db *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{db: db}
}

const registrationColumns = `id, tenant_id, email, phone, name, password_hash, code_id, code_hash, attempts, expires_at, sent_at, created_at`

// UpsertRegistrationTx replaces any pending signup for the same email: only the inbox owner can finish either.
func (r *AccountRepository) UpsertRegistrationTx(ctx context.Context, tx pgx.Tx, reg model.Registration) (model.Registration, error) {
	if reg.TenantID == uuid.Nil {
		return model.Registration{}, ErrScopeRequired
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO auth_registrations
			(tenant_id, email, phone, name, password_hash, code_id, code_hash, attempts, expires_at, sent_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9, $9)
		 ON CONFLICT (tenant_id, email) DO UPDATE
		    SET phone = EXCLUDED.phone, name = EXCLUDED.name, password_hash = EXCLUDED.password_hash,
		        code_id = EXCLUDED.code_id, code_hash = EXCLUDED.code_hash, attempts = 0,
		        expires_at = EXCLUDED.expires_at, sent_at = EXCLUDED.sent_at
		 RETURNING `+registrationColumns,
		reg.TenantID, reg.Email, reg.Phone, reg.Name, reg.PasswordHash, reg.CodeID, reg.CodeHash, reg.ExpiresAt, reg.SentAt)
	return scanRegistration(row)
}

// FindRegistrationForUpdateTx locks the row, so concurrent guesses are counted one at a time.
func (r *AccountRepository) FindRegistrationForUpdateTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, email string) (model.Registration, error) {
	if tenantID == uuid.Nil {
		return model.Registration{}, ErrScopeRequired
	}
	row := tx.QueryRow(ctx,
		`SELECT `+registrationColumns+` FROM auth_registrations WHERE tenant_id = $1 AND email = $2 FOR UPDATE`,
		tenantID, email)
	return scanRegistration(row)
}

// ReplaceCodeTx issues a new code for the same signup and resets its guess budget.
func (r *AccountRepository) ReplaceCodeTx(ctx context.Context, tx pgx.Tx, tenantID, id, codeID uuid.UUID, codeHash []byte, expiresAt, sentAt time.Time) error {
	tag, err := tx.Exec(ctx,
		`UPDATE auth_registrations
		    SET code_id = $3, code_hash = $4, attempts = 0, expires_at = $5, sent_at = $6
		  WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, codeID, codeHash, expiresAt, sentAt)
	if err != nil {
		return fmt.Errorf("replace registration code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AccountRepository) CountAttemptTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`UPDATE auth_registrations SET attempts = attempts + 1 WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return fmt.Errorf("count registration attempt: %w", err)
	}
	return nil
}

func (r *AccountRepository) DeleteRegistrationTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM auth_registrations WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
		return fmt.Errorf("delete registration: %w", err)
	}
	return nil
}

func scanRegistration(row pgx.Row) (model.Registration, error) {
	var reg model.Registration
	err := row.Scan(&reg.ID, &reg.TenantID, &reg.Email, &reg.Phone, &reg.Name, &reg.PasswordHash,
		&reg.CodeID, &reg.CodeHash, &reg.Attempts, &reg.ExpiresAt, &reg.SentAt, &reg.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Registration{}, ErrNotFound
	}
	if err != nil {
		return model.Registration{}, fmt.Errorf("scan registration: %w", err)
	}
	return reg, nil
}

// CreateResetTx first retires the account's open resets, so only the newest link ever works.
func (r *AccountRepository) CreateResetTx(ctx context.Context, tx pgx.Tx, reset model.PasswordReset) error {
	if _, err := tx.Exec(ctx,
		`UPDATE auth_password_resets SET consumed_at = $2 WHERE user_id = $1 AND consumed_at IS NULL`,
		reset.UserID, reset.CreatedAt); err != nil {
		return fmt.Errorf("retire open resets: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO auth_password_resets (user_id, tenant_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		reset.UserID, reset.TenantID, reset.TokenHash, reset.ExpiresAt, reset.CreatedAt); err != nil {
		return fmt.Errorf("insert reset: %w", err)
	}
	return nil
}

// ConsumeResetTx is the single-use check and the use in one statement, so two racing submits cannot both win.
func (r *AccountRepository) ConsumeResetTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, hash []byte, at time.Time) (model.PasswordReset, error) {
	var reset model.PasswordReset
	err := tx.QueryRow(ctx,
		`UPDATE auth_password_resets SET consumed_at = $3
		  WHERE token_hash = $1 AND tenant_id IS NOT DISTINCT FROM $2
		    AND consumed_at IS NULL AND expires_at > $3
		  RETURNING id, user_id, tenant_id, token_hash, expires_at, consumed_at, created_at`,
		hash, tenantID, at,
	).Scan(&reset.ID, &reset.UserID, &reset.TenantID, &reset.TokenHash, &reset.ExpiresAt, &reset.ConsumedAt, &reset.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PasswordReset{}, ErrNotFound
	}
	if err != nil {
		return model.PasswordReset{}, fmt.Errorf("consume reset: %w", err)
	}
	return reset, nil
}

// ThrottleRepository keeps fixed-window counters; the caller hashes the key so no identifier is stored.
type ThrottleRepository struct {
	db *pgxpool.Pool
}

func NewThrottleRepository(db *pgxpool.Pool) *ThrottleRepository {
	return &ThrottleRepository{db: db}
}

// Hit counts one attempt in a single statement, so concurrent attempts can never under-count.
func (r *ThrottleRepository) Hit(ctx context.Context, keyHash []byte, window time.Duration, at time.Time) (int, time.Time, error) {
	var hits int
	var start time.Time
	err := r.db.QueryRow(ctx,
		`INSERT INTO auth_throttles (key_hash, window_start, hits) VALUES ($1, $2, 1)
		 ON CONFLICT (key_hash) DO UPDATE SET
		    hits = CASE WHEN auth_throttles.window_start <= $2 - make_interval(secs => $3)
		                THEN 1 ELSE auth_throttles.hits + 1 END,
		    window_start = CASE WHEN auth_throttles.window_start <= $2 - make_interval(secs => $3)
		                        THEN $2 ELSE auth_throttles.window_start END
		 RETURNING hits, window_start`,
		keyHash, at, window.Seconds(),
	).Scan(&hits, &start)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("hit throttle: %w", err)
	}
	return hits, start, nil
}

func (r *ThrottleRepository) Clear(ctx context.Context, keyHash []byte) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM auth_throttles WHERE key_hash = $1`, keyHash); err != nil {
		return fmt.Errorf("clear throttle: %w", err)
	}
	return nil
}
