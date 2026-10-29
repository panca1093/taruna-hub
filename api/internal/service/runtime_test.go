package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type runtimeHarness struct {
	tryoutHarness
	rt    *RuntimeService
	repo  *repository.RuntimeRepository
	users *repository.UserRepository
}

func newRuntimeHarness(t *testing.T) runtimeHarness {
	t.Helper()
	base := newTryoutHarness(t)
	h := runtimeHarness{tryoutHarness: base, repo: repository.NewRuntimeRepository(base.pool), users: repository.NewUserRepository(base.pool)}
	h.rt = NewRuntimeService(base.pool, h.repo, WithRuntimeClock(base.clock.Now), WithRuntimeSeed(func() int64 { return 99 }))
	// Registered after the tryout cleanup, so sessions go before the tryouts and users they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{h.q.a.ID, h.q.b.ID}
		for _, sql := range []string{
			`DELETE FROM tryout_score_totals WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_results WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_violations WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_extensions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_answers WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_questions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_parts WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_sessions WHERE tenant_id = ANY($1)`,
			`DELETE FROM enrollments WHERE tenant_id = ANY($1)`,
			`DELETE FROM tenant_settings WHERE tenant_id = ANY($1)`,
		} {
			if _, err := h.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup runtime fixture: %v", err)
			}
		}
	})
	return h
}

func (h runtimeHarness) tenant() model.Tenant { return h.q.a }

func (h runtimeHarness) scopeA() HostScope { return h.scope(h.tenant()) }

func (h runtimeHarness) student(t *testing.T) (model.User, model.Principal) {
	t.Helper()
	user := walletUser(t, h.users, h.tenant().ID, model.RoleStudent, "Siswa Runtime")
	id := h.tenant().ID
	return user, model.Principal{UserID: user.ID, TenantID: &id, Name: user.Name, Role: model.RoleStudent}
}

func (h runtimeHarness) staff(role model.Role) model.Principal { return h.principal(h.tenant(), role) }

func (h runtimeHarness) enroll(t *testing.T, user model.User, tryoutID uuid.UUID) {
	t.Helper()
	_, err := h.pool.Exec(context.Background(),
		`INSERT INTO enrollments (tenant_id, user_id, tryout_id) VALUES ($1, $2, $3)`, h.tenant().ID, user.ID, tryoutID)
	if err != nil {
		t.Fatalf("enroll student: %v", err)
	}
}

func (h runtimeHarness) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// rtFixture is one published tryout of two subtests holding all four question types and an option-weighted one.
// With the default weights its maximum is 6 in subtest 1 (1 + 2 + 3) and 4 in subtest 2 (1 + 3).
type rtFixture struct {
	tryout   model.Tryout
	pg       model.Question
	multi    model.Question
	isian    model.Question
	tf       model.Question
	weighted model.Question
}

func (f rtFixture) sub1() uuid.UUID { return f.tryout.Subtests[0].ID }
func (f rtFixture) sub2() uuid.UUID { return f.tryout.Subtests[1].ID }

func optID(t *testing.T, q model.Question, label string) string {
	t.Helper()
	for _, o := range q.Options {
		if o.Label == label {
			return o.ID
		}
	}
	t.Fatalf("question has no option labelled %q", label)
	return ""
}

func (h runtimeHarness) fixture(t *testing.T, mutate func(*TryoutInput)) rtFixture {
	t.Helper()
	tenant := h.tenant()
	base := h.q.pgInput(tenant)
	base.Options = []QuestionOptionInput{{ID: "a", Label: "3"}, {ID: "b", Label: "4"}}
	base.Correct = []string{"b"}

	multi := base
	multi.Type, multi.Difficulty = model.QuestionPGMulti, model.DifficultyMedium
	multi.Options = []QuestionOptionInput{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}, {ID: "c", Label: "C"}}
	multi.Correct = []string{"a", "c"}

	isian := base
	isian.Type, isian.Difficulty = model.QuestionIsian, model.DifficultyHard
	isian.Options, isian.Correct = nil, []string{"Jakarta"}

	tf := base
	tf.Type = model.QuestionTrueFalse
	tf.Options, tf.Correct = nil, []string{"salah"}

	weighted := base
	weighted.Difficulty, weighted.OptionWeighted = model.DifficultyMedium, true
	weighted.Options = []QuestionOptionInput{
		{ID: "a", Label: "A", Points: pts(0.5)}, {ID: "b", Label: "B", Points: pts(3)}, {ID: "c", Label: "C", Points: pts(1)},
	}
	weighted.Correct = nil

	f := rtFixture{
		pg: h.q.create(t, tenant, base), multi: h.q.create(t, tenant, multi), isian: h.q.create(t, tenant, isian),
		tf: h.q.create(t, tenant, tf), weighted: h.q.create(t, tenant, weighted),
	}
	input := h.tryoutInput(tenant,
		subtestInput("Subtes 1", 1800, f.pg, f.multi, f.isian),
		subtestInput("Subtes 2", 900, f.tf, f.weighted))
	if mutate != nil {
		mutate(&input)
	}
	created := h.create(t, tenant, input)
	published, err := h.publish(tenant, created.ID)
	if err != nil {
		t.Fatalf("publish fixture tryout: %v", err)
	}
	f.tryout = published
	return f
}

func (h runtimeHarness) enrolledStudent(t *testing.T, f rtFixture) (model.User, model.Principal) {
	t.Helper()
	user, principal := h.student(t)
	h.enroll(t, user, f.tryout.ID)
	return user, principal
}

func (h runtimeHarness) begin(t *testing.T, p model.Principal, f rtFixture) AttemptView {
	t.Helper()
	view, err := h.rt.Start(context.Background(), h.scopeA(), p, f.tryout.ID)
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}
	return view
}

