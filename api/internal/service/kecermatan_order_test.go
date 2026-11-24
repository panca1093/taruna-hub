package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func (h kecHarness) kecProduct(t *testing.T, tenant model.Tenant, test model.KecermatanTest, price string, attempts int) model.Product {
	t.Helper()
	in := h.input(tenant)
	in.Type, in.TryoutIDs, in.Price = model.ProductKecermatan, nil, price
	in.KecermatanTestID, in.IncludedAttempts = test.ID, attempts
	return h.create(t, tenant, in)
}

func TestKecermatanProductValidation(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	foreign := h.testIn(t, h.t.q.b)
	ctx := context.Background()

	cases := []struct {
		name   string
		mutate func(*ProductInput)
		field  string
	}{
		{"no test", func(in *ProductInput) { in.KecermatanTestID = uuid.Nil }, "kecermatan_test_id"},
		{"missing test", func(in *ProductInput) { in.KecermatanTestID = uuid.New() }, "kecermatan_test_id"},
		{"another tenant's test", func(in *ProductInput) { in.KecermatanTestID = foreign.ID }, "kecermatan_test_id"},
		{"zero attempts", func(in *ProductInput) { in.IncludedAttempts = 0 }, "included_attempts"},
		{"1001 attempts", func(in *ProductInput) { in.IncludedAttempts = 1001 }, "included_attempts"},
		{"with a tryout", func(in *ProductInput) { in.TryoutIDs = []uuid.UUID{uuid.New()} }, "tryout_ids"},
		{"another program's test", func(in *ProductInput) {
			in.ProgramID = uuid.New()
		}, "program_id"},
		{"an individual product naming a test", func(in *ProductInput) {
			in.Type, in.TryoutIDs = model.ProductIndividual, []uuid.UUID{uuid.New()}
		}, "type"},
	}
	for _, tc := range cases {
		in := h.input(tenant)
		in.Type, in.TryoutIDs, in.KecermatanTestID, in.IncludedAttempts = model.ProductKecermatan, nil, test.ID, 3
		tc.mutate(&in)
		_, err := h.productHarness.svc.Create(ctx, h.scope(tenant), h.admin(tenant), in)
		wantProductField(t, err, tc.field)
	}

	product := h.kecProduct(t, tenant, test, "20000", 3)
	if product.Type != model.ProductKecermatan || product.KecermatanTestID == nil || *product.KecermatanTestID != test.ID ||
		product.IncludedAttempts == nil || *product.IncludedAttempts != 3 || len(product.Tryouts) != 0 {
		t.Errorf("product = %+v, want a kecermatan product of 3 attempts with no tryouts", product)
	}
	got, err := h.productHarness.svc.Get(ctx, h.scope(tenant), h.admin(tenant), product.ID)
	if err != nil || got.KecermatanTestTitle != test.Title {
		t.Errorf("get = %+v, %v, want the test title joined in", got, err)
	}
	if err := h.kec.Delete(ctx, h.scope(tenant), h.staff(tenant, model.RoleAdmin), test.ID); !errors.Is(err, ErrKecermatanLocked) {
		t.Errorf("delete a test that a product sells: err = %v, want ErrKecermatanLocked", err)
	}
}

func TestKecermatanCatalogShowsTheTestWithoutItsSeed(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	ctx := context.Background()
	product := h.kecProduct(t, tenant, test, "20000", 3)

	list, total, err := h.productHarness.svc.CatalogProducts(ctx, h.scope(tenant), ProductFilter{Type: model.ProductKecermatan})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("catalog = %d products, %v, want the one kecermatan product", total, err)
	}
	k := list[0].Kecermatan
	if k == nil || k.TestID != test.ID || k.KolomCount != kecKolomCount || k.SoalPerKolom != kecSoalCount ||
		k.KolomDurationSeconds != kecKolomSeconds || k.OptionsPerSoal != 5 || *list[0].IncludedAttempts != 3 {
		t.Errorf("catalog product = %+v, want the test's structure and the attempts per purchase", list[0])
	}
	if other, _, _ := h.productHarness.svc.CatalogProducts(ctx, h.scope(tenant), ProductFilter{Type: model.ProductIndividual}); len(other) != 0 {
		t.Errorf("individual filter = %d, want the kecermatan product excluded", len(other))
	}
	in := h.input(tenant)
	in.Type, in.TryoutIDs, in.KecermatanTestID, in.IncludedAttempts, in.IsActive = model.ProductKecermatan, nil, test.ID, 3, false
	in.Price = product.Price
	h.update(t, tenant, product.ID, in)
	if _, err := h.productHarness.svc.CatalogProduct(ctx, h.scope(tenant), product.ID); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("inactive product: err = %v, want not found", err)
	}
}

