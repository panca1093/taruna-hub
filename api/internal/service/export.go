package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrExportNotFound  = errors.New("export not found")
	ErrExportForbidden = errors.New("exports are staff only")
	ErrExportNotReady  = errors.New("export is not ready")
)

const (
	ReasonExportKind = "export_kind"

	JobKindExport = "export"
	QueueExports  = "exports"
	// ExportMaxAttempts is the per-job budget, independent of any client default.
	ExportMaxAttempts = 3

	TemplateExportReady = "export-ready"

	exportLinkTTL      = 60 * time.Second
	exportMaxNameRunes = 120
	exportCodeRender   = "render_failed"
	exportCodeUpload   = "upload_failed"
	exportCodeStorage  = "storage_unavailable"
	exportCodeNoRender = "no_renderer"
	exportCodeInternal = "internal_error"
)

var knownExportKinds = map[string]bool{
	model.ExportTryoutPDF:         true,
	model.ExportTryoutResultsCSV:  true,
	model.ExportTryoutResultsXLSX: true,
}

type ExportArtifact struct {
	Data        []byte
	ContentType string
	FileName    string
}

// ExportRenderer reads only what the stored job names: the kind's params and the cut-off.
type ExportRenderer interface {
	Render(ctx context.Context, job model.Export) (ExportArtifact, error)
}

// ExportError lets a renderer pick the machine-readable code the requester sees; anything else is render_failed.
type ExportError struct {
	Code string
	Err  error
}

func (e *ExportError) Error() string { return e.Code + ": " + e.Err.Error() }
func (e *ExportError) Unwrap() error { return e.Err }

// ExportJobArgs is the River payload.
type ExportJobArgs struct {
	TenantID uuid.UUID `json:"tenant_id"`
	ExportID uuid.UUID `json:"export_id"`
}

func (ExportJobArgs) Kind() string { return JobKindExport }

type ExportJobInserter interface {
	InsertExportTx(ctx context.Context, tx pgx.Tx, args ExportJobArgs) error
}

// DispositionSigner is the optional storage upgrade that makes a download save under the export's file name.
type DispositionSigner interface {
	PresignGetAs(ctx context.Context, key string, ttl time.Duration, fileName string) (string, error)
}

type ExportService struct {
	db        *pgxpool.Pool
	repo      *repository.ExportRepository
	store     ObjectStore
	jobs      ExportJobInserter
	notes     *NotificationService
	logger    *slog.Logger
	renderers map[string]ExportRenderer
}

// NewExportService takes a nil store when storage is not configured; downloads then answer ErrStorageUnavailable.
func NewExportService(db *pgxpool.Pool, repo *repository.ExportRepository, store ObjectStore, jobs ExportJobInserter, notes *NotificationService, logger *slog.Logger) *ExportService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExportService{db: db, repo: repo, store: store, jobs: jobs, notes: notes, logger: logger, renderers: map[string]ExportRenderer{}}
}

// Register is called once per kind while the worker starts, before any job runs.
func (s *ExportService) Register(kind string, r ExportRenderer) {
	s.renderers[kind] = r
}

func (s *ExportService) staffTenant(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return uuid.Nil, err
	}
	if !isStaff(principal.Role) {
		return uuid.Nil, ErrExportForbidden
	}
	return tenantID, nil
}

