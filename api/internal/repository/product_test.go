package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

const pgCheckViolation = "23514"

// productFixture hangs off the tryout fixture so products have real programs and tryouts to point at.
type productFixture struct {
	tryoutFixture
	productRepo *ProductRepository
}

func newProductFixture(t *testing.T) productFixture {
	t.Helper()
	base := newTryoutFixture(t)
	f := productFixture{tryoutFixture: base, productRepo: NewProductRepository(base.pool)}
	// Registered after the tryout cleanup, so links and products go before the tryouts they reference.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{f.tenant.ID, f.other.ID}
		for _, sql := range []string{
			`DELETE FROM product_tryouts WHERE tenant_id = ANY($1)`,
			`DELETE FROM products WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup products: %v", err)
			}
		}
	})
	return f
}

func (f productFixture) createTryout(t *testing.T, tenantID, authorID, programID uuid.UUID, status string, questions ...model.Question) model.Tryout {
	t.Helper()
	in := model.Tryout{
		ProgramID: programID, Title: "Tryout " + uuid.NewString()[:8], Description: "Latihan",
		ScoringMode: model.ScoringWeighted, ActiveUntil: time.Now().Add(48 * time.Hour),
		Subtests: []model.Subtest{subtestOf("Subtes 1", 1800, questions...)},
	}
	created, err := f.repo.Create(context.Background(), tenantID, authorID, in)
	if err != nil {
		t.Fatalf("create tryout: %v", err)
	}
	if status != "" {
		if _, err := f.pool.Exec(context.Background(),
			`UPDATE tryouts SET status = $3 WHERE tenant_id = $1 AND id = $2`,
			tenantID, created.ID, status); err != nil {
			t.Fatalf("set tryout status: %v", err)
		}
		created.Status = status
	}
	return created
}

func (f productFixture) product(programID uuid.UUID, tryouts ...model.Tryout) model.Product {
	out := model.Product{
		Type: model.ProductIndividual, Title: "Paket " + uuid.NewString()[:8], Description: "Paket latihan",
		ProgramID: programID, Price: "150000.00", IsActive: true,
	}
	if len(tryouts) > 1 {
		out.Type = model.ProductBundle
	}
	for _, tryout := range tryouts {
		out.Tryouts = append(out.Tryouts, model.ProductTryout{ID: tryout.ID})
	}
	return out
}

func (f productFixture) create(t *testing.T, product model.Product) model.Product {
	t.Helper()
	created, err := f.productRepo.Create(context.Background(), f.tenant.ID, f.author.ID, product)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	return created
}

func TestProductsRequireScope(t *testing.T) {
	pool := testPool(t)
	repo := NewProductRepository(pool)
	ctx := context.Background()

	if _, _, err := repo.List(ctx, uuid.Nil, ProductFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Find(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.Create(ctx, uuid.Nil, uuid.New(), model.Product{Price: "1.00"}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Create(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.TitleTaken(ctx, uuid.Nil, "x", uuid.Nil); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("TitleTaken(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.ProgramExists(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ProgramExists(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.TryoutsForAttach(ctx, pool, uuid.Nil, []uuid.UUID{uuid.New()}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("TryoutsForAttach(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, _, err := repo.PublicList(ctx, uuid.Nil, time.Now(), ProductFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("PublicList(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.PublicFind(ctx, uuid.Nil, time.Now(), uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("PublicFind(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := repo.PublicSummary(ctx, uuid.Nil, time.Now()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("PublicSummary(Nil): err = %v, want ErrScopeRequired", err)
	}

	inTx := func(fn func(pgx.Tx) error) error { return inCurriculumTx(t, pool, fn) }
	for name, call := range map[string]func(pgx.Tx) error{
		"FindTx":           func(tx pgx.Tx) error { _, err := repo.FindTx(ctx, tx, uuid.Nil, uuid.New()); return err },
		"LockTx":           func(tx pgx.Tx) error { _, err := repo.LockTx(ctx, tx, uuid.Nil, uuid.New()); return err },
		"UpdateHeadTx":     func(tx pgx.Tx) error { return repo.UpdateHeadTx(ctx, tx, uuid.Nil, uuid.New(), model.Product{}) },
		"ReplaceTryoutsTx": func(tx pgx.Tx) error { return repo.ReplaceTryoutsTx(ctx, tx, uuid.Nil, uuid.New(), nil) },
		"DeleteTx":         func(tx pgx.Tx) error { return repo.DeleteTx(ctx, tx, uuid.Nil, uuid.New()) },
	} {
		if err := inTx(call); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("%s(Nil): err = %v, want ErrScopeRequired", name, err)
		}
	}
	if err := inTx(func(tx pgx.Tx) error {
		_, err := repo.UnpublishedAttached(ctx, tx, uuid.Nil, uuid.New())
		return err
	}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("UnpublishedAttached(Nil): err = %v, want ErrScopeRequired", err)
	}
}

func TestProductCompositeFKsRejectCrossTenantRefs(t *testing.T) {
	f := newProductFixture(t)
	ctx := context.Background()
	mine := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.first)
	spare := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.second)
	created := f.create(t, f.product(f.program.ID, mine))

	cases := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "program of another tenant",
			sql: `INSERT INTO products (tenant_id, type, title, program_id, price, created_by)
			      VALUES ($1, 'individual', 'Silang', $2, 10.00, $3)`,
			args: []any{f.tenant.ID, f.otherProgram.ID, f.author.ID},
		},
		{
			name: "tryout of another tenant",
			sql: `INSERT INTO product_tryouts (product_id, tryout_id, tenant_id, position)
			      VALUES ($1, $2, $3, 2)`,
			args: []any{created.ID, uuid.New(), f.tenant.ID},
		},
		{
			name: "link of another tenant",
			sql: `INSERT INTO product_tryouts (product_id, tryout_id, tenant_id, position)
			      VALUES ($1, $2, $3, 2)`,
			args: []any{created.ID, spare.ID, f.other.ID},
		},
	}
	for _, tc := range cases {
		_, err := f.pool.Exec(ctx, tc.sql, tc.args...)
		if !isForeignKeyViolation(err, pgForeignKeyViolation) {
			t.Errorf("%s: err = %v, want SQLSTATE %s", tc.name, err, pgForeignKeyViolation)
		}
	}

	var products, links int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM products WHERE tenant_id = $1`, f.tenant.ID).Scan(&products); err != nil {
		t.Fatalf("count products: %v", err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM product_tryouts WHERE product_id = $1`, created.ID).Scan(&links); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if products != 1 || links != 1 {
		t.Errorf("products = %d links = %d, want 1 and 1: every rejected insert must have rolled back", products, links)
	}
}

func TestProductPriceIsExactNumeric(t *testing.T) {
	f := newProductFixture(t)
	ctx := context.Background()
	mine := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.first)

	for _, price := range []string{"0.10", "150000.00", "9999999999.99", "1.05"} {
		in := f.product(f.program.ID, mine)
		in.Price = price
		created := f.create(t, in)
		reloaded, err := f.productRepo.Find(ctx, f.tenant.ID, created.ID)
		if err != nil {
			t.Fatalf("find %s: %v", price, err)
		}
		if reloaded.Price != price {
			t.Errorf("price %q came back as %q, want it unchanged", price, reloaded.Price)
		}
	}

	for _, price := range []any{"0", "-1.00", "0.00"} {
		_, err := f.pool.Exec(ctx, `INSERT INTO products (tenant_id, type, title, program_id, price, created_by)
			VALUES ($1, 'individual', $4, $2, $3, $5)`,
			f.tenant.ID, f.program.ID, price, "Harga "+uuid.NewString()[:8], f.author.ID)
		if !isForeignKeyViolation(err, pgCheckViolation) {
			t.Errorf("price %v: err = %v, want SQLSTATE %s", price, err, pgCheckViolation)
		}
	}
}

func TestTitleIsUniqueCaseInsensitivelyPerTenant(t *testing.T) {
	f := newProductFixture(t)
	ctx := context.Background()
	mine := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.first)
	theirs := f.createTryout(t, f.other.ID, f.otherBy.ID, f.otherProgram.ID, model.TryoutPublished, f.otherQ)

	in := f.product(f.program.ID, mine)
	in.Title = "Paket Juara UTBK"
	f.create(t, in)

	duplicate := f.product(f.program.ID, mine)
	duplicate.Title = "paket juara utbk"
	if _, err := f.productRepo.Create(ctx, f.tenant.ID, f.author.ID, duplicate); !errors.Is(err, ErrDuplicateTitle) {
		t.Fatalf("same title in another case: err = %v, want ErrDuplicateTitle", err)
	}

	other := f.product(f.otherProgram.ID, theirs)
	other.Title = "Paket Juara UTBK"
	if _, err := f.productRepo.Create(ctx, f.other.ID, f.otherBy.ID, other); err != nil {
		t.Errorf("same title in another tenant: err = %v, want it allowed", err)
	}
}

func TestDeleteProductDeletesTryoutLinksAtomically(t *testing.T) {
	f := newProductFixture(t)
	ctx := context.Background()
	first := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.first)
	second := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.second)
	created := f.create(t, f.product(f.program.ID, first, second))

	if _, err := f.pool.Exec(ctx, `DELETE FROM products WHERE tenant_id = $1 AND id = $2`, f.tenant.ID, created.ID); !isForeignKeyViolation(err, pgForeignKeyViolation) {
		t.Fatalf("delete a product that still has links: err = %v, want SQLSTATE %s", err, pgForeignKeyViolation)
	}

	if err := inCurriculumTx(t, f.pool, func(tx pgx.Tx) error {
		if _, err := f.productRepo.LockTx(ctx, tx, f.tenant.ID, created.ID); err != nil {
			return err
		}
		return f.productRepo.DeleteTx(ctx, tx, f.tenant.ID, created.ID)
	}); err != nil {
		t.Fatalf("delete product: %v", err)
	}

	for table, column := range map[string]string{"products": "id", "product_tryouts": "product_id"} {
		var count int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+column+` = $1`, created.ID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s still holds %d rows of the deleted product", table, count)
		}
	}
	var tryouts int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM tryouts WHERE tenant_id = $1 AND id = ANY($2)`,
		f.tenant.ID, []uuid.UUID{first.ID, second.ID}).Scan(&tryouts); err != nil {
		t.Fatalf("count tryouts: %v", err)
	}
	if tryouts != 2 {
		t.Errorf("tryouts = %d after deleting the product, want both kept", tryouts)
	}
	if _, err := f.productRepo.Find(ctx, f.tenant.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("find after delete: err = %v, want ErrNotFound", err)
	}
}

func TestPublicQueriesAreTenantScoped(t *testing.T) {
	f := newProductFixture(t)
	ctx := context.Background()
	now := time.Now()
	mine := f.createTryout(t, f.tenant.ID, f.author.ID, f.program.ID, model.TryoutPublished, f.first)
	theirs := f.createTryout(t, f.other.ID, f.otherBy.ID, f.otherProgram.ID, model.TryoutPublished, f.otherQ)
	productA := f.create(t, f.product(f.program.ID, mine))
	productB, err := f.productRepo.Create(ctx, f.other.ID, f.otherBy.ID, f.product(f.otherProgram.ID, theirs))
	if err != nil {
		t.Fatalf("create product for B: %v", err)
	}

	listA, totalA, err := f.productRepo.PublicList(ctx, f.tenant.ID, now, ProductFilter{Limit: 50})
	if err != nil {
		t.Fatalf("public list A: %v", err)
	}
	if totalA != 1 || len(listA) != 1 || listA[0].ID != productA.ID {
		t.Fatalf("tenant A public list = %+v (total %d), want only its own product", listA, totalA)
	}
	if listA[0].ProgramCode == "" || listA[0].ProgramName == "" {
		t.Errorf("public product %+v carries no program display data", listA[0])
	}

	listB, totalB, err := f.productRepo.PublicList(ctx, f.other.ID, now, ProductFilter{Limit: 50})
	if err != nil {
		t.Fatalf("public list B: %v", err)
	}
	if totalB != 1 || listB[0].ID != productB.ID {
		t.Errorf("tenant B public list = %+v (total %d), want only B's product", listB, totalB)
	}

	if _, err := f.productRepo.PublicFind(ctx, f.other.ID, now, productA.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B read A's product: err = %v, want ErrNotFound", err)
	}

	summaryA, err := f.productRepo.PublicSummary(ctx, f.tenant.ID, now)
	if err != nil {
		t.Fatalf("summary A: %v", err)
	}
	summaryB, err := f.productRepo.PublicSummary(ctx, f.other.ID, now)
	if err != nil {
		t.Fatalf("summary B: %v", err)
	}
	if summaryA.Products != 1 || summaryB.Products != 1 {
		t.Errorf("summary products = %d and %d, want 1 each", summaryA.Products, summaryB.Products)
	}
	if summaryA.Tryouts != 1 || summaryB.Tryouts != 1 {
		t.Errorf("summary tryouts = %d and %d, want 1 each", summaryA.Tryouts, summaryB.Tryouts)
	}
}
