package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type topupFixture struct {
	walletFixture
	repo   *TopupRepository
	assets *AssetRepository
}

func newTopupFixture(t *testing.T) topupFixture {
	t.Helper()
	f := topupFixture{walletFixture: newWalletFixture(t)}
	f.repo = NewTopupRepository(f.pool)
	f.assets = NewAssetRepository(f.pool)
	// Registered after the wallet fixture, so requests and proofs go before the users and tenants they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenant.ID, f.other.ID}
		for _, sql := range []string{
			`DELETE FROM topup_requests WHERE tenant_id = ANY($1)`,
			`DELETE FROM assets WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup top-up fixture: %v", err)
			}
		}
	})
	return f
}

func (f topupFixture) proof(t *testing.T, tenantID uuid.UUID, uploader model.User) uuid.UUID {
	t.Helper()
	id := uuid.New()
	asset, err := f.assets.Insert(context.Background(), tenantID, model.Asset{
		ID: id, Purpose: model.PurposePaymentProof, ObjectKey: tenantID.String() + "/payment_proof/" + id.String(),
		ContentType: "image/png", SizeBytes: 10, UploadedBy: &uploader.ID,
	})
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	return asset.ID
}

func (f topupFixture) insert(t *testing.T, tenantID uuid.UUID, user model.User, amount string, proof *uuid.UUID) (model.Topup, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	topup, err := f.repo.InsertTx(ctx, tx, tenantID, model.Topup{
		UserID: user.ID, Amount: amount, Method: model.TopupManual, Key: "key-" + uuid.NewString()[:12], ProofAssetID: proof,
	})
	if err != nil {
		return model.Topup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return topup, nil
}

func TestTopupRequiresScope(t *testing.T) {
	f := newTopupFixture(t)
	ctx := context.Background()
	if _, err := f.repo.Find(ctx, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find: %v, want ErrScopeRequired", err)
	}
	if _, _, err := f.repo.List(ctx, uuid.Nil, TopupFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List: %v, want ErrScopeRequired", err)
	}
	if _, _, err := f.repo.ListForUser(ctx, uuid.Nil, f.student.ID, 10, 0); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("ListForUser: %v, want ErrScopeRequired", err)
	}
	if _, err := f.repo.MinTopup(ctx, f.pool, uuid.Nil); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("MinTopup: %v, want ErrScopeRequired", err)
	}
	if _, err := f.insert(t, uuid.Nil, f.student, "50000", nil); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("InsertTx: %v, want ErrScopeRequired", err)
	}
}

func TestTopupRowsAreScopedToTheirTenant(t *testing.T) {
	f := newTopupFixture(t)
	ctx := context.Background()
	proof := f.proof(t, f.tenant.ID, f.student)
	topup, err := f.insert(t, f.tenant.ID, f.student, "50000", &proof)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := f.repo.Find(ctx, f.other.ID, topup.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find from another tenant: %v, want ErrNotFound", err)
	}
	if list, total, err := f.repo.List(ctx, f.other.ID, TopupFilter{Limit: 10}); err != nil || total != 0 || len(list) != 0 {
		t.Errorf("List from another tenant = %d rows total %d err %v, want none", len(list), total, err)
	}
	if list, total, err := f.repo.ListForUser(ctx, f.tenant.ID, f.second.ID, 10, 0); err != nil || total != 0 || len(list) != 0 {
		t.Errorf("ListForUser of another student = %d rows total %d err %v, want none", len(list), total, err)
	}
	if minimum, err := f.repo.MinTopup(ctx, f.pool, f.tenant.ID); err != nil || minimum != model.DefaultMinTopup {
		t.Errorf("MinTopup without a settings row = %q err %v, want %s", minimum, err, model.DefaultMinTopup)
	}
}

func TestTopupProofCannotBackTwoRequestsOrCrossTenants(t *testing.T) {
	f := newTopupFixture(t)
	proof := f.proof(t, f.tenant.ID, f.student)
	if _, err := f.insert(t, f.tenant.ID, f.student, "50000", &proof); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := f.insert(t, f.tenant.ID, f.student, "50000", &proof); !errors.Is(err, ErrProofTaken) {
		t.Errorf("second request on one proof: %v, want ErrProofTaken", err)
	}
	foreign := f.proof(t, f.other.ID, f.otherStudent)
	if _, err := f.insert(t, f.tenant.ID, f.student, "50000", &foreign); err == nil {
		t.Error("a proof of another tenant backed a request")
	}
}

func TestTopupTableRefusesIncoherentRows(t *testing.T) {
	f := newTopupFixture(t)
	ctx := context.Background()
	proof := func() *uuid.UUID {
		id := f.proof(t, f.tenant.ID, f.student)
		return &id
	}
	now := time.Now()
	insert := func(amount, status string, proofID *uuid.UUID, reason string, rejectedAt, approvedAt *time.Time) error {
		_, err := f.pool.Exec(ctx, `
			INSERT INTO topup_requests (tenant_id, user_id, amount, method, status, idempotency_key, proof_asset_id,
			        reject_reason, rejected_at, approved_at)
			VALUES ($1, $2, $3::numeric, 'manual', $4, $5, $6, $7, $8, $9)`,
			f.tenant.ID, f.student.ID, amount, status, "key-"+uuid.NewString()[:12], proofID, reason, rejectedAt, approvedAt)
		return err
	}

	cases := map[string]error{
		"fractional amount":       insert("50000.50", "pending", proof(), "", nil, nil),
		"zero amount":             insert("0", "pending", proof(), "", nil, nil),
		"manual without proof":    insert("50000", "pending", nil, "", nil, nil),
		"approved without a time": insert("50000", "approved", proof(), "", nil, nil),
		"rejected without reason": insert("50000", "rejected", proof(), "", &now, nil),
		"unknown status":          insert("50000", "paid", proof(), "", nil, nil),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s was accepted, want a constraint violation", name)
		}
	}
	if err := insert("50000", "pending", proof(), "", nil, nil); err != nil {
		t.Errorf("a coherent pending row: %v", err)
	}
	if err := insert("50000", "rejected", proof(), "Nominal tidak sesuai", &now, nil); err != nil {
		t.Errorf("a coherent rejected row: %v", err)
	}
}