func (h runtimeHarness) startPart(t *testing.T, p model.Principal, session, subtest uuid.UUID) AttemptView {
	t.Helper()
	view, err := h.rt.StartSubtest(context.Background(), h.scopeA(), p, session, subtest)
	if err != nil {
		t.Fatalf("start subtest: %v", err)
	}
	return view
}

func (h runtimeHarness) submitPart(t *testing.T, p model.Principal, session, subtest uuid.UUID) AttemptView {
	t.Helper()
	view, err := h.rt.SubmitSubtest(context.Background(), h.scopeA(), p, session, subtest)
	if err != nil {
		t.Fatalf("submit subtest: %v", err)
	}
	return view
}

func (h runtimeHarness) answer(p model.Principal, session, question uuid.UUID, seq int64, answer ...string) (AnswerAck, error) {
	return h.rt.SaveAnswer(context.Background(), h.scopeA(), p, session, question, AnswerInput{Seq: seq, Answer: answer})
}

func (h runtimeHarness) mustAnswer(t *testing.T, p model.Principal, session, question uuid.UUID, seq int64, answer ...string) AnswerAck {
	t.Helper()
	ack, err := h.answer(p, session, question, seq, answer...)
	if err != nil {
		t.Fatalf("save answer: %v", err)
	}
	return ack
}

func wantErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("err = %v, want %v", got, want)
	}
}

func TestRuntimeAttemptScoresFromTheFrozenSnapshot(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)

	view := h.begin(t, p, f)
	if view.Attempt.AttemptNumber != 1 || view.Attempt.Status != model.SessionInProgress || len(view.Parts) != 2 {
		t.Fatalf("started session = %+v, want attempt 1 in progress with 2 parts", view.Attempt)
	}
	if view.Parts[0].Status != model.PartPending || view.Parts[0].Deadline != nil {
		t.Errorf("part 1 = %+v, want pending without a deadline until the student starts it", view.Parts[0])
	}
	if !h.get(t, h.tenant(), f.tryout.ID).Locked {
		t.Error("the first attempt must lock the tryout")
	}
	if view.Attempt.Weights != model.DefaultWeights {
		t.Errorf("frozen weights = %+v, want the 1/2/3 defaults", view.Attempt.Weights)
	}

	sid := view.Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))
	h.mustAnswer(t, p, sid, f.multi.ID, 1, optID(t, f.multi, "A"))
	h.mustAnswer(t, p, sid, f.isian.ID, 1, "  jakarta ")
	mid := h.submitPart(t, p, sid, f.sub1())
	if mid.Attempt.Status != model.SessionInProgress || mid.Result != nil || mid.Parts[0].Score != nil {
		t.Errorf("after subtest 1 the session = %+v, want no score shown before completion", mid)
	}

	h.startPart(t, p, sid, f.sub2())
	h.mustAnswer(t, p, sid, f.tf.ID, 1, optID(t, f.tf, "Benar"))
	h.mustAnswer(t, p, sid, f.weighted.ID, 1, optID(t, f.weighted, "A"))
	done := h.submitPart(t, p, sid, f.sub2())

	// Subtest 1: pg 1 + isian 3 of 6. Subtest 2: wrong true/false 0, weighted option 0.5, of 1 + 3.
	if done.Attempt.Status != model.SessionCompleted || done.Result == nil {
		t.Fatalf("completed session = %+v, want a result with the completion response", done)
	}
	res := done.Result
	if res.EarnedPoints != 4.5 || res.MaxPoints != 10 || res.Score != 45 || res.ScoringVersion != model.ScoringVersion {
		t.Errorf("result = %+v, want 4.5 of 10, score 45, scoring version %d", res, model.ScoringVersion)
	}
	if got := []float64{res.Parts[0].Score, res.Parts[1].Score}; got[0] != 66.67 || got[1] != 12.5 {
		t.Errorf("subtest scores = %v, want 66.67 and 12.5", got)
	}
	if res.Parts[0].CorrectCount != 2 || res.Parts[1].CorrectCount != 0 {
		t.Errorf("correct counts = %d and %d, want 2 and 0", res.Parts[0].CorrectCount, res.Parts[1].CorrectCount)
	}
	sum, count, err := h.repo.ScoreTotal(context.Background(), h.pool, h.tenant().ID, p.UserID, f.tryout.ID)
	if err != nil || sum != 45 || count != 1 {
		t.Errorf("score total = %v over %d (err %v), want 45 over 1", sum, count, err)
	}

	// Changing the setting afterwards moves neither the stored result nor the attempt's frozen weights.
	admin := h.staff(model.RoleAdmin)
	if _, err := h.rt.UpdateSettings(context.Background(), h.scopeA(), admin, SettingsInput{Weights: model.Weights{Easy: 2, Medium: 4, Hard: 6}, MinTopup: "10000"}); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	again, err := h.rt.Result(context.Background(), h.scopeA(), p, sid)
	if err != nil || again.Score != 45 || again.EarnedPoints != 4.5 {
		t.Errorf("result after a settings change = %+v (err %v), want it unchanged", again, err)
	}
	frozen, err := h.repo.FindAttempt(context.Background(), h.pool, h.tenant().ID, p.UserID, sid, false)
	if err != nil || frozen.Weights != model.DefaultWeights {
		t.Errorf("first attempt weights = %+v (err %v), want 1/2/3", frozen.Weights, err)
	}

	second := h.begin(t, p, f)
	if second.Attempt.AttemptNumber != 2 || second.Attempt.Weights != (model.Weights{Easy: 2, Medium: 4, Hard: 6}) {
		t.Errorf("second attempt = number %d weights %+v, want attempt 2 freezing 2/4/6", second.Attempt.AttemptNumber, second.Attempt.Weights)
	}
}

