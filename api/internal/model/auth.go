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