func TestKecermatanCheckoutAddsCreditsToOneEnrollment(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	ctx := context.Background()
	product := h.kecProduct(t, tenant, test, "20000", 3)
	user, p := h.student(t, tenant, "100000")

	order, replayed, err := h.wallet(t, tenant, p, product)
	if err != nil || replayed || order.Status != model.OrderConfirmed || order.KecermatanTestID == nil || *order.IncludedAttempts != 3 || len(order.Items) != 0 {
		t.Fatalf("checkout = %+v replayed=%v err=%v, want a confirmed kecermatan order of 3 attempts", order, replayed, err)
	}
	if c := h.credit(t, user, test); c.purchased != 3 || c.used != 0 {
		t.Fatalf("credit = %+v, want 3 attempts", c)
	}

	// A replay of the same key changes nothing: no second debit and no second credit.
	key := "replay-" + uuid.NewString()[:8]
	in := CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: key}
	if _, _, err := h.svc.Checkout(ctx, h.scope(tenant), p, in); err != nil {
		t.Fatalf("second purchase: %v", err)
	}
	if _, replayed, err := h.svc.Checkout(ctx, h.scope(tenant), p, in); err != nil || !replayed {
		t.Fatalf("replay = %v, %v, want a replay", replayed, err)
	}
	if c := h.credit(t, user, test); c.purchased != 6 {
		t.Errorf("credit = %+v, want 6 after two purchases and one replay", c)
	}
	if n := h.enrollments(t, user.ID); n != 1 {
		t.Errorf("enrollments = %d, want the purchases to share one", n)
	}
	if bal := h.balance(t, user.ID); bal != "60000.00" {
		t.Errorf("balance = %s, want 100000 - 2 x 20000", bal)
	}

	// Price drift, a thin wallet and the credit all fail together.
	if _, _, err := h.svc.Checkout(ctx, h.scope(tenant), p, CheckoutInput{ProductID: product.ID, ExpectedPrice: "1000", Method: model.PaymentWallet, Key: "drift-" + uuid.NewString()[:8]}); !errors.Is(err, ErrPriceChanged) {
		t.Errorf("stale price: err = %v, want ErrPriceChanged", err)
	}
	_, poor := h.student(t, tenant, "5000")
	if _, _, err := h.wallet(t, tenant, poor, product); err == nil {
		t.Error("a purchase beyond the balance succeeded")
	}
	if n := h.count(t, `SELECT count(*) FROM enrollments WHERE user_id = $1`, poor.UserID); n != 0 {
		t.Errorf("a failed purchase left %d enrollments, want none", n)
	}
}

