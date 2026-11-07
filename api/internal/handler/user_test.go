package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type userOpsFixture struct {
	kecHTTPFixture
	ops http.Handler
}

func newUserOpsFixture(t *testing.T) userOpsFixture {
	t.Helper()
	f := userOpsFixture{kecHTTPFixture: newKecHTTPFixture(t)}
	assets := newAssetService(f.tenantFixture, linkStore{}, time.Minute)
	admin := service.NewUserAdminService(f.pool, repository.NewUserRepository(f.pool), repository.NewSessionRepository(f.pool),
		newOrderService(f.tenantFixture, assets), service.WithUserAdminCost(bcrypt.MinCost))
	reports := service.NewReportService(repository.NewReportRepository(f.pool))
	auth := newAuthService(f.tenantFixture)
	f.ops = f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(f.logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(f.logger))
		New(f.logger, NewHealthHandler(f.pool), auth, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, AuthOptions{AllowedOrigin: httpx.OriginPolicy(devOrigins, true)}).
			WithStudentOps(admin, reports).Register(e, ResolveHostScope(f.tenants, slug))
		return e
	})
	// Registered after the fixture cleanups, so audit rows go before the users they name.
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenantA.ID, f.tenantB.ID}
		for _, sql := range []string{
			`DELETE FROM audit_events WHERE tenant_id = ANY($1)`,
			`DELETE FROM order_items WHERE tenant_id = ANY($1)`,
			`DELETE FROM product_tryouts WHERE tenant_id = ANY($1)`,
		} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup user ops fixture: %v", err)
			}
		}
	})
	return f
}

func (f userOpsFixture) call(method, path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.ops, method, f.hostA, path, body, token)
}

func decodeAs[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %s does not decode: %v", rec.Body.String(), err)
	}
	return out
}

func expect(t *testing.T, what string, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s: status = %d, want %d; body = %s", what, rec.Code, want, rec.Body.String())
	}
}

func (f userOpsFixture) audit(t *testing.T, tenantID, target uuid.UUID) []model.AuditEvent {
	t.Helper()
	events, err := repository.NewUserRepository(f.pool).Audit(context.Background(), tenantID, target)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	return events
}

func (f userOpsFixture) firstProgram(t *testing.T) programResponse {
	t.Helper()
	rec := getWithToken(f.e, f.hostA, "/api/v1/programs", f.tokenAdminA)
	expect(t, "list programs", rec, http.StatusOK)
	programs := decodeAs[programsResponse](t, rec).Programs
	if len(programs) == 0 {
		t.Fatal("the kecermatan fixture left no program")
	}
	return programs[0]
}

func (f userOpsFixture) publishedTryoutID(t *testing.T) uuid.UUID {
	t.Helper()
	return publishedTryout(t, f.e, f.hostA, f.tokenAdminA, f.firstProgram(t)).ID
}

