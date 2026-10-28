package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var testBank = BankAccount{Bank: "Bank Uji", Number: "1234567890", Holder: "Taruna Eduka"}

// memStore keeps uploaded proofs in memory; storage itself is P06's concern and has its own tests.
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memStore) Put(_ context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = map[string][]byte{}
	}
	m.objects[key] = data
	return nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *memStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

// failingJobs makes the notification enqueue fail, which is the last step of every checkout transaction.
type failingJobs struct{}

func (failingJobs) InsertNotificationTx(context.Context, pgx.Tx, NotificationJobArgs) error {
	return errors.New("job queue is down")
}

type orderHarness struct {
	productHarness
	svc     *OrderService
	wallets *WalletService
	assets  *AssetService
	users   *repository.UserRepository
}

func newOrderHarness(t *testing.T, jobs JobInserter, bank BankAccount) orderHarness {
	t.Helper()
	base := newProductHarness(t)
	h := orderHarness{productHarness: base, users: repository.NewUserRepository(base.pool)}
	h.wallets = NewWalletService(base.pool, repository.NewWalletRepository(base.pool))
	h.assets = NewAssetService(base.pool, repository.NewAssetRepository(base.pool), &memStore{}, time.Minute)
	notes, err := NewNotificationService(repository.NewNotificationRepository(base.pool), jobs, nil,
		bytes.Repeat([]byte{5}, SealKeyBytes), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.svc = NewOrderService(base.pool, repository.NewOrderRepository(base.pool), repository.NewProductRepository(base.pool),
		h.wallets, h.assets, notes, bank, WithOrderClock(base.clock.Now))
	// Registered last, so orders, grants, proofs and the ledger go before the tryouts and users they reference.
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []uuid.UUID{h.t.q.a.ID, h.t.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM enrollment_grants WHERE tenant_id = ANY($1)`,
			`DELETE FROM enrollments WHERE tenant_id = ANY($1)`,
			`DELETE FROM order_items WHERE tenant_id = ANY($1)`,
			`DELETE FROM orders WHERE tenant_id = ANY($1)`,
			`DELETE FROM assets WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(ctx, sql, ids); err != nil {
				t.Errorf("cleanup order fixture: %v", err)
			}
		}
		wipeLedger(t, h.pool, ids)
	})
	return h
}

func (h orderHarness) student(t *testing.T, tenant model.Tenant, balance string) (model.User, model.Principal) {
	t.Helper()
	user := walletUser(t, h.users, tenant.ID, model.RoleStudent, "Siswa Uji")
	if balance != "" {
		err := h.inTx(func(tx pgx.Tx) error {
			_, _, err := h.wallets.Apply(context.Background(), tx, Movement{
				TenantID: tenant.ID, UserID: user.ID, Amount: balance, Kind: model.KindTopup,
				ActorKind: model.ActorProvider, SourceType: "topup", SourceID: uuid.NewString(),
			})
			return err
		})
		if err != nil {
			t.Fatalf("fund student: %v", err)
		}
	}
	id := tenant.ID
	return user, model.Principal{UserID: user.ID, TenantID: &id, Name: user.Name, Role: model.RoleStudent}
}

func (h orderHarness) inTx(fn func(pgx.Tx) error) error {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h orderHarness) staff(tenant model.Tenant, role model.Role) model.Principal {
	return h.t.principal(tenant, role)
}

func (h orderHarness) product(t *testing.T, tenant model.Tenant, price string, tryouts ...model.Tryout) model.Product {
	t.Helper()
	in := h.input(tenant, tryouts...)
	in.Price = price
	return h.create(t, tenant, in)
}

func (h orderHarness) proof(t *testing.T, tenant model.Tenant, p model.Principal) uuid.UUID {
	t.Helper()
	asset, err := h.assets.Upload(context.Background(), h.scope(tenant), p, model.PurposePaymentProof, testPNG(t, 8, 8))
	if err != nil {
		t.Fatalf("upload proof: %v", err)
	}
	return asset.ID
}

func (h orderHarness) wallet(t *testing.T, tenant model.Tenant, p model.Principal, product model.Product) (model.Order, bool, error) {
	t.Helper()
	return h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "key-" + uuid.NewString()[:12],
	})
}

func (h orderHarness) manual(t *testing.T, tenant model.Tenant, p model.Principal, product model.Product) model.Order {
	t.Helper()
	proof := h.proof(t, tenant, p)
	order, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof,
	})
	if err != nil {
		t.Fatalf("manual checkout: %v", err)
	}
	return order
}

