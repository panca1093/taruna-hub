package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrKecNotEnrolled     = errors.New("no kecermatan enrollment for this test")
	ErrKecNoCredit        = errors.New("no kecermatan attempts left")
	ErrKecSessionNotFound = errors.New("kecermatan attempt not found")
	ErrKecSessionClosed   = errors.New("kecermatan attempt already completed")
	ErrKecKolomNotRunning = errors.New("kolom is not running")
	ErrKecAnswerLocked    = errors.New("soal already answered differently")
	ErrKecNotCompleted    = errors.New("kecermatan attempt not completed")
)

const (
	ReasonKecAnswer      = "kecermatan_answer"
	ReasonKecAnswerSoal  = "kecermatan_answer_soal"
	ReasonKecResultQuery = "kecermatan_result_query"
)

// KecermatanPauseSeconds is the jeda between two kolom, as the mockup shows it (a 5 second countdown with a
// "Lanjut" button). The next kolom starts at the earlier of the student's press and the end of this pause, and
// the clock of that kolom starts then, so a closed tab never gains time.
const KecermatanPauseSeconds = 5

const (
	kecDueBatch         = 50
	defaultKecResultLen = 50
	maxKecResultLen     = 100
)

const (
	KecPhaseKolom     = "kolom"
	KecPhasePause     = "pause"
	KecPhaseCompleted = "completed"
)

// KecSoalView is one soal as a student sees it: the shown labels only. The missing item is never here.
type KecSoalView struct {
	Number int
	Shown  []string
}

// KecAnswerView is a stored answer of the running kolom; nil is a skip.
type KecAnswerView struct {
	Soal   int
	Answer *string
}

// KecKolomView is the one running kolom: its reference set, its soal and what was answered so far.
type KecKolomView struct {
	Number     int
	StartedAt  time.Time
	DeadlineAt time.Time
	Reference  []KecItem
	Soal       []KecSoalView
	Answers    []KecAnswerView
}

// KecSessionView is the student's state of one attempt. Only the running kolom's content is ever included.
type KecSessionView struct {
	Session       model.KecSession
	ServerTime    time.Time
	Phase         string
	Kolom         *KecKolomView
	FinishedKolom int
	NextKolom     int
	NextStartsAt  *time.Time
	Result        *model.KecResultRow
}

// KecOverview is the student's page of one test.
type KecOverview struct {
	Test      model.KecStudentTest
	Current   *KecSessionView
	Attempts  []model.KecAttempt
	ProductID *uuid.UUID
}

// KecResultView is a completed attempt's authoritative counts and composite scores.
type KecResultView struct {
	Result model.KecResultRow
	Kolom  []model.KecKolom
}

// KecReviewKolom is one kolom of the detailed review: which generated column it replays and what was answered.
type KecReviewKolom struct {
	Number       int
	SourceColumn int
	Benar        int
	Salah        int
	Kosong       int
	Answers      []*string
}

// KecReview reproduces a completed attempt exactly: the generated columns, and per kolom the answers.
type KecReview struct {
	Result  model.KecResultRow
	Columns []KecColumn
	Kolom   []KecReviewKolom
}

// KecAnswerAck confirms a stored answer.
type KecAnswerAck struct {
	Kolom      int
	Soal       int
	Answer     *string
	ServerTime time.Time
}

type KecResultFilter struct {
	TestID    *uuid.UUID
	ProgramID *uuid.UUID
	Limit     int
	Offset    int
}

type KecermatanRuntimeOption func(*KecermatanRuntimeService)

func WithKecermatanRuntimeClock(now func() time.Time) KecermatanRuntimeOption {
	return func(s *KecermatanRuntimeService) { s.now = now }
}

// WithKecermatanNonce replaces the fresh randomness an attempt seed derives from; tests pin it.
func WithKecermatanNonce(nonce func() ([]byte, error)) KecermatanRuntimeOption {
	return func(s *KecermatanRuntimeService) { s.nonce = nonce }
}

