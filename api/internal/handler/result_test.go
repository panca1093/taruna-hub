package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// completeAttempt answers the first question right and runs both subtests to completion over HTTP.
func (f runtimeHTTPFixture) completeAttempt(t *testing.T, token string) uuid.UUID {
	t.Helper()
	session := f.startSession(t, token)
	right := ""
	for _, o := range f.q1.Options {
		if o.Label == "4" {
			right = o.ID
		}
	}
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil, token)
	if rec := f.call(http.MethodPut, sessionPath(session.ID, "/answers/"+f.q1.ID.String()), map[string]any{"seq": 1, "answer": []string{right}}, token); rec.Code != http.StatusOK {
		t.Fatalf("answer: status = %d body = %s", rec.Code, rec.Body.String())
	}
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/submit"), nil, token)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/start"), nil, token)
	done := decodeInto[sessionResponse](t, f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/submit"), nil, token))
	if done.Status != "completed" {
		t.Fatalf("attempt did not complete: %+v", done)
	}
	return session.ID
}

func (f runtimeHTTPFixture) setRelease(t *testing.T, at *time.Time) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `UPDATE tryouts SET explanation_release_at = $2 WHERE id = $1`, f.tryout.ID, at); err != nil {
		t.Fatalf("set release time: %v", err)
	}
}

func TestResultEndpointsRoles(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	sid := f.completeAttempt(t, f.tokenA)

	studentRoutes := map[string]string{
		"result": sessionPath(sid, "/result"),
		"review": sessionPath(sid, "/review"),
	}
	for name, path := range studentRoutes {
		if rec := f.call(http.MethodGet, path, nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s: status = %d, want 401", name, rec.Code)
		}
		for role, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			if rec := f.call(http.MethodGet, path, nil, token); rec.Code != http.StatusForbidden {
				t.Errorf("%s on %s: status = %d, want 403", role, name, rec.Code)
			}
		}
		if rec := f.call(http.MethodGet, path, nil, f.tokenSecond); rec.Code != http.StatusNotFound {
			t.Errorf("another student on %s: status = %d, want 404", name, rec.Code)
		}
	}
	if rec := f.call(http.MethodGet, sessionPath(uuid.New(), "/review"), nil, f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown session review: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, "/api/v1/my/tryout-sessions/not-a-uuid/review", nil, f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed session review: status = %d, want 404", rec.Code)
	}

	staffRoutes := map[string]string{
		"results":     "/api/v1/admin/results",
		"leaderboard": "/api/v1/admin/tryouts/" + f.tryout.ID.String() + "/leaderboard",
		"student":     studentResultsPath(f.studentA.ID, f.tryout.ID),
	}
	for name, path := range staffRoutes {
		if rec := f.call(http.MethodGet, path, nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s: status = %d, want 401", name, rec.Code)
		}
		if rec := f.call(http.MethodGet, path, nil, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student on %s: status = %d, want 403", name, rec.Code)
		}
		for role, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			if rec := f.call(http.MethodGet, path, nil, token); rec.Code != http.StatusOK {
				t.Errorf("%s on %s: status = %d body = %s, want 200", role, name, rec.Code, rec.Body.String())
			}
		}
	}
	if rec := f.call(http.MethodGet, "/api/v1/admin/tryouts/"+uuid.NewString()+"/leaderboard", nil, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown tryout leaderboard: status = %d, want 404", rec.Code)
	}
	for _, query := range []string{"?limit=0x", "?limit=101", "?offset=-1", "?tryout_id=nope"} {
		if rec := f.call(http.MethodGet, "/api/v1/admin/results"+query, nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("results%s: status = %d, want 422", query, rec.Code)
		}
	}
}

