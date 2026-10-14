package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrInvalidNotification = errors.New("invalid notification")
	ErrUnknownTemplate     = errors.New("unknown notification template")
	ErrTemplateExists      = errors.New("notification template already registered")
	ErrSealKeyRequired     = errors.New("NOTIFICATION_SEAL_KEY is required to enqueue secret params")
	ErrNoSealKey           = errors.New("notification sealed payload cannot be opened: no key configured")
	ErrSealOpen            = errors.New("notification sealed payload cannot be opened")
	ErrInvalidSealKey      = errors.New("invalid notification seal key")
)

// ErrBusy is a claim conflict, not a failure: the caller answers it with a snooze.
var ErrBusy = errors.New("notification is claimed by another attempt")

const (
	// JobKindNotification is the River kind and the Reconcile join key.
	JobKindNotification = "notification"
	// QueueNotifications is the only queue P02 defines; reports, PDFs and imports get their own later.
	QueueNotifications = "notifications"
	// NotificationMaxAttempts is the per-job budget, independent of any client default.
	NotificationMaxAttempts = 6
	// ClaimLease must outlive the SMTP deadline so a slow send never loses its own row.
	ClaimLease = 2 * time.Minute
	// ReconcileCutoff keeps the maintenance scan away from work that is merely young.
	ReconcileCutoff = time.Hour
	// ReconcileErrorText is the fixed, sanitized reason recorded for an orphaned queued row.
	ReconcileErrorText = "reconciled: delivery job ended without a result"
	ReconcileLimit     = 100
	// SealKeyBytes is the AES-256 key length NOTIFICATION_SEAL_KEY must decode to.
	SealKeyBytes = 32

	templateNameSMTPTest = "smtp-test"
)

// Classification is the only thing a transport tells the service, so no provider text crosses it.
type Classification int

const (
	ClassTransient Classification = iota
	ClassPermanent
	ClassUncertain
)

// DeliveryError carries a fixed class reason, never provider-controlled text.
type DeliveryError struct {
	Class  Classification
	Reason string
}

func (e *DeliveryError) Error() string { return e.Reason }

func IsPermanent(err error) bool {
	var de *DeliveryError
	return errors.As(err, &de) && de.Class == ClassPermanent
}

// Scope says whose notification this is: a tenant, or the platform when TenantID is nil.
type Scope struct {
	TenantID *uuid.UUID
}

// NewNotification is one business event asking to be delivered on one channel.
type NewNotification struct {
	Scope        Scope
	EventKey     string
	Channel      string
	Recipient    string
	Template     string
	Params       map[string]string
	SecretParams map[string]string
	ExpiresAt    *time.Time
}

type Message struct {
	Recipient string
	Subject   string
	Body      string
}

// Sender is one channel's transport; an unconfigured one becomes a terminal unavailable row.
type Sender interface {
	Configured() bool
	Send(ctx context.Context, msg Message) error
}

// NotificationJobArgs is the River payload; its JSON key is the Reconcile join key.
type NotificationJobArgs struct {
	NotificationID uuid.UUID `json:"notification_id"`
}

func (NotificationJobArgs) Kind() string { return JobKindNotification }

type JobInserter interface {
	InsertNotificationTx(ctx context.Context, tx pgx.Tx, args NotificationJobArgs) error
}

// templateData is what a template executes against, sorted by key so rendering is stable.
type templateData struct {
	Values []templateValue
}

type templateValue struct {
	Key   string
	Value string
}