type KecermatanRuntimeService struct {
	db       *pgxpool.Pool
	repo     *repository.KecermatanRuntimeRepository
	tests    *KecermatanService
	testRepo *repository.KecermatanRepository
	now      func() time.Time
	nonce    func() ([]byte, error)
}

func NewKecermatanRuntimeService(db *pgxpool.Pool, repo *repository.KecermatanRuntimeRepository, tests *KecermatanService, testRepo *repository.KecermatanRepository, opts ...KecermatanRuntimeOption) *KecermatanRuntimeService {
	s := &KecermatanRuntimeService{db: db, repo: repo, tests: tests, testRepo: testRepo, now: time.Now, nonce: freshNonce}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func freshNonce() ([]byte, error) {
	raw := make([]byte, seedBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("draw kecermatan nonce: %w", err)
	}
	return raw, nil
}

// deriveKecermatanAttemptSeed binds an attempt to its test without being computable from it: an HMAC of fresh
// randomness keyed with the test's stored seed. Neither the stored seed nor the nonce is ever sent out, and
// the derived seed is persisted with the attempt, so replaying it never needs the nonce again.
func deriveKecermatanAttemptSeed(testSeed string, nonce []byte) (string, error) {
	key, err := hex.DecodeString(testSeed)
	if err != nil || len(key) != seedBytes {
		return "", errors.New("kecermatan test seed must be 16 bytes of hex")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(nonce)
	return hex.EncodeToString(mac.Sum(nil)[:seedBytes]), nil
}

// Tests lists the kecermatan tests the student holds credits for.
func (s *KecermatanRuntimeService) Tests(ctx context.Context, scope HostScope, principal model.Principal) ([]model.KecStudentTest, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return nil, err
	}
	return s.repo.StudentTests(ctx, tenantID, principal.UserID)
}

// Overview is the student's page of one test; a test the student holds no credit for answers not found.
func (s *KecermatanRuntimeService) Overview(ctx context.Context, scope HostScope, principal model.Principal, testID uuid.UUID) (KecOverview, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecOverview{}, err
	}
	test, err := s.repo.StudentTest(ctx, tenantID, principal.UserID, testID)
	if errors.Is(err, repository.ErrNotFound) {
		return KecOverview{}, ErrKecNotEnrolled
	}
	if err != nil {
		return KecOverview{}, kecError(err)
	}
	out := KecOverview{Test: test}
	if active, err := s.repo.ActiveSession(ctx, s.db, tenantID, principal.UserID, testID, false); err == nil {
		view, err := s.view(ctx, tenantID, principal.UserID, active.ID)
		if err != nil {
			return KecOverview{}, err
		}
		if view.Session.Status == model.KecSessionInProgress {
			out.Current = &view
		}
		// The settle may have completed the attempt and used a credit.
		if test, err = s.repo.StudentTest(ctx, tenantID, principal.UserID, testID); err != nil {
			return KecOverview{}, kecError(err)
		}
		out.Test = test
	} else if !errors.Is(err, repository.ErrNotFound) {
		return KecOverview{}, kecError(err)
	}
	if out.Attempts, err = s.repo.Attempts(ctx, s.db, tenantID, principal.UserID, testID); err != nil {
		return KecOverview{}, kecError(err)
	}
	if id, err := s.repo.SellableProduct(ctx, tenantID, testID, s.now()); err == nil {
		out.ProductID = &id
	} else if !errors.Is(err, repository.ErrNotFound) {
		return KecOverview{}, kecError(err)
	}
	return out, nil
}

