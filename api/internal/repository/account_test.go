package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func inTx(t *testing.T, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func throttleTestKey() []byte {
	sum := sha256.Sum256([]byte("throttle-test-" + uuid.NewString()))
	return sum[:]
}

func TestThrottleCountsConcurrentHitsAndRollsTheWindow(t *testing.T) {
	pool := testPool(t)
	repo := NewThrottleRepository(pool)
	ctx := context.Background()
	key := throttleTestKey()
	t.Cleanup(func() { _ = repo.Clear(ctx, key) })
	start := time.Now().UTC().Truncate(time.Microsecond)

	var wg sync.WaitGroup
	seen := make(chan int, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, _, err := repo.Hit(ctx, key, time.Minute, start)
			if err != nil {
				t.Errorf("hit: %v", err)
				return
			}
			seen <- hits
		}()
	}
	wg.Wait()
	close(seen)
	counts := map[int]bool{}
	for hits := range seen {
		counts[hits] = true
	}
	for i := 1; i <= 20; i++ {
		if !counts[i] {
			t.Fatalf("no hit saw count %d; concurrent hits were lost or doubled: %v", i, counts)
		}
	}

	// Still inside the window: the count keeps going and the start stays put.
	hits, windowStart, err := repo.Hit(ctx, key, time.Minute, start.Add(59*time.Second))
	if err != nil || hits != 21 || !windowStart.Equal(start) {
		t.Errorf("inside the window = %d, %s, %v; want 21, %s", hits, windowStart, err, start)
	}
	// At the window's end a new window opens with this hit as its first.
	later := start.Add(time.Minute)
	hits, windowStart, err = repo.Hit(ctx, key, time.Minute, later)
	if err != nil || hits != 1 || !windowStart.Equal(later) {
		t.Errorf("after the window = %d, %s, %v; want 1, %s", hits, windowStart, err, later)
	}
	if err := repo.Clear(ctx, key); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if hits, _, _ := repo.Hit(ctx, key, time.Minute, later); hits != 1 {
		t.Errorf("after clear = %d, want 1", hits)
	}
}

func TestPasswordResetIsScopedAndSingleUse(t *testing.T) {
	pool := testPool(t)
	tenants, users, accounts := NewTenantRepository(pool), NewUserRepository(pool), NewAccountRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)
	user := newUser(t, users, &a.ID, model.RoleStudent, "reset-"+uuid.NewString()[:8]+"@example.com")
	now := time.Now().UTC().Truncate(time.Microsecond)

	create := func(hash []byte, at time.Time) {
		t.Helper()
		if err := inTx(t, pool, func(tx pgx.Tx) error {
			return accounts.CreateResetTx(context.Background(), tx, model.PasswordReset{
				UserID: user.ID, TenantID: &a.ID, TokenHash: hash, ExpiresAt: at.Add(30 * time.Minute), CreatedAt: at,
			})
		}); err != nil {
			t.Fatalf("create reset: %v", err)
		}
	}
	consume := func(scope *uuid.UUID, hash []byte, at time.Time) error {
		return inTx(t, pool, func(tx pgx.Tx) error {
			_, err := accounts.ConsumeResetTx(context.Background(), tx, scope, hash, at)
			return err
		})
	}

	older, newer := testHash(), testHash()
	create(older, now)
	create(newer, now.Add(time.Second))
	if err := consume(&a.ID, older, now.Add(2*time.Second)); !errors.Is(err, ErrNotFound) {
		t.Errorf("a token the newer one retired: err = %v, want ErrNotFound", err)
	}
	if err := consume(&b.ID, newer, now.Add(2*time.Second)); !errors.Is(err, ErrNotFound) {
		t.Errorf("another tenant's scope: err = %v, want ErrNotFound", err)
	}
	if err := consume(nil, newer, now.Add(2*time.Second)); !errors.Is(err, ErrNotFound) {
		t.Errorf("the platform scope: err = %v, want ErrNotFound", err)
	}
	if err := consume(&a.ID, newer, now.Add(30*time.Minute+time.Second)); !errors.Is(err, ErrNotFound) {
		t.Errorf("past expiry: err = %v, want ErrNotFound", err)
	}
	if err := consume(&a.ID, newer, now.Add(2*time.Second)); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if err := consume(&a.ID, newer, now.Add(3*time.Second)); !errors.Is(err, ErrNotFound) {
		t.Errorf("a second use: err = %v, want ErrNotFound", err)
	}
}

