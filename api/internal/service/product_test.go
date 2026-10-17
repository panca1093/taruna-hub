package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type productHarness struct {
	svc   *ProductService
	pool  *pgxpool.Pool
	clock *testClock
	t     tryoutHarness
}

func newProductHarness(t *testing.T) productHarness {
	t.Helper()
	base := newTryoutHarness(t)
	h := productHarness{pool: base.pool, clock: base.clock, t: base}
	h.svc = NewProductService(base.pool, repository.NewProductRepository(base.pool), WithProductClock(base.clock.Now))
	// Registered last, so products and links go before the tryouts they reference.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{h.t.q.a.ID, h.t.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM product_tryouts WHERE tenant_id = ANY($1)`,
			`DELETE FROM products WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup product fixture: %v", err)
			}
		}
	})
	return h
}

func (h productHarness) scope(tenant model.Tenant) HostScope { return h.t.scope(tenant) }

func (h productHarness) admin(tenant model.Tenant) model.Principal {
	return h.t.principal(tenant, model.RoleAdmin)
}

func (h productHarness) tryout(t *testing.T, tenant model.Tenant, status string) model.Tryout {
	t.Helper()
	return h.tryoutIn(t, tenant, h.t.q.program(tenant).ID, status)
}

func (h productHarness) tryoutIn(t *testing.T, tenant model.Tenant, programID uuid.UUID, status string) model.Tryout {
	t.Helper()
	in := h.t.tryoutInput(tenant, subtestInput("Subtes 1", 1800, h.t.question(t, tenant)))
	in.ProgramID = programID
	created := h.t.create(t, tenant, in)
	if status == model.TryoutPublished {
		return h.publish(t, tenant, created.ID)
	}
	return created
}

func (h productHarness) publish(t *testing.T, tenant model.Tenant, id uuid.UUID) model.Tryout {
	t.Helper()
	published, err := h.t.publish(tenant, id)
	if err != nil {
		t.Fatalf("publish tryout: %v", err)
	}
	return published
}

// spareProgram is a second program in the same tenant, for the cross-program rule.
func (h productHarness) spareProgram(t *testing.T, tenant model.Tenant) model.Program {
	t.Helper()
	tree, err := h.t.curriculum.Create(context.Background(), h.scope(tenant),
		h.t.principal(tenant, model.RoleAdmin), ProgramInput{Code: "SP", Name: "Program Kedua"})
	if err != nil {
		t.Fatalf("create spare program: %v", err)
	}
	return tree.Program
}

func (h productHarness) input(tenant model.Tenant, tryouts ...model.Tryout) ProductInput {
	until := h.clock.Now().Add(30 * 24 * time.Hour)
	in := ProductInput{
		Type: model.ProductIndividual, Title: "Paket " + uuid.NewString()[:8], Description: "Paket latihan",
		ProgramID: h.t.q.program(tenant).ID, Price: "150000", IsActive: true, ActiveUntil: &until,
	}
	if len(tryouts) > 1 {
		in.Type = model.ProductBundle
	}
	for _, tryout := range tryouts {
		in.TryoutIDs = append(in.TryoutIDs, tryout.ID)
	}
	return in
}

func (h productHarness) create(t *testing.T, tenant model.Tenant, in ProductInput) model.Product {
	t.Helper()
	created, err := h.svc.Create(context.Background(), h.scope(tenant), h.admin(tenant), in)
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	return created
}

func (h productHarness) update(t *testing.T, tenant model.Tenant, id uuid.UUID, in ProductInput) model.Product {
	t.Helper()
	updated, err := h.svc.Update(context.Background(), h.scope(tenant), h.admin(tenant), id, in)
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	return updated
}

func (h productHarness) catalog(t *testing.T, tenant model.Tenant, filter ProductFilter) []model.PublicProduct {
	t.Helper()
	products, _, err := h.svc.CatalogProducts(context.Background(), h.scope(tenant), filter)
	if err != nil {
		t.Fatalf("catalog products: %v", err)
	}
	return products
}

func catalogHas(products []model.PublicProduct, id uuid.UUID) bool {
	for _, product := range products {
		if product.ID == id {
			return true
		}
	}
	return false
}

func wantProductField(t *testing.T, err error, path string) {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) {
		t.Fatalf("err = %v, want an InputError naming %s", err, path)
	}
	if input.Fields[path] == "" {
		t.Fatalf("no message for %s in %v", path, input.Fields)
	}
}

func TestProductValidation(t *testing.T) {
	h := newProductHarness(t)
	tenant, other := h.t.q.a, h.t.q.b
	published := h.tryout(t, tenant, model.TryoutPublished)
	second := h.tryout(t, tenant, model.TryoutPublished)
	draft := h.tryout(t, tenant, model.TryoutDraft)
	foreign := h.tryout(t, other, model.TryoutPublished)
	otherProgram := h.tryoutIn(t, tenant, h.spareProgram(t, tenant).ID, model.TryoutPublished)

	base := func() ProductInput { return h.input(tenant, published) }
	withTryouts := func(in *ProductInput, ids ...uuid.UUID) {
		in.Type = model.ProductBundle
		in.TryoutIDs = ids
	}

	cases := []struct {
		name   string
		mutate func(*ProductInput)
		path   string
	}{
		{"empty title", func(in *ProductInput) { in.Title = "   " }, "title"},
		{"title too long", func(in *ProductInput) { in.Title = strings.Repeat("a", 201) }, "title"},
		{"description too long", func(in *ProductInput) { in.Description = strings.Repeat("a", 4001) }, "description"},
		{"unknown type", func(in *ProductInput) { in.Type = "kecermatan" }, "type"},
		{"missing program", func(in *ProductInput) { in.ProgramID = uuid.New() }, "program_id"},
		{"price exponent", func(in *ProductInput) { in.Price = "1e3" }, "price"},
		{"price NaN", func(in *ProductInput) { in.Price = "NaN" }, "price"},
		{"price negative", func(in *ProductInput) { in.Price = "-1" }, "price"},
		{"price three decimals", func(in *ProductInput) { in.Price = "1.234" }, "price"},
		{"price leading space", func(in *ProductInput) { in.Price = " 5" }, "price"},
		{"price trailing space", func(in *ProductInput) { in.Price = "5 " }, "price"},
		{"price empty", func(in *ProductInput) { in.Price = "" }, "price"},
		{"price zero", func(in *ProductInput) { in.Price = "0" }, "price"},
		{"price zero cents", func(in *ProductInput) { in.Price = "0.00" }, "price"},
		{"price above the maximum", func(in *ProductInput) { in.Price = "10000000000.00" }, "price"},
		{"original below price", func(in *ProductInput) {
			in.Price = "1000"
			original := "999.99"
			in.OriginalPrice = &original
		}, "original_price"},
		{"individual without a tryout", func(in *ProductInput) { in.TryoutIDs = nil }, "tryout_ids"},
		{"individual with two tryouts", func(in *ProductInput) {
			withTryouts(in, published.ID, second.ID)
			in.Type = model.ProductIndividual
		}, "tryout_ids"},
		{"bundle with one tryout", func(in *ProductInput) { withTryouts(in, published.ID) }, "tryout_ids"},
		{"bundle with twenty-one tryouts", func(in *ProductInput) {
			ids := make([]uuid.UUID, 0, 21)
			for i := 0; i < 21; i++ {
				ids = append(ids, uuid.New())
			}
			withTryouts(in, ids...)
		}, "tryout_ids"},
		{"duplicate tryout", func(in *ProductInput) { withTryouts(in, published.ID, published.ID) }, "tryout_ids[1]"},
		{"draft tryout", func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{draft.ID} }, "tryout_ids[0]"},
		{"another tenant's tryout", func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{foreign.ID} }, "tryout_ids[0]"},
		{"missing tryout", func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{uuid.New()} }, "tryout_ids[0]"},
		{"tryout of another program", func(in *ProductInput) {
			in.TryoutIDs = []uuid.UUID{otherProgram.ID}
		}, "tryout_ids[0]"},
	}
	for _, tc := range cases {
		in := base()
		tc.mutate(&in)
		if _, err := h.svc.Create(context.Background(), h.scope(tenant), h.admin(tenant), in); err == nil {
			t.Errorf("%s: err = nil, want a 422 on %s", tc.name, tc.path)
		} else {
			wantProductField(t, err, tc.path)
		}
	}

	// A draft, a foreign and a missing tryout must be indistinguishable.
	reasons := map[string]string{}
	for name, mutate := range map[string]func(*ProductInput){
		"draft":   func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{draft.ID} },
		"foreign": func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{foreign.ID} },
		"missing": func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{uuid.New()} },
	} {
		in := base()
		mutate(&in)
		_, err := h.svc.Create(context.Background(), h.scope(tenant), h.admin(tenant), in)
		var input *InputError
		if !errors.As(err, &input) {
			t.Fatalf("%s tryout: err = %v, want an InputError", name, err)
		}
		reasons[name] = input.Fields["tryout_ids[0]"]
	}
	if reasons["draft"] == "" || reasons["draft"] != reasons["foreign"] || reasons["draft"] != reasons["missing"] {
		t.Errorf("tryout reasons = %v, want one identical message so existence never leaks", reasons)
	}

	duplicate := base()
	duplicate.Title = "Paket Unik " + uuid.NewString()[:8]
	h.create(t, tenant, duplicate)
	sameTitle := duplicate
	sameTitle.Title = strings.ToUpper(duplicate.Title)
	if _, err := h.svc.Create(context.Background(), h.scope(tenant), h.admin(tenant), sameTitle); err == nil {
		t.Error("duplicate title in another case: err = nil, want a 422 on title")
	} else {
		wantProductField(t, err, "title")
	}

	for raw, want := range map[string]string{"0.01": "0.01", "1": "1.00", "99.9": "99.90", "0.29": "0.29", "1.15": "1.15", "19.99": "19.99", "4.35": "4.35"} {
		in := base()
		in.Price = raw
		created := h.create(t, tenant, in)
		reloaded, err := h.svc.Get(context.Background(), h.scope(tenant), h.admin(tenant), created.ID)
		if err != nil {
			t.Fatalf("get after creating price %q: %v", raw, err)
		}
		if reloaded.Price != want {
			t.Errorf("price %q stored as %q, want %q", raw, reloaded.Price, want)
		}
	}
}

func TestActivationRequiresPublishedTryouts(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()

	tryout := h.tryout(t, tenant, model.TryoutPublished)
	in := h.input(tenant, tryout)
	in.IsActive = false
	created := h.create(t, tenant, in)

	if _, err := h.t.unpublish(tenant, tryout.ID); err != nil {
		t.Fatalf("unpublish tryout: %v", err)
	}

	activate := h.input(tenant, tryout)
	activate.Title = created.Title
	if _, err := h.svc.Update(ctx, h.scope(tenant), h.admin(tenant), created.ID, activate); err == nil {
		t.Fatal("activate with an unpublished tryout: err = nil, want a 422 on tryout_ids")
	} else {
		var input *InputError
		if !errors.As(err, &input) {
			t.Fatalf("err = %v, want an InputError", err)
		}
		if input.Fields["tryout_ids"] == "" && input.Fields["tryout_ids[0]"] == "" {
			t.Errorf("fields = %v, want a tryout_ids path", input.Fields)
		}
	}

	stored, err := h.svc.Get(ctx, h.scope(tenant), h.admin(tenant), created.ID)
	if err != nil {
		t.Fatalf("get after the refused activation: %v", err)
	}
	if stored.IsActive {
		t.Error("product became active even though activation was refused")
	}
}

func TestVisibilityRules(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()

	visible, _ := h.makeVisible(t, tenant)
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), visible.ID) {
		t.Fatal("an active product with a published tryout is missing from the catalog")
	}
	if _, err := h.svc.CatalogProduct(ctx, h.scope(tenant), visible.ID); err != nil {
		t.Fatalf("catalog detail of a visible product: %v", err)
	}

	inactive := h.input(tenant, h.tryout(t, tenant, model.TryoutPublished))
	inactive.IsActive = false
	hidden := h.create(t, tenant, inactive)
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), hidden.ID) {
		t.Error("an inactive product is listed publicly")
	}
	if _, err := h.svc.CatalogProduct(ctx, h.scope(tenant), hidden.ID); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("inactive detail: err = %v, want ErrProductNotFound", err)
	}
	restored := h.input(tenant, h.tryout(t, tenant, model.TryoutPublished))
	restored.Title = hidden.Title
	restored.TryoutIDs = nil
	// Reuse the stored package: only is_active changes.
	stored, err := h.svc.Get(ctx, h.scope(tenant), h.admin(tenant), hidden.ID)
	if err != nil {
		t.Fatalf("get inactive: %v", err)
	}
	restored.TryoutIDs = []uuid.UUID{stored.Tryouts[0].ID}
	h.update(t, tenant, hidden.ID, restored)
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), hidden.ID) {
		t.Error("activating a product did not make it visible again")
	}

	unpublished, tryout := h.makeVisible(t, tenant)
	if _, err := h.t.unpublish(tenant, tryout.ID); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), unpublished.ID) {
		t.Error("a product whose tryout was unpublished after saving is still listed")
	}
	if _, err := h.svc.CatalogProduct(ctx, h.scope(tenant), unpublished.ID); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("unpublished detail: err = %v, want ErrProductNotFound", err)
	}
	h.publish(t, tenant, tryout.ID)
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), unpublished.ID) {
		t.Error("republishing the tryout did not restore the product")
	}

	expired, ended := h.makeVisible(t, tenant)
	if _, err := h.pool.Exec(ctx, `UPDATE tryouts SET active_until = $3 WHERE tenant_id = $1 AND id = $2`,
		tenant.ID, ended.ID, h.clock.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("end the tryout window: %v", err)
	}
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), expired.ID) {
		t.Error("a product whose tryout window ended is still listed")
	}
	if _, err := h.pool.Exec(ctx, `UPDATE tryouts SET active_until = $3 WHERE tenant_id = $1 AND id = $2`,
		tenant.ID, ended.ID, h.clock.Now().Add(48*time.Hour)); err != nil {
		t.Fatalf("reopen the tryout window: %v", err)
	}
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), expired.ID) {
		t.Error("reopening the tryout window did not restore the product")
	}

	winding, _ := h.makeVisible(t, tenant)
	h.clock.Advance(31 * 24 * time.Hour)
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), winding.ID) {
		t.Error("a product past its own active_until is still listed")
	}
}

func (h productHarness) makeVisible(t *testing.T, tenant model.Tenant) (model.Product, model.Tryout) {
	t.Helper()
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	return h.create(t, tenant, h.input(tenant, tryout)), tryout
}

func TestPublicDiscountPercentIsComputedServerSide(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	tryout := h.tryout(t, tenant, model.TryoutPublished)

	cases := []struct {
		price    string
		original *string
		want     *int
	}{
		{"150000.00", nil, nil},
		{"75000.00", ptr("100000.00"), ptrInt(25)},
		{"66666.67", ptr("100000.00"), ptrInt(33)},
		{"87500.00", ptr("100000.00"), ptrInt(13)},
		{"50000.00", ptr("50000.00"), nil},
	}
	for _, tc := range cases {
		in := h.input(tenant, tryout)
		in.Price = tc.price
		in.OriginalPrice = tc.original
		created := h.create(t, tenant, in)

		detail, err := h.svc.CatalogProduct(ctx, h.scope(tenant), created.ID)
		if err != nil {
			t.Fatalf("catalog detail for %s: %v", tc.price, err)
		}
		if !samePercent(detail.DiscountPercent, tc.want) {
			t.Errorf("price %s original %v: discount = %v, want %v",
				tc.price, tc.original, formatPercent(detail.DiscountPercent), formatPercent(tc.want))
		}
		if detail.Price != tc.price {
			t.Errorf("price came back as %q, want %q", detail.Price, tc.price)
		}
	}
	list := h.catalog(t, tenant, ProductFilter{})
	for _, product := range list {
		if product.OriginalPrice != nil && product.DiscountPercent == nil && moneyCents(*product.OriginalPrice) > moneyCents(product.Price) {
			t.Errorf("listed product %s has a lower price than its original but no discount", product.ID)
		}
		if product.OriginalPrice == nil && product.DiscountPercent != nil {
			t.Errorf("product %s has a discount without an original price", product.ID)
		}
	}
}

func ptr(value string) *string { return &value }

func ptrInt(value int) *int { return &value }

func samePercent(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func formatPercent(value *int) string {
	if value == nil {
		return "nil"
	}
	return fmt.Sprintf("%d%%", *value)
}

func TestProductFilters(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	spare := h.spareProgram(t, tenant)

	first := h.tryout(t, tenant, model.TryoutPublished)
	second := h.tryout(t, tenant, model.TryoutPublished)
	extra := h.tryoutIn(t, tenant, spare.ID, model.TryoutPublished)

	individual := h.input(tenant, first)
	individual.Title = "Paket 50% Spesial"
	bundle := h.input(tenant, first, second)
	bundle.Title = "Paket_A Hemat"
	inactive := h.input(tenant, second)
	inactive.Title = "Paket Nonaktif"
	inactive.IsActive = false
	elsewhere := h.input(tenant, extra)
	elsewhere.ProgramID = spare.ID
	elsewhere.Title = "PaketXA Lain"

	for _, in := range []ProductInput{individual, bundle, inactive, elsewhere} {
		h.create(t, tenant, in)
	}

	list := func(filter ProductFilter) []model.Product {
		t.Helper()
		products, _, err := h.svc.List(ctx, h.scope(tenant), h.admin(tenant), filter)
		if err != nil {
			t.Fatalf("list products: %v", err)
		}
		return products
	}
	titles := func(products []model.Product) []string {
		out := make([]string, 0, len(products))
		for _, product := range products {
			out = append(out, product.Title)
		}
		return out
	}

	if got := titles(list(ProductFilter{ProgramID: &spare.ID})); len(got) != 1 || got[0] != "PaketXA Lain" {
		t.Errorf("program filter = %v, want only the spare program's product", got)
	}
	if got := titles(list(ProductFilter{Type: model.ProductBundle})); len(got) != 1 || got[0] != "Paket_A Hemat" {
		t.Errorf("type filter = %v, want only the bundle", got)
	}
	if got := titles(list(ProductFilter{Status: "inactive"})); len(got) != 1 || got[0] != "Paket Nonaktif" {
		t.Errorf("status=inactive = %v, want only the inactive product", got)
	}
	if got := len(list(ProductFilter{Status: "active"})); got != 3 {
		t.Errorf("status=active returned %d products, want 3", got)
	}
	if got := titles(list(ProductFilter{Search: "50%"})); len(got) != 1 || got[0] != "Paket 50% Spesial" {
		t.Errorf("q=50%% = %v, want %%, not a wildcard, so only the literal title", got)
	}
	if got := titles(list(ProductFilter{Search: "Paket_A"})); len(got) != 1 || got[0] != "Paket_A Hemat" {
		t.Errorf("q=Paket_A = %v, want _ to be literal", got)
	}
	if got := titles(list(ProductFilter{Search: "paket"})); len(got) != 4 {
		t.Errorf("q=paket = %v, want all four titles case-insensitively", got)
	}

	page := list(ProductFilter{Limit: 2})
	if len(page) != 2 {
		t.Fatalf("limit 2 returned %d products", len(page))
	}
	_, total, err := h.svc.List(ctx, h.scope(tenant), h.admin(tenant), ProductFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list total: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	rest := list(ProductFilter{Limit: 2, Offset: 2})
	if len(rest) != 2 || rest[0].ID == page[0].ID {
		t.Errorf("offset page = %v, want the other two products", titles(rest))
	}

	if _, _, err := h.svc.List(ctx, h.scope(tenant), h.admin(tenant), ProductFilter{Type: "kecermatan"}); err == nil {
		t.Error("unknown type filter: err = nil, want a 422")
	}
}

func TestTryoutDeleteBlockedByProduct(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	_, tryout := h.makeVisible(t, tenant)

	err := h.t.svc.Delete(ctx, h.scope(tenant), h.admin(tenant), tryout.ID)
	if !errors.Is(err, ErrTryoutReferenced) {
		t.Errorf("delete a tryout a product uses: err = %v, want ErrTryoutReferenced (409)", err)
	}
	if _, err := h.t.svc.Get(ctx, h.scope(tenant), h.admin(tenant), tryout.ID); err != nil {
		t.Errorf("the refused delete also removed the tryout: %v", err)
	}
}

func TestProgramDeleteBlockedByProduct(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	h.makeVisible(t, tenant)

	err := h.t.curriculum.Delete(ctx, h.scope(tenant), h.admin(tenant), h.t.q.program(tenant).ID)
	if !errors.Is(err, ErrProgramReferenced) {
		t.Errorf("delete a program a product uses: err = %v, want ErrProgramReferenced (409)", err)
	}
}

func TestConcurrentProductSavesAreAtomic(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	tryouts := []model.Tryout{
		h.tryout(t, tenant, model.TryoutPublished),
		h.tryout(t, tenant, model.TryoutPublished),
		h.tryout(t, tenant, model.TryoutPublished),
	}

	in := h.input(tenant, tryouts[0], tryouts[1])
	in.IsActive = false
	created := h.create(t, tenant, in)

	sets := [][]uuid.UUID{
		{tryouts[0].ID, tryouts[1].ID},
		{tryouts[0].ID, tryouts[2].ID},
		{tryouts[1].ID, tryouts[2].ID},
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(sets)*4)
	for round := 0; round < 4; round++ {
		for _, set := range sets {
			wg.Add(1)
			go func(ids []uuid.UUID) {
				defer wg.Done()
				next := h.input(tenant, tryouts[0], tryouts[1])
				next.Title = created.Title
				next.IsActive = false
				next.TryoutIDs = ids
				_, err := h.svc.Update(ctx, h.scope(tenant), h.admin(tenant), created.ID, next)
				errs <- err
			}(set)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent update: %v", err)
		}
	}

	stored, err := h.svc.Get(ctx, h.scope(tenant), h.admin(tenant), created.ID)
	if err != nil {
		t.Fatalf("get after concurrent saves: %v", err)
	}
	if len(stored.Tryouts) != 2 {
		t.Fatalf("final package has %d tryouts, want exactly one submitted set of 2", len(stored.Tryouts))
	}
	got := map[uuid.UUID]bool{stored.Tryouts[0].ID: true, stored.Tryouts[1].ID: true}
	for _, set := range sets {
		if got[set[0]] && got[set[1]] {
			return
		}
	}
	t.Errorf("final package = %v, want exactly one of the submitted sets", stored.Tryouts)
}

func TestProductOwnWindowHidesItIndependentlyOfItsTryouts(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a

	tryout := h.tryout(t, tenant, model.TryoutPublished)
	in := h.input(tenant, tryout)
	until := h.clock.Now().Add(time.Hour)
	in.ActiveUntil = &until
	created := h.create(t, tenant, in)
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), created.ID) {
		t.Fatal("a product inside its own window is missing from the catalog")
	}

	h.clock.Advance(2 * time.Hour)
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), created.ID) {
		t.Error("a product past its own active_until is still listed while its tryout is still open")
	}
}

func TestPriceGrammarRejectsAnIncompleteDecimal(t *testing.T) {
	for _, raw := range []string{"5.", ".5", "5.0.0", "+5", "١٢٣"} {
		if got, ok := parseMoney(raw); ok {
			t.Errorf("parseMoney(%q) = %q, want it rejected", raw, got)
		}
	}
}

func TestProductWithoutTryoutsIsNeverPublic(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a

	created, _ := h.makeVisible(t, tenant)
	if _, err := h.pool.Exec(context.Background(), `DELETE FROM product_tryouts WHERE tenant_id = $1 AND product_id = $2`,
		tenant.ID, created.ID); err != nil {
		t.Fatalf("detach tryouts: %v", err)
	}
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), created.ID) {
		t.Error("an active product with no tryouts is listed publicly")
	}
}
