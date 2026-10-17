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

// ProgramTree is a program with its ordered subjects and chapters; a Program.ID that is uuid.Nil
// asks the repository to create it rather than update it.
type ProgramTree struct {
	Program  Program
	Subjects []SubjectTree
}

type SubjectTree struct {
	Subject  Subject
	Chapters []Chapter
}
