package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const (
	secretExplanation = "PEMBAHASAN-RAHASIA-empat"
	irtTestThreshold  = 3
)

type resultHarness struct {
	runtimeHarness
	rs      *ResultService
	results *repository.ResultRepository
}

func newResultHarness(t *testing.T) resultHarness {
	t.Helper()
	base := newRuntimeHarness(t)
	results := repository.NewResultRepository(base.pool)
	return resultHarness{
		runtimeHarness: base, results: results,
		rs: NewResultService(base.rt, results, base.repo, WithResultClock(base.clock.Now), WithIRTThreshold(irtTestThreshold)),
	}
}

func (h resultHarness) question(t *testing.T) model.Question {
	t.Helper()
	in := h.q.pgInput(h.tenant())
	in.Explanation = secretExplanation
	return h.q.create(t, h.tenant(), in)
}

func (h resultHarness) tryoutOf(t *testing.T, mutate func(*TryoutInput), qs ...model.Question) model.Tryout {
	t.Helper()
	in := h.tryoutInput(h.tenant(), subtestInput("Subtes", 1800, qs...))
	if mutate != nil {
		mutate(&in)
	}
	created := h.create(t, h.tenant(), in)
	published, err := h.publish(h.tenant(), created.ID)
	if err != nil {
		t.Fatalf("publish tryout: %v", err)
	}
	return published
}

// take runs one whole attempt of a one-subtest tryout. An empty option leaves the question blank.
func (h resultHarness) take(t *testing.T, p model.Principal, tr model.Tryout, answers map[uuid.UUID]string) uuid.UUID {
	t.Helper()
	view, err := h.rt.Start(context.Background(), h.scopeA(), p, tr.ID)
	if err != nil {
		t.Fatalf("start attempt: %v", err)
	}
	sid := view.Attempt.ID
	h.startPart(t, p, sid, tr.Subtests[0].ID)
	for question, option := range answers {
		if option != "" {
			h.mustAnswer(t, p, sid, question, 1, option)
		}
	}
	done := h.submitPart(t, p, sid, tr.Subtests[0].ID)
	if done.Attempt.Status != model.SessionCompleted {
		t.Fatalf("attempt did not complete: %+v", done.Attempt)
	}
	return sid
}

func (h resultHarness) ranking(t *testing.T, tryoutID, userID uuid.UUID) model.Ranking {
	t.Helper()
	r, ok, err := h.results.Ranking(context.Background(), h.pool, h.tenant().ID, tryoutID, userID)
	if err != nil || !ok {
		t.Fatalf("ranking of %s = ok %v err %v", userID, ok, err)
	}
	return r
}

func (h resultHarness) addScores(t *testing.T, userID, tryoutID uuid.UUID, scores ...float64) {
	t.Helper()
	for _, s := range scores {
		if err := h.repo.AddScoreTotal(context.Background(), h.pool, h.tenant().ID, userID, tryoutID, s); err != nil {
			t.Fatalf("add score total: %v", err)
		}
	}
}

func TestResultRankingAveragesAllAttemptsAndTiesShareARank(t *testing.T) {
	h := newResultHarness(t)
	tr := h.tryoutOf(t, nil, h.question(t))
	student := func(scores ...float64) uuid.UUID {
		u, _ := h.student(t)
		h.addScores(t, u.ID, tr.ID, scores...)
		return u.ID
	}

	threeAttempts := student(60, 80, 100)
	single80 := student(80)
	top := student(100)
	low := student(50)

	if r := h.ranking(t, tr.ID, threeAttempts); r.Average != 80 || r.Attempts != 3 {
		t.Errorf("60/80/100 ranking = %+v, want average 80 over 3 attempts", r)
	}
	want := map[uuid.UUID]int{top: 1, threeAttempts: 2, single80: 2, low: 4}
	for id, rank := range want {
		if r := h.ranking(t, tr.ID, id); r.Rank != rank || r.Participants != 4 {
			t.Errorf("student %s ranking = %+v, want rank %d of 4 (1, 2, 2, 4)", id, r, rank)
		}
	}

	board, total, err := h.rs.Leaderboard(context.Background(), h.scopeA(), h.staff(model.RoleOperator), tr.ID, 0, 0)
	if err != nil || total != 4 || len(board) != 4 {
		t.Fatalf("leaderboard = %d rows of %d (err %v), want 4 of 4", len(board), total, err)
	}
	ranks := []int{board[0].Rank, board[1].Rank, board[2].Rank, board[3].Rank}
	if ranks[0] != 1 || ranks[1] != 2 || ranks[2] != 2 || ranks[3] != 4 {
		t.Errorf("leaderboard ranks = %v, want [1 2 2 4]", ranks)
	}
	page, total, err := h.rs.Leaderboard(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), tr.ID, 2, 3)
	if err != nil || total != 4 || len(page) != 1 || page[0].UserID != low {
		t.Errorf("leaderboard page = %+v of %d (err %v), want the last student of 4", page, total, err)
	}
}