func (h orderHarness) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func (h orderHarness) balance(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	var balance string
	if err := h.pool.QueryRow(context.Background(), `SELECT balance::text FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	return balance
}

func (h orderHarness) enrollments(t *testing.T, userID uuid.UUID) int {
	return h.count(t, `SELECT count(*) FROM enrollments WHERE user_id = $1`, userID)
}

func (h orderHarness) grants(t *testing.T, userID uuid.UUID) int {
	return h.count(t, `SELECT count(*) FROM enrollment_grants g JOIN enrollments e ON e.id = g.enrollment_id WHERE e.user_id = $1`, userID)
}

func (h orderHarness) ordersOf(t *testing.T, userID uuid.UUID) int {
	return h.count(t, `SELECT count(*) FROM orders WHERE user_id = $1`, userID)
}

func (h orderHarness) ledger(t *testing.T, userID uuid.UUID) int {
	return h.count(t, `SELECT count(*) FROM wallet_entries WHERE user_id = $1`, userID)
}

func (h orderHarness) notifications(t *testing.T, tenant model.Tenant, template string) int {
	return h.count(t, `SELECT count(*) FROM notification_logs WHERE tenant_id = $1 AND template = $2`, tenant.ID, template)
}

func wantInputField(t *testing.T, err error, field string) {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) || input.Fields[field] == "" {
		t.Fatalf("err = %v, want an InputError on %q", err, field)
	}
}

func TestWalletCheckoutDebitsConfirmsAndGrants(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "250000")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))

	order, replayed, err := h.wallet(t, tenant, p, product)
	if err != nil || replayed {
		t.Fatalf("wallet checkout = %v, replayed %v", err, replayed)
	}
	if order.Status != model.OrderConfirmed || order.PaymentMethod != model.PaymentWallet || order.ConfirmedAt == nil {
		t.Fatalf("order = %+v, want a confirmed wallet order", order)
	}
	if order.ProductTitle != product.Title || order.Price != "100000.00" || len(order.Items) != 1 {
		t.Fatalf("snapshot = %q %s %d items", order.ProductTitle, order.Price, len(order.Items))
	}
	if got := h.balance(t, user.ID); got != "150000.00" {
		t.Errorf("balance = %s, want 150000.00", got)
	}
	if h.enrollments(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("enrollments %d grants %d, want 1 and 1", h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
	var amount, sourceID, kind string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT amount::text, source_id, kind FROM wallet_entries WHERE user_id = $1 AND source_type = 'order'`,
		user.ID).Scan(&amount, &sourceID, &kind); err != nil {
		t.Fatalf("read the debit: %v", err)
	}
	if amount != "-100000.00" || sourceID != order.ID.String() || kind != model.KindSpend {
		t.Errorf("ledger = %s %s %s, want one spend of the order", amount, sourceID, kind)
	}
	// Email and WhatsApp, one each, enqueued with the order.
	if got := h.notifications(t, tenant, TemplateOrderConfirmed); got != 2 {
		t.Errorf("confirmation notifications = %d, want 2", got)
	}
}

func TestInsufficientBalanceChangesNothing(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "99999.99")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	ledger := h.ledger(t, user.ID)

	_, _, err := h.wallet(t, tenant, p, product)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	if h.ordersOf(t, user.ID) != 0 || h.enrollments(t, user.ID) != 0 || h.grants(t, user.ID) != 0 {
		t.Errorf("orders %d enrollments %d grants %d, want none", h.ordersOf(t, user.ID), h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
	if got := h.balance(t, user.ID); got != "99999.99" || h.ledger(t, user.ID) != ledger {
		t.Errorf("balance %s ledger %d, want untouched", got, h.ledger(t, user.ID))
	}
	if h.notifications(t, tenant, TemplateOrderConfirmed) != 0 {
		t.Error("a notification was queued for a payment that failed")
	}
}

func TestMidTransactionFailureRollsBackTheDebit(t *testing.T) {
	h := newOrderHarness(t, failingJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "250000")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	ledger := h.ledger(t, user.ID)

	if _, _, err := h.wallet(t, tenant, p, product); err == nil {
		t.Fatal("checkout succeeded although the notification job could not be queued")
	}
	if got := h.balance(t, user.ID); got != "250000.00" || h.ledger(t, user.ID) != ledger {
		t.Errorf("balance %s ledger %d, want the debit rolled back", got, h.ledger(t, user.ID))
	}
	if h.ordersOf(t, user.ID) != 0 || h.enrollments(t, user.ID) != 0 || h.grants(t, user.ID) != 0 {
		t.Errorf("orders %d enrollments %d grants %d, want none", h.ordersOf(t, user.ID), h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
}

func TestRepeatedWalletCheckoutIsOneDebitOrderAndGrant(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "500000")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	in := CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "retry-key-0001"}

	first, replayed, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, in)
	if err != nil || replayed {
		t.Fatalf("first = %v, replayed %v", err, replayed)
	}
	second, replayed, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, in)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("second = %v, replayed %v, id %s want %s", err, replayed, second.ID, first.ID)
	}
	if got := h.balance(t, user.ID); got != "400000.00" {
		t.Errorf("balance = %s, want one debit", got)
	}
	if h.ordersOf(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("orders %d grants %d, want 1 and 1", h.ordersOf(t, user.ID), h.grants(t, user.ID))
	}

	other := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	_, _, err = h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: other.ID, ExpectedPrice: other.Price, Method: model.PaymentWallet, Key: in.Key,
	})
	if !errors.Is(err, ErrOrderKeyConflict) {
		t.Errorf("same key for another product: err = %v, want ErrOrderKeyConflict", err)
	}
}

