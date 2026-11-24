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

// Points exist only in weighted mode; an option without points counts as 1 at scoring (P16).
// Label is the plain-text projection of Doc; the stored JSON carries both, and a row written before documents
// existed has no doc, which the repository fills from Label.
type QuestionOption struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Doc    RichDoc  `json:"doc"`
	Points *float64 `json:"points,omitempty"`
}

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
	Content        RichDoc
	Difficulty     string
	Explanation    RichDoc
	OptionWeighted bool
	CaseSensitive  bool
	Options        []QuestionOption
	Correct        []string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// RescoreAttempts is how many completed attempts an edit would rescore; only a single-question read fills it.
	RescoreAttempts int
}

// ImportRowError names the spreadsheet column a row failed on, with the service's reason code.
type ImportRowError struct {
	Field  string
	Reason string
}

// ImportRowPreview is the short summary of one row the confirm step shows.
type ImportRowPreview struct {
	Type       string
	Content    string
	Program    string
	Subject    string
	Difficulty string
}

// ImportRow is one data row of an uploaded file with its real validation result.
type ImportRow struct {
	Row     int
	Valid   bool
	Errors  []ImportRowError
	Preview *ImportRowPreview
}

type ImportPreview struct {
	FileName string
	Format   string
	Total    int
	Valid    int
	Invalid  int
	Rows     []ImportRow
}

type ImportCommit struct {
	Created         int
	AlreadyImported int
	QuestionIDs     []uuid.UUID
}
