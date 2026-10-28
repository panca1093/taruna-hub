package model

import (
	"time"

	"github.com/google/uuid"
)

// D04 is open, so the two modes are stored and nothing else: no weights, no fitted IRT.
const (
	ScoringWeighted = "weighted"
	ScoringIRT      = "irt"
)

// A draft may be incomplete; publishing validates the playable definition.
const (
	TryoutDraft     = "draft"
	TryoutPublished = "published"
)

// Assignment is the read model of the frozen version; it never carries options, correct or explanation.
type Assignment struct {
	QuestionID uuid.UUID
	VersionID  uuid.UUID
	Version    int
	Number     int64
	Type       string
	Difficulty string
	Content    string
	Position   int
}

type Subtest struct {
	ID              uuid.UUID
	Name            string
	DurationSeconds int
	Position        int
	Questions       []Assignment
}

type Tryout struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	ProgramID            uuid.UUID
	Title                string
	Description          string
	ScoringMode          string
	Status               string
	ActiveFrom           *time.Time
	ActiveUntil          time.Time
	ExplanationReleaseAt *time.Time
	RandomizeQuestions   bool
	RandomizeOptions     bool
	GraceSeconds         int
	Revision             int
	Locked               bool
	Subtests             []Subtest
	SubtestCount         int
	QuestionCount        int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// QuestionKey is what assigning needs to know about a question: its identity and its current version.
type QuestionKey struct {
	QuestionID uuid.UUID
	VersionID  uuid.UUID
	Version    int
}