func TestConcurrentWalletCheckoutsWithOneKeyDebitOnce(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "500000")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	in := CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "burst-key-0001"}

	var wg sync.WaitGroup
	var failures atomic.Int32
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, in); err != nil {
				failures.Add(1)
				t.Errorf("checkout: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := h.balance(t, user.ID); got != "400000.00" || failures.Load() != 0 {
		t.Errorf("balance %s, failures %d, want one debit and no failure", got, failures.Load())
	}
	if h.ordersOf(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("orders %d grants %d, want 1 and 1", h.ordersOf(t, user.ID), h.grants(t, user.ID))
	}
}

func TestManualOrderWaitsThenConfirmOpensAccessOnce(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))

	order := h.manual(t, tenant, p, product)
	if order.Status != model.OrderPending || order.ProofAssetID == nil {
		t.Fatalf("order = %+v, want pending with a proof", order)
	}
	if h.enrollments(t, user.ID) != 0 || h.notifications(t, tenant, TemplateOrderConfirmed) != 0 {
		t.Fatal("a pending order opened access or notified")
	}
	if got := h.count(t, `SELECT count(*) FROM assets WHERE id = $1 AND status = 'final'`, *order.ProofAssetID); got != 1 {
		t.Error("the proof was not finalized with the order")
	}

	operator := h.staff(tenant, model.RoleOperator)
	confirmed, err := h.svc.Confirm(context.Background(), h.scope(tenant), operator, order.ID)
	if err != nil || confirmed.Status != model.OrderConfirmed || confirmed.ReviewedBy == nil {
		t.Fatalf("confirm = %+v, %v", confirmed, err)
	}
	again, err := h.svc.Confirm(context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin), order.ID)
	if err != nil || again.ConfirmedAt == nil || !again.ConfirmedAt.Equal(*confirmed.ConfirmedAt) {
		t.Fatalf("second confirm = %+v, %v; want the first transition unchanged", again, err)
	}
	if h.enrollments(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("enrollments %d grants %d, want 1 and 1", h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
	if got := h.notifications(t, tenant, TemplateOrderConfirmed); got != 2 {
		t.Errorf("confirmation notifications = %d, want 2 (email and WhatsApp, once)", got)
	}
}

func TestConcurrentConfirmIsOneTransition(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	order := h.manual(t, tenant, p, product)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.svc.Confirm(context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin), order.ID); err != nil {
				t.Errorf("confirm: %v", err)
			}
		}()
	}
	wg.Wait()
	if h.grants(t, user.ID) != 1 || h.notifications(t, tenant, TemplateOrderConfirmed) != 2 {
		t.Errorf("grants %d notifications %d, want 1 and 2", h.grants(t, user.ID), h.notifications(t, tenant, TemplateOrderConfirmed))
	}
}

