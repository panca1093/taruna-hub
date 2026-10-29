package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type paymentFixture struct {
	topupFixture
	payments *PaymentRepository
}

func newPaymentFixture(t *testing.T) paymentFixture {
	t.Helper()
	f := paymentFixture{topupFixture: newTopupFixture(t), payments: nil}
	f.payments = NewPaymentRepository(f.pool)
	// Registered after the top-up fixture, so payment rows go before the requests and users they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenant.ID, f.other.ID}
		for _, sql := range []string{
			`DELETE FROM payment_exceptions WHERE tenant_id = ANY($1)`,
			`DELETE FROM payment_events WHERE tenant_id = ANY($1)`,
			`DELETE FROM payment_attempts WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup payment fixture: %v", err)
			}
		}
	})
	return f
}

func (f paymentFixture) gatewayTopup(t *testing.T, tenantID uuid.UUID, user model.User) model.Topup {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	topup, err := f.repo.InsertTx(ctx, tx, tenantID, model.Topup{
		UserID: user.ID, Amount: "50000.00", Method: model.TopupGateway, Key: "key-" + uuid.NewString()[:12],
	})
	if err != nil {
		t.Fatalf("insert gateway top-up: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return topup
}

func (f paymentFixture) attempt(tenantID uuid.UUID, user model.User, topupID uuid.UUID, providerID string) (model.PaymentAttempt, error) {
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		return model.PaymentAttempt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	a, err := f.payments.InsertAttemptTx(ctx, tx, tenantID, model.PaymentAttempt{
		UserID: user.ID, ProviderOrderID: providerID, TopupID: &topupID, Amount: "50000.00", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		return model.PaymentAttempt{}, err
	}
	return a, tx.Commit(ctx)
}

func TestPaymentRepositoryRequiresScope(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()
	if _, err := f.payments.AttemptForTopup(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("AttemptForTopup: %v, want ErrScopeRequired", err)
	}
	if _, err := f.payments.AttemptForOrder(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("AttemptForOrder: %v, want ErrScopeRequired", err)
	}
	if _, _, err := f.payments.ListExceptions(ctx, uuid.Nil, "", 10, 0); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ListExceptions: %v, want ErrScopeRequired", err)
	}
	if _, err := f.payments.FindException(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("FindException: %v, want ErrScopeRequired", err)
	}
	if err := f.payments.SetRedirect(ctx, uuid.Nil, uuid.New(), "x", time.Now()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("SetRedirect: %v, want ErrScopeRequired", err)
	}
}

func TestAttemptReferenceIsGloballyUniqueAndOnePerTarget(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()
	first := f.gatewayTopup(t, f.tenant.ID, f.student)
	second := f.gatewayTopup(t, f.tenant.ID, f.second)
	ref := "TE-" + uuid.NewString()

	a, err := f.attempt(f.tenant.ID, f.student, first.ID, ref)
	if err != nil || a.Status != model.AttemptPending || a.Amount != "50000.00" {
		t.Fatalf("insert = %+v, %v", a, err)
	}
	if _, err := f.attempt(f.tenant.ID, f.second, second.ID, ref); err == nil {
		t.Error("a second attempt reused a provider order id")
	}
	if _, err := f.attempt(f.tenant.ID, f.student, first.ID, "TE-"+uuid.NewString()); err == nil {
		t.Error("a second attempt was opened for the same top-up")
	}
	// The stored reference resolves the tenant; a foreign tenant scope finds nothing.
	got, err := f.payments.AttemptByProviderOrder(ctx, ref)
	if err != nil || got.TenantID != f.tenant.ID || got.UserID != f.student.ID {
		t.Errorf("by reference = %+v, %v", got, err)
	}
	if _, err := f.payments.AttemptForTopup(ctx, f.other.ID, first.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign scope: %v, want ErrNotFound", err)
	}
	if _, err := f.payments.AttemptByProviderOrder(ctx, "TE-missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown reference: %v, want ErrNotFound", err)
	}
}

func TestEventsAreStoredOncePerBodyAndExceptionsOncePerAttempt(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()
	topup := f.gatewayTopup(t, f.tenant.ID, f.student)
	a, err := f.attempt(f.tenant.ID, f.student, topup.ID, "TE-"+uuid.NewString())
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	event := model.PaymentEvent{AttemptID: a.ID, TenantID: a.TenantID, Source: "notification", Status: "settlement", BodyHash: "abc", Raw: []byte(`{"a":1}`)}
	for i := 0; i < 3; i++ {
		if err := f.payments.InsertEventTx(ctx, tx, event, "applied"); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
	event.BodyHash = "def"
	if err := f.payments.InsertEventTx(ctx, tx, event, "ignored"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM payment_events WHERE attempt_id = $1`, a.ID).Scan(&n); err != nil || n != 2 {
		t.Errorf("events = %d, %v; want 2 (one per distinct body)", n, err)
	}
	for i := 0; i < 2; i++ {
		if err := f.payments.InsertExceptionTx(ctx, tx, a.TenantID, a, model.ExceptionLateSuccess, "50000.00"); err != nil {
			t.Fatalf("exception %d: %v", i, err)
		}
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM payment_exceptions WHERE attempt_id = $1`, a.ID).Scan(&n); err != nil || n != 1 {
		t.Errorf("exceptions = %d, %v; want 1", n, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	list, total, err := f.payments.ListExceptions(ctx, f.tenant.ID, model.ExceptionHeld, 10, 0)
	if err != nil || total != 1 || len(list) != 1 || list[0].TopupNumber == nil || list[0].StudentName != f.student.Name {
		t.Fatalf("list = %+v total %d, %v", list, total, err)
	}
	if _, total, _ := f.payments.ListExceptions(ctx, f.other.ID, "", 10, 0); total != 0 {
		t.Error("another tenant listed this tenant's exceptions")
	}
	// A resolution needs an actor and a reason, and only moves a held exception once.
	tx2, _ := f.pool.Begin(ctx)
	defer func() { _ = tx2.Rollback(ctx) }()
	if err := f.payments.ResolveExceptionTx(ctx, tx2, f.tenant.ID, list[0].ID, model.ExceptionRefunded, f.admin.ID, " "); err == nil {
		t.Error("a blank reason resolved the exception")
	}
}