func TestRuntimeConcurrentStartsShareOneSession(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	user, p := h.enrolledStudent(t, f)

	const starters = 8
	ids := make([]uuid.UUID, starters)
	errs := make([]error, starters)
	var wg sync.WaitGroup
	for i := 0; i < starters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			view, err := h.rt.Start(context.Background(), h.scopeA(), p, f.tryout.ID)
			ids[i], errs[i] = view.Attempt.ID, err
		}()
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil {
			t.Fatalf("start %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("start %d returned session %s, want %s", i, ids[i], ids[0])
		}
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_sessions WHERE user_id = $1`, user.ID); n != 1 {
		t.Errorf("%d sessions after %d concurrent starts, want 1", n, starters)
	}
	if n := h.count(t, `SELECT max(attempt_number) FROM tryout_sessions WHERE user_id = $1`, user.ID); n != 1 {
		t.Errorf("attempt number = %d, want 1 consumed", n)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_questions WHERE session_id = $1`, ids[0]); n != 5 {
		t.Errorf("snapshot holds %d questions, want 5 written once", n)
	}
}

func TestRuntimeResumeKeepsSnapshotAndAnswers(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, func(in *TryoutInput) { in.RandomizeQuestions, in.RandomizeOptions = true, true })
	_, p := h.enrolledStudent(t, f)

	seeds := atomic.Int64{}
	rt := NewRuntimeService(h.pool, h.repo, WithRuntimeClock(h.clock.Now), WithRuntimeSeed(func() int64 { return seeds.Add(100) }))
	scope := h.scopeA()

	first, err := rt.Start(context.Background(), scope, p, f.tryout.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	sid := first.Attempt.ID
	if _, err := rt.StartSubtest(context.Background(), scope, p, sid, f.sub1()); err != nil {
		t.Fatalf("start subtest: %v", err)
	}
	read := func() SubtestView {
		view, err := rt.Subtest(context.Background(), scope, p, sid, f.sub1())
		if err != nil {
			t.Fatalf("read subtest: %v", err)
		}
		return view
	}
	before := read()
	if _, err := rt.SaveAnswer(context.Background(), scope, p, sid, f.pg.ID, AnswerInput{Seq: 1, Answer: []string{optID(t, f.pg, "4")}}); err != nil {
		t.Fatalf("answer: %v", err)
	}

	resumed, err := rt.Start(context.Background(), scope, p, f.tryout.ID)
	if err != nil || resumed.Attempt.ID != sid {
		t.Fatalf("resume = %v (err %v), want the same session %s", resumed.Attempt.ID, err, sid)
	}
	after := read()
	if len(before.Questions) != 3 || len(after.Questions) != 3 {
		t.Fatalf("subtest holds %d then %d questions, want 3", len(before.Questions), len(after.Questions))
	}
	for i := range before.Questions {
		b, a := before.Questions[i], after.Questions[i]
		if b.QuestionID != a.QuestionID || len(b.Options) != len(a.Options) {
			t.Fatalf("resume reordered question %d", i)
		}
		for j := range b.Options {
			if b.Options[j].ID != a.Options[j].ID {
				t.Errorf("resume reordered the options of question %d", i)
			}
		}
	}
	saved := after.Answers[f.pg.ID]
	if saved.Revision != 1 || saved.Seq != 1 || len(saved.Answer) != 1 || saved.Answer[0] != optID(t, f.pg, "4") {
		t.Errorf("saved answer after resume = %+v, want revision 1 seq 1 with the chosen option", saved)
	}

	if _, err := rt.StartSubtest(context.Background(), scope, p, sid, f.sub2()); !errors.Is(err, ErrPartOrder) {
		t.Fatalf("starting subtest 2 first: %v, want ErrPartOrder", err)
	}
	if _, err := rt.SubmitSubtest(context.Background(), scope, p, sid, f.sub1()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := rt.StartSubtest(context.Background(), scope, p, sid, f.sub2()); err != nil {
		t.Fatalf("start subtest 2: %v", err)
	}
	if _, err := rt.SubmitSubtest(context.Background(), scope, p, sid, f.sub2()); err != nil {
		t.Fatalf("submit subtest 2: %v", err)
	}

	next, err := rt.Start(context.Background(), scope, p, f.tryout.ID)
	if err != nil || next.Attempt.ID == sid || next.Attempt.AttemptNumber != 2 {
		t.Fatalf("second attempt = %+v (err %v), want a new session numbered 2", next.Attempt, err)
	}
	if next.Attempt.Seed == first.Attempt.Seed {
		t.Errorf("both attempts used seed %d, want an independent seed per attempt", next.Attempt.Seed)
	}
	if first.Attempt.Seed == 0 || h.count(t, `SELECT count(*) FROM tryout_session_questions WHERE session_id = $1`, next.Attempt.ID) != 5 {
		t.Error("each attempt snapshots its own five questions")
	}
}

func TestRuntimeOptionPointsSurviveAShuffle(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, func(in *TryoutInput) { in.RandomizeOptions = true })
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	h.submitPart(t, p, sid, f.sub1())
	h.startPart(t, p, sid, f.sub2())

	view, err := h.rt.Subtest(context.Background(), h.scopeA(), p, sid, f.sub2())
	if err != nil {
		t.Fatalf("read subtest: %v", err)
	}
	for _, q := range view.Questions {
		if q.QuestionID != f.weighted.ID {
			continue
		}
		if len(q.Options) != 3 {
			t.Fatalf("weighted question shows %d options, want 3", len(q.Options))
		}
	}
	// Pick option B by its id wherever the shuffle put it: it is worth 3 of the subtest's 4.
	h.mustAnswer(t, p, sid, f.weighted.ID, 1, optID(t, f.weighted, "B"))
	done := h.submitPart(t, p, sid, f.sub2())
	if got := done.Result.Parts[1]; got.EarnedPoints != 3 || got.MaxPoints != 4 {
		t.Errorf("subtest 2 = %v of %v, want 3 of 4 whatever the option order", got.EarnedPoints, got.MaxPoints)
	}
}