func TestResultRawResponsesHideTheKeyUntilRelease(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	future := time.Now().Add(time.Hour)
	f.setRelease(t, &future)

	// Mid-attempt the student's question payloads never carry a key or an explanation.
	session := f.startSession(t, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)
	live := f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA).Body.String()
	for _, leaked := range []string{`"correct"`, `"explanation"`, "2 + 2 = 4.", `"points"`} {
		if strings.Contains(live, leaked) {
			t.Errorf("subtest payload contains %s: %s", leaked, live)
		}
	}
	right := ""
	for _, o := range f.q1.Options {
		if o.Label == "4" {
			right = o.ID
		}
	}
	f.call(http.MethodPut, sessionPath(session.ID, "/answers/"+f.q1.ID.String()), map[string]any{"seq": 1, "answer": []string{right}}, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/submit"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/start"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/submit"), nil, f.tokenA)
	sid := session.ID

	result := f.call(http.MethodGet, sessionPath(sid, "/result"), nil, f.tokenA)
	detail := decodeInto[resultDetailResponse](t, result)
	if result.Code != http.StatusOK || detail.ExplanationsReleased || detail.ExplanationReleaseAt == nil || detail.Ranking.Rank != 1 || detail.Ranking.ParticipantCount != 1 {
		t.Fatalf("result before release: status = %d body = %s, want rank 1 of 1 and explanations withheld", result.Code, result.Body.String())
	}
	if strings.Contains(result.Body.String(), "irt_warning") {
		t.Errorf("student result exposes the IRT warning: %s", result.Body.String())
	}
	for _, path := range []string{sessionPath(sid, ""), sessionPath(sid, "/result")} {
		body := f.call(http.MethodGet, path, nil, f.tokenA).Body.String()
		for _, leaked := range []string{`"correct"`, `"explanation"`, "2 + 2 = 4."} {
			if strings.Contains(body, leaked) {
				t.Errorf("%s before release contains %s: %s", path, leaked, body)
			}
		}
	}
	review := f.call(http.MethodGet, sessionPath(sid, "/review"), nil, f.tokenA)
	wantCode(t, review, http.StatusConflict, codeExplanationsNotReleased)
	for _, leaked := range []string{`"correct"`, `"explanation"`, "2 + 2 = 4.", "Hasil dari"} {
		if strings.Contains(review.Body.String(), leaked) {
			t.Errorf("review before release contains %s: %s", leaked, review.Body.String())
		}
	}

	past := time.Now().Add(-time.Hour)
	f.setRelease(t, &past)
	released := f.call(http.MethodGet, sessionPath(sid, "/review"), nil, f.tokenA)
	body := decodeInto[reviewResponse](t, released)
	if released.Code != http.StatusOK || len(body.Subtests) != 2 || len(body.Subtests[0].Questions) != 1 {
		t.Fatalf("review after release: status = %d body = %s", released.Code, released.Body.String())
	}
	q := body.Subtests[0].Questions[0]
	if q.Explanation != "2 + 2 = 4." || len(q.Correct) != 1 || !q.IsCorrect || len(q.Answer) != 1 {
		t.Errorf("released question = %+v, want the explanation, the key and the right answer", q)
	}
	if got := decodeInto[resultDetailResponse](t, f.call(http.MethodGet, sessionPath(sid, "/result"), nil, f.tokenA)); !got.ExplanationsReleased {
		t.Error("result after release must say explanations are released")
	}

	staff := decodeInto[staffResultStudentListResponse](t, f.call(http.MethodGet, "/api/v1/admin/results?tryout_id="+f.tryout.ID.String(), nil, f.tokenOperatorA))
	if staff.Total != 1 || staff.Results[0].Rank != 1 || staff.Results[0].IRTWarning || staff.Results[0].AttemptsCompleted != 1 {
		t.Errorf("staff results = %+v, want one rank 1 row without an IRT warning (weighted tryout)", staff)
	}
	board := decodeInto[leaderboardResponse](t, f.call(http.MethodGet, "/api/v1/admin/tryouts/"+f.tryout.ID.String()+"/leaderboard", nil, f.tokenAdminA))
	if board.Total != 1 || board.Entries[0].Rank != 1 || board.Entries[0].AverageScore != 50 {
		t.Errorf("leaderboard = %+v, want one entry averaging 50", board)
	}
}

func studentResultsPath(student, tryout uuid.UUID) string {
	return "/api/v1/admin/results/students/" + student.String() + "/tryouts/" + tryout.String()
}

