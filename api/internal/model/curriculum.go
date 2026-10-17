package model

import (
	"time"

	"github.com/google/uuid"
)

type Program struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Code        string
	Name        string
	Description string
	Position    int
	CreatedAt   time.Time
}

type Subject struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ProgramID uuid.UUID
	Name      string
	Position  int
	CreatedAt time.Time
}

type Chapter struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	SubjectID uuid.UUID
	Name      string
	Position  int
	CreatedAt time.Time
}

// A Program.ID of uuid.Nil asks the repository to create rather than update.
type ProgramTree struct {
	Program  Program
	Subjects []SubjectTree
}

type SubjectTree struct {
	Subject  Subject
	Chapters []Chapter
}
