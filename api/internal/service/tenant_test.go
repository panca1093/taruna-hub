package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func TestHostLabel(t *testing.T) {
	cases := []struct {
		host      string
		wantLabel string
		wantOK    bool
	}{
		{"taruna.tarunaeduka.id", "taruna", true},
		{"TARUNA.TarunaEduka.ID", "taruna", true},
		{"taruna.tarunaeduka.id:3000", "taruna", true},
		{"taruna.tarunaeduka.id.", "taruna", true},
		{"a1.tarunaeduka.id", "a1", true},
		{"tarunaeduka.id", "", false},
		{"", "", false},
		{"taruna.tarunaeduka.id.evil.com", "", false},
		{"taruna.tarunaeduka.idx", "", false},
		{"taruna-tarunaeduka.id", "", false},
		{"nottarunaeduka.id", "", false},
		{"evil.com", "", false},
		{"a.b.tarunaeduka.id", "", false},
		{".tarunaeduka.id", "", false},
		{"-bad.tarunaeduka.id", "", false},
		{"bad-.tarunaeduka.id", "", false},
		{"under_score.tarunaeduka.id", "", false},
		{"xn--e1afmkfd.tarunaeduka.id", "xn--e1afmkfd", true},
		{"taruna.tarunaeduka.id:abc", "", false},
		{"127.0.0.1", "", false},
		{"[::1]:8081", "", false},
		{"taruna.tarunaeduka.id, evil.com", "", false},
		{strings.Repeat("a", 33) + ".tarunaeduka.id", "", false},
	}
	for _, tc := range cases {
		label, ok := hostLabel(tc.host, "tarunaeduka.id")
		if label != tc.wantLabel || ok != tc.wantOK {
			t.Errorf("hostLabel(%q) = %q, %v; want %q, %v", tc.host, label, ok, tc.wantLabel, tc.wantOK)
		}
	}
}

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
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE name = 'svc-test'`) })
	return NewTenantService(repository.NewTenantRepository(pool), "tarunaeduka.id"), pool
}

func TestResolveHost(t *testing.T) {
	svc, _ := testTenantService(t)
	ctx := context.Background()
	slug := "t-" + uuid.NewString()[:8]
	tenant, err := svc.Provision(ctx, slug, "svc-test")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	got, err := svc.ResolveHost(ctx, slug+".tarunaeduka.id:3000")
	if err != nil || got.Platform || got.Tenant.ID != tenant.ID {
		t.Fatalf("ResolveHost(tenant) = %+v, %v; want tenant %s", got, err, tenant.ID)
	}

	got, err = svc.ResolveHost(ctx, "admin.tarunaeduka.id")
	if err != nil || !got.Platform {
		t.Fatalf("ResolveHost(admin) = %+v, %v; want platform", got, err)
	}

	for _, host := range []string{"nope-" + slug + ".tarunaeduka.id", "tarunaeduka.id", "www.tarunaeduka.id", slug + ".tarunaeduka.id.evil.com"} {
		if _, err := svc.ResolveHost(ctx, host); !errors.Is(err, ErrTenantNotFound) {
			t.Errorf("ResolveHost(%q): err = %v, want ErrTenantNotFound", host, err)
		}
	}
}

func TestResolveHostSkipsInactiveTenant(t *testing.T) {
	svc, pool := testTenantService(t)
	ctx := context.Background()
	slug := "t-" + uuid.NewString()[:8]
	if _, err := svc.Provision(ctx, slug, "svc-test"); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET is_active = false WHERE slug = $1`, slug); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.ResolveHost(ctx, slug+".tarunaeduka.id"); !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("inactive tenant: err = %v, want ErrTenantNotFound", err)
	}
}

func TestProvisionRejectsBadAndDuplicateSlugs(t *testing.T) {
	svc, _ := testTenantService(t)
	ctx := context.Background()

	for _, slug := range []string{"www", "Bad_Slug", ""} {
		if _, err := svc.Provision(ctx, slug, "svc-test"); !errors.Is(err, ErrInvalidSlug) {
			t.Errorf("Provision(%q): err = %v, want ErrInvalidSlug", slug, err)
		}
	}

	slug := "t-" + uuid.NewString()[:8]
	if _, err := svc.Provision(ctx, slug, "svc-test"); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	if _, err := svc.Provision(ctx, slug, "svc-test"); !errors.Is(err, ErrSlugTaken) {
		t.Errorf("duplicate slug: err = %v, want ErrSlugTaken", err)
	}
}
