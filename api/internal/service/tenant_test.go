package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func TestValidSlug(t *testing.T) {
	for _, slug := range []string{"taruna", "sekolah-a", "a", "abc123", strings.Repeat("a", 32)} {
		if !validSlug(slug) {
			t.Errorf("validSlug(%q) = false, want true", slug)
		}
	}
	for _, slug := range []string{"", "Taruna", "-a", "a-", "a_b", "a.b", "www", "api", "admin", "tryout", strings.Repeat("a", 33)} {
		if validSlug(slug) {
			t.Errorf("validSlug(%q) = true, want false", slug)
		}
	}
}

// testRedis is the throttle store; like the database, it is required and never mocked.
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

func testTenantService(t *testing.T) (*TenantService, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	// Built with pgxpool directly: infra imports service, so importing infra here would cycle.
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewTenantService(repository.NewTenantRepository(pool)), pool
}

// Tests share one database and run in parallel, so nothing may be deleted by name or prefix.
func provisionTenant(t *testing.T, svc *TenantService, pool *pgxpool.Pool) model.Tenant {
	t.Helper()
	tenant, err := svc.Provision(context.Background(), "t-"+uuid.NewString()[:8], "svc-test")
	if err != nil {
		t.Fatalf("provision tenant: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		// Rows that reference the tenant or its users go first, or the deletes below would fail.
		for _, table := range []string{"notification_logs"} {
			if _, err := pool.Exec(ctx, `DELETE FROM `+table+` WHERE tenant_id = $1`, tenant.ID); err != nil {
				t.Errorf("cleanup %s of tenant %s: %v", table, tenant.ID, err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM auth_sessions WHERE tenant_id = $1`, tenant.ID); err != nil {
			t.Errorf("cleanup sessions of tenant %s: %v", tenant.ID, err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = $1`, tenant.ID); err != nil {
			t.Errorf("cleanup users of tenant %s: %v", tenant.ID, err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
			t.Errorf("cleanup tenant %s: %v", tenant.ID, err)
		}
	})
	return tenant
}

func TestActiveLoadsTheConfiguredTenant(t *testing.T) {
	svc, pool := testTenantService(t)
	ctx := context.Background()
	tenant := provisionTenant(t, svc, pool)

	got, err := svc.Active(ctx, tenant.Slug)
	if err != nil || got.Tenant.ID != tenant.ID {
		t.Fatalf("Active(%q) = %+v, %v; want tenant %s", tenant.Slug, got, err, tenant.ID)
	}
	for _, slug := range []string{"nope-" + tenant.Slug, ""} {
		if _, err := svc.Active(ctx, slug); !errors.Is(err, ErrTenantNotFound) {
			t.Errorf("Active(%q): err = %v, want ErrTenantNotFound", slug, err)
		}
	}
}

func TestActiveSkipsInactiveTenant(t *testing.T) {
	svc, pool := testTenantService(t)
	ctx := context.Background()
	tenant := provisionTenant(t, svc, pool)
	if _, err := pool.Exec(ctx, `UPDATE tenants SET is_active = false WHERE id = $1`, tenant.ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.Active(ctx, tenant.Slug); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("inactive tenant: err = %v, want ErrTenantNotFound", err)
	}
}

func TestProvisionRejectsBadAndDuplicateSlugs(t *testing.T) {
	svc, pool := testTenantService(t)
	ctx := context.Background()

	for _, slug := range []string{"www", "Bad_Slug", ""} {
		if _, err := svc.Provision(ctx, slug, "svc-test"); !errors.Is(err, ErrInvalidSlug) {
			t.Errorf("Provision(%q): err = %v, want ErrInvalidSlug", slug, err)
		}
	}

	tenant := provisionTenant(t, svc, pool)
	if _, err := svc.Provision(ctx, tenant.Slug, "svc-test"); !errors.Is(err, ErrSlugTaken) {
		t.Errorf("duplicate slug: err = %v, want ErrSlugTaken", err)
	}
}