func TestRuntimeAnswerSequencingAndValidation(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	a, b := optID(t, f.pg, "3"), optID(t, f.pg, "4")

	first := h.mustAnswer(t, p, sid, f.pg.ID, 1, a)
	if first.Revision != 1 {
		t.Fatalf("first write revision = %d, want 1", first.Revision)
	}
	retry := h.mustAnswer(t, p, sid, f.pg.ID, 1, a)
	if retry.Revision != 1 {
		t.Errorf("retry of write 1 revision = %d, want 1 (idempotent)", retry.Revision)
	}
	if _, err := h.answer(p, sid, f.pg.ID, 1, b); !errors.Is(err, ErrStaleAnswer) {
		t.Errorf("same sequence with another answer: %v, want ErrStaleAnswer", err)
	}

	newer := h.mustAnswer(t, p, sid, f.pg.ID, 3, b)
	if newer.Revision != 2 {
		t.Errorf("write 3 revision = %d, want 2", newer.Revision)
	}
	// Write 2 was delayed on the network and arrives after write 3: it must not overwrite it.
	if _, err := h.answer(p, sid, f.pg.ID, 2, a); !errors.Is(err, ErrStaleAnswer) {
		t.Fatalf("reordered write: %v, want ErrStaleAnswer", err)
	}
	view, err := h.rt.Subtest(context.Background(), h.scopeA(), p, sid, f.sub1())
	if err != nil {
		t.Fatalf("read subtest: %v", err)
	}
	if saved := view.Answers[f.pg.ID]; len(saved.Answer) != 1 || saved.Answer[0] != b || saved.Seq != 3 || saved.Revision != 2 {
		t.Errorf("stored answer = %+v, want the newest write (option 4, seq 3, revision 2)", saved)
	}

	same := h.mustAnswer(t, p, sid, f.pg.ID, 4, b)
	if same.Revision != 2 {
		t.Errorf("a newer write of the same answer moved revision to %d, want 2", same.Revision)
	}
	cleared := h.mustAnswer(t, p, sid, f.pg.ID, 5)
	if cleared.Revision != 3 {
		t.Errorf("clearing revision = %d, want 3", cleared.Revision)
	}

	for name, in := range map[string]struct {
		question uuid.UUID
		seq      int64
		answer   []string
	}{
		"unknown option":        {f.pg.ID, 6, []string{"nope"}},
		"two picks on a pg":     {f.pg.ID, 7, []string{a, b}},
		"duplicate multi pick":  {f.multi.ID, 1, []string{optID(t, f.multi, "A"), optID(t, f.multi, "A")}},
		"two isian answers":     {f.isian.ID, 1, []string{"x", "y"}},
		"oversized isian":       {f.isian.ID, 2, []string{string(make([]rune, 501))}},
		"sequence below one":    {f.pg.ID, 0, []string{a}},
		"question of the other": {f.tf.ID, 1, []string{"benar"}},
	} {
		_, err := h.answer(p, sid, in.question, in.seq, in.answer...)
		var input *InputError
		if name == "question of the other" {
			if !errors.Is(err, ErrPartNotRunning) {
				t.Errorf("%s: %v, want ErrPartNotRunning", name, err)
			}
			continue
		}
		if !errors.As(err, &input) {
			t.Errorf("%s: %v, want an InputError", name, err)
		}
	}
	if _, err := h.answer(p, sid, uuid.New(), 1, "x"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("unknown question: %v, want ErrSessionNotFound", err)
	}
}

