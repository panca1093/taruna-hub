package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	mathrand "math/rand/v2"
	"sort"
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
	ErrSessionNotFound  = errors.New("session not found")
	ErrRuntimeForbidden = errors.New("not allowed on this session")
	ErrTryoutNotOpen    = errors.New("tryout is not open yet")
	ErrTryoutClosed     = errors.New("tryout window is closed")
	ErrSessionClosed    = errors.New("session already completed")
	ErrPartNotRunning   = errors.New("subtest is not running")
	ErrDeadlinePassed   = errors.New("subtest deadline passed")
	ErrStaleAnswer      = errors.New("stale answer write")
	ErrPartOrder        = errors.New("an earlier subtest is still open")
	ErrNotCompleted     = errors.New("session has no result yet")
)

const (
	ReasonAnswerSeq     = "answer_seq"
	ReasonAnswerShape   = "answer_shape"
	ReasonExtraSeconds  = "extension_seconds"
	ReasonExtraReason   = "extension_reason"
	ReasonExtraTotal    = "extension_total"
	ReasonWeight        = "settings_weight"
	ReasonMinTopup      = "settings_min_topup"
	maxAnswerLen        = 500
	maxExtensionSeconds = 14400
	maxExtraTotal       = 28800
	maxExtraReasonLen   = 500
	maxWeight           = 1000
)

// dueBatch bounds one worker tick, so a backlog after downtime drains over a few ticks instead of one long scan.
const dueBatch = 200

type RuntimeOption func(*RuntimeService)

// WithRuntimeClock replaces the clock; every deadline is time-based, so tests move time and never sleep.
func WithRuntimeClock(now func() time.Time) RuntimeOption {
	return func(s *RuntimeService) { s.now = now }
}

// WithRuntimeSeed replaces the permutation seed source, so a test can fix the shuffle.
func WithRuntimeSeed(seed func() int64) RuntimeOption {
	return func(s *RuntimeService) { s.seed = seed }
}

type RuntimeService struct {
	db   *pgxpool.Pool
	repo *repository.RuntimeRepository
	now  func() time.Time
	seed func() int64
}

func NewRuntimeService(db *pgxpool.Pool, repo *repository.RuntimeRepository, opts ...RuntimeOption) *RuntimeService {
	s := &RuntimeService{db: db, repo: repo, now: time.Now, seed: cryptoSeed}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func cryptoSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]) >> 1)
}

// PartView is a part as the student sees it. Deadline is the instant answers stop being admitted: the part
// deadline plus its staff extension plus the tryout's live grace (D10), so the countdown never lies about it.
type PartView struct {
	model.AttemptPart
	Deadline *time.Time
	Score    *float64
}

type AttemptView struct {
	Attempt    model.Attempt
	Parts      []PartView
	ServerTime time.Time
	Result     *model.Result
}

type SubtestView struct {
	Attempt    model.Attempt
	Part       PartView
	Questions  []model.AttemptQuestion
	Answers    map[uuid.UUID]model.AttemptAnswer
	ServerTime time.Time
}

// StudentTryout is the overview the student opens before and between subtests.
type StudentTryout struct {
	Tryout      model.Tryout
	ProgramCode string
	ProgramName string
	Subtests    []model.OverviewSubtest
	Current     *AttemptView
	Attempts    []model.AttemptSummary
}

type AnswerInput struct {
	Seq    int64
	Answer []string
}

type AnswerAck struct {
	QuestionID uuid.UUID
	Revision   int
	Seq        int64
	ServerTime time.Time
}

type ExtensionInput struct {
	Seconds int
	Reason  string
}

// keep marks a rejection that must still commit what the transaction already settled: an expiry noticed while
// refusing an answer is real state, and rolling it back would only make the next request do the same work again.
type keep struct{ err error }

func (k keep) Error() string { return k.err.Error() }
func (k keep) Unwrap() error { return k.err }

func (s *RuntimeService) run(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	fnErr := fn(tx)
	var k keep
	if fnErr != nil && !errors.As(fnErr, &k) {
		return fnErr
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	if fnErr != nil {
		return k.err
	}
	return nil
}

func studentOf(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return uuid.Nil, err
	}
	if principal.Role != model.RoleStudent {
		return uuid.Nil, ErrRuntimeForbidden
	}
	return tenantID, nil
}

func runtimeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrSessionNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

