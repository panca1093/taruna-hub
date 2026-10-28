package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type kecermatanFixture struct {
	tryoutFixture
	kec      *KecermatanRepository
	packages *ItemPackageRepository
}

func newKecermatanFixture(t *testing.T) kecermatanFixture {
	t.Helper()
	f := kecermatanFixture{tryoutFixture: newTryoutFixture(t)}
	f.kec, f.packages = NewKecermatanRepository(f.pool), NewItemPackageRepository(f.pool)
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenant.ID, f.other.ID}
		for _, sql := range []string{
			`DELETE FROM kecermatan_test_packages WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_tests WHERE tenant_id = ANY($1)`,
			`DELETE FROM item_packages WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup kecermatan: %v", err)
			}
		}
	})
	return f
}

func (f kecermatanFixture) tx(t *testing.T, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	fn(tx)
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (f kecermatanFixture) pkg(t *testing.T, tenant model.Tenant, creator model.User) model.ItemPackage {
	t.Helper()
	var out model.ItemPackage
	f.tx(t, func(tx pgx.Tx) {
		var err error
		out, err = f.packages.CreateTx(context.Background(), tx, tenant.ID, creator.ID, model.ItemSymbol, "Paket "+uuid.NewString()[:8])
		if err != nil {
			t.Fatalf("create package: %v", err)
		}
	})
	return out
}

func (f kecermatanFixture) draftTest(programID uuid.UUID) model.KecermatanTest {
	return model.KecermatanTest{
		ProgramID: programID, Title: "Kecermatan", ItemType: model.KecSymbol, ItemLength: 4, OptionsPerSoal: 5,
		SoalPerKolom: 50, KolomCount: 10, KolomDurationSeconds: 120, CoverFrom: "#f59e0b", CoverTo: "#ef4444",
		Seed: "00112233445566778899aabbccddeeff",
	}
}

func (f kecermatanFixture) create(t *testing.T, pins ...KecermatanPin) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	f.tx(t, func(tx pgx.Tx) {
		var err error
		if id, err = f.kec.CreateTx(context.Background(), tx, f.tenant.ID, f.author.ID, f.draftTest(f.program.ID)); err != nil {
			t.Fatalf("create test: %v", err)
		}
		if err = f.kec.ReplacePinsTx(context.Background(), tx, f.tenant.ID, id, pins); err != nil {
			t.Fatalf("pin packages: %v", err)
		}
	})
	return id
}

func TestKecermatanTestIsScopedToItsTenant(t *testing.T) {
	f := newKecermatanFixture(t)
	ctx := context.Background()
	pkg := f.pkg(t, f.tenant, f.author)
	id := f.create(t, KecermatanPin{PackageID: pkg.ID, Revision: pkg.Revision, RequiredCount: 20})

	got, err := f.kec.Find(ctx, f.pool, f.tenant.ID, id)
	if err != nil || got.Revision != 1 || got.Locked || got.Seed == "" || len(got.Packages) != 1 || got.Packages[0].RequiredCount != 20 || got.Packages[0].Type != model.ItemSymbol {
		t.Fatalf("Find = %+v, %v; want revision 1, unlocked, one pinned symbol package needing 20", got, err)
	}
	if _, err := f.kec.Find(ctx, f.pool, f.other.ID, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find from another tenant: err = %v, want ErrNotFound", err)
	}
	if list, total, err := f.kec.List(ctx, f.other.ID, KecermatanFilter{Limit: 10}); err != nil || total != 0 || len(list) != 0 {
		t.Errorf("List from another tenant = %v, %d, %v; want nothing", list, total, err)
	}
	f.tx(t, func(tx pgx.Tx) {
		if _, err := f.kec.LockTx(ctx, tx, f.other.ID, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("LockTx from another tenant: err = %v, want ErrNotFound", err)
		}
		if err := f.kec.UpdateTx(ctx, tx, f.other.ID, id, f.draftTest(f.otherProgram.ID)); !errors.Is(err, ErrNotFound) {
			t.Errorf("UpdateTx from another tenant: err = %v, want ErrNotFound", err)
		}
		if err := f.kec.DeleteTx(ctx, tx, f.other.ID, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteTx from another tenant: err = %v, want ErrNotFound", err)
		}
	})
	if _, err := f.kec.Find(ctx, f.pool, uuid.Nil, id); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("Find without a tenant: err = %v, want ErrScopeRequired", err)
	}
	if _, _, err := f.kec.List(ctx, uuid.Nil, KecermatanFilter{Limit: 10}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("List without a tenant: err = %v, want ErrScopeRequired", err)
	}
}

func TestKecermatanPinsAreHeldByForeignKeys(t *testing.T) {
	f := newKecermatanFixture(t)
	ctx := context.Background()
	pkg := f.pkg(t, f.tenant, f.author)
	foreign := f.pkg(t, f.other, f.otherBy)
	id := f.create(t, KecermatanPin{PackageID: pkg.ID, Revision: pkg.Revision, RequiredCount: 3})

	violates := func(name string, err error) {
		t.Helper()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgForeignKeyViolation {
			t.Errorf("%s: err = %v, want a foreign-key violation", name, err)
		}
	}
	_, err := f.pool.Exec(ctx, `DELETE FROM kecermatan_tests WHERE id = $1`, id)
	violates("raw delete of a test that still pins a package", err)
	_, err = f.pool.Exec(ctx, `DELETE FROM item_packages WHERE id = $1`, pkg.ID)
	violates("raw delete of a pinned package", err)
	_, err = f.pool.Exec(ctx, `
		INSERT INTO kecermatan_test_packages (test_id, tenant_id, package_id, revision, required_count)
		VALUES ($1, $2, $3, 1, 1)`, uuid.New(), f.tenant.ID, foreign.ID)
	violates("a pin whose test does not exist", err)
	_, err = f.pool.Exec(ctx, `
		INSERT INTO kecermatan_test_packages (test_id, tenant_id, package_id, revision, required_count)
		VALUES ($1, $2, $3, 1, 1)`, id, f.tenant.ID, foreign.ID)
	violates("a pin of another tenant's package", err)
	_, err = f.pool.Exec(ctx, `
		INSERT INTO kecermatan_tests (tenant_id, program_id, title, item_type, item_length, options_per_soal,
		        soal_per_kolom, kolom_count, kolom_duration_seconds, cover_from, cover_to, generation_seed, created_by)
		VALUES ($1, $2, 't', 'char', 4, 5, 50, 10, 60, '#000000', '#ffffff', repeat('0', 32), $3)`,
		f.tenant.ID, f.otherProgram.ID, f.author.ID)
	violates("a test built on another tenant's program", err)

	f.tx(t, func(tx pgx.Tx) {
		if err := f.kec.DeleteTx(ctx, tx, f.tenant.ID, id); err != nil {
			t.Fatalf("DeleteTx: %v", err)
		}
	})
	var pins int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM kecermatan_test_packages WHERE package_id = $1`, pkg.ID).Scan(&pins)
	if pins != 0 {
		t.Errorf("%d pins left after the test was deleted, want none", pins)
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM item_packages WHERE id = $1`, pkg.ID); err != nil {
		t.Errorf("deleting the released package: %v, want it allowed", err)
	}
}

func TestKecermatanUpdateBumpsRevisionAndKeepsTheSeed(t *testing.T) {
	f := newKecermatanFixture(t)
	ctx := context.Background()
	id := f.create(t)
	before, _ := f.kec.Find(ctx, f.pool, f.tenant.ID, id)

	next := f.draftTest(f.program.ID)
	next.Title, next.Seed = "Baru", "ffffffffffffffffffffffffffffffff"
	f.tx(t, func(tx pgx.Tx) {
		if err := f.kec.UpdateTx(ctx, tx, f.tenant.ID, id, next); err != nil {
			t.Fatal(err)
		}
	})
	after, _ := f.kec.Find(ctx, f.pool, f.tenant.ID, id)
	if after.Revision != before.Revision+1 || after.Title != "Baru" || after.Seed != before.Seed {
		t.Errorf("after update: revision %d (was %d), title %q, seed %q (was %q); want revision +1, the new title and the same seed",
			after.Revision, before.Revision, after.Title, after.Seed, before.Seed)
	}
}

func TestKecermatanTableRefusesWhatTheServiceShouldNeverSend(t *testing.T) {
	f := newKecermatanFixture(t)
	for name, mutate := range map[string]func(*model.KecermatanTest){
		"odd kolom":     func(k *model.KecermatanTest) { k.KolomCount = 11 },
		"9 kolom":       func(k *model.KecermatanTest) { k.KolomCount = 8 },
		"49 soal":       func(k *model.KecermatanTest) { k.SoalPerKolom = 49 },
		"length 6":      func(k *model.KecermatanTest) { k.ItemLength = 6 },
		"zero duration": func(k *model.KecermatanTest) { k.KolomDurationSeconds = 0 },
		"bad seed":      func(k *model.KecermatanTest) { k.Seed = "xyz" },
		"unknown type":  func(k *model.KecermatanTest) { k.ItemType = "audio" },
	} {
		bad := f.draftTest(f.program.ID)
		mutate(&bad)
		f.tx2(t, func(tx pgx.Tx) {
			if _, err := f.kec.CreateTx(context.Background(), tx, f.tenant.ID, f.author.ID, bad); err == nil {
				t.Errorf("%s: the table accepted it", name)
			}
		})
	}
}

// tx2 is tx for a body expected to fail inside the transaction, so nothing is committed.
func (f kecermatanFixture) tx2(t *testing.T, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	fn(tx)
}
