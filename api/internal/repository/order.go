package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// ErrProofTaken is the unique index that lets one upload back one order.
var ErrProofTaken = errors.New("payment proof already used")

// ErrPendingExists is the partial unique index that allows one pending order per student and product.
var ErrPendingExists = errors.New("pending order already exists for this product")

// OrderFilter is already validated by the service; the repository only turns it into SQL.
type OrderFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

const orderColumns = `o.id, o.number, o.tenant_id, o.user_id, u.name, o.product_id, o.product_type,
	o.product_title, o.price::text, o.payment_method, o.status, o.idempotency_key, o.proof_asset_id,
	o.cancel_reason, o.reviewed_by, o.confirmed_at, o.cancelled_at, o.created_at, o.updated_at,
	o.kecermatan_test_id, o.included_attempts`

const orderFrom = ` FROM orders o JOIN users u ON u.id = o.user_id AND u.tenant_id = o.tenant_id`

// LockStudentTx serializes one student's checkouts and wallet movements on the same row.
func (r *OrderRepository) LockStudentTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID) (model.Contact, error) {
	if tenantID == uuid.Nil {
		return model.Contact{}, ErrScopeRequired
	}
	var c model.Contact
	err := tx.QueryRow(ctx,
		`SELECT name, email, phone FROM users WHERE tenant_id = $1 AND id = $2 AND role = 'student' FOR UPDATE`,
		tenantID, userID).Scan(&c.Name, &c.Email, &c.Phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Contact{}, ErrNotFound
	}
	if err != nil {
		return model.Contact{}, fmt.Errorf("lock student: %w", err)
	}
	return c, nil
}

func (r *OrderRepository) FindByKeyTx(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, key string) (model.Order, error) {
	if tenantID == uuid.Nil {
		return model.Order{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `o.tenant_id = $1 AND o.user_id = $2 AND o.idempotency_key = $3`, false, tenantID, userID, key)
}

func (r *OrderRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Order, error) {
	if tenantID == uuid.Nil {
		return model.Order{}, ErrScopeRequired
	}
	return r.find(ctx, r.db, `o.tenant_id = $1 AND o.id = $2`, false, tenantID, id)
}

// LockTx takes the order row, so concurrent reviews of one order queue behind each other.
func (r *OrderRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Order, error) {
	if tenantID == uuid.Nil {
		return model.Order{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `o.tenant_id = $1 AND o.id = $2`, true, tenantID, id)
}

func (r *OrderRepository) FindTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Order, error) {
	if tenantID == uuid.Nil {
		return model.Order{}, ErrScopeRequired
	}
	return r.find(ctx, tx, `o.tenant_id = $1 AND o.id = $2`, false, tenantID, id)
}

func (r *OrderRepository) InsertTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, order model.Order) (model.Order, error) {
	if tenantID == uuid.Nil {
		return model.Order{}, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO orders (tenant_id, user_id, product_id, product_type, product_title, price,
		        payment_method, status, idempotency_key, proof_asset_id, kecermatan_test_id, included_attempts)
		SELECT $1::uuid, $2::uuid, $3::uuid, $4::text, $5::text, $6::numeric, $7::text, 'pending', $8::text, $9::uuid,
		       $10::uuid, $11::int
		 WHERE $9::uuid IS NULL OR NOT EXISTS (SELECT 1 FROM topup_requests WHERE proof_asset_id = $9::uuid)
		RETURNING id`,
		tenantID, order.UserID, order.ProductID, order.ProductType, order.ProductTitle, order.Price,
		order.PaymentMethod, order.Key, order.ProofAssetID, order.KecermatanTestID, order.IncludedAttempts).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Order{}, ErrProofTaken
	}
	if err != nil {
		return model.Order{}, mapOrderError("insert order", err)
	}
	for i, item := range order.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, tenant_id, tryout_id, tryout_title, position)
			VALUES ($1, $2, $3, $4, $5)`, id, tenantID, item.TryoutID, item.Title, i+1); err != nil {
			return model.Order{}, mapOrderError("insert order item", err)
		}
	}
	return r.FindTx(ctx, tx, tenantID, id)
}

