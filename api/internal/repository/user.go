package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrScopeRequired = errors.New("tenant scope required")
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, tenant_id, name, email, phone, password_hash, role, balance, created_at, is_active`

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
	row := r.db.QueryRow(ctx,
		`INSERT INTO users (tenant_id, name, email, phone, password_hash, role)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+userColumns,
		u.TenantID, u.Name, u.Email, u.Phone, u.PasswordHash, u.Role)
	return scanUser(row)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.Balance, &u.CreatedAt, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.User{}, ErrDuplicate
	}
	if err != nil {
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