func TestStudentOpsRoleMatrixOverHTTP(t *testing.T) {
	f := newUserOpsFixture(t)
	studentID, operatorID := f.studentA.ID.String(), f.operatorA.ID.String()
	grant := map[string]any{"tryout_id": uuid.NewString(), "reason": "x", "idempotency_key": "matrix-key-0001"}

	type route struct {
		method, path string
		body         any
	}
	studentRoutes := []route{
		{http.MethodGet, "/api/v1/admin/users?role=student", nil},
		{http.MethodGet, "/api/v1/admin/users/" + studentID, nil},
		{http.MethodGet, "/api/v1/students/" + studentID + "/enrollments", nil},
		{http.MethodPost, "/api/v1/students/" + studentID + "/grants", grant},
	}
	allRoutes := append([]route{
		{http.MethodPost, "/api/v1/admin/users", map[string]any{}},
		{http.MethodPut, "/api/v1/admin/users/" + studentID, map[string]any{}},
		{http.MethodPost, "/api/v1/admin/users/" + studentID + "/deactivate", map[string]any{}},
		{http.MethodPost, "/api/v1/admin/users/" + studentID + "/reactivate", map[string]any{}},
		{http.MethodGet, "/api/v1/admin/reports/revenue", nil},
	}, studentRoutes...)
	for _, r := range allRoutes {
		if rec := f.call(r.method, r.path, r.body, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: status = %d, want 401", r.method, r.path, rec.Code)
		}
		if rec := f.call(r.method, r.path, r.body, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student %s %s: status = %d, want 403", r.method, r.path, rec.Code)
		}
	}

	// The Operator reaches student operations (D02) and nothing about staff accounts or the recap.
	expect(t, "operator student list", f.call(http.MethodGet, "/api/v1/admin/users?role=student", nil, f.tokenOperatorA), http.StatusOK)
	list := decodeAs[adminUserListResponse](t, f.call(http.MethodGet, "/api/v1/admin/users", nil, f.tokenOperatorA))
	for _, u := range list.Users {
		if u.Role != model.RoleStudent {
			t.Errorf("operator default list holds a %s account", u.Role)
		}
	}
	for _, path := range []string{"/api/v1/admin/users?role=operator", "/api/v1/admin/users?role=admin", "/api/v1/admin/users?role=staff"} {
		expect(t, "operator "+path, f.call(http.MethodGet, path, nil, f.tokenOperatorA), http.StatusForbidden)
	}
	expect(t, "admin staff list", f.call(http.MethodGet, "/api/v1/admin/users?role=staff", nil, f.tokenAdminA), http.StatusOK)
	expect(t, "operator reads a student", f.call(http.MethodGet, "/api/v1/admin/users/"+studentID, nil, f.tokenOperatorA), http.StatusOK)
	for _, r := range []route{
		{http.MethodGet, "/api/v1/admin/users/" + operatorID, nil},
		{http.MethodPut, "/api/v1/admin/users/" + operatorID, map[string]any{"name": "Baru", "email": "baru-op@example.com"}},
		{http.MethodPost, "/api/v1/admin/users/" + operatorID + "/deactivate", map[string]any{"reason": "uji"}},
		{http.MethodPost, "/api/v1/admin/users/" + f.adminA.ID.String() + "/deactivate", map[string]any{"reason": "uji"}},
	} {
		expect(t, "operator "+r.method+" "+r.path, f.call(r.method, r.path, r.body, f.tokenOperatorA), http.StatusForbidden)
	}
	for _, role := range []string{"operator", "admin"} {
		expect(t, "operator creates "+role, f.call(http.MethodPost, "/api/v1/admin/users", map[string]any{
			"name": "Staf", "email": uuid.NewString()[:8] + "@example.com", "password": "password-123", "role": role,
		}, f.tokenOperatorA), http.StatusForbidden)
		expect(t, "admin creates "+role, f.call(http.MethodPost, "/api/v1/admin/users", map[string]any{
			"name": "Staf", "email": uuid.NewString()[:8] + "@example.com", "password": "password-123", "role": role,
		}, f.tokenAdminA), http.StatusCreated)
	}
	expect(t, "operator recap", f.call(http.MethodGet, "/api/v1/admin/reports/revenue", nil, f.tokenOperatorA), http.StatusForbidden)
	expect(t, "admin recap", f.call(http.MethodGet, "/api/v1/admin/reports/revenue", nil, f.tokenAdminA), http.StatusOK)

	// Another tenant's student, or a staff id on a student route, is simply absent.
	foreign := f.studentB.ID.String()
	for _, r := range []route{
		{http.MethodGet, "/api/v1/admin/users/" + foreign, nil},
		{http.MethodPut, "/api/v1/admin/users/" + foreign, map[string]any{"name": "Baru", "email": "baru-x@example.com"}},
		{http.MethodPost, "/api/v1/admin/users/" + foreign + "/deactivate", map[string]any{"reason": "uji"}},
		{http.MethodPost, "/api/v1/admin/users/" + foreign + "/reactivate", map[string]any{"reason": "uji"}},
		{http.MethodGet, "/api/v1/students/" + foreign + "/enrollments", nil},
		{http.MethodPost, "/api/v1/students/" + foreign + "/grants", grant},
		{http.MethodGet, "/api/v1/students/" + operatorID + "/enrollments", nil},
		{http.MethodGet, "/api/v1/admin/users/not-a-uuid", nil},
	} {
		for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
			expect(t, name+" "+r.method+" "+r.path, f.call(r.method, r.path, r.body, token), http.StatusNotFound)
		}
	}
	if rec := f.call(http.MethodGet, "/api/v1/admin/users/"+studentID, nil, f.tokenAdminA); rec.Header().Get("Cache-Control") == "" {
		t.Error("account responses must carry Cache-Control")
	}
}

