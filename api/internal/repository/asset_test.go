package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type assetFixture struct {
	repo          *AssetRepository
	tenant, other model.Tenant
	uploader      model.User
}

func newAssetFixture(t *testing.T) assetFixture {
	t.Helper()
	pool := testPool(t)
	tenants := NewTenantRepository(pool)
	f := assetFixture{repo: NewAssetRepository(pool), tenant: newTenant(t, tenants), other: newTenant(t, tenants)}
	f.uploader = newUser(t, NewUserRepository(pool), &f.tenant.ID, model.RoleStudent, "a-"+uuid.NewString()[:8]+"@example.com")
	// Registered after the tenant and user cleanups, so these rows go before what they reference.
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM assets WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.tenant.ID, f.other.ID}); err != nil {
			t.Errorf("cleanup assets: %v", err)
		}
	})
	return f
}

func (f assetFixture) insert(t *testing.T, tenantID uuid.UUID) model.Asset {
	t.Helper()
	id := uuid.New()
	a, err := f.repo.Insert(context.Background(), tenantID, model.Asset{
		ID: id, Purpose: model.PurposePaymentProof, ObjectKey: tenantID.String() + "/payment_proof/" + id.String(),
		ContentType: "image/png", SizeBytes: 10, UploadedBy: &f.uploader.ID,
	})
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	return a
}

func TestAssetRowIsInvisibleToAnotherTenant(t *testing.T) {
	f := newAssetFixture(t)
	ctx := context.Background()
	a := f.insert(t, f.tenant.ID)

	if got, err := f.repo.Find(ctx, f.repo.db, f.tenant.ID, a.ID); err != nil || got.Status != model.AssetStaged {
		t.Fatalf("own tenant: asset = %+v, err = %v; want a staged row", got, err)
	}
	if _, err := f.repo.Find(ctx, f.repo.db, f.other.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find from another tenant: err = %v, want ErrNotFound", err)
	}
	if _, err := f.repo.Finalize(ctx, f.repo.db, f.other.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Finalize from another tenant: err = %v, want ErrNotFound", err)
	}
	if got, _ := f.repo.Find(ctx, f.repo.db, f.tenant.ID, a.ID); got.Status != model.AssetStaged {
		t.Errorf("status after a foreign finalize = %q, want staged", got.Status)
	}
}

func TestAssetMethodsRefuseANilTenant(t *testing.T) {
	f := newAssetFixture(t)
	ctx := context.Background()
	if _, err := f.repo.Insert(ctx, uuid.Nil, model.Asset{}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Insert(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := f.repo.Find(ctx, f.repo.db, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := f.repo.Finalize(ctx, f.repo.db, uuid.Nil, uuid.New()); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Finalize(Nil): err = %v, want ErrScopeRequired", err)
	}
}

func TestAssetFinalizeKeepsTheFirstTimestamp(t *testing.T) {
	f := newAssetFixture(t)
	ctx := context.Background()
	a := f.insert(t, f.tenant.ID)

	first, err := f.repo.Finalize(ctx, f.repo.db, f.tenant.ID, a.ID)
	if err != nil || first.Status != model.AssetFinal || first.FinalizedAt == nil {
		t.Fatalf("first finalize = %+v, err = %v; want a final row with a timestamp", first, err)
	}
	again, err := f.repo.Finalize(ctx, f.repo.db, f.tenant.ID, a.ID)
	if err != nil || !again.FinalizedAt.Equal(*first.FinalizedAt) {
		t.Errorf("second finalize = %+v, err = %v; want the first timestamp %v", again, err, first.FinalizedAt)
	}
}

func TestAssetPurgeScanAndDeleteSkipFinalRows(t *testing.T) {
	f := newAssetFixture(t)
	ctx := context.Background()
	staged, final := f.insert(t, f.tenant.ID), f.insert(t, f.tenant.ID)
	if _, err := f.repo.Finalize(ctx, f.repo.db, f.tenant.ID, final.ID); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if _, err := f.repo.db.Exec(ctx, `UPDATE assets SET created_at = now() - interval '2 days' WHERE id = ANY($1)`,
		[]uuid.UUID{staged.ID, final.ID}); err != nil {
		t.Fatalf("age rows: %v", err)
	}
	cutoff := time.Now().Add(-24 * time.Hour)

	old, err := f.repo.StagedBefore(ctx, cutoff, 1000)
	if err != nil {
		t.Fatalf("StagedBefore: %v", err)
	}
	seen := map[uuid.UUID]bool{}
	for _, a := range old {
		seen[a.ID] = true
	}
	if !seen[staged.ID] || seen[final.ID] {
		t.Errorf("scan saw staged=%v final=%v, want true and false", seen[staged.ID], seen[final.ID])
	}

	tx, err := f.repo.db.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := f.repo.LockStaged(ctx, tx, f.tenant.ID, final.ID, cutoff); !errors.Is(err, ErrNotFound) {
		t.Errorf("LockStaged on a final row: err = %v, want ErrNotFound", err)
	}
	if err := f.repo.DeleteStaged(ctx, tx, f.tenant.ID, final.ID); err != nil {
		t.Fatalf("DeleteStaged: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := f.repo.Find(ctx, f.repo.db, f.tenant.ID, final.ID); err != nil {
		t.Errorf("final row after DeleteStaged: err = %v, want it kept", err)
	}
}
