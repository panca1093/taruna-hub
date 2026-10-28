package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrTryoutNotFound   = errors.New("tryout not found")
	ErrTryoutStale      = errors.New("tryout changed by someone else")
	ErrTryoutLocked     = errors.New("tryout already attempted")
	ErrTryoutReferenced = errors.New("tryout still referenced")
	ErrTryoutForbidden  = errors.New("tryout change needs an admin")
)

// Tryout field reasons; the transport turns each into the copy the builder shows.
const (
	ReasonTryoutTitle       = "tryout_title"
	ReasonTryoutDescription = "tryout_description"
	ReasonTryoutScoring     = "tryout_scoring"
	ReasonTryoutProgram     = "tryout_program"
	ReasonTryoutActiveUntil = "tryout_active_until"
	ReasonTryoutActiveFrom  = "tryout_active_from"
	ReasonTryoutSubtestName = "tryout_subtest_name"
	ReasonTryoutDuration    = "tryout_duration"
	ReasonTryoutSubtestMax  = "tryout_subtest_max"
	ReasonTryoutQuestionMax = "tryout_question_max"
	ReasonTryoutQuestion    = "tryout_question"
	ReasonTryoutDuplicate   = "tryout_question_duplicate"
	ReasonTryoutVersion     = "tryout_version"
	ReasonTryoutEmpty       = "tryout_empty"
	ReasonTryoutWindow      = "tryout_window"
	ReasonTryoutRevision    = "tryout_base_revision"
	ReasonTryoutQuery       = "tryout_query"
	ReasonTryoutGrace       = "tryout_grace"
)

const (
	minSubtestSeconds      = 60
	maxSubtestSeconds      = 28800
	maxSubtests            = 30
	maxQuestionsPerSubtest = 200
	maxQuestionsPerTryout  = 500
	maxTryoutTitleLen      = 200
	maxTryoutDescription   = 2000
	maxGraceSeconds        = 3600
	defaultTryoutLimit     = 50
	maxTryoutLimit         = 100
)

// TryoutQuestionInput carries an optional version: only a version already assigned to this tryout is kept.
type TryoutQuestionInput struct {
	QuestionID uuid.UUID
	VersionID  *uuid.UUID
}

type TryoutSubtestInput struct {
	Name            string
	DurationSeconds int
	Questions       []TryoutQuestionInput
}

type TryoutInput struct {
	ProgramID            uuid.UUID
	Title                string
	Description          string
	ScoringMode          string
	ActiveFrom           *time.Time
	ActiveUntil          *time.Time
	ExplanationReleaseAt *time.Time
	RandomizeQuestions   bool
	RandomizeOptions     bool
	GraceSeconds         *int
	Subtests             []TryoutSubtestInput
	BaseRevision         int
}

type TryoutFilter struct {
	ProgramID *uuid.UUID
	Status    string
	Search    string
	Limit     int
	Offset    int
}

type TryoutOption func(*TryoutService)

// WithTryoutClock replaces the clock; publish validation is time-based, so tests move time, never sleep.
func WithTryoutClock(now func() time.Time) TryoutOption {
	return func(s *TryoutService) { s.now = now }
}

type TryoutService struct {
	db   *pgxpool.Pool
	repo *repository.TryoutRepository
	now  func() time.Time
}

func NewTryoutService(db *pgxpool.Pool, repo *repository.TryoutRepository, opts ...TryoutOption) *TryoutService {
	s := &TryoutService{db: db, repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *TryoutService) List(ctx context.Context, scope HostScope, principal model.Principal, filter TryoutFilter) ([]model.Tryout, int, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	filter, err = validatedTryoutFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	tryouts, total, err := s.repo.List(ctx, tenantID, repository.TryoutFilter{
		ProgramID: filter.ProgramID, Status: filter.Status, Search: filter.Search,
		Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tryouts: %w", err)
	}
	return tryouts, total, nil
}

func (s *TryoutService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Tryout, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Tryout{}, err
	}
	tryout, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Tryout{}, tryoutError(err)
	}
	return tryout, nil
}

