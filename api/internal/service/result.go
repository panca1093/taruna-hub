package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var ErrExplanationsNotReleased = errors.New("explanations are not released yet")

const (
	ReasonResultQuery = "result_query"

	// IRTMinStudents is the PRD's calibration floor: fewer distinct students than this on one question version
	// makes the result carry an IRT warning (D04).
	IRTMinStudents = 30

	defaultResultLimit = 50
	maxResultLimit     = 100
)

type ResultOption func(*ResultService)

func WithResultClock(now func() time.Time) ResultOption {
	return func(s *ResultService) { s.now = now }
}

// WithIRTThreshold lets a test calibrate with a handful of students instead of thirty.
func WithIRTThreshold(n int) ResultOption {
	return func(s *ResultService) { s.irtMin = n }
}

// ResultService reads what P16 persisted: current ranks, the controlled review and the staff result list. Ranking is
// computed from PostgreSQL on every call; nothing here stores a rank, so no rank can be stale.
type ResultService struct {
	db     *pgxpool.Pool
	rt     *RuntimeService
	repo   *repository.ResultRepository
	rtRepo *repository.RuntimeRepository
	now    func() time.Time
	irtMin int
}

func NewResultService(rt *RuntimeService, repo *repository.ResultRepository, rtRepo *repository.RuntimeRepository, opts ...ResultOption) *ResultService {
	s := &ResultService{db: rt.db, rt: rt, repo: repo, rtRepo: rtRepo, now: time.Now, irtMin: IRTMinStudents}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ResultDetail is a completed attempt with the student's current rank on the tryout and the explanation release.
type ResultDetail struct {
	Result    model.Result
	Ranking   model.Ranking
	ReleaseAt *time.Time
	Released  bool
}

func released(at *time.Time, now time.Time) bool { return at == nil || !now.Before(*at) }

// Detail is the owner's result plus the live rank. The rank is the student's average over all completed attempts
// of this tryout, so every attempt of the same student shows the same rank.
func (s *ResultService) Detail(ctx context.Context, scope HostScope, principal model.Principal, sessionID uuid.UUID) (ResultDetail, error) {
	result, err := s.rt.Result(ctx, scope, principal, sessionID)
	if err != nil {
		return ResultDetail{}, err
	}
	rank, ok, err := s.repo.Ranking(ctx, s.db, result.TenantID, result.TryoutID, result.UserID)
	if err != nil {
		return ResultDetail{}, runtimeError(err)
	}
	if !ok {
		return ResultDetail{}, fmt.Errorf("result %s has no score total", sessionID)
	}
	at, err := s.repo.TryoutRelease(ctx, s.db, result.TenantID, result.TryoutID)
	if err != nil {
		return ResultDetail{}, runtimeError(err)
	}
	return ResultDetail{Result: result, Ranking: rank, ReleaseAt: at, Released: released(at, s.now())}, nil
}

// ReviewItem is one question of the review with the student's own answer.
type ReviewItem struct {
	Question model.ReviewQuestion
	Answer   []string
	Correct  bool
}

type ReviewSubtest struct {
	Part      model.PartResult
	Questions []ReviewItem
}

type Review struct {
	Result   model.Result
	Subtests []ReviewSubtest
}

// Review returns questions, the student's answers, keys and explanations of one completed attempt of their own.
// Before the tryout's release time it answers ErrExplanationsNotReleased and reads nothing but the release time,
// so a key or an explanation cannot leave by any path.
func (s *ResultService) Review(ctx context.Context, scope HostScope, principal model.Principal, sessionID uuid.UUID) (Review, error) {
	view, err := s.rt.Session(ctx, scope, principal, sessionID)
	if err != nil {
		return Review{}, err
	}
	if view.Result == nil {
		return Review{}, ErrNotCompleted
	}
	a := view.Attempt
	at, err := s.repo.TryoutRelease(ctx, s.db, a.TenantID, a.TryoutID)
	if err != nil {
		return Review{}, runtimeError(err)
	}
	if !released(at, s.now()) {
		return Review{}, ErrExplanationsNotReleased
	}
	questions, err := s.repo.ReviewQuestions(ctx, s.db, a.TenantID, a.ID)
	if err != nil {
		return Review{}, runtimeError(err)
	}
	saved, err := s.rtRepo.Answers(ctx, s.db, a.TenantID, a.ID, nil)
	if err != nil {
		return Review{}, runtimeError(err)
	}
	answers := make(map[uuid.UUID][]string, len(saved))
	for _, ans := range saved {
		answers[ans.QuestionID] = ans.Answer
	}
	out := Review{Result: *view.Result}
	bySubtest := make(map[uuid.UUID]int, len(view.Result.Parts))
	for _, p := range view.Result.Parts {
		bySubtest[p.SubtestID] = len(out.Subtests)
		out.Subtests = append(out.Subtests, ReviewSubtest{Part: p, Questions: []ReviewItem{}})
	}
	for _, q := range questions {
		given := answers[q.QuestionID]
		if given == nil {
			given = []string{}
		}
		_, _, correct := scoreQuestion(q.AttemptQuestion, given, a.Weights)
		i := bySubtest[q.SubtestID]
		out.Subtests[i].Questions = append(out.Subtests[i].Questions, ReviewItem{Question: q, Answer: given, Correct: correct})
	}
	return out, nil
}

func staffTenant(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return uuid.Nil, err
	}
	if !isStaff(principal.Role) {
		return uuid.Nil, ErrRuntimeForbidden
	}
	return tenantID, nil
}

