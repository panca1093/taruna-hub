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
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func newCurriculumService(f tenantFixture) *service.CurriculumService {
	return service.NewCurriculumService(f.pool, repository.NewCurriculumRepository(f.pool))
}

func newQuestionService(f tenantFixture) *service.QuestionService {
	return service.NewQuestionService(f.pool, repository.NewQuestionRepository(f.pool))
}

func newTryoutService(f tenantFixture) *service.TryoutService {
	return service.NewTryoutService(f.pool, repository.NewTryoutRepository(f.pool))
}

func newProductService(f tenantFixture) *service.ProductService {
	return service.NewProductService(f.pool, repository.NewProductRepository(f.pool))
}

func newWalletService(f tenantFixture) *service.WalletService {
	return service.NewWalletService(f.pool, repository.NewWalletRepository(f.pool))
}

// curriculumToken gives one tenant its own account, so role checks never borrow another tenant's.
func curriculumToken(t *testing.T, f authFixture, tenant model.Tenant, role model.Role) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(authTestPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "curr-" + uuid.NewString()[:8] + "@example.com"
	user, err := repository.NewUserRepository(f.pool).Create(context.Background(), model.User{
		TenantID: &tenant.ID, Name: "Curriculum Probe", Email: &email,
		PasswordHash: string(hash), Role: role,
	})
	if err != nil {
		t.Fatalf("create %s user: %v", role, err)
	}
	return loginToken(t, newAuthService(f.tenantFixture), service.HostScope{Tenant: tenant}, *user.Email, authTestPassword)
}

func sendCurriculum(e *echo.Echo, method, host, path string, body any, token string) *httptest.ResponseRecorder {
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

func decodeProgram(t *testing.T, rec *httptest.ResponseRecorder) programResponse {
	t.Helper()
	var body programResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a program: %v", rec.Body.String(), err)
	}
	return body
}

func decodePrograms(t *testing.T, rec *httptest.ResponseRecorder) programsResponse {
	t.Helper()
	var body programsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a program list: %v", rec.Body.String(), err)
	}
	return body
}

// sdBody is one program with one subject and one chapter, the shape every write test starts from.
func sdBody(code, name string) map[string]any {
	return map[string]any{
		"code": code, "name": name, "description": "Latihan SD",
		"subjects": []any{
			map[string]any{"name": "Matematika", "chapters": []any{map[string]any{"name": "Bilangan"}}},
		},
	}
}