func TestCancelOpensNoAccessAndNeverReopens(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	order := h.manual(t, tenant, p, product)
	admin := h.staff(tenant, model.RoleAdmin)

	_, err := h.svc.Cancel(context.Background(), h.scope(tenant), admin, order.ID, "  ")
	wantInputField(t, err, "reason")

	cancelled, err := h.svc.Cancel(context.Background(), h.scope(tenant), admin, order.ID, "Bukti tidak sesuai")
	if err != nil || cancelled.Status != model.OrderCancelled || cancelled.CancelReason != "Bukti tidak sesuai" {
		t.Fatalf("cancel = %+v, %v", cancelled, err)
	}
	if h.enrollments(t, user.ID) != 0 {
		t.Error("a cancelled order opened access")
	}
	if got := h.notifications(t, tenant, TemplateOrderCancelled); got != 2 {
		t.Errorf("cancellation notifications = %d, want 2", got)
	}
	if _, err := h.svc.Confirm(context.Background(), h.scope(tenant), admin, order.ID); !errors.Is(err, ErrOrderState) {
		t.Errorf("confirm after cancel: err = %v, want ErrOrderState", err)
	}
	repeat, err := h.svc.Cancel(context.Background(), h.scope(tenant), admin, order.ID, "Alasan lain")
	if err != nil || repeat.CancelReason != "Bukti tidak sesuai" {
		t.Errorf("repeat cancel = %+v, %v; want the first reason kept", repeat, err)
	}

	second := h.manual(t, tenant, p, product)
	if _, err := h.svc.Confirm(context.Background(), h.scope(tenant), admin, second.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := h.svc.Cancel(context.Background(), h.scope(tenant), admin, second.ID, "Terlambat"); !errors.Is(err, ErrOrderState) {
		t.Errorf("cancel after confirm: err = %v, want ErrOrderState (a confirmed order never goes back)", err)
	}
}

func TestIndividualProductCannotBeBoughtTwice(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "500000")
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	product := h.product(t, tenant, "100000", tryout)
	again := h.product(t, tenant, "90000", tryout)

	if _, _, err := h.wallet(t, tenant, p, product); err != nil {
		t.Fatalf("first purchase: %v", err)
	}
	for _, repeat := range []model.Product{product, again} {
		if _, _, err := h.wallet(t, tenant, p, repeat); !errors.Is(err, ErrAlreadyOwned) {
			t.Errorf("repurchase of %s: err = %v, want ErrAlreadyOwned", repeat.Title, err)
		}
	}
	if _, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "manual-key-001", ProofAssetID: refOf(h.proof(t, tenant, p)),
	}); !errors.Is(err, ErrAlreadyOwned) {
		t.Errorf("manual repurchase: err = %v, want ErrAlreadyOwned", err)
	}
	if got := h.balance(t, user.ID); got != "400000.00" || h.ordersOf(t, user.ID) != 1 {
		t.Errorf("balance %s orders %d, want one purchase only", got, h.ordersOf(t, user.ID))
	}
}

func TestBundleGrantsOnlyTheTryoutsNotYetHeld(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "900000")
	a := h.tryout(t, tenant, model.TryoutPublished)
	b := h.tryout(t, tenant, model.TryoutPublished)
	c := h.tryout(t, tenant, model.TryoutPublished)
	single := h.product(t, tenant, "100000", a)
	bundle := h.product(t, tenant, "250000", a, b, c)

	if _, _, err := h.wallet(t, tenant, p, single); err != nil {
		t.Fatalf("single: %v", err)
	}
	order, _, err := h.wallet(t, tenant, p, bundle)
	if err != nil {
		t.Fatalf("a bundle with a held tryout must still be purchasable: %v", err)
	}
	if len(order.Items) != 3 {
		t.Errorf("snapshot has %d tryouts, want all 3", len(order.Items))
	}
	if h.enrollments(t, user.ID) != 3 || h.grants(t, user.ID) != 3 {
		t.Errorf("enrollments %d grants %d, want 3 and 3 (A once)", h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
	if got := h.count(t, `SELECT count(*) FROM enrollment_grants g JOIN enrollments e ON e.id = g.enrollment_id
		WHERE e.user_id = $1 AND g.order_id = $2`, user.ID, order.ID); got != 2 {
		t.Errorf("the bundle order granted %d tryouts, want 2 (B and C)", got)
	}
	if got := h.balance(t, user.ID); got != "550000.00" {
		t.Errorf("balance = %s, want both purchases charged", got)
	}
}

func TestBundleGrantsEveryTryout(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	bundle := h.product(t, tenant, "250000",
		h.tryout(t, tenant, model.TryoutPublished), h.tryout(t, tenant, model.TryoutPublished), h.tryout(t, tenant, model.TryoutPublished))
	order := h.manual(t, tenant, p, bundle)
	if _, err := h.svc.Confirm(context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin), order.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if h.enrollments(t, user.ID) != 3 {
		t.Errorf("enrollments = %d, want 3", h.enrollments(t, user.ID))
	}
	holds, err := h.svc.Enrollments(context.Background(), h.scope(tenant), p)
	if err != nil || len(holds) != 3 {
		t.Errorf("Enrollments = %d, %v; want the 3 held", len(holds), err)
	}
}

func TestSaleWindowDecidesWhatCanBeBought(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "500000")
	tryout := h.tryout(t, tenant, model.TryoutPublished)

	in := h.input(tenant, tryout)
	from := h.clock.Now().Add(time.Hour)
	until := h.clock.Now().Add(2 * time.Hour)
	in.ActiveFrom, in.ActiveUntil = &from, &until
	product := h.create(t, tenant, in)

	if _, _, err := h.wallet(t, tenant, p, product); !errors.Is(err, ErrProductUnavailable) {
		t.Errorf("before the window: err = %v, want ErrProductUnavailable", err)
	}
	if catalogHas(h.catalog(t, tenant, ProductFilter{}), product.ID) {
		t.Error("the catalog lists a product before its sale window")
	}
	h.clock.Advance(90 * time.Minute)
	if !catalogHas(h.catalog(t, tenant, ProductFilter{}), product.ID) {
		t.Fatal("the catalog hides a product inside its sale window")
	}
	if _, _, err := h.wallet(t, tenant, p, product); err != nil {
		t.Errorf("inside the window: %v", err)
	}

	other := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	h.clock.Advance(time.Hour * 24 * 40)
	if _, _, err := h.wallet(t, tenant, p, other); !errors.Is(err, ErrProductUnavailable) {
		t.Errorf("after the window: err = %v, want ErrProductUnavailable", err)
	}
}

