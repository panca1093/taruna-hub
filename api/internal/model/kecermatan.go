package model

import (
	"time"

	"github.com/google/uuid"
)

// Item types of a kecermatan test (FR-ADM-KEC-07). char and number are built in, the packaged types draw
// from an item package, shape is built in with an optional package subset, and mix combines sources.
const (
	KecChar   = "char"
	KecNumber = "number"
	KecSymbol = "symbol"
	KecEmoji  = "emoji"
	KecImage  = "image"
	KecShape  = "shape"
	KecMix    = "mix"
)

// KecermatanElement is one drawable member of a generation pool. Key is the dedup identity inside its kind
// (the symbol, the emoji or shape id, an image's content hash).
type KecermatanElement struct {
	Kind    string     `json:"kind"`
	Key     string     `json:"key"`
	Value   string     `json:"value"`
	AssetID *uuid.UUID `json:"asset_id,omitempty"`
}

// KecermatanPackageRef is a package a test pins at one frozen revision.
type KecermatanPackageRef struct {
	PackageID     uuid.UUID
	Name          string
	Type          string
	Revision      int
	ElementCount  int
	RequiredCount int
}

// KecermatanTest is the frozen generation configuration. Seed never reaches a response: it is what
// reproduces the generated columns, so it is as sensitive as the answer keys.
type KecermatanTest struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	ProgramID            uuid.UUID
	Title                string
	Description          string
	ItemType             string
	ItemLength           int
	OptionsPerSoal       int
	SoalPerKolom         int
	KolomCount           int
	KolomDurationSeconds int
	BuiltinSources       []string
	CoverFrom            string
	CoverTo              string
	Seed                 string
	Revision             int
	Locked               bool
	Packages             []KecermatanPackageRef
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
