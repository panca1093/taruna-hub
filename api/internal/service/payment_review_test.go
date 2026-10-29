package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func TestStaffCannotConfirmOrApproveAGatewayPayment(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	ctx, admin := context.Background(), h.staff(tenant, model.RoleAdmin)
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	if _, err := h.svc.Confirm(ctx, h.scope(tenant), admin, order.ID); !errors.Is(err, ErrGatewayManaged) {
		t.Errorf("confirm gateway order = %v, want ErrGatewayManaged", err)
	}
	topup, _, _ := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0010"})
	if _, err := h.topups.Approve(ctx, h.scope(tenant), admin, topup.ID); !errors.Is(err, ErrGatewayManaged) {
		t.Errorf("approve gateway top-up = %v, want ErrGatewayManaged", err)
	}
	if got := h.order(t, tenant, order.ID); got.Status != model.OrderPending {
		t.Errorf("order = %s after a refused confirm", got.Status)
	}
	// Cancelling and rejecting stay allowed.
	if _, err := h.svc.Cancel(ctx, h.scope(tenant), admin, order.ID, "tidak jadi"); err != nil {
		t.Errorf("cancel: %v", err)
	}
	if _, err := h.topups.Reject(ctx, h.scope(tenant), admin, topup.ID, "tidak jadi"); err != nil {
		t.Errorf("reject: %v", err)
	}
}

func TestResolvingAHeldPaymentClosesTheLinkedPendingRow(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	ctx, admin := context.Background(), h.staff(tenant, model.RoleAdmin)
	product := gatewayProduct(h, t, tenant)

	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)
	h.gw.set(id, "settlement", "", "90000.00")
	_ = h.notify(id, "settlement", "", "90000.00")
	x := h.order(t, tenant, order.ID).ExceptionID
	if x == nil || h.order(t, tenant, order.ID).Status != model.OrderPending {
		t.Fatal("want a pending order with a held exception")
	}
	if _, err := h.pay.Credit(ctx, h.scope(tenant), admin, *x, "jumlah salah, kredit saldo"); err != nil {
		t.Fatalf("credit: %v", err)
	}
	got := h.order(t, tenant, order.ID)
	if got.Status != model.OrderCancelled || got.CancelReason != "jumlah salah, kredit saldo" || got.ReviewedBy == nil {
		t.Errorf("order = %+v; want cancelled with the staff reason", got)
	}
	if h.notifications(t, tenant, TemplateOrderCancelled) != 2 || h.balance(t, user.ID) != "90000.00" {
		t.Error("want the cancellation notices and the credit")
	}
	if _, _, err := h.checkout(tenant, p, product, ""); err != nil {
		t.Errorf("repurchase after resolution: %v", err)
	}

	topup, _, _ := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0011"})
	tid := h.topupProviderID(t, topup.ID)
	h.gw.set(tid, "settlement", "", "40000.00")
	_ = h.notify(tid, "settlement", "", "40000.00")
	var xid string
	if err := h.pool.QueryRow(ctx, `SELECT id::text FROM payment_exceptions WHERE topup_id = $1`, topup.ID).Scan(&xid); err != nil {
		t.Fatal(err)
	}
	parsed := mustUUID(t, xid)
	if _, err := h.pay.MarkRefunded(ctx, h.scope(tenant), admin, parsed, "refund manual"); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if got, _ := h.topups.Topup(ctx, h.scope(tenant), admin, topup.ID); got.Status != model.TopupRejected || got.RejectReason != "refund manual" {
		t.Errorf("top-up = %+v; want rejected with the staff reason", got)
	}
}

func TestStudentCanAbandonAnUnpaidGatewayPayment(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	_, other := h.student(t, tenant, "")
	ctx := context.Background()
	product := gatewayProduct(h, t, tenant)

	order, _, _ := h.checkout(tenant, p, product, "")
	id := h.providerID(t, order.ID)
	if _, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), other, order.ID); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("foreign cancel = %v, want ErrOrderNotFound", err)
	}
	if _, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), h.staff(tenant, model.RoleAdmin), order.ID); err == nil {
		t.Error("staff used the student cancel")
	}
	h.gw.set(id, "pending", "", "100000.00")
	got, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), p, order.ID)
	if err != nil || got.Status != model.OrderCancelled || got.CancelReason != studentCancelReason {
		t.Fatalf("cancel = %+v, %v", got, err)
	}
	if again, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), p, order.ID); err != nil || again.Status != model.OrderCancelled {
		t.Errorf("repeat cancel = %+v, %v; want unchanged", again, err)
	}
	// If the payment later settles anyway, staff review it.
	h.gw.set(id, "settlement", "", "100000.00")
	_ = h.notify(id, "settlement", "", "100000.00")
	if x := h.order(t, tenant, order.ID); x.ExceptionKind == nil || *x.ExceptionKind != model.ExceptionLateSuccess || h.enrollments(t, user.ID) != 0 {
		t.Errorf("order = %+v; want a held late_success", x)
	}

	// A payment that already succeeded confirms instead of cancelling.
	paid, _, _ := h.checkout(tenant, p, product, "")
	pid := h.providerID(t, paid.ID)
	h.gw.set(pid, "settlement", "", "100000.00")
	if _, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), p, paid.ID); !errors.Is(err, ErrOrderState) {
		t.Errorf("cancel of a paid order = %v, want ErrOrderState", err)
	}
	if h.order(t, tenant, paid.ID).Status != model.OrderConfirmed {
		t.Error("the settled payment was not confirmed")
	}
	// Not a gateway order.
	_, funded := h.student(t, tenant, "500000")
	wallet, _, _ := h.wallet(t, tenant, funded, gatewayProduct(h, t, tenant))
	if _, err := h.pay.CancelOwnOrder(ctx, h.scope(tenant), funded, wallet.ID); !errors.Is(err, ErrOrderState) {
		t.Errorf("cancel of a wallet order = %v, want ErrOrderState", err)
	}

	topup, _, _ := h.topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Method: model.TopupGateway, Amount: "50000", Key: "topup-key-0012"})
	cancelled, err := h.pay.CancelOwnTopup(ctx, h.scope(tenant), p, topup.ID)
	if err != nil || cancelled.Status != model.TopupRejected || cancelled.RedirectURL != nil {
		t.Errorf("cancel top-up = %+v, %v", cancelled, err)
	}
	if _, err := h.pay.CancelOwnTopup(ctx, h.scope(tenant), other, topup.ID); !errors.Is(err, ErrTopupNotFound) {
		t.Errorf("foreign top-up cancel = %v, want ErrTopupNotFound", err)
	}
}

