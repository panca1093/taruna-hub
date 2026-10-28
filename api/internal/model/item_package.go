package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ItemSymbol = "symbol"
	ItemImage  = "image"
	ItemEmoji  = "emoji"
	ItemShape  = "shape"
)

// ItemPackage is one named element pool. Revision is the current frozen revision; a test pins one.
type ItemPackage struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Type         string
	Name         string
	Revision     int
	ElementCount int
	// RequiredCount is the largest pool size any referencing test needs; zero means unused.
	RequiredCount int
	InUse         bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ItemElement is one member of a package revision. Key is its dedup identity: the symbol, the emoji or
// shape id, or the image's SHA-256. Label is dashboard-only and never reaches a student.
type ItemElement struct {
	ID       uuid.UUID
	Position int
	Kind     string
	Key      string
	Value    string
	Label    string
	AssetID  *uuid.UUID
}

// ElementCheck is the verdict on one candidate element of an upload.
type ElementCheck struct {
	Label    string
	Kind     string
	Key      string
	Accepted bool
	Reason   string
}

type ElementPreview struct {
	Accepted int
	Rejected int
	Items    []ElementCheck
}

type ElementCommit struct {
	Added          int
	AlreadyPresent int
	Rejected       int
	Revision       int
}
