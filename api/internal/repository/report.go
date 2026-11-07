package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

// Revenue aggregates one tenant's confirmed orders and approved top-ups in [from, to) in a single snapshot,
// so the totals and the day rows describe the same data. Days are Asia/Jakarta calendar days, newest first.
func (r *ReportRepository) Revenue(ctx context.Context, tenantID uuid.UUID, from, to time.Time, limit, offset int) (model.RevenueRecap, error) {
	if tenantID == uuid.Nil {
		return model.RevenueRecap{}, ErrScopeRequired
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return model.RevenueRecap{}, fmt.Errorf("begin revenue read: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var out model.RevenueRecap
	const money = `coalesce(sum(%s), 0)::numeric(16,2)::text`
	if err := tx.QueryRow(ctx, `
		SELECT count(*), `+fmt.Sprintf(money, "price")+`
		  FROM orders WHERE tenant_id = $1 AND status = 'confirmed' AND confirmed_at >= $2 AND confirmed_at < $3`,
		tenantID, from, to).Scan(&out.Orders.Count, &out.Orders.Total); err != nil {
		return model.RevenueRecap{}, fmt.Errorf("sum orders: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT count(*), `+fmt.Sprintf(money, "amount")+`
		  FROM topup_requests WHERE tenant_id = $1 AND status = 'approved' AND approved_at >= $2 AND approved_at < $3`,
		tenantID, from, to).Scan(&out.Topups.Count, &out.Topups.Total); err != nil {
		return model.RevenueRecap{}, fmt.Errorf("sum topups: %w", err)
	}
	for column, target := range map[string]*[]model.RevenueBucket{"payment_method": &out.ByMethod, "product_type": &out.ByProduct} {
		rows, err := tx.Query(ctx, `
			SELECT `+column+`, count(*), `+fmt.Sprintf(money, "price")+`
			  FROM orders WHERE tenant_id = $1 AND status = 'confirmed' AND confirmed_at >= $2 AND confirmed_at < $3
			 GROUP BY 1 ORDER BY 1`, tenantID, from, to)
		if err != nil {
			return model.RevenueRecap{}, fmt.Errorf("group orders by %s: %w", column, err)
		}
		buckets := []model.RevenueBucket{}
		for rows.Next() {
			var b model.RevenueBucket
			if err := rows.Scan(&b.Key, &b.Count, &b.Total); err != nil {
				rows.Close()
				return model.RevenueRecap{}, fmt.Errorf("scan %s bucket: %w", column, err)
			}
			buckets = append(buckets, b)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return model.RevenueRecap{}, fmt.Errorf("group orders by %s: %w", column, err)
		}
		*target = buckets
	}

	const days = `
		WITH o AS (
		    SELECT (confirmed_at AT TIME ZONE 'Asia/Jakarta')::date AS d, count(*) AS n, sum(price) AS s
		      FROM orders WHERE tenant_id = $1 AND status = 'confirmed' AND confirmed_at >= $2 AND confirmed_at < $3
		     GROUP BY 1),
		t AS (
		    SELECT (approved_at AT TIME ZONE 'Asia/Jakarta')::date AS d, count(*) AS n, sum(amount) AS s
		      FROM topup_requests WHERE tenant_id = $1 AND status = 'approved' AND approved_at >= $2 AND approved_at < $3
		     GROUP BY 1),
		d AS (SELECT d FROM o UNION SELECT d FROM t)`
	if err := tx.QueryRow(ctx, days+` SELECT count(*) FROM d`, tenantID, from, to).Scan(&out.DaysTotal); err != nil {
		return model.RevenueRecap{}, fmt.Errorf("count revenue days: %w", err)
	}
	rows, err := tx.Query(ctx, days+`
		SELECT d.d, coalesce(o.n, 0), coalesce(o.s, 0)::numeric(16,2)::text, coalesce(t.n, 0), coalesce(t.s, 0)::numeric(16,2)::text
		  FROM d LEFT JOIN o ON o.d = d.d LEFT JOIN t ON t.d = d.d
		 ORDER BY d.d DESC LIMIT $4 OFFSET $5`, tenantID, from, to, limit, offset)
	if err != nil {
		return model.RevenueRecap{}, fmt.Errorf("list revenue days: %w", err)
	}
	defer rows.Close()
	out.Days = []model.RevenueDay{}
	for rows.Next() {
		var d model.RevenueDay
		if err := rows.Scan(&d.Date, &d.Orders, &d.OrdersTotal, &d.Topups, &d.TopupsTotal); err != nil {
			return model.RevenueRecap{}, fmt.Errorf("scan revenue day: %w", err)
		}
		out.Days = append(out.Days, d)
	}
	if err := rows.Err(); err != nil {
		return model.RevenueRecap{}, fmt.Errorf("list revenue days: %w", err)
	}
	return out, nil
}