// Overview is the student's view of one tryout: its definition, the session in progress and the attempt history.
func (s *RuntimeService) Overview(ctx context.Context, scope HostScope, principal model.Principal, tryoutID uuid.UUID) (StudentTryout, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return StudentTryout{}, err
	}
	ok, err := s.repo.HasEnrollment(ctx, s.db, tenantID, principal.UserID, tryoutID)
	if err != nil {
		return StudentTryout{}, runtimeError(err)
	}
	if !ok {
		return StudentTryout{}, ErrTryoutNotFound
	}
	out, err := s.repo.TryoutOverview(ctx, s.db, tenantID, tryoutID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return StudentTryout{}, ErrTryoutNotFound
		}
		return StudentTryout{}, runtimeError(err)
	}
	overview := StudentTryout{Tryout: out.Tryout, ProgramCode: out.ProgramCode, ProgramName: out.ProgramName}
	overview.Subtests = out.Subtests
	if active, err := s.repo.ActiveAttempt(ctx, s.db, tenantID, principal.UserID, tryoutID, false); err == nil {
		view, err := s.view(ctx, tenantID, principal.UserID, active.ID)
		if err != nil {
			return StudentTryout{}, err
		}
		if view.Attempt.Status == model.SessionInProgress {
			overview.Current = &view
		}
	} else if !errors.Is(err, repository.ErrNotFound) {
		return StudentTryout{}, runtimeError(err)
	}
	overview.Attempts, err = s.repo.Attempts(ctx, s.db, tenantID, principal.UserID, &tryoutID)
	if err != nil {
		return StudentTryout{}, runtimeError(err)
	}
	return overview, nil
}

// Attempts is every session of the signed-in student, newest first.
func (s *RuntimeService) Attempts(ctx context.Context, scope HostScope, principal model.Principal) ([]model.AttemptSummary, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.Attempts(ctx, s.db, tenantID, principal.UserID, nil)
	return out, runtimeError(err)
}

// Start returns the student's session in progress or admits a new attempt. Both happen under the enrollment row
// lock, so two concurrent starts end with one session and the loser reads the winner's.
func (s *RuntimeService) Start(ctx context.Context, scope HostScope, principal model.Principal, tryoutID uuid.UUID) (AttemptView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return AttemptView{}, err
	}
	var id uuid.UUID
	err = s.run(ctx, func(tx pgx.Tx) error {
		if err := s.repo.LockEnrollment(ctx, tx, tenantID, principal.UserID, tryoutID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrTryoutNotFound
			}
			return runtimeError(err)
		}
		now := s.now()
		active, err := s.repo.ActiveAttempt(ctx, tx, tenantID, principal.UserID, tryoutID, true)
		if err == nil {
			if _, err := s.settleLocked(ctx, tx, active, now); err != nil {
				return err
			}
			id = active.ID
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return runtimeError(err)
		}

		tryout, err := s.repo.LockTryout(ctx, tx, tenantID, tryoutID, now)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrTryoutNotFound
			}
			return runtimeError(err)
		}
		switch {
		case tryout.Status != model.TryoutPublished:
			return ErrTryoutNotOpen
		case tryout.ActiveFrom != nil && now.Before(*tryout.ActiveFrom):
			return ErrTryoutNotOpen
		case !now.Before(tryout.ActiveUntil):
			return ErrTryoutClosed
		}
		id, err = s.admit(ctx, tx, tenantID, principal.UserID, tryout, now)
		return err
	})
	if err != nil {
		return AttemptView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, id)
}

// admit snapshots the tryout into a new session: versions, frozen weights and one permutation per session.
func (s *RuntimeService) admit(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, tryout model.Tryout, now time.Time) (uuid.UUID, error) {
	source, err := s.repo.SourceSubtests(ctx, tx, tenantID, tryout.ID)
	if err != nil {
		return uuid.Nil, runtimeError(err)
	}
	if len(source) == 0 {
		return uuid.Nil, ErrTryoutNotOpen
	}
	weights, err := s.repo.Weights(ctx, tx, tenantID)
	if err != nil {
		return uuid.Nil, runtimeError(err)
	}
	number, err := s.repo.NextAttemptNumber(ctx, tx, tenantID, userID, tryout.ID)
	if err != nil {
		return uuid.Nil, runtimeError(err)
	}
	seed := s.seed()
	parts, questions := snapshot(seed, tryout, source)
	id, err := s.repo.CreateAttempt(ctx, tx, model.Attempt{
		TenantID: tenantID, UserID: userID, TryoutID: tryout.ID, AttemptNumber: number,
		ScoringMode: tryout.ScoringMode, ScoringVersion: model.ScoringVersion, Weights: weights,
		Seed: seed, StartedAt: now,
	}, parts, questions)
	if err != nil {
		return uuid.Nil, runtimeError(err)
	}
	return id, nil
}