func TestSweepSettlesOverdueAttemptsWithoutAReader(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	_, p2 := h.student(t, tenant, "")
	ctx := context.Background()
	lost, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	paid, _, _ := h.checkout(tenant, p2, gatewayProduct(h, t, tenant), "")
	h.gw.set(h.providerID(t, paid.ID), "settlement", "", "100000.00")

	if n, err := h.pay.SweepExpired(ctx, 10); err != nil || n != 0 {
		t.Fatalf("early sweep = %d, %v; want nothing due", n, err)
	}
	h.clock.Advance(25 * time.Hour)
	if n, err := h.pay.SweepExpired(ctx, 10); err != nil || n != 2 {
		t.Fatalf("sweep = %d, %v; want 2 looked at", n, err)
	}
	if got := h.order(t, tenant, lost.ID); got.Status != model.OrderCancelled {
		t.Errorf("unknown overdue order = %s, want cancelled", got.Status)
	}
	if got := h.order(t, tenant, paid.ID); got.Status != model.OrderConfirmed {
		t.Errorf("settled overdue order = %s, want confirmed", got.Status)
	}
	if n, _ := h.pay.SweepExpired(ctx, 10); n != 0 || h.enrollments(t, user.ID) != 0 {
		t.Errorf("second sweep looked at %d; the cancelled order opened access", n)
	}
}

func TestExpiryRestartsWhenThePageIsCreated(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	h.gw.createErrs = []error{ErrGatewayAmbiguous}
	_, _, _ = h.checkout(tenant, p, product, "retry-key-cccc")
	h.clock.Advance(2 * time.Hour)
	order, _, err := h.checkout(tenant, p, product, "retry-key-cccc")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	var expires time.Time
	if err := h.pool.QueryRow(context.Background(), `SELECT expires_at FROM payment_attempts WHERE order_id = $1`, order.ID).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if want := h.clock.Now().Add(gatewayExpiryMinutes * time.Minute); !expires.Equal(want) {
		t.Errorf("expires_at = %s, want %s (24h from the page)", expires, want)
	}
}

func TestCaptureWithoutFraudStatusIsASuccess(t *testing.T) {
	cases := []struct {
		status, fraud, want string
	}{
		{"capture", "", kindSuccess},
		{"capture", "accept", kindSuccess},
		{"capture", "challenge", kindPending},
		{"capture", "deny", kindFailed},
		{"settlement", "", kindSuccess},
	}
	for _, c := range cases {
		if got := classifyProvider(ProviderStatus{TransactionStatus: c.status, FraudStatus: c.fraud}); got != c.want {
			t.Errorf("classify(%s, %q) = %s, want %s", c.status, c.fraud, got, c.want)
		}
	}
}

func TestDuplicateCreateWithALivePaymentCancelsInsteadOfStrandingTheOrder(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	user, p := h.student(t, tenant, "")
	product := gatewayProduct(h, t, tenant)
	h.gw.createErrs, h.gw.createdDespiteErr = []error{ErrGatewayAmbiguous}, true
	_, _, _ = h.checkout(tenant, p, product, "retry-key-dddd")
	h.gw.set(h.gw.firstCreated(), "pending", "", "100000.00")
	if _, _, err := h.checkout(tenant, p, product, "retry-key-dddd"); !errors.Is(err, ErrPaymentCreateFailed) {
		t.Fatalf("retry = %v, want ErrPaymentCreateFailed", err)
	}
	if h.count(t, `SELECT count(*) FROM orders WHERE user_id = $1 AND status = 'pending'`, user.ID) != 0 {
		t.Error("the order was left pending with no payment page")
	}
}

func TestStatusFailureDoesNotThrottleTheNextRead(t *testing.T) {
	h := newPaymentHarness(t)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	order, _, _ := h.checkout(tenant, p, gatewayProduct(h, t, tenant), "")
	ctx := context.Background()
	h.gw.statusErr = ErrGatewayAmbiguous
	_, _ = h.svc.Order(ctx, h.scope(tenant), p, order.ID)
	h.gw.statusErr = nil
	h.gw.set(h.providerID(t, order.ID), "settlement", "", "100000.00")
	if got, err := h.svc.Order(ctx, h.scope(tenant), p, order.ID); err != nil || got.Status != model.OrderConfirmed {
		t.Errorf("read after an outage = %+v, %v; want confirmed at once", got, err)
	}
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
