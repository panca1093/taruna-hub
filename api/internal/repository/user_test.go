package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jackc/pgx/v5"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"time"
)

// Built with pgxpool directly: infra now imports service, so importing infra from this package would cycle.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTenant(t *testing.T, repo *TenantRepository) model.Tenant {
	t.Helper()
	ctx := context.Background()
	tenant, err := repo.Create(ctx, "t-"+uuid.NewString()[:8], "Test Tenant")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	t.Cleanup(func() {
		for _, table := range []string{"notification_logs"} {
			if _, err := repo.db.Exec(ctx, `DELETE FROM `+table+` WHERE tenant_id = $1`, tenant.ID); err != nil {
				t.Errorf("cleanup %s of tenant %s: %v", table, tenant.ID, err)
			}
		}
		if _, err := repo.db.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id IN (SELECT id FROM users WHERE tenant_id = $1)`, tenant.ID); err != nil {
			t.Errorf("cleanup sessions of tenant %s: %v", tenant.ID, err)
		}
		if _, err := repo.db.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, tenant.ID); err != nil {
			t.Errorf("cleanup users of tenant %s: %v", tenant.ID, err)
		}
		if _, err := repo.db.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
			t.Errorf("cleanup tenant %s: %v", tenant.ID, err)
		}
	})
	return tenant
}

func newUser(t *testing.T, repo *UserRepository, tenantID *uuid.UUID, role model.Role, email string) model.User {
	t.Helper()
	u, err := repo.Create(context.Background(), model.User{
		TenantID: tenantID, Name: "Probe", Email: &email, PasswordHash: "x", Role: role,
	})
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	t.Cleanup(func() {
		if _, err := repo.db.Exec(context.Background(), `DELETE FROM auth_sessions WHERE user_id = $1`, u.ID); err != nil {
			t.Errorf("cleanup sessions of user %s: %v", u.ID, err)
		}
		if _, err := repo.db.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
			t.Errorf("cleanup user %s: %v", u.ID, err)
		}
	})
	return u
}

func TestSameEmailAllowedAcrossTenantsRejectedWithin(t *testing.T) {
	pool := testPool(t)
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)
	email := "dup-" + uuid.NewString()[:8] + "@example.com"

	newUser(t, users, &a.ID, model.RoleStudent, email)
	newUser(t, users, &b.ID, model.RoleStudent, email)

	_, err := users.Create(context.Background(), model.User{
		TenantID: &a.ID, Name: "Dup", Email: &email, PasswordHash: "x", Role: model.RoleStudent,
	})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate inside tenant A: err = %v, want ErrDuplicate", err)
	}
}

func TestPlatformAccountsAreUniqueAndRoleMustMatchTenant(t *testing.T) {
	pool := testPool(t)
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a := newTenant(t, tenants)
	email := "plat-" + uuid.NewString()[:8] + "@example.com"

	newUser(t, users, nil, model.RolePlatformAdmin, email)
	_, err := users.Create(context.Background(), model.User{
		Name: "Dup", Email: &email, PasswordHash: "x", Role: model.RolePlatformAdmin,
	})
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate platform email: err = %v, want ErrDuplicate", err)
	}

	cases := map[string]model.User{
		"tenant user without tenant":   {Name: "x", Email: &email, PasswordHash: "x", Role: model.RoleStudent},
		"platform admin inside tenant": {TenantID: &a.ID, Name: "x", Email: &email, PasswordHash: "x", Role: model.RolePlatformAdmin},
	}
	for name, u := range cases {
		if _, err := users.Create(context.Background(), u); err == nil {
			t.Errorf("%s: insert succeeded, want constraint failure", name)
		}
	}
}

func TestUserLookupsNeverCrossTenants(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)
	emailB := "only-b-" + uuid.NewString()[:8] + "@example.com"
	userB := newUser(t, users, &b.ID, model.RoleStudent, emailB)

	if _, err := users.FindByID(ctx, a.ID, userB.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID(A, userB): err = %v, want ErrNotFound", err)
	}
	if _, err := users.FindByIdentifier(ctx, a.ID, emailB); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByIdentifier(A, emailB): err = %v, want ErrNotFound", err)
	}
	if _, err := users.FindPlatformByIdentifier(ctx, emailB); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindPlatformByIdentifier(emailB): err = %v, want ErrNotFound", err)
	}
	if got, err := users.FindByIdentifier(ctx, b.ID, emailB); err != nil || got.ID != userB.ID {
		t.Errorf("FindByIdentifier(B, emailB) = %v, %v; want userB", got.ID, err)
	}
}

// uuid.Nil must never widen a query into "any tenant" or "platform".
func TestUserLookupsRequireScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUserRepository(pool)
	email := "plat-" + uuid.NewString()[:8] + "@example.com"
	platform := newUser(t, users, nil, model.RolePlatformAdmin, email)

	if _, err := users.FindByID(ctx, uuid.Nil, platform.ID); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("FindByID(Nil): err = %v, want ErrScopeRequired", err)
	}
	if _, err := users.FindByIdentifier(ctx, uuid.Nil, email); !errors.Is(err, ErrScopeRequired) {
		t.Errorf("FindByIdentifier(Nil): err = %v, want ErrScopeRequired", err)
	}
}

func TestFindPlatformByIDNeverLeavesThePlatformScope(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a := newTenant(t, tenants)

	platform := newUser(t, users, nil, model.RolePlatformAdmin, "plat-"+uuid.NewString()[:8]+"@example.com")
	tenantUser := newUser(t, users, &a.ID, model.RoleStudent, "tenant-"+uuid.NewString()[:8]+"@example.com")

	if got, err := users.FindPlatformByID(ctx, platform.ID); err != nil || got.ID != platform.ID {
		t.Errorf("FindPlatformByID(platform) = %v, %v; want the platform account", got.ID, err)
	}
	if _, err := users.FindPlatformByID(ctx, tenantUser.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindPlatformByID(tenant user): err = %v, want ErrNotFound", err)
	}
	if _, err := users.FindPlatformByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindPlatformByID(unknown): err = %v, want ErrNotFound", err)
	}
}

func TestUserIsActiveRoundTrips(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a := newTenant(t, tenants)
	u := newUser(t, users, &a.ID, model.RoleStudent, "active-"+uuid.NewString()[:8]+"@example.com")

	if !u.IsActive {
		t.Error("a new user is inactive, want active by default")
	}
	if got, err := users.FindByID(ctx, a.ID, u.ID); err != nil || !got.IsActive {
		t.Errorf("FindByID = active %v, err %v; want true, nil", got.IsActive, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET is_active = false WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if got, err := users.FindByID(ctx, a.ID, u.ID); err != nil || got.IsActive {
		t.Errorf("after deactivation FindByID = active %v, err %v; want false, nil", got.IsActive, err)
	}
}

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

func TestEmailCodeLifecycleIsScoped(t *testing.T) {
	pool := testPool(t)
	tenants, users := NewTenantRepository(pool), NewUserRepository(pool)
	a, b := newTenant(t, tenants), newTenant(t, tenants)
	user := newUser(t, users, &a.ID, model.RoleStudent, "code-"+uuid.NewString()[:8]+"@example.com")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	hash := testHash()

	if user.EmailVerifiedAt != nil {
		t.Fatal("a new account is verified before any code")
	}
	if err := inTx(t, pool, func(tx pgx.Tx) error { return users.SetEmailCodeTx(ctx, tx, &b.ID, user.ID, hash, now) }); !errors.Is(err, ErrNotFound) {
		t.Errorf("setting a code through another tenant: err = %v, want ErrNotFound", err)
	}
	if err := inTx(t, pool, func(tx pgx.Tx) error {
		if err := users.SetEmailCodeTx(ctx, tx, &a.ID, user.ID, hash, now.Add(time.Minute)); err != nil {
			return err
		}
		return users.CountEmailCodeAttemptTx(ctx, tx, &a.ID, user.ID)
	}); err != nil {
		t.Fatalf("set and count: %v", err)
	}
	var code model.EmailCode
	if err := inTx(t, pool, func(tx pgx.Tx) error {
		var err error
		code, err = users.FindEmailCodeForUpdateTx(ctx, tx, &a.ID, user.ID)
		return err
	}); err != nil {
		t.Fatalf("read code: %v", err)
	}
	if string(code.Hash) != string(hash) || code.Attempts != 1 || code.ExpiresAt == nil || code.VerifiedAt != nil {
		t.Errorf("code = %+v, want the hash, one attempt and an expiry", code)
	}
	if err := inTx(t, pool, func(tx pgx.Tx) error { return users.MarkEmailVerifiedTx(ctx, tx, &a.ID, user.ID, now) }); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	got, err := users.FindByID(ctx, a.ID, user.ID)
	if err != nil || got.EmailVerifiedAt == nil || !got.EmailVerifiedAt.Equal(now) {
		t.Errorf("verified at = %v, %v; want %s", got.EmailVerifiedAt, err, now)
	}
}

// The swap only succeeds against the hash the caller saw, which is what makes a reset link single use.
func TestReplacePasswordIsConditional(t *testing.T) {
	pool := testPool(t)
	tenant := newTenant(t, NewTenantRepository(pool))
	users := NewUserRepository(pool)
	user := newUser(t, users, &tenant.ID, model.RoleStudent, "pw-"+uuid.NewString()[:8]+"@example.com")
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	replace := func(scope *uuid.UUID, current, next string) bool {
		t.Helper()
		var ok bool
		if err := inTx(t, pool, func(tx pgx.Tx) error {
			var err error
			ok, err = users.ReplacePasswordTx(ctx, tx, scope, user.ID, current, next, now)
			return err
		}); err != nil {
			t.Fatalf("replace: %v", err)
		}
		return ok
	}
	if replace(nil, user.PasswordHash, "y") {
		t.Error("the platform scope replaced a tenant account's password")
	}
	if replace(&tenant.ID, "not-the-hash", "y") {
		t.Error("a stale hash replaced the password")
	}
	if !replace(&tenant.ID, user.PasswordHash, "y") {
		t.Fatal("the current hash did not replace the password")
	}
	if replace(&tenant.ID, user.PasswordHash, "z") {
		t.Error("the same old hash replaced the password twice")
	}
	got, _ := users.FindByID(ctx, tenant.ID, user.ID)
	if got.PasswordHash != "y" || got.EmailVerifiedAt == nil {
		t.Errorf("after reset: hash = %q, verified = %v; want y and verified", got.PasswordHash, got.EmailVerifiedAt)
	}
}