// snapshot orders each subtest's questions and options for one session. Subtests keep the tryout's order; the
// option ids are never rewritten, so points follow the option through any shuffle.
func snapshot(seed int64, tryout model.Tryout, source []model.SourceSubtest) ([]model.AttemptPart, []repository.OrderedQuestion) {
	var parts []model.AttemptPart
	var questions []repository.OrderedQuestion
	for _, st := range source {
		parts = append(parts, model.AttemptPart{
			SubtestID: st.ID, Position: st.Position, Name: st.Name,
			DurationSeconds: st.DurationSeconds, QuestionCount: len(st.Questions),
		})
		order := identity(len(st.Questions))
		if tryout.RandomizeQuestions {
			order = permutation(seed, st.ID.String(), len(st.Questions))
		}
		for position, at := range order {
			q := st.Questions[at]
			options := append([]string{}, q.OptionIDs...)
			if tryout.RandomizeOptions && (q.Type == model.QuestionPG || q.Type == model.QuestionPGMulti) {
				options = reordered(options, permutation(seed, q.QuestionID.String(), len(options)))
			}
			questions = append(questions, repository.OrderedQuestion{
				SubtestID: st.ID, Position: position + 1, QuestionID: q.QuestionID,
				VersionID: q.VersionID, OptionOrder: options,
			})
		}
	}
	return parts, questions
}