func checkedResultPage(limit, offset int, fields map[string]string) (int, int) {
	if limit == 0 {
		limit = defaultResultLimit
	}
	if limit < 1 || limit > maxResultLimit {
		fields["limit"] = ReasonResultQuery
	}
	if offset < 0 {
		fields["offset"] = ReasonResultQuery
	}
	return limit, offset
}

// Leaderboard is one page of a tryout's standings for Admin and Operator.
func (s *ResultService) Leaderboard(ctx context.Context, scope HostScope, principal model.Principal, tryoutID uuid.UUID, limit, offset int) ([]model.Standing, int, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	fields := map[string]string{}
	limit, offset = checkedResultPage(limit, offset, fields)
	if len(fields) > 0 {
		return nil, 0, &InputError{Fields: fields}
	}
	if _, err := s.repo.TryoutRelease(ctx, s.db, tenantID, tryoutID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, 0, ErrTryoutNotFound
		}
		return nil, 0, runtimeError(err)
	}
	standings, total, err := s.repo.Leaderboard(ctx, s.db, tenantID, tryoutID, limit, offset)
	return standings, total, runtimeError(err)
}

type StaffResultFilter struct {
	TryoutID *uuid.UUID
	Search   string
	Limit    int
	Offset   int
}

// StaffResults lists completed attempts for Admin and Operator. IRTWarning is set only on results of IRT-mode
// sessions and is never part of a student response.
func (s *ResultService) StaffResults(ctx context.Context, scope HostScope, principal model.Principal, f StaffResultFilter) ([]model.StaffResult, int, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	fields := map[string]string{}
	f.Limit, f.Offset = checkedResultPage(f.Limit, f.Offset, fields)
	if utf8.RuneCountInString(f.Search) > 100 {
		fields["q"] = ReasonResultQuery
	}
	if len(fields) > 0 {
		return nil, 0, &InputError{Fields: fields}
	}
	rows, total, err := s.repo.StaffResults(ctx, s.db, tenantID, repository.StaffResultFilter{
		TryoutID: f.TryoutID, Search: f.Search, Limit: f.Limit, Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, runtimeError(err)
	}
	if len(rows) == 0 {
		return rows, total, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	var irtIDs []uuid.UUID
	for _, r := range rows {
		ids = append(ids, r.SessionID)
		if r.ScoringMode == "irt" {
			irtIDs = append(irtIDs, r.SessionID)
		}
	}
	parts, err := s.repo.SessionParts(ctx, s.db, tenantID, ids)
	if err != nil {
		return nil, 0, runtimeError(err)
	}
	under := map[uuid.UUID]bool{}
	if len(irtIDs) > 0 {
		if under, err = s.repo.IRTUnderThreshold(ctx, s.db, tenantID, irtIDs, s.irtMin); err != nil {
			return nil, 0, runtimeError(err)
		}
	}
	for i := range rows {
		rows[i].IRTWarning = under[rows[i].SessionID]
		for _, p := range parts[rows[i].SessionID] {
			p.Score = percent(p.EarnedPoints, p.MaxPoints)
			rows[i].Subtests = append(rows[i].Subtests, p)
		}
	}
	return rows, total, nil
}
