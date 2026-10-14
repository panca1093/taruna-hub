package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
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
		_, _ = repo.db.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, tenant.ID)
		_, _ = repo.db.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID)
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
	t.Cleanup(func() { _, _ = repo.db.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u.ID) })
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