func TestRegistrationUpsertReplacesThePendingRow(t *testing.T) {
	pool := testPool(t)
	tenant := newTenant(t, NewTenantRepository(pool))
	accounts := NewAccountRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	email := "pending-" + uuid.NewString()[:8] + "@example.com"

	upsert := func(name string) model.Registration {
		t.Helper()
		var reg model.Registration
		if err := inTx(t, pool, func(tx pgx.Tx) error {
			var err error
			reg, err = accounts.UpsertRegistrationTx(ctx, tx, model.Registration{
				TenantID: tenant.ID, Email: email, Name: name, PasswordHash: "x",
				CodeID: uuid.New(), CodeHash: testHash(), ExpiresAt: now.Add(10 * time.Minute), SentAt: now,
			})
			return err
		}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		return reg
	}
	first := upsert("First")
	if err := inTx(t, pool, func(tx pgx.Tx) error { return accounts.CountAttemptTx(ctx, tx, tenant.ID, first.ID) }); err != nil {
		t.Fatalf("count attempt: %v", err)
	}
	second := upsert("Second")
	if second.ID != first.ID || second.Name != "Second" || second.Attempts != 0 || second.CodeID == first.CodeID {
		t.Errorf("re-signup = %+v, want the same row with the new name, a new code and a fresh budget", second)
	}
	if err := inTx(t, pool, func(tx pgx.Tx) error {
		_, err := accounts.UpsertRegistrationTx(ctx, tx, model.Registration{Email: email})
		return err
	}); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("an unscoped upsert: err = %v, want ErrScopeRequired", err)
	}
}

func TestRotateIsConditionalAndKeepsThePreviousHash(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)
	s := f.newSession(t, at)
	next, stale := testHash(), testHash()

	if ok, err := f.srepo.Rotate(ctx, s.ID, stale, testHash(), at); err != nil || ok {
		t.Errorf("rotate from a token that is not current = %v, %v; want false, nil", ok, err)
	}
	if ok, err := f.srepo.Rotate(ctx, s.ID, s.RefreshHash, next, at); err != nil || !ok {
		t.Fatalf("rotate = %v, %v; want true, nil", ok, err)
	}
	// The replaced hash still finds the session, so the service can tell a replay from a stranger.
	byOld, err := f.srepo.FindByRefreshHash(ctx, s.RefreshHash)
	if err != nil || byOld.ID != s.ID || string(byOld.RefreshHash) != string(next) || byOld.RotatedAt == nil {
		t.Errorf("lookup by the replaced hash = %+v, %v; want the rotated session", byOld, err)
	}
	if ok, _ := f.srepo.Rotate(ctx, s.ID, s.RefreshHash, testHash(), at); ok {
		t.Error("a second rotation from the replaced hash succeeded")
	}
}

func TestCreateUserReportsWhichIdentifierIsTaken(t *testing.T) {
	pool := testPool(t)
	tenant := newTenant(t, NewTenantRepository(pool))
	users := NewUserRepository(pool)
	ctx := context.Background()
	email, phone := "dup-"+uuid.NewString()[:8]+"@example.com", "+6281255501234"
	if _, err := users.Create(ctx, model.User{TenantID: &tenant.ID, Name: "A", Email: &email, Phone: &phone, PasswordHash: "x", Role: model.RoleStudent}); err != nil {
		t.Fatalf("create: %v", err)
	}
	other, otherPhone := "other-"+uuid.NewString()[:8]+"@example.com", "+6281299990000"
	if _, err := users.Create(ctx, model.User{TenantID: &tenant.ID, Name: "B", Email: &email, Phone: &otherPhone, PasswordHash: "x", Role: model.RoleStudent}); !errors.Is(err, ErrDuplicateEmail) || !errors.Is(err, ErrDuplicate) {
		t.Errorf("same email: err = %v, want ErrDuplicateEmail", err)
	}
	if _, err := users.Create(ctx, model.User{TenantID: &tenant.ID, Name: "C", Email: &other, Phone: &phone, PasswordHash: "x", Role: model.RoleStudent}); !errors.Is(err, ErrDuplicatePhone) {
		t.Errorf("same phone: err = %v, want ErrDuplicatePhone", err)
	}
	upper, local := "Upper@Example.com", "081234567890"
	if _, err := users.Create(ctx, model.User{TenantID: &tenant.ID, Name: "D", Email: &upper, PasswordHash: "x", Role: model.RoleStudent}); err == nil {
		t.Error("a non-canonical email was stored, want the check constraint to refuse it")
	}
	if _, err := users.Create(ctx, model.User{TenantID: &tenant.ID, Name: "E", Email: &other, Phone: &local, PasswordHash: "x", Role: model.RoleStudent}); err == nil {
		t.Error("a non-E.164 phone was stored, want the check constraint to refuse it")
	}
}
