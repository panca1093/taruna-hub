package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

func violationsPath(id uuid.UUID) string { return sessionPath(id, "/violations") }

func staffViolationsPath(id uuid.UUID) string {
	return "/api/v1/admin/tryout-sessions/" + id.String() + "/violations"
}

const activeSessionsPath = "/api/v1/admin/monitoring/sessions"

func violationBody(kind string) map[string]any {
	return map[string]any{"event_id": uuid.NewString(), "type": kind}
}

func TestMonitoringEndpointsAuthAndRoles(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	sid := f.startSession(t, f.tokenA).ID
	f.call(http.MethodPost, sessionPath(sid, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)

	for role, token := range map[string]string{"anonymous": "", "admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		want := http.StatusForbidden
		if role == "anonymous" {
			want = http.StatusUnauthorized
		}
		if rec := f.call(http.MethodPost, violationsPath(sid), violationBody("exit_fullscreen"), token); rec.Code != want {
			t.Errorf("%s reporting a violation: status = %d, want %d", role, rec.Code, want)
		}
	}
	for name, path := range map[string]string{"list": activeSessionsPath, "timeline": staffViolationsPath(sid)} {
		for role, token := range map[string]string{"anonymous": "", "student": f.tokenA, "other student": f.tokenSecond} {
			want := http.StatusForbidden
			if role == "anonymous" {
				want = http.StatusUnauthorized
			}
			if rec := f.call(http.MethodGet, path, nil, token); rec.Code != want {
				t.Errorf("%s on staff %s: status = %d, want %d", role, name, rec.Code, want)
			}
		}
		for role, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			if rec := f.call(http.MethodGet, path, nil, token); rec.Code != http.StatusOK {
				t.Errorf("%s on staff %s: status = %d body = %s, want 200", role, name, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestMonitoringViolationFlowEndToEnd(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)
	a, b := f.startSession(t, f.tokenA), f.startSession(t, f.tokenSecond)
	path := violationsPath(a.ID)

	wantCode(t, f.call(http.MethodPost, path, violationBody("exit_fullscreen"), f.tokenA), http.StatusConflict, codeSubtestNotRunning)
	f.call(http.MethodPost, sessionPath(a.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)

	event := uuid.NewString()
	first := f.call(http.MethodPost, path, map[string]any{"event_id": event, "type": "exit_fullscreen"}, f.tokenA)
	if first.Code != http.StatusCreated {
		t.Fatalf("first report: status = %d body = %s, want 201", first.Code, first.Body.String())
	}
	created := decodeInto[violationResponse](t, first)
	if created.SessionID != a.ID || created.Type != "exit_fullscreen" || created.SubtestName != "Subtes 1" || created.OccurredAt.IsZero() {
		t.Errorf("violation = %+v", created)
	}
	retry := f.call(http.MethodPost, path, map[string]any{"event_id": event, "type": "exit_fullscreen"}, f.tokenA)
	if retry.Code != http.StatusOK || decodeInto[violationResponse](t, retry).ID != created.ID {
		t.Errorf("retry: status = %d body = %s, want 200 with the stored row", retry.Code, retry.Body.String())
	}
	if rec := f.call(http.MethodPost, path, violationBody("screenshot_attempt"), f.tokenA); rec.Code != http.StatusCreated {
		t.Fatalf("screenshot report: status = %d body = %s", rec.Code, rec.Body.String())
	}

	for name, body := range map[string]any{
		"unknown type":  violationBody("tab_switch"),
		"missing type":  map[string]any{"event_id": uuid.NewString()},
		"missing event": map[string]any{"type": "exit_fullscreen"},
		"extra field":   map[string]any{"event_id": uuid.NewString(), "type": "exit_fullscreen", "occurred_at": "2020-01-01T00:00:00Z"},
		"bad event id":  map[string]any{"event_id": "nope", "type": "exit_fullscreen"},
	} {
		if rec := f.call(http.MethodPost, path, body, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", name, rec.Code)
		}
	}

	if rec := f.call(http.MethodPost, path, violationBody("exit_fullscreen"), f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("student B on student A's session: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodPost, violationsPath(uuid.New()), violationBody("exit_fullscreen"), f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown session: status = %d, want 404", rec.Code)
	}

	list := decodeInto[activeSessionListResponse](t, f.call(http.MethodGet, activeSessionsPath, nil, f.tokenOperatorA))
	if list.Total != 2 || len(list.Sessions) != 2 {
		t.Fatalf("active list = %+v, want both sessions", list)
	}
	for _, s := range list.Sessions {
		switch s.SessionID {
		case a.ID:
			if s.ViolationCount != 2 || s.Subtest == nil || s.Subtest.Name != "Subtes 1" || s.StudentName == "" || s.TryoutTitle != "Tryout Runtime" {
				t.Errorf("student A row = %+v, want 2 violations on Subtes 1", s)
			}
		case b.ID:
			if s.ViolationCount != 0 || s.Subtest != nil {
				t.Errorf("student B row = %+v, want no violations and no running subtest", s)
			}
		default:
			t.Errorf("unexpected session %s in the list", s.SessionID)
		}
	}

	timeline := decodeInto[violationTimelineResponse](t, f.call(http.MethodGet, staffViolationsPath(a.ID), nil, f.tokenAdminA))
	if timeline.Total != 2 || len(timeline.Violations) != 2 || timeline.Status != "in_progress" ||
		timeline.Violations[0].Type != "exit_fullscreen" || timeline.Violations[1].Type != "screenshot_attempt" {
		t.Errorf("timeline = %+v, want both violations oldest first", timeline)
	}
	if rec := f.call(http.MethodGet, staffViolationsPath(uuid.New()), nil, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("guessed session id: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, activeSessionsPath+"?limit=0x", nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("non-numeric limit: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodGet, activeSessionsPath+"?limit=201", nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("limit 201: status = %d, want 422", rec.Code)
	}

	f.call(http.MethodPost, sessionPath(a.ID, "/subtests/"+f.sub(0)+"/submit"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(a.ID, "/subtests/"+f.sub(1)+"/start"), nil, f.tokenA)
	if done := decodeInto[sessionResponse](t, f.call(http.MethodPost, sessionPath(a.ID, "/subtests/"+f.sub(1)+"/submit"), nil, f.tokenA)); done.Status != "completed" {
		t.Fatalf("session status = %s, want completed", done.Status)
	}
	wantCode(t, f.call(http.MethodPost, path, violationBody("exit_fullscreen"), f.tokenA), http.StatusConflict, codeSessionCompleted)

	list = decodeInto[activeSessionListResponse](t, f.call(http.MethodGet, activeSessionsPath, nil, f.tokenOperatorA))
	if list.Total != 1 || list.Sessions[0].SessionID != b.ID {
		t.Errorf("after A completed: %+v, want only student B", list)
	}
	kept := decodeInto[violationTimelineResponse](t, f.call(http.MethodGet, staffViolationsPath(a.ID), nil, f.tokenOperatorA))
	if kept.Status != "completed" || kept.CompletedAt == nil || kept.Total != 2 {
		t.Errorf("history after completion = %+v, want the completed session with both violations", kept)
	}
}

func TestMonitoringFloodReturns429(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	sid := f.startSession(t, f.tokenA).ID
	f.call(http.MethodPost, sessionPath(sid, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)
	f.clock.Advance(time.Second)

	for i := 0; i < 100; i++ {
		if rec := f.call(http.MethodPost, violationsPath(sid), violationBody("screenshot_attempt"), f.tokenA); rec.Code != http.StatusCreated {
			t.Fatalf("violation %d: status = %d body = %s", i+1, rec.Code, rec.Body.String())
		}
	}
	wantCode(t, f.call(http.MethodPost, violationsPath(sid), violationBody("screenshot_attempt"), f.tokenA), http.StatusTooManyRequests, httpx.CodeRateLimited)
}