func TestProductWindowMustStartBeforeItEnds(t *testing.T) {
	h := newProductHarness(t)
	tenant := h.t.q.a
	published := h.tryout(t, tenant, model.TryoutPublished)
	in := h.input(tenant, published)
	from := h.clock.Now().Add(48 * time.Hour)
	in.ActiveFrom = &from
	until := from
	in.ActiveUntil = &until
	_, err := h.svc.Create(context.Background(), h.scope(tenant), h.admin(tenant), in)
	wantProductField(t, err, "active_from")
}

func TestInactiveProductCannotBeBought(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "500000")
	in := h.input(tenant, h.tryout(t, tenant, model.TryoutPublished))
	in.IsActive = false
	product := h.create(t, tenant, in)
	if _, _, err := h.wallet(t, tenant, p, product); !errors.Is(err, ErrProductUnavailable) {
		t.Errorf("err = %v, want ErrProductUnavailable", err)
	}
	if _, _, err := h.wallet(t, tenant, p, model.Product{ID: uuid.New(), Price: "100000.00"}); !errors.Is(err, ErrProductUnavailable) {
		t.Errorf("unknown product: err = %v, want ErrProductUnavailable", err)
	}
}

func TestProofMustBeTheStudentsOwnUnusedUpload(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	mine, pMine := h.student(t, tenant, "")
	_, pOther := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished), h.tryout(t, tenant, model.TryoutPublished))

	foreign := h.proof(t, tenant, pOther)
	_, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), pMine, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "foreign-proof-1", ProofAssetID: &foreign,
	})
	wantInputField(t, err, "proof_asset_id")
	_, _, err = h.svc.Checkout(context.Background(), h.scope(tenant), pMine, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "missing-proof-1", ProofAssetID: refOf(uuid.New()),
	})
	wantInputField(t, err, "proof_asset_id")
	if h.ordersOf(t, mine.ID) != 0 {
		t.Error("a rejected proof still left an order")
	}
	if got := h.count(t, `SELECT count(*) FROM assets WHERE id = $1 AND status = 'staged'`, foreign); got != 1 {
		t.Error("another student's proof was finalized by a rejected checkout")
	}

	order := h.manual(t, tenant, pMine, product)
	_, _, err = h.svc.Checkout(context.Background(), h.scope(tenant), pMine, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "reuse-proof-001", ProofAssetID: order.ProofAssetID,
	})
	wantInputField(t, err, "proof_asset_id")
}

