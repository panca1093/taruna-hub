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

// ErrDuplicateTitle is the per-tenant case-insensitive title index, so the service answers 422 title.
var ErrDuplicateTitle = errors.New("duplicate product title")

// ProductFilter is already validated by the service; the repository only turns it into SQL.
type ProductFilter struct {
	ProgramID *uuid.UUID
	Type      string
	Status    string
	Search    string
	Limit     int
	Offset    int
}

type ProductRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

const productColumns = `p.id, p.tenant_id, p.type, p.title, p.description, p.program_id,
	p.price::text, p.original_price::text, p.is_active, p.active_from, p.active_until, p.created_by, p.created_at, p.updated_at`

// A product is public only while it is on sale and every attached tryout is playable ($2 is now).
const publicVisible = `p.is_active
	AND (p.active_from IS NULL OR p.active_from <= $2)
	AND (p.active_until IS NULL OR p.active_until > $2)
	AND EXISTS (SELECT 1 FROM product_tryouts hp WHERE hp.product_id = p.id)
	AND NOT EXISTS (
		SELECT 1 FROM product_tryouts vp
		  JOIN tryouts vt ON vt.id = vp.tryout_id AND vt.tenant_id = vp.tenant_id
		 WHERE vp.product_id = p.id AND (vt.status <> 'published' OR vt.active_until <= $2))`

func (r *ProductRepository) List(ctx context.Context, tenantID uuid.UUID, filter ProductFilter) ([]model.Product, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"p.tenant_id = $1"}
	args := []any{tenantID}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.ProgramID != nil {
		add("p.program_id = $%d", *filter.ProgramID)
	}
	if filter.Type != "" {
		add("p.type = $%d", filter.Type)
	}
	switch filter.Status {
	case "active":
		where = append(where, "p.is_active")
	case "inactive":
		where = append(where, "NOT p.is_active")
	}
	if filter.Search != "" {
		add(`p.title ILIKE $%d ESCAPE '\'`, likePattern(filter.Search))
	}
	from := " FROM products p WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+productColumns+from+
		fmt.Sprintf(` ORDER BY p.created_at DESC, p.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	out := make([]model.Product, 0, filter.Limit)
	for rows.Next() {
		product, err := scanProductHead(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, product)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	if err := r.attachTryouts(ctx, r.db, tenantID, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *ProductRepository) Find(ctx context.Context, tenantID, id uuid.UUID) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	return r.find(ctx, r.db, tenantID, id, false)
}

func (r *ProductRepository) FindTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, false)
}

// The row lock serializes concurrent saves, so a package is always exactly one caller's set.
func (r *ProductRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	return r.find(ctx, tx, tenantID, id, true)
}

func (r *ProductRepository) Create(ctx context.Context, tenantID, createdBy uuid.UUID, product model.Product) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	var out model.Product
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		created, err := r.CreateTx(ctx, tx, tenantID, createdBy, product)
		out = created
		return err
	})
	return out, err
}

func (r *ProductRepository) CreateTx(ctx context.Context, tx pgx.Tx, tenantID, createdBy uuid.UUID, product model.Product) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO products (tenant_id, type, title, description, program_id, price,
		        original_price, is_active, active_from, active_until, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		tenantID, product.Type, product.Title, product.Description, product.ProgramID,
		product.Price, product.OriginalPrice, product.IsActive, product.ActiveFrom, product.ActiveUntil, createdBy).Scan(&id)
	if err != nil {
		return model.Product{}, mapProductError("create product", err)
	}
	if err := r.replaceTryoutsTx(ctx, tx, tenantID, id, product.Tryouts); err != nil {
		return model.Product{}, err
	}
	return r.find(ctx, tx, tenantID, id, false)
}

func (r *ProductRepository) UpdateHeadTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, product model.Product) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	tag, err := tx.Exec(ctx, `
		UPDATE products
		   SET type = $3, title = $4, description = $5, program_id = $6, price = $7,
		       original_price = $8, is_active = $9, active_from = $10, active_until = $11, updated_at = now()
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, product.Type, product.Title, product.Description, product.ProgramID,
		product.Price, product.OriginalPrice, product.IsActive, product.ActiveFrom, product.ActiveUntil)
	if err != nil {
		return mapProductError("update product", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProductRepository) ReplaceTryoutsTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, tryouts []model.ProductTryout) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	return r.replaceTryoutsTx(ctx, tx, tenantID, id, tryouts)
}