func TestRuntimeGraceChangedMidExamTakesEffect(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	admin, ctx := h.staff(model.RoleAdmin), context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	started := h.clock.Now()
	part := h.startPart(t, p, sid, f.sub1()).Parts[0]
	if want := started.Add(30 * time.Minute); !part.Deadline.Equal(want) {
		t.Fatalf("deadline = %v, want start + 30 minutes %v", part.Deadline, want)
	}

	edit := h.tryoutInput(h.tenant())
	edit.BaseRevision = f.tryout.Revision
	if _, err := h.update(h.tenant(), f.tryout.ID, edit); !errors.Is(err, ErrTryoutLocked) {
		t.Fatalf("editing a locked tryout: %v, want ErrTryoutLocked", err)
	}
	if _, err := h.svc.SetGrace(ctx, h.scopeA(), h.staff(model.RoleOperator), f.tryout.ID, 60); !errors.Is(err, ErrTryoutForbidden) {
		t.Errorf("operator setting the grace: %v, want ErrTryoutForbidden", err)
	}
	var input *InputError
	if _, err := h.svc.SetGrace(ctx, h.scopeA(), admin, f.tryout.ID, 3601); !errors.As(err, &input) {
		t.Errorf("grace of 3601: %v, want an InputError", err)
	}
	visible := func(want time.Time, why string) {
		t.Helper()
		view, err := h.rt.Session(ctx, h.scopeA(), p, sid)
		if err != nil || !view.Parts[0].Deadline.Equal(want) {
			t.Fatalf("countdown deadline %s = %v (err %v), want %v", why, view.Parts[0].Deadline, err, want)
		}
	}
	if got, err := h.svc.SetGrace(ctx, h.scopeA(), admin, f.tryout.ID, 120); err != nil || got.GraceSeconds != 120 {
		t.Fatalf("setting the grace on a locked tryout = %+v (err %v), want 120", got, err)
	}
	visible(started.Add(32*time.Minute), "with a 120 second grace")
	if got, err := h.svc.SetGrace(ctx, h.scopeA(), admin, f.tryout.ID, 60); err != nil || got.GraceSeconds != 60 || !got.Locked {
		t.Fatalf("setting the grace on a locked tryout = %+v (err %v), want 60 and still locked", got, err)
	}
	visible(started.Add(31*time.Minute), "after the grace dropped to 60 mid-exam")

	h.clock.Advance(30*time.Minute + 30*time.Second)
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))

	h.clock.Advance(31 * time.Second)
	if _, err := h.answer(p, sid, f.pg.ID, 2, optID(t, f.pg, "3")); !errors.Is(err, ErrDeadlinePassed) {
		t.Fatalf("answer past deadline + grace: %v, want ErrDeadlinePassed", err)
	}
	view, err := h.rt.Session(ctx, h.scopeA(), p, sid)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	settled := view.Parts[0]
	if settled.Status != model.PartSubmitted || settled.SubmitReason != model.SubmitExpired {
		t.Fatalf("part = %+v, want submitted by expiry", settled)
	}
	if want := started.Add(31 * time.Minute); !settled.SubmittedAt.Equal(want) {
		t.Errorf("submitted at %v, want the deadline plus grace %v", settled.SubmittedAt, want)
	}
	if got := deref(settled.EarnedPoints); got != 1 {
		t.Errorf("earned %v, want 1: the answer inside the grace counts, the late one does not", got)
	}
	if _, err := h.rt.Subtest(ctx, h.scopeA(), p, sid, f.sub1()); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("reading an expired subtest: %v, want ErrPartNotRunning", err)
	}
}

