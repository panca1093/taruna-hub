package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ViolationExitFullscreen    = "exit_fullscreen"
	ViolationScreenshotAttempt = "screenshot_attempt"
)

// Violation is one piece of evidence about a tryout session. OccurredAt is the server's clock, never the browser's.
type Violation struct {
	ID          uuid.UUID
	SessionID   uuid.UUID
	SubtestID   uuid.UUID
	SubtestName string
	Type        string
	EventID     uuid.UUID
	OccurredAt  time.Time
}

// ActiveSession is one in-progress tryout session as monitoring lists it. Subtest is nil between subtests, when no
// part is running.
type ActiveSession struct {
	SessionID      uuid.UUID
	UserID         uuid.UUID
	StudentName    string
	TryoutID       uuid.UUID
	TryoutTitle    string
	AttemptNumber  int
	StartedAt      time.Time
	Subtest        *ActiveSubtest
	ViolationCount int
}

type ActiveSubtest struct {
	ID        uuid.UUID
	Name      string
	Position  int
	StartedAt time.Time
}

// SessionInfo identifies a session of any status for the violation timeline.
type SessionInfo struct {
	SessionID     uuid.UUID
	UserID        uuid.UUID
	StudentName   string
	TryoutID      uuid.UUID
	TryoutTitle   string
	AttemptNumber int
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
}
