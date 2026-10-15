package model

import (
	"time"

	"github.com/google/uuid"
)

// Only the SHA-256 of the refresh token is stored; the token itself lives in the browser cookie alone.
type Session struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	TenantID       *uuid.UUID
	RefreshHash    []byte
	CreatedAt      time.Time
	LastActivityAt time.Time
	RevokedAt      *time.Time
	// PreviousRefreshHash is the token the last rotation replaced; replaying it is how theft shows.
	PreviousRefreshHash []byte
	RotatedAt           *time.Time
}

// The tenant is nil only for a Platform Admin; balance is deliberately not part of identity.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	TenantID  *uuid.UUID
	Name      string
	Email     *string
	Phone     *string
	Role      Role
}

// Registration is a signup waiting for its email code (D01); it becomes a User only once verified.
type Registration struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	Phone        *string
	Name         string
	PasswordHash string
	CodeID       uuid.UUID
	CodeHash     []byte
	Attempts     int
	ExpiresAt    time.Time
	SentAt       time.Time
	CreatedAt    time.Time
}

// PasswordReset is a single-use recovery credential; only the SHA-256 of its token is stored.
type PasswordReset struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TenantID   *uuid.UUID
	TokenHash  []byte
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}