func TestStudentsSeeOnlyTheirOwnOrders(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, a := h.student(t, tenant, "")
	_, b := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	orderA := h.manual(t, tenant, a, product)

	own, total, err := h.svc.Orders(context.Background(), h.scope(tenant), a, 0, 0)
	if err != nil || total != 1 || len(own) != 1 || own[0].ID != orderA.ID {
		t.Fatalf("own orders = %d of %d, %v", len(own), total, err)
	}
	theirs, total, err := h.svc.Orders(context.Background(), h.scope(tenant), b, 0, 0)
	if err != nil || total != 0 || len(theirs) != 0 {
		t.Errorf("another student's list = %d of %d, %v; want none", len(theirs), total, err)
	}
	if _, err := h.svc.Order(context.Background(), h.scope(tenant), b, orderA.ID); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("reading another student's order: err = %v, want ErrOrderNotFound", err)
	}
	if _, err := h.svc.Order(context.Background(), h.scope(tenant), a, orderA.ID); err != nil {
		t.Errorf("reading an own order: %v", err)
	}
	if holds, _ := h.svc.Enrollments(context.Background(), h.scope(tenant), b); len(holds) != 0 {
		t.Error("another student's access is visible")
	}
}

func TestOnlyStaffReviewOrders(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	order := h.manual(t, tenant, p, product)
	ctx, scope := context.Background(), h.scope(tenant)

	if _, err := h.svc.Confirm(ctx, scope, p, order.ID); !errors.Is(err, ErrOrderForbidden) {
		t.Errorf("student confirm: err = %v, want ErrOrderForbidden", err)
	}
	if _, err := h.svc.Cancel(ctx, scope, p, order.ID, "x"); !errors.Is(err, ErrOrderForbidden) {
		t.Errorf("student cancel: err = %v, want ErrOrderForbidden", err)
	}
	if _, _, err := h.svc.StaffOrders(ctx, scope, p, OrderFilter{}); !errors.Is(err, ErrOrderForbidden) {
		t.Errorf("student staff list: err = %v, want ErrOrderForbidden", err)
	}
	for _, role := range []model.Role{model.RoleAdmin, model.RoleOperator} {
		list, total, err := h.svc.StaffOrders(ctx, scope, h.staff(tenant, role), OrderFilter{Status: model.OrderPending})
		if err != nil || total < 1 || len(list) < 1 {
			t.Errorf("%s list = %d, %v; want the pending order", role, len(list), err)
		}
	}
	if _, _, err := h.svc.StaffOrders(ctx, scope, h.staff(tenant, model.RoleAdmin), OrderFilter{Status: "paid"}); err == nil {
		t.Error("an unknown status filter was accepted")
	}
	if _, err := h.svc.Confirm(ctx, scope, h.staff(tenant, model.RoleOperator), order.ID); err != nil {
		t.Errorf("operator confirm: %v", err)
	}
	if _, err := h.svc.Confirm(ctx, scope, h.staff(tenant, model.RoleAdmin), uuid.New()); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("unknown order: err = %v, want ErrOrderNotFound", err)
	}
}

func TestOrderKeepsWhatWasBoughtWhenTheProductChanges(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "500000")
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	product := h.product(t, tenant, "100000", tryout)
	order, _, err := h.wallet(t, tenant, p, product)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	edit := h.input(tenant, tryout)
	edit.Title, edit.Price = "Judul Baru "+uuid.NewString()[:6], "999000"
	h.update(t, tenant, product.ID, edit)

	got, err := h.svc.Order(context.Background(), h.scope(tenant), p, order.ID)
	if err != nil || got.ProductTitle != product.Title || got.Price != "100000.00" || got.Items[0].Title != tryout.Title {
		t.Errorf("order after edit = %q %s %v, %v; want the checkout snapshot", got.ProductTitle, got.Price, got.Items, err)
	}
}

func TestCheckoutRejectsWhatItCannotPay(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "500000")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	proof := h.proof(t, tenant, p)
	ctx, scope := context.Background(), h.scope(tenant)

	cases := []struct {
		name  string
		in    CheckoutInput
		field string
	}{
		{"gateway is not available yet", CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: "gateway", Key: "valid-key-0001"}, "payment_method"},
		{"manual without a proof", CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "valid-key-0002"}, "proof_asset_id"},
		{"wallet with a proof", CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "valid-key-0003", ProofAssetID: &proof}, "proof_asset_id"},
		{"short key", CheckoutInput{ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "short"}, "idempotency_key"},
		{"no product", CheckoutInput{Method: model.PaymentWallet, Key: "valid-key-0004"}, "product_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := h.svc.Checkout(ctx, scope, p, tc.in)
			wantInputField(t, err, tc.field)
		})
	}
	if _, _, err := h.svc.Checkout(ctx, scope, h.staff(tenant, model.RoleAdmin), CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentWallet, Key: "valid-key-0005",
	}); !errors.Is(err, ErrOrderForbidden) {
		t.Errorf("staff checkout: err = %v, want ErrOrderForbidden", err)
	}
}