func identity(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func reordered(items []string, order []int) []string {
	out := make([]string, len(items))
	for i, at := range order {
		out[i] = items[at]
	}
	return out
}

// permutation is a Fisher-Yates shuffle of 0..n-1 driven by the session seed and a key naming what is shuffled.
// PCG is specified as stable across Go releases, which is what lets a fixture pin an exact order.
func permutation(seed int64, key string, n int) []int {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	rng := mathrand.New(mathrand.NewPCG(uint64(seed), h.Sum64()))
	out := identity(n)
	rng.Shuffle(n, func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// visibleDeadline is the last instant an answer is admitted and where the student's countdown ends: D10's part
// deadline plus the extension granted to this part plus the tryout's grace, which is read live.
func visibleDeadline(a model.Attempt, p model.AttemptPart) time.Time {
	return p.DeadlineAt.Add(time.Duration(p.ExtraSeconds+a.GraceSeconds) * time.Second)
}

// settleLocked applies every deadline that has passed. It expects the session row locked, and it is idempotent:
// a second call finds nothing left to do. It reports whether anything changed.
func (s *RuntimeService) settleLocked(ctx context.Context, tx pgx.Tx, a model.Attempt, now time.Time) (bool, error) {
	if a.Status != model.SessionInProgress {
		return false, nil
	}
	parts, err := s.repo.Parts(ctx, tx, a.TenantID, a.ID)
	if err != nil {
		return false, runtimeError(err)
	}
	changed := false
	for i, p := range parts {
		if p.Status != model.PartRunning || now.Before(visibleDeadline(a, p)) {
			continue
		}
		if err := s.settlePart(ctx, tx, a, p, model.SubmitExpired, visibleDeadline(a, p)); err != nil {
			return false, err
		}
		parts[i].Status = model.PartSubmitted
		changed = true
	}
	done, err := s.maybeFinalize(ctx, tx, a, parts, now)
	return changed || done, err
}

// maybeFinalize completes the session when every part is submitted, or when the tryout window is over and no part
// is still inside its grace. Unstarted parts then score 0.
func (s *RuntimeService) maybeFinalize(ctx context.Context, tx pgx.Tx, a model.Attempt, parts []model.AttemptPart, now time.Time) (bool, error) {
	allSubmitted := true
	for _, p := range parts {
		switch p.Status {
		case model.PartRunning:
			return false, nil
		case model.PartPending:
			allSubmitted = false
		}
	}
	if !allSubmitted && now.Before(a.ActiveUntil) {
		return false, nil
	}
	return true, s.finalize(ctx, tx, a, now)
}

// settlePart scores one part from its saved answers and makes it immutable. A part never started scores zero
// earned over its full maximum.
func (s *RuntimeService) settlePart(ctx context.Context, tx pgx.Tx, a model.Attempt, p model.AttemptPart, reason string, at time.Time) error {
	subtestID := p.SubtestID
	questions, err := s.repo.Questions(ctx, tx, a.TenantID, a.ID, &subtestID)
	if err != nil {
		return runtimeError(err)
	}
	saved, err := s.repo.Answers(ctx, tx, a.TenantID, a.ID, &subtestID)
	if err != nil {
		return runtimeError(err)
	}
	answers := make(map[uuid.UUID][]string, len(saved))
	for _, answer := range saved {
		answers[answer.QuestionID] = answer.Answer
	}
	score := scorePart(questions, answers, a.Weights)
	if _, err := s.repo.SettlePart(ctx, tx, a.TenantID, a.ID, p.SubtestID, reason, at,
		round4(score.Earned), round4(score.Max), score.Correct); err != nil {
		return runtimeError(err)
	}
	return nil
}

// finalize writes the session's one result and the running totals P17 ranks on. Settling the remaining parts,
// the result row, the totals and the completion all commit together, so the completion response always has a score.
func (s *RuntimeService) finalize(ctx context.Context, tx pgx.Tx, a model.Attempt, now time.Time) error {
	parts, err := s.repo.Parts(ctx, tx, a.TenantID, a.ID)
	if err != nil {
		return runtimeError(err)
	}
	for _, p := range parts {
		if p.Status == model.PartSubmitted {
			continue
		}
		if err := s.settlePart(ctx, tx, a, p, model.SubmitUnstarted, now); err != nil {
			return err
		}
	}
	if parts, err = s.repo.Parts(ctx, tx, a.TenantID, a.ID); err != nil {
		return runtimeError(err)
	}
	var earned, maximum float64
	for _, p := range parts {
		earned += deref(p.EarnedPoints)
		maximum += deref(p.MaxPoints)
	}
	res := model.Result{
		SessionID: a.ID, TenantID: a.TenantID, UserID: a.UserID, TryoutID: a.TryoutID,
		AttemptNumber: a.AttemptNumber, EarnedPoints: round4(earned), MaxPoints: round4(maximum),
		Score: percent(earned, maximum), ScoringMode: a.ScoringMode, ScoringVersion: a.ScoringVersion,
		CompletedAt: now,
	}
	inserted, err := s.repo.InsertResult(ctx, tx, res)
	if err != nil {
		return runtimeError(err)
	}
	if inserted {
		if err := s.repo.AddScoreTotal(ctx, tx, a.TenantID, a.UserID, a.TryoutID, res.Score); err != nil {
			return runtimeError(err)
		}
	}
	if _, err := s.repo.CompleteAttempt(ctx, tx, a.TenantID, a.ID, now); err != nil {
		return runtimeError(err)
	}
	return nil
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// needsSettling is the cheap read-only check that lets a plain read skip the write path.
func needsSettling(a model.Attempt, parts []model.AttemptPart, now time.Time) bool {
	if a.Status != model.SessionInProgress {
		return false
	}
	running := false
	for _, p := range parts {
		if p.Status == model.PartRunning {
			running = true
			if !now.Before(visibleDeadline(a, p)) {
				return true
			}
		}
	}
	return !running && !now.Before(a.ActiveUntil)
}

// view reads a session for its owner, settling any passed deadline first so a reload after expiry shows the truth.
func (s *RuntimeService) view(ctx context.Context, tenantID, userID, id uuid.UUID) (AttemptView, error) {
	a, err := s.repo.FindAttempt(ctx, s.db, tenantID, userID, id, false)
	if err != nil {
		return AttemptView{}, runtimeError(err)
	}
	parts, err := s.repo.Parts(ctx, s.db, tenantID, id)
	if err != nil {
		return AttemptView{}, runtimeError(err)
	}
	if needsSettling(a, parts, s.now()) {
		err = s.run(ctx, func(tx pgx.Tx) error {
			locked, err := s.repo.FindAttempt(ctx, tx, tenantID, userID, id, true)
			if err != nil {
				return runtimeError(err)
			}
			_, err = s.settleLocked(ctx, tx, locked, s.now())
			return err
		})
		if err != nil {
			return AttemptView{}, err
		}
		if a, err = s.repo.FindAttempt(ctx, s.db, tenantID, userID, id, false); err != nil {
			return AttemptView{}, runtimeError(err)
		}
		if parts, err = s.repo.Parts(ctx, s.db, tenantID, id); err != nil {
			return AttemptView{}, runtimeError(err)
		}
	}
	return s.assemble(ctx, a, parts)
}

func (s *RuntimeService) assemble(ctx context.Context, a model.Attempt, parts []model.AttemptPart) (AttemptView, error) {
	out := AttemptView{Attempt: a, ServerTime: s.now()}
	for _, p := range parts {
		out.Parts = append(out.Parts, partView(a, p))
	}
	if a.Status == model.SessionCompleted {
		res, err := s.resultOf(ctx, a, parts)
		if err != nil {
			return AttemptView{}, err
		}
		out.Result = &res
		for i := range out.Parts {
			score := percent(deref(out.Parts[i].EarnedPoints), deref(out.Parts[i].MaxPoints))
			out.Parts[i].Score = &score
		}
	}
	return out, nil
}

func partView(a model.Attempt, p model.AttemptPart) PartView {
	v := PartView{AttemptPart: p}
	if p.DeadlineAt != nil {
		d := visibleDeadline(a, p)
		v.Deadline = &d
	}
	return v
}

func (s *RuntimeService) resultOf(ctx context.Context, a model.Attempt, parts []model.AttemptPart) (model.Result, error) {
	res, err := s.repo.Result(ctx, s.db, a.TenantID, a.ID)
	if err != nil {
		return model.Result{}, runtimeError(err)
	}
	for _, p := range parts {
		res.Parts = append(res.Parts, model.PartResult{
			SubtestID: p.SubtestID, Name: p.Name, Position: p.Position,
			EarnedPoints: deref(p.EarnedPoints), MaxPoints: deref(p.MaxPoints),
			Score:        percent(deref(p.EarnedPoints), deref(p.MaxPoints)),
			CorrectCount: derefInt(p.CorrectCount), QuestionCount: p.QuestionCount, SubmitReason: p.SubmitReason,
		})
	}
	return res, nil
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// Session is the owner's view of one session.
func (s *RuntimeService) Session(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (AttemptView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return AttemptView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, id)
}

// Result is the immediate basic result of a completed session.
func (s *RuntimeService) Result(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Result, error) {
	view, err := s.Session(ctx, scope, principal, id)
	if err != nil {
		return model.Result{}, err
	}
	if view.Result == nil {
		return model.Result{}, ErrNotCompleted
	}
	return *view.Result, nil
}

// Subtest returns the questions of the running subtest with the student's saved answers. A part that is pending,
// submitted or expired is refused: a submitted subtest can never be reopened, by URL or otherwise.
func (s *RuntimeService) Subtest(ctx context.Context, scope HostScope, principal model.Principal, sessionID, subtestID uuid.UUID) (SubtestView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return SubtestView{}, err
	}
	view, err := s.view(ctx, tenantID, principal.UserID, sessionID)
	if err != nil {
		return SubtestView{}, err
	}
	part, ok := findPart(view.Parts, subtestID)
	if !ok {
		return SubtestView{}, ErrSessionNotFound
	}
	if part.Status != model.PartRunning {
		return SubtestView{}, ErrPartNotRunning
	}
	questions, err := s.repo.Questions(ctx, s.db, tenantID, sessionID, &subtestID)
	if err != nil {
		return SubtestView{}, runtimeError(err)
	}
	saved, err := s.repo.Answers(ctx, s.db, tenantID, sessionID, &subtestID)
	if err != nil {
		return SubtestView{}, runtimeError(err)
	}
	answers := make(map[uuid.UUID]model.AttemptAnswer, len(saved))
	for _, a := range saved {
		answers[a.QuestionID] = a
	}
	return SubtestView{Attempt: view.Attempt, Part: part, Questions: questions, Answers: answers, ServerTime: view.ServerTime}, nil
}

func findPart(parts []PartView, subtestID uuid.UUID) (PartView, bool) {
	for _, p := range parts {
		if p.SubtestID == subtestID {
			return p, true
		}
	}
	return PartView{}, false
}

// StartSubtest begins the next subtest on the student's explicit action. Starting a running one is a no-op, so a
// retried request cannot move its deadline; a submitted one cannot be started again.
func (s *RuntimeService) StartSubtest(ctx context.Context, scope HostScope, principal model.Principal, sessionID, subtestID uuid.UUID) (AttemptView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return AttemptView{}, err
	}
	err = s.run(ctx, func(tx pgx.Tx) error {
		a, err := s.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true)
		if err != nil {
			return runtimeError(err)
		}
		now := s.now()
		if _, err := s.settleLocked(ctx, tx, a, now); err != nil {
			return err
		}
		if a, err = s.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true); err != nil {
			return runtimeError(err)
		}
		if a.Status != model.SessionInProgress {
			return keep{ErrSessionClosed}
		}
		parts, err := s.repo.Parts(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		var target *model.AttemptPart
		for i := range parts {
			if parts[i].SubtestID == subtestID {
				target = &parts[i]
			}
		}
		if target == nil {
			return ErrSessionNotFound
		}
		switch target.Status {
		case model.PartRunning:
			return nil
		case model.PartSubmitted:
			return keep{ErrPartNotRunning}
		}
		for _, p := range parts {
			if p.Position < target.Position && p.Status != model.PartSubmitted {
				return keep{ErrPartOrder}
			}
			if p.Status == model.PartRunning {
				return keep{ErrPartOrder}
			}
		}
		deadline := now.Add(time.Duration(target.DurationSeconds) * time.Second)
		if deadline.After(a.ActiveUntil) {
			deadline = a.ActiveUntil
		}
		return runtimeError(s.repo.StartPart(ctx, tx, tenantID, sessionID, subtestID, now, deadline))
	})
	if err != nil {
		return AttemptView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, sessionID)
}

// SubmitSubtest ends the running subtest on the student's action. A part already settled, by an earlier submit or by
// its deadline, answers with the settled state, so a retry is harmless.
func (s *RuntimeService) SubmitSubtest(ctx context.Context, scope HostScope, principal model.Principal, sessionID, subtestID uuid.UUID) (AttemptView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return AttemptView{}, err
	}
	err = s.run(ctx, func(tx pgx.Tx) error {
		a, err := s.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true)
		if err != nil {
			return runtimeError(err)
		}
		now := s.now()
		if _, err := s.settleLocked(ctx, tx, a, now); err != nil {
			return err
		}
		parts, err := s.repo.Parts(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		var target *model.AttemptPart
		for i := range parts {
			if parts[i].SubtestID == subtestID {
				target = &parts[i]
			}
		}
		if target == nil {
			return ErrSessionNotFound
		}
		switch target.Status {
		case model.PartSubmitted:
			return nil
		case model.PartPending:
			return keep{ErrPartNotRunning}
		}
		if a, err = s.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true); err != nil {
			return runtimeError(err)
		}
		if err := s.settlePart(ctx, tx, a, *target, model.SubmitStudent, now); err != nil {
			return err
		}
		target.Status = model.PartSubmitted
		_, err = s.maybeFinalize(ctx, tx, a, parts, now)
		return err
	})
	if err != nil {
		return AttemptView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, sessionID)
}

