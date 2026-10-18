package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrScopeRequired = errors.New("tenant scope required")
	// Both wrap ErrDuplicate; registration needs to say which identifier is already taken.
	ErrDuplicateEmail = fmt.Errorf("%w: email", ErrDuplicate)
	ErrDuplicatePhone = fmt.Errorf("%w: phone", ErrDuplicate)
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, tenant_id, name, email, phone, password_hash, role, balance::text, created_at, is_active, email_verified_at`

func (r *UserRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanUser(row)
}

// FindPlatformByID is the platform-scope counterpart of FindByID: the only null-tenant read by id.
func (r *UserRepository) FindPlatformByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id IS NULL AND id = $1`, id)
	return scanUser(row)
}

// FindByIdentifier backs FR-AUTH-02: one credential field matching either column.
func (r *UserRepository) FindByIdentifier(ctx context.Context, tenantID uuid.UUID, identifier string) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND (email = $2 OR phone = $2)`,
		tenantID, identifier)
	return scanUser(row)
}

// FindPlatformByIdentifier is the only entry point that reads null-tenant accounts.
func (r *UserRepository) FindPlatformByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id IS NULL AND (email = $1 OR phone = $1)`,
		identifier)
	return scanUser(row)
}

func (r *UserRepository) Create(ctx context.Context, u model.User) (model.User, error) {
	return insertUser(ctx, r.db, u)
}

// CreateTx is Create inside the caller's transaction, so an account and what made it commit together.
func (r *UserRepository) CreateTx(ctx context.Context, tx pgx.Tx, u model.User) (model.User, error) {
	return insertUser(ctx, tx, u)
}

// ReplacePasswordTx swaps the hash only while it is still the one the caller saw, so a reset link
// works once and two racing submits cannot both win. Following an emailed link proves the inbox,
// so it also verifies the email and retires any outstanding code.
func (r *UserRepository) ReplacePasswordTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, current, next string, at time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE users
		    SET password_hash = $4, email_verified_at = coalesce(email_verified_at, $5),
		        email_code_hash = NULL, email_code_expires_at = NULL, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 AND password_hash = $3`,
		id, tenantID, current, next, at)
	if err != nil {
		return false, fmt.Errorf("replace password: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// FindEmailCodeForUpdateTx locks the account row, so concurrent guesses are counted one at a time.
func (r *UserRepository) FindEmailCodeForUpdateTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID) (model.EmailCode, error) {
	var code model.EmailCode
	err := tx.QueryRow(ctx,
		`SELECT email_code_hash, email_code_expires_at, email_code_attempts, email_verified_at
		   FROM users WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 FOR UPDATE`,
		id, tenantID).Scan(&code.Hash, &code.ExpiresAt, &code.Attempts, &code.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.EmailCode{}, ErrNotFound
	}
	if err != nil {
		return model.EmailCode{}, fmt.Errorf("read email code: %w", err)
	}
	return code, nil
}

// SetEmailCodeTx replaces the outstanding code and restores its guess budget.
func (r *UserRepository) SetEmailCodeTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, hash []byte, expiresAt time.Time) error {
	return execOne(ctx, tx, "set email code",
		`UPDATE users SET email_code_hash = $3, email_code_expires_at = $4, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID, hash, expiresAt)
}

func (r *UserRepository) CountEmailCodeAttemptTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID) error {
	return execOne(ctx, tx, "count email code attempt",
		`UPDATE users SET email_code_attempts = email_code_attempts + 1
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID)
}

func (r *UserRepository) MarkEmailVerifiedTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, at time.Time) error {
	return execOne(ctx, tx, "mark email verified",
		`UPDATE users SET email_verified_at = $3,
		        email_code_hash = NULL, email_code_expires_at = NULL, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID, at)
}

func execOne(ctx context.Context, tx pgx.Tx, what, sql string, args ...any) error {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertUser(ctx context.Context, q rowQuerier, u model.User) (model.User, error) {
	row := q.QueryRow(ctx,
		`INSERT INTO users (tenant_id, name, email, phone, password_hash, role, email_verified_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+userColumns,
		u.TenantID, u.Name, u.Email, u.Phone, u.PasswordHash, u.Role, u.EmailVerifiedAt)
	return scanUser(row)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.Balance, &u.CreatedAt, &u.IsActive,
		&u.EmailVerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.User{}, duplicateUser(err)
	}
	if err != nil {
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

func duplicateUser(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "users_tenant_email_key", "users_platform_email_key":
			return ErrDuplicateEmail
		case "users_tenant_phone_key", "users_platform_phone_key":
			return ErrDuplicatePhone
		}
	}
	return ErrDuplicate
}