func (s *TryoutService) Create(ctx context.Context, scope HostScope, principal model.Principal, input TryoutInput) (model.Tryout, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Tryout{}, err
	}
	input = adminOnlyGrace(principal, input)
	input, err = tryoutScalars(input)
	if err != nil {
		return model.Tryout{}, err
	}
	if err := s.checkProgram(ctx, tenantID, input.ProgramID); err != nil {
		return model.Tryout{}, err
	}
	subtests, err := s.resolve(ctx, s.db, tenantID, uuid.Nil, input)
	if err != nil {
		return model.Tryout{}, err
	}
	created, err := s.repo.Create(ctx, tenantID, principal.UserID, tryoutOf(input, subtests))
	if err != nil {
		return model.Tryout{}, tryoutError(err)
	}
	return created, nil
}

// Update replaces the whole definition in one transaction, so a stale revision can never merge.
func (s *TryoutService) Update(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, input TryoutInput) (model.Tryout, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Tryout{}, err
	}
	if input.BaseRevision < 1 {
		return model.Tryout{}, &InputError{Fields: map[string]string{"base_revision": ReasonTryoutRevision}}
	}
	input = adminOnlyGrace(principal, input)
	input, err = tryoutScalars(input)
	if err != nil {
		return model.Tryout{}, err
	}
	if err := s.checkProgram(ctx, tenantID, input.ProgramID); err != nil {
		return model.Tryout{}, err
	}

	var out model.Tryout
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return tryoutError(err)
		}
		if current.Locked {
			return ErrTryoutLocked
		}
		if current.Revision != input.BaseRevision {
			return ErrTryoutStale
		}
		subtests, err := s.resolve(ctx, tx, tenantID, id, input)
		if err != nil {
			return err
		}
		next := tryoutOf(input, subtests)
		if input.GraceSeconds == nil {
			next.GraceSeconds = current.GraceSeconds
		}
		if current.Status == model.TryoutPublished {
			if err := playable(next, s.now()); err != nil {
				return err
			}
		}
		if err := s.repo.UpdateHeadTx(ctx, tx, tenantID, id, next); err != nil {
			return tryoutError(err)
		}
		if err := s.repo.ReplaceChildrenTx(ctx, tx, tenantID, id, subtests); err != nil {
			return tryoutError(err)
		}
		out, err = s.repo.FindTx(ctx, tx, tenantID, id)
		return tryoutError(err)
	})
	if err != nil {
		return model.Tryout{}, err
	}
	return out, nil
}

func (s *TryoutService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return tryoutError(err)
		}
		if current.Locked {
			return ErrTryoutLocked
		}
		return tryoutError(s.repo.DeleteTx(ctx, tx, tenantID, id))
	})
}

func (s *TryoutService) Publish(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Tryout, error) {
	return s.setStatus(ctx, scope, principal, id, model.TryoutPublished)
}

func (s *TryoutService) Unpublish(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Tryout, error) {
	return s.setStatus(ctx, scope, principal, id, model.TryoutDraft)
}

func (s *TryoutService) setStatus(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, status string) (model.Tryout, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Tryout{}, err
	}
	var out model.Tryout
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return tryoutError(err)
		}
		if current.Locked {
			return ErrTryoutLocked
		}
		if status == model.TryoutPublished && current.Status != model.TryoutPublished {
			full, err := s.repo.FindTx(ctx, tx, tenantID, id)
			if err != nil {
				return tryoutError(err)
			}
			if err := playable(full, s.now()); err != nil {
				return err
			}
		}
		if err := s.repo.SetStatusTx(ctx, tx, tenantID, id, status); err != nil {
			return tryoutError(err)
		}
		out, err = s.repo.FindTx(ctx, tx, tenantID, id)
		return tryoutError(err)
	})
	if err != nil {
		return model.Tryout{}, err
	}
	return out, nil
}