func TestKecermatanManualOrderAddsCreditsOnConfirmOnly(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	test := h.testIn(t, tenant)
	ctx := context.Background()
	product := h.kecProduct(t, tenant, test, "20000", 2)
	user, p := h.student(t, tenant, "")
	staff := h.staff(tenant, model.RoleOperator)

	pending := h.manual(t, tenant, p, product)
	if n := h.enrollments(t, user.ID); n != 0 {
		t.Fatalf("a pending order opened %d enrollments, want none", n)
	}
	proof := h.proof(t, tenant, p)
	if _, _, err := h.svc.Checkout(ctx, h.scope(tenant), p, CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "second-" + uuid.NewString()[:8], ProofAssetID: &proof}); !errors.Is(err, ErrOrderPending) {
		t.Errorf("second pending order: err = %v, want ErrOrderPending", err)
	}
	if _, err := h.svc.Confirm(ctx, h.scope(tenant), staff, pending.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := h.svc.Confirm(ctx, h.scope(tenant), staff, pending.ID); err != nil {
		t.Fatalf("repeat confirm: %v", err)
	}
	if c := h.credit(t, user, test); c.purchased != 2 {
		t.Errorf("credit = %+v, want 2 after a confirm and its repeat", c)
	}

	second := h.manual(t, tenant, p, product)
	if _, err := h.svc.Cancel(ctx, h.scope(tenant), staff, second.ID, "bukti tidak sesuai"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if c := h.credit(t, user, test); c.purchased != 2 {
		t.Errorf("credit after a cancel = %+v, want still 2", c)
	}
	third := h.manual(t, tenant, p, product)
	if _, err := h.svc.Confirm(ctx, h.scope(tenant), staff, third.ID); err != nil {
		t.Fatalf("confirm third: %v", err)
	}
	if c := h.credit(t, user, test); c.purchased != 4 {
		t.Errorf("credit = %+v, want a repeat purchase to add 2 more to the same enrollment", c)
	}
	if g := h.grants(t, user.ID); g != 2 {
		t.Errorf("grants = %d, want one per confirmed order", g)
	}
}

func TestMixedBundleValidationAndCatalog(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	test, other := h.testIn(t, tenant), h.testIn(t, tenant)
	foreign := h.testIn(t, h.t.q.b)

	many := make([]uuid.UUID, 0, 20)
	for range 20 {
		many = append(many, uuid.New())
	}
	cases := []struct {
		name   string
		mutate func(*ProductInput)
		field  string
	}{
		{"one item", func(in *ProductInput) { in.TryoutIDs = nil }, "tryout_ids"},
		{"21 items", func(in *ProductInput) { in.TryoutIDs = many }, "tryout_ids"},
		{"duplicate test", func(in *ProductInput) { in.KecermatanTestIDs = []uuid.UUID{test.ID, test.ID} }, "kecermatan_test_ids[1]"},
		{"another tenant's test", func(in *ProductInput) { in.KecermatanTestIDs = []uuid.UUID{foreign.ID} }, "kecermatan_test_ids[0]"},
		{"tests on an individual product", func(in *ProductInput) { in.Type = model.ProductIndividual }, "kecermatan_test_ids"},
	}
	for _, tc := range cases {
		in := h.input(tenant, tryout)
		in.Type, in.KecermatanTestIDs = model.ProductBundle, []uuid.UUID{test.ID}
		tc.mutate(&in)
		_, err := h.productHarness.svc.Create(ctx, h.scope(tenant), h.admin(tenant), in)
		wantProductField(t, err, tc.field)
	}

	in := h.input(tenant, tryout)
	in.Type, in.KecermatanTestIDs = model.ProductBundle, []uuid.UUID{test.ID, other.ID}
	mixed := h.create(t, tenant, in)
	if len(mixed.Tryouts) != 1 || len(mixed.KecermatanTests) != 2 || mixed.KecermatanTests[0].Title != test.Title {
		t.Fatalf("mixed bundle = %+v, want one tryout and both tests in order", mixed)
	}
	onlyTests := h.input(tenant)
	onlyTests.Type, onlyTests.KecermatanTestIDs = model.ProductBundle, []uuid.UUID{test.ID, other.ID}
	testsBundle := h.create(t, tenant, onlyTests)

	got, err := h.productHarness.svc.CatalogProduct(ctx, h.scope(tenant), testsBundle.ID)
	if err != nil || len(got.KecermatanTests) != 2 || got.KecermatanTests[1].TestID != other.ID || got.KecermatanTests[1].KolomCount != kecKolomCount {
		t.Fatalf("catalog of a tests-only bundle = %+v, %v, want it on sale with both tests", got, err)
	}
	if err := h.kec.Delete(ctx, h.scope(tenant), h.staff(tenant, model.RoleAdmin), test.ID); !errors.Is(err, ErrKecermatanLocked) {
		t.Errorf("delete a test that a bundle sells: err = %v, want ErrKecermatanLocked", err)
	}

	// Buy-more prefers the test's own product, then the smallest bundle holding it.
	if id, err := h.repo.SellableProduct(ctx, tenant.ID, other.ID, h.clock.Now()); err != nil || id != testsBundle.ID {
		t.Errorf("sellable for a bundled test = %v, %v, want the smaller tests-only bundle", id, err)
	}
	own := h.kecProduct(t, tenant, other, "20000", 3)
	if id, err := h.repo.SellableProduct(ctx, tenant.ID, other.ID, h.clock.Now()); err != nil || id != own.ID {
		t.Errorf("sellable = %v, %v, want the test's own product first", id, err)
	}
}

func TestMixedBundleCreditsEveryMemberOncePerOrder(t *testing.T) {
	h := newKecHarness(t)
	tenant := h.t.q.a
	ctx := context.Background()
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	test := h.testIn(t, tenant)
	in := h.input(tenant, tryout)
	in.Type, in.KecermatanTestIDs, in.IncludedAttempts, in.Price = model.ProductBundle, []uuid.UUID{test.ID}, 2, "30000"
	bundle := h.create(t, tenant, in)
	user, p := h.student(t, tenant, "")
	staff := h.staff(tenant, model.RoleOperator)

	pending := h.manual(t, tenant, p, bundle)
	// A later edit drops the test; the pending order still delivers what was bought.
	edited := in
	edited.KecermatanTestIDs = []uuid.UUID{h.testIn(t, tenant).ID}
	h.update(t, tenant, bundle.ID, edited)

	for range 2 {
		if _, err := h.svc.Confirm(ctx, h.scope(tenant), staff, pending.ID); err != nil {
			t.Fatalf("confirm: %v", err)
		}
	}
	if c := h.credit(t, user, test); c.purchased != 2 {
		t.Errorf("kecermatan credit = %+v, want 2 from one confirmed order", c)
	}
	if n := h.count(t, `SELECT attempts_purchased FROM enrollments WHERE user_id = $1 AND tryout_id = $2`, user.ID, tryout.ID); n != 2 {
		t.Errorf("tryout credit = %d, want 2 from one confirmed order", n)
	}
	if g := h.grants(t, user.ID); g != 2 {
		t.Errorf("grants = %d, want one per member", g)
	}
}