// SaveAnswer admits one answer by server time. The client's per-question sequence is both the retry identity and the
// reorder guard: a resend of the stored write is acknowledged unchanged, an older one is refused, a newer one wins.
func (s *RuntimeService) SaveAnswer(ctx context.Context, scope HostScope, principal model.Principal, sessionID, questionID uuid.UUID, in AnswerInput) (AnswerAck, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return AnswerAck{}, err
	}
	if in.Seq < 1 {
		return AnswerAck{}, &InputError{Fields: map[string]string{"seq": ReasonAnswerSeq}}
	}
	var ack AnswerAck
	err = s.run(ctx, func(tx pgx.Tx) error {
		a, err := s.repo.FindAttempt(ctx, tx, tenantID, principal.UserID, sessionID, true)
		if err != nil {
			return runtimeError(err)
		}
		now := s.now()
		if _, err := s.settleLocked(ctx, tx, a, now); err != nil {
			return err
		}
		question, err := s.repo.Question(ctx, tx, tenantID, sessionID, questionID)
		if err != nil {
			return runtimeError(err)
		}
		parts, err := s.repo.Parts(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		var part model.AttemptPart
		for _, p := range parts {
			if p.SubtestID == question.SubtestID {
				part = p
			}
		}
		if part.Status != model.PartRunning {
			if part.SubmitReason == model.SubmitExpired {
				return keep{ErrDeadlinePassed}
			}
			return keep{ErrPartNotRunning}
		}
		clean, err := checkedAnswer(question, in.Answer)
		if err != nil {
			return err
		}

		revision := 1
		current, err := s.repo.Answer(ctx, tx, tenantID, sessionID, questionID)
		switch {
		case errors.Is(err, repository.ErrNotFound):
		case err != nil:
			return runtimeError(err)
		default:
			same := sameAnswer(current.Answer, clean)
			if in.Seq < current.Seq || (in.Seq == current.Seq && !same) {
				return ErrStaleAnswer
			}
			revision = current.Revision
			if in.Seq == current.Seq {
				ack = AnswerAck{QuestionID: questionID, Revision: revision, Seq: in.Seq, ServerTime: now}
				return nil
			}
			if !same {
				revision++
			}
		}
		if err := s.repo.PutAnswer(ctx, tx, tenantID, sessionID, model.AttemptAnswer{
			QuestionID: questionID, Answer: clean, Revision: revision, Seq: in.Seq, UpdatedAt: now,
		}); err != nil {
			if errors.Is(err, repository.ErrStaleWrite) {
				return ErrStaleAnswer
			}
			return runtimeError(err)
		}
		ack = AnswerAck{QuestionID: questionID, Revision: revision, Seq: in.Seq, ServerTime: now}
		return nil
	})
	return ack, err
}

