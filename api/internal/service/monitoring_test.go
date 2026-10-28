package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type monitoringHarness struct {
	runtimeHarness
	ms *MonitoringService
}

func newMonitoringHarness(t *testing.T) monitoringHarness {
	t.Helper()
	base := newRuntimeHarness(t)
	return monitoringHarness{runtimeHarness: base, ms: NewMonitoringService(base.rt, repository.NewMonitoringRepository(base.pool))}
}

func (h monitoringHarness) report(p model.Principal, session uuid.UUID, kind string) (model.Violation, bool, error) {
	return h.ms.RecordViolation(context.Background(), h.scopeA(), p, session, ViolationInput{EventID: uuid.New(), Type: kind})
}

func (h monitoringHarness) mustReport(t *testing.T, p model.Principal, session uuid.UUID, kind string) model.Violation {
	t.Helper()
	v, created, err := h.report(p, session, kind)
	if err != nil || !created {
		t.Fatalf("record %s: created = %v, err = %v", kind, created, err)
	}
	return v
}

func (h monitoringHarness) active(t *testing.T) ([]model.ActiveSession, int) {
	t.Helper()
	rows, total, err := h.ms.ActiveSessions(context.Background(), h.scopeA(), h.staff(model.RoleOperator), 0, 0)
	if err != nil {
		t.Fatalf("list active sessions: %v", err)
	}
	return rows, total
}

func (h monitoringHarness) finish(t *testing.T, p model.Principal, session uuid.UUID, f rtFixture) {
	t.Helper()
	h.submitPart(t, p, session, f.sub1())
	h.startPart(t, p, session, f.sub2())
	if view := h.submitPart(t, p, session, f.sub2()); view.Attempt.Status != model.SessionCompleted {
		t.Fatalf("session status = %s, want completed after both subtests", view.Attempt.Status)
	}
}

func TestViolationIsStampedWithServerTimeAndTheRunningSubtest(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())

	at := h.clock.Now()
	v := h.mustReport(t, p, sid, model.ViolationExitFullscreen)
	if !v.OccurredAt.Equal(at) || v.SubtestID != f.sub1() || v.Type != model.ViolationExitFullscreen {
		t.Errorf("violation = %+v, want the server clock %s on subtest 1", v, at)
	}
	if v.SubtestName != "Subtes 1" {
		t.Errorf("subtest name = %q, want Subtes 1", v.SubtestName)
	}

	h.clock.Advance(90 * time.Second)
	later := h.mustReport(t, p, sid, model.ViolationScreenshotAttempt)
	if !later.OccurredAt.Equal(at.Add(90 * time.Second)) {
		t.Errorf("second violation at %s, want %s", later.OccurredAt, at.Add(90*time.Second))
	}

	view, err := h.rt.Session(context.Background(), h.scopeA(), p, sid)
	if err != nil || view.Attempt.Status != model.SessionInProgress || view.Parts[0].Status != model.PartRunning {
		t.Errorf("session after violations = %+v (err %v), want the timer and session untouched", view.Attempt.Status, err)
	}
}

