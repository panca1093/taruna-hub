package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// SetWeights upserts the tenant's scoring weights; sessions already started keep the copy they froze.
func (r *RuntimeRepository) SetWeights(ctx context.Context, tenantID, updatedBy uuid.UUID, w model.Weights) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO tenant_settings (tenant_id, weight_easy, weight_medium, weight_hard, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (tenant_id)
		DO UPDATE SET weight_easy = EXCLUDED.weight_easy, weight_medium = EXCLUDED.weight_medium,
		              weight_hard = EXCLUDED.weight_hard, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		tenantID, w.Easy, w.Medium, w.Hard, updatedBy)
	if err != nil {
		return fmt.Errorf("set weights: %w", err)
	}
	return nil
}
