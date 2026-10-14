package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var ErrDuplicate = errors.New("duplicate")

const pgUniqueViolation = "23505"

type TenantRepository struct {
	db *pgxpool.Pool
}

func NewTenantRepository(db *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{db: db}
}

const tenantColumns = `id, slug, name, is_active, created_at`

func (r *TenantRepository) Create(ctx context.Context, slug, name string) (model.Tenant, error) {
	row := r.db.QueryRow(ctx,
		`INSERT INTO tenants (slug, name) VALUES ($1, $2) RETURNING `+tenantColumns, slug, name)
	return scanTenant(row)
}

// FindBySlug returns inactive tenants too; callers decide whether that matters.
func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (model.Tenant, error) {
	row := r.db.QueryRow(ctx, `SELECT `+tenantColumns+` FROM tenants WHERE slug = $1`, slug)
	return scanTenant(row)
}

func scanTenant(row scannable) (model.Tenant, error) {
	var t model.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.IsActive, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Tenant{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.Tenant{}, ErrDuplicate
	}
	if err != nil {
		return model.Tenant{}, fmt.Errorf("scan tenant: %w", err)
	}
	return t, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