func TestViolationRetryOfTheSameEventAddsNothing(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	ctx, event := context.Background(), uuid.New()

	first, created, err := h.ms.RecordViolation(ctx, h.scopeA(), p, sid, ViolationInput{EventID: event, Type: model.ViolationExitFullscreen})
	if err != nil || !created {
		t.Fatalf("first send: created = %v, err = %v", created, err)
	}
	h.clock.Advance(time.Minute)
	again, created, err := h.ms.RecordViolation(ctx, h.scopeA(), p, sid, ViolationInput{EventID: event, Type: model.ViolationExitFullscreen})
	if err != nil || created || again.ID != first.ID || !again.OccurredAt.Equal(first.OccurredAt) {
		t.Fatalf("retry = %+v created=%v err=%v, want the stored row unchanged", again, created, err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("stored %d violations, want 1", n)
	}
}

func TestViolationInputIsValidated(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())

	for name, in := range map[string]ViolationInput{
		"unknown type":  {EventID: uuid.New(), Type: "tab_switch"},
		"empty type":    {EventID: uuid.New()},
		"no event id":   {Type: model.ViolationExitFullscreen},
		"wrong casing":  {EventID: uuid.New(), Type: "Exit_Fullscreen"},
		"padded string": {EventID: uuid.New(), Type: " exit_fullscreen"},
	} {
		var input *InputError
		if _, _, err := h.ms.RecordViolation(context.Background(), h.scopeA(), p, sid, in); !errors.As(err, &input) {
			t.Errorf("%s: %v, want an InputError", name, err)
		}
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != 0 {
		t.Errorf("rejected input stored %d rows, want 0", n)
	}
}

func TestViolationOnlyOnTheStudentsOwnSession(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, owner := h.enrolledStudent(t, f)
	_, stranger := h.student(t)
	sid := h.begin(t, owner, f).Attempt.ID
	h.startPart(t, owner, sid, f.sub1())

	if _, _, err := h.report(stranger, sid, model.ViolationExitFullscreen); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("another student: %v, want ErrSessionNotFound", err)
	}
	if _, _, err := h.report(owner, uuid.New(), model.ViolationExitFullscreen); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("unknown session: %v, want ErrSessionNotFound", err)
	}
	for _, role := range []model.Role{model.RoleAdmin, model.RoleOperator} {
		if _, _, err := h.report(h.staff(role), sid, model.ViolationExitFullscreen); !errors.Is(err, ErrRuntimeForbidden) {
			t.Errorf("%s reporting: %v, want ErrRuntimeForbidden", role, err)
		}
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != 0 {
		t.Errorf("refused reports stored %d rows, want 0", n)
	}
}