func TestResultTiesAreJudgedOnTheDisplayedAverage(t *testing.T) {
	h := newResultHarness(t)
	tr := h.tryoutOf(t, nil, h.question(t))
	a, _ := h.student(t)
	b, _ := h.student(t)
	h.addScores(t, a.ID, tr.ID, 66.67, 66.66)
	h.addScores(t, b.ID, tr.ID, 66.67)
	ra, rb := h.ranking(t, tr.ID, a.ID), h.ranking(t, tr.ID, b.ID)
	if ra.Average != 66.67 || ra.Rank != 1 || rb.Rank != 1 {
		t.Errorf("averages 66.665 and 66.67 rank %d and %d (average %v), want both 66.67 and rank 1", ra.Rank, rb.Rank, ra.Average)
	}
}

func TestResultDetailShowsTheLiveRankFromEveryAttempt(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	_, strong := h.enrolledStudentOf(t, tr)
	_, weak := h.enrolledStudentOf(t, tr)

	weakSession := h.take(t, weak, tr, map[uuid.UUID]string{q.ID: optID(t, q, "3")})
	detail, err := h.rs.Detail(context.Background(), h.scopeA(), weak, weakSession)
	if err != nil || detail.Result.Score != 0 || detail.Ranking.Rank != 1 || detail.Ranking.Participants != 1 {
		t.Fatalf("weak alone = %+v (err %v), want score 0, rank 1 of 1", detail, err)
	}

	strongSession := h.take(t, strong, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})
	detail, err = h.rs.Detail(context.Background(), h.scopeA(), weak, weakSession)
	if err != nil || detail.Ranking.Rank != 2 || detail.Ranking.Participants != 2 || detail.Ranking.Average != 0 {
		t.Errorf("weak after strong = %+v (err %v), want the rank to move to 2 of 2 without any stored value", detail.Ranking, err)
	}
	detail, err = h.rs.Detail(context.Background(), h.scopeA(), strong, strongSession)
	if err != nil || detail.Result.Score != 100 || detail.Ranking.Rank != 1 || detail.Ranking.Average != 100 {
		t.Errorf("strong = %+v (err %v), want score 100 and rank 1", detail, err)
	}

	// A retake that scores 100 moves the average to 50: still the single attempt that matters is the average.
	retake := h.take(t, weak, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})
	for _, sid := range []uuid.UUID{weakSession, retake} {
		d, err := h.rs.Detail(context.Background(), h.scopeA(), weak, sid)
		if err != nil || d.Ranking.Average != 50 || d.Ranking.Attempts != 2 || d.Ranking.Rank != 2 {
			t.Errorf("weak attempt %s ranking = %+v (err %v), want average 50 over 2 attempts, rank 2", sid, d.Ranking, err)
		}
	}

	_, other := h.enrolledStudentOf(t, tr)
	if _, err := h.rs.Detail(context.Background(), h.scopeA(), other, weakSession); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("another student's detail err = %v, want not found", err)
	}
	if _, err := h.rs.Detail(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), weakSession); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("staff detail err = %v, want forbidden", err)
	}
}

func (h resultHarness) enrolledStudentOf(t *testing.T, tr model.Tryout) (model.User, model.Principal) {
	t.Helper()
	u, p := h.student(t)
	h.enroll(t, u, tr.ID)
	return u, p
}