func (r *ProductRepository) DeleteTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM product_tryouts WHERE tenant_id = $1 AND product_id = $2`, tenantID, id); err != nil {
		return mapProductError("delete product tryouts", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM products WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return mapProductError("delete product", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TitleTaken ignores the product being edited so a save keeps its own title.
func (r *ProductRepository) TitleTaken(ctx context.Context, tenantID uuid.UUID, title string, except uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var taken bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM products WHERE tenant_id = $1 AND lower(title) = lower($2) AND id <> $3)`,
		tenantID, title, except).Scan(&taken); err != nil {
		return false, fmt.Errorf("check product title: %w", err)
	}
	return taken, nil
}

func (r *ProductRepository) ProgramExists(ctx context.Context, tenantID, programID uuid.UUID) (bool, error) {
	if tenantID == uuid.Nil {
		return false, ErrScopeRequired
	}
	var ok bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM programs WHERE tenant_id = $1 AND id = $2)`,
		tenantID, programID).Scan(&ok); err != nil {
		return false, fmt.Errorf("check product program: %w", err)
	}
	return ok, nil
}

// TryoutsForAttach reads only the tenant's own rows, so a foreign id is simply absent.
func (r *ProductRepository) TryoutsForAttach(ctx context.Context, q queryer, tenantID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]model.TryoutAttach, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := q.Query(ctx,
		`SELECT id, status, program_id FROM tryouts WHERE tenant_id = $1 AND id = ANY($2)`,
		tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("read product tryouts: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]model.TryoutAttach, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var attach model.TryoutAttach
		if err := rows.Scan(&id, &attach.Status, &attach.ProgramID); err != nil {
			return nil, fmt.Errorf("scan product tryout: %w", err)
		}
		out[id] = attach
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read product tryouts: %w", err)
	}
	return out, nil
}

// UnpublishedAttached is the activation re-check, read after the links are written.
func (r *ProductRepository) UnpublishedAttached(ctx context.Context, q queryer, tenantID, productID uuid.UUID) (int, error) {
	if tenantID == uuid.Nil {
		return 0, ErrScopeRequired
	}
	var count int
	if err := q.QueryRow(ctx, `
		SELECT count(*) FROM product_tryouts pt
		  JOIN tryouts t ON t.id = pt.tryout_id AND t.tenant_id = pt.tenant_id
		 WHERE pt.tenant_id = $1 AND pt.product_id = $2 AND t.status <> 'published'`,
		tenantID, productID).Scan(&count); err != nil {
		return 0, fmt.Errorf("check attached tryouts: %w", err)
	}
	return count, nil
}

func (r *ProductRepository) PublicList(ctx context.Context, tenantID uuid.UUID, now time.Time, filter ProductFilter) ([]model.PublicProduct, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	where := []string{"p.tenant_id = $1", publicVisible}
	args := []any{tenantID, now}
	add := func(condition string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}
	if filter.ProgramID != nil {
		add("p.program_id = $%d", *filter.ProgramID)
	}
	if filter.Type != "" {
		add("p.type = $%d", filter.Type)
	}
	if filter.Search != "" {
		add(`p.title ILIKE $%d ESCAPE '\'`, likePattern(filter.Search))
	}
	from := ` FROM products p
		JOIN programs pr ON pr.id = p.program_id AND pr.tenant_id = p.tenant_id
		WHERE ` + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog: %w", err)
	}

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+publicColumns+from+
		fmt.Sprintf(` ORDER BY p.created_at DESC, p.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list catalog: %w", err)
	}
	defer rows.Close()

	out := make([]model.PublicProduct, 0, filter.Limit)
	for rows.Next() {
		product, err := scanPublicProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, product)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list catalog: %w", err)
	}
	if err := r.attachPublicTryouts(ctx, tenantID, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *ProductRepository) PublicFind(ctx context.Context, tenantID uuid.UUID, now time.Time, id uuid.UUID) (model.PublicProduct, error) {
	if tenantID == uuid.Nil {
		return model.PublicProduct{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx, `SELECT `+publicColumns+`
		FROM products p
		JOIN programs pr ON pr.id = p.program_id AND pr.tenant_id = p.tenant_id
		WHERE p.tenant_id = $1 AND `+publicVisible+` AND p.id = $3`, tenantID, now, id)
	product, err := scanPublicProduct(row)
	if err != nil {
		return model.PublicProduct{}, err
	}
	products := []model.PublicProduct{product}
	if err := r.attachPublicTryouts(ctx, tenantID, products); err != nil {
		return model.PublicProduct{}, err
	}
	return products[0], nil
}

// SellableTx reads a product only while the public catalog would list it, so checkout and the catalog
// can never disagree about what may be bought.
func (r *ProductRepository) SellableTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, now time.Time) (model.Product, error) {
	if tenantID == uuid.Nil {
		return model.Product{}, ErrScopeRequired
	}
	out, err := scanProductHead(tx.QueryRow(ctx,
		`SELECT `+productColumns+` FROM products p WHERE p.tenant_id = $1 AND `+publicVisible+` AND p.id = $3`,
		tenantID, now, id))
	if err != nil {
		return model.Product{}, err
	}
	products := []model.Product{out}
	if err := r.attachTryouts(ctx, tx, tenantID, products); err != nil {
		return model.Product{}, err
	}
	return products[0], nil
}

func (r *ProductRepository) PublicSummary(ctx context.Context, tenantID uuid.UUID, now time.Time) (model.CatalogSummary, error) {
	if tenantID == uuid.Nil {
		return model.CatalogSummary{}, ErrScopeRequired
	}
	var out model.CatalogSummary
	err := r.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM programs WHERE tenant_id = $1),
		       (SELECT count(*) FROM products p WHERE p.tenant_id = $1 AND `+publicVisible+`),
		       (SELECT count(*) FROM tryouts t
		         WHERE t.tenant_id = $1 AND t.status = 'published' AND t.active_until > $2)`,
		tenantID, now).Scan(&out.Programs, &out.Products, &out.Tryouts)
	if err != nil {
		return model.CatalogSummary{}, fmt.Errorf("read catalog summary: %w", err)
	}
	return out, nil
}

