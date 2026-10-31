package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *RuntimeRepository) DashboardAccount(ctx context.Context, q dbtx, tenantID, userID uuid.UUID) (int, string, error) {
	if tenantID == uuid.Nil {
		return 0, "", ErrScopeRequired
	}
	var owned int
	var balance string
	err := q.QueryRow(ctx, `SELECT u.balance::text,
		(SELECT count(*) FROM enrollments e WHERE e.tenant_id = u.tenant_id AND e.user_id = u.id AND e.tryout_id IS NOT NULL)
		FROM users u WHERE u.tenant_id = $1 AND u.id = $2 AND u.role = 'student'`, tenantID, userID).Scan(&balance, &owned)
	if err != nil {
		return 0, "", fmt.Errorf("read dashboard account: %w", err)
	}
	return owned, balance, nil
}