func TestRuntimeStaffExtensionExtendsOnlyThatSession(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, pa := h.enrolledStudent(t, f)
	_, pb := h.enrolledStudent(t, f)
	ctx, operator := context.Background(), h.staff(model.RoleOperator)

	sa, sb := h.begin(t, pa, f).Attempt.ID, h.begin(t, pb, f).Attempt.ID
	partA := h.startPart(t, pa, sa, f.sub1()).Parts[0]
	h.startPart(t, pb, sb, f.sub1())

	var input *InputError
	if _, err := h.rt.Extend(ctx, h.scopeA(), operator, sa, ExtensionInput{Seconds: 0, Reason: "x"}); !errors.As(err, &input) {
		t.Errorf("zero seconds: %v, want an InputError", err)
	}
	if _, err := h.rt.Extend(ctx, h.scopeA(), operator, sa, ExtensionInput{Seconds: 60, Reason: "  "}); !errors.As(err, &input) {
		t.Errorf("blank reason: %v, want an InputError", err)
	}
	if _, err := h.rt.Extend(ctx, h.scopeA(), pa, sa, ExtensionInput{Seconds: 60, Reason: "minta"}); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("a student granting time: %v, want ErrRuntimeForbidden", err)
	}
	ext, err := h.rt.Extend(ctx, h.scopeA(), operator, sa, ExtensionInput{Seconds: 120, Reason: "Listrik padam di lokasi"})
	if err != nil || ext.TotalExtra != 120 {
		t.Fatalf("extend = %+v (err %v), want 120 seconds in total", ext, err)
	}

	view, err := h.rt.Session(ctx, h.scopeA(), pa, sa)
	if err != nil || !view.Parts[0].Deadline.Equal(partA.Deadline.Add(2*time.Minute)) {
		t.Fatalf("visible deadline = %v (err %v), want the extension added", view.Parts[0].Deadline, err)
	}

	h.clock.Advance(30*time.Minute + 60*time.Second)
	h.mustAnswer(t, pa, sa, f.pg.ID, 1, optID(t, f.pg, "4"))
	if _, err := h.answer(pb, sb, f.pg.ID, 1, optID(t, f.pg, "4")); !errors.Is(err, ErrDeadlinePassed) {
		t.Errorf("student B at the same instant: %v, want ErrDeadlinePassed (the extension is A's alone)", err)
	}

	var reason string
	var grantedBy, subtestID uuid.UUID
	var seconds int
	err = h.pool.QueryRow(ctx, `SELECT seconds, reason, granted_by, subtest_id FROM tryout_session_extensions WHERE session_id = $1`, sa).
		Scan(&seconds, &reason, &grantedBy, &subtestID)
	if err != nil || seconds != 120 || reason != "Listrik padam di lokasi" || grantedBy != operator.UserID || subtestID != f.sub1() {
		t.Errorf("audit row = %d %q by %s on %s (err %v), want the grant with its reason, actor and subtest", seconds, reason, grantedBy, subtestID, err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_extensions WHERE session_id = $1`, sb); n != 0 {
		t.Errorf("student B has %d extension rows, want 0", n)
	}
}

func TestRuntimeSubmittedPartCannotBeReopened(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID

	if _, err := h.rt.StartSubtest(ctx, h.scopeA(), p, sid, f.sub2()); !errors.Is(err, ErrPartOrder) {
		t.Fatalf("starting subtest 2 before 1: %v, want ErrPartOrder", err)
	}
	if _, err := h.rt.Subtest(ctx, h.scopeA(), p, sid, f.sub1()); !errors.Is(err, ErrPartNotRunning) {
		t.Fatalf("reading an unstarted subtest: %v, want ErrPartNotRunning", err)
	}
	if _, err := h.rt.SubmitSubtest(ctx, h.scopeA(), p, sid, f.sub1()); !errors.Is(err, ErrPartNotRunning) {
		t.Fatalf("submitting an unstarted subtest: %v, want ErrPartNotRunning", err)
	}

	started := h.startPart(t, p, sid, f.sub1())
	h.clock.Advance(5 * time.Minute)
	if again := h.startPart(t, p, sid, f.sub1()); !again.Parts[0].Deadline.Equal(*started.Parts[0].Deadline) {
		t.Errorf("a repeated start moved the deadline to %v from %v", again.Parts[0].Deadline, started.Parts[0].Deadline)
	}
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))
	first := h.submitPart(t, p, sid, f.sub1())
	second := h.submitPart(t, p, sid, f.sub1())
	if first.Parts[0].Status != model.PartSubmitted || !second.Parts[0].SubmittedAt.Equal(*first.Parts[0].SubmittedAt) {
		t.Errorf("a retried submit changed the settled part: %+v then %+v", first.Parts[0], second.Parts[0])
	}

	if _, err := h.answer(p, sid, f.pg.ID, 2, optID(t, f.pg, "3")); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("writing to a submitted part: %v, want ErrPartNotRunning", err)
	}
	if _, err := h.rt.StartSubtest(ctx, h.scopeA(), p, sid, f.sub1()); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("reopening a submitted part: %v, want ErrPartNotRunning", err)
	}
	if _, err := h.rt.Subtest(ctx, h.scopeA(), p, sid, f.sub1()); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("reading a submitted part: %v, want ErrPartNotRunning", err)
	}
	if got := deref(first.Parts[0].EarnedPoints); got != 1 {
		t.Errorf("earned %v, want 1 and unchanged by the rejected write", got)
	}
}

func TestRuntimeDeadlineIsCutByTheTryoutWindow(t *testing.T) {
	h := newRuntimeHarness(t)
	until := h.clock.Now().Add(10 * time.Minute)
	f := h.fixture(t, func(in *TryoutInput) { in.ActiveUntil = &until })
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID

	part := h.startPart(t, p, sid, f.sub1()).Parts[0]
	if !part.Deadline.Equal(until) || part.DurationSeconds != 1800 {
		t.Fatalf("deadline = %v, want active_until %v rather than start + 30 minutes", part.Deadline, until)
	}
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))

	h.clock.Advance(10*time.Minute + time.Second)
	if _, err := h.answer(p, sid, f.pg.ID, 2, optID(t, f.pg, "3")); !errors.Is(err, ErrDeadlinePassed) {
		t.Fatalf("answer after active_until: %v, want ErrDeadlinePassed", err)
	}
	view, err := h.rt.Session(ctx, h.scopeA(), p, sid)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if view.Attempt.Status != model.SessionCompleted || view.Result == nil {
		t.Fatalf("session = %+v, want it finalized when the tryout window closed", view.Attempt)
	}
	if view.Parts[1].SubmitReason != model.SubmitUnstarted || deref(view.Parts[1].EarnedPoints) != 0 || deref(view.Parts[1].MaxPoints) != 4 {
		t.Errorf("subtest 2 = %+v, want unstarted and scoring 0 of its full 4", view.Parts[1])
	}
	if view.Result.EarnedPoints != 1 || view.Result.MaxPoints != 10 || view.Result.Score != 10 {
		t.Errorf("result = %+v, want 1 of 10, score 10", view.Result)
	}
	if _, err := h.rt.StartSubtest(ctx, h.scopeA(), p, sid, f.sub2()); !errors.Is(err, ErrSessionClosed) {
		t.Errorf("starting a part after the window: %v, want ErrSessionClosed", err)
	}
}

func TestRuntimeWorkerFinalizesAnAbandonedSessionOnce(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	user, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))
	h.mustAnswer(t, p, sid, f.isian.ID, 1, "jakarta")

	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 0 {
		t.Fatalf("finalize before any deadline = %d (err %v), want nothing due", n, err)
	}

	drain := func() int {
		var settled atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n, err := h.rt.FinalizeDue(ctx)
				if err != nil {
					t.Errorf("finalize: %v", err)
				}
				settled.Add(int64(n))
			}()
		}
		wg.Wait()
		return int(settled.Load())
	}

	// The tab is closed and subtest 1's time is up; the window is still open, so the session waits for the student.
	h.clock.Advance(31 * time.Minute)
	if n := drain(); n != 1 {
		t.Fatalf("settled %d sessions from the expired part, want exactly 1 across 4 workers", n)
	}
	if n := drain(); n != 0 {
		t.Errorf("a second pass settled %d, want 0", n)
	}
	view, err := h.rt.Session(ctx, h.scopeA(), p, sid)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if view.Attempt.Status != model.SessionInProgress || view.Parts[0].SubmitReason != model.SubmitExpired || view.Parts[1].Status != model.PartPending {
		t.Fatalf("after the part deadline: %+v, want subtest 1 expired and the session waiting at subtest 2", view)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_results WHERE user_id = $1`, user.ID); n != 0 {
		t.Errorf("%d results before the window closed, want 0", n)
	}

	h.clock.Advance(49 * time.Hour)
	if n := drain(); n != 1 {
		t.Fatalf("settled %d sessions once the window closed, want exactly 1", n)
	}
	if n := drain(); n != 0 {
		t.Errorf("a repeated finalize settled %d, want 0", n)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_results WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("%d results for the session, want exactly 1", n)
	}
	sum, count, err := h.repo.ScoreTotal(ctx, h.pool, h.tenant().ID, user.ID, f.tryout.ID)
	if err != nil || count != 1 || sum != 40 {
		t.Errorf("score total = %v over %d (err %v), want 40 over 1", sum, count, err)
	}
	done, err := h.rt.Result(ctx, h.scopeA(), p, sid)
	if err != nil || done.Score != 40 || done.Parts[1].SubmitReason != model.SubmitUnstarted {
		t.Errorf("result = %+v (err %v), want 4 of 10 with subtest 2 unstarted", done, err)
	}
	if _, err := h.rt.Extend(ctx, h.scopeA(), h.staff(model.RoleAdmin), sid, ExtensionInput{Seconds: 60, Reason: "terlambat"}); !errors.Is(err, ErrSessionClosed) {
		t.Errorf("extending a completed session: %v, want ErrSessionClosed", err)
	}
}

