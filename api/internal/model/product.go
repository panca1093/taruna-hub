package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ProductIndividual = "individual"
	ProductBundle     = "bundle"
	ProductKecermatan = "kecermatan"
)

// PublicKecermatan is what the catalog shows of a kecermatan test; never a seed, key or content.
type PublicKecermatan struct {
	TestID               uuid.UUID
	ItemType             string
	OptionsPerSoal       int
	KolomCount           int
	SoalPerKolom         int
	KolomDurationSeconds int
}

// ProductTryout is a package member as staff read it: identity and counts, never content.
type ProductTryout struct {
	ID            uuid.UUID
	Title         string
	Status        string
	SubtestCount  int
	QuestionCount int
}

// PublicProductTryout adds the playable length the catalog shows.
type PublicProductTryout struct {
	ID                   uuid.UUID
	Title                string
	SubtestCount         int
	QuestionCount        int
	DurationSecondsTotal int
}

// Price is the canonical decimal string exactly as PostgreSQL stores it, never a float.
type Product struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Type          string
	Title         string
	Description   string
	ProgramID     uuid.UUID
	Price         string
	OriginalPrice *string
	IsActive      bool
	ActiveFrom    *time.Time
	ActiveUntil   *time.Time
	CreatedBy     uuid.UUID
	Tryouts       []ProductTryout
	// Kecermatan products sell included_attempts credits for one test and attach no tryouts.
	KecermatanTestID    *uuid.UUID
	KecermatanTestTitle string
	IncludedAttempts    *int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PublicProduct is the anonymous read model: display data only, no author and no revision.
type PublicProduct struct {
	ID               uuid.UUID
	Type             string
	Title            string
	Description      string
	ProgramID        uuid.UUID
	ProgramCode      string
	ProgramName      string
	Price            string
	OriginalPrice    *string
	DiscountPercent  *int
	ActiveUntil      *time.Time
	Tryouts          []PublicProductTryout
	IncludedAttempts *int
	Kecermatan       *PublicKecermatan
}

type CatalogSummary struct {
	Programs int
	Products int
	Tryouts  int
}

// TryoutAttach is what the service needs to know before a product may point at a tryout.
type TryoutAttach struct {
	Status    string
	ProgramID uuid.UUID
}
