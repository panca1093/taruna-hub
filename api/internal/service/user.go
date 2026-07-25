package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) ByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	u, err := s.users.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (s *UserService) ByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	u, err := s.users.FindByIdentifier(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by identifier: %w", err)
	}
	return u, nil
}
