package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func TestProvisionAdmin(t *testing.T) {
	tenants, pool := testTenantService(t)
	ctx := context.Background()
	users := NewUserService(repository.NewUserRepository(pool))
	tenant := provisionTenant(t, tenants, pool)

	// Every id this test creates is deleted by that exact id, never by a shared prefix.
	var owned []uuid.UUID
	t.Cleanup(func() {
		for _, id := range owned {
			if _, err := pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, id); err != nil {
				t.Errorf("cleanup sessions of user %s: %v", id, err)
			}
			if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup user %s: %v", id, err)
			}
		}
	})

	tenantAdmin, err := users.ProvisionAdmin(ctx, &tenant.ID, "Owner", "owner@example.com", "correct-horse")
	if err != nil {
		t.Fatalf("tenant admin: %v", err)
	}
	owned = append(owned, tenantAdmin.ID)
	if tenantAdmin.Role != model.RoleAdmin || tenantAdmin.TenantID == nil || *tenantAdmin.TenantID != tenant.ID {
		t.Errorf("tenant admin = role %s tenant %v; want admin in %s", tenantAdmin.Role, tenantAdmin.TenantID, tenant.ID)
	}
	if bcrypt.CompareHashAndPassword([]byte(tenantAdmin.PasswordHash), []byte("correct-horse")) != nil {
		t.Error("password is not stored as a bcrypt hash of the input")
	}

	email := "svc-plat-" + uuid.NewString()[:8] + "@example.com"
	platform, err := users.ProvisionAdmin(ctx, nil, "Root", email, "correct-horse")
	if err != nil {
		t.Fatalf("platform admin: %v", err)
	}
	owned = append(owned, platform.ID)
	if platform.Role != model.RolePlatformAdmin || platform.TenantID != nil {
		t.Errorf("platform admin = role %s tenant %v; want platform_admin, no tenant", platform.Role, platform.TenantID)
	}

	if _, err := users.ProvisionAdmin(ctx, &tenant.ID, "Owner", "owner@example.com", "correct-horse"); !errors.Is(err, ErrUserExists) {
		t.Errorf("duplicate: err = %v, want ErrUserExists", err)
	}
	for name, args := range map[string][3]string{
		"short password": {"A", "a@example.com", "short"},
		"blank name":     {" ", "a@example.com", "correct-horse"},
		"bad email":      {"A", "not-an-email", "correct-horse"},
	} {
		if _, err := users.ProvisionAdmin(ctx, &tenant.ID, args[0], args[1], args[2]); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}