// checkedAnswer validates an answer against the snapshotted question and returns its stored form. An empty answer
// is how a student clears a question.
func checkedAnswer(q model.AttemptQuestion, answer []string) ([]string, error) {
	bad := &InputError{Fields: map[string]string{"answer": ReasonAnswerShape}}
	if answer == nil {
		answer = []string{}
	}
	ids := make(map[string]bool, len(q.Options))
	for _, o := range q.Options {
		ids[o.ID] = true
	}
	switch q.Type {
	case model.QuestionIsian:
		if len(answer) > 1 {
			return nil, bad
		}
		if len(answer) == 1 {
			if utf8.RuneCountInString(answer[0]) > maxAnswerLen {
				return nil, bad
			}
			if strings.TrimSpace(answer[0]) == "" {
				return []string{}, nil
			}
		}
		return answer, nil
	case model.QuestionPG, model.QuestionTrueFalse:
		if len(answer) > 1 || (len(answer) == 1 && !ids[answer[0]]) {
			return nil, bad
		}
		return answer, nil
	case model.QuestionPGMulti:
		seen := map[string]bool{}
		for _, id := range answer {
			if !ids[id] || seen[id] {
				return nil, bad
			}
			seen[id] = true
		}
		return answer, nil
	}
	return nil, bad
}

