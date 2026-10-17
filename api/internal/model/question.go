package model

import (
	"time"

	"github.com/google/uuid"
)

// The four question types of PRD §5.5.
const (
	QuestionPG        = "pg"
	QuestionPGMulti   = "pg_multi"
	QuestionIsian     = "isian"
	QuestionTrueFalse = "benar_salah"
)

// Difficulty is a label only: D04 keeps the per-difficulty weight out of this slice.
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// QuestionOption is one answer option. Points exists only in weighted mode, where an option without
// points counts as 1 at scoring time (P16); the id is assigned by the service and never rewritten.
type QuestionOption struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Points *float64 `json:"points,omitempty"`
}

// Question is the current version of one question: the head row's identity and ordering plus the
// immutable content of the version it points at.
type Question struct {
	ID             uuid.UUID
	Number         int64
	TenantID       uuid.UUID
	Version        int
	VersionID      uuid.UUID
	ProgramID      uuid.UUID
	SubjectID      uuid.UUID
	ChapterID      *uuid.UUID
	Type           string
	Content        string
	Difficulty     string
	Explanation    string
	OptionWeighted bool
	CaseSensitive  bool
	Options        []QuestionOption
	Correct        []string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