func TestResultReviewIsWithheldUntilTheReleaseTime(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	release := h.clock.Now().Add(24 * time.Hour)
	tr := h.tryoutOf(t, func(in *TryoutInput) { in.ExplanationReleaseAt = &release }, q)
	_, p := h.enrolledStudentOf(t, tr)

	// An attempt in progress has no review at all.
	inProgress, err := h.rt.Start(context.Background(), h.scopeA(), p, tr.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := h.rs.Review(context.Background(), h.scopeA(), p, inProgress.Attempt.ID); !errors.Is(err, ErrNotCompleted) {
		t.Errorf("review of an open session err = %v, want not completed", err)
	}
	h.startPart(t, p, inProgress.Attempt.ID, tr.Subtests[0].ID)
	h.mustAnswer(t, p, inProgress.Attempt.ID, q.ID, 1, optID(t, q, "3"))
	h.submitPart(t, p, inProgress.Attempt.ID, tr.Subtests[0].ID)
	sid := inProgress.Attempt.ID

	before, err := h.rs.Review(context.Background(), h.scopeA(), p, sid)
	if !errors.Is(err, ErrExplanationsNotReleased) {
		t.Fatalf("review before release err = %v, want not released", err)
	}
	if raw, _ := json.Marshal(before); strings.Contains(string(raw), secretExplanation) || len(before.Subtests) != 0 {
		t.Errorf("review before release leaked a body: %s", raw)
	}
	detail, err := h.rs.Detail(context.Background(), h.scopeA(), p, sid)
	if err != nil || detail.Released || detail.ReleaseAt == nil || !detail.ReleaseAt.Equal(release) {
		t.Errorf("detail before release = released %v at %v (err %v), want withheld until %v", detail.Released, detail.ReleaseAt, err, release)
	}

	h.clock.Advance(24*time.Hour + time.Second)
	after, err := h.rs.Review(context.Background(), h.scopeA(), p, sid)
	if err != nil {
		t.Fatalf("review after release: %v", err)
	}
	item := after.Subtests[0].Questions[0]
	if item.Question.Explanation != secretExplanation || len(item.Question.Correct) != 1 || item.Question.Correct[0] != optID(t, q, "4") {
		t.Errorf("review item = %+v, want the explanation and the key after release", item.Question)
	}
	if item.Correct || len(item.Answer) != 1 || item.Answer[0] != optID(t, q, "3") {
		t.Errorf("review answer = %v correct %v, want the student's wrong answer", item.Answer, item.Correct)
	}

	_, other := h.enrolledStudentOf(t, tr)
	if _, err := h.rs.Review(context.Background(), h.scopeA(), other, sid); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("another student's review err = %v, want not found", err)
	}
	if _, err := h.rs.Review(context.Background(), h.scopeA(), h.staff(model.RoleOperator), sid); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("staff review err = %v, want forbidden", err)
	}
}

func TestResultNoReleaseTimeMeansReviewIsOpenAtOnce(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	tr := h.tryoutOf(t, nil, q)
	_, p := h.enrolledStudentOf(t, tr)
	sid := h.take(t, p, tr, map[uuid.UUID]string{q.ID: optID(t, q, "4")})
	review, err := h.rs.Review(context.Background(), h.scopeA(), p, sid)
	if err != nil || !review.Subtests[0].Questions[0].Correct {
		t.Errorf("review = %+v (err %v), want an immediate review with the answer marked correct", review, err)
	}
}

// irtFixture builds the PRD's calibration case on a threshold of three students.
func TestResultIRTCountsDistinctStudentsPerQuestionVersion(t *testing.T) {
	h := newResultHarness(t)
	q := h.question(t)
	irt := func(in *TryoutInput) { in.ScoringMode = model.ScoringIRT }
	tr := h.tryoutOf(t, irt, q)
	right := optID(t, q, "4")

	warned := func(sid uuid.UUID) bool {
		t.Helper()
		under, err := h.results.IRTUnderThreshold(context.Background(), h.pool, h.tenant().ID, []uuid.UUID{sid}, irtTestThreshold)
		if err != nil {
			t.Fatalf("irt count: %v", err)
		}
		return under[sid]
	}

	_, a := h.enrolledStudentOf(t, tr)
	_, b := h.enrolledStudentOf(t, tr)
	_, c := h.enrolledStudentOf(t, tr)
	_, blank := h.enrolledStudentOf(t, tr)

	sa := h.take(t, a, tr, map[uuid.UUID]string{q.ID: right})
	// A retake by the same student is the same student: it must not push the count up.
	h.take(t, a, tr, map[uuid.UUID]string{q.ID: right})
	sb := h.take(t, b, tr, map[uuid.UUID]string{q.ID: right})
	// A blank answer is not a response.
	h.take(t, blank, tr, map[uuid.UUID]string{q.ID: ""})
	if !warned(sa) {
		t.Error("two distinct students (one retaking) and a blank must stay under the threshold of 3")
	}

	// An attempt that is still running answers the question without counting.
	_, running := h.enrolledStudentOf(t, tr)
	open, err := h.rt.Start(context.Background(), h.scopeA(), running, tr.ID)
	if err != nil {
		t.Fatalf("start running attempt: %v", err)
	}
	h.startPart(t, running, open.Attempt.ID, tr.Subtests[0].ID)
	h.mustAnswer(t, running, open.Attempt.ID, q.ID, 1, right)
	if !warned(sa) {
		t.Error("an incomplete session must not count as a respondent")
	}

	sc := h.take(t, c, tr, map[uuid.UUID]string{q.ID: optID(t, q, "3")})
	if warned(sa) || warned(sb) || warned(sc) {
		t.Error("three distinct students with non-blank answers (right or wrong) reach the threshold; every attempt of the version clears")
	}

	// A new version of the same question starts again from zero.
	updated, err := h.q.update(t, h.tenant(), q.ID, QuestionInput{
		ProgramID: q.ProgramID, SubjectID: q.SubjectID, Type: q.Type, Content: "Hasil dari 3 + 3 adalah ...",
		Difficulty: q.Difficulty, Explanation: secretExplanation, BaseVersion: q.Version,
		Options: []QuestionOptionInput{{ID: "a", Label: "6"}, {ID: "b", Label: "7"}}, Correct: []string{"a"},
	})
	if err != nil {
		t.Fatalf("update question: %v", err)
	}
	tr2 := h.tryoutOf(t, irt, updated)
	_, d := h.enrolledStudentOf(t, tr2)
	sd := h.take(t, d, tr2, map[uuid.UUID]string{updated.ID: optID(t, updated, "6")})
	if !warned(sd) {
		t.Error("version 2 has one respondent and must warn even though version 1 is well calibrated")
	}
	if warned(sa) {
		t.Error("the old attempt on version 1 must not change when version 2 appears")
	}
}

