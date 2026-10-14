package model

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	IsActive  bool
	CreatedAt time.Time
}
