package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	TopupPending  = "pending"
	TopupApproved = "approved"
	TopupRejected = "rejected"
)

const TopupManual = "manual"

// WalletSourceTopup is the ledger source type of a credited top-up; the source id is the request id.
const WalletSourceTopup = "topup"

// DefaultMinTopup is what a tenant without a settings row uses (D12).
const DefaultMinTopup = "10000.00"

// Topup is one request for a wallet credit. The request is the evidence; the ledger entry is the money.
type Topup struct {
	ID           uuid.UUID
	Number       int64
	TenantID     uuid.UUID
	UserID       uuid.UUID
	StudentName  string
	Amount       string
	Method       string
	Status       string
	Key          string
	ProofAssetID *uuid.UUID
	RejectReason string
	ReviewedBy   *uuid.UUID
	ApprovedAt   *time.Time
	RejectedAt   *time.Time
	// RedirectURL is the provider's payment page of a gateway top-up while it is open.
	RedirectURL *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Settings are the tenant values an Admin changes.
type Settings struct {
	Weights  Weights
	MinTopup string
}