// Request writes the row and its job in one transaction, so a request is never recorded without a job.
func (s *ExportService) Request(ctx context.Context, scope HostScope, principal model.Principal, kind string, params any) (model.Export, error) {
	tenantID, err := s.staffTenant(scope, principal)
	if err != nil {
		return model.Export{}, err
	}
	if !knownExportKinds[kind] {
		return model.Export{}, &InputError{Fields: map[string]string{"kind": ReasonExportKind}}
	}
	if params == nil {
		params = struct{}{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return model.Export{}, fmt.Errorf("encode export params: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.Export{}, fmt.Errorf("begin export request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	created, err := s.repo.Insert(ctx, tx, model.Export{TenantID: tenantID, Kind: kind, Params: raw, RequestedBy: principal.UserID})
	if err != nil {
		return model.Export{}, err
	}
	if err := s.jobs.InsertExportTx(ctx, tx, ExportJobArgs{TenantID: tenantID, ExportID: created.ID}); err != nil {
		return model.Export{}, fmt.Errorf("insert export job: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Export{}, fmt.Errorf("commit export request: %w", err)
	}
	return created, nil
}

func (s *ExportService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Export, error) {
	tenantID, err := s.staffTenant(scope, principal)
	if err != nil {
		return model.Export{}, err
	}
	return s.find(ctx, tenantID, id)
}

func (s *ExportService) find(ctx context.Context, tenantID, id uuid.UUID) (model.Export, error) {
	e, err := s.repo.Find(ctx, tenantID, id)
	return e, exportRepoError(err)
}

// List is newest first; an empty kind lists every kind.
func (s *ExportService) List(ctx context.Context, scope HostScope, principal model.Principal, kind string, limit int) ([]model.Export, error) {
	tenantID, err := s.staffTenant(scope, principal)
	if err != nil {
		return nil, err
	}
	if kind != "" && !knownExportKinds[kind] {
		return nil, &InputError{Fields: map[string]string{"kind": ReasonExportKind}}
	}
	out, err := s.repo.List(ctx, tenantID, kind, limit)
	return out, exportRepoError(err)
}

// DownloadLink re-authorizes on every call and never signs an object that is not ready.
func (s *ExportService) DownloadLink(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (AssetLink, error) {
	tenantID, err := s.staffTenant(scope, principal)
	if err != nil {
		return AssetLink{}, err
	}
	e, err := s.find(ctx, tenantID, id)
	if err != nil {
		return AssetLink{}, err
	}
	if e.Status != model.ExportReady || e.ObjectKey == nil || e.FileName == nil {
		return AssetLink{}, ErrExportNotReady
	}
	if s.store == nil {
		return AssetLink{}, ErrStorageUnavailable
	}
	expires := time.Now().Add(exportLinkTTL)
	var link string
	if signer, ok := s.store.(DispositionSigner); ok {
		link, err = signer.PresignGetAs(ctx, *e.ObjectKey, exportLinkTTL, *e.FileName)
	} else {
		link, err = s.store.PresignGet(ctx, *e.ObjectKey, exportLinkTTL)
	}
	if err != nil {
		return AssetLink{}, fmt.Errorf("presign export: %w", err)
	}
	return AssetLink{URL: link, ExpiresAt: expires}, nil
}

// Run is one River attempt. A finished export is a no-op, so a restarted worker never renders twice. A failed
// attempt returns the error for a retry until the last, which ends the export as failed and returns nil.
func (s *ExportService) Run(ctx context.Context, tenantID, exportID uuid.UUID, attempt, maxAttempts int) error {
	job, err := s.repo.Claim(ctx, tenantID, exportID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !knownExportKinds[job.Kind] {
		return s.fail(ctx, job, exportCodeInternal, fmt.Errorf("unknown export kind %q", job.Kind), true)
	}
	final := attempt >= maxAttempts

	renderer, ok := s.renderers[job.Kind]
	if !ok {
		return s.fail(ctx, job, exportCodeNoRender, fmt.Errorf("no renderer for %s", job.Kind), final)
	}
	if s.store == nil {
		return s.fail(ctx, job, exportCodeStorage, ErrStorageUnavailable, final)
	}
	artifact, err := renderer.Render(ctx, job)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		code := exportCodeRender
		var coded *ExportError
		if errors.As(err, &coded) && coded.Code != "" {
			code = coded.Code
		}
		return s.fail(ctx, job, code, err, final)
	}
	if len(artifact.Data) == 0 || artifact.ContentType == "" {
		return s.fail(ctx, job, exportCodeRender, errors.New("renderer returned an empty artifact"), final)
	}
	fileName := safeFileName(artifact.FileName, job)
	key := fmt.Sprintf("%s/export/%s", job.TenantID, job.ID)
	if err := s.store.Put(ctx, key, artifact.Data, artifact.ContentType); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return s.fail(ctx, job, exportCodeUpload, err, final)
	}
	if err := s.finish(ctx, job, key, artifact.ContentType, fileName, int64(len(artifact.Data))); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return s.fail(ctx, job, exportCodeInternal, err, final)
	}
	return nil
}

// finish commits the ready state with its notification, so the notice exists exactly when the file does.
func (s *ExportService) finish(ctx context.Context, job model.Export, key, contentType, fileName string, size int64) error {
	email, tenantName, err := s.repo.Requester(ctx, job.TenantID, job.RequestedBy)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin export finish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := s.repo.MarkReady(ctx, tx, job.TenantID, job.ID, key, contentType, fileName, size); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	if email != nil && strings.TrimSpace(*email) != "" {
		if err := s.notes.Enqueue(ctx, tx, NewNotification{
			Scope:    Scope{TenantID: &job.TenantID},
			EventKey: TemplateExportReady + ":" + job.ID.String(),
			Channel:  string(model.ChannelEmail), Recipient: *email, Template: TemplateExportReady,
			Params: map[string]string{"file_name": fileName, "kind": job.Kind, "tenant_name": tenantName},
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *ExportService) fail(ctx context.Context, job model.Export, code string, cause error, final bool) error {
	s.logger.Warn("export attempt failed", slog.String("export_id", job.ID.String()), slog.String("kind", job.Kind),
		slog.String("code", code), slog.Bool("final", final), slog.String("error", cause.Error()))
	if err := s.repo.MarkAttemptFailed(context.WithoutCancel(ctx), job.TenantID, job.ID, code, final); err != nil {
		return err
	}
	if final {
		return nil
	}
	return cause
}

func exportRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrExportNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

// safeFileName keeps a renderer's name from carrying a path or header-breaking characters.
func safeFileName(name string, job model.Export) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r == 0x7f, strings.ContainsRune(`"\/:*?<>|;`, r):
			continue
		}
		b.WriteRune(r)
	}
	clean := []rune(strings.TrimSpace(b.String()))
	if len(clean) > exportMaxNameRunes {
		clean = clean[:exportMaxNameRunes]
	}
	if len(clean) == 0 || strings.Trim(string(clean), ".") == "" {
		return job.Kind + "-" + job.ID.String()
	}
	return string(clean)
}