// Start admits an attempt. Under the enrollment row lock it either returns the attempt already in progress
// (a retry, a second tab, a resume: nothing is consumed) or, with at least one credit left, freezes a new attempt
// and starts its first kolom. A credit is only consumed when an attempt completes (D05). The bool is true for
// a newly created attempt.
func (s *KecermatanRuntimeService) Start(ctx context.Context, scope HostScope, principal model.Principal, testID uuid.UUID) (KecSessionView, bool, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecSessionView{}, false, err
	}
	var id uuid.UUID
	created := false
	err = runKeep(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := s.repo.LockCredit(ctx, tx, tenantID, principal.UserID, testID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrKecNotEnrolled
			}
			return kecError(err)
		}
		now := s.now()
		active, err := s.repo.ActiveSession(ctx, tx, tenantID, principal.UserID, testID, true)
		switch {
		case err == nil:
			if _, err := s.settleLocked(ctx, tx, active, now); err != nil {
				return err
			}
			if active, err = s.repo.FindSession(ctx, tx, tenantID, principal.UserID, active.ID, false); err != nil {
				return kecError(err)
			}
			if active.Status == model.KecSessionInProgress {
				id = active.ID
				return nil
			}
		case !errors.Is(err, repository.ErrNotFound):
			return kecError(err)
		}
		// Read after the possible settle above: it may have just consumed this attempt's credit.
		credit, err := s.repo.Credit(ctx, tx, tenantID, principal.UserID, testID)
		if err != nil {
			return kecError(err)
		}
		if credit.Remaining() < 1 {
			return keep{ErrKecNoCredit}
		}
		if id, err = s.admit(ctx, tx, tenantID, principal.UserID, testID, now); err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return KecSessionView{}, false, err
	}
	view, err := s.view(ctx, tenantID, principal.UserID, id)
	return view, created, err
}

// admit snapshots the test into a new attempt and starts kolom 1. The attempt's seed derives from fresh
// randomness, so every attempt differs and the stored test seed stays server-side.
func (s *KecermatanRuntimeService) admit(ctx context.Context, tx pgx.Tx, tenantID, userID, testID uuid.UUID, now time.Time) (uuid.UUID, error) {
	if err := s.repo.LockTest(ctx, tx, tenantID, testID, now); err != nil {
		return uuid.Nil, kecError(err)
	}
	test, err := s.testRepo.Find(ctx, tx, tenantID, testID)
	if err != nil {
		return uuid.Nil, kecError(err)
	}
	pool, err := s.tests.Pool(ctx, tx, test)
	if err != nil {
		return uuid.Nil, err
	}
	nonce, err := s.nonce()
	if err != nil {
		return uuid.Nil, err
	}
	seed, err := deriveKecermatanAttemptSeed(test.Seed, nonce)
	if err != nil {
		return uuid.Nil, err
	}
	gen, err := GenerateKecermatan(seed, pool, ParamsOf(test))
	if err != nil {
		return uuid.Nil, err
	}
	columns, err := json.Marshal(gen.Columns)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encode kecermatan snapshot: %w", err)
	}
	number, err := s.repo.NextAttemptNumber(ctx, tx, tenantID, userID, testID)
	if err != nil {
		return uuid.Nil, kecError(err)
	}
	duration := time.Duration(test.KolomDurationSeconds) * time.Second
	deadline := now.Add(duration)
	id, err := s.repo.CreateSession(ctx, tx, model.KecSession{
		TenantID: tenantID, UserID: userID, TestID: testID, AttemptNumber: number, Seed: seed,
		TestRevision: test.Revision, KolomCount: test.KolomCount, SoalPerKolom: test.SoalPerKolom,
		OptionsPerSoal: test.OptionsPerSoal, KolomDurationSeconds: test.KolomDurationSeconds,
		PauseSeconds: KecermatanPauseSeconds, DueAt: &deadline, StartedAt: now,
	}, columns)
	if err != nil {
		return uuid.Nil, kecError(err)
	}
	return id, kecError(s.repo.StartKolom(ctx, tx, tenantID, id, 1, now, deadline))
}

// Session is the student's current state of an attempt.
func (s *KecermatanRuntimeService) Session(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (KecSessionView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecSessionView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, id)
}

