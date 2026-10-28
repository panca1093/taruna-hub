package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	KecSessionInProgress = "in_progress"
	KecSessionCompleted  = "completed"

	KecKolomPending = "pending"
	KecKolomRunning = "running"
	KecKolomEnded   = "ended"
)

// KecCredit is the credit state of one enrollment: remaining is Purchased - Used, and credits never expire (D05).
type KecCredit struct {
	EnrollmentID uuid.UUID
	Purchased    int
	Used         int
}

func (c KecCredit) Remaining() int { return c.Purchased - c.Used }

// KecSession is one attempt's head row; the frozen columns are read separately because they are large.
type KecSession struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	UserID               uuid.UUID
	TestID               uuid.UUID
	TestTitle            string
	AttemptNumber        int
	Status               string
	Seed                 string
	TestRevision         int
	KolomCount           int
	SoalPerKolom         int
	OptionsPerSoal       int
	KolomDurationSeconds int
	PauseSeconds         int
	DueAt                *time.Time
	StartedAt            time.Time
	CompletedAt          *time.Time
}

// KecKolom is one kolom's place on the server timeline. Counts exist once it has ended.
type KecKolom struct {
	Number     int
	Status     string
	StartedAt  *time.Time
	DeadlineAt *time.Time
	Benar      *int
	Salah      *int
	Kosong     *int
}

// KecAnswer is a stored answer; a nil Answer is an explicit skip.
type KecAnswer struct {
	Kolom  int
	Soal   int
	Answer *string
}

// KecResult is the immutable counts plus the composite scores under one scoring version.
type KecResult struct {
	SessionID      uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	StudentName    string
	TestID         uuid.UUID
	TestTitle      string
	ProgramID      uuid.UUID
	AttemptNumber  int
	BenarTotal     int
	SalahTotal     int
	KosongTotal    int
	Kecepatan      float64
	Ketahanan      float64
	Ketelitian     float64
	NilaiAkhir     float64
	ScoringVersion int
	CompletedAt    time.Time
}

// KecAttempt is one row of a student's history on one test.
type KecAttempt struct {
	SessionID     uuid.UUID
	AttemptNumber int
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	NilaiAkhir    *float64
}

// KecStudentTest is what a student holds in one kecermatan test.
type KecStudentTest struct {
	TestID               uuid.UUID
	Title                string
	Description          string
	ProgramCode          string
	ProgramName          string
	OptionsPerSoal       int
	KolomCount           int
	SoalPerKolom         int
	KolomDurationSeconds int
	CoverFrom            string
	CoverTo              string
	Purchased            int
	Used                 int
	InProgress           bool
}

// KecResultRow is a result with the names staff filter by.
type KecResultRow struct {
	KecResult
	ProgramCode string
}
