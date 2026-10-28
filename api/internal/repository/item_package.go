package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrDuplicateName is the per-tenant package name.
var ErrDuplicateName = fmt.Errorf("%w: name", ErrDuplicate)

// itemPackageSelect joins the current revision's size and the pins P10's tests hold on the package.
const itemPackageSelect = `
	SELECT p.id, p.tenant_id, p.type, p.name, p.current_revision, r.element_count,
	       COALESCE((SELECT max(u.required_count) FROM kecermatan_test_packages u WHERE u.package_id = p.id), 0),
	       EXISTS (SELECT 1 FROM kecermatan_test_packages u WHERE u.package_id = p.id),
	       p.created_at, p.updated_at
	  FROM item_packages p
	  JOIN item_package_revisions r ON r.package_id = p.id AND r.revision = p.current_revision`

type ItemPackageRepository struct {
	db *pgxpool.Pool
}

func NewItemPackageRepository(db *pgxpool.Pool) *ItemPackageRepository {
	return &ItemPackageRepository{db: db}
}

func scanItemPackage(row pgx.Row) (model.ItemPackage, error) {
	var p model.ItemPackage
	err := row.Scan(&p.ID, &p.TenantID, &p.Type, &p.Name, &p.Revision, &p.ElementCount,
		&p.RequiredCount, &p.InUse, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func itemPackageError(err error, what string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "item_packages_name_uniq":
			return ErrDuplicateName
		case pgErr.Code == pgForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrReferenced, what)
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}

// CreateTx writes the package and its empty first revision.
func (r *ItemPackageRepository) CreateTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, kind, name string) (model.ItemPackage, error) {
	if tenantID == uuid.Nil {
		return model.ItemPackage{}, ErrScopeRequired
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO item_packages (tenant_id, type, name, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		tenantID, kind, name, createdBy).Scan(&id); err != nil {
		return model.ItemPackage{}, itemPackageError(err, "insert item package")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO item_package_revisions (package_id, revision, element_count, created_by) VALUES ($1, 1, 0, $2)`,
		id, createdBy); err != nil {
		return model.ItemPackage{}, itemPackageError(err, "insert first revision")
	}
	return r.Find(ctx, tx, tenantID, id)
}

// Find answers ErrNotFound for another tenant's id exactly as for an unknown one.
func (r *ItemPackageRepository) Find(ctx context.Context, q queryer, tenantID, id uuid.UUID) (model.ItemPackage, error) {
	if tenantID == uuid.Nil {
		return model.ItemPackage{}, ErrScopeRequired
	}
	p, err := scanItemPackage(q.QueryRow(ctx, itemPackageSelect+` WHERE p.tenant_id = $1 AND p.id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ItemPackage{}, ErrNotFound
	}
	if err != nil {
		return model.ItemPackage{}, fmt.Errorf("find item package: %w", err)
	}
	return p, nil
}

// Lock takes the package row for the rest of the transaction, so concurrent writers and P10's pinning queue up.
func (r *ItemPackageRepository) Lock(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.ItemPackage, error) {
	if tenantID == uuid.Nil {
		return model.ItemPackage{}, ErrScopeRequired
	}
	var locked uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM item_packages WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ItemPackage{}, ErrNotFound
	}
	if err != nil {
		return model.ItemPackage{}, fmt.Errorf("lock item package: %w", err)
	}
	return r.Find(ctx, tx, tenantID, id)
}

func (r *ItemPackageRepository) List(ctx context.Context, tenantID uuid.UUID, kind string) ([]model.ItemPackage, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, itemPackageSelect+`
		WHERE p.tenant_id = $1 AND ($2 = '' OR p.type = $2)
		ORDER BY lower(p.name), p.id`, tenantID, kind)
	if err != nil {
		return nil, fmt.Errorf("list item packages: %w", err)
	}
	defer rows.Close()
	out := []model.ItemPackage{}
	for rows.Next() {
		p, err := scanItemPackage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan item package: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Elements reads one frozen revision in order; the package is scoped by tenant, so a foreign id is empty.
func (r *ItemPackageRepository) Elements(ctx context.Context, q queryer, tenantID, packageID uuid.UUID, revision int) ([]model.ItemElement, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT e.element_id, e.position, e.kind, e.key, e.value, e.label, e.asset_id
		  FROM item_package_elements e
		  JOIN item_packages p ON p.id = e.package_id
		 WHERE p.tenant_id = $1 AND e.package_id = $2 AND e.revision = $3
		 ORDER BY e.position`, tenantID, packageID, revision)
	if err != nil {
		return nil, fmt.Errorf("list package elements: %w", err)
	}
	defer rows.Close()
	out := []model.ItemElement{}
	for rows.Next() {
		var e model.ItemElement
		if err := rows.Scan(&e.ID, &e.Position, &e.Kind, &e.Key, &e.Value, &e.Label, &e.AssetID); err != nil {
			return nil, fmt.Errorf("scan package element: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// WriteRevisionTx freezes elements as the package's next revision; the caller holds the package lock.
func (r *ItemPackageRepository) WriteRevisionTx(ctx context.Context, tx pgx.Tx, tenantID, packageID, createdBy uuid.UUID, revision int, elements []model.ItemElement) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO item_package_revisions (package_id, revision, element_count, created_by) VALUES ($1, $2, $3, $4)`,
		packageID, revision, len(elements), createdBy); err != nil {
		return itemPackageError(err, "insert revision")
	}
	for at, e := range elements {
		if _, err := tx.Exec(ctx, `
			INSERT INTO item_package_elements (package_id, revision, element_id, position, kind, key, value, label, asset_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			packageID, revision, e.ID, at, e.Kind, e.Key, e.Value, e.Label, e.AssetID); err != nil {
			return itemPackageError(err, "insert element")
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE item_packages SET current_revision = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, packageID, revision); err != nil {
		return itemPackageError(err, "advance revision")
	}
	return nil
}

func (r *ItemPackageRepository) RenameTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, name string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `UPDATE item_packages SET name = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, name)
	if err != nil {
		return itemPackageError(err, "rename item package")
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTx removes the package with every revision; a pinned revision makes the foreign key refuse it.
func (r *ItemPackageRepository) DeleteTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `DELETE FROM item_packages WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return itemPackageError(err, "delete item package")
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
