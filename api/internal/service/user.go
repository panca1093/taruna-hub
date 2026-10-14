package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidInput = errors.New("invalid input")
)

const minPasswordLength = 8

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) ByID(ctx context.Context, tenantID, id uuid.UUID) (model.User, error) {
	u, err := s.users.FindByID(ctx, tenantID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (s *UserService) ByIdentifier(ctx context.Context, tenantID uuid.UUID, identifier string) (model.User, error) {
	u, err := s.users.FindByIdentifier(ctx, tenantID, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by identifier: %w", err)
	}
	return u, nil
}

func (s *UserService) PlatformByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	u, err := s.users.FindPlatformByIdentifier(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find platform user by identifier: %w", err)
	}
	return u, nil
}

// ProvisionAdmin creates a tenant's first admin (or a Platform Admin when tenantID is nil); bootstrap command only.
func (s *UserService) ProvisionAdmin(ctx context.Context, tenantID *uuid.UUID, name, email, password string) (model.User, error) {
	name = strings.TrimSpace(name)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || name == "" || len(password) < minPasswordLength {
		return model.User{}, ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	role := model.RolePlatformAdmin
	if tenantID != nil {
		role = model.RoleAdmin
	}
	u, err := s.users.Create(ctx, model.User{
		TenantID: tenantID, Name: name, Email: &email, PasswordHash: string(hash), Role: role,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return model.User{}, ErrUserExists
	}
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}
