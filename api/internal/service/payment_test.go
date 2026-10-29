package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const fakeServerKey = "SB-test-server-key"

// fakeGateway is Midtrans in memory: tests set what the Status API says and what a create does.
type fakeGateway struct {
	mu         sync.Mutex
	configured bool
	statuses   map[string]ProviderStatus
	created    map[string]SnapRequest
	createErrs []error
	// createdDespiteErr models a timeout after the provider already created the transaction.
	createdDespiteErr bool
	statusErr         error
	createCalls       int
	statusCalls       int
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{configured: true, statuses: map[string]ProviderStatus{}, created: map[string]SnapRequest{}}
}

func (g *fakeGateway) Configured() bool { return g.configured }

func (g *fakeGateway) CreateTransaction(_ context.Context, req SnapRequest) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.createCalls++
	if len(g.createErrs) > 0 {
		err := g.createErrs[0]
		g.createErrs = g.createErrs[1:]
		if err != nil {
			if g.createdDespiteErr {
				g.created[req.ProviderOrderID] = req
			}
			return "", err
		}
	}
	if _, dup := g.created[req.ProviderOrderID]; dup {
		return "", ErrGatewayDuplicate
	}
	g.created[req.ProviderOrderID] = req
	return "https://pay.example/redirect/" + req.ProviderOrderID, nil
}

func (g *fakeGateway) Status(_ context.Context, id string) (ProviderStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.statusCalls++
	if g.statusErr != nil {
		return ProviderStatus{}, g.statusErr
	}
	st, ok := g.statuses[id]
	if !ok {
		return ProviderStatus{}, ErrGatewayNotFound
	}
	return st, nil
}

func (g *fakeGateway) VerifySignature(n Notification) bool {
	return SignatureMatches(MidtransSignature(n.OrderID, n.StatusCode, n.GrossAmount, fakeServerKey), n.SignatureKey)
}

// set programs what the provider reports for an id.
func (g *fakeGateway) set(id, status, fraud, gross string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.statuses[id] = ProviderStatus{OrderID: id, TransactionID: "trx-" + id, StatusCode: "200", GrossAmount: gross,
		TransactionStatus: status, FraudStatus: fraud, Raw: []byte(`{"order_id":"` + id + `","transaction_status":"` + status + `"}`)}
}

func (g *fakeGateway) firstCreated() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	for id := range g.created {
		return id
	}
	return ""
}

func signedNotification(id, status, fraud, gross string) (Notification, []byte) {
	n := Notification{
		ProviderStatus: ProviderStatus{OrderID: id, StatusCode: "200", GrossAmount: gross, TransactionStatus: status, FraudStatus: fraud},
		SignatureKey:   MidtransSignature(id, "200", gross, fakeServerKey),
	}
	raw, _ := json.Marshal(map[string]string{"order_id": id, "transaction_status": status, "fraud_status": fraud, "gross_amount": gross, "signature_key": n.SignatureKey})
	return n, raw
}

type paymentHarness struct {
	orderHarness
	topups *TopupService
	pay    *PaymentService
	gw     *fakeGateway
}