// ConfirmTx and CancelTx only move a row that is still pending; the caller already holds its lock.
func (r *OrderRepository) ConfirmTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, reviewer *uuid.UUID) error {
	return r.transition(ctx, tx, tenantID, id, `
		UPDATE orders SET status = 'confirmed', confirmed_at = now(), reviewed_by = $3, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, reviewer)
}

func (r *OrderRepository) CancelTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, reviewer uuid.UUID, reason string) error {
	return r.transition(ctx, tx, tenantID, id, `
		UPDATE orders SET status = 'cancelled', cancelled_at = now(), reviewed_by = $3, cancel_reason = $4,
		       updated_at = now()
		 WHERE tenant_id = $1 AND id = $2 AND status = 'pending'`, &reviewer, reason)
}

func (r *OrderRepository) transition(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, sql string, args ...any) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, sql, append([]any{tenantID, id}, args...)...)
	if err != nil {
		return mapOrderError("move order", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *OrderRepository) ListForUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.Order, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	return r.list(ctx, []string{"o.tenant_id = $1", "o.user_id = $2"}, []any{tenantID, userID}, limit, offset)
}

func (r *OrderRepository) List(ctx context.Context, tenantID uuid.UUID, filter OrderFilter) ([]model.Order, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"o.tenant_id = $1"}
	args := []any{tenantID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("o.status = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, likePattern(filter.Search))
		where = append(where, fmt.Sprintf(
			`('ORD-' || lpad(o.number::text, 6, '0') ILIKE $%[1]d ESCAPE '\' OR u.name ILIKE $%[1]d ESCAPE '\'
			  OR o.product_title ILIKE $%[1]d ESCAPE '\')`, len(args)))
	}
	return r.list(ctx, where, args, filter.Limit, filter.Offset)
}

func (r *OrderRepository) list(ctx context.Context, where []string, args []any, limit, offset int) ([]model.Order, int, error) {
	from := orderFrom + ` WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, `SELECT `+orderColumns+from+
		fmt.Sprintf(` ORDER BY o.created_at DESC, o.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	out := make([]model.Order, 0, limit)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, order)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	if err := r.attachItems(ctx, r.db, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *OrderRepository) find(ctx context.Context, q queryer, where string, forUpdate bool, args ...any) (model.Order, error) {
	sql := `SELECT ` + orderColumns + orderFrom + ` WHERE ` + where
	if forUpdate {
		// Only the order row is locked; the student name comes from a plain join.
		sql += ` FOR UPDATE OF o`
	}
	order, err := scanOrder(q.QueryRow(ctx, sql, args...))
	if err != nil {
		return model.Order{}, err
	}
	orders := []model.Order{order}
	if err := r.attachItems(ctx, q, orders); err != nil {
		return model.Order{}, err
	}
	return orders[0], nil
}

func (r *OrderRepository) attachItems(ctx context.Context, q queryer, orders []model.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(orders))
	index := make(map[uuid.UUID]int, len(orders))
	for i, order := range orders {
		ids = append(ids, order.ID)
		index[order.ID] = i
	}
	rows, err := q.Query(ctx, `
		SELECT order_id, tryout_id, tryout_title FROM order_items
		 WHERE tenant_id = $1 AND order_id = ANY($2) ORDER BY order_id, position`, orders[0].TenantID, ids)
	if err != nil {
		return fmt.Errorf("read order items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderID uuid.UUID
		var item model.OrderItem
		if err := rows.Scan(&orderID, &item.TryoutID, &item.Title); err != nil {
			return fmt.Errorf("scan order item: %w", err)
		}
		if at, ok := index[orderID]; ok {
			orders[at].Items = append(orders[at].Items, item)
		}
	}
	return rows.Err()
}

// HeldTryouts returns the given tryouts the student already holds with access still open (D05b).
func (r *OrderRepository) HeldTryouts(ctx context.Context, q queryer, tenantID, userID uuid.UUID, tryoutIDs []uuid.UUID, now time.Time) (map[uuid.UUID]bool, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx, `
		SELECT e.tryout_id FROM enrollments e
		  JOIN tryouts t ON t.id = e.tryout_id AND t.tenant_id = e.tenant_id
		 WHERE e.tenant_id = $1 AND e.user_id = $2 AND e.tryout_id = ANY($3) AND t.active_until > $4`,
		tenantID, userID, tryoutIDs, now)
	if err != nil {
		return nil, fmt.Errorf("read held tryouts: %w", err)
	}
	defer rows.Close()
	held := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan held tryout: %w", err)
		}
		held[id] = true
	}
	return held, rows.Err()
}

