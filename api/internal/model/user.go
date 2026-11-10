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
	// Balance is the wallet projection, read as the canonical decimal string it is stored as.
	Balance   string
	IsActive  bool
	CreatedAt time.Time
	// EmailVerifiedAt is nil until the owner proves the inbox, by a code or a password reset.
	EmailVerifiedAt *time.Time
	// AvatarKey is the built-in avatar the user picked; nil leaves the choice to the web app.
	AvatarKey *string
}

// ValidAvatarKey reports whether key names one of the thirty-five built-in avatars, a01 to a35.
func ValidAvatarKey(key string) bool {
	if len(key) != 3 || key[0] != 'a' || key[1] < '0' || key[1] > '3' || key[2] < '0' || key[2] > '9' {
		return false
	}
	n := int(key[1]-'0')*10 + int(key[2]-'0')
	return n >= 1 && n <= 35
}

const (
	AuditUserCreate     = "user.create"
	AuditUserUpdate     = "user.update"
	AuditUserDeactivate = "user.deactivate"
	AuditUserReactivate = "user.reactivate"
	AuditGrantManual    = "grant.manual"

	AuditTargetUser       = "user"
	AuditTargetEnrollment = "enrollment"
)

// AuditEvent keeps the staff actor of an account or grant action; it is never edited.
type AuditEvent struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	ActorID    uuid.UUID
	ActorRole  Role
	Action     string
	TargetType string
	TargetID   uuid.UUID
	Reason     string
	Detail     map[string]string
	CreatedAt  time.Time
}

const (
	AccessTryout     = "tryout"
	AccessKecermatan = "kecermatan"
)

// StudentAccess is one thing a student holds. Active is the server's answer for the moment of the read:
// a published tryout inside its window, or a kecermatan test with credits left.
type StudentAccess struct {
	EnrollmentID      uuid.UUID
	Kind              string
	TestID            uuid.UUID
	Title             string
	ActiveFrom        *time.Time
	ActiveUntil       *time.Time
	AttemptsPurchased *int
	AttemptsUsed      int
	Active            bool
	// SourceType is how access was first opened: "order" or "staff".
	SourceType string
	CreatedAt  time.Time
}