func TestCurriculumIsolatedBetweenTenants(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	rec := sendCurriculum(e, http.MethodPost, hostA, "/api/v1/programs", sdBody("SD", "Sekolah Dasar"), adminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create in A: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	mine := decodeProgram(t, rec)

	other := decodePrograms(t, getWithToken(e, hostB, "/api/v1/programs", adminB))
	if len(other.Programs) != 0 {
		t.Errorf("tenant B sees %+v, want none of tenant A's programs", other.Programs)
	}
	if strings.Contains(getWithToken(e, hostB, "/api/v1/programs", adminB).Body.String(), mine.ID.String()) {
		t.Error("tenant B's list carries tenant A's program id")
	}

	// B may reuse the code — the unique key is per tenant — and its tree stays its own.
	if rec := sendCurriculum(e, http.MethodPost, hostB, "/api/v1/programs", sdBody("SD", "SD Sekolah B"), adminB); rec.Code != http.StatusCreated {
		t.Errorf("create in B with a code A already uses: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	own := decodePrograms(t, getWithToken(e, hostB, "/api/v1/programs", adminB))
	if len(own.Programs) != 1 || own.Programs[0].ID == mine.ID || own.Programs[0].Name != "SD Sekolah B" {
		t.Errorf("tenant B programs = %+v, want only its own program", own.Programs)
	}

	path := "/api/v1/programs/" + mine.ID.String()
	if rec := sendCurriculum(e, http.MethodPut, hostB, path, sdBody("SD", "Hijack"), adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B PUT on A's program: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendCurriculum(e, http.MethodDelete, hostB, path, nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B DELETE on A's program: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}

	// A's tree is untouched by B's attempts.
	still := decodePrograms(t, getWithToken(e, hostA, "/api/v1/programs", adminA))
	if len(still.Programs) != 1 || still.Programs[0].Name != "Sekolah Dasar" || len(still.Programs[0].Subjects) != 1 {
		t.Errorf("tenant A programs = %+v, want the original tree", still.Programs)
	}
}

func TestCurriculumRoles(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	operator := curriculumToken(t, f, tenant, model.RoleOperator)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	created := decodeProgram(t, sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("SD", "Sekolah Dasar"), admin))
	path := "/api/v1/programs/" + created.ID.String()
	update := map[string]any{
		"code": "SD", "name": "Sekolah Dasar",
		"subjects": []any{map[string]any{
			"id": created.Subjects[0].ID.String(), "name": "Matematika",
			"chapters": []any{map[string]any{"id": created.Subjects[0].Chapters[0].ID.String(), "name": "Bilangan"}},
		}},
	}

	cases := []struct {
		method string
		path   string
		body   any
		who    string
		token  string
		want   int
	}{
		{"GET", "/api/v1/programs", nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", "/api/v1/programs", nil, "student", student, http.StatusForbidden},
		{"GET", "/api/v1/programs", nil, "operator", operator, http.StatusOK},
		{"GET", "/api/v1/programs", nil, "admin", admin, http.StatusOK},
		{"POST", "/api/v1/programs", sdBody("SMP", "SMP"), "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/programs", sdBody("SMP", "SMP"), "student", student, http.StatusForbidden},
		{"POST", "/api/v1/programs", sdBody("SMP", "SMP"), "operator", operator, http.StatusForbidden},
		{"POST", "/api/v1/programs", sdBody("SMP", "SMP"), "admin", admin, http.StatusCreated},
		{"PUT", path, update, "anonymous", "", http.StatusUnauthorized},
		{"PUT", path, update, "student", student, http.StatusForbidden},
		{"PUT", path, update, "operator", operator, http.StatusForbidden},
		{"PUT", path, update, "admin", admin, http.StatusOK},
		{"DELETE", path, nil, "anonymous", "", http.StatusUnauthorized},
		{"DELETE", path, nil, "student", student, http.StatusForbidden},
		{"DELETE", path, nil, "operator", operator, http.StatusForbidden},
		{"DELETE", path, nil, "admin", admin, http.StatusNoContent},
	}
	for _, tc := range cases {
		rec := sendCurriculum(e, tc.method, host, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s as %s: status = %d body = %s, want %d", tc.method, tc.path, tc.who, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestCurriculumResponsesAreNoStore(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	created := decodeProgram(t, sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("SD", "SD"), admin))
	path := "/api/v1/programs/" + created.ID.String()
	update := map[string]any{"code": "SD", "name": "SD"}

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/programs", admin)},
		{"list without a token", getWithToken(e, host, "/api/v1/programs", "")},
		{"create", sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("SMP", "SMP"), admin)},
		{"create rejected", sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("X", "X"), admin)},
		{"update", sendCurriculum(e, http.MethodPut, host, path, update, admin)},
		{"delete", sendCurriculum(e, http.MethodDelete, host, path, nil, admin)},
	}
	for _, tc := range cases {
		if got := tc.rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", tc.name, got)
		}
	}
}

func TestCurriculumErrorEnvelope(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	first := decodeProgram(t, sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("SD", "SD"), admin))
	second := decodeProgram(t, sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody("SMP", "SMP"), admin))

	duplicate := sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", sdBody(" sd ", "SD Lagi"), admin)
	wantError(t, "duplicate code", duplicate, http.StatusConflict, httpx.CodeConflict, "code")

	cases := []struct {
		name string
		body any
		path string
	}{
		{
			name: "duplicate subject names",
			body: map[string]any{"code": "SMA", "name": "SMA", "subjects": []any{
				map[string]any{"name": "Fisika"}, map[string]any{"name": "FISIKA"},
			}},
			path: "subjects[1].name",
		},
		{
			name: "duplicate chapter names",
			body: map[string]any{"code": "SMA", "name": "SMA", "subjects": []any{
				map[string]any{"name": "Fisika", "chapters": []any{
					map[string]any{"name": "Mekanika"}, map[string]any{"name": "mekanika"},
				}},
			}},
			path: "subjects[0].chapters[1].name",
		},
		{"code rule", map[string]any{"code": "S", "name": "S"}, "code"},
		{"empty name", map[string]any{"code": "SMA"}, "name"},
	}
	for _, tc := range cases {
		wantError(t, tc.name, sendCurriculum(e, http.MethodPost, host, "/api/v1/programs", tc.body, admin),
			http.StatusUnprocessableEntity, httpx.CodeValidation, tc.path)
	}

	// An id from another program of the same tenant is a field error naming its path.
	foreign := map[string]any{
		"code": "SD", "name": "SD",
		"subjects": []any{
			map[string]any{"name": "Matematika"},
			map[string]any{"id": second.Subjects[0].ID.String(), "name": "Fisika"},
		},
	}
	wantError(t, "foreign subject id",
		sendCurriculum(e, http.MethodPut, host, "/api/v1/programs/"+first.ID.String(), foreign, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "subjects[1].id")

	// A referenced program cannot be deleted: 409 conflict, and it is still there.
	ref := "ref_" + uuid.NewString()[:8]
	if _, err := f.pool.Exec(context.Background(), fmt.Sprintf(`CREATE TABLE %s (chapter_id uuid REFERENCES chapters (id))`, ref)); err != nil {
		t.Fatalf("create %s: %v", ref, err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DROP TABLE `+ref); err != nil {
			t.Errorf("drop %s: %v", ref, err)
		}
	})
	if _, err := f.pool.Exec(context.Background(),
		fmt.Sprintf(`INSERT INTO %s (chapter_id) VALUES ($1)`, ref), first.Subjects[0].Chapters[0].ID); err != nil {
		t.Fatalf("reference chapter: %v", err)
	}
	referenced := sendCurriculum(e, http.MethodDelete, host, "/api/v1/programs/"+first.ID.String(), nil, admin)
	wantError(t, "referenced delete", referenced, http.StatusConflict, httpx.CodeConflict)
	if msg := errorEnvelope(t, referenced).Error.Message; !strings.Contains(msg, "Bab materi") {
		t.Errorf("referenced delete message = %q, want it to name the chapter", msg)
	}

	if list := decodePrograms(t, getWithToken(e, host, "/api/v1/programs", admin)); len(list.Programs) != 2 {
		t.Errorf("programs = %d after the rejected delete, want both still there", len(list.Programs))
	}
}