func sameAnswer(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// Extend is D10's staff allowance: seconds added to the subtest that is running in one student's session, audited
// with its subtest. Later subtests keep their normal timing and a settled part is never reopened; with no part
// running there is nothing to extend.
func (s *RuntimeService) Extend(ctx context.Context, scope HostScope, principal model.Principal, sessionID uuid.UUID, in ExtensionInput) (model.Extension, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Extension{}, err
	}
	if principal.Role != model.RoleAdmin && principal.Role != model.RoleOperator {
		return model.Extension{}, ErrRuntimeForbidden
	}
	in.Reason = strings.TrimSpace(in.Reason)
	fields := map[string]string{}
	if in.Seconds < 1 || in.Seconds > maxExtensionSeconds {
		fields["seconds"] = ReasonExtraSeconds
	}
	if n := utf8.RuneCountInString(in.Reason); n < 1 || n > maxExtraReasonLen {
		fields["reason"] = ReasonExtraReason
	}
	if len(fields) > 0 {
		return model.Extension{}, &InputError{Fields: fields}
	}
	var out model.Extension
	err = s.run(ctx, func(tx pgx.Tx) error {
		a, err := s.repo.FindAttempt(ctx, tx, tenantID, uuid.Nil, sessionID, true)
		if err != nil {
			return runtimeError(err)
		}
		if _, err := s.settleLocked(ctx, tx, a, s.now()); err != nil {
			return err
		}
		if a, err = s.repo.FindAttempt(ctx, tx, tenantID, uuid.Nil, sessionID, true); err != nil {
			return runtimeError(err)
		}
		if a.Status != model.SessionInProgress {
			return keep{ErrSessionClosed}
		}
		parts, err := s.repo.Parts(ctx, tx, tenantID, sessionID)
		if err != nil {
			return runtimeError(err)
		}
		running := -1
		for i, p := range parts {
			if p.Status == model.PartRunning {
				running = i
			}
		}
		if running < 0 {
			return keep{ErrPartNotRunning}
		}
		if parts[running].ExtraSeconds+in.Seconds > maxExtraTotal {
			return &InputError{Fields: map[string]string{"seconds": ReasonExtraTotal}}
		}
		subtestID, total, err := s.repo.AddExtra(ctx, tx, tenantID, sessionID, principal.UserID, in.Seconds, in.Reason)
		if err != nil {
			return runtimeError(err)
		}
		out = model.Extension{SessionID: sessionID, SubtestID: subtestID, Seconds: in.Seconds, Reason: in.Reason, GrantedBy: principal.UserID, TotalExtra: total}
		return nil
	})
	return out, err
}

