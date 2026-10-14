package service

import (
	"context"
	"errors"
	"fmt"
	"net"
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

// platformLabel is the one reserved label that selects the platform scope instead of a tenant.
const platformLabel = "admin"

// FR-TEN-02 names www, api, admin, tryout; the rest are infrastructure names no tenant may claim.
var reservedSlugs = map[string]bool{
	"www": true, "api": true, "admin": true, "tryout": true,
	"app": true, "mail": true, "static": true, "assets": true, "cdn": true,
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,30}[a-z0-9])?$`)

// HostScope is what a request host resolves to: exactly one tenant, or the platform.
type HostScope struct {
	Tenant   model.Tenant
	Platform bool
}

type TenantService struct {
	tenants    *repository.TenantRepository
	baseDomain string
}

func NewTenantService(tenants *repository.TenantRepository, baseDomain string) *TenantService {
	return &TenantService{tenants: tenants, baseDomain: strings.ToLower(baseDomain)}
}

// ResolveHost maps a host to its scope; every failure is ErrTenantNotFound so hosts are indistinguishable.
func (s *TenantService) ResolveHost(ctx context.Context, host string) (HostScope, error) {
	label, ok := hostLabel(host, s.baseDomain)
	if !ok {
		return HostScope{}, ErrTenantNotFound
	}
	if label == platformLabel {
		return HostScope{Platform: true}, nil
	}
	if reservedSlugs[label] {
		return HostScope{}, ErrTenantNotFound
	}

	tenant, err := s.tenants.FindBySlug(ctx, label)
	if errors.Is(err, repository.ErrNotFound) {
		return HostScope{}, ErrTenantNotFound
	}
	if err != nil {
		return HostScope{}, fmt.Errorf("find tenant by slug: %w", err)
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

// hostLabel matches the suffix with its leading dot, so look-alike hosts never resolve.
func hostLabel(host, baseDomain string) (string, bool) {
	host = strings.ToLower(host)
	if h, port, err := net.SplitHostPort(host); err == nil {
		if port == "" || strings.Trim(port, "0123456789") != "" {
			return "", false
		}
		host = h
	} else if strings.Contains(host, ":") {
		return "", false
	}
	host = strings.TrimSuffix(host, ".")

	label, ok := strings.CutSuffix(host, "."+baseDomain)
	if !ok || !slugPattern.MatchString(label) {
		return "", false
	}
	return label, true
}
