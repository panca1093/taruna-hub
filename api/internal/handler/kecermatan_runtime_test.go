package handler

import (
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
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// linkStore only presigns; no test here stores an object.
type linkStore struct{}

func (linkStore) Put(context.Context, string, []byte, string) error { return nil }
func (linkStore) Delete(context.Context, string) error              { return nil }
func (linkStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

type kecHTTPFixture struct {
	walletHTTPFixture
	e     http.Handler
	clock *runtimeClock
	test  model.KecermatanTest
}

func newKecHTTPFixture(t *testing.T) kecHTTPFixture {
	t.Helper()
	f := kecHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t), clock: &runtimeClock{at: time.Now().Truncate(time.Second)}}
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenantA.ID, f.tenantB.ID}
		for _, sql := range []string{
			`DELETE FROM kecermatan_results WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_session_answers WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_session_kolom WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_sessions WHERE tenant_id = ANY($1)`,
			`DELETE FROM assets WHERE tenant_id = ANY($1) AND purpose = 'item_image'`,
			`DELETE FROM enrollment_grants WHERE tenant_id = ANY($1)`,
			`DELETE FROM enrollments WHERE tenant_id = ANY($1)`,
			`DELETE FROM orders WHERE tenant_id = ANY($1)`,
			`DELETE FROM products WHERE tenant_id = ANY($1)`,
			`DELETE FROM kecermatan_tests WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup kecermatan runtime fixture: %v", err)
			}
		}
	})

	testRepo := repository.NewKecermatanRepository(f.pool)
	tests := newKecermatanService(f.tenantFixture)
	rt := service.NewKecermatanRuntimeService(f.pool, repository.NewKecermatanRuntimeRepository(f.pool), tests, testRepo,
		service.WithKecermatanRuntimeClock(f.clock.Now))
	logger := f.logger
	auth := newAuthService(f.tenantFixture)
	account := newAccountService(t, f.tenantFixture, auth)
	assets := newAssetService(f.tenantFixture, linkStore{}, time.Minute)
	runtime := newRuntimeService(f.tenantFixture)
	f.e = f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(logger))
		New(logger, NewHealthHandler(f.pool), auth, account, newCurriculumService(f.tenantFixture), newQuestionService(f.tenantFixture),
			newTryoutService(f.tenantFixture), newProductService(f.tenantFixture), newWalletService(f.tenantFixture), assets,
			newOrderService(f.tenantFixture, assets), runtime, newResultService(f.tenantFixture, runtime), newMonitoringService(f.tenantFixture, runtime),
			newItemPackageService(f.tenantFixture, assets), tests, AuthOptions{AllowedOrigin: httpx.OriginPolicy(devOrigins, true)}).
			WithKecermatanRuntime(rt).Register(e, ResolveHostScope(f.tenants, slug))
		return e
	})

	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	id := f.tenantA.ID
	admin := model.Principal{UserID: f.adminA.ID, TenantID: &id, Role: model.RoleAdmin}
	test, err := tests.Create(context.Background(), service.HostScope{Tenant: f.tenantA}, admin, service.KecermatanInput{
		ProgramID: program.ID, Title: "Kecermatan HTTP", ItemType: model.KecChar, ItemLength: 2, OptionsPerSoal: 5,
		SoalPerKolom: 50, KolomCount: 10, KolomDurationSeconds: 120, CoverFrom: "#f59e0b", CoverTo: "#ef4444",
	})
	if err != nil {
		t.Fatalf("create kecermatan test: %v", err)
	}
	f.test = test
	return f
}

func (f kecHTTPFixture) call(method, path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, method, f.hostA, path, body, token)
}

func (f kecHTTPFixture) credit(t *testing.T, user model.User, attempts int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO enrollments (tenant_id, user_id, kecermatan_test_id, attempts_purchased) VALUES ($1, $2, $3, $4)`,
		f.tenantA.ID, user.ID, f.test.ID, attempts); err != nil {
		t.Fatalf("credit student: %v", err)
	}
}

func (f kecHTTPFixture) testPath(rest string) string {
	return "/api/v1/my/kecermatan-tests/" + f.test.ID.String() + rest
}

func kecSessionPath(id uuid.UUID, rest string) string {
	return "/api/v1/my/kecermatan-sessions/" + id.String() + rest
}

// forbiddenKeys are JSON object keys that must never appear in anything a student reads during an attempt.
var forbiddenKeys = []string{"missing", "seed", "generation_seed", "columns", "correct", "key", "answer_key", "benar", "salah", "kosong", "scores"}

func assertNoSecretKeys(t *testing.T, what, body string, forbidden ...string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("%s: body %s is not JSON: %v", what, body, err)
	}
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				for _, bad := range forbidden {
					if strings.EqualFold(k, bad) {
						t.Errorf("%s leaks the key %q at %s.%s", what, k, path, k)
					}
				}
				walk(child, path+"."+k)
			}
		case []any:
			for _, child := range x {
				walk(child, path+"[]")
			}
		}
	}
	walk(decoded, "$")
}