// SetGrace is deliberately allowed on a locked tryout: D10 lets Admin change the grace while the exam runs,
// and the runtime reads it live, so the new value governs every answer admitted after this commit.
func (s *TryoutService) SetGrace(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, seconds int) (model.Tryout, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Tryout{}, err
	}
	if principal.Role != model.RoleAdmin {
		return model.Tryout{}, ErrTryoutForbidden
	}
	if seconds < 0 || seconds > maxGraceSeconds {
		return model.Tryout{}, &InputError{Fields: map[string]string{"grace_seconds": ReasonTryoutGrace}}
	}
	if err := s.repo.SetGraceSeconds(ctx, tenantID, id, seconds); err != nil {
		return model.Tryout{}, tryoutError(err)
	}
	return s.Get(ctx, scope, principal, id)
}

func (s *TryoutService) checkProgram(ctx context.Context, tenantID, programID uuid.UUID) error {
	ok, err := s.repo.ProgramExists(ctx, tenantID, programID)
	if err != nil {
		return err
	}
	if !ok {
		return &InputError{Fields: map[string]string{"program_id": ReasonTryoutProgram}}
	}
	return nil
}

// An existing assignment keeps its version, a new one takes the question's current version.
func (s *TryoutService) resolve(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}, tenantID, tryoutID uuid.UUID, input TryoutInput) ([]model.Subtest, error) {
	fields := map[string]string{}
	ids := make([]uuid.UUID, 0, len(input.Subtests))
	seen := map[uuid.UUID]bool{}
	for i, subtest := range input.Subtests {
		for j, question := range subtest.Questions {
			path := fmt.Sprintf("subtests[%d].questions[%d].question_id", i, j)
			if question.QuestionID == uuid.Nil {
				fields[path] = ReasonTryoutQuestion
				continue
			}
			if seen[question.QuestionID] {
				fields[path] = ReasonTryoutDuplicate
				continue
			}
			seen[question.QuestionID] = true
			ids = append(ids, question.QuestionID)
		}
	}
	if len(fields) > 0 {
		return nil, &InputError{Fields: fields}
	}

	current, err := s.repo.CurrentQuestions(ctx, q, tenantID, ids)
	if err != nil {
		return nil, err
	}
	assigned := map[uuid.UUID]uuid.UUID{}
	if tryoutID != uuid.Nil {
		if assigned, err = s.repo.AssignedVersions(ctx, q, tenantID, tryoutID); err != nil {
			return nil, err
		}
	}

	out := make([]model.Subtest, 0, len(input.Subtests))
	for i, subtest := range input.Subtests {
		resolved := model.Subtest{Name: subtest.Name, DurationSeconds: subtest.DurationSeconds}
		for j, question := range subtest.Questions {
			key, ok := current[question.QuestionID]
			if !ok {
				fields[fmt.Sprintf("subtests[%d].questions[%d].question_id", i, j)] = ReasonTryoutQuestion
				continue
			}
			versionID := key.VersionID
			if question.VersionID != nil {
				kept, ok := assigned[question.QuestionID]
				if !ok || kept != *question.VersionID {
					fields[fmt.Sprintf("subtests[%d].questions[%d].version_id", i, j)] = ReasonTryoutVersion
					continue
				}
				versionID = kept
			}
			resolved.Questions = append(resolved.Questions, model.Assignment{
				QuestionID: question.QuestionID, VersionID: versionID, Position: j + 1,
			})
		}
		out = append(out, resolved)
	}
	if len(fields) > 0 {
		return nil, &InputError{Fields: fields}
	}
	return out, nil
}

// playable is the publish rule, also re-applied to every save of an already published tryout.
func playable(tryout model.Tryout, now time.Time) error {
	fields := map[string]string{}
	if len(tryout.Subtests) == 0 {
		fields["subtests"] = ReasonTryoutEmpty
	}
	for i, subtest := range tryout.Subtests {
		if len(subtest.Questions) == 0 {
			fields[fmt.Sprintf("subtests[%d].questions", i)] = ReasonTryoutEmpty
		}
	}
	if tryout.ActiveUntil.IsZero() || !tryout.ActiveUntil.After(now) {
		fields["active_until"] = ReasonTryoutWindow
	}
	if len(fields) > 0 {
		return &InputError{Fields: fields}
	}
	return nil
}

