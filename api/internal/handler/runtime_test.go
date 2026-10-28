package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// runtimeClock is the runtime service's now(): the exam rules are time-based, so tests move it and never sleep.
type runtimeClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *runtimeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *runtimeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

type runtimeHTTPFixture struct {
	walletHTTPFixture
	e      http.Handler
	clock  *runtimeClock
	tryout tryoutResponse
	q1, q2 questionResponse
}

func newRuntimeHTTPFixture(t *testing.T) runtimeHTTPFixture {
	t.Helper()
	f := runtimeHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t), clock: &runtimeClock{at: time.Now().Truncate(time.Second)}}
	// Registered after the tenant cleanup, so sessions go before the tryouts and users they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenantA.ID, f.tenantB.ID}
		for _, sql := range []string{
			`DELETE FROM tryout_score_totals WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_results WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_extensions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_answers WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_questions WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_session_parts WHERE tenant_id = ANY($1)`,
			`DELETE FROM tryout_sessions WHERE tenant_id = ANY($1)`,
			`DELETE FROM enrollments WHERE tenant_id = ANY($1)`,
			`DELETE FROM tenant_settings WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup runtime fixture: %v", err)
			}
		}
	})

	rt := service.NewRuntimeService(f.pool, repository.NewRuntimeRepository(f.pool), service.WithRuntimeClock(f.clock.Now))
	f.e = f.runtimeServer(t, rt)

	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	f.q1 = createTryoutQuestion(t, f.e, f.hostA, f.tokenAdminA, program)
	f.q2 = createTryoutQuestion(t, f.e, f.hostA, f.tokenAdminA, program)
	body := map[string]any{
		"program_id": program.ID.String(), "title": "Tryout Runtime", "description": "Uji runtime", "scoring_mode": "weighted",
		"active_until": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		"subtests": []any{
			map[string]any{"name": "Subtes 1", "duration_seconds": 600, "questions": []any{map[string]any{"question_id": f.q1.ID.String()}}},
			map[string]any{"name": "Subtes 2", "duration_seconds": 600, "questions": []any{map[string]any{"question_id": f.q2.ID.String()}}},
		},
	}
	created := decodeTryout(t, sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/tryouts", body, f.tokenAdminA))
	published := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/tryouts/"+created.ID.String()+"/publish", nil, f.tokenAdminA)
	if published.Code != http.StatusOK {
		t.Fatalf("publish tryout: status = %d body = %s", published.Code, published.Body.String())
	}
	f.tryout = decodeTryout(t, published)
	return f
}

// runtimeServer is echoWithAssets with the runtime service the test wants, so its clock is the test's clock.
func (f runtimeHTTPFixture) runtimeServer(t *testing.T, rt *service.RuntimeService) http.Handler {
	t.Helper()
	logger := f.logger
	auth := newAuthService(f.tenantFixture)
	account := newAccountService(t, f.tenantFixture, auth)
	assets := newAssetService(f.tenantFixture, nil, time.Minute)
	return f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(logger))
		e.Use(httpx.CORS(devOrigins, true))
		New(logger, NewHealthHandler(f.pool), auth, account, newCurriculumService(f.tenantFixture), newQuestionService(f.tenantFixture),
			newTryoutService(f.tenantFixture), newProductService(f.tenantFixture), newWalletService(f.tenantFixture), assets,
			newOrderService(f.tenantFixture, assets), rt, AuthOptions{AllowedOrigin: httpx.OriginPolicy(devOrigins, true)}).
			Register(e, ResolveHostScope(f.tenants, slug))
		return e
	})
}

func (f runtimeHTTPFixture) call(method, path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, method, f.hostA, path, body, token)
}

func (f runtimeHTTPFixture) enroll(t *testing.T, student model.User) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO enrollments (tenant_id, user_id, tryout_id) VALUES ($1, $2, $3)`, f.tenantA.ID, student.ID, f.tryout.ID); err != nil {
		t.Fatalf("enroll: %v", err)
	}
}