func TestResultStaffListFiltersRanksAndFlagsIRTOnlyOnIRTTryouts(t *testing.T) {
	h := newResultHarness(t)
	q1, q2 := h.question(t), h.question(t)
	weighted := h.tryoutOf(t, nil, q1)
	irt := h.tryoutOf(t, func(in *TryoutInput) { in.ScoringMode = model.ScoringIRT }, q2)

	ada, pa := h.enrolledStudentOf(t, weighted)
	h.enroll(t, ada, irt.ID)
	_, pb := h.enrolledStudentOf(t, weighted)
	h.take(t, pa, weighted, map[uuid.UUID]string{q1.ID: optID(t, q1, "4")})
	h.take(t, pb, weighted, map[uuid.UUID]string{q1.ID: optID(t, q1, "3")})
	h.take(t, pa, irt, map[uuid.UUID]string{q2.ID: optID(t, q2, "4")})

	ctx, scope := context.Background(), h.scopeA()
	rows, total, err := h.rs.StaffResults(ctx, scope, h.staff(model.RoleOperator), StaffResultFilter{TryoutID: &weighted.ID})
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("weighted results = %d of %d (err %v), want 2", len(rows), total, err)
	}
	if rows[0].Rank != 1 || rows[0].Score != 100 || rows[1].Rank != 2 || rows[1].Participants != 2 || rows[0].IRTWarning || rows[1].IRTWarning {
		t.Errorf("weighted rows = %+v / %+v, want ranks 1 and 2 of 2 and no IRT flag", rows[0], rows[1])
	}
	if len(rows[0].Subtests) != 1 || rows[0].Subtests[0].Score != 100 {
		t.Errorf("subtests = %+v, want the one subtest scored 100", rows[0].Subtests)
	}

	rows, total, err = h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{TryoutID: &irt.ID})
	if err != nil || total != 1 || !rows[0].IRTWarning {
		t.Errorf("irt results = %+v of %d (err %v), want one row with the IRT warning (1 respondent < 3)", rows, total, err)
	}

	rows, total, err = h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{})
	if err != nil || total != 3 || len(rows) != 3 {
		t.Errorf("all results = %d of %d (err %v), want 3", len(rows), total, err)
	}
	rows, total, err = h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{Limit: 1, Offset: 2})
	if err != nil || total != 3 || len(rows) != 1 {
		t.Errorf("page = %d of %d (err %v), want 1 of 3", len(rows), total, err)
	}
	rows, total, err = h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{Search: "%"})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Errorf("a literal %% search = %d rows (err %v), want none", len(rows), err)
	}
	rows, _, err = h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{Search: "siswa runtime"})
	if err != nil || len(rows) != 3 {
		t.Errorf("name search = %d rows (err %v), want all 3", len(rows), err)
	}

	if _, _, err := h.rs.StaffResults(ctx, scope, pa, StaffResultFilter{}); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student staff list err = %v, want forbidden", err)
	}
	if _, _, err := h.rs.StaffResults(ctx, scope, h.staff(model.RoleAdmin), StaffResultFilter{Limit: 101}); err == nil {
		t.Error("a limit over 100 must be rejected")
	}
	if _, _, err := h.rs.Leaderboard(ctx, scope, pa, weighted.ID, 0, 0); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student leaderboard err = %v, want forbidden", err)
	}
	if _, _, err := h.rs.Leaderboard(ctx, scope, h.staff(model.RoleAdmin), uuid.New(), 0, 0); !errors.Is(err, ErrTryoutNotFound) {
		t.Errorf("unknown tryout leaderboard err = %v, want tryout not found", err)
	}
}

func TestIRTDefaultThresholdIsThirty(t *testing.T) {
	if IRTMinStudents != 30 {
		t.Fatalf("IRTMinStudents = %d, want 30 (PRD, D04)", IRTMinStudents)
	}
}
