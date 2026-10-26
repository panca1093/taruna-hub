package model

import (
	"time"

	"github.com/google/uuid"
)

// What an asset is for; each purpose has its own set of roles that may upload it.
const (
	PurposeQuestionImage = "question_image"
	PurposePaymentProof  = "payment_proof"
)

// A staged asset is an upload nothing references yet; a final one may be referenced by frozen content.
const (
	AssetStaged = "staged"
	AssetFinal  = "final"
)

// Asset is object metadata only; ObjectKey is where the bytes live and is never sent to a client.
type Asset struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Purpose     string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	Status      string
	UploadedBy  *uuid.UUID
	CreatedAt   time.Time
	FinalizedAt *time.Time
}
