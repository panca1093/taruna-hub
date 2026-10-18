package model

import (
	"time"

	"github.com/google/uuid"
)

// The ledger's kinds: an opening row explains a migrated balance, the rest are movements.
const (
	KindOpening    = "opening"
	KindTopup      = "topup"
	KindSpend      = "spend"
	KindAdjustment = "adjustment"
)

// Who moved the money; a nullable actor alone cannot tell a student from a provider from staff.
const (
	ActorSystem   = "system"
	ActorStudent  = "student"
	ActorProvider = "provider"
	ActorStaff    = "staff"
	ActorPlatform = "platform"
)

// WalletEntry is one immutable, signed movement; BalanceAfter is the projection just after it.
type WalletEntry struct {
	ID           uuid.UUID
	Seq          int64
	TenantID     uuid.UUID
	UserID       uuid.UUID
	Amount       string
	BalanceAfter string
	Kind         string
	ActorKind    string
	ActorID      *uuid.UUID
	// ActorName is the staff member's display name only, never their email or phone.
	ActorName  *string
	Reason     string
	SourceType string
	SourceID   string
	CreatedAt  time.Time
}

// Wallet is the projection with the two ledger totals, every value a canonical decimal string.
type Wallet struct {
	Balance      string
	TotalCredits string
	TotalDebits  string
}

// WalletStudent is one student row of the staff list.
type WalletStudent struct {
	ID        uuid.UUID
	Name      string
	Email     *string
	Phone     *string
	Balance   string
	CreatedAt time.Time
}
