package model

import (
	"time"

	"github.com/google/uuid"
)

// PaymentGateway is the method name on orders and top-ups; ProviderMidtrans is the only provider (D16).
const (
	PaymentGateway   = "gateway"
	TopupGateway     = "gateway"
	ProviderMidtrans = "midtrans"
)

const (
	AttemptPending   = "pending"
	AttemptFailed    = "failed"
	AttemptPaid      = "paid"
	AttemptCancelled = "cancelled"
	AttemptHeld      = "held"
)

const (
	ExceptionHeld     = "held"
	ExceptionCredited = "credited"
	ExceptionRefunded = "refunded"

	ExceptionLateSuccess     = "late_success"
	ExceptionAmountMismatch  = "amount_mismatch"
	ExceptionUnexpectedState = "unexpected_state"
)

// WalletSourceException is the ledger source type of a payment exception the staff credited.
const WalletSourceException = "payment_exception"

// PaymentAttempt is one provider payment for one order or top-up.
type PaymentAttempt struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	UserID          uuid.UUID
	ProviderOrderID string
	OrderID         *uuid.UUID
	TopupID         *uuid.UUID
	Amount          string
	Status          string
	RedirectURL     *string
	ExpiresAt       time.Time
	LastCheckedAt   *time.Time
	PaidAt          *time.Time
	CreatedAt       time.Time
}

// PaymentEvent is provider evidence as it arrived.
type PaymentEvent struct {
	AttemptID     uuid.UUID
	TenantID      uuid.UUID
	Source        string
	Status        string
	FraudStatus   string
	StatusCode    string
	GrossAmount   string
	TransactionID string
	BodyHash      string
	Raw           []byte
}

// PaymentException is a success held for staff review.
type PaymentException struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AttemptID     uuid.UUID
	UserID        uuid.UUID
	StudentName   string
	OrderID       *uuid.UUID
	OrderNumber   *int64
	TopupID       *uuid.UUID
	TopupNumber   *int64
	Kind          string
	PaidAmount    string
	Status        string
	ResolvedBy    *uuid.UUID
	ResolveReason string
	ResolvedAt    *time.Time
	CreatedAt     time.Time
}
