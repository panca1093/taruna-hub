package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func newUserAdminHarness(t *testing.T) (walletHarness, *UserAdminService) {
	t.Helper()
	h := newWalletHarness(t)
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM audit_events WHERE tenant_id = $1`, h.tenant.ID); err != nil {
			t.Errorf("cleanup audit events: %v", err)
		}
	})
	svc := NewUserAdminService(h.pool, repository.NewUserRepository(h.pool), repository.NewSessionRepository(h.pool), nil, WithUserAdminCost(4))
	return h, svc
}

func TestUserAdminRefusesNonStaffBeforeAnyQuery(t *testing.T) {
	h, svc := newUserAdminHarness(t)
	ctx := context.Background()
	scope, p := h.scope(h.tenant), h.principal(h.tenant, h.student)
	calls := map[string]func() error{
		"List":   func() error { _, _, err := svc.List(ctx, scope, p, UserListInput{}); return err },
		"Get":    func() error { _, err := svc.Get(ctx, scope, p, h.second.ID); return err },
		"Access": func() error { _, _, err := svc.Access(ctx, scope, p, h.second.ID, 10, 0); return err },
		"Grant":  func() error { _, err := svc.Grant(ctx, scope, p, h.second.ID, GrantInput{}); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrUserForbidden) {
			t.Errorf("%s as student = %v, want ErrUserForbidden", name, err)
		}
	}
}

func TestDeactivatingTheLastAdminIsRefused(t *testing.T) {
	h, svc := newUserAdminHarness(t)
	ctx := context.Background()
	scope := h.scope(h.tenant)
	users := repository.NewUserRepository(h.pool)
	second := walletUser(t, users, h.tenant.ID, model.RoleAdmin, "Admin Dua")

	if _, err := svc.Deactivate(ctx, scope, h.principal(h.tenant, h.admin), second.ID, "pindah"); err != nil {
		t.Fatalf("deactivate with two admins: %v", err)
	}
	third := walletUser(t, users, h.tenant.ID, model.RoleAdmin, "Admin Tiga")
	if _, err := svc.Deactivate(ctx, scope, h.principal(h.tenant, third), h.admin.ID, "x"); err != nil {
		t.Fatalf("deactivate admin with another active: %v", err)
	}
	if _, err := svc.Deactivate(ctx, scope, h.principal(h.tenant, h.admin), third.ID, "x"); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("deactivating the last admin = %v, want ErrLastAdmin", err)
	}
}

func TestConcurrentCrossDeactivationKeepsOneAdmin(t *testing.T) {
	h, svc := newUserAdminHarness(t)
	ctx := context.Background()
	scope := h.scope(h.tenant)
	b := walletUser(t, repository.NewUserRepository(h.pool), h.tenant.ID, model.RoleAdmin, "Admin Dua")

	var wg sync.WaitGroup
	for _, pair := range [][2]model.User{{h.admin, b}, {b, h.admin}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Deactivate(ctx, scope, h.principal(h.tenant, pair[0]), pair[1].ID, "x")
		}()
	}
	wg.Wait()
	var active int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1 AND role = 'admin' AND is_active`, h.tenant.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active < 1 {
		t.Errorf("%d active admins, want at least 1", active)
	}
}
