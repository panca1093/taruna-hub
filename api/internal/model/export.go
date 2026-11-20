package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ExportTryoutPDF         = "tryout_pdf"
	ExportTryoutResultsCSV  = "tryout_results_csv"
	ExportTryoutResultsXLSX = "tryout_results_xlsx"
)

const (
	ExportQueued  = "queued"
	ExportRunning = "running"
	ExportReady   = "ready"
	ExportFailed  = "failed"
)

type Export struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Kind        string
	Params      json.RawMessage
	Status      string
	RequestedBy uuid.UUID
	CutOffAt    time.Time
	ObjectKey   *string
	ContentType *string
	FileName    *string
	SizeBytes   *int64
	ErrorCode   string
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FinishedAt  *time.Time
}