// FinalizeDue is the deadline worker's tick: it settles every session whose deadline has passed, whether or not
// its tab is open. PostgreSQL is the only source of "due"; the call is idempotent and safe to run from several
// workers at once, because each session settles under its own row lock and re-checks what it finds there.
func (s *RuntimeService) FinalizeDue(ctx context.Context) (int, error) {
	due, err := s.repo.DueSessions(ctx, s.db, s.now(), dueBatch)
	if err != nil {
		return 0, err
	}
	settled := 0
	var first error
	for _, d := range due {
		changed := false
		err := s.run(ctx, func(tx pgx.Tx) error {
			a, err := s.repo.FindAttempt(ctx, tx, d.TenantID, uuid.Nil, d.ID, true)
			if err != nil {
				return runtimeError(err)
			}
			changed, err = s.settleLocked(ctx, tx, a, s.now())
			return err
		})
		if err != nil {
			if ctx.Err() != nil {
				return settled, err
			}
			// One session that fails must not hold back the others in the batch.
			if first == nil {
				first = err
			}
			continue
		}
		if changed {
			settled++
		}
	}
	return settled, first
}

// SettingsInput is what an Admin saves: D04's scoring weights and D12's minimum top-up.
type SettingsInput struct {
	Weights  model.Weights
	MinTopup string
}

// Settings reads the tenant's scoring weights and minimum top-up.
func (s *RuntimeService) Settings(ctx context.Context, scope HostScope, principal model.Principal) (model.Settings, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Settings{}, err
	}
	if principal.Role != model.RoleAdmin {
		return model.Settings{}, ErrRuntimeForbidden
	}
	w, err := s.repo.Weights(ctx, s.db, tenantID)
	if err != nil {
		return model.Settings{}, runtimeError(err)
	}
	minimum, err := s.repo.MinTopup(ctx, s.db, tenantID)
	if err != nil {
		return model.Settings{}, runtimeError(err)
	}
	return model.Settings{Weights: w, MinTopup: minimum}, nil
}

// UpdateSettings stores the weights new attempts will freeze, so sessions already started keep theirs, and the
// minimum the next top-up request is checked against.
func (s *RuntimeService) UpdateSettings(ctx context.Context, scope HostScope, principal model.Principal, in SettingsInput) (model.Settings, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Settings{}, err
	}
	if principal.Role != model.RoleAdmin {
		return model.Settings{}, ErrRuntimeForbidden
	}
	fields := map[string]string{}
	for name, w := range map[string]float64{"easy": in.Weights.Easy, "medium": in.Weights.Medium, "hard": in.Weights.Hard} {
		if w <= 0 || w > maxWeight || round2(w) != w {
			fields["weights."+name] = ReasonWeight
		}
	}
	minimum := ""
	if in.MinTopup != "" {
		var ok bool
		if minimum, ok = parseWholeRupiah(in.MinTopup); !ok {
			fields["min_topup"] = ReasonMinTopup
		}
	}
	if len(fields) > 0 {
		return model.Settings{}, &InputError{Fields: fields}
	}
	if err := s.repo.SetSettings(ctx, tenantID, principal.UserID, in.Weights, minimum); err != nil {
		return model.Settings{}, runtimeError(err)
	}
	current, err := s.repo.MinTopup(ctx, s.db, tenantID)
	if err != nil {
		return model.Settings{}, runtimeError(err)
	}
	return model.Settings{Weights: in.Weights, MinTopup: current}, nil
}
