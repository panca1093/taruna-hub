package model

import (
	"time"

	"github.com/google/uuid"
)

const WebsiteSchemaVersion = 1

type WebsiteHome struct {
	HeroTitle   string `json:"hero_title"`
	HeroBody    string `json:"hero_body"`
	ProgramCTA  string `json:"program_cta"`
	PracticeCTA string `json:"practice_cta"`
}

type WebsiteProgram struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Schedule string `json:"schedule"`
	Location string `json:"location"`
	Fees     string `json:"fees"`
	Contact  string `json:"contact"`
}

// WebsiteLocale keeps programs keyed by slot so a stray slot survives decoding and validation can name it.
type WebsiteLocale struct {
	Home     WebsiteHome               `json:"home"`
	Programs map[string]WebsiteProgram `json:"programs"`
}

type WebsiteContent struct {
	SchemaVersion int                      `json:"schema_version"`
	Locales       map[string]WebsiteLocale `json:"locales"`
}

type WebsiteWorkbench struct {
	Draft             WebsiteContent  `json:"draft"`
	DraftRevision     int64           `json:"draft_revision"`
	PublishedRevision int64           `json:"published_revision"`
	Published         *WebsiteContent `json:"published"`
	UpdatedAt         time.Time       `json:"updated_at"`
	UpdatedBy         *uuid.UUID      `json:"updated_by"`
}

type PublishedWebsite struct {
	Content           WebsiteContent `json:"content"`
	PublishedRevision int64          `json:"published_revision"`
	PublishedAt       time.Time      `json:"published_at"`
}

type WebsitePublication struct {
	PublishedRevision   int64     `json:"published_revision"`
	SourceDraftRevision int64     `json:"source_draft_revision"`
	PublishedAt         time.Time `json:"published_at"`
	PublishedBy         uuid.UUID `json:"published_by"`
	Changed             bool      `json:"changed"`
}

type WebsitePublicationList struct {
	Items      []WebsitePublication `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}
