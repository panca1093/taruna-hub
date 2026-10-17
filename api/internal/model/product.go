package model

import (
	"time"

	"github.com/google/uuid"
)

// Two product types ship now; kecermatan has no test entity yet, so it is simply not offered.
const (
	ProductIndividual = "individual"
	ProductBundle     = "bundle"
)

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
	ActiveUntil   *time.Time
	CreatedBy     uuid.UUID
	Tryouts       []ProductTryout
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PublicProduct is the anonymous read model: display data only, no author and no revision.
type PublicProduct struct {
	ID              uuid.UUID
	Type            string
	Title           string
	Description     string
	ProgramID       uuid.UUID
	ProgramCode     string
	ProgramName     string
	Price           string
	OriginalPrice   *string
	DiscountPercent *int
	ActiveUntil     *time.Time
	Tryouts         []PublicProductTryout
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
