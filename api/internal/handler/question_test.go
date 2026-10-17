package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func sendQuestionJSON(e *echo.Echo, method, host, path string, body any, token string) *httptest.ResponseRecorder {
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

func decodeQuestion(t *testing.T, rec *httptest.ResponseRecorder) questionResponse {
	t.Helper()
	var body questionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a question: %v", rec.Body.String(), err)
	}
	return body
}

func decodeQuestions(t *testing.T, rec *httptest.ResponseRecorder) questionsResponse {
	t.Helper()
	var body questionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a question list: %v", rec.Body.String(), err)
	}
	return body
}

// questionCurriculum creates the program/subject a question needs; the tenant cleanup removes it.
func questionCurriculum(t *testing.T, e *echo.Echo, host, token string) programResponse {
	t.Helper()
	rec := sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("QSD", "Sekolah Soal"), token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create program: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeProgram(t, rec)
}

func pgQuestionBody(program programResponse) map[string]any {
	return map[string]any{
		"program_id": program.ID.String(), "subject_id": program.Subjects[0].ID.String(),
		"type": "pg", "content": "Hasil dari 2 + 2 adalah ...", "difficulty": "easy",
		"explanation": "2 + 2 = 4.",
		"options":     []any{map[string]any{"id": "a", "label": "3"}, map[string]any{"id": "b", "label": "4"}},
		"correct":     []string{"b"},
	}
}

// updateBodyOf resends a question exactly as stored, so the PUT is a no-op rather than a new version.
func updateBodyOf(question questionResponse) map[string]any {
	options := make([]any, 0, len(question.Options))
	for _, option := range question.Options {
		item := map[string]any{"id": option.ID, "label": option.Label}
		if option.Points != nil {
			item["points"] = *option.Points
		}
		options = append(options, item)
	}
	body := map[string]any{
		"program_id": question.ProgramID.String(), "subject_id": question.SubjectID.String(),
		"type": question.Type, "content": question.Content, "difficulty": question.Difficulty,
		"explanation": question.Explanation, "option_weighted": question.OptionWeighted,
		"case_sensitive": question.CaseSensitive, "options": options, "correct": question.Correct,
		"base_version": question.Version,
	}
	if question.ChapterID != nil {
		body["chapter_id"] = question.ChapterID.String()
	}
	return body
}

func TestQuestionsIsolatedBetweenTenants(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	program := questionCurriculum(t, e, hostA, adminA)
	rec := sendQuestionJSON(e, http.MethodPost, hostA, "/api/v1/questions", pgQuestionBody(program), adminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create in A: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	mine := decodeQuestion(t, rec)

	other := decodeQuestions(t, getWithToken(e, hostB, "/api/v1/questions", adminB))
	if other.Total != 0 || len(other.Questions) != 0 {
		t.Errorf("tenant B sees %+v (total %d), want none of tenant A's questions", other.Questions, other.Total)
	}
	if strings.Contains(getWithToken(e, hostB, "/api/v1/questions", adminB).Body.String(), mine.ID.String()) {
		t.Error("tenant B's list carries tenant A's question id")
	}

	path := "/api/v1/questions/" + mine.ID.String()
	if rec := getWithToken(e, hostB, path, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B GET on A's question: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	body := updateBodyOf(mine)
	if rec := sendQuestionJSON(e, http.MethodPut, hostB, path, body, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B PUT on A's question: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendQuestionJSON(e, http.MethodDelete, hostB, path, nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B DELETE on A's question: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}

	still := decodeQuestion(t, getWithToken(e, hostA, path, adminA))
	if still.ID != mine.ID || still.Content != mine.Content || still.Version != mine.Version {
		t.Errorf("tenant A question = %+v, want the original untouched", still)
	}
}

func TestQuestionRoles(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	operator := curriculumToken(t, f, tenant, model.RoleOperator)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	program := questionCurriculum(t, e, host, admin)
	first := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))
	spare := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))
	doomed := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))

	firstPath := "/api/v1/questions/" + first.ID.String()
	operatorDelete := "/api/v1/questions/" + spare.ID.String()
	adminDelete := "/api/v1/questions/" + doomed.ID.String()
	update := updateBodyOf(first)

	cases := []struct {
		method string
		path   string
		body   any
		who    string
		token  string
		want   int
	}{
		{"GET", "/api/v1/questions", nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", "/api/v1/questions", nil, "student", student, http.StatusForbidden},
		{"GET", "/api/v1/questions", nil, "operator", operator, http.StatusOK},
		{"GET", "/api/v1/questions", nil, "admin", admin, http.StatusOK},
		{"GET", firstPath, nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", firstPath, nil, "student", student, http.StatusForbidden},
		{"GET", firstPath, nil, "operator", operator, http.StatusOK},
		{"GET", firstPath, nil, "admin", admin, http.StatusOK},
		{"POST", "/api/v1/questions", pgQuestionBody(program), "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/questions", pgQuestionBody(program), "student", student, http.StatusForbidden},
		{"POST", "/api/v1/questions", pgQuestionBody(program), "operator", operator, http.StatusCreated},
		{"POST", "/api/v1/questions", pgQuestionBody(program), "admin", admin, http.StatusCreated},
		{"PUT", firstPath, update, "anonymous", "", http.StatusUnauthorized},
		{"PUT", firstPath, update, "student", student, http.StatusForbidden},
		{"PUT", firstPath, update, "operator", operator, http.StatusOK},
		{"PUT", firstPath, update, "admin", admin, http.StatusOK},
		{"DELETE", operatorDelete, nil, "anonymous", "", http.StatusUnauthorized},
		{"DELETE", operatorDelete, nil, "student", student, http.StatusForbidden},
		{"DELETE", operatorDelete, nil, "operator", operator, http.StatusNoContent},
		{"DELETE", adminDelete, nil, "admin", admin, http.StatusNoContent},
	}
	for _, tc := range cases {
		rec := sendQuestionJSON(e, tc.method, host, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s as %s: status = %d body = %s, want %d", tc.method, tc.path, tc.who, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestQuestionResponsesAreNoStore(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	created := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))
	path := "/api/v1/questions/" + created.ID.String()
	doomed := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))

	invalid := pgQuestionBody(program)
	invalid["content"] = ""

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/questions", admin)},
		{"list without a token", getWithToken(e, host, "/api/v1/questions", "")},
		{"get", getWithToken(e, host, path, admin)},
		{"create", sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin)},
		{"create rejected", sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", invalid, admin)},
		{"update", sendQuestionJSON(e, http.MethodPut, host, path, updateBodyOf(created), admin)},
		{"delete", sendQuestionJSON(e, http.MethodDelete, host, "/api/v1/questions/"+doomed.ID.String(), nil, admin)},
	}
	for _, tc := range cases {
		if got := tc.rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", tc.name, got)
		}
	}
}

