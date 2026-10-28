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

const assetColumns = `id, tenant_id, purpose, object_key, content_type, size_bytes, status,
	uploaded_by, created_at, finalized_at`

type AssetRepository struct {
	db *pgxpool.Pool
}

func NewAssetRepository(db *pgxpool.Pool) *AssetRepository {
	return &AssetRepository{db: db}
}

func scanAsset(row pgx.Row) (model.Asset, error) {
	var a model.Asset
	err := row.Scan(&a.ID, &a.TenantID, &a.Purpose, &a.ObjectKey, &a.ContentType, &a.SizeBytes,
		&a.Status, &a.UploadedBy, &a.CreatedAt, &a.FinalizedAt)
	return a, err
}

// Insert always writes a staged row; the caller chose the id because the object key is built from it.
func (r *AssetRepository) Insert(ctx context.Context, tenantID uuid.UUID, a model.Asset) (model.Asset, error) {
	if tenantID == uuid.Nil {
		return model.Asset{}, ErrScopeRequired
	}
	created, err := scanAsset(r.db.QueryRow(ctx, `
		INSERT INTO assets (id, tenant_id, purpose, object_key, content_type, size_bytes, status, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, 'staged', $7)
		RETURNING `+assetColumns,
		a.ID, tenantID, a.Purpose, a.ObjectKey, a.ContentType, a.SizeBytes, a.UploadedBy))
	if err != nil {
		return model.Asset{}, fmt.Errorf("insert asset: %w", err)
	}
	return created, nil
}

// Find answers ErrNotFound for another tenant's id exactly as for an unknown one.
func (r *AssetRepository) Find(ctx context.Context, q queryer, tenantID, id uuid.UUID) (model.Asset, error) {
	if tenantID == uuid.Nil {
		return model.Asset{}, ErrScopeRequired
	}
	a, err := scanAsset(q.QueryRow(ctx,
		`SELECT `+assetColumns+` FROM assets WHERE tenant_id = $1 AND id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Asset{}, ErrNotFound
	}
	if err != nil {
		return model.Asset{}, fmt.Errorf("find asset: %w", err)
	}
	return a, nil
}

// KecermatanItemVisible reports whether the student's own kecermatan attempt currently shows the asset: in a
// completed attempt any element of the snapshot (the review), in one in progress only the reference elements of the
// running kolom's column. A paused or finished kolom, a later kolom and other students' attempts show nothing.
func (r *AssetRepository) KecermatanItemVisible(ctx context.Context, q queryer, tenantID, userID, assetID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var ok bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM kecermatan_sessions s
		   WHERE s.tenant_id = $1 AND s.user_id = $2 AND (
		     (s.status = 'completed'
		      AND jsonb_path_exists(s.columns, '$[*].reference[*].elements[*] ? (@.asset_id == $id)',
		                            jsonb_build_object('id', $3::text)))
		     OR (s.status = 'in_progress' AND EXISTS (
		         SELECT 1 FROM kecermatan_session_kolom k
		          WHERE k.session_id = s.id AND k.status = 'running'
		            AND jsonb_path_exists(s.columns -> ((k.kolom_number - 1) % (s.kolom_count / 2)),
		                                  '$.reference[*].elements[*] ? (@.asset_id == $id)',
		                                  jsonb_build_object('id', $3::text))))))`,
		tenantID, userID, assetID.String()).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check kecermatan item visibility: %w", err)
	}
	return ok, nil
}

// Finalize is idempotent: a final row keeps its first finalized_at.
func (r *AssetRepository) Finalize(ctx context.Context, q queryer, tenantID, id uuid.UUID) (model.Asset, error) {
	if tenantID == uuid.Nil {
		return model.Asset{}, ErrScopeRequired
	}
	a, err := scanAsset(q.QueryRow(ctx, `
		UPDATE assets SET status = 'final', finalized_at = COALESCE(finalized_at, now())
		 WHERE tenant_id = $1 AND id = $2
		RETURNING `+assetColumns, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Asset{}, ErrNotFound
	}
	if err != nil {
		return model.Asset{}, fmt.Errorf("finalize asset: %w", err)
	}
	return a, nil
}

// StagedBefore is the bounded purge scan across tenants, reachable from no caller-supplied scope.
func (r *AssetRepository) StagedBefore(ctx context.Context, cutoff time.Time, limit int) ([]model.Asset, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+assetColumns+` FROM assets
		 WHERE status = 'staged' AND created_at < $1
		 ORDER BY created_at, id
		 LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("scan staged assets: %w", err)
	}
	defer rows.Close()

	var out []model.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, fmt.Errorf("scan staged asset: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LockStaged takes the row only while it is still staged and old enough; a concurrent finalize
// makes it vanish from here, and a concurrent purge makes it skip.
func (r *AssetRepository) LockStaged(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, cutoff time.Time) (model.Asset, error) {
	if tenantID == uuid.Nil {
		return model.Asset{}, ErrScopeRequired
	}
	a, err := scanAsset(tx.QueryRow(ctx, `
		SELECT `+assetColumns+` FROM assets
		 WHERE tenant_id = $1 AND id = $2 AND status = 'staged' AND created_at < $3
		   FOR UPDATE SKIP LOCKED`, tenantID, id, cutoff))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Asset{}, ErrNotFound
	}
	if err != nil {
		return model.Asset{}, fmt.Errorf("lock staged asset: %w", err)
	}
	return a, nil
}

// DeleteStaged never removes a final row, whatever id it is given.
func (r *AssetRepository) DeleteStaged(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM assets WHERE tenant_id = $1 AND id = $2 AND status = 'staged'`, tenantID, id); err != nil {
		return fmt.Errorf("delete staged asset: %w", err)
	}
	return nil
}
