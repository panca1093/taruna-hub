package handler

import (
	"net/http"
	"testing"
)

func TestStudentDashboardExcludesKecermatan(t *testing.T) {
	f := newKecHTTPFixture(t)
	f.credit(t, f.studentA, 2)
	rec := f.call(http.MethodGet, "/api/v1/my/dashboard", nil, f.tokenA)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: status %d: %s", rec.Code, rec.Body.String())
	}
	summary := decodeInto[dashboardSummaryResponse](t, rec)
	if summary.OwnedTryouts != 0 || summary.CompletedTryouts != 0 || summary.AverageScore != nil {
		t.Fatalf("kecermatan credit must not count as a tryout: %+v", summary)
	}
}

func TestStudentDashboardIdentityRolesAndPersistedResults(t *testing.T) {
	f := newRuntimeHTTPFixture(t)
	const path = "/api/v1/my/dashboard"
	for _, token := range []string{"", f.tokenAdminA, f.tokenOperatorA} {
		want := http.StatusForbidden
		if token == "" {
			want = http.StatusUnauthorized
		}
		if rec := f.call(http.MethodGet, path, nil, token); rec.Code != want {
			t.Fatalf("role guard: status %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	read := func(token, query string) dashboardSummaryResponse {
		t.Helper()
		rec := f.call(http.MethodGet, path+query, nil, token)
		if rec.Code != http.StatusOK {
			t.Fatalf("dashboard: status %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("dashboard must not be cached: %s", rec.Header().Get("Cache-Control"))
		}
		return decodeInto[dashboardSummaryResponse](t, rec)
	}
	empty := read(f.tokenA, "")
	if empty.OwnedTryouts != 0 || empty.CompletedTryouts != 0 || empty.AverageScore != nil || empty.Balance != "0.00" {
		t.Fatalf("new account summary = %+v", empty)
	}
	f.enroll(t, f.studentA)
	if own := read(f.tokenA, ""); own.OwnedTryouts != 1 || own.AverageScore != nil {
		t.Fatalf("owned, unstarted tryout summary = %+v", own)
	}
	if other := read(f.tokenSecond, "?user_id="+f.studentA.ID.String()); other.OwnedTryouts != 0 {
		t.Fatalf("client user ID changed authenticated scope: %+v", other)
	}
	session := f.startSession(t, f.tokenA)
	for i := range 2 {
		for _, action := range []string{"start", "submit"} {
			if rec := f.call(http.MethodPost, sessionPath(session.ID, "/subtests/"+f.sub(i)+"/"+action), nil, f.tokenA); rec.Code != http.StatusOK {
				t.Fatalf("complete tryout: status %d: %s", rec.Code, rec.Body.String())
			}
		}
	}
	completed := read(f.tokenA, "")
	if completed.CompletedTryouts != 1 || completed.AverageScore == nil || *completed.AverageScore != 0 {
		t.Fatalf("persisted zero result summary = %+v", completed)
	}
}
