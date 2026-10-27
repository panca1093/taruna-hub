package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func sendTryoutJSON(e http.Handler, method, host, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(e, req)
}

func decodeTryout(t *testing.T, rec *httptest.ResponseRecorder) tryoutResponse {
	t.Helper()
	var body tryoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a tryout: %v", rec.Body.String(), err)
	}
	return body
}

func decodeTryouts(t *testing.T, rec *httptest.ResponseRecorder) tryoutsResponse {
	t.Helper()
	var body tryoutsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a tryout list: %v", rec.Body.String(), err)
	}
	return body
}

// tryoutBody is one draft with one subtest and the questions the caller names.
func tryoutBody(program programResponse, questionIDs ...string) map[string]any {
	questions := make([]any, 0, len(questionIDs))
	for _, id := range questionIDs {
		questions = append(questions, map[string]any{"question_id": id})
	}
	return map[string]any{
		"program_id": program.ID.String(), "title": "Tryout " + uuid.NewString()[:8],
		"description": "Latihan tryout", "scoring_mode": "weighted",
		"active_until":      time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		"randomize_options": true,
		"subtests": []any{map[string]any{
			"name": "Subtes 1", "duration_seconds": 1800, "questions": questions,
		}},
	}
}

// tryoutQuestion creates one question over HTTP and returns the response the body needs.
func createTryoutQuestion(t *testing.T, e http.Handler, host, token string, program programResponse) questionResponse {
	t.Helper()
	rec := sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create question: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeQuestion(t, rec)
}