func newPaymentHarness(t *testing.T) paymentHarness {
	t.Helper()
	h := paymentHarness{orderHarness: newOrderHarness(t, noJobs{}, testBank), gw: newFakeGateway()}
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil,
		bytes.Repeat([]byte{5}, SealKeyBytes), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.topups = NewTopupService(h.pool, repository.NewTopupRepository(h.pool), h.wallets, h.assets, notes)
	h.pay = NewPaymentService(h.pool, repository.NewPaymentRepository(h.pool), h.svc, h.topups, h.wallets, h.gw,
		"https://app.example", WithPaymentClock(h.clock.Now))
	// Registered after the order fixture, so payment rows go before the orders and users they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{h.t.q.a.ID, h.t.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM payment_exceptions WHERE tenant_id = ANY($1)`,
			`DELETE FROM payment_events WHERE tenant_id = ANY($1)`,
			`DELETE FROM payment_attempts WHERE tenant_id = ANY($1)`,
			`DELETE FROM topup_requests WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup payment fixture: %v", err)
			}
		}
	})
	return h
}

func (h paymentHarness) checkout(tenant model.Tenant, p model.Principal, product model.Product, key string) (model.Order, bool, error) {
	if key == "" {
		key = "key-" + uuid.NewString()[:12]
	}
	return h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentGateway, Key: key,
	})
}

func (h paymentHarness) providerID(t *testing.T, orderID uuid.UUID) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(), `SELECT provider_order_id FROM payment_attempts WHERE order_id = $1`, orderID).Scan(&id); err != nil {
		t.Fatalf("read provider order id: %v", err)
	}
	return id
}

func (h paymentHarness) topupProviderID(t *testing.T, topupID uuid.UUID) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(), `SELECT provider_order_id FROM payment_attempts WHERE topup_id = $1`, topupID).Scan(&id); err != nil {
		t.Fatalf("read provider order id: %v", err)
	}
	return id
}

// notify delivers a signed notification the way the webhook does.
func (h paymentHarness) notify(id, status, fraud, gross string) error {
	n, raw := signedNotification(id, status, fraud, gross)
	return h.pay.HandleNotification(context.Background(), n, raw)
}

func (h paymentHarness) order(t *testing.T, tenant model.Tenant, id uuid.UUID) model.Order {
	t.Helper()
	o, err := h.svc.repo.Find(context.Background(), tenant.ID, id)
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	return o
}

func (h paymentHarness) attemptStatus(t *testing.T, providerID string) string {
	t.Helper()
	var status string
	if err := h.pool.QueryRow(context.Background(), `SELECT status FROM payment_attempts WHERE provider_order_id = $1`, providerID).Scan(&status); err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	return status
}

func gatewayProduct(h paymentHarness, t *testing.T, tenant model.Tenant) model.Product {
	return h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
}

func TestGatewayOrderIsPendingWithARedirectUntilTheProviderConfirms(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)

	order, replayed, err := h.checkout(tenant, p, product, "")
	if err != nil || replayed {
		t.Fatalf("gateway checkout = %v, replayed %v", err, replayed)
	}
	if order.Status != model.OrderPending || order.PaymentMethod != model.PaymentGateway || order.RedirectURL == nil {
		t.Fatalf("order = %+v, want pending with the provider page", order)
	}
	id := h.providerID(t, order.ID)
	req := h.gw.created[id]
	if req.Amount != "100000.00" || req.FinishURL != "https://app.example/student/orders" || req.ProviderOrderID != id {
		t.Errorf("snap request = %+v", req)
	}
	if h.enrollments(t, user.ID) != 0 || h.ledger(t, user.ID) != 0 {
		t.Fatal("a created gateway order opened access or moved money")
	}
	// Reading the order with the provider still pending changes nothing.
	h.gw.set(id, "pending", "", "100000.00")
	got, err := h.svc.Order(context.Background(), h.scope(tenant), p, order.ID)
	if err != nil || got.Status != model.OrderPending {
		t.Fatalf("pending read = %+v, %v", got, err)
	}
}

func TestReplayAndReorderedNotificationsGrantOnce(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	order, _, err := h.checkout(tenant, p, product, "")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	id := h.providerID(t, order.ID)

	// A late "pending" notification arriving after the provider settled must not undo anything.
	h.gw.set(id, "settlement", "", "100000.00")
	if err := h.notify(id, "pending", "", "100000.00"); err != nil {
		t.Fatalf("reordered pending notification: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := h.notify(id, "settlement", "", "100000.00"); err != nil {
			t.Fatalf("settlement replay %d: %v", i, err)
		}
	}
	if err := h.notify(id, "pending", "", "100000.00"); err != nil {
		t.Fatalf("pending after success: %v", err)
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderConfirmed || got.ReviewedBy != nil || got.RedirectURL != nil {
		t.Errorf("order = %+v, want confirmed by the provider with no open page", got)
	}
	if h.enrollments(t, user.ID) != 1 || h.grants(t, user.ID) != 1 {
		t.Errorf("enrollments %d grants %d, want 1 and 1", h.enrollments(t, user.ID), h.grants(t, user.ID))
	}
	if got := h.notifications(t, tenant, TemplateOrderConfirmed); got != 2 {
		t.Errorf("confirmation notifications = %d, want 2", got)
	}
	if h.attemptStatus(t, id) != model.AttemptPaid || h.ledger(t, user.ID) != 0 {
		t.Errorf("attempt %s, ledger %d; a gateway order moves no wallet money", h.attemptStatus(t, id), h.ledger(t, user.ID))
	}
	if n := h.count(t, `SELECT count(*) FROM payment_events WHERE provider_transaction_id <> '' OR source = 'notification'`); n < 2 {
		t.Errorf("events = %d, want the evidence kept", n)
	}
	// Identical replays are stored once.
	if n := h.count(t, `SELECT count(*) FROM payment_events e JOIN payment_attempts a ON a.id = e.attempt_id
		 WHERE a.provider_order_id = $1 AND e.source = 'notification' AND e.transaction_status = 'settlement'`, id); n != 1 {
		t.Errorf("settlement notification rows = %d, want 1", n)
	}
}

func TestNotificationThatTheProviderDoesNotConfirmMovesNothing(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)

	// Wrong signature.
	n, raw := signedNotification(id, "settlement", "", "100000.00")
	n.SignatureKey = "deadbeef"
	if err := h.pay.HandleNotification(context.Background(), n, raw); !errors.Is(err, ErrPaymentSignature) {
		t.Errorf("bad signature = %v, want ErrPaymentSignature", err)
	}
	// A signed notification for a reference we never issued.
	if err := h.notify("TE-unknown", "settlement", "", "100000.00"); !errors.Is(err, ErrPaymentUnknown) {
		t.Errorf("unknown reference = %v, want ErrPaymentUnknown", err)
	}
	// The notification claims success but the provider says pending.
	h.gw.set(id, "pending", "", "100000.00")
	if err := h.notify(id, "settlement", "", "100000.00"); err != nil {
		t.Fatalf("unconfirmed claim: %v", err)
	}
	// The provider cannot be reached: the webhook must fail so the provider retries.
	h.gw.statusErr = ErrGatewayAmbiguous
	if err := h.notify(id, "settlement", "", "100000.00"); err == nil {
		t.Error("unconfirmable notification was acknowledged, want an error")
	}
	h.gw.statusErr = nil
	// Fraud deny on a captured card.
	h.gw.set(id, "capture", "deny", "100000.00")
	if err := h.notify(id, "capture", "deny", "100000.00"); err != nil {
		t.Fatalf("fraud deny: %v", err)
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderPending || h.enrollments(t, user.ID) != 0 {
		t.Fatalf("order %s with %d enrollments after refused notifications", got.Status, h.enrollments(t, user.ID))
	}
	if h.attemptStatus(t, id) != model.AttemptFailed {
		t.Errorf("attempt = %s, want failed", h.attemptStatus(t, id))
	}
	// A failed attempt can still be paid for: the payer may retry inside the payment page.
	h.gw.set(id, "settlement", "", "100000.00")
	if err := h.notify(id, "settlement", "", "100000.00"); err != nil {
		t.Fatalf("settlement after deny: %v", err)
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderConfirmed {
		t.Errorf("order = %s, want confirmed", got.Status)
	}
}

func TestWrongAmountIsHeldNotConfirmed(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)

	h.gw.set(id, "settlement", "", "90000.00")
	if err := h.notify(id, "settlement", "", "90000.00"); err != nil {
		t.Fatalf("mismatched settlement: %v", err)
	}
	got := h.order(t, tenant, order.ID)
	if got.Status != model.OrderPending || h.enrollments(t, user.ID) != 0 || got.ExceptionKind == nil || *got.ExceptionKind != model.ExceptionAmountMismatch {
		t.Fatalf("order = %+v; want pending, no access and a held amount_mismatch", got)
	}
	var paid string
	if err := h.pool.QueryRow(context.Background(), `SELECT paid_amount::text FROM payment_exceptions WHERE order_id = $1`, order.ID).Scan(&paid); err != nil || paid != "90000.00" {
		t.Errorf("held amount = %s, %v; want what the provider reports", paid, err)
	}
}

func TestLateSuccessOnACancelledOrderIsHeldAndStaffCreditOnce(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)
	admin := h.staff(tenant, model.RoleAdmin)
	ctx := context.Background()

	if _, err := h.svc.Cancel(ctx, h.scope(tenant), admin, order.ID, "salah paket"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	h.gw.set(id, "settlement", "", "100000.00")
	for i := 0; i < 3; i++ {
		if err := h.notify(id, "settlement", "", "100000.00"); err != nil {
			t.Fatalf("late settlement %d: %v", i, err)
		}
	}
	got := h.order(t, tenant, order.ID)
	if got.Status != model.OrderCancelled || h.enrollments(t, user.ID) != 0 || got.ExceptionID == nil {
		t.Fatalf("order = %+v; want cancelled, no access, one held exception", got)
	}
	if h.count(t, `SELECT count(*) FROM payment_exceptions WHERE order_id = $1`, order.ID) != 1 || h.attemptStatus(t, id) != model.AttemptHeld {
		t.Fatal("want exactly one exception and a held attempt")
	}

	for name, in := range map[string]model.Principal{"student": p} {
		if _, err := h.pay.Credit(ctx, h.scope(tenant), in, *got.ExceptionID, "x"); !errors.Is(err, ErrExceptionForbidden) {
			t.Errorf("%s credit = %v, want forbidden", name, err)
		}
	}
	if _, err := h.pay.Credit(ctx, h.scope(tenant), admin, *got.ExceptionID, "  "); err == nil {
		t.Error("credit without a reason succeeded")
	}

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		role := model.RoleAdmin
		if i%2 == 1 {
			role = model.RoleOperator
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := h.pay.Credit(ctx, h.scope(tenant), h.staff(tenant, role), *got.ExceptionID, "pembayaran terlambat"); err != nil {
				t.Errorf("concurrent credit: %v", err)
			}
		}()
	}
	wg.Wait()
	if h.ledger(t, user.ID) != 1 || h.balance(t, user.ID) != "100000.00" {
		t.Errorf("ledger %d balance %s, want one credit of the paid amount", h.ledger(t, user.ID), h.balance(t, user.ID))
	}
	var kind, actor, source string
	if err := h.pool.QueryRow(ctx, `SELECT kind, actor_kind, source_type FROM wallet_entries WHERE user_id = $1`, user.ID).Scan(&kind, &actor, &source); err != nil ||
		kind != model.KindTopup || actor != model.ActorStaff || source != model.WalletSourceException {
		t.Errorf("entry = %s %s %s, %v", kind, actor, source, err)
	}
	if _, err := h.pay.MarkRefunded(ctx, h.scope(tenant), admin, *got.ExceptionID, "sudah"); !errors.Is(err, ErrExceptionState) {
		t.Errorf("refund after credit = %v, want ErrExceptionState", err)
	}
	if h.order(t, tenant, order.ID).ExceptionID != nil {
		t.Error("a resolved exception still flags the order")
	}
	if h.enrollments(t, user.ID) != 0 {
		t.Error("crediting the wallet reopened access")
	}
}

func TestStaffCanMarkAHeldPaymentRefunded(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	id := h.providerID(t, order.ID)
	op := h.staff(tenant, model.RoleOperator)
	ctx := context.Background()
	h.gw.set(id, "settlement", "", "50000.00")
	_ = h.notify(id, "settlement", "", "50000.00")
	x := h.order(t, tenant, order.ID).ExceptionID
	if x == nil {
		t.Fatal("no exception held")
	}
	if _, err := h.pay.MarkRefunded(ctx, h.scope(tenant), op, *x, ""); err == nil {
		t.Error("refund without a reason succeeded")
	}
	done, err := h.pay.MarkRefunded(ctx, h.scope(tenant), op, *x, "refund manual via dashboard")
	if err != nil || done.Status != model.ExceptionRefunded || done.ResolvedBy == nil || *done.ResolvedBy != op.UserID {
		t.Fatalf("refund = %+v, %v", done, err)
	}
	if again, err := h.pay.MarkRefunded(ctx, h.scope(tenant), op, *x, "lagi"); err != nil || again.ResolveReason != "refund manual via dashboard" {
		t.Errorf("repeat refund = %+v, %v; want the first resolution unchanged", again, err)
	}
	if _, err := h.pay.Credit(ctx, h.scope(tenant), op, *x, "x"); !errors.Is(err, ErrExceptionState) {
		t.Errorf("credit after refund = %v, want ErrExceptionState", err)
	}
	if h.ledger(t, user.ID) != 0 {
		t.Error("marking refunded moved wallet money")
	}
}

func TestExpiredPaymentCancelsTheOrderAndALaterSuccessIsHeld(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)

	h.gw.set(id, "expire", "", "100000.00")
	if err := h.notify(id, "expire", "", "100000.00"); err != nil {
		t.Fatalf("expire: %v", err)
	}
	got := h.order(t, tenant, order.ID)
	if got.Status != model.OrderCancelled || got.CancelReason == "" || h.notifications(t, tenant, TemplateOrderCancelled) != 2 {
		t.Fatalf("order = %+v; want cancelled with a reason and notified", got)
	}
	// The one-pending rule is released: the student can buy again.
	if _, _, err := h.checkout(tenant, p, product, ""); err != nil {
		t.Errorf("repurchase after expiry: %v", err)
	}
	h.gw.set(id, "settlement", "", "100000.00")
	_ = h.notify(id, "settlement", "", "100000.00")
	if got := h.order(t, tenant, order.ID); got.ExceptionKind == nil || *got.ExceptionKind != model.ExceptionLateSuccess || h.enrollments(t, user.ID) != 0 {
		t.Errorf("order = %+v; want a held late_success and no access", got)
	}
}

func TestRefundAndChargebackAreEvidenceOnly(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	id := h.providerID(t, order.ID)
	h.gw.set(id, "settlement", "", "100000.00")
	_ = h.notify(id, "settlement", "", "100000.00")
	for _, status := range []string{"refund", "chargeback"} {
		h.gw.set(id, status, "", "100000.00")
		if err := h.notify(id, status, "", "100000.00"); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderConfirmed || h.enrollments(t, user.ID) != 1 {
		t.Errorf("order %s enrollments %d; evidence must not change state", got.Status, h.enrollments(t, user.ID))
	}
	if n := h.count(t, `SELECT count(*) FROM payment_events WHERE outcome = 'evidence'`); n < 2 {
		t.Errorf("evidence rows = %d, want the refund and chargeback kept", n)
	}
}

func TestAmbiguousCreateNeverMakesASecondPayment(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)

	// The first call times out before the answer arrives; it did not reach the provider.
	h.gw.createErrs = []error{ErrGatewayAmbiguous}
	_, _, err := h.checkout(tenant, p, product, "retry-key-aaaa")
	if !errors.Is(err, ErrGatewayAmbiguous) {
		t.Fatalf("first checkout = %v, want ErrGatewayAmbiguous", err)
	}
	if h.ordersOf(t, user.ID) != 1 || h.count(t, `SELECT count(*) FROM payment_attempts WHERE user_id = $1`, user.ID) != 1 {
		t.Fatal("want one pending order and one attempt after the ambiguous create")
	}
	order, replayed, err := h.checkout(tenant, p, product, "retry-key-aaaa")
	if err != nil || !replayed || order.RedirectURL == nil {
		t.Fatalf("retry = %+v, replayed %v, %v; want the same order with its page", order, replayed, err)
	}
	if len(h.gw.created) != 1 || h.gw.firstCreated() != h.providerID(t, order.ID) {
		t.Errorf("provider transactions = %d, want one under the stored order id", len(h.gw.created))
	}
	// A repeat after success opens nothing new.
	before := h.gw.createCalls
	if _, _, err := h.checkout(tenant, p, product, "retry-key-aaaa"); err != nil || h.gw.createCalls != before {
		t.Errorf("third call: err %v, create calls %d -> %d", err, before, h.gw.createCalls)
	}
}

func TestAmbiguousCreateThatReachedTheProviderCancelsInsteadOfDuplicating(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)

	h.gw.createErrs, h.gw.createdDespiteErr = []error{ErrGatewayAmbiguous}, true
	if _, _, err := h.checkout(tenant, p, product, "retry-key-bbbb"); !errors.Is(err, ErrGatewayAmbiguous) {
		t.Fatalf("first checkout = %v", err)
	}
	// The retry reuses the order id; the provider says it exists and has no payment, and the payer never saw its page.
	if _, _, err := h.checkout(tenant, p, product, "retry-key-bbbb"); !errors.Is(err, ErrPaymentCreateFailed) {
		t.Fatalf("retry = %v, want ErrPaymentCreateFailed", err)
	}
	if len(h.gw.created) != 1 || h.ordersOf(t, user.ID) != 1 {
		t.Errorf("provider transactions %d, orders %d; want 1 and 1", len(h.gw.created), h.ordersOf(t, user.ID))
	}
	if got := h.count(t, `SELECT count(*) FROM orders WHERE user_id = $1 AND status = 'cancelled'`, user.ID); got != 1 {
		t.Error("the unusable order stayed pending")
	}
	// A fresh key starts a clean purchase.
	if _, _, err := h.checkout(tenant, p, product, ""); err != nil {
		t.Errorf("fresh checkout: %v", err)
	}
}

func TestRejectedCreateCancelsTheOrder(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	h.gw.createErrs = []error{ErrGatewayRejected}
	if _, _, err := h.checkout(tenant, p, gatewayProduct(h, t, tenant), ""); !errors.Is(err, ErrPaymentCreateFailed) {
		t.Fatalf("checkout = %v, want ErrPaymentCreateFailed", err)
	}
	if h.count(t, `SELECT count(*) FROM orders WHERE user_id = $1 AND status = 'cancelled'`, user.ID) != 1 {
		t.Error("a refused create left the order pending")
	}
}

func TestGatewayUnavailableAndPriceRules(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	h.gw.configured = false
	if _, _, err := h.checkout(tenant, p, product, ""); !errors.Is(err, ErrGatewayUnavailable) {
		t.Errorf("unconfigured checkout = %v, want ErrGatewayUnavailable", err)
	}
	proof := (*uuid.UUID)(nil)
	if _, _, err := h.topups.Create(context.Background(), h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0001", ProofAssetID: proof}); !errors.Is(err, ErrGatewayUnavailable) {
		t.Errorf("unconfigured top-up = %v, want ErrGatewayUnavailable", err)
	}
	h.gw.configured = true
	cents := h.product(t, tenant, "100000.50", h.tryout(t, tenant, model.TryoutPublished))
	_, _, err := h.checkout(tenant, p, cents, "")
	wantInputField(t, err, "payment_method")
	asset := h.proof(t, tenant, p)
	_, _, err = h.svc.Checkout(context.Background(), h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentGateway, Key: "proof-key-0001", ProofAssetID: &asset,
	})
	wantInputField(t, err, "proof_asset_id")
}

func TestGatewayTopupCreditsOnce(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	ctx := context.Background()

	topup, replayed, err := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0002"})
	if err != nil || replayed || topup.Method != model.TopupGateway || topup.Status != model.TopupPending || topup.RedirectURL == nil {
		t.Fatalf("create = %+v, %v, %v", topup, replayed, err)
	}
	id := h.topupProviderID(t, topup.ID)
	if h.ledger(t, user.ID) != 0 {
		t.Fatal("a created top-up credited the wallet")
	}
	h.gw.set(id, "settlement", "", "50000.00")
	for i := 0; i < 3; i++ {
		if err := h.notify(id, "settlement", "", "50000.00"); err != nil {
			t.Fatalf("settlement %d: %v", i, err)
		}
	}
	if err := h.notify(id, "pending", "", "50000.00"); err != nil {
		t.Fatalf("pending after success: %v", err)
	}
	if h.ledger(t, user.ID) != 1 || h.balance(t, user.ID) != "50000.00" {
		t.Errorf("ledger %d balance %s, want one credit of 50000.00", h.ledger(t, user.ID), h.balance(t, user.ID))
	}
	var kind, actor, source, sourceID string
	if err := h.pool.QueryRow(ctx, `SELECT kind, actor_kind, source_type, source_id FROM wallet_entries WHERE user_id = $1`, user.ID).Scan(&kind, &actor, &source, &sourceID); err != nil ||
		kind != model.KindTopup || actor != model.ActorProvider || source != model.WalletSourceTopup || sourceID != topup.ID.String() {
		t.Errorf("entry = %s %s %s %s, %v", kind, actor, source, sourceID, err)
	}
	got, err := h.topups.Topup(ctx, h.scope(tenant), p, topup.ID)
	if err != nil || got.Status != model.TopupApproved || got.ReviewedBy != nil || got.RedirectURL != nil {
		t.Errorf("top-up = %+v, %v; want approved by the provider", got, err)
	}
	if h.notifications(t, tenant, TemplateTopupApproved) != 2 {
		t.Error("want the approval email and WhatsApp once")
	}
	// Same key replays the same request.
	again, replayed, err := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0002"})
	if err != nil || !replayed || again.ID != topup.ID {
		t.Errorf("replay = %+v, %v, %v", again, replayed, err)
	}
	// A different method under the same key is a conflict.
	if _, _, err := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "60000", Key: "topup-key-0002"}); !errors.Is(err, ErrTopupKeyConflict) {
		t.Errorf("different amount under one key = %v, want ErrTopupKeyConflict", err)
	}
}

func TestGatewayTopupBelowMinimumAndExpiry(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	ctx := context.Background()
	_, _, err := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "5000", Key: "topup-key-0003"})
	wantInputField(t, err, "amount")

	topup, _, err := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "20000", Key: "topup-key-0004"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := h.topupProviderID(t, topup.ID)
	h.gw.set(id, "expire", "", "20000.00")
	_ = h.notify(id, "expire", "", "20000.00")
	got, _ := h.topups.Topup(ctx, h.scope(tenant), p, topup.ID)
	if got.Status != model.TopupRejected || got.RejectReason == "" || h.ledger(t, user.ID) != 0 {
		t.Errorf("top-up = %+v; want rejected with a reason and no credit", got)
	}
	h.gw.set(id, "settlement", "", "20000.00")
	_ = h.notify(id, "settlement", "", "20000.00")
	if h.ledger(t, user.ID) != 0 || h.count(t, `SELECT count(*) FROM payment_exceptions WHERE topup_id = $1`, topup.ID) != 1 {
		t.Error("a success after rejection must be held, not credited")
	}
}

func TestReadingAPendingOrderReconcilesWithTheProvider(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	id := h.providerID(t, order.ID)
	ctx := context.Background()

	// The webhook never arrived; the provider reports success when the student looks.
	h.gw.set(id, "settlement", "", "100000.00")
	got, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID)
	if err != nil || got.Status != model.OrderConfirmed || h.enrollments(t, user.ID) != 1 {
		t.Fatalf("read = %+v, %v; want confirmed through the Status API", got, err)
	}
	calls := h.gw.statusCalls
	if _, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID); err != nil || h.gw.statusCalls != calls {
		t.Errorf("a settled order was checked again (%d -> %d)", calls, h.gw.statusCalls)
	}
}

func TestReconcileThrottlesAndExpiresOverdueAttempts(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	ctx := context.Background()

	// Not found at the provider, not yet overdue: stay pending, and a quick second read does not call again.
	if _, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID); err != nil {
		t.Fatal(err)
	}
	calls := h.gw.statusCalls
	if _, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID); err != nil || h.gw.statusCalls != calls {
		t.Errorf("status calls %d -> %d within the throttle window", calls, h.gw.statusCalls)
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderPending {
		t.Fatalf("order = %s before expiry", got.Status)
	}
	h.clock.Advance(25 * time.Hour)
	got, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID)
	if err != nil || got.Status != model.OrderCancelled {
		t.Errorf("overdue read = %+v, %v; want cancelled", got, err)
	}
}