// Continue is "Lanjut": it starts the next kolom now instead of waiting out the pause. Repeating it, or calling
// it when no pause is running, changes nothing.
func (s *KecermatanRuntimeService) Continue(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (KecSessionView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecSessionView{}, err
	}
	err = s.withSession(ctx, tenantID, principal.UserID, id, func(tx pgx.Tx, sess model.KecSession, now time.Time) error {
		if sess.Status != model.KecSessionInProgress {
			return nil
		}
		kolom, err := s.repo.Kolom(ctx, tx, tenantID, id)
		if err != nil {
			return kecError(err)
		}
		next := nextOpenKolom(kolom)
		if next == nil || next.Status != model.KecKolomPending {
			return nil
		}
		deadline := now.Add(time.Duration(sess.KolomDurationSeconds) * time.Second)
		if err := s.repo.StartKolom(ctx, tx, tenantID, id, next.Number, now, deadline); err != nil {
			return kecError(err)
		}
		return kecError(s.repo.SetDue(ctx, tx, tenantID, id, deadline))
	})
	if err != nil {
		return KecSessionView{}, err
	}
	return s.view(ctx, tenantID, principal.UserID, id)
}

// Answer stores one soal's answer, or a skip when answer is nil. Answers are write-once per soal: repeating the
// same one is a harmless retry, a different one answers ErrKecAnswerLocked. Only the running kolom accepts
// answers and only before its server deadline; settling runs first, so an expiry noticed here is committed.
func (s *KecermatanRuntimeService) Answer(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, kolom, soal int, answer *string) (KecAnswerAck, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecAnswerAck{}, err
	}
	var ack KecAnswerAck
	err = s.withSession(ctx, tenantID, principal.UserID, id, func(tx pgx.Tx, sess model.KecSession, now time.Time) error {
		if sess.Status != model.KecSessionInProgress {
			return keep{ErrKecSessionClosed}
		}
		fields := map[string]string{}
		if soal < 1 || soal > sess.SoalPerKolom {
			fields["soal"] = ReasonKecAnswerSoal
		}
		if answer != nil && !validKecLabel(*answer, sess.OptionsPerSoal) {
			fields["answer"] = ReasonKecAnswer
		}
		if len(fields) > 0 {
			return &InputError{Fields: fields}
		}
		kolomRows, err := s.repo.Kolom(ctx, tx, tenantID, id)
		if err != nil {
			return kecError(err)
		}
		if kolom < 1 || kolom > len(kolomRows) || kolomRows[kolom-1].Status != model.KecKolomRunning {
			return keep{ErrKecKolomNotRunning}
		}
		stored, _, err := s.repo.PutAnswer(ctx, tx, tenantID, id, kolom, soal, answer, now)
		if err != nil {
			return kecError(err)
		}
		if !sameLabel(stored, answer) {
			return keep{ErrKecAnswerLocked}
		}
		ack = KecAnswerAck{Kolom: kolom, Soal: soal, Answer: stored, ServerTime: now}
		return nil
	})
	return ack, err
}

// Result is a completed attempt's counts and composite scores; only its owner reads it.
func (s *KecermatanRuntimeService) Result(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (KecResultView, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecResultView{}, err
	}
	return s.completed(ctx, tenantID, principal.UserID, id)
}

func (s *KecermatanRuntimeService) completed(ctx context.Context, tenantID, userID, id uuid.UUID) (KecResultView, error) {
	var out KecResultView
	err := s.withSession(ctx, tenantID, userID, id, func(tx pgx.Tx, sess model.KecSession, _ time.Time) error {
		if sess.Status != model.KecSessionCompleted {
			return keep{ErrKecNotCompleted}
		}
		res, err := s.repo.Result(ctx, tx, tenantID, uuid.Nil, id)
		if err != nil {
			return kecError(err)
		}
		kolom, err := s.repo.Kolom(ctx, tx, tenantID, id)
		if err != nil {
			return kecError(err)
		}
		out = KecResultView{Result: res, Kolom: kolom}
		return nil
	})
	return out, err
}