func TestKecermatanRuntimeRoutesAuthAndRoles(t *testing.T) {
	f := newKecHTTPFixture(t)
	f.credit(t, f.studentA, 1)
	view := decodeInto[kecSessionResponse](t, f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenA))
	sid := view.ID

	student := map[string]struct{ method, path string }{
		"tests":    {http.MethodGet, "/api/v1/my/kecermatan-tests"},
		"overview": {http.MethodGet, f.testPath("")},
		"start":    {http.MethodPost, f.testPath("/sessions")},
		"session":  {http.MethodGet, kecSessionPath(sid, "")},
		"continue": {http.MethodPost, kecSessionPath(sid, "/continue")},
		"answer":   {http.MethodPut, kecSessionPath(sid, "/kolom/1/soal/1")},
		"result":   {http.MethodGet, kecSessionPath(sid, "/result")},
		"review":   {http.MethodGet, kecSessionPath(sid, "/review")},
	}
	for name, route := range student {
		if rec := f.call(route.method, route.path, map[string]any{"answer": "A"}, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s: status = %d, want 401", name, rec.Code)
		}
		for role, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			if rec := f.call(route.method, route.path, map[string]any{"answer": "A"}, token); rec.Code != http.StatusForbidden {
				t.Errorf("%s on student route %s: status = %d, want 403", role, name, rec.Code)
			}
		}
	}
	for name, path := range map[string]string{"list": "/api/v1/admin/kecermatan-results", "detail": "/api/v1/admin/kecermatan-results/" + sid.String()} {
		if rec := f.call(http.MethodGet, path, nil, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student on staff %s: status = %d, want 403", name, rec.Code)
		}
		if rec := f.call(http.MethodGet, path, nil, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous on staff %s: status = %d, want 401", name, rec.Code)
		}
	}

	// Ownership and tenancy: another student, one with no credit and one of another tenant see the same 404.
	for name, c := range map[string]struct {
		token string
		want  int
	}{"second student": {f.tokenSecond, http.StatusNotFound}, "other tenant": {f.tokenB, http.StatusUnauthorized}} {
		token, want := c.token, c.want
		for route, path := range map[string]string{"session": kecSessionPath(sid, ""), "review": kecSessionPath(sid, "/review"), "result": kecSessionPath(sid, "/result")} {
			if rec := f.call(http.MethodGet, path, nil, token); rec.Code != want {
				t.Errorf("%s reads %s of someone else's attempt: status = %d, want %d", name, route, rec.Code, want)
			}
		}
		if rec := f.call(http.MethodPut, kecSessionPath(sid, "/kolom/1/soal/1"), map[string]any{"answer": "A"}, token); rec.Code != want {
			t.Errorf("%s answers someone else's attempt: status = %d, want %d", name, rec.Code, want)
		}
		if rec := f.call(http.MethodPost, kecSessionPath(sid, "/continue"), nil, token); rec.Code != want {
			t.Errorf("%s continues someone else's attempt: status = %d, want %d", name, rec.Code, want)
		}
	}
	if rec := f.call(http.MethodGet, f.testPath(""), nil, f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("overview without credits: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("start without credits: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, "/api/v1/my/kecermatan-sessions/not-a-uuid", nil, f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed attempt id: status = %d, want 404", rec.Code)
	}
}