func (f runtimeHTTPFixture) sub(i int) string { return f.tryout.Subtests[i].ID.String() }

func decodeInto[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	return out
}

func (f runtimeHTTPFixture) startSession(t *testing.T, token string) sessionResponse {
	t.Helper()
	rec := f.call(http.MethodPost, "/api/v1/my/tryouts/"+f.tryout.ID.String()+"/sessions", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("start session: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeInto[sessionResponse](t, rec)
}

func sessionPath(id uuid.UUID, rest string) string {
	return "/api/v1/my/tryout-sessions/" + id.String() + rest
}

func wantCode(t *testing.T, rec *httptest.ResponseRecorder, status int, code httpx.Code) {
	t.Helper()
	if rec.Code != status || errorBody(t, rec).Code != code {
		t.Fatalf("status = %d body = %s, want %d %s", rec.Code, rec.Body.String(), status, code)
	}
}

func TestRuntimeEndpointsAuthAndRoles(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	sid := f.startSession(t, f.tokenA).ID
	student := map[string]struct{ method, path string }{
		"overview":  {http.MethodGet, "/api/v1/my/tryouts/" + f.tryout.ID.String()},
		"start":     {http.MethodPost, "/api/v1/my/tryouts/" + f.tryout.ID.String() + "/sessions"},
		"attempts":  {http.MethodGet, "/api/v1/my/tryout-sessions"},
		"session":   {http.MethodGet, sessionPath(sid, "")},
		"result":    {http.MethodGet, sessionPath(sid, "/result")},
		"subtest":   {http.MethodGet, sessionPath(sid, "/subtests/"+f.sub(0))},
		"start sub": {http.MethodPost, sessionPath(sid, "/subtests/"+f.sub(0)+"/start")},
		"submit":    {http.MethodPost, sessionPath(sid, "/subtests/"+f.sub(0)+"/submit")},
		"answer":    {http.MethodPut, sessionPath(sid, "/answers/"+f.q1.ID.String())},
	}
	for name, route := range student {
		if rec := f.call(route.method, route.path, map[string]any{"seq": 1, "answer": []string{}}, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s: status = %d, want 401", name, rec.Code)
		}
		for role, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			if rec := f.call(route.method, route.path, map[string]any{"seq": 1, "answer": []string{}}, token); rec.Code != http.StatusForbidden {
				t.Errorf("%s on student route %s: status = %d, want 403", role, name, rec.Code)
			}
		}
	}

	staff := map[string]struct{ method, path string }{
		"extension": {http.MethodPost, "/api/v1/admin/tryout-sessions/" + sid.String() + "/extensions"},
		"settings":  {http.MethodGet, "/api/v1/admin/settings"},
		"put":       {http.MethodPut, "/api/v1/admin/settings"},
		"grace":     {http.MethodPut, "/api/v1/tryouts/" + f.tryout.ID.String() + "/grace"},
	}
	for name, route := range staff {
		if rec := f.call(route.method, route.path, map[string]any{}, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student on %s: status = %d, want 403", name, rec.Code)
		}
		if rec := f.call(route.method, route.path, map[string]any{}, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous on %s: status = %d, want 401", name, rec.Code)
		}
	}
	for _, name := range []string{"settings", "put", "grace"} {
		route := staff[name]
		if rec := f.call(route.method, route.path, map[string]any{}, f.tokenOperatorA); rec.Code != http.StatusForbidden {
			t.Errorf("operator on admin-only %s: status = %d, want 403", name, rec.Code)
		}
	}
}

func TestRuntimeEndpointsFlowOwnershipAndSecrecy(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)
	tryoutPath := "/api/v1/my/tryouts/" + f.tryout.ID.String()

	_, tokenNoAccess := walletAccount(t, f.authFixture, f.tenantA, model.RoleStudent)
	if rec := f.call(http.MethodGet, tryoutPath, nil, tokenNoAccess); rec.Code != http.StatusNotFound {
		t.Errorf("student without access reads the overview: status = %d, want 404", rec.Code)
	}
	if rec := f.call(http.MethodGet, "/api/v1/my/tryouts/not-a-uuid", nil, f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed tryout id: status = %d, want 404", rec.Code)
	}
	overview := decodeInto[studentTryoutResponse](t, f.call(http.MethodGet, tryoutPath, nil, f.tokenA))
	if overview.Title != "Tryout Runtime" || len(overview.Subtests) != 2 || overview.Current != nil || len(overview.Attempts) != 0 {
		t.Fatalf("overview = %+v, want the tryout with two subtests and no session yet", overview)
	}

	session := f.startSession(t, f.tokenA)
	if again := f.startSession(t, f.tokenA); again.ID != session.ID {
		t.Fatalf("a second start returned session %s, want the same %s", again.ID, session.ID)
	}
	if session.Status != "in_progress" || session.AttemptNumber != 1 || len(session.Subtests) != 2 || session.Result != nil {
		t.Fatalf("session = %+v", session)
	}
	if rec := f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA); rec.Code != http.StatusConflict {
		t.Errorf("reading an unstarted subtest: status = %d, want 409", rec.Code)
	} else {
		wantCode(t, rec, http.StatusConflict, codeSubtestNotRunning)
	}
	wantCode(t, f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/start"), nil, f.tokenA), http.StatusConflict, codeSubtestOrder)

	started := decodeInto[sessionResponse](t, f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA))
	if started.Subtests[0].Status != "running" || started.Subtests[0].DeadlineAt == nil {
		t.Fatalf("started subtest = %+v, want running with a deadline", started.Subtests[0])
	}
	if want := started.ServerTime.Add(10 * time.Minute); !started.Subtests[0].DeadlineAt.Equal(want) {
		t.Errorf("deadline = %v, want server time + 10 minutes %v", started.Subtests[0].DeadlineAt, want)
	}

	read := f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA)
	if read.Code != http.StatusOK {
		t.Fatalf("read subtest: status = %d body = %s", read.Code, read.Body.String())
	}
	raw := read.Body.String()
	for _, secret := range []string{`"correct"`, `"points"`, `"explanation"`, `"version`, `"difficulty"`, "2 + 2 = 4"} {
		if strings.Contains(raw, secret) {
			t.Errorf("the student's questions leak %q: %s", secret, raw)
		}
	}
	subtest := decodeInto[runtimeSubtestResponse](t, read)
	if len(subtest.Questions) != 1 || subtest.Questions[0].ID != f.q1.ID || len(subtest.Questions[0].Options) != 2 {
		t.Fatalf("questions = %+v, want the one pg question with two options", subtest.Questions)
	}
	right := ""
	for _, o := range f.q1.Options {
		if o.Label == "4" {
			right = o.ID
		}
	}

	answerPath := sessionPath(session.ID, "/answers/"+f.q1.ID.String())
	ack := decodeInto[answerResponse](t, f.call(http.MethodPut, answerPath, map[string]any{"seq": 1, "answer": []string{right}}, f.tokenA))
	if ack.Revision != 1 || ack.Seq != 1 || ack.QuestionID != f.q1.ID {
		t.Errorf("ack = %+v", ack)
	}
	if retry := decodeInto[answerResponse](t, f.call(http.MethodPut, answerPath, map[string]any{"seq": 1, "answer": []string{right}}, f.tokenA)); retry.Revision != 1 {
		t.Errorf("retry revision = %d, want 1", retry.Revision)
	}
	wantCode(t, f.call(http.MethodPut, answerPath, map[string]any{"seq": 1, "answer": []string{}}, f.tokenA), http.StatusConflict, codeStaleAnswer)
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"seq": 0, "answer": []string{}}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("seq 0: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"seq": 2, "answer": []string{"nope"}}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown option: status = %d, want 422", rec.Code)
	}
	if rec := f.call(http.MethodPut, answerPath, map[string]any{"seq": 2, "answer": []string{}, "extra": 1}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown field: status = %d, want 422", rec.Code)
	}
	reread := decodeInto[runtimeSubtestResponse](t, f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA))
	if got := reread.Questions[0]; got.Revision != 1 || got.Seq != 1 || len(got.Answer) != 1 || got.Answer[0] != right {
		t.Errorf("accepted answer after a reload = %+v, want the saved one", got)
	}

	// Student B (a different student, same tenant) cannot touch student A's session by any route.
	for name, route := range map[string]struct {
		method, path string
		body         any
	}{
		"session": {http.MethodGet, sessionPath(session.ID, ""), nil},
		"result":  {http.MethodGet, sessionPath(session.ID, "/result"), nil},
		"subtest": {http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil},
		"start":   {http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil},
		"submit":  {http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/submit"), nil},
		"answer":  {http.MethodPut, answerPath, map[string]any{"seq": 9, "answer": []string{}}},
	} {
		if rec := f.call(route.method, route.path, route.body, f.tokenSecond); rec.Code != http.StatusNotFound {
			t.Errorf("student B on A's %s: status = %d body = %s, want 404", name, rec.Code, rec.Body.String())
		}
	}
	if list := decodeInto[attemptListResponse](t, f.call(http.MethodGet, "/api/v1/my/tryout-sessions", nil, f.tokenSecond)); len(list.Attempts) != 0 {
		t.Errorf("student B lists %d attempts, want 0", len(list.Attempts))
	}
	wantCode(t, f.call(http.MethodGet, sessionPath(session.ID, "/result"), nil, f.tokenA), http.StatusConflict, codeResultNotReady)

	submit1 := sessionPath(session.ID, "/subtests/"+f.sub(0)+"/submit")
	first := decodeInto[sessionResponse](t, f.call(http.MethodPost, submit1, nil, f.tokenA))
	second := decodeInto[sessionResponse](t, f.call(http.MethodPost, submit1, nil, f.tokenA))
	if first.Subtests[0].Status != "submitted" || first.Subtests[0].Score != nil || first.Result != nil ||
		!second.Subtests[0].SubmittedAt.Equal(*first.Subtests[0].SubmittedAt) {
		t.Errorf("submit then retry = %+v then %+v, want the same settled part and no score yet", first.Subtests[0], second.Subtests[0])
	}
	wantCode(t, f.call(http.MethodPut, answerPath, map[string]any{"seq": 3, "answer": []string{}}, f.tokenA), http.StatusConflict, codeSubtestNotRunning)
	wantCode(t, f.call(http.MethodGet, sessionPath(session.ID, "/subtests/"+f.sub(0)), nil, f.tokenA), http.StatusConflict, codeSubtestNotRunning)
	wantCode(t, f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA), http.StatusConflict, codeSubtestNotRunning)

	f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/start"), nil, f.tokenA)
	done := decodeInto[sessionResponse](t, f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(1)+"/submit"), nil, f.tokenA))
	if done.Status != "completed" || done.Result == nil || done.Result.Score != 50 || done.Result.ScoringVersion != 1 {
		t.Fatalf("completion response = %+v, want completed with the persisted score 50", done)
	}
	if done.Subtests[0].Score == nil || *done.Subtests[0].Score != 100 || *done.Subtests[1].Score != 0 {
		t.Errorf("subtest scores = %v and %v, want 100 and 0", done.Subtests[0].Score, done.Subtests[1].Score)
	}
	result := f.call(http.MethodGet, sessionPath(session.ID, "/result"), nil, f.tokenA)
	if result.Code != http.StatusOK || strings.Contains(result.Body.String(), "earned") || strings.Contains(result.Body.String(), "max_points") {
		t.Errorf("result: status = %d body = %s, want 200 without raw points", result.Code, result.Body.String())
	}
	if got := decodeInto[resultResponse](t, result); got.Score != 50 || got.AttemptNumber != 1 || len(got.Subtests) != 2 {
		t.Errorf("result = %+v", got)
	}

	list := decodeInto[attemptListResponse](t, f.call(http.MethodGet, "/api/v1/my/tryout-sessions", nil, f.tokenA))
	if len(list.Attempts) != 1 || list.Attempts[0].Score == nil || *list.Attempts[0].Score != 50 || list.Attempts[0].Status != "completed" {
		t.Errorf("attempts = %+v, want one completed attempt scoring 50", list.Attempts)
	}
	again := decodeInto[studentTryoutResponse](t, f.call(http.MethodGet, tryoutPath, nil, f.tokenA))
	if again.Current != nil || len(again.Attempts) != 1 {
		t.Errorf("overview after completion = %+v, want no current session and one attempt", again)
	}
	next := f.startSession(t, f.tokenA)
	if next.AttemptNumber != 2 || next.ID == session.ID {
		t.Errorf("start after completion = attempt %d session %s, want a new attempt 2", next.AttemptNumber, next.ID)
	}
}

