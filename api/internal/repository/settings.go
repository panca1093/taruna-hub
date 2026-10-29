package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// SetSettings upserts the tenant's settings; sessions already started keep the weights they froze, and a
// top-up request already made keeps the minimum it was checked against. An empty minTopup keeps the current one.
func (r *RuntimeRepository) SetSettings(ctx context.Context, tenantID, updatedBy uuid.UUID, w model.Weights, minTopup string) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO tenant_settings (tenant_id, weight_easy, weight_medium, weight_hard, min_topup, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5, '')::numeric, 10000), $6, now())
		ON CONFLICT (tenant_id)
		DO UPDATE SET weight_easy = EXCLUDED.weight_easy, weight_medium = EXCLUDED.weight_medium,
		              weight_hard = EXCLUDED.weight_hard, min_topup = COALESCE(NULLIF($5, '')::numeric, tenant_settings.min_topup),
		              updated_by = EXCLUDED.updated_by, updated_at = now()`,
		tenantID, w.Easy, w.Medium, w.Hard, minTopup, updatedBy)
	if err != nil {
		return fmt.Errorf("set settings: %w", err)
	}
	return nil
}

// MinTopup is the tenant's minimum top-up, or the D12 default when it never saved one.
func (r *RuntimeRepository) MinTopup(ctx context.Context, q dbtx, tenantID uuid.UUID) (string, error) {
	return minTopup(ctx, q, tenantID)
}

func minTopup(ctx context.Context, q queryer, tenantID uuid.UUID) (string, error) {
	if tenantID == uuid.Nil {
		return "", ErrScopeRequired
	}
	var minimum string
	err := q.QueryRow(ctx, `SELECT min_topup::text FROM tenant_settings WHERE tenant_id = $1`, tenantID).Scan(&minimum)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefaultMinTopup, nil
	}
	if err != nil {
		return "", fmt.Errorf("read minimum top-up: %w", err)
	}
	return minimum, nil
}