func TestQuestionErrorEnvelope(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	created := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))
	path := "/api/v1/questions/" + created.ID.String()

	stale := updateBodyOf(created)
	stale["base_version"] = created.Version + 3
	stale["content"] = "Isi yang lebih baru."
	conflict := sendQuestionJSON(e, http.MethodPut, host, path, stale, admin)
	wantError(t, "stale base_version", conflict, http.StatusConflict, httpx.CodeConflict)
	if msg := errorEnvelope(t, conflict).Error.Message; !strings.Contains(msg, "muat ulang") {
		t.Errorf("stale message = %q, want it to tell the editor to reload", msg)
	}

	duplicateLabels := pgQuestionBody(program)
	duplicateLabels["options"] = []any{
		map[string]any{"id": "a", "label": "Empat"}, map[string]any{"id": "b", "label": " empat "},
	}
	wantError(t, "duplicate option labels",
		sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", duplicateLabels, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "options[1].label")

	badCorrect := pgQuestionBody(program)
	badCorrect["correct"] = []string{"z"}
	wantError(t, "correct id not in options",
		sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", badCorrect, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "correct")

	emptyContent := pgQuestionBody(program)
	emptyContent["content"] = "   "
	wantError(t, "empty content",
		sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", emptyContent, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "content")

	dataImage := pgQuestionBody(program)
	dataImage["content"] = "Lihat ![x](data:image/png;base64,AAAA)"
	wantError(t, "data image",
		sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", dataImage, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "content")

	unknownField := pgQuestionBody(program)
	unknownField["weight"] = 2
	wantError(t, "unknown field",
		sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", unknownField, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "body")

	// A question a later slice already references cannot be deleted: 409, and it is still there.
	ref := "qref_" + uuid.NewString()[:8]
	if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`CREATE TABLE %s (question_version_id uuid REFERENCES question_versions (id))`, ref)); err != nil {
		t.Fatalf("create %s: %v", ref, err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DROP TABLE `+ref); err != nil {
			t.Errorf("drop %s: %v", ref, err)
		}
	})
	if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`INSERT INTO %s (question_version_id) VALUES ($1)`, ref), created.VersionID); err != nil {
		t.Fatalf("reference version: %v", err)
	}
	referenced := sendQuestionJSON(e, http.MethodDelete, host, path, nil, admin)
	wantError(t, "referenced delete", referenced, http.StatusConflict, httpx.CodeConflict)
	if msg := errorEnvelope(t, referenced).Error.Message; !strings.Contains(msg, "konten lain") {
		t.Errorf("referenced delete message = %q, want it to name the other content", msg)
	}
	if rec := getWithToken(e, host, path, admin); rec.Code != http.StatusOK {
		t.Errorf("question after the rejected delete: status = %d, want 200", rec.Code)
	}
}

func TestStudentNeverSeesAnswerKeys(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	program := questionCurriculum(t, e, host, admin)
	created := decodeQuestion(t, sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), admin))
	path := "/api/v1/questions/" + created.ID.String()

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/questions", student)},
		{"get", getWithToken(e, host, path, student)},
		{"create", sendQuestionJSON(e, http.MethodPost, host, "/api/v1/questions", pgQuestionBody(program), student)},
		{"update", sendQuestionJSON(e, http.MethodPut, host, path, updateBodyOf(created), student)},
		{"delete", sendQuestionJSON(e, http.MethodDelete, host, path, nil, student)},
	}
	for _, tc := range cases {
		if tc.rec.Code != http.StatusForbidden {
			t.Errorf("%s as student: status = %d body = %s, want 403", tc.name, tc.rec.Code, tc.rec.Body.String())
		}
		if body := tc.rec.Body.String(); strings.Contains(body, `"correct"`) || strings.Contains(body, created.Correct[0]) {
			t.Errorf("%s as student: body carries an answer key: %s", tc.name, body)
		}
	}
}