// Review is the detailed review of a completed attempt. An attempt in progress has none, so a key can never
// be read while it can still matter.
func (s *KecermatanRuntimeService) Review(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (KecReview, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return KecReview{}, err
	}
	var out KecReview
	err = s.withSession(ctx, tenantID, principal.UserID, id, func(tx pgx.Tx, sess model.KecSession, _ time.Time) error {
		if sess.Status != model.KecSessionCompleted {
			return keep{ErrKecNotCompleted}
		}
		res, err := s.repo.Result(ctx, tx, tenantID, uuid.Nil, id)
		if err != nil {
			return kecError(err)
		}
		gen, err := s.snapshot(ctx, tx, sess)
		if err != nil {
			return err
		}
		kolom, err := s.repo.Kolom(ctx, tx, tenantID, id)
		if err != nil {
			return kecError(err)
		}
		stored, err := s.repo.Answers(ctx, tx, tenantID, id, 0)
		if err != nil {
			return kecError(err)
		}
		byKolom := map[int][]*string{}
		for _, a := range stored {
			if byKolom[a.Kolom] == nil {
				byKolom[a.Kolom] = make([]*string, sess.SoalPerKolom)
			}
			byKolom[a.Kolom][a.Soal-1] = a.Answer
		}
		out = KecReview{Result: res, Columns: gen.Columns}
		for _, k := range kolom {
			_, source := gen.Slot(k.Number)
			answers := byKolom[k.Number]
			if answers == nil {
				answers = make([]*string, sess.SoalPerKolom)
			}
			out.Kolom = append(out.Kolom, KecReviewKolom{
				Number: k.Number, SourceColumn: source, Benar: intOf(k.Benar), Salah: intOf(k.Salah),
				Kosong: intOf(k.Kosong), Answers: answers,
			})
		}
		return nil
	})
	return out, err
}

// StaffResults lists every completed attempt of the tenant.
func (s *KecermatanRuntimeService) StaffResults(ctx context.Context, scope HostScope, principal model.Principal, filter KecResultFilter) ([]model.KecResultRow, int, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	if !isStaff(principal.Role) {
		return nil, 0, ErrRuntimeForbidden
	}
	fields := map[string]string{}
	if filter.Limit == 0 {
		filter.Limit = defaultKecResultLen
	}
	if filter.Limit < 1 || filter.Limit > maxKecResultLen {
		fields["limit"] = ReasonKecResultQuery
	}
	if filter.Offset < 0 {
		fields["offset"] = ReasonKecResultQuery
	}
	if len(fields) > 0 {
		return nil, 0, &InputError{Fields: fields}
	}
	return s.repo.ListResults(ctx, tenantID, repository.KecermatanResultFilter{
		TestID: filter.TestID, ProgramID: filter.ProgramID, Limit: filter.Limit, Offset: filter.Offset,
	})
}

// StaffResult is one completed attempt with its per-kolom counts.
func (s *KecermatanRuntimeService) StaffResult(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (KecResultView, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return KecResultView{}, err
	}
	if !isStaff(principal.Role) {
		return KecResultView{}, ErrRuntimeForbidden
	}
	res, err := s.repo.Result(ctx, s.db, tenantID, uuid.Nil, id)
	if err != nil {
		return KecResultView{}, kecError(err)
	}
	kolom, err := s.repo.Kolom(ctx, s.db, tenantID, id)
	if err != nil {
		return KecResultView{}, kecError(err)
	}
	return KecResultView{Result: res, Kolom: kolom}, nil
}