func TestRuntimeSubmitAndFinalizeRaceYieldsOneResult(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	user, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	h.submitPart(t, p, sid, f.sub1())
	h.startPart(t, p, sid, f.sub2())

	h.clock.Advance(16 * time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := h.rt.FinalizeDue(ctx); err != nil {
				t.Errorf("finalize: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := h.rt.SubmitSubtest(ctx, h.scopeA(), p, sid, f.sub2()); err != nil {
				t.Errorf("submit: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := h.count(t, `SELECT count(*) FROM tryout_results WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("%d results, want 1", n)
	}
	if _, count, _ := h.repo.ScoreTotal(ctx, h.pool, h.tenant().ID, user.ID, f.tryout.ID); count != 1 {
		t.Errorf("score total counted %d results, want 1", count)
	}
}

func TestRuntimeAccessAndOwnership(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	ctx, scope := context.Background(), h.scopeA()
	_, owner := h.enrolledStudent(t, f)
	_, stranger := h.student(t)
	sid := h.begin(t, owner, f).Attempt.ID
	h.startPart(t, owner, sid, f.sub1())

	if _, err := h.rt.Start(ctx, scope, stranger, f.tryout.ID); !errors.Is(err, ErrTryoutNotFound) {
		t.Errorf("start without an enrollment: %v, want ErrTryoutNotFound", err)
	}
	if _, err := h.rt.Overview(ctx, scope, stranger, f.tryout.ID); !errors.Is(err, ErrTryoutNotFound) {
		t.Errorf("overview without an enrollment: %v, want ErrTryoutNotFound", err)
	}

	checks := map[string]func() error{
		"session": func() error { _, err := h.rt.Session(ctx, scope, stranger, sid); return err },
		"result":  func() error { _, err := h.rt.Result(ctx, scope, stranger, sid); return err },
		"subtest": func() error { _, err := h.rt.Subtest(ctx, scope, stranger, sid, f.sub1()); return err },
		"start":   func() error { _, err := h.rt.StartSubtest(ctx, scope, stranger, sid, f.sub1()); return err },
		"submit":  func() error { _, err := h.rt.SubmitSubtest(ctx, scope, stranger, sid, f.sub1()); return err },
		"answer":  func() error { _, err := h.answer(stranger, sid, f.pg.ID, 1, optID(t, f.pg, "4")); return err },
	}
	for name, check := range checks {
		if err := check(); !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("student B on student A's %s: %v, want ErrSessionNotFound", name, err)
		}
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_answers WHERE session_id = $1`, sid); n != 0 {
		t.Errorf("the stranger's write stored %d answers, want 0", n)
	}

	for _, role := range []model.Role{model.RoleAdmin, model.RoleOperator} {
		staff := h.staff(role)
		if _, err := h.rt.Session(ctx, scope, staff, sid); !errors.Is(err, ErrRuntimeForbidden) {
			t.Errorf("%s reading a student session: %v, want ErrRuntimeForbidden", role, err)
		}
		if _, err := h.rt.Start(ctx, scope, staff, f.tryout.ID); !errors.Is(err, ErrRuntimeForbidden) {
			t.Errorf("%s starting an attempt: %v, want ErrRuntimeForbidden", role, err)
		}
	}
	if _, err := h.rt.Settings(ctx, scope, h.staff(model.RoleOperator)); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("operator reading settings: %v, want ErrRuntimeForbidden", err)
	}
	var input *InputError
	if _, err := h.rt.UpdateSettings(ctx, scope, h.staff(model.RoleAdmin), SettingsInput{Weights: model.Weights{Easy: 0, Medium: 2, Hard: 3}, MinTopup: "10000"}); !errors.As(err, &input) {
		t.Errorf("a zero weight: %v, want an InputError", err)
	}
	if w, err := h.rt.Settings(ctx, scope, h.staff(model.RoleAdmin)); err != nil || w.Weights != model.DefaultWeights || w.MinTopup != model.DefaultMinTopup {
		t.Errorf("settings without a saved row = %+v (err %v), want the 1/2/3 defaults", w, err)
	}
}

func TestRuntimeAdmissionWindow(t *testing.T) {
	h := newRuntimeHarness(t)
	from := h.clock.Now().Add(time.Hour)
	f := h.fixture(t, func(in *TryoutInput) { in.ActiveFrom = &from })
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()

	if _, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID); !errors.Is(err, ErrTryoutNotOpen) {
		t.Fatalf("start before active_from: %v, want ErrTryoutNotOpen", err)
	}
	if h.get(t, h.tenant(), f.tryout.ID).Locked {
		t.Error("a refused start must not lock the tryout")
	}
	h.clock.Advance(90 * time.Minute)
	if _, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID); err != nil {
		t.Fatalf("start inside the window: %v", err)
	}

	_, late := h.enrolledStudent(t, f)
	h.clock.Advance(49 * time.Hour)
	if _, err := h.rt.Start(ctx, h.scopeA(), late, f.tryout.ID); !errors.Is(err, ErrTryoutClosed) {
		t.Errorf("start after active_until: %v, want ErrTryoutClosed", err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_sessions WHERE user_id = $1`, late.UserID); n != 0 {
		t.Errorf("a refused start left %d sessions", n)
	}

	view, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID)
	if err != nil || view.Attempt.Status != model.SessionCompleted || view.Result == nil || view.Result.Score != 0 {
		t.Errorf("resuming after the window = %+v (err %v), want the settled session with a zero result", view.Attempt, err)
	}
}

func TestRuntimeExtensionAppliesOnlyToTheRunningSubtest(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	ctx, operator := context.Background(), h.staff(model.RoleOperator)
	sid := h.begin(t, p, f).Attempt.ID

	if _, err := h.rt.Extend(ctx, h.scopeA(), operator, sid, ExtensionInput{Seconds: 60, Reason: "belum mulai"}); !errors.Is(err, ErrPartNotRunning) {
		t.Fatalf("extending before any subtest runs: %v, want ErrPartNotRunning", err)
	}
	first := h.startPart(t, p, sid, f.sub1()).Parts[0]
	ext, err := h.rt.Extend(ctx, h.scopeA(), operator, sid, ExtensionInput{Seconds: 120, Reason: "Listrik padam"})
	if err != nil || ext.SubtestID != f.sub1() || ext.TotalExtra != 120 {
		t.Fatalf("extend = %+v (err %v), want 120 seconds on subtest 1", ext, err)
	}
	h.submitPart(t, p, sid, f.sub1())

	if _, err := h.rt.Extend(ctx, h.scopeA(), operator, sid, ExtensionInput{Seconds: 60, Reason: "antar subtes"}); !errors.Is(err, ErrPartNotRunning) {
		t.Fatalf("extending between subtests: %v, want ErrPartNotRunning", err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_extensions WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("%d audit rows, want only the one grant that landed", n)
	}

	secondStart := h.clock.Now()
	view := h.startPart(t, p, sid, f.sub2())
	if want := secondStart.Add(15 * time.Minute); !view.Parts[1].Deadline.Equal(want) {
		t.Errorf("subtest 2 deadline = %v, want normal timing %v", view.Parts[1].Deadline, want)
	}
	if view.Parts[0].Deadline == nil || !view.Parts[0].Deadline.Equal(first.Deadline.Add(2*time.Minute)) {
		t.Errorf("subtest 1 deadline = %v, want its own extension kept", view.Parts[0].Deadline)
	}
	h.clock.Advance(15*time.Minute + time.Second)
	if _, err := h.answer(p, sid, f.tf.ID, 1, "salah"); !errors.Is(err, ErrDeadlinePassed) {
		t.Errorf("answer past subtest 2's normal deadline: %v, want ErrDeadlinePassed (no carried extension)", err)
	}
}

func TestRuntimeStartOnAnOverdueSessionSettlesItOnce(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	h.mustAnswer(t, p, sid, f.pg.ID, 1, optID(t, f.pg, "4"))

	h.clock.Advance(31 * time.Minute)
	first, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID)
	if err != nil || first.Attempt.ID != sid {
		t.Fatalf("start on an overdue session = %v (err %v), want the same session", first.Attempt.ID, err)
	}
	part := first.Parts[0]
	if part.Status != model.PartSubmitted || part.SubmitReason != model.SubmitExpired || deref(part.EarnedPoints) != 1 {
		t.Fatalf("part = %+v, want settled by expiry holding the saved answer", part)
	}
	second, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID)
	if err != nil || !second.Parts[0].SubmittedAt.Equal(*part.SubmittedAt) {
		t.Errorf("a second start changed the settled part: %+v (err %v)", second.Parts[0], err)
	}
	if n, err := h.rt.FinalizeDue(ctx); err != nil || n != 0 {
		t.Errorf("worker after start settled %d (err %v), want 0: start already did it", n, err)
	}
}

