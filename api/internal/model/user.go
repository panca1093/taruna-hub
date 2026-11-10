package model

import (
	"hash/fnv"
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
	// AvatarKey is the built-in avatar the user picked; nil means DefaultAvatarKey chooses.
	AvatarKey *string
	Gender    *string
	BirthDate *time.Time
}

const (
	GenderMale   = "male"
	GenderFemale = "female"

	avatarYoungBelowAge = 25
)

// TODO: uncertain — groups were sorted by eye from a low-resolution sheet, verify with the owner.
var (
	avatarsMaleYoung   = []string{"a05", "a10", "a12", "a15", "a18", "a19", "a23", "a24"}
	avatarsMaleAdult   = []string{"a03", "a04", "a09", "a13", "a17", "a22", "a26", "a29", "a30", "a35"}
	avatarsFemaleYoung = []string{"a01", "a02", "a11", "a16", "a21", "a27", "a32", "a34"}
	avatarsFemaleAdult = []string{"a06", "a07", "a08", "a14", "a20", "a25", "a28", "a31", "a33"}
)

// DefaultAvatarKey is the avatar of a user who has not picked one: narrowed by gender and age when known,
// and always the same for one user because the id picks within the group.
func DefaultAvatarKey(id uuid.UUID, gender *string, birthDate *time.Time, now time.Time) string {
	male, female := gender == nil || *gender == GenderMale, gender == nil || *gender == GenderFemale
	young, adult := true, true
	if birthDate != nil {
		young = AgeAt(*birthDate, now) < avatarYoungBelowAge
		adult = !young
	}
	var pool []string
	for _, group := range []struct {
		in   bool
		keys []string
	}{
		{male && young, avatarsMaleYoung}, {male && adult, avatarsMaleAdult},
		{female && young, avatarsFemaleYoung}, {female && adult, avatarsFemaleAdult},
	} {
		if group.in {
			pool = append(pool, group.keys...)
		}
	}
	hash := fnv.New32a()
	hash.Write(id[:])
	return pool[hash.Sum32()%uint32(len(pool))]
}

// AgeAt is the number of whole years lived on the day of now.
func AgeAt(birthDate, now time.Time) int {
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
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