// FinalizeDue is the deadline worker: PostgreSQL decides what is due and each attempt settles in its own
// transaction, so a restart or a missed tick only delays it. Settling an expired attempt records its unanswered
// soal as kosong, writes the result and consumes the credit, all in that one transaction.
func (s *KecermatanRuntimeService) FinalizeDue(ctx context.Context) (int, error) {
	due, err := s.repo.DueSessions(ctx, s.now(), kecDueBatch)
	if err != nil {
		return 0, err
	}
	settled := 0
	var first error
	for _, d := range due {
		changed := false
		err := s.withSession(ctx, d.TenantID, uuid.Nil, d.ID, func(_ pgx.Tx, _ model.KecSession, _ time.Time) error {
			changed = true
			return nil
		})
		if err != nil {
			if ctx.Err() != nil {
				return settled, err
			}
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

// RecomputeScores rewrites the composite scores of every result of a tenant under another scoring version. It
// reads only the immutable per-kolom counts, so a revised client formula never touches what happened in an attempt.
func (s *KecermatanRuntimeService) RecomputeScores(ctx context.Context, tenantID uuid.UUID, version int) (int, error) {
	if _, err := ScoreKecermatan(version, make([]KecermatanKolomCount, 2)); err != nil {
		return 0, err
	}
	after := uuid.Nil
	total := 0
	for {
		ids, err := s.repo.ResultSessions(ctx, s.db, tenantID, after, 200)
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}
		for _, id := range ids {
			kolom, err := s.repo.Kolom(ctx, s.db, tenantID, id)
			if err != nil {
				return total, err
			}
			scores, err := ScoreKecermatan(version, countsOf(kolom))
			if err != nil {
				return total, err
			}
			if err := s.repo.UpdateScores(ctx, s.db, tenantID, id, version, scores.Kecepatan, scores.Ketahanan,
				scores.Ketelitian, scores.NilaiAkhir, [3]float64{scores.KecepatanRaw, scores.KetahananRaw, scores.KetelitianRaw}); err != nil {
				return total, err
			}
			total++
			after = id
		}
	}
}

// withSession runs fn with the attempt settled to now. A zero userID is the worker's. The lock order is the same
// for every caller: the enrollment row (the credit), then the session row. A completed attempt needs neither
// lock, because nothing can change it any more.
func (s *KecermatanRuntimeService) withSession(ctx context.Context, tenantID, userID, id uuid.UUID, fn func(pgx.Tx, model.KecSession, time.Time) error) error {
	head, err := s.repo.FindSession(ctx, s.db, tenantID, userID, id, false)
	if err != nil {
		return kecError(err)
	}
	return runKeep(ctx, s.db, func(tx pgx.Tx) error {
		sess := head
		if head.Status == model.KecSessionInProgress {
			if _, err := s.repo.LockCredit(ctx, tx, tenantID, head.UserID, head.TestID); err != nil {
				return kecError(err)
			}
			locked, err := s.repo.FindSession(ctx, tx, tenantID, userID, id, true)
			if err != nil {
				return kecError(err)
			}
			now := s.now()
			if _, err := s.settleLocked(ctx, tx, locked, now); err != nil {
				return err
			}
			if sess, err = s.repo.FindSession(ctx, tx, tenantID, userID, id, false); err != nil {
				return kecError(err)
			}
			return fn(tx, sess, now)
		}
		return fn(tx, sess, s.now())
	})
}

// view reads the attempt settled to now and builds the student's state.
func (s *KecermatanRuntimeService) view(ctx context.Context, tenantID, userID, id uuid.UUID) (KecSessionView, error) {
	var out KecSessionView
	err := s.withSession(ctx, tenantID, userID, id, func(tx pgx.Tx, sess model.KecSession, now time.Time) error {
		var err error
		out, err = s.build(ctx, tx, sess, now)
		return err
	})
	return out, err
}

func (s *KecermatanRuntimeService) build(ctx context.Context, tx pgx.Tx, sess model.KecSession, now time.Time) (KecSessionView, error) {
	out := KecSessionView{Session: sess, ServerTime: now, Phase: KecPhaseCompleted}
	if sess.Status == model.KecSessionCompleted {
		res, err := s.repo.Result(ctx, tx, sess.TenantID, uuid.Nil, sess.ID)
		if err != nil {
			return KecSessionView{}, kecError(err)
		}
		out.Result = &res
		return out, nil
	}
	kolom, err := s.repo.Kolom(ctx, tx, sess.TenantID, sess.ID)
	if err != nil {
		return KecSessionView{}, kecError(err)
	}
	next := nextOpenKolom(kolom)
	if next == nil {
		return out, nil
	}
	if next.Status == model.KecKolomPending {
		prev := kolom[next.Number-2]
		starts := prev.DeadlineAt.Add(time.Duration(sess.PauseSeconds) * time.Second)
		out.Phase, out.FinishedKolom, out.NextKolom, out.NextStartsAt = KecPhasePause, prev.Number, next.Number, &starts
		return out, nil
	}
	gen, err := s.snapshot(ctx, tx, sess)
	if err != nil {
		return KecSessionView{}, err
	}
	column, _ := gen.Slot(next.Number)
	if column == nil {
		return KecSessionView{}, fmt.Errorf("kecermatan snapshot has no column for kolom %d", next.Number)
	}
	view := &KecKolomView{
		Number: next.Number, StartedAt: *next.StartedAt, DeadlineAt: *next.DeadlineAt,
		Reference: column.Reference, Soal: make([]KecSoalView, 0, len(column.Soal)),
	}
	for i, soal := range column.Soal {
		view.Soal = append(view.Soal, KecSoalView{Number: i + 1, Shown: soal.Shown})
	}
	stored, err := s.repo.Answers(ctx, tx, sess.TenantID, sess.ID, next.Number)
	if err != nil {
		return KecSessionView{}, kecError(err)
	}
	for _, a := range stored {
		view.Answers = append(view.Answers, KecAnswerView{Soal: a.Soal, Answer: a.Answer})
	}
	out.Phase, out.Kolom = KecPhaseKolom, view
	return out, nil
}

func (s *KecermatanRuntimeService) snapshot(ctx context.Context, q pgx.Tx, sess model.KecSession) (KecGeneration, error) {
	raw, err := s.repo.Columns(ctx, q, sess.TenantID, sess.ID)
	if err != nil {
		return KecGeneration{}, kecError(err)
	}
	var columns []KecColumn
	if err := json.Unmarshal(raw, &columns); err != nil {
		return KecGeneration{}, fmt.Errorf("decode kecermatan snapshot: %w", err)
	}
	return KecGeneration{KolomCount: sess.KolomCount, Columns: columns}, nil
}

// settleLocked applies every passed instant of the timeline, under the enrollment and session locks. It is the
// one routine that moves a kolom or an attempt forward by time, and it runs from every mutating call, from reads
// and from the worker. A running kolom past its deadline ends there, with its unanswered soal as kosong. The next
// kolom starts at the end of the pause (or earlier on "Lanjut"), at that instant rather than "now", so late
// settling never shifts a timer. The last kolom's end completes the attempt: result and credit use commit with it.
func (s *KecermatanRuntimeService) settleLocked(ctx context.Context, tx pgx.Tx, sess model.KecSession, now time.Time) (bool, error) {
	if sess.Status != model.KecSessionInProgress {
		return false, nil
	}
	kolom, err := s.repo.Kolom(ctx, tx, sess.TenantID, sess.ID)
	if err != nil {
		return false, kecError(err)
	}
	var gen *KecGeneration
	pause := time.Duration(sess.PauseSeconds) * time.Second
	duration := time.Duration(sess.KolomDurationSeconds) * time.Second
	changed := false
	var due time.Time
	for {
		k := nextOpenKolom(kolom)
		if k == nil {
			last := kolom[len(kolom)-1]
			return true, s.complete(ctx, tx, sess, kolom, *last.DeadlineAt)
		}
		if k.Status == model.KecKolomRunning {
			if now.Before(*k.DeadlineAt) {
				due = *k.DeadlineAt
				break
			}
			if gen == nil {
				loaded, err := s.snapshot(ctx, tx, sess)
				if err != nil {
					return false, err
				}
				gen = &loaded
			}
			benar, salah, kosong, err := s.count(ctx, tx, sess, *gen, k.Number)
			if err != nil {
				return false, err
			}
			if err := s.repo.EndKolom(ctx, tx, sess.TenantID, sess.ID, k.Number, benar, salah, kosong); err != nil {
				return false, kecError(err)
			}
			k.Status, k.Benar, k.Salah, k.Kosong = model.KecKolomEnded, &benar, &salah, &kosong
			changed = true
			continue
		}
		auto := kolom[k.Number-2].DeadlineAt.Add(pause)
		if now.Before(auto) {
			due = auto
			break
		}
		deadline := auto.Add(duration)
		if err := s.repo.StartKolom(ctx, tx, sess.TenantID, sess.ID, k.Number, auto, deadline); err != nil {
			return false, kecError(err)
		}
		k.Status, k.StartedAt, k.DeadlineAt = model.KecKolomRunning, &auto, &deadline
		changed = true
	}
	if sess.DueAt == nil || !sess.DueAt.Equal(due) {
		if err := s.repo.SetDue(ctx, tx, sess.TenantID, sess.ID, due); err != nil {
			return false, kecError(err)
		}
		changed = true
	}
	return changed, nil
}

// count scores one kolom against the snapshot's key for that kolom: a stored label that equals the key is
// benar, any other label is salah, and everything not answered (or skipped) is kosong.
func (s *KecermatanRuntimeService) count(ctx context.Context, tx pgx.Tx, sess model.KecSession, gen KecGeneration, kolom int) (benar, salah, kosong int, err error) {
	column, _ := gen.Slot(kolom)
	if column == nil {
		return 0, 0, 0, fmt.Errorf("kecermatan snapshot has no column for kolom %d", kolom)
	}
	stored, err := s.repo.Answers(ctx, tx, sess.TenantID, sess.ID, kolom)
	if err != nil {
		return 0, 0, 0, kecError(err)
	}
	for _, a := range stored {
		switch {
		case a.Answer == nil || a.Soal < 1 || a.Soal > len(column.Soal):
		case *a.Answer == column.Soal[a.Soal-1].Missing:
			benar++
		default:
			salah++
		}
	}
	return benar, salah, sess.SoalPerKolom - benar - salah, nil
}

// complete writes the result, closes the attempt and consumes one credit in the caller's transaction, so the
// three move together or not at all. The enrollment row is already locked by the caller.
func (s *KecermatanRuntimeService) complete(ctx context.Context, tx pgx.Tx, sess model.KecSession, kolom []model.KecKolom, at time.Time) error {
	counts := countsOf(kolom)
	scores, err := ScoreKecermatan(KecermatanScoringVersion, counts)
	if err != nil {
		return err
	}
	var benar, salah, kosong int
	for _, c := range counts {
		benar, salah, kosong = benar+c.Benar, salah+c.Salah, kosong+c.Kosong
	}
	if err := s.repo.InsertResult(ctx, tx, model.KecResult{
		SessionID: sess.ID, TenantID: sess.TenantID, UserID: sess.UserID, TestID: sess.TestID,
		AttemptNumber: sess.AttemptNumber, BenarTotal: benar, SalahTotal: salah, KosongTotal: kosong,
		Kecepatan: scores.Kecepatan, Ketahanan: scores.Ketahanan, Ketelitian: scores.Ketelitian,
		NilaiAkhir: scores.NilaiAkhir, ScoringVersion: scores.Version, CompletedAt: at,
	}, [3]float64{scores.KecepatanRaw, scores.KetahananRaw, scores.KetelitianRaw}); err != nil {
		return kecError(err)
	}
	if err := s.repo.CompleteSession(ctx, tx, sess.TenantID, sess.ID, at); err != nil {
		return kecError(err)
	}
	credit, err := s.repo.Credit(ctx, tx, sess.TenantID, sess.UserID, sess.TestID)
	if err != nil {
		return kecError(err)
	}
	return kecError(s.repo.ConsumeCredit(ctx, tx, sess.TenantID, credit.EnrollmentID))
}

// nextOpenKolom is the lowest kolom that has not ended, or nil when all have.
func nextOpenKolom(kolom []model.KecKolom) *model.KecKolom {
	for i := range kolom {
		if kolom[i].Status != model.KecKolomEnded {
			return &kolom[i]
		}
	}
	return nil
}

func countsOf(kolom []model.KecKolom) []KecermatanKolomCount {
	out := make([]KecermatanKolomCount, 0, len(kolom))
	for _, k := range kolom {
		out = append(out, KecermatanKolomCount{Benar: intOf(k.Benar), Salah: intOf(k.Salah), Kosong: intOf(k.Kosong)})
	}
	return out
}

func intOf(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func validKecLabel(label string, options int) bool {
	return len(label) == 1 && label[0] >= 'A' && int(label[0]-'A') < options
}

func sameLabel(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func kecError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrKecSessionNotFound
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

// runKeep runs fn in a transaction. An error wrapped in keep still commits what fn already settled.
func runKeep(ctx context.Context, db *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
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
