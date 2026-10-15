package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/redis/go-redis/v9"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"os"
	"sync"
)

type sessionFixture struct {
	pool   *pgxpool.Pool
	tenant model.Tenant
	user   model.User
	srepo  *SessionRepository
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()
	pool := testPool(t)
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	tenant := newTenant(t, tenants)
	user := newUser(t, users, &tenant.ID, model.RoleStudent, "sess-"+uuid.NewString()[:8]+"@example.com")
	return sessionFixture{pool: pool, tenant: tenant, user: user, srepo: NewSessionRepository(pool)}
}

func (f sessionFixture) newSession(t *testing.T, at time.Time) model.Session {
	t.Helper()
	s, err := f.srepo.Create(context.Background(), model.Session{
		UserID:         f.user.ID,
		TenantID:       f.user.TenantID,
		RefreshHash:    testHash(),
		CreatedAt:      at,
		LastActivityAt: at,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return s
}

func (f sessionFixture) lastActivity(t *testing.T, id uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := f.pool.QueryRow(context.Background(), `SELECT last_activity_at FROM auth_sessions WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("read last_activity_at: %v", err)
	}
	return at
}

func testHash() []byte {
	h := sha256.Sum256([]byte(uuid.NewString()))
	return h[:]
}

func TestSessionRoundTripAndLookups(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Microsecond)

	s := f.newSession(t, at)
	if s.ID == uuid.Nil || s.UserID != f.user.ID || s.TenantID == nil || *s.TenantID != f.tenant.ID {
		t.Fatalf("created session = %+v, want it owned by user %s in tenant %s", s, f.user.ID, f.tenant.ID)
	}
	if s.RevokedAt != nil {
		t.Errorf("new session revoked_at = %v, want nil", s.RevokedAt)
	}

	byID, err := f.srepo.FindByID(ctx, s.ID)
	if err != nil || byID.ID != s.ID {
		t.Errorf("FindByID = %+v, %v; want session %s", byID, err, s.ID)
	}
	byHash, err := f.srepo.FindByRefreshHash(ctx, s.RefreshHash)
	if err != nil || byHash.ID != s.ID {
		t.Errorf("FindByRefreshHash = %+v, %v; want session %s", byHash, err, s.ID)
	}
	if _, err := f.srepo.FindByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID(unknown): err = %v, want ErrNotFound", err)
	}
	if _, err := f.srepo.FindByRefreshHash(ctx, testHash()); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByRefreshHash(unknown): err = %v, want ErrNotFound", err)
	}
}

// The column is the only place a refresh token exists server-side, so its shape is a real defence.
func TestRefreshHashShapeIsEnforced(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()

	_, err := f.srepo.Create(ctx, model.Session{
		UserID: f.user.ID, TenantID: f.user.TenantID,
		RefreshHash: make([]byte, 31), CreatedAt: at, LastActivityAt: at,
	})
	if err == nil {
		t.Error("a 31-byte refresh_hash was accepted, want the length check to reject it")
	}

	hash := testHash()
	if _, err := f.srepo.Create(ctx, model.Session{
		UserID: f.user.ID, TenantID: f.user.TenantID,
		RefreshHash: hash, CreatedAt: at, LastActivityAt: at,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = f.srepo.Create(ctx, model.Session{
		UserID: f.user.ID, TenantID: f.user.TenantID,
		RefreshHash: hash, CreatedAt: at, LastActivityAt: at,
	})
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate refresh_hash: err = %v, want ErrDuplicate", err)
	}
}

func TestRevokeByRefreshHashRespectsScope(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	other := newTenant(t, NewTenantRepository(f.pool))

	tenantSession := f.newSession(t, at)
	platform, err := NewUserRepository(f.pool).Create(ctx, model.User{
		Name: "Root", Email: ptr("plat-" + uuid.NewString()[:8] + "@example.com"),
		PasswordHash: "x", Role: model.RolePlatformAdmin,
	})
	if err != nil {
		t.Fatalf("create platform user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, platform.ID); err != nil {
			t.Errorf("cleanup platform sessions: %v", err)
		}
		if _, err := f.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, platform.ID); err != nil {
			t.Errorf("cleanup platform user: %v", err)
		}
	})
	platformSession, err := f.srepo.Create(ctx, model.Session{
		UserID: platform.ID, RefreshHash: testHash(), CreatedAt: at, LastActivityAt: at,
	})
	if err != nil {
		t.Fatalf("create platform session: %v", err)
	}

	cases := []struct {
		name     string
		hash     []byte
		scope    *uuid.UUID
		wantGone bool
	}{
		{"another tenant cannot revoke it", tenantSession.RefreshHash, &other.ID, false},
		{"the platform scope cannot revoke a tenant session", tenantSession.RefreshHash, nil, false},
		{"the owning tenant revokes it", tenantSession.RefreshHash, &f.tenant.ID, true},
		{"a tenant cannot revoke a platform session", platformSession.RefreshHash, &f.tenant.ID, false},
		{"the platform scope revokes its own session", platformSession.RefreshHash, nil, true},
		{"an unknown hash is a no-op", testHash(), &f.tenant.ID, false},
	}
	for _, tc := range cases {
		gone, err := f.srepo.RevokeByRefreshHash(ctx, tc.hash, tc.scope, at)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if gone != tc.wantGone {
			t.Errorf("%s: revoked = %v, want %v", tc.name, gone, tc.wantGone)
		}
	}

	if gone, err := f.srepo.RevokeByRefreshHash(ctx, tenantSession.RefreshHash, &f.tenant.ID, at); err != nil || gone {
		t.Errorf("second revoke: revoked = %v, err = %v; want false, nil (idempotent)", gone, err)
	}
}

func TestTouchOnlyMovesStaleSessions(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Microsecond)
	s := f.newSession(t, base)

	if err := f.srepo.Touch(ctx, s.ID, base.Add(30*time.Second), base.Add(time.Minute)); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if got := f.lastActivity(t, s.ID); !got.Equal(base.Add(time.Minute)) {
		t.Errorf("last_activity_at = %s, want %s", got, base.Add(time.Minute))
	}

	// Newer than the threshold: the conditional update must be a no-op.
	if err := f.srepo.Touch(ctx, s.ID, base.Add(30*time.Second), base.Add(10*time.Minute)); err != nil {
		t.Fatalf("touch again: %v", err)
	}
	if got := f.lastActivity(t, s.ID); !got.Equal(base.Add(time.Minute)) {
		t.Errorf("last_activity_at = %s, want it unchanged at %s", got, base.Add(time.Minute))
	}

	if err := f.srepo.Touch(ctx, s.ID, base.Add(2*time.Minute), base.Add(3*time.Minute)); err != nil {
		t.Fatalf("touch stale: %v", err)
	}
	if got := f.lastActivity(t, s.ID); !got.Equal(base.Add(3 * time.Minute)) {
		t.Errorf("last_activity_at = %s, want %s", got, base.Add(3*time.Minute))
	}
}

func ptr(s string) *string { return &s }

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestThrottleCountsConcurrentHitsAndRollsTheWindow(t *testing.T) {
	repo := NewThrottleRepository(testRedis(t))
	ctx := context.Background()
	key := testHash()
	window := time.Minute
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	seen := make(chan int, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, _, err := repo.Hit(ctx, key, window, start)
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

	hits, end, err := repo.Hit(ctx, key, window, start.Add(59*time.Second))
	if err != nil || hits != 21 || !end.Equal(start.Add(window)) {
		t.Errorf("inside the window = %d, %s, %v; want 21, %s", hits, end, err, start.Add(window))
	}
	next := start.Add(window)
	if hits, end, _ := repo.Hit(ctx, key, window, next); hits != 1 || !end.Equal(next.Add(window)) {
		t.Errorf("the next window = %d ending %s, want 1 ending %s", hits, end, next.Add(window))
	}
	if err := repo.Clear(ctx, key, window, next); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if hits, _, _ := repo.Hit(ctx, key, window, next); hits != 1 {
		t.Errorf("after clear = %d, want 1", hits)
	}
	if ttl := repo.rdb.TTL(ctx, throttleKey(key, next.UnixNano()/int64(window))).Val(); ttl <= 0 || ttl > 2*window {
		t.Errorf("key TTL = %s, want it bounded by two windows", ttl)
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