func TestAccountCreateEditAndAudit(t *testing.T) {
	f := newUserOpsFixture(t)
	email := "siswa-" + uuid.NewString()[:8] + "@example.com"
	rec := f.call(http.MethodPost, "/api/v1/admin/users", map[string]any{
		"name": "  Siswa Baru ", "email": email, "phone": "0812-3456-7890", "password": "password-123", "role": "student",
	}, f.tokenOperatorA)
	expect(t, "operator creates a student", rec, http.StatusCreated)
	created := decodeAs[adminUserResponse](t, rec)
	if created.Name != "Siswa Baru" || created.Role != model.RoleStudent || !created.IsActive || created.Phone == nil || *created.Phone != "+6281234567890" {
		t.Fatalf("created = %+v", created)
	}
	login := postJSON(f.ops, f.hostA, "/api/v1/auth/login", map[string]any{"identifier": email, "password": "password-123"}, nil)
	expect(t, "created student signs in", login, http.StatusOK)

	for name, body := range map[string]map[string]any{
		"duplicate email": {"name": "Lain", "email": email, "password": "password-123", "role": "student"},
		"duplicate phone": {"name": "Lain", "email": "lain-" + uuid.NewString()[:6] + "@example.com", "phone": "081234567890", "password": "password-123", "role": "student"},
	} {
		expect(t, name, f.call(http.MethodPost, "/api/v1/admin/users", body, f.tokenAdminA), http.StatusConflict)
	}
	for name, body := range map[string]map[string]any{
		"bad email":      {"name": "A", "email": "bukan-email", "password": "password-123", "role": "student"},
		"short password": {"name": "A", "email": "a-" + uuid.NewString()[:6] + "@example.com", "password": "short", "role": "student"},
		"bad role":       {"name": "A", "email": "a-" + uuid.NewString()[:6] + "@example.com", "password": "password-123", "role": "platform_admin"},
		"blank name":     {"name": " ", "email": "a-" + uuid.NewString()[:6] + "@example.com", "password": "password-123", "role": "student"},
		"bad phone":      {"name": "A", "email": "a-" + uuid.NewString()[:6] + "@example.com", "phone": "12", "password": "password-123", "role": "student"},
	} {
		expect(t, name, f.call(http.MethodPost, "/api/v1/admin/users", body, f.tokenAdminA), http.StatusUnprocessableEntity)
	}

	path := "/api/v1/admin/users/" + created.ID.String()
	other := "ganti-" + uuid.NewString()[:8] + "@example.com"
	expect(t, "edit to a taken email", f.call(http.MethodPut, path, map[string]any{"name": "X", "email": f.studentA.Email}, f.tokenAdminA), http.StatusConflict)
	rec = f.call(http.MethodPut, path, map[string]any{"name": "Nama Baru", "email": other, "phone": ""}, f.tokenOperatorA)
	expect(t, "operator edits a student", rec, http.StatusOK)
	edited := decodeAs[adminUserResponse](t, rec)
	if edited.Name != "Nama Baru" || edited.Email == nil || *edited.Email != other || edited.Phone != nil || edited.EmailVerified {
		t.Fatalf("edited = %+v, want the new name and email, no phone, unverified", edited)
	}
	expect(t, "unchanged edit", f.call(http.MethodPut, path, map[string]any{"name": "Nama Baru", "email": other}, f.tokenAdminA), http.StatusOK)

	expect(t, "deactivate without reason", f.call(http.MethodPost, path+"/deactivate", map[string]any{"reason": " "}, f.tokenAdminA), http.StatusUnprocessableEntity)
	expect(t, "admin deactivates itself", f.call(http.MethodPost, "/api/v1/admin/users/"+f.adminA.ID.String()+"/deactivate", map[string]any{"reason": "uji"}, f.tokenAdminA), http.StatusForbidden)

	events := f.audit(t, f.tenantA.ID, created.ID)
	if len(events) != 2 || events[0].Action != model.AuditUserUpdate || events[1].Action != model.AuditUserCreate {
		t.Fatalf("audit = %+v, want one update (the no-op edit adds none) after the create", events)
	}
	if events[0].ActorID != f.operatorA.ID || events[0].ActorRole != model.RoleOperator || events[0].Detail["fields"] != "name,email,phone" {
		t.Errorf("update audit = %+v, want the operator and the changed fields", events[0])
	}
	if events[1].ActorID != f.operatorA.ID || events[1].Detail["role"] != "student" || events[1].TargetType != model.AuditTargetUser {
		t.Errorf("create audit = %+v", events[1])
	}
}

