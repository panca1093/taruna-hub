package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrTenantNotFound = errors.New("tenant not found")
	ErrInvalidSlug    = errors.New("invalid tenant slug")
	ErrSlugTaken      = errors.New("tenant slug taken")
)

// FR-TEN-02 names www, api, admin, tryout; the rest are infrastructure names no tenant may claim.
var reservedSlugs = map[string]bool{
	"www": true, "api": true, "admin": true, "tryout": true,
	"app": true, "mail": true, "static": true, "assets": true, "cdn": true,
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,30}[a-z0-9])?$`)

// HostScope is the one tenant every request in this deployment runs under (D17).
type HostScope struct {
	Tenant model.Tenant
}

type TenantService struct {
	tenants *repository.TenantRepository
}

func NewTenantService(tenants *repository.TenantRepository) *TenantService {
	return &TenantService{tenants: tenants}
}

// Active loads the configured tenant; a missing or inactive one is ErrTenantNotFound so requests fail closed.
func (s *TenantService) Active(ctx context.Context, slug string) (HostScope, error) {
	tenant, err := s.BySlug(ctx, slug)
	if err != nil {
		return HostScope{}, err
	}
	if !tenant.IsActive {
		return HostScope{}, ErrTenantNotFound
	}
	return HostScope{Tenant: tenant}, nil
}

func (s *TenantService) BySlug(ctx context.Context, slug string) (model.Tenant, error) {
	tenant, err := s.tenants.FindBySlug(ctx, slug)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Tenant{}, ErrTenantNotFound
	}
	if err != nil {
		return model.Tenant{}, fmt.Errorf("find tenant by slug: %w", err)
	}
	return tenant, nil
}

func (s *TenantService) Provision(ctx context.Context, slug, name string) (model.Tenant, error) {
	name = strings.TrimSpace(name)
	if !validSlug(slug) || name == "" {
		return model.Tenant{}, ErrInvalidSlug
	}
	tenant, err := s.tenants.Create(ctx, slug, name)
	if errors.Is(err, repository.ErrDuplicate) {
		return model.Tenant{}, ErrSlugTaken
	}
	if err != nil {
		return model.Tenant{}, fmt.Errorf("create tenant: %w", err)
	}
	return tenant, nil
}

func validSlug(slug string) bool {
	return slugPattern.MatchString(slug) && !reservedSlugs[slug]
}
