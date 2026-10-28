package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	SessionInProgress = "in_progress"
	SessionCompleted  = "completed"
)

const (
	PartPending   = "pending"
	PartRunning   = "running"
	PartSubmitted = "submitted"
)

// Why a part ended: the student pressed submit, the deadline passed, or it never started before the session closed.
const (
	SubmitStudent   = "student"
	SubmitExpired   = "expired"
	SubmitUnstarted = "unstarted"
)

// ScoringVersion is stored beside every result, so a later policy change never rewrites an old score.
const ScoringVersion = 1

// Weights are the D04 per-difficulty points; a session freezes its own copy at attempt start.
type Weights struct {
	Easy   float64
	Medium float64
	Hard   float64
}

// DefaultWeights are what a tenant without a settings row uses (D04).
var DefaultWeights = Weights{Easy: 1, Medium: 2, Hard: 3}

func (w Weights) Of(difficulty string) float64 {
	switch difficulty {
	case DifficultyMedium:
		return w.Medium
	case DifficultyHard:
		return w.Hard
	}
	return w.Easy
}

// Attempt is one session of one student on one tryout. The tryout fields are read live.
type Attempt struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	TryoutID       uuid.UUID
	AttemptNumber  int
	Status         string
	ScoringMode    string
	ScoringVersion int
	Weights        Weights
	Seed           int64
	StartedAt      time.Time
	CompletedAt    *time.Time

	TryoutTitle  string
	ActiveUntil  time.Time
	GraceSeconds int
}

// AttemptPart is one subtest of a session: its own status, absolute deadline and, once settled, its raw score.
type AttemptPart struct {
	SessionID       uuid.UUID
	SubtestID       uuid.UUID
	Position        int
	Name            string
	DurationSeconds int
	Status          string
	StartedAt       *time.Time
	DeadlineAt      *time.Time
	ExtraSeconds    int
	SubmittedAt     *time.Time
	SubmitReason    string
	QuestionCount   int
	EarnedPoints    *float64
	MaxPoints       *float64
	CorrectCount    *int
}

// AttemptQuestion is a snapshotted question in the order this student sees it. Options are already in the
// student's order; Correct and the points stay server-side.
type AttemptQuestion struct {
	SessionID      uuid.UUID
	SubtestID      uuid.UUID
	Position       int
	QuestionID     uuid.UUID
	VersionID      uuid.UUID
	Type           string
	Difficulty     string
	Content        string
	OptionWeighted bool
	CaseSensitive  bool
	Options        []QuestionOption
	Correct        []string
}

type AttemptAnswer struct {
	QuestionID uuid.UUID
	Answer     []string
	Revision   int
	Seq        int64
	UpdatedAt  time.Time
}

// SourceQuestion is what admission needs from the frozen assignment to snapshot it.
type SourceQuestion struct {
	SubtestID  uuid.UUID
	QuestionID uuid.UUID
	VersionID  uuid.UUID
	Type       string
	OptionIDs  []string
}

// SourceSubtest is one subtest as the tryout defines it, in tryout order.
type SourceSubtest struct {
	ID              uuid.UUID
	Name            string
	DurationSeconds int
	Position        int
	Questions       []SourceQuestion
}

// OverviewSubtest is a subtest as the overview lists it, before any session exists.
type OverviewSubtest struct {
	ID              uuid.UUID
	Name            string
	DurationSeconds int
	Position        int
	QuestionCount   int
}

// AttemptSummary is the history row: the score is nil until the attempt has a result.
type AttemptSummary struct {
	SessionID     uuid.UUID
	TryoutID      uuid.UUID
	AttemptNumber int
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	Score         *float64
}

type PartResult struct {
	SubtestID     uuid.UUID
	Name          string
	Position      int
	EarnedPoints  float64
	MaxPoints     float64
	Score         float64
	CorrectCount  int
	QuestionCount int
	SubmitReason  string
}

// Result is the one persisted outcome of a completed session.
type Result struct {
	SessionID      uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	TryoutID       uuid.UUID
	AttemptNumber  int
	EarnedPoints   float64
	MaxPoints      float64
	Score          float64
	ScoringMode    string
	ScoringVersion int
	CompletedAt    time.Time
	Parts          []PartResult
}

// Extension is one audited staff grant of extra time to the subtest that was running in one session.
type Extension struct {
	SessionID  uuid.UUID
	SubtestID  uuid.UUID
	Seconds    int
	Reason     string
	GrantedBy  uuid.UUID
	TotalExtra int
}