// InsertEnrollmentTx creates the user's single row for a tryout; created is false when one already existed.
func (r *OrderRepository) InsertEnrollmentTx(ctx context.Context, tx pgx.Tx, tenantID, userID, tryoutID uuid.UUID) (uuid.UUID, bool, error) {
	if tenantID == uuid.Nil {
		return uuid.Nil, false, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO enrollments (tenant_id, user_id, tryout_id) VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, user_id, tryout_id) DO NOTHING
		RETURNING id`, tenantID, userID, tryoutID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, mapOrderError("insert enrollment", err)
	}
	return id, true, nil
}

// AddKecermatanCreditTx opens (or finds) the user's single enrollment for a test and adds the credits of one
// grant source. The enrollment row is locked first, so it serializes with an attempt's completion on the same row;
// the grant's unique source key makes a retried source add nothing twice. It reports whether credits were added.
func (r *OrderRepository) AddKecermatanCreditTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, g model.EnrollmentGrant, userID, testID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	if g.Attempts == nil || *g.Attempts < 1 {
		return false, fmt.Errorf("kecermatan grant needs a positive credit")
	}
	var enrollmentID uuid.UUID
	created := true
	err := tx.QueryRow(ctx, `
		INSERT INTO enrollments (tenant_id, user_id, kecermatan_test_id, attempts_purchased)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, user_id, kecermatan_test_id) DO NOTHING
		RETURNING id`, tenantID, userID, testID, *g.Attempts).Scan(&enrollmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = tx.QueryRow(ctx, `
			SELECT id FROM enrollments WHERE tenant_id = $1 AND user_id = $2 AND kecermatan_test_id = $3 FOR UPDATE`,
			tenantID, userID, testID).Scan(&enrollmentID)
	}
	if err != nil {
		return false, mapOrderError("open kecermatan enrollment", err)
	}
	g.EnrollmentID = enrollmentID
	granted, err := r.InsertGrantTx(ctx, tx, tenantID, g)
	if err != nil || !granted {
		return false, err
	}
	if !created {
		if _, err := tx.Exec(ctx, `UPDATE enrollments SET attempts_purchased = attempts_purchased + $3
			 WHERE tenant_id = $1 AND id = $2`, tenantID, enrollmentID, *g.Attempts); err != nil {
			return false, mapOrderError("add kecermatan credit", err)
		}
	}
	return true, nil
}

// InsertGrantTx reports false when this source already granted this enrollment.
func (r *OrderRepository) InsertGrantTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, g model.EnrollmentGrant) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO enrollment_grants (tenant_id, enrollment_id, order_id, source_type, source_id,
		        actor_kind, actor_id, reason, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, source_type, source_id, enrollment_id) DO NOTHING`,
		tenantID, g.EnrollmentID, g.OrderID, g.SourceType, g.SourceID, g.ActorKind, g.ActorID, g.Reason, g.Attempts)
	if err != nil {
		return false, mapOrderError("insert enrollment grant", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *OrderRepository) Enrollments(ctx context.Context, tenantID, userID uuid.UUID) ([]model.Enrollment, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.tenant_id, e.user_id, e.tryout_id, t.title, pr.code, pr.name,
		       (SELECT count(*) FROM tryout_subtests s WHERE s.tenant_id = e.tenant_id AND s.tryout_id = t.id),
		       t.active_from, t.active_until, e.attempts_purchased, e.attempts_used, e.created_at
		  FROM enrollments e
		  JOIN tryouts t ON t.id = e.tryout_id AND t.tenant_id = e.tenant_id
		  JOIN programs pr ON pr.id = t.program_id AND pr.tenant_id = t.tenant_id
		 WHERE e.tenant_id = $1 AND e.user_id = $2
		 ORDER BY e.created_at DESC, e.id DESC`, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("list enrollments: %w", err)
	}
	defer rows.Close()
	out := []model.Enrollment{}
	for rows.Next() {
		var e model.Enrollment
		if err := rows.Scan(&e.ID, &e.TenantID, &e.UserID, &e.TryoutID, &e.TryoutTitle, &e.ProgramCode,
			&e.ProgramName, &e.SubtestCount, &e.TryoutActiveFrom, &e.TryoutActiveUntil,
			&e.AttemptsPurchased, &e.AttemptsUsed, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan enrollment: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanOrder(row pgx.Row) (model.Order, error) {
	var o model.Order
	err := row.Scan(&o.ID, &o.Number, &o.TenantID, &o.UserID, &o.StudentName, &o.ProductID, &o.ProductType,
		&o.ProductTitle, &o.Price, &o.PaymentMethod, &o.Status, &o.Key, &o.ProofAssetID,
		&o.CancelReason, &o.ReviewedBy, &o.ConfirmedAt, &o.CancelledAt, &o.CreatedAt, &o.UpdatedAt,
		&o.KecermatanTestID, &o.IncludedAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Order{}, ErrNotFound
	}
	if err != nil {
		return model.Order{}, fmt.Errorf("scan order: %w", err)
	}
	return o, nil
}

func mapOrderError(what string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		switch pgErr.ConstraintName {
		case "orders_proof_key":
			return ErrProofTaken
		case "orders_pending_product_key":
			return ErrPendingExists
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}
