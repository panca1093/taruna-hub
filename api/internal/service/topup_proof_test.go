package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func newTopupOverOrders(t *testing.T, h orderHarness) *TopupService {
	t.Helper()
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil,
		bytes.Repeat([]byte{5}, SealKeyBytes), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	// Registered after the order harness, so it runs first and frees the proofs before they are deleted.
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM topup_requests WHERE tenant_id = $1`, h.t.q.a.ID); err != nil {
			t.Errorf("cleanup top-ups: %v", err)
		}
	})
	return NewTopupService(h.pool, repository.NewTopupRepository(h.pool), h.wallets, h.assets, notes)
}

func TestProofBacksEitherAnOrderOrATopupNotBoth(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	topups := newTopupOverOrders(t, h)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	ctx := context.Background()

	// Top-up first, then an order on the same proof.
	proof := h.proof(t, tenant, p)
	if _, _, err := topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Amount: "50000", Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof}); err != nil {
		t.Fatalf("top-up: %v", err)
	}
	_, _, err := h.svc.Checkout(ctx, h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof,
	})
	wantInputField(t, err, "proof_asset_id")
	if h.ordersOf(t, p.UserID) != 0 {
		t.Error("an order was written on a proof that backs a top-up")
	}

	// Order first, then a top-up on the same proof.
	other := h.proof(t, tenant, p)
	if _, _, err := h.svc.Checkout(ctx, h.scope(tenant), p, CheckoutInput{
		ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "key-" + uuid.NewString()[:12], ProofAssetID: &other,
	}); err != nil {
		t.Fatalf("order: %v", err)
	}
	_, _, err = topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Amount: "50000", Key: "key-" + uuid.NewString()[:12], ProofAssetID: &other})
	wantInputField(t, err, "proof_asset_id")
}

func TestConcurrentOrderAndTopupOnOneProofExactlyOneWins(t *testing.T) {
	h := newOrderHarness(t, noJobs{}, testBank)
	topups := newTopupOverOrders(t, h)
	tenant := h.t.q.a
	_, p := h.student(t, tenant, "")
	product := h.product(t, tenant, "100000", h.tryout(t, tenant, model.TryoutPublished))
	ctx := context.Background()

	for round := 0; round < 5; round++ {
		proof := h.proof(t, tenant, p)
		var wg sync.WaitGroup
		var orderErr, topupErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, orderErr = h.svc.Checkout(ctx, h.scope(tenant), p, CheckoutInput{
				ProductID: product.ID, ExpectedPrice: product.Price, Method: model.PaymentManual, Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof,
			})
		}()
		go func() {
			defer wg.Done()
			_, _, topupErr = topups.Create(ctx, h.scope(tenant), p, CreateTopupInput{Amount: "50000", Key: "key-" + uuid.NewString()[:12], ProofAssetID: &proof})
		}()
		wg.Wait()
		if (orderErr == nil) == (topupErr == nil) {
			t.Fatalf("round %d: order err %v, top-up err %v, want exactly one to succeed", round, orderErr, topupErr)
		}
		used := h.count(t, `SELECT (SELECT count(*) FROM orders WHERE proof_asset_id = $1) + (SELECT count(*) FROM topup_requests WHERE proof_asset_id = $1)`, proof)
		if used != 1 {
			t.Fatalf("round %d: proof backs %d rows, want 1", round, used)
		}
		// Free the pending order so the next round's order is not refused by order_pending.
		if orderErr == nil {
			for _, sql := range []string{`DELETE FROM order_items WHERE tenant_id = $1`, `DELETE FROM orders WHERE tenant_id = $1`} {
				if _, err := h.pool.Exec(ctx, sql, tenant.ID); err != nil {
					t.Fatalf("reset orders: %v", err)
				}
			}
		}
	}
}
