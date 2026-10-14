package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleStudent       Role = "student"
	RoleOperator      Role = "operator"
	RoleAdmin         Role = "admin"
	RolePlatformAdmin Role = "platform_admin"
)

type User struct {
	ID           uuid.UUID
	TenantID     *uuid.UUID
	Name         string
	Email        *string
	Phone        *string
	PasswordHash string
	Role         Role
	Balance      float64
	IsActive     bool
	CreatedAt    time.Time
}

type Program struct {
	ID        uuid.UUID
	Code      string
	Name      string
	CreatedAt time.Time
}