func TestKecermatanRuntimeFlowNeverLeaksKeysDuringAnAttempt(t *testing.T) {
	f := newKecHTTPFixture(t)
	f.credit(t, f.studentA, 1)

	rec := f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start = %d %s, want 201", rec.Code, rec.Body.String())
	}
	assertNoSecretKeys(t, "start", rec.Body.String(), forbiddenKeys...)
	session := decodeInto[kecSessionResponse](t, rec)
	if session.Phase != "kolom" || session.Kolom == nil || session.Kolom.Number != 1 || len(session.Kolom.Soal) != 50 ||
		len(session.Kolom.Reference) != 5 || session.PauseSeconds != service.KecermatanPauseSeconds || session.Result != nil {
		t.Fatalf("session = %+v, want kolom 1 with 50 soal and 5 reference items", session)
	}
	sid := session.ID
	if again := f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenA); again.Code != http.StatusOK ||
		decodeInto[kecSessionResponse](t, again).ID != sid {
		t.Errorf("second start = %d, want 200 with the same attempt", again.Code)
	}
	if strings.Contains(rec.Body.String(), f.test.Seed) {
		t.Error("a response contains the test's stored seed")
	}

	// Every read during the attempt is free of keys, scores and other kolom.
	for name, path := range map[string]string{"session": kecSessionPath(sid, ""), "overview": f.testPath(""), "list": "/api/v1/my/kecermatan-tests"} {
		read := f.call(http.MethodGet, path, nil, f.tokenA)
		if read.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", name, read.Code, read.Body.String())
		}
		assertNoSecretKeys(t, name, read.Body.String(), forbiddenKeys...)
	}
	for _, path := range []string{kecSessionPath(sid, "/result"), kecSessionPath(sid, "/review")} {
		wantCode(t, f.call(http.MethodGet, path, nil, f.tokenA), http.StatusConflict, "kecermatan_result_not_ready")
	}

	answerPath := kecSessionPath(sid, "/kolom/1/soal/1")
	if rec := f.call(http.MethodPut, answerPath, map[string]any{}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("answer without the field: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"answer": "Z"}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("answer Z: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodPut, kecSessionPath(sid, "/kolom/1/soal/99"), map[string]any{"answer": "A"}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("soal 99: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"answer": "A"}, f.tokenA); rec.Code != http.StatusOK {
		t.Fatalf("answer A = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"answer": "A"}, f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("answer retry: status = %d, want 200", rec.Code)
	}
	wantCode(t, f.call(http.MethodPut, answerPath, map[string]any{"answer": "B"}, f.tokenA), http.StatusConflict, "kecermatan_answer_locked")
	if rec := f.call(http.MethodPut, kecSessionPath(sid, "/kolom/1/soal/2"), map[string]any{"answer": nil}, f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("skip: status = %d, want 200", rec.Code)
	}
	wantCode(t, f.call(http.MethodPut, kecSessionPath(sid, "/kolom/2/soal/1"), map[string]any{"answer": "A"}, f.tokenA), http.StatusConflict, "kecermatan_kolom_not_running")
	resumed := decodeInto[kecSessionResponse](t, f.call(http.MethodGet, kecSessionPath(sid, ""), nil, f.tokenA))
	if len(resumed.Kolom.Answers) != 2 || resumed.Kolom.Answers[1].Answer != nil {
		t.Errorf("resumed answers = %+v, want the answer and the stored skip", resumed.Kolom.Answers)
	}

	// The clock passes the last kolom: the attempt completes and only now do the result and the review open.
	f.clock.Advance(10 * 125 * time.Second)
	done := decodeInto[kecSessionResponse](t, f.call(http.MethodGet, kecSessionPath(sid, ""), nil, f.tokenA))
	if done.Phase != "completed" || done.Kolom != nil || done.Result == nil {
		t.Fatalf("after the last kolom = %+v, want a completed attempt with its result", done)
	}
	wantCode(t, f.call(http.MethodPut, answerPath, map[string]any{"answer": "A"}, f.tokenA), http.StatusConflict, "kecermatan_session_completed")
	result := decodeInto[kecResultResponse](t, f.call(http.MethodGet, kecSessionPath(sid, "/result"), nil, f.tokenA))
	if result.ScoringVersion != 1 || result.BenarTotal+result.SalahTotal+result.KosongTotal != 500 || len(result.Kolom) != 10 || result.AttemptNumber != 1 {
		t.Errorf("result = %+v, want version 1 counting 500 soal over 10 kolom", result)
	}
	review := f.call(http.MethodGet, kecSessionPath(sid, "/review"), nil, f.tokenA)
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), `"missing"`) {
		t.Errorf("review = %d, want 200 carrying the reference set and the missing item", review.Code)
	}
	if strings.Contains(review.Body.String(), f.test.Seed) || strings.Contains(review.Body.String(), `"seed"`) {
		t.Error("the review leaks a seed")
	}

	// Another attempt is refused with the code the web redirects to the purchase on.
	wantCode(t, f.call(http.MethodPost, f.testPath("/sessions"), nil, f.tokenA), http.StatusConflict, "kecermatan_no_credit")

	// Staff read completed results only.
	list := decodeInto[kecStaffResultsResponse](t, f.call(http.MethodGet, "/api/v1/admin/kecermatan-results?test_id="+f.test.ID.String(), nil, f.tokenOperatorA))
	if list.Total != 1 || len(list.Results) != 1 || list.Results[0].SessionID != sid || list.Results[0].StudentName == "" {
		t.Fatalf("staff list = %+v, want the one result with the student's name", list)
	}
	detail := decodeInto[kecStaffResultDetailResponse](t, f.call(http.MethodGet, "/api/v1/admin/kecermatan-results/"+sid.String(), nil, f.tokenAdminA))
	if len(detail.Kolom) != 10 || detail.BenarTotal != result.BenarTotal {
		t.Errorf("staff detail = %+v, want the same counts and 10 kolom", detail)
	}
	if rec := f.call(http.MethodGet, "/api/v1/admin/kecermatan-results/"+sid.String(), nil, f.tokenAdmB); rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Errorf("another tenant's admin reads the result: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, "/api/v1/admin/kecermatan-results?limit=0x", nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("malformed limit: status = %d, want 422", rec.Code)
	}
}