func TestRuntimeEndpointsDeadlinesGraceAndExtension(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	f.enroll(t, f.secondStudent)
	right := f.q1.Options[0].ID

	a, b := f.startSession(t, f.tokenA), f.startSession(t, f.tokenSecond)
	f.call(http.MethodPost, sessionPath(a.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenA)
	f.call(http.MethodPost, sessionPath(b.ID, "/subtests/"+f.sub(0)+"/start"), nil, f.tokenSecond)
	answerA := sessionPath(a.ID, "/answers/"+f.q1.ID.String())
	answerB := sessionPath(b.ID, "/answers/"+f.q1.ID.String())

	extPath := "/api/v1/admin/tryout-sessions/" + b.ID.String() + "/extensions"
	for name, body := range map[string]map[string]any{
		"zero seconds": {"seconds": 0, "reason": "listrik padam"},
		"blank reason": {"seconds": 60, "reason": "  "},
		"too long":     {"seconds": 14401, "reason": "listrik padam"},
	} {
		if rec := f.call(http.MethodPost, extPath, body, f.tokenOperatorA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("extension with %s: status = %d, want 422", name, rec.Code)
		}
	}
	if rec := f.call(http.MethodPost, "/api/v1/admin/tryout-sessions/"+uuid.NewString()+"/extensions", map[string]any{"seconds": 60, "reason": "x"}, f.tokenOperatorA); rec.Code != http.StatusNotFound {
		t.Errorf("extension on an unknown session: status = %d, want 404", rec.Code)
	}
	ext := decodeInto[extensionResponse](t, f.call(http.MethodPost, extPath, map[string]any{"seconds": 120, "reason": "Listrik padam"}, f.tokenOperatorA))
	if ext.Seconds != 120 || ext.TotalSeconds != 120 || ext.SessionID != b.ID {
		t.Errorf("extension = %+v", ext)
	}
	var audited int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tryout_session_extensions WHERE session_id = $1 AND reason = 'Listrik padam'`, b.ID).Scan(&audited); err != nil || audited != 1 {
		t.Errorf("audit rows = %d (err %v), want 1", audited, err)
	}

	f.clock.Advance(10*time.Minute + 60*time.Second)
	if rec := f.call(http.MethodPut, answerA, map[string]any{"seq": 1, "answer": []string{right}}, f.tokenA); rec.Code != http.StatusConflict {
		t.Fatalf("student A past the deadline: status = %d, want 409", rec.Code)
	} else {
		wantCode(t, rec, http.StatusConflict, codeDeadlinePassed)
	}
	if rec := f.call(http.MethodPut, answerB, map[string]any{"seq": 1, "answer": []string{right}}, f.tokenSecond); rec.Code != http.StatusOK {
		t.Errorf("extended student B: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	settled := decodeInto[sessionResponse](t, f.call(http.MethodGet, sessionPath(a.ID, ""), nil, f.tokenA))
	if settled.Subtests[0].Status != "submitted" {
		t.Errorf("student A's part = %+v, want settled by its deadline", settled.Subtests[0])
	}

	// D10: Admin changes the grace while student B's exam is running, on a tryout that is already locked.
	graceBody := map[string]any{"grace_seconds": 90}
	graced := f.call(http.MethodPut, "/api/v1/tryouts/"+f.tryout.ID.String()+"/grace", graceBody, f.tokenAdminA)
	if graced.Code != http.StatusOK || decodeTryout(t, graced).GraceSeconds != 90 || !decodeTryout(t, graced).Locked {
		t.Fatalf("set grace: status = %d body = %s, want 200 with 90 on a locked tryout", graced.Code, graced.Body.String())
	}
	if rec := f.call(http.MethodPut, "/api/v1/tryouts/"+f.tryout.ID.String()+"/grace", map[string]any{"grace_seconds": 4000}, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("grace of 4000: status = %d, want 422", rec.Code)
	}
	// B's deadline is 10m + 120s after its start; the grace adds 90s. Time is now start + 11m.
	f.clock.Advance(2*time.Minute + 20*time.Second)
	if rec := f.call(http.MethodPut, answerB, map[string]any{"seq": 2, "answer": []string{}}, f.tokenSecond); rec.Code != http.StatusOK {
		t.Errorf("inside the extension and grace: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	f.clock.Advance(20 * time.Second)
	wantCode(t, f.call(http.MethodPut, answerB, map[string]any{"seq": 3, "answer": []string{right}}, f.tokenSecond), http.StatusConflict, codeDeadlinePassed)
}

func TestRuntimeSettingsEndpoint(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	got := decodeInto[settingsBody](t, f.call(http.MethodGet, "/api/v1/admin/settings", nil, f.tokenAdminA))
	if got.ScoringWeights != (weightsBody{Easy: 1, Medium: 2, Hard: 3}) {
		t.Errorf("default weights = %+v, want 1/2/3", got.ScoringWeights)
	}
	saved := f.call(http.MethodPut, "/api/v1/admin/settings", settingsBody{ScoringWeights: weightsBody{Easy: 1.5, Medium: 2, Hard: 4}}, f.tokenAdminA)
	if saved.Code != http.StatusOK {
		t.Fatalf("save: status = %d body = %s", saved.Code, saved.Body.String())
	}
	if got := decodeInto[settingsBody](t, f.call(http.MethodGet, "/api/v1/admin/settings", nil, f.tokenAdminA)); got.ScoringWeights != (weightsBody{Easy: 1.5, Medium: 2, Hard: 4}) {
		t.Errorf("weights after save = %+v", got.ScoringWeights)
	}
	for name, body := range map[string]settingsBody{
		"zero":      {ScoringWeights: weightsBody{Easy: 0, Medium: 2, Hard: 3}},
		"negative":  {ScoringWeights: weightsBody{Easy: 1, Medium: -2, Hard: 3}},
		"huge":      {ScoringWeights: weightsBody{Easy: 1, Medium: 2, Hard: 1001}},
		"3 decimal": {ScoringWeights: weightsBody{Easy: 1.234, Medium: 2, Hard: 3}},
	} {
		if rec := f.call(http.MethodPut, "/api/v1/admin/settings", body, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s weight: status = %d, want 422", name, rec.Code)
		}
	}
}

// D13: an exam request is authenticated activity, so an attempt keeps its login session alive.
func TestRuntimeRequestsCountAsSessionActivity(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	f.enroll(t, f.studentA)
	sid := f.startSession(t, f.tokenA).ID
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE auth_sessions SET last_activity_at = now() - interval '2 hours' WHERE user_id = $1`, f.studentA.ID); err != nil {
		t.Fatalf("age the login session: %v", err)
	}
	if rec := f.call(http.MethodGet, sessionPath(sid, ""), nil, f.tokenA); rec.Code != http.StatusOK {
		t.Fatalf("read session: status = %d", rec.Code)
	}
	var idle time.Duration
	if err := f.pool.QueryRow(context.Background(),
		`SELECT now() - max(last_activity_at) FROM auth_sessions WHERE user_id = $1`, f.studentA.ID).Scan(&idle); err != nil {
		t.Fatalf("read activity: %v", err)
	}
	if idle > time.Minute {
		t.Errorf("login session idle for %v after an exam request, want the request to count as activity", idle)
	}
}