func TestViolationOnlyWhileASubtestRuns(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID

	if _, _, err := h.report(p, sid, model.ViolationExitFullscreen); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("before any subtest starts: %v, want ErrPartNotRunning", err)
	}
	h.startPart(t, p, sid, f.sub1())
	h.mustReport(t, p, sid, model.ViolationExitFullscreen)
	h.submitPart(t, p, sid, f.sub1())
	if _, _, err := h.report(p, sid, model.ViolationExitFullscreen); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("between subtests: %v, want ErrPartNotRunning", err)
	}

	h.startPart(t, p, sid, f.sub2())
	h.submitPart(t, p, sid, f.sub2())
	if _, _, err := h.report(p, sid, model.ViolationExitFullscreen); !errors.Is(err, ErrSessionClosed) {
		t.Errorf("after the session completed: %v, want ErrSessionClosed", err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != 1 {
		t.Errorf("stored %d violations, want only the one made while subtest 1 ran", n)
	}
}

func TestViolationAfterTheDeadlineIsRefusedAndNeverStored(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())

	h.clock.Advance(31 * time.Minute)
	if _, _, err := h.report(p, sid, model.ViolationExitFullscreen); !errors.Is(err, ErrPartNotRunning) {
		t.Errorf("after subtest 1 expired: %v, want ErrPartNotRunning", err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != 0 {
		t.Errorf("stored %d violations after the deadline, want 0", n)
	}
}

func TestViolationFloodIsCappedPerSession(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	ctx := context.Background()

	var first uuid.UUID
	for i := 0; i < MaxViolationsPerSession; i++ {
		event := uuid.New()
		if i == 0 {
			first = event
		}
		if _, _, err := h.ms.RecordViolation(ctx, h.scopeA(), p, sid, ViolationInput{EventID: event, Type: model.ViolationScreenshotAttempt}); err != nil {
			t.Fatalf("violation %d: %v", i+1, err)
		}
	}
	if _, _, err := h.report(p, sid, model.ViolationScreenshotAttempt); !errors.Is(err, ErrViolationLimit) {
		t.Errorf("one past the cap: %v, want ErrViolationLimit", err)
	}
	if _, created, err := h.ms.RecordViolation(ctx, h.scopeA(), p, sid, ViolationInput{EventID: first, Type: model.ViolationScreenshotAttempt}); err != nil || created {
		t.Errorf("retry of a stored event at the cap: created = %v, err = %v, want it acknowledged unchanged", created, err)
	}
	if n := h.count(t, `SELECT count(*) FROM tryout_session_violations WHERE session_id = $1`, sid); n != MaxViolationsPerSession {
		t.Errorf("stored %d violations, want exactly the cap %d", n, MaxViolationsPerSession)
	}
}

func TestMonitoringListsActiveSessionsAndDropsCompletedOnes(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	userA, pa := h.enrolledStudent(t, f)
	_, pb := h.enrolledStudent(t, f)

	if rows, total := h.active(t); len(rows) != 0 || total != 0 {
		t.Fatalf("before any start: %d rows, total %d, want none", len(rows), total)
	}

	sa := h.begin(t, pa, f).Attempt.ID
	h.clock.Advance(time.Minute)
	sb := h.begin(t, pb, f).Attempt.ID
	rows, total := h.active(t)
	if len(rows) != 2 || total != 2 || rows[0].SessionID != sb || rows[1].SessionID != sa {
		t.Fatalf("after two starts: %+v total %d, want both, newest first", rows, total)
	}
	if rows[1].Subtest != nil || rows[1].ViolationCount != 0 {
		t.Errorf("a session between subtests = %+v, want no active subtest and no violations", rows[1])
	}

	h.startPart(t, pa, sa, f.sub1())
	h.mustReport(t, pa, sa, model.ViolationExitFullscreen)
	h.mustReport(t, pa, sa, model.ViolationScreenshotAttempt)
	rows, _ = h.active(t)
	got := rows[1]
	if got.StudentName != userA.Name || got.TryoutTitle != f.tryout.Title || got.AttemptNumber != 1 ||
		got.Subtest == nil || got.Subtest.ID != f.sub1() || got.Subtest.Name != "Subtes 1" || got.ViolationCount != 2 {
		t.Errorf("active row = %+v subtest %+v, want student, tryout, running subtest 1 and 2 violations", got, got.Subtest)
	}

	h.startPart(t, pb, sb, f.sub1())
	h.finish(t, pb, sb, f)
	rows, total = h.active(t)
	if len(rows) != 1 || total != 1 || rows[0].SessionID != sa {
		t.Errorf("after student B completed: %+v total %d, want only student A", rows, total)
	}
}

func TestMonitoringPagesTheActiveList(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	for i := 0; i < 3; i++ {
		_, p := h.enrolledStudent(t, f)
		h.begin(t, p, f)
		h.clock.Advance(time.Second)
	}
	ctx, op := context.Background(), h.staff(model.RoleOperator)
	rows, total, err := h.ms.ActiveSessions(ctx, h.scopeA(), op, 2, 2)
	if err != nil || len(rows) != 1 || total != 3 {
		t.Fatalf("page 2 of size 2: %d rows, total %d, err %v, want 1 row of 3", len(rows), total, err)
	}
	var input *InputError
	for _, bad := range [][2]int{{201, 0}, {-1, 0}, {10, -1}} {
		if _, _, err := h.ms.ActiveSessions(ctx, h.scopeA(), op, bad[0], bad[1]); !errors.As(err, &input) {
			t.Errorf("limit %d offset %d: %v, want an InputError", bad[0], bad[1], err)
		}
	}
}

func TestViolationHistoryPersistsAfterCompletion(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	user, p := h.enrolledStudent(t, f)
	sid := h.begin(t, p, f).Attempt.ID
	h.startPart(t, p, sid, f.sub1())
	first := h.mustReport(t, p, sid, model.ViolationExitFullscreen)
	h.clock.Advance(5 * time.Second)
	second := h.mustReport(t, p, sid, model.ViolationScreenshotAttempt)
	h.finish(t, p, sid, f)

	for _, role := range []model.Role{model.RoleAdmin, model.RoleOperator} {
		info, list, err := h.ms.Timeline(context.Background(), h.scopeA(), h.staff(role), sid)
		if err != nil {
			t.Fatalf("%s timeline: %v", role, err)
		}
		if info.Status != model.SessionCompleted || info.CompletedAt == nil || info.StudentName != user.Name || info.TryoutTitle != f.tryout.Title {
			t.Errorf("%s: info = %+v, want the completed session of %s", role, info, user.Name)
		}
		if len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID ||
			list[0].Type != model.ViolationExitFullscreen || list[1].Type != model.ViolationScreenshotAttempt ||
			!list[1].OccurredAt.After(list[0].OccurredAt) {
			t.Errorf("%s: timeline = %+v, want both violations oldest first", role, list)
		}
	}
	if rows, _ := h.active(t); len(rows) != 0 {
		t.Errorf("the completed session is still listed as active: %+v", rows)
	}
}

func TestMonitoringIsStaffOnly(t *testing.T) {
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	_, owner := h.enrolledStudent(t, f)
	_, other := h.student(t)
	sid := h.begin(t, owner, f).Attempt.ID
	ctx, scope := context.Background(), h.scopeA()

	for name, p := range map[string]model.Principal{"owner": owner, "other student": other} {
		if _, _, err := h.ms.ActiveSessions(ctx, scope, p, 0, 0); !errors.Is(err, ErrRuntimeForbidden) {
			t.Errorf("%s listing active sessions: %v, want ErrRuntimeForbidden", name, err)
		}
		if _, _, err := h.ms.Timeline(ctx, scope, p, sid); !errors.Is(err, ErrRuntimeForbidden) {
			t.Errorf("%s reading a timeline: %v, want ErrRuntimeForbidden", name, err)
		}
	}
	if _, _, err := h.ms.Timeline(ctx, scope, h.staff(model.RoleAdmin), uuid.New()); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("a guessed session id: %v, want ErrSessionNotFound", err)
	}
}

// TestMonitoringPollingFiftyActiveSessions is P19's documented polling fixture: 50 in-progress sessions with
// violations on every one, polled repeatedly the way the monitoring page does. The p95 must stay within 500ms.
func TestMonitoringPollingFiftyActiveSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 50 sessions")
	}
	h := newMonitoringHarness(t)
	f := h.fixture(t, nil)
	for i := 0; i < 50; i++ {
		_, p := h.enrolledStudent(t, f)
		sid := h.begin(t, p, f).Attempt.ID
		h.startPart(t, p, sid, f.sub1())
		for j := 0; j < 1+i%4; j++ {
			h.mustReport(t, p, sid, model.ViolationExitFullscreen)
		}
		h.clock.Advance(time.Second)
	}

	const polls = 200
	took := make([]time.Duration, 0, polls)
	for i := 0; i < polls; i++ {
		start := time.Now()
		rows, total, err := h.ms.ActiveSessions(context.Background(), h.scopeA(), h.staff(model.RoleOperator), 0, 0)
		took = append(took, time.Since(start))
		if err != nil || len(rows) != 50 || total != 50 {
			t.Fatalf("poll %d: %d rows, total %d, err %v, want 50", i, len(rows), total, err)
		}
	}
	sort.Slice(took, func(a, b int) bool { return took[a] < took[b] })
	p50, p95, worst := took[polls/2], took[polls*95/100], took[polls-1]
	t.Logf("polling 50 active sessions x%d: p50 = %s, p95 = %s, max = %s", polls, p50, p95, worst)
	if p95 > 500*time.Millisecond {
		t.Errorf("p95 = %s, want <= 500ms", p95)
	}
}