// adminOnlyGrace drops a grace sent by anyone but Admin (D10), so the stored value stands.
func adminOnlyGrace(principal model.Principal, input TryoutInput) TryoutInput {
	if principal.Role != model.RoleAdmin {
		input.GraceSeconds = nil
	}
	return input
}

func tryoutScalars(input TryoutInput) (TryoutInput, error) {
	fields := map[string]string{}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if n := utf8.RuneCountInString(input.Title); n < 1 || n > maxTryoutTitleLen {
		fields["title"] = ReasonTryoutTitle
	}
	if utf8.RuneCountInString(input.Description) > maxTryoutDescription {
		fields["description"] = ReasonTryoutDescription
	}
	if input.ScoringMode != model.ScoringWeighted && input.ScoringMode != model.ScoringIRT {
		fields["scoring_mode"] = ReasonTryoutScoring
	}
	if input.ProgramID == uuid.Nil {
		fields["program_id"] = ReasonTryoutProgram
	}
	if input.ActiveUntil == nil {
		fields["active_until"] = ReasonTryoutActiveUntil
	} else if input.ActiveFrom != nil && !input.ActiveFrom.Before(*input.ActiveUntil) {
		fields["active_from"] = ReasonTryoutActiveFrom
	}
	if input.GraceSeconds != nil && (*input.GraceSeconds < 0 || *input.GraceSeconds > maxGraceSeconds) {
		fields["grace_seconds"] = ReasonTryoutGrace
	}
	if len(input.Subtests) > maxSubtests {
		fields["subtests"] = ReasonTryoutSubtestMax
	}

	total := 0
	for i, subtest := range input.Subtests {
		subtest.Name = strings.TrimSpace(subtest.Name)
		if n := utf8.RuneCountInString(subtest.Name); n < 1 || n > maxTryoutTitleLen {
			fields[fmt.Sprintf("subtests[%d].name", i)] = ReasonTryoutSubtestName
		}
		if subtest.DurationSeconds < minSubtestSeconds || subtest.DurationSeconds > maxSubtestSeconds {
			fields[fmt.Sprintf("subtests[%d].duration_seconds", i)] = ReasonTryoutDuration
		}
		if len(subtest.Questions) > maxQuestionsPerSubtest {
			fields[fmt.Sprintf("subtests[%d].questions", i)] = ReasonTryoutQuestionMax
		}
		total += len(subtest.Questions)
		input.Subtests[i].Name = subtest.Name
	}
	if total > maxQuestionsPerTryout {
		fields["subtests"] = ReasonTryoutQuestionMax
	}
	if len(fields) > 0 {
		return TryoutInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

func validatedTryoutFilter(filter TryoutFilter) (TryoutFilter, error) {
	fields := map[string]string{}
	if filter.Limit == 0 {
		filter.Limit = defaultTryoutLimit
	}
	if filter.Limit < 1 || filter.Limit > maxTryoutLimit {
		fields["limit"] = ReasonTryoutQuery
	}
	if filter.Offset < 0 {
		fields["offset"] = ReasonTryoutQuery
	}
	if filter.Status != "" && filter.Status != model.TryoutDraft && filter.Status != model.TryoutPublished {
		fields["status"] = ReasonTryoutQuery
	}
	if len(fields) > 0 {
		return TryoutFilter{}, &InputError{Fields: fields}
	}
	return filter, nil
}

func tryoutOf(input TryoutInput, subtests []model.Subtest) model.Tryout {
	out := model.Tryout{
		ProgramID: input.ProgramID, Title: input.Title, Description: input.Description,
		ScoringMode: input.ScoringMode, ActiveFrom: input.ActiveFrom,
		ExplanationReleaseAt: input.ExplanationReleaseAt, RandomizeQuestions: input.RandomizeQuestions,
		RandomizeOptions: input.RandomizeOptions, Subtests: subtests,
	}
	if input.GraceSeconds != nil {
		out.GraceSeconds = *input.GraceSeconds
	}
	if input.ActiveUntil != nil {
		out.ActiveUntil = *input.ActiveUntil
	}
	return out
}

func tryoutError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrTryoutNotFound
	case errors.Is(err, repository.ErrReferenced):
		return ErrTryoutReferenced
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func (s *TryoutService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
