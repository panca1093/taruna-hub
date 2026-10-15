package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

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

// e164 is the stored phone shape; Indonesian local forms are rewritten into it first.
var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// NormalizeEmail returns the stored form of an address: trimmed and lowercased, a bare addr-spec only.
func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return email, true
}

// NormalizePhone returns E.164: 0812…, 62812… and +62 812-… all become +62812….
func NormalizePhone(raw string) (string, bool) {
	phone := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '(', ')':
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(phone, "0"):
		phone = "+62" + phone[1:]
	case strings.HasPrefix(phone, "62"):
		phone = "+" + phone
	}
	if !e164.MatchString(phone) {
		return "", false
	}
	return phone, true
}

// NormalizeIdentifier reads the one login field as an email when it has an @, else as a phone.
func NormalizeIdentifier(raw string) (string, bool) {
	if strings.Contains(raw, "@") {
		return NormalizeEmail(raw)
	}
	return NormalizePhone(raw)
}

// validPassword is the shared rule: at least 8 characters, at most bcrypt's 72 bytes.
func validPassword(password string) bool {
	return utf8.RuneCountInString(password) >= minPasswordLength && len(password) <= maxPasswordBytes
}

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
	email, ok := NormalizeEmail(email)
	if !ok || name == "" || !validPassword(password) {
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