// Get lets a template pull one named value; a missing key renders as empty.
func (d templateData) Get(key string) string {
	for _, v := range d.Values {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

type notificationTemplate struct {
	subject *template.Template
	body    *template.Template
}

type Attempt struct {
	N   int
	Max int
}

type NotificationService struct {
	repo      *repository.NotificationRepository
	jobs      JobInserter
	senders   map[string]Sender
	sealer    *Sealer
	templates map[string]notificationTemplate
	logger    *slog.Logger
	now       func() time.Time
	// reconcileLimit bounds one maintenance scan; it is a field so a test can lower it.
	reconcileLimit int
}

type Option func(*NotificationService)

// WithClock replaces the clock; tests use it to move a message past its expiry between checks.
func WithClock(now func() time.Time) Option {
	return func(s *NotificationService) { s.now = now }
}

// WithReconcileLimit lowers the maintenance bound; tests assert only on their own rows.
func WithReconcileLimit(limit int) Option {
	return func(s *NotificationService) { s.reconcileLimit = limit }
}

func NewNotificationService(repo *repository.NotificationRepository, jobs JobInserter, senders map[string]Sender, sealKey []byte, logger *slog.Logger, opts ...Option) (*NotificationService, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &NotificationService{
		repo:           repo,
		jobs:           jobs,
		senders:        senders,
		templates:      map[string]notificationTemplate{},
		logger:         logger,
		now:            time.Now,
		reconcileLimit: ReconcileLimit,
	}
	for _, opt := range opts {
		opt(s)
	}
	if sealKey != nil {
		sealer, err := NewSealer(sealKey)
		if err != nil {
			return nil, err
		}
		s.sealer = sealer
	}
	// The one template P02 ships. P03 adds reset and OTP templates through RegisterTemplate.
	if err := s.RegisterTemplate(templateNameSMTPTest,
		`Taruna Eduka test message{{with .Get "subject"}} - {{.}}{{end}}`,
		"Pesan uji dari Taruna Eduka.{{range .Values}}\n{{.Key}}: {{.Value}}{{end}}"); err != nil {
		return nil, err
	}
	return s, nil
}

// RegisterTemplate is the seam later slices use to add event templates without touching transport.
func (s *NotificationService) RegisterTemplate(name, subject, body string) error {
	if _, ok := s.templates[name]; ok {
		return fmt.Errorf("%w: %s", ErrTemplateExists, name)
	}
	subjectTmpl, err := template.New(name + ".subject").Parse(subject)
	if err != nil {
		return fmt.Errorf("parse subject template %s: %w", name, err)
	}
	bodyTmpl, err := template.New(name + ".body").Parse(body)
	if err != nil {
		return fmt.Errorf("parse body template %s: %w", name, err)
	}
	s.templates[name] = notificationTemplate{subject: subjectTmpl, body: bodyTmpl}
	return nil
}

// Enqueue writes the row and its job in the caller's transaction; the caller must roll back on error.
func (s *NotificationService) Enqueue(ctx context.Context, tx pgx.Tx, n NewNotification) error {
	if n.Scope.TenantID != nil && *n.Scope.TenantID == uuid.Nil {
		// The all-zero UUID is the identity index's platform sentinel, so it can never be a scope.
		return fmt.Errorf("%w: tenant id is the zero UUID", ErrInvalidNotification)
	}
	if n.Channel != string(model.ChannelEmail) && n.Channel != string(model.ChannelWhatsApp) {
		return fmt.Errorf("%w: unknown channel %q", ErrInvalidNotification, n.Channel)
	}
	if strings.TrimSpace(n.Recipient) == "" {
		return fmt.Errorf("%w: recipient is required", ErrInvalidNotification)
	}
	if n.EventKey == "" {
		return fmt.Errorf("%w: event key is required", ErrInvalidNotification)
	}
	if _, ok := s.templates[n.Template]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTemplate, n.Template)
	}
	if n.ExpiresAt != nil && !n.ExpiresAt.After(s.now()) {
		return fmt.Errorf("%w: expiry is not in the future", ErrInvalidNotification)
	}
	for key := range n.SecretParams {
		if _, clash := n.Params[key]; clash {
			return fmt.Errorf("%w: param %q is both public and secret", ErrInvalidNotification, key)
		}
	}

	// The id is generated first so it can be the seal's associated data.
	id := uuid.New()
	sealed, err := s.sealSecrets(id, n.SecretParams)
	if err != nil {
		return err
	}

	params := n.Params
	if params == nil {
		params = map[string]string{}
	}
	rowID, err := s.repo.Create(ctx, tx, model.Notification{
		ID:           id,
		TenantID:     n.Scope.TenantID,
		EventKey:     n.EventKey,
		Channel:      model.Channel(n.Channel),
		Recipient:    n.Recipient,
		Template:     n.Template,
		Params:       params,
		SealedParams: sealed,
		ExpiresAt:    n.ExpiresAt,
	})
	if err != nil {
		return err
	}
	if rowID == uuid.Nil {
		// A duplicate identity is a suppression, not a failure, and inserts no second job.
		return nil
	}
	if err := s.jobs.InsertNotificationTx(ctx, tx, NotificationJobArgs{NotificationID: id}); err != nil {
		return fmt.Errorf("insert notification job: %w", err)
	}
	return nil
}

func (s *NotificationService) sealSecrets(id uuid.UUID, secret map[string]string) ([]byte, error) {
	if len(secret) == 0 {
		return nil, nil
	}
	if s.sealer == nil {
		return nil, ErrSealKeyRequired
	}
	plaintext, err := json.Marshal(secret)
	if err != nil {
		return nil, fmt.Errorf("encode secret params: %w", err)
	}
	return s.sealer.Seal(plaintext, id[:])
}