func TestManualCheckoutNeedsABankAccount(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, BankAccount{})
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	proof := h.proof(t, tenant, p)
	_, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "no-bank-key-01", ProofAssetID: &proof,
	})
	if !errors.Is(err, ErrBankTransferUnavailable) {
		t.Errorf("err = %v, want ErrBankTransferUnavailable", err)
	}
	if _, err := h.svc.BankTransfer(h.scope(tenant), p); !errors.Is(err, ErrBankTransferUnavailable) {
		t.Errorf("BankTransfer: err = %v, want ErrBankTransferUnavailable", err)
	}
}

func TestIssueEnrollmentsIsOncePerSource(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, _ := h.student(t, tenant, "")
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	admin := h.admin(tenant)
	in := IssueInput{
		TenantID: tenant.ID, UserID: user.ID, TryoutIDs: []uuid.UUID{tryout.ID},
		SourceType: model.GrantSourceStaff, SourceID: "grant-1", ActorKind: model.ActorStaff,
		ActorID: &admin.UserID, Reason: "Beasiswa",
	}
	issue := func(in IssueInput) ([]uuid.UUID, error) {
		var opened []uuid.UUID
		err := h.inTx(func(tx pgx.Tx) error {
			var err error
			opened, err = h.svc.IssueEnrollmentsTx(context.Background(), tx, in)
			return err
		})
		return opened, err
	}
	if opened, err := issue(in); err != nil || len(opened) != 1 {
		t.Fatalf("first issue = %v, %v", opened, err)
	}
	if opened, err := issue(in); err != nil || len(opened) != 0 {
		t.Errorf("repeat issue = %v, %v; want nothing granted twice", opened, err)
	}
	noReason := in
	noReason.SourceID, noReason.Reason = "grant-2", " "
	noReason.TryoutIDs = []uuid.UUID{h.tryout(t, tenant, model.TryoutPublished).ID}
	if _, err := issue(noReason); err == nil {
		t.Error("a staff grant without a reason was accepted")
	}
	if _, err := issue(IssueInput{TenantID: tenant.ID, UserID: user.ID, TryoutIDs: in.TryoutIDs}); !errors.Is(err, ErrEnrollmentSourceRequired) {
		t.Errorf("grant without a source: err = %v, want ErrEnrollmentSourceRequired", err)
	}
	if h.enrollments(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("enrollments %d grants %d, want 1 and 1", h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
}

func TestOrderCodeIsStable(t *testing.T) {
	for number, want := range map[int64]string{1: "ORD-000001", 42: "ORD-000042", 1234567: "ORD-1234567"} {
		if got := OrderCode(number); got != want {
			t.Errorf("OrderCode(%d) = %q, want %q", number, got, want)
		}
	}
}

func refOf[T any](v T) *T { return &v }

func TestCheckoutRejectsAPriceThatChanged(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "500000")
	tryout := h.tryout(t, tenant, model.TryoutPublished)
	product := h.product(t, tenant, "100000", tryout)
	ctx, scope := context.Background(), h.scope(tenant)

	edit := h.input(tenant, tryout)
	edit.Title, edit.Price = product.Title, "120000"
	h.update(t, tenant, product.ID, edit)

	_, _, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
		ProductID: product.ID, Method: model.PaymentWallet, Key: "stale-price-01", ExpectedPrice: "100000.00",
	})
	if !errors.Is(err, ErrPriceChanged) {
		t.Fatalf("stale price: err = %v, want ErrPriceChanged", err)
	}
	if h.ordersOf(t, user.ID) != 0 || h.ledger(t, user.ID) != 1 || h.balance(t, user.ID) != "500000.00" || h.enrollments(t, user.ID) != 0 {
		t.Error("a rejected price still created or debited something")
	}

	order, _, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
		ProductID: product.ID, Method: model.PaymentWallet, Key: "fresh-price-01", ExpectedPrice: "120000",
	})
	if err != nil || order.Price != "120000.00" {
		t.Fatalf("matching price = %+v, %v; want the order at 120000.00", order, err)
	}

	edit.Price = "130000"
	h.update(t, tenant, product.ID, edit)
	replay, replayed, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
		ProductID: product.ID, Method: model.PaymentWallet, Key: "fresh-price-01", ExpectedPrice: "120000",
	})
	if err != nil || !replayed || replay.ID != order.ID {
		t.Errorf("replay after a price change = %v, replayed %v; want the original order", err, replayed)
	}

	for _, bad := range []string{"", "abc", "10.999", "-5"} {
		_, _, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
			ProductID: product.ID, Method: model.PaymentWallet, Key: "bad-price-001", ExpectedPrice: bad,
		})
		wantInputField(t, err, "expected_price")
	}
}