func TestTryoutsIsolatedBetweenTenants(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	programA := questionCurriculum(t, e, hostA, adminA)
	questionA := createTryoutQuestion(t, e, hostA, adminA, programA)
	programB := questionCurriculum(t, e, hostB, adminB)

	rec := sendTryoutJSON(e, http.MethodPost, hostA, "/api/v1/tryouts", tryoutBody(programA, questionA.ID.String()), adminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create in A: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	mine := decodeTryout(t, rec)

	other := decodeTryouts(t, getWithToken(e, hostB, "/api/v1/tryouts", adminB))
	if other.Total != 0 || len(other.Tryouts) != 0 {
		t.Errorf("tenant B sees %+v (total %d), want none of tenant A's tryouts", other.Tryouts, other.Total)
	}
	if strings.Contains(getWithToken(e, hostB, "/api/v1/tryouts", adminB).Body.String(), mine.ID.String()) {
		t.Error("tenant B's list carries tenant A's tryout id")
	}

	path := "/api/v1/tryouts/" + mine.ID.String()
	if rec := getWithToken(e, hostB, path, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B GET on A's tryout: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	update := tryoutBody(programB, questionA.ID.String())
	update["base_revision"] = mine.Revision
	update["title"] = "Hijack"
	if rec := sendTryoutJSON(e, http.MethodPut, hostB, path, update, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B PUT on A's tryout: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendTryoutJSON(e, http.MethodDelete, hostB, path, nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B DELETE on A's tryout: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendTryoutJSON(e, http.MethodPost, hostB, path+"/publish", nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B publish on A's tryout: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendTryoutJSON(e, http.MethodPost, hostB, path+"/unpublish", nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B unpublish on A's tryout: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}

	// B cannot assign A's question: the path names the assignment and no existence leaks.
	foreign := tryoutBody(programB, questionA.ID.String())
	rec = sendTryoutJSON(e, http.MethodPost, hostB, "/api/v1/tryouts", foreign, adminB)
	wantError(t, "assign another tenant's question", rec,
		http.StatusUnprocessableEntity, httpx.CodeValidation, "subtests[0].questions[0].question_id")

	still := decodeTryout(t, getWithToken(e, hostA, path, adminA))
	if still.ID != mine.ID || still.Title != mine.Title || still.Revision != mine.Revision || len(still.Subtests) != 1 {
		t.Errorf("tenant A tryout = %+v, want the original untouched", still)
	}
}

func TestTryoutRoles(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	operator := curriculumToken(t, f, tenant, model.RoleOperator)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	program := questionCurriculum(t, e, host, admin)
	question := createTryoutQuestion(t, e, host, admin, program)

	first := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	operatorPut := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	adminPut := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	operatorDelete := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program), admin))
	adminDelete := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program), admin))
	operatorPublish := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	adminPublish := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))

	firstPath := "/api/v1/tryouts/" + first.ID.String()
	update := tryoutBody(program, question.ID.String())
	update["base_revision"] = first.Revision
	update["title"] = "Judul baru"

	// Each writer gets its own row, because a second PUT on the same row would be stale by design.
	operatorUpdate := tryoutBody(program, question.ID.String())
	operatorUpdate["base_revision"] = operatorPut.Revision
	operatorUpdate["title"] = "Judul operator"
	adminUpdate := tryoutBody(program, question.ID.String())
	adminUpdate["base_revision"] = adminPut.Revision
	adminUpdate["title"] = "Judul admin"

	cases := []struct {
		method string
		path   string
		body   any
		who    string
		token  string
		want   int
	}{
		{"GET", "/api/v1/tryouts", nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", "/api/v1/tryouts", nil, "student", student, http.StatusForbidden},
		{"GET", "/api/v1/tryouts", nil, "operator", operator, http.StatusOK},
		{"GET", "/api/v1/tryouts", nil, "admin", admin, http.StatusOK},
		{"GET", firstPath, nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", firstPath, nil, "student", student, http.StatusForbidden},
		{"GET", firstPath, nil, "operator", operator, http.StatusOK},
		{"GET", firstPath, nil, "admin", admin, http.StatusOK},
		{"POST", "/api/v1/tryouts", tryoutBody(program), "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/tryouts", tryoutBody(program), "student", student, http.StatusForbidden},
		{"POST", "/api/v1/tryouts", tryoutBody(program), "operator", operator, http.StatusCreated},
		{"POST", "/api/v1/tryouts", tryoutBody(program), "admin", admin, http.StatusCreated},
		{"PUT", firstPath, update, "anonymous", "", http.StatusUnauthorized},
		{"PUT", firstPath, update, "student", student, http.StatusForbidden},
		{"PUT", "/api/v1/tryouts/" + operatorPut.ID.String(), operatorUpdate, "operator", operator, http.StatusOK},
		{"PUT", "/api/v1/tryouts/" + adminPut.ID.String(), adminUpdate, "admin", admin, http.StatusOK},
		{"POST", firstPath + "/publish", nil, "anonymous", "", http.StatusUnauthorized},
		{"POST", firstPath + "/publish", nil, "student", student, http.StatusForbidden},
		{"POST", "/api/v1/tryouts/" + operatorPublish.ID.String() + "/publish", nil, "operator", operator, http.StatusOK},
		{"POST", "/api/v1/tryouts/" + adminPublish.ID.String() + "/publish", nil, "admin", admin, http.StatusOK},
		{"POST", "/api/v1/tryouts/" + operatorPublish.ID.String() + "/unpublish", nil, "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/tryouts/" + operatorPublish.ID.String() + "/unpublish", nil, "student", student, http.StatusForbidden},
		{"POST", "/api/v1/tryouts/" + operatorPublish.ID.String() + "/unpublish", nil, "operator", operator, http.StatusOK},
		{"POST", "/api/v1/tryouts/" + adminPublish.ID.String() + "/unpublish", nil, "admin", admin, http.StatusOK},
		{"DELETE", "/api/v1/tryouts/" + operatorDelete.ID.String(), nil, "anonymous", "", http.StatusUnauthorized},
		{"DELETE", "/api/v1/tryouts/" + operatorDelete.ID.String(), nil, "student", student, http.StatusForbidden},
		{"DELETE", "/api/v1/tryouts/" + operatorDelete.ID.String(), nil, "operator", operator, http.StatusNoContent},
		{"DELETE", "/api/v1/tryouts/" + adminDelete.ID.String(), nil, "admin", admin, http.StatusNoContent},
	}
	for _, tc := range cases {
		rec := sendTryoutJSON(e, tc.method, host, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s as %s: status = %d body = %s, want %d", tc.method, tc.path, tc.who, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestTryoutResponsesAreNoStore(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	question := createTryoutQuestion(t, e, host, admin, program)
	created := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	path := "/api/v1/tryouts/" + created.ID.String()
	update := tryoutBody(program, question.ID.String())
	update["base_revision"] = created.Revision
	invalid := tryoutBody(program)
	invalid["title"] = ""

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/tryouts", admin)},
		{"list without a token", getWithToken(e, host, "/api/v1/tryouts", "")},
		{"get", getWithToken(e, host, path, admin)},
		{"create", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program), admin)},
		{"create rejected", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", invalid, admin)},
		{"update", sendTryoutJSON(e, http.MethodPut, host, path, update, admin)},
		{"publish", sendTryoutJSON(e, http.MethodPost, host, path+"/publish", nil, admin)},
		{"unpublish", sendTryoutJSON(e, http.MethodPost, host, path+"/unpublish", nil, admin)},
	}
	for _, tc := range cases {
		if got := tc.rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", tc.name, got)
		}
	}

	doomed := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program), admin))
	if got := sendTryoutJSON(e, http.MethodDelete, host, "/api/v1/tryouts/"+doomed.ID.String(), nil, admin).Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("delete: Cache-Control = %q, want no-store", got)
	}
}

