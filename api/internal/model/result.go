package model

import (
	"time"

	"github.com/google/uuid"
)

// Standing is one student's place among a tryout's participants. Average is the mean of the student's completed-attempt
// scores rounded to 2 decimals: the value that is displayed and the value ties are judged on.
type Standing struct {
	UserID   uuid.UUID
	Name     string
	Average  float64
	Attempts int
	Rank     int
}

// Ranking is the live rank of one student on one tryout. It is computed on every read and never stored.
type Ranking struct {
	Rank         int
	Participants int
	Average      float64
	Attempts     int
}

// StaffResult is one completed attempt as Admin and Operator list it, with the student's current rank.
type StaffResult struct {
	SessionID     uuid.UUID
	TryoutID      uuid.UUID
	TryoutTitle   string
	ProgramCode   string
	ScoringMode   string
	UserID        uuid.UUID
	StudentName   string
	AttemptNumber int
	Score         float64
	Average       float64
	Rank          int
	Participants  int
	IRTWarning    bool
	CompletedAt   time.Time
	Subtests      []PartResult
}

// ReviewQuestion is a snapshotted question with its explanation. Correct is already on AttemptQuestion; both only
// leave the service after the tryout's release time.
type ReviewQuestion struct {
	AttemptQuestion
	Explanation string
}
