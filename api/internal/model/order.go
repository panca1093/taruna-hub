package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	OrderPending   = "pending"
	OrderConfirmed = "confirmed"
	OrderCancelled = "cancelled"
)

const (
	PaymentManual = "manual"
	PaymentWallet = "wallet"
)

const (
	GrantSourceOrder = "order"
	GrantSourceStaff = "staff"
)

// OrderItem is one tryout of the package as it was named at checkout.
type OrderItem struct {
	TryoutID uuid.UUID
	Title    string
}

// Order is the checkout snapshot: later product edits or deletes never change what was bought.
type Order struct {
	ID            uuid.UUID
	Number        int64
	TenantID      uuid.UUID
	UserID        uuid.UUID
	StudentName   string
	ProductID     uuid.UUID
	ProductType   string
	ProductTitle  string
	Price         string
	PaymentMethod string
	Status        string
	Key           string
	ProofAssetID  *uuid.UUID
	CancelReason  string
	ReviewedBy    *uuid.UUID
	ConfirmedAt   *time.Time
	CancelledAt   *time.Time
	Items         []OrderItem
	// A kecermatan order has no items; it snapshots the test and the credits it adds.
	KecermatanTestID *uuid.UUID
	IncludedAttempts *int
	// Gateway orders carry the provider's payment page while it is open, and the held exception, if any.
	RedirectURL     *string
	ExceptionID     *uuid.UUID
	ExceptionKind   *string
	ExceptionAmount *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Enrollment is what a student holds in one tryout. The window is the tryout's own (D05a), read live.
type Enrollment struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	UserID            uuid.UUID
	TryoutID          uuid.UUID
	TryoutTitle       string
	ProgramCode       string
	ProgramName       string
	SubtestCount      int
	TryoutActiveFrom  *time.Time
	TryoutActiveUntil time.Time
	AttemptsPurchased *int
	AttemptsUsed      int
	CreatedAt         time.Time
}

// EnrollmentGrant records why an enrollment exists; a staff grant keeps its actor and reason.
type EnrollmentGrant struct {
	EnrollmentID uuid.UUID
	OrderID      *uuid.UUID
	SourceType   string
	SourceID     string
	ActorKind    string
	ActorID      *uuid.UUID
	Reason       string
	// Attempts is the credit a kecermatan grant adds; nil for a tryout grant.
	Attempts *int
}

// Contact is where an order's notifications go.
type Contact struct {
	Name  string
	Email *string
	Phone *string
}
