package model

import (
	"time"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelWhatsApp Channel = "whatsapp"
)

type NotificationStatus string

const (
	// StatusQueued is the only non-terminal state: it is the only one with finished_at NULL.
	StatusQueued      NotificationStatus = "queued"
	StatusSent        NotificationStatus = "sent"
	StatusFailed      NotificationStatus = "failed"
	StatusExpired     NotificationStatus = "expired"
	StatusUnavailable NotificationStatus = "unavailable"
)

type Outcome string

const (
	OutcomeOK        Outcome = "ok"
	OutcomeTransient Outcome = "transient"
	OutcomePermanent Outcome = "permanent"
	OutcomeUncertain Outcome = "uncertain"
)

// Notification is one durable delivery attempt of one business event to one recipient.
type Notification struct {
	ID           uuid.UUID
	TenantID     *uuid.UUID
	EventKey     string
	Channel      Channel
	Recipient    string
	Template     string
	Params       map[string]string
	SealedParams []byte
	ExpiresAt    *time.Time
	Status       NotificationStatus
	Attempts     int
	LastOutcome  *Outcome
	LastError    *string
	ClaimToken   *uuid.UUID
	ClaimedUntil *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	FinishedAt   *time.Time
}