// Deliver runs one attempt; only the claim owner may transition the row.
func (s *NotificationService) Deliver(ctx context.Context, id uuid.UUID, a Attempt) error {
	// The one unscoped read: it discovers the row's scope, which every later call must pass.
	row, err := s.repo.FindForDelivery(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Status != model.StatusQueued {
		return nil
	}

	token := uuid.New()
	claimed, err := s.repo.Claim(ctx, row.TenantID, id, token, ClaimLease)
	if errors.Is(err, repository.ErrNotFound) {
		// The repository cannot tell "no such row" from "already claimed", and both mean snooze.
		return ErrBusy
	}
	if err != nil {
		return err
	}
	scope := claimed.TenantID

	if s.expired(claimed.ExpiresAt) {
		// An expiry is not a success, so it records no outcome.
		return s.finish(ctx, scope, claimed.ID, token, model.StatusExpired, nil, nil, a.N)
	}

	sender := s.senders[string(claimed.Channel)]
	if sender == nil || !sender.Configured() {
		// An unavailable channel is not a success either.
		return s.finish(ctx, scope, claimed.ID, token, model.StatusUnavailable, nil, nil, a.N)
	}

	tmpl, ok := s.templates[claimed.Template]
	if !ok {
		reason := "template is not registered"
		if err := s.finish(ctx, scope, claimed.ID, token, model.StatusFailed, outcomeOf(model.OutcomePermanent), &reason, a.N); err != nil {
			return err
		}
		return &DeliveryError{Class: ClassPermanent, Reason: reason}
	}

	params, err := s.renderParams(claimed)
	if err != nil {
		reason := err.Error()
		if err := s.finish(ctx, scope, claimed.ID, token, model.StatusFailed, outcomeOf(model.OutcomePermanent), &reason, a.N); err != nil {
			return err
		}
		return &DeliveryError{Class: ClassPermanent, Reason: reason}
	}

	msg, err := render(tmpl, params, claimed)
	if err != nil {
		reason := "template render failed"
		if err := s.finish(ctx, scope, claimed.ID, token, model.StatusFailed, outcomeOf(model.OutcomePermanent), &reason, a.N); err != nil {
			return err
		}
		return &DeliveryError{Class: ClassPermanent, Reason: reason}
	}

	// Expiry is re-checked after unsealing and rendering, so nothing at all runs between it and the send.
	if s.expired(claimed.ExpiresAt) {
		return s.finish(ctx, scope, claimed.ID, token, model.StatusExpired, nil, nil, a.N)
	}

	sendErr := sender.Send(ctx, msg)
	if sendErr == nil {
		s.logger.Info("notification sent",
			slog.String("notification_id", id.String()), slog.String("channel", string(claimed.Channel)),
			slog.String("recipient", MaskRecipient(claimed.Recipient)), slog.Int("attempt", a.N))
		return s.finish(ctx, scope, claimed.ID, token, model.StatusSent, outcomeOf(model.OutcomeOK), nil, a.N)
	}

	de := &DeliveryError{Class: ClassTransient, Reason: "delivery failed"}
	_ = errors.As(sendErr, &de)

	s.logger.Warn("notification delivery failed",
		slog.String("notification_id", id.String()), slog.String("channel", string(claimed.Channel)),
		slog.String("recipient", MaskRecipient(claimed.Recipient)), slog.Int("attempt", a.N),
		slog.String("class", de.Class.String()), slog.String("error", de.Reason))

	switch {
	case de.Class == ClassPermanent:
		if err := s.finish(ctx, scope, claimed.ID, token, model.StatusFailed, outcomeOf(model.OutcomePermanent), &de.Reason, a.N); err != nil {
			return err
		}
		return de
	case a.N >= a.Max:
		// The same owner write retires the row; River then discards the exhausted job.
		last := model.OutcomeTransient
		if de.Class == ClassUncertain {
			last = model.OutcomeUncertain
		}
		if err := s.finish(ctx, scope, claimed.ID, token, model.StatusFailed, &last, &de.Reason, a.N); err != nil {
			return err
		}
		return de
	default:
		return s.release(ctx, scope, claimed.ID, token, de, a.N)
	}
}

func (c Classification) String() string {
	switch c {
	case ClassPermanent:
		return "permanent"
	case ClassUncertain:
		return "uncertain"
	default:
		return "transient"
	}
}

func (s *NotificationService) expired(at *time.Time) bool {
	return at != nil && !s.now().Before(*at)
}

// renderParams merges the public params with the unsealed secrets, sorted so rendering is stable.
func (s *NotificationService) renderParams(n model.Notification) (templateData, error) {
	merged := make(map[string]string, len(n.Params))
	for k, v := range n.Params {
		merged[k] = v
	}
	if len(n.SealedParams) > 0 {
		if s.sealer == nil {
			return templateData{}, ErrNoSealKey
		}
		plaintext, err := s.sealer.Open(n.SealedParams, n.ID[:])
		if err != nil {
			s.logger.Error("notification sealed payload could not be opened",
				slog.String("notification_id", n.ID.String()))
			return templateData{}, ErrSealOpen
		}
		var secret map[string]string
		if err := json.Unmarshal(plaintext, &secret); err != nil {
			return templateData{}, ErrSealOpen
		}
		for k, v := range secret {
			merged[k] = v
		}
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	data := templateData{Values: make([]templateValue, 0, len(keys))}
	for _, k := range keys {
		data.Values = append(data.Values, templateValue{Key: k, Value: merged[k]})
	}
	return data, nil
}

func render(tmpl notificationTemplate, data templateData, n model.Notification) (Message, error) {
	var subject, body strings.Builder
	if err := tmpl.subject.Execute(&subject, data); err != nil {
		return Message{}, err
	}
	if err := tmpl.body.Execute(&body, data); err != nil {
		return Message{}, err
	}
	return Message{Recipient: n.Recipient, Subject: subject.String(), Body: body.String()}, nil
}

// finish writes a terminal state as the claim owner, detached from ctx cancellation.
func (s *NotificationService) finish(ctx context.Context, scope *uuid.UUID, id, token uuid.UUID, status model.NotificationStatus, outcome *model.Outcome, lastError *string, attempts int) error {
	ok, err := s.repo.Finish(context.WithoutCancel(ctx), scope, id, token, status, attempts, outcome, lastError)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.settleLostOwnership(ctx, scope, id)
}

func outcomeOf(o model.Outcome) *model.Outcome { return &o }

func (s *NotificationService) release(ctx context.Context, scope *uuid.UUID, id, token uuid.UUID, retry *DeliveryError, attempts int) error {
	outcome := model.OutcomeTransient
	if retry.Class == ClassUncertain {
		outcome = model.OutcomeUncertain
	}
	ok, err := s.repo.Release(context.WithoutCancel(ctx), scope, id, token, attempts, outcome, &retry.Reason)
	if err != nil {
		return err
	}
	if !ok {
		// Losing the lease to a terminal row settles it; a still-queued row means snooze.
		return s.settleLostOwnership(ctx, scope, id)
	}
	return retry
}

// settleLostOwnership resolves a write that matched no row: terminal settles, queued means snooze.
func (s *NotificationService) settleLostOwnership(ctx context.Context, scope *uuid.UUID, id uuid.UUID) error {
	row, err := s.repo.Find(context.WithoutCancel(ctx), scope, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Status != model.StatusQueued {
		return nil
	}
	return ErrBusy
}

// Reconcile retires queued rows whose River job ended without a result.
func (s *NotificationService) Reconcile(ctx context.Context) (int, error) {
	candidates, err := s.repo.ReconcileCandidates(ctx, s.now().Add(-ReconcileCutoff), s.reconcileLimit)
	if err != nil {
		return 0, err
	}
	failed := 0
	for _, n := range candidates {
		ok, err := s.repo.FailOrphan(ctx, n.TenantID, n.ID, ReconcileErrorText)
		if err != nil {
			return failed, err
		}
		if ok {
			failed++
			s.logger.Warn("notification reconciled as failed",
				slog.String("notification_id", n.ID.String()), slog.String("reason", ReconcileErrorText))
		}
	}
	return failed, nil
}

// RetryDelay is the bounded production schedule: the delay after failed attempt n.
func RetryDelay(attempt int) time.Duration {
	schedule := [...]time.Duration{
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		time.Hour,
		6 * time.Hour,
	}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > len(schedule) {
		attempt = len(schedule)
	}
	return schedule[attempt-1]
}

// MaskRecipient keeps a log line useful without writing the address down.
func MaskRecipient(recipient string) string {
	at := strings.LastIndex(recipient, "@")
	if at <= 0 {
		return "***"
	}
	return recipient[:1] + "***" + recipient[at:]
}

// Sealer is AES-256-GCM with one key. Replacing the key invalidates every pending sealed payload.
type Sealer struct {
	aead cipher.AEAD
}

func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != SealKeyBytes {
		return nil, fmt.Errorf("%w: NOTIFICATION_SEAL_KEY must decode to exactly %d bytes, got %d", ErrInvalidSealKey, SealKeyBytes, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal returns nonce||ciphertext with the notification id as associated data.
func (s *Sealer) Seal(plaintext, associatedData []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("seal nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, associatedData), nil
}

func (s *Sealer) Open(sealed, associatedData []byte) ([]byte, error) {
	if len(sealed) < s.aead.NonceSize() {
		return nil, ErrSealOpen
	}
	nonce, ciphertext := sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return nil, ErrSealOpen
	}
	return plaintext, nil
}
