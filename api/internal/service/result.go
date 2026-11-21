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
	TryoutID  *uuid.UUID
	ProgramID *uuid.UUID
	Search    string
	Limit     int
	Offset    int
}

// StaffStudentRow is one student on one tryout. IRTWarning is staff-only and never part of a student response.
type StaffStudentRow struct {
	model.StaffStudentResult
	Avatar     string
	IRTWarning bool
}

// StaffResults lists one row per student and tryout with at least one completed attempt, for Admin and Operator.
func (s *ResultService) StaffResults(ctx context.Context, scope HostScope, principal model.Principal, f StaffResultFilter) ([]StaffStudentRow, int, error) {
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
	rows, total, err := s.repo.StaffStudentResults(ctx, s.db, tenantID, repository.StaffResultFilter{
		TryoutID: f.TryoutID, ProgramID: f.ProgramID, Search: f.Search, Limit: f.Limit, Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, runtimeError(err)
	}
	out := make([]StaffStudentRow, len(rows))
	if len(rows) == 0 {
		return out, total, nil
	}
	pairs := make([]repository.StudentTryout, len(rows))
	for i, r := range rows {
		pairs[i] = repository.StudentTryout{StudentID: r.StudentID, TryoutID: r.TryoutID}
	}
	attempts, err := s.repo.StaffAttempts(ctx, s.db, tenantID, pairs)
	if err != nil {
		return nil, 0, runtimeError(err)
	}
	under, err := s.irtFlags(ctx, tenantID, attempts)
	if err != nil {
		return nil, 0, err
	}
	for i, r := range rows {
		out[i] = StaffStudentRow{StaffStudentResult: r, Avatar: staffAvatarOf(r)}
		for _, a := range attempts[pairs[i]] {
			if under[a.SessionID] {
				out[i].IRTWarning = true
			}
		}
	}
	return out, total, nil
}

func staffAvatarOf(r model.StaffStudentResult) string {
	return avatarOf(model.User{ID: r.StudentID, AvatarKey: r.AvatarKey, Gender: r.Gender, BirthDate: r.BirthDate})
}

// irtFlags flags the IRT-mode attempts that hold a question version answered by too few distinct students.
func (s *ResultService) irtFlags(ctx context.Context, tenantID uuid.UUID, attempts map[repository.StudentTryout][]model.StaffAttempt) (map[uuid.UUID]bool, error) {
	var ids []uuid.UUID
	for _, list := range attempts {
		for _, a := range list {
			if a.ScoringMode == "irt" {
				ids = append(ids, a.SessionID)
			}
		}
	}
	if len(ids) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	under, err := s.repo.IRTUnderThreshold(ctx, s.db, tenantID, ids, s.irtMin)
	if err != nil {
		return nil, runtimeError(err)
	}
	return under, nil
}

// StaffStudentDetail is one student's standing and completed attempts on one tryout.
type StaffStudentDetail struct {
	Student  StaffStudentRow
	Ranking  model.Ranking
	Attempts []model.StaffAttempt
}

// StaffStudentResults is one student's completed attempts on one tryout, newest first. A student or tryout outside
// the tenant, or a student without a completed attempt, is not found.
func (s *ResultService) StaffStudentResults(ctx context.Context, scope HostScope, principal model.Principal, studentID, tryoutID uuid.UUID) (StaffStudentDetail, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return StaffStudentDetail{}, err
	}
	rows, _, err := s.repo.StaffStudentResults(ctx, s.db, tenantID, repository.StaffResultFilter{TryoutID: &tryoutID, StudentID: &studentID, Limit: 1})
	if err != nil {
		return StaffStudentDetail{}, runtimeError(err)
	}
	if len(rows) == 0 {
		return StaffStudentDetail{}, ErrStudentResultNotFound
	}
	row := rows[0]
	pair := repository.StudentTryout{StudentID: studentID, TryoutID: tryoutID}
	byPair, err := s.repo.StaffAttempts(ctx, s.db, tenantID, []repository.StudentTryout{pair})
	if err != nil {
		return StaffStudentDetail{}, runtimeError(err)
	}
	attempts := byPair[pair]
	ids := make([]uuid.UUID, len(attempts))
	for i, a := range attempts {
		ids[i] = a.SessionID
	}
	parts, err := s.repo.SessionParts(ctx, s.db, tenantID, ids)
	if err != nil {
		return StaffStudentDetail{}, runtimeError(err)
	}
	under, err := s.irtFlags(ctx, tenantID, byPair)
	if err != nil {
		return StaffStudentDetail{}, err
	}
	detail := StaffStudentDetail{
		Student:  StaffStudentRow{StaffStudentResult: row, Avatar: staffAvatarOf(row)},
		Ranking:  model.Ranking{Rank: row.Rank, Participants: row.Participants, Average: row.Average, Attempts: row.AttemptsCounted},
		Attempts: attempts,
	}
	for i := range detail.Attempts {
		a := &detail.Attempts[i]
		a.IRTWarning = under[a.SessionID]
		for _, p := range parts[a.SessionID] {
			p.Score = percent(p.EarnedPoints, p.MaxPoints)
			a.Subtests = append(a.Subtests, p)
		}
	}
	return detail, nil
}