const publicColumns = `p.id, p.type, p.title, p.description, p.program_id, pr.code, pr.name,
	p.price::text, p.original_price::text, p.active_until`

func (r *ProductRepository) find(ctx context.Context, q queryer, tenantID, id uuid.UUID, forUpdate bool) (model.Product, error) {
	sql := `SELECT ` + productColumns + ` FROM products p WHERE p.tenant_id = $1 AND p.id = $2`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	out, err := scanProductHead(q.QueryRow(ctx, sql, tenantID, id))
	if err != nil {
		return model.Product{}, err
	}
	if forUpdate {
		return out, nil
	}
	products := []model.Product{out}
	if err := r.attachTryouts(ctx, q, tenantID, products); err != nil {
		return model.Product{}, err
	}
	return products[0], nil
}

func (r *ProductRepository) attachTryouts(ctx context.Context, q queryer, tenantID uuid.UUID, products []model.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(products))
	index := make(map[uuid.UUID]int, len(products))
	for i, product := range products {
		ids = append(ids, product.ID)
		index[product.ID] = i
	}
	rows, err := q.Query(ctx, `
		SELECT pt.product_id, t.id, t.title, t.status,
		       (SELECT count(*) FROM tryout_subtests s WHERE s.tenant_id = pt.tenant_id AND s.tryout_id = t.id),
		       (SELECT count(*) FROM tryout_questions tq WHERE tq.tenant_id = pt.tenant_id AND tq.tryout_id = t.id)
		  FROM product_tryouts pt
		  JOIN tryouts t ON t.id = pt.tryout_id AND t.tenant_id = pt.tenant_id
		 WHERE pt.tenant_id = $1 AND pt.product_id = ANY($2)
		 ORDER BY pt.product_id, pt.position`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("read product package: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var productID uuid.UUID
		var item model.ProductTryout
		if err := rows.Scan(&productID, &item.ID, &item.Title, &item.Status,
			&item.SubtestCount, &item.QuestionCount); err != nil {
			return fmt.Errorf("scan product package: %w", err)
		}
		if at, ok := index[productID]; ok {
			products[at].Tryouts = append(products[at].Tryouts, item)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read product package: %w", err)
	}
	return nil
}

func (r *ProductRepository) attachPublicTryouts(ctx context.Context, tenantID uuid.UUID, products []model.PublicProduct) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(products))
	index := make(map[uuid.UUID]int, len(products))
	for i, product := range products {
		ids = append(ids, product.ID)
		index[product.ID] = i
	}
	rows, err := r.db.Query(ctx, `
		SELECT pt.product_id, t.id, t.title,
		       (SELECT count(*) FROM tryout_subtests s WHERE s.tenant_id = pt.tenant_id AND s.tryout_id = t.id),
		       (SELECT count(*) FROM tryout_questions tq WHERE tq.tenant_id = pt.tenant_id AND tq.tryout_id = t.id),
		       (SELECT COALESCE(sum(s.duration_seconds), 0) FROM tryout_subtests s
		         WHERE s.tenant_id = pt.tenant_id AND s.tryout_id = t.id)
		  FROM product_tryouts pt
		  JOIN tryouts t ON t.id = pt.tryout_id AND t.tenant_id = pt.tenant_id
		 WHERE pt.tenant_id = $1 AND pt.product_id = ANY($2)
		 ORDER BY pt.product_id, pt.position`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("read catalog package: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var productID uuid.UUID
		var item model.PublicProductTryout
		if err := rows.Scan(&productID, &item.ID, &item.Title, &item.SubtestCount, &item.QuestionCount,
			&item.DurationSecondsTotal); err != nil {
			return fmt.Errorf("scan catalog package: %w", err)
		}
		if at, ok := index[productID]; ok {
			products[at].Tryouts = append(products[at].Tryouts, item)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read catalog package: %w", err)
	}
	return nil
}

func (r *ProductRepository) replaceTryoutsTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, tryouts []model.ProductTryout) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM product_tryouts WHERE tenant_id = $1 AND product_id = $2`, tenantID, id); err != nil {
		return mapProductError("replace product tryouts", err)
	}
	for i, tryout := range tryouts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_tryouts (product_id, tryout_id, tenant_id, position)
			VALUES ($1, $2, $3, $4)`, id, tryout.ID, tenantID, i+1); err != nil {
			return mapProductError("insert product tryout", err)
		}
	}
	return nil
}

