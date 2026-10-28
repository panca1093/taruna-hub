package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type itemPackageFixture struct {
	repo          *ItemPackageRepository
	tenant, other model.Tenant
	creator       model.User
}

func newItemPackageFixture(t *testing.T) itemPackageFixture {
	t.Helper()
	pool := testPool(t)
	tenants := NewTenantRepository(pool)
	f := itemPackageFixture{repo: NewItemPackageRepository(pool), tenant: newTenant(t, tenants), other: newTenant(t, tenants)}
	f.creator = newUser(t, NewUserRepository(pool), &f.tenant.ID, model.RoleAdmin, "ip-"+uuid.NewString()[:8]+"@example.com")
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM item_packages WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.tenant.ID, f.other.ID}); err != nil {
			t.Errorf("cleanup item packages: %v", err)
		}
	})
	return f
}

func (f itemPackageFixture) tx(t *testing.T, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := f.repo.db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	fn(tx)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (f itemPackageFixture) create(t *testing.T, name string) model.ItemPackage {
	t.Helper()
	var pkg model.ItemPackage
	f.tx(t, func(tx pgx.Tx) {
		var err error
		pkg, err = f.repo.CreateTx(context.Background(), tx, f.tenant.ID, f.creator.ID, model.ItemSymbol, name)
		if err != nil {
			t.Fatalf("create package: %v", err)
		}
	})
	return pkg
}

func TestItemPackageIsInvisibleToAnotherTenant(t *testing.T) {
	f := newItemPackageFixture(t)
	ctx := context.Background()
	pkg := f.create(t, "Milik sendiri")

	if got, err := f.repo.Find(ctx, f.repo.db, f.tenant.ID, pkg.ID); err != nil || got.Revision != 1 || got.ElementCount != 0 || got.InUse {
		t.Fatalf("own tenant: %+v, %v; want an empty unused package at revision 1", got, err)
	}
	if _, err := f.repo.Find(ctx, f.repo.db, f.other.ID, pkg.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find from another tenant: err = %v, want ErrNotFound", err)
	}
	if list, err := f.repo.List(ctx, f.other.ID, ""); err != nil || len(list) != 0 {
		t.Errorf("List from another tenant = %v, %v; want nothing", list, err)
	}
	if els, err := f.repo.Elements(ctx, f.repo.db, f.other.ID, pkg.ID, 1); err != nil || len(els) != 0 {
		t.Errorf("Elements from another tenant = %v, %v; want nothing", els, err)
	}
	f.tx(t, func(tx pgx.Tx) {
		if _, err := f.repo.Lock(ctx, tx, f.other.ID, pkg.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("Lock from another tenant: err = %v, want ErrNotFound", err)
		}
		if err := f.repo.RenameTx(ctx, tx, f.other.ID, pkg.ID, "x"); !errors.Is(err, ErrNotFound) {
			t.Errorf("RenameTx from another tenant: err = %v, want ErrNotFound", err)
		}
		if err := f.repo.DeleteTx(ctx, tx, f.other.ID, pkg.ID); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteTx from another tenant: err = %v, want ErrNotFound", err)
		}
	})
	if _, err := f.repo.Find(ctx, f.repo.db, f.tenant.ID, pkg.ID); err != nil {
		t.Errorf("the package vanished after foreign writes: %v", err)
	}
}

func TestItemPackageMethodsRefuseANilTenant(t *testing.T) {
	f := newItemPackageFixture(t)
	ctx := context.Background()
	id := uuid.New()
	if _, err := f.repo.Find(ctx, f.repo.db, uuid.Nil, id); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find(Nil): %v", err)
	}
	if _, err := f.repo.List(ctx, uuid.Nil, ""); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List(Nil): %v", err)
	}
	if _, err := f.repo.Elements(ctx, f.repo.db, uuid.Nil, id, 1); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Elements(Nil): %v", err)
	}
	f.tx(t, func(tx pgx.Tx) {
		if _, err := f.repo.CreateTx(ctx, tx, uuid.Nil, f.creator.ID, model.ItemSymbol, "x"); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("CreateTx(Nil): %v", err)
		}
		if _, err := f.repo.Lock(ctx, tx, uuid.Nil, id); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("Lock(Nil): %v", err)
		}
		if err := f.repo.WriteRevisionTx(ctx, tx, uuid.Nil, id, f.creator.ID, 2, nil); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("WriteRevisionTx(Nil): %v", err)
		}
		if err := f.repo.RenameTx(ctx, tx, uuid.Nil, id, "x"); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("RenameTx(Nil): %v", err)
		}
		if err := f.repo.DeleteTx(ctx, tx, uuid.Nil, id); !errors.Is(err, ErrScopeRequired) {
			t.Errorf("DeleteTx(Nil): %v", err)
		}
	})
}

func TestItemPackageRevisionsAreAppendOnlyAndKeyedByContent(t *testing.T) {
	f := newItemPackageFixture(t)
	ctx := context.Background()
	pkg := f.create(t, "Revisi")

	elements := []model.ItemElement{
		{ID: uuid.New(), Kind: model.ItemSymbol, Key: "♠", Value: "♠", Label: "♠"},
		{ID: uuid.New(), Kind: model.ItemSymbol, Key: "♥", Value: "♥", Label: "♥"},
	}
	f.tx(t, func(tx pgx.Tx) {
		if err := f.repo.WriteRevisionTx(ctx, tx, f.tenant.ID, pkg.ID, f.creator.ID, 2, elements); err != nil {
			t.Fatalf("write revision 2: %v", err)
		}
	})
	got, err := f.repo.Find(ctx, f.repo.db, f.tenant.ID, pkg.ID)
	if err != nil || got.Revision != 2 || got.ElementCount != 2 {
		t.Fatalf("after revision 2: %+v, %v; want revision 2 with 2 elements", got, err)
	}
	if old, err := f.repo.Elements(ctx, f.repo.db, f.tenant.ID, pkg.ID, 1); err != nil || len(old) != 0 {
		t.Errorf("revision 1 = %v, %v; want it still empty", old, err)
	}

	dup := append(append([]model.ItemElement{}, elements...), model.ItemElement{ID: uuid.New(), Kind: model.ItemSymbol, Key: "♠", Value: "♠"})
	tx, err := f.repo.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.WriteRevisionTx(ctx, tx, f.tenant.ID, pkg.ID, f.creator.ID, 3, dup); err == nil {
		t.Error("two elements with the same key in one revision were accepted")
	}
	_ = tx.Rollback(ctx)

	tx, err = f.repo.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.CreateTx(ctx, tx, f.tenant.ID, f.creator.ID, model.ItemImage, " REVISI "); !errors.Is(err, ErrDuplicateName) {
		t.Errorf("duplicate name: err = %v, want ErrDuplicateName", err)
	}
	_ = tx.Rollback(ctx)
}