func TestTryoutErrorEnvelope(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	question := createTryoutQuestion(t, e, host, admin, program)
	created := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	path := "/api/v1/tryouts/" + created.ID.String()

	stale := tryoutBody(program, question.ID.String())
	stale["base_revision"] = created.Revision + 5
	stale["title"] = "Judul lebih baru"
	conflict := sendTryoutJSON(e, http.MethodPut, host, path, stale, admin)
	wantError(t, "stale base_revision", conflict, http.StatusConflict, httpx.CodeConflict)
	if msg := errorEnvelope(t, conflict).Error.Message; !strings.Contains(msg, "muat ulang") {
		t.Errorf("stale message = %q, want it to tell the builder to reload", msg)
	}

	emptyTitle := tryoutBody(program)
	emptyTitle["title"] = "   "
	wantError(t, "empty title", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", emptyTitle, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "title")

	badWindow := tryoutBody(program)
	badWindow["active_from"] = time.Now().Add(96 * time.Hour).UTC().Format(time.RFC3339)
	wantError(t, "window reversed", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", badWindow, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "active_from")

	badQuestion := tryoutBody(program, uuid.NewString())
	wantError(t, "unknown question", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", badQuestion, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "subtests[0].questions[0].question_id")

	unknownField := tryoutBody(program)
	unknownField["cover_from"] = "#fff"
	wantError(t, "unknown field", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", unknownField, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "body")

	draft := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program), admin))
	emptyPublish := sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts/"+draft.ID.String()+"/publish", nil, admin)
	wantError(t, "publish without a question", emptyPublish, http.StatusUnprocessableEntity, httpx.CodeValidation, "subtests[0].questions")

	// The lock answers 409 with the exact copy the builder shows.
	if _, err := f.pool.Exec(context.Background(), `UPDATE tryouts SET first_attempt_at = now() WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("stamp first_attempt_at: %v", err)
	}
	lockedBody := tryoutBody(program, question.ID.String())
	lockedBody["base_revision"] = created.Revision
	locked := sendTryoutJSON(e, http.MethodPut, host, path, lockedBody, admin)
	wantError(t, "locked update", locked, http.StatusConflict, httpx.CodeConflict)
	if msg := errorEnvelope(t, locked).Error.Message; msg != "Tryout sudah dikerjakan siswa dan tidak bisa diubah." {
		t.Errorf("locked message = %q, want the agreed copy", msg)
	}
	wantError(t, "locked delete", sendTryoutJSON(e, http.MethodDelete, host, path, nil, admin),
		http.StatusConflict, httpx.CodeConflict)

	wantError(t, "another tenant's id shape", getWithToken(e, host, "/api/v1/tryouts/"+uuid.NewString(), admin),
		http.StatusNotFound, httpx.CodeNotFound)
}

func TestTryoutResponsesNeverCarryAnswerKeys(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	question := createTryoutQuestion(t, e, host, admin, program)
	created := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin))
	path := "/api/v1/tryouts/" + created.ID.String()
	update := tryoutBody(program, question.ID.String())
	update["base_revision"] = created.Revision
	update["title"] = "Judul (diubah)"

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/tryouts", admin)},
		{"get", getWithToken(e, host, path, admin)},
		{"create", sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts", tryoutBody(program, question.ID.String()), admin)},
		{"update", sendTryoutJSON(e, http.MethodPut, host, path, update, admin)},
		{"unpublish", sendTryoutJSON(e, http.MethodPost, host, path+"/unpublish", nil, admin)},
		{"publish", sendTryoutJSON(e, http.MethodPost, host, path+"/publish", nil, admin)},
	}
	for _, tc := range cases {
		body := tc.rec.Body.String()
		if tc.rec.Code < 200 || tc.rec.Code >= 300 {
			t.Fatalf("%s: status = %d body = %s", tc.name, tc.rec.Code, body)
		}
		for _, leaked := range []string{`"correct"`, `"options"`, `"explanation"`} {
			if strings.Contains(body, leaked) {
				t.Errorf("%s: body carries %s: %s", tc.name, leaked, body)
			}
		}
		if !strings.Contains(body, `"content"`) && tc.name != "list" {
			t.Errorf("%s: body has no assignment content, so the check proved nothing: %s", tc.name, body)
		}
	}
}

func TestTryoutBodyIsBoundedBeforeItIsMapped(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	program := questionCurriculum(t, e, host, admin)

	item := `{"question_id":""},`
	body := `{"program_id":"` + program.ID.String() + `","title":"x","scoring_mode":"weighted","active_until":"2030-01-01T00:00:00Z","subtests":[{"name":"s","duration_seconds":600,"questions":[` +
		strings.Repeat(item, 200000) + `{"question_id":""}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tryouts", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+admin)
	req.Host = host
	rec := serve(e, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized body: status = %d, want 422", rec.Code)
	}
	if got := len(errorEnvelope(t, rec).Error.Fields); got > 2 {
		t.Fatalf("oversized body produced %d field errors, want it refused before it is mapped", got)
	}
}