func TestOnlyOnePendingOrderPerProduct(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "500000")
	_, other := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished), h.tryout(t, tenant, model.TryoutPublished))
	ctx, scope, admin := context.Background(), h.scope(tenant), h.staff(tenant, model.RoleAdmin)

	first := h.manual(t, tenant, p, product)
	if first.Status != model.OrderPending {
		t.Fatalf("first order status = %s, want pending", first.Status)
	}
	proof := h.proof(t, tenant, p)
	_, _, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "second-manual-1", ProofAssetID: &proof,
	})
	if !errors.Is(err, ErrOrderPending) {
		t.Errorf("second manual order: err = %v, want ErrOrderPending", err)
	}
	if _, _, err := h.wallet(t, tenant, p, product); !errors.Is(err, ErrOrderPending) {
		t.Errorf("wallet order beside a pending one: err = %v, want ErrOrderPending", err)
	}
	if h.ordersOf(t, user.ID) != 1 || h.balance(t, user.ID) != "500000.00" {
		t.Error("a rejected second order left a row or a debit")
	}
	if got := h.count(t, `SELECT count(*) FROM assets WHERE id = $1 AND status = 'staged'`, proof); got != 1 {
		t.Error("the rejected order consumed its proof")
	}
	if replay, replayed, err := h.svc.Checkout(ctx, scope, p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: keyOf(t, h, first.ID), ProofAssetID: first.ProofAssetID,
	}); err != nil || !replayed || replay.ID != first.ID {
		t.Errorf("replay beside the pending order = %v, replayed %v; want the original", err, replayed)
	}
	if o := h.manual(t, tenant, other, product); o.Status != model.OrderPending {
		t.Errorf("another student's order = %s, want pending (the rule is per student)", o.Status)
	}

	if _, err := h.svc.Cancel(ctx, scope, admin, first.ID, "Bukti tidak sesuai"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	second := h.manual(t, tenant, p, product)
	if _, err := h.svc.Confirm(ctx, scope, admin, second.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, _, err := h.wallet(t, tenant, p, product); err != nil {
		t.Errorf("after confirm: err = %v, want a bundle to be buyable again (no longer pending)", err)
	}
}

func TestConcurrentManualOrdersLeaveOnePending(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	proofs := make([]uuid.UUID, 4)
	for i := range proofs {
		proofs[i] = h.proof(t, tenant, p)
	}

	var wg sync.WaitGroup
	var created, pending atomic.Int32
	for i := range proofs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
				ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual,
				Key: fmt.Sprintf("race-key-%04d", i), ProofAssetID: &proofs[i],
			})
			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, ErrOrderPending):
				pending.Add(1)
			default:
				t.Errorf("checkout %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 || pending.Load() != 3 || h.ordersOf(t, user.ID) != 1 {
		t.Errorf("created %d, order_pending %d, rows %d; want 1, 3, 1", created.Load(), pending.Load(), h.ordersOf(t, user.ID))
	}
}

func TestPendingIndexRejectsASecondPendingRow(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	h.manual(t, tenant, p, product)

	_, err := h.pool.Exec(context.Background(), `
		INSERT INTO orders (tenant_id, user_id, product_id, product_type, product_title, price, payment_method, status, idempotency_key)
		VALUES ($1, $2, $3, 'individual', 'x', 1, 'wallet', 'pending', 'raw-insert-001')`, tenant.ID, user.ID, product.ID)
	if err == nil || !strings.Contains(err.Error(), "orders_pending_product_key") {
		t.Errorf("raw second pending insert: err = %v, want orders_pending_product_key", err)
	}
}

func keyOf(t *testing.T, h orderHarness, orderID uuid.UUID) string {
	t.Helper()
	var key string
	if err := h.pool.QueryRow(context.Background(), `SELECT idempotency_key FROM orders WHERE id = $1`, orderID).Scan(&key); err != nil {
		t.Fatalf("read key: %v", err)
	}
	return key
}