func TestDeactivationBlocksLoginAndKeepsHistory(t *testing.T) {
	f := newUserOpsFixture(t)
	studentPath := "/api/v1/admin/users/" + f.secondStudent.ID.String()

	// The student buys with the wallet, so an order, a ledger row and an enrollment exist before deactivation.
	tryoutID := f.publishedTryoutID(t)
	productID := createProduct(t, f.e, f.hostA, f.tokenAdminA,
		productBody(f.firstProgram(t), []string{tryoutID.String()}, map[string]any{"price": "100000.00"})).ID
	expect(t, "fund", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/students/"+f.secondStudent.ID.String()+"/wallet/adjustments",
		adjustmentBody("credit", "250000.00", "Saldo uji", "fund-"+uuid.NewString()[:8]), f.tokenAdminA), http.StatusCreated)
	expect(t, "wallet order", sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/orders",
		checkoutBody(productID.String(), "wallet", nil), f.tokenSecond), http.StatusCreated)

	email := *f.secondStudent.Email
	login := postJSON(f.ops, f.hostA, "/api/v1/auth/login", map[string]any{"identifier": email, "password": authTestPassword}, nil)
	expect(t, "login before", login, http.StatusOK)
	refresh := cookieNamed(login.Result().Cookies(), refreshCookieName)
	if refresh == nil {
		t.Fatal("login set no refresh cookie")
	}

	rec := f.call(http.MethodPost, studentPath+"/deactivate", map[string]any{"reason": "Pindah sekolah"}, f.tokenOperatorA)
	expect(t, "operator deactivates a student", rec, http.StatusOK)
	if decodeAs[adminUserResponse](t, rec).IsActive {
		t.Fatal("deactivated account still reports is_active")
	}

	expect(t, "login after", postJSON(f.ops, f.hostA, "/api/v1/auth/login", map[string]any{"identifier": email, "password": authTestPassword}, nil), http.StatusUnauthorized)
	expect(t, "refresh after", postWithCookie(f.ops, f.hostA, "/api/v1/auth/refresh", refresh, devOrigins[0]), http.StatusUnauthorized)
	expect(t, "old access token", getWithToken(f.ops, f.hostA, "/api/v1/me", f.tokenSecond), http.StatusUnauthorized)
	var live int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NULL`, f.secondStudent.ID).Scan(&live); err != nil || live != 0 {
		t.Fatalf("live sessions = %d (err %v), want none", live, err)
	}

	// Every link survives: the account is still listed, and its order, ledger and access still resolve.
	list := decodeAs[adminUserListResponse](t, f.call(http.MethodGet, "/api/v1/admin/users?role=student&active=false&q="+email, nil, f.tokenAdminA))
	if list.Total != 1 || list.Users[0].ID != f.secondStudent.ID || list.Users[0].IsActive || list.Users[0].Balance != "150000.00" {
		t.Fatalf("inactive list = %+v, want the deactivated student with its balance", list)
	}
	active := decodeAs[adminUserListResponse](t, f.call(http.MethodGet, "/api/v1/admin/users?role=student&active=true&q="+email, nil, f.tokenAdminA))
	if active.Total != 0 {
		t.Fatalf("active filter still returns the deactivated student: %+v", active)
	}
	orders := decodeOrders(t, getWithToken(f.e, f.hostA, "/api/v1/admin/orders?q="+url.QueryEscape(f.secondStudent.Name), f.tokenAdminA))
	found := false
	for _, o := range orders.Orders {
		found = found || (o.StudentID == f.secondStudent.ID && o.Status == "confirmed")
	}
	if !found {
		t.Fatalf("the deactivated student's confirmed order no longer resolves: %+v", orders)
	}
	entries := decodeWalletEntries(t, getWithToken(f.e, f.hostA, "/api/v1/students/"+f.secondStudent.ID.String()+"/wallet/entries", f.tokenAdminA))
	if entries.Total < 2 {
		t.Fatalf("ledger entries = %d, want the top-up and the spend", entries.Total)
	}
	access := decodeAs[studentAccessListResponse](t, f.call(http.MethodGet, "/api/v1/students/"+f.secondStudent.ID.String()+"/enrollments", nil, f.tokenAdminA))
	if access.Total != 1 || access.Access[0].SourceType != "order" {
		t.Fatalf("access = %+v, want the purchased tryout", access)
	}

	// Repeating the action changes nothing and audits nothing more.
	expect(t, "repeat deactivate", f.call(http.MethodPost, studentPath+"/deactivate", map[string]any{"reason": "lagi"}, f.tokenAdminA), http.StatusOK)

	expect(t, "reactivate", f.call(http.MethodPost, studentPath+"/reactivate", map[string]any{"reason": "Kembali aktif"}, f.tokenAdminA), http.StatusOK)
	expect(t, "login restored", postJSON(f.ops, f.hostA, "/api/v1/auth/login", map[string]any{"identifier": email, "password": authTestPassword}, nil), http.StatusOK)

	events := f.audit(t, f.tenantA.ID, f.secondStudent.ID)
	if len(events) != 2 || events[0].Action != model.AuditUserReactivate || events[1].Action != model.AuditUserDeactivate {
		t.Fatalf("audit = %+v, want deactivate then reactivate", events)
	}
	if events[1].ActorID != f.operatorA.ID || events[1].ActorRole != model.RoleOperator || events[1].Reason != "Pindah sekolah" {
		t.Errorf("deactivate audit = %+v, want the operator and the reason", events[1])
	}
	if events[0].ActorID != f.adminA.ID || events[0].ActorRole != model.RoleAdmin || events[0].Reason != "Kembali aktif" {
		t.Errorf("reactivate audit = %+v, want the admin and the reason", events[0])
	}
}

func TestManualGrantOpensOnlyTheTarget(t *testing.T) {
	f := newUserOpsFixture(t)
	target, other := f.publishedTryoutID(t), f.publishedTryoutID(t)
	student := f.secondStudent.ID.String()
	path := "/api/v1/students/" + student + "/grants"
	key := "grant-" + uuid.NewString()[:12]
	body := map[string]any{"tryout_id": target.String(), "reason": "Kompensasi gangguan sistem", "idempotency_key": key}

	rec := f.call(http.MethodPost, path, body, f.tokenOperatorA)
	expect(t, "first grant", rec, http.StatusCreated)
	first := decodeAs[studentAccessResponse](t, rec)
	if first.Kind != "tryout" || first.TestID != target || !first.Active || first.SourceType != "staff" {
		t.Fatalf("grant = %+v, want an active staff-sourced tryout", first)
	}

	var rows int
	var actor uuid.UUID
	var reason, actorKind string
	var orderID *uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `
		SELECT count(*), min(actor_id::text)::uuid, min(reason), min(actor_kind), (array_agg(order_id))[1]
		  FROM enrollment_grants WHERE tenant_id = $1 AND source_type = 'staff' AND source_id = $2`,
		f.tenantA.ID, key).Scan(&rows, &actor, &reason, &actorKind, &orderID); err != nil {
		t.Fatalf("read grant: %v", err)
	}
	if rows != 1 || actor != f.operatorA.ID || reason != "Kompensasi gangguan sistem" || actorKind != "staff" || orderID != nil {
		t.Fatalf("grant row = %d %s %q %s %v, want one orderless staff grant by the operator", rows, actor, reason, actorKind, orderID)
	}

	access := decodeAs[studentAccessListResponse](t, f.call(http.MethodGet, "/api/v1/students/"+student+"/enrollments", nil, f.tokenAdminA))
	if access.Total != 1 || access.Access[0].TestID != target || access.Access[0].EnrollmentID != first.EnrollmentID {
		t.Fatalf("access = %+v, want exactly the target tryout", access)
	}
	mine := decodeEnrollments(t, getWithToken(f.e, f.hostA, "/api/v1/enrollments", f.tokenSecond))
	if len(mine.Enrollments) != 1 || mine.Enrollments[0].TryoutID != target {
		t.Fatalf("student's own access = %+v, want only the target", mine)
	}
	for _, e := range mine.Enrollments {
		if e.TryoutID == other {
			t.Fatal("a grant opened a tryout it did not name")
		}
	}

	// Same key and body replays; nothing is written twice.
	rec = f.call(http.MethodPost, path, body, f.tokenAdminA)
	expect(t, "replay", rec, http.StatusOK)
	if decodeAs[studentAccessResponse](t, rec).EnrollmentID != first.EnrollmentID {
		t.Error("replay answered another enrollment")
	}
	var grants, audits int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM enrollment_grants WHERE tenant_id = $1 AND source_id = $2`, f.tenantA.ID, key).Scan(&grants); err != nil || grants != 1 {
		t.Errorf("grants after replay = %d (err %v), want 1", grants, err)
	}
	events := f.audit(t, f.tenantA.ID, first.EnrollmentID)
	audits = len(events)
	if audits != 1 || events[0].Action != model.AuditGrantManual || events[0].ActorID != f.operatorA.ID ||
		events[0].Reason != "Kompensasi gangguan sistem" || events[0].TargetType != model.AuditTargetEnrollment {
		t.Fatalf("grant audit = %+v, want one manual-grant event by the operator", events)
	}

	conflict := map[string]any{"tryout_id": other.String(), "reason": "Kompensasi gangguan sistem", "idempotency_key": key}
	expect(t, "same key, other tryout", f.call(http.MethodPost, path, conflict, f.tokenAdminA), http.StatusConflict)
	expect(t, "same key, other student", f.call(http.MethodPost, "/api/v1/students/"+f.studentA.ID.String()+"/grants", body, f.tokenAdminA), http.StatusConflict)

	held := map[string]any{"tryout_id": target.String(), "reason": "lagi", "idempotency_key": "held-" + uuid.NewString()[:10]}
	rec = f.call(http.MethodPost, path, held, f.tokenAdminA)
	expect(t, "already held", rec, http.StatusConflict)
	if code := decodeAs[map[string]map[string]any](t, rec)["error"]["code"]; code != "already_owned" {
		t.Errorf("held code = %v, want already_owned", code)
	}

	for name, in := range map[string]map[string]any{
		"no reason":      {"tryout_id": other.String(), "reason": " ", "idempotency_key": "k-" + uuid.NewString()[:10]},
		"bad key":        {"tryout_id": other.String(), "reason": "x", "idempotency_key": "short"},
		"no target":      {"reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]},
		"both targets":   {"tryout_id": other.String(), "kecermatan_test_id": f.test.ID.String(), "attempts": 2, "reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]},
		"attempts on it": {"tryout_id": other.String(), "attempts": 2, "reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]},
		"no attempts":    {"kecermatan_test_id": f.test.ID.String(), "reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]},
	} {
		expect(t, name, f.call(http.MethodPost, path, in, f.tokenAdminA), http.StatusUnprocessableEntity)
	}
	for name, id := range map[string]string{"unknown": uuid.NewString()} {
		expect(t, name+" tryout", f.call(http.MethodPost, path,
			map[string]any{"tryout_id": id, "reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]}, f.tokenAdminA), http.StatusNotFound)
	}
	var draft uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `UPDATE tryouts SET status = 'draft' WHERE id = $1 RETURNING id`, other).Scan(&draft); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	expect(t, "draft tryout", f.call(http.MethodPost, path,
		map[string]any{"tryout_id": other.String(), "reason": "x", "idempotency_key": "k-" + uuid.NewString()[:10]}, f.tokenAdminA), http.StatusNotFound)
}

func TestManualGrantAddsKecermatanCredits(t *testing.T) {
	f := newUserOpsFixture(t)
	student := f.secondStudent.ID.String()
	path := "/api/v1/students/" + student + "/grants"
	grant := func(attempts int, key string) *httptest.ResponseRecorder {
		return f.call(http.MethodPost, path, map[string]any{
			"kecermatan_test_id": f.test.ID.String(), "attempts": attempts, "reason": "Paket sekolah", "idempotency_key": key,
		}, f.tokenOperatorA)
	}
	k1, k2 := "kec-"+uuid.NewString()[:10], "kec-"+uuid.NewString()[:10]
	rec := grant(3, k1)
	expect(t, "first credit", rec, http.StatusCreated)
	got := decodeAs[studentAccessResponse](t, rec)
	if got.Kind != "kecermatan" || got.TestID != f.test.ID || got.AttemptsPurchased == nil || *got.AttemptsPurchased != 3 || got.AttemptsUsed != 0 || !got.Active {
		t.Fatalf("first credit = %+v, want 3 purchased and active", got)
	}
	expect(t, "replay", grant(3, k1), http.StatusOK)
	expect(t, "same key other credits", grant(5, k1), http.StatusConflict)
	expect(t, "second source adds", grant(2, k2), http.StatusCreated)

	access := decodeAs[studentAccessListResponse](t, f.call(http.MethodGet, "/api/v1/students/"+student+"/enrollments", nil, f.tokenAdminA))
	if access.Total != 1 || *access.Access[0].AttemptsPurchased != 5 {
		t.Fatalf("access = %+v, want one enrollment holding 5 credits", access)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE enrollments SET attempts_used = 5 WHERE id = $1`, access.Access[0].EnrollmentID); err != nil {
		t.Fatalf("use credits: %v", err)
	}
	access = decodeAs[studentAccessListResponse](t, f.call(http.MethodGet, "/api/v1/students/"+student+"/enrollments", nil, f.tokenAdminA))
	if access.Access[0].Active || access.Access[0].AttemptsUsed != 5 {
		t.Fatalf("spent credits = %+v, want used 5 and inactive", access.Access[0])
	}
	events := f.audit(t, f.tenantA.ID, access.Access[0].EnrollmentID)
	if len(events) != 2 || events[0].Detail["attempts"] != "2" || events[1].Detail["attempts"] != "3" {
		t.Fatalf("credit audit = %+v, want one event per source", events)
	}
}

func TestStudentAccessWindowFollowsTheTryout(t *testing.T) {
	f := newUserOpsFixture(t)
	target := f.publishedTryoutID(t)
	student := f.secondStudent.ID.String()
	expect(t, "grant", f.call(http.MethodPost, "/api/v1/students/"+student+"/grants",
		map[string]any{"tryout_id": target.String(), "reason": "uji", "idempotency_key": "win-" + uuid.NewString()[:10]}, f.tokenAdminA), http.StatusCreated)

	read := func() studentAccessResponse {
		return decodeAs[studentAccessListResponse](t, f.call(http.MethodGet, "/api/v1/students/"+student+"/enrollments", nil, f.tokenAdminA)).Access[0]
	}
	if a := read(); !a.Active || a.ActiveUntil == nil {
		t.Fatalf("open window = %+v, want active with an end", a)
	}
	for name, sql := range map[string]string{
		"ended":  `UPDATE tryouts SET active_from = now() - interval '2 days', active_until = now() - interval '1 day' WHERE id = $1`,
		"future": `UPDATE tryouts SET active_from = now() + interval '1 day', active_until = now() + interval '2 days' WHERE id = $1`,
		"unpub":  `UPDATE tryouts SET active_from = NULL, active_until = now() + interval '2 days', status = 'draft' WHERE id = $1`,
	} {
		if _, err := f.pool.Exec(context.Background(), sql, target); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if a := read(); a.Active {
			t.Errorf("%s window reports active: %+v", name, a)
		}
	}
}

func TestConcurrentManualGrantsWithOneKeyGrantOnce(t *testing.T) {
	f := newUserOpsFixture(t)
	target := f.publishedTryoutID(t)
	key := "race-" + uuid.NewString()[:10]
	body := map[string]any{"tryout_id": target.String(), "reason": "uji paralel", "idempotency_key": key}
	codes := make(chan int, 6)
	for i := 0; i < cap(codes); i++ {
		go func() {
			codes <- f.call(http.MethodPost, "/api/v1/students/"+f.secondStudent.ID.String()+"/grants", body, f.tokenAdminA).Code
		}()
	}
	created, replayed := 0, 0
	for i := 0; i < cap(codes); i++ {
		switch code := <-codes; code {
		case http.StatusCreated:
			created++
		case http.StatusOK:
			replayed++
		default:
			t.Errorf("concurrent grant: status = %d", code)
		}
	}
	if created != 1 || replayed != 5 {
		t.Errorf("created %d, replayed %d, want 1 and 5", created, replayed)
	}
	var grants int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM enrollment_grants WHERE tenant_id = $1 AND source_id = $2`, f.tenantA.ID, key).Scan(&grants); err != nil || grants != 1 {
		t.Errorf("grants = %d (err %v), want 1", grants, err)
	}
}