func scanProductHead(row pgx.Row) (model.Product, error) {
	var out model.Product
	var original *string
	var activeFrom, activeUntil *time.Time
	err := row.Scan(&out.ID, &out.TenantID, &out.Type, &out.Title, &out.Description, &out.ProgramID,
		&out.Price, &original, &out.IsActive, &activeFrom, &activeUntil, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Product{}, ErrNotFound
		}
		return model.Product{}, fmt.Errorf("scan product: %w", err)
	}
	out.OriginalPrice, out.ActiveFrom, out.ActiveUntil = original, activeFrom, activeUntil
	return out, nil
}

func scanPublicProduct(row pgx.Row) (model.PublicProduct, error) {
	var out model.PublicProduct
	var original *string
	var activeUntil *time.Time
	err := row.Scan(&out.ID, &out.Type, &out.Title, &out.Description, &out.ProgramID,
		&out.ProgramCode, &out.ProgramName, &out.Price, &original, &activeUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.PublicProduct{}, ErrNotFound
		}
		return model.PublicProduct{}, fmt.Errorf("scan catalog product: %w", err)
	}
	out.OriginalPrice, out.ActiveUntil = original, activeUntil
	out.Tryouts = []model.PublicProductTryout{}
	return out, nil
}

func mapProductError(what string, err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "products_tenant_title_key":
			return ErrDuplicateTitle
		case pgErr.Code == pgForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrReferenced, what)
		}
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

func (r *ProductRepository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