func TestStaffStudentResultsOverHTTP(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)
	first := f.completeAttempt(t, f.tokenA)
	second := f.completeAttempt(t, f.tokenA)

	list := decodeInto[staffResultStudentListResponse](t, f.call(http.MethodGet, "/api/v1/admin/results?tryout_id="+f.tryout.ID.String(), nil, f.tokenOperatorA))
	if list.Total != 1 || len(list.Results) != 1 {
		t.Fatalf("list = %+v, want one student row for two attempts and an idle second student absent", list)
	}
	row := list.Results[0]
	if row.StudentID != f.studentA.ID || row.TryoutID != f.tryout.ID || row.AttemptsCompleted != 2 || row.LatestScore != 50 ||
		row.AverageScore != 50 || row.Rank != 1 || row.ParticipantCount != 1 || row.ScoringMode != "weighted" || row.IRTWarning ||
		row.StudentName == "" || row.TryoutTitle == "" || row.ProgramCode == "" || !strings.HasPrefix(row.StudentAvatar, "a") {
		t.Errorf("row = %+v, want the grouped row of the student", row)
	}
	raw := f.call(http.MethodGet, "/api/v1/admin/results", nil, f.tokenAdminA).Body.String()
	for _, key := range []string{"student_avatar", "last_completed_at", "attempts_completed", "latest_score"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("list body lacks %q: %s", key, raw)
		}
	}
	for _, key := range []string{"session_id", "subtests", "user_id"} {
		if strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("list body still carries per-attempt key %q: %s", key, raw)
		}
	}

	path := studentResultsPath(f.studentA.ID, f.tryout.ID)
	detail := decodeInto[staffResultStudentDetailResponse](t, f.call(http.MethodGet, path, nil, f.tokenOperatorA))
	if detail.Student.ID != f.studentA.ID || detail.Student.Name == "" || !strings.HasPrefix(detail.Student.Avatar, "a") ||
		detail.Tryout.ID != f.tryout.ID || detail.Tryout.ProgramName == "" || detail.Tryout.ProgramCode == "" || detail.Tryout.ScoringMode != "weighted" {
		t.Errorf("detail head = %+v", detail)
	}
	own := decodeInto[resultDetailResponse](t, f.call(http.MethodGet, sessionPath(second, "/result"), nil, f.tokenA))
	if detail.Ranking != own.Ranking || detail.Ranking.AttemptsCounted != 2 {
		t.Errorf("ranking = %+v, student's own = %+v, want the same", detail.Ranking, own.Ranking)
	}
	if len(detail.Attempts) != 2 || detail.Attempts[0].SessionID != second || detail.Attempts[1].SessionID != first ||
		detail.Attempts[0].AttemptNumber != 2 || len(detail.Attempts[0].Subtests) != 2 || detail.Attempts[0].Score != 50 {
		t.Errorf("attempts = %+v, want two newest first with two subtests each", detail.Attempts)
	}
	if s := detail.Attempts[0].Subtests[0]; s.Name != "Subtes 1" || s.Position != 1 || s.Score != 100 || s.CorrectCount != 1 || s.QuestionCount != 1 {
		t.Errorf("first subtest = %+v, want 100 with 1 of 1 correct", s)
	}

	for name, c := range map[string]struct {
		path, token string
		want        int
	}{
		"anonymous":           {path, "", http.StatusUnauthorized},
		"student":             {path, f.tokenA, http.StatusForbidden},
		"another tenant":      {path, f.tokenAdmB, http.StatusUnauthorized},
		"unknown student":     {studentResultsPath(uuid.New(), f.tryout.ID), f.tokenAdminA, http.StatusNotFound},
		"unknown tryout":      {studentResultsPath(f.studentA.ID, uuid.New()), f.tokenAdminA, http.StatusNotFound},
		"no completed result": {studentResultsPath(f.secondStudent.ID, f.tryout.ID), f.tokenAdminA, http.StatusNotFound},
		"malformed student":   {"/api/v1/admin/results/students/nope/tryouts/" + f.tryout.ID.String(), f.tokenAdminA, http.StatusNotFound},
		"malformed tryout":    {"/api/v1/admin/results/students/" + f.studentA.ID.String() + "/tryouts/nope", f.tokenAdminA, http.StatusNotFound},
	} {
		rec := f.call(http.MethodGet, c.path, nil, c.token)
		if rec.Code != c.want {
			t.Errorf("%s: status = %d %s, want %d", name, rec.Code, rec.Body.String(), c.want)
		} else if c.want == http.StatusNotFound && errorBody(t, rec).Message != studentResultNotFound {
			t.Errorf("%s: message = %q, want %q", name, errorBody(t, rec).Message, studentResultNotFound)
		}
	}

	for q, want := range map[string]int{"WALLET": 1, "zzz": 0, "%25": 0} {
		got := decodeInto[staffResultStudentListResponse](t, f.call(http.MethodGet, "/api/v1/admin/results?q="+q, nil, f.tokenAdminA))
		if len(got.Results) != want || got.Total != want {
			t.Errorf("q=%s = %d of %d rows, want %d", q, len(got.Results), got.Total, want)
		}
	}
	long := f.call(http.MethodGet, "/api/v1/admin/results?q="+strings.Repeat("x", 101), nil, f.tokenAdminA)
	if long.Code != http.StatusUnprocessableEntity || errorBody(t, long).Fields["q"] == "" {
		t.Errorf("101-character q = %d %s, want 422 with a q field error", long.Code, long.Body.String())
	}
}