func TestRuntimeStartRacingTheWorkerYieldsOneResult(t *testing.T) {
	h := newRuntimeHarness(t)
	until := h.clock.Now().Add(time.Hour)
	f := h.fixture(t, func(in *TryoutInput) { in.ActiveUntil = &until })
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())

	h.clock.Advance(2 * time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := h.rt.Start(ctx, h.scopeA(), p, f.tryout.ID); err != nil && !errors.Is(err, ErrTryoutClosed) {
				t.Errorf("start: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := h.rt.FinalizeDue(ctx); err != nil {
				t.Errorf("finalize: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := h.count(t, `SELECT count(*) FROM tryout_results WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("%d results, want exactly 1", n)
	}
}

func TestRuntimePutAnswerDBGuardRefusesAnEqualOrOlderSequence(t *testing.T) {
	h := newRuntimeHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	ctx := context.Background()
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	four, three := optID(t, f.pg, "4"), optID(t, f.pg, "3")
	h.mustAnswer(t, p, sid, f.pg.ID, 3, four)

	put := func(seq int64, answer string) error {
		return h.repo.PutAnswer(ctx, h.pool, h.tenant().ID, sid, model.AttemptAnswer{
			QuestionID: f.pg.ID, Answer: []string{answer}, Revision: 9, Seq: seq, UpdatedAt: h.clock.Now(),
		})
	}
	for _, seq := range []int64{2, 3} {
		if err := put(seq, three); !errors.Is(err, repository.ErrStaleWrite) {
			t.Errorf("seq %d against a stored 3: %v, want ErrStaleWrite", seq, err)
		}
	}
	if saved, err := h.repo.Answer(ctx, h.pool, h.tenant().ID, sid, f.pg.ID); err != nil || saved.Seq != 3 || saved.Answer[0] != four || saved.Revision != 1 {
		t.Fatalf("stored = %+v (err %v), want the untouched seq 3 answer", saved, err)
	}
	if err := put(4, three); err != nil {
		t.Errorf("seq 4: %v, want it applied", err)
	}
}
