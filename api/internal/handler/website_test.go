package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type websiteHTTPFixture struct {
	walletHTTPFixture
	e http.Handler
}

func newWebsiteHTTPFixture(t *testing.T) websiteHTTPFixture {
	t.Helper()
	f := websiteHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t)}
	web := service.NewWebsiteService(f.pool, repository.NewWebsiteRepository(f.pool))
	auth := newAuthService(f.tenantFixture)
	f.e = f.deployments(func(slug string) *echo.Echo {
		e := echo.New()
		e.HideBanner = true
		e.HTTPErrorHandler = httpx.ErrorHandler(f.logger)
		e.Use(httpx.RequestID())
		e.Use(httpx.Recover(f.logger))
		New(f.logger, NewHealthHandler(f.pool), auth, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, AuthOptions{}).
			WithWebsite(web).Register(e, ResolveHostScope(f.tenants, slug))
		return e
	})
	// Registered after the fixture cleanups, so the website rows go before the users and tenants they reference.
	t.Cleanup(func() {
		ids := []uuid.UUID{f.tenantA.ID, f.tenantB.ID}
		for _, sql := range []string{`DELETE FROM website_publications WHERE tenant_id = ANY($1)`, `DELETE FROM website_content WHERE tenant_id = ANY($1)`} {
			if _, err := f.pool.Exec(context.Background(), sql, ids); err != nil {
				t.Errorf("cleanup website fixture: %v", err)
			}
		}
	})
	return f
}

func (f websiteHTTPFixture) call(method, path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, method, f.hostA, path, body, token)
}

func (f websiteHTTPFixture) raw(method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Host = f.hostA
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(f.e, req)
}

func (f websiteHTTPFixture) workbench(t *testing.T) map[string]any {
	t.Helper()
	rec := f.call(http.MethodGet, "/api/v1/admin/website", nil, f.tokenAdminA)
	if rec.Code != http.StatusOK {
		t.Fatalf("workbench: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeMap(t, rec)
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %s is not an object: %v", rec.Body.String(), err)
	}
	return m
}

// contentWith edits the draft the workbench returned, so the request carries exactly the contract's shape.
func contentWith(draft map[string]any, edit func(id map[string]any)) map[string]any {
	buf, _ := json.Marshal(draft)
	var copyOf map[string]any
	_ = json.Unmarshal(buf, &copyOf)
	edit(copyOf["locales"].(map[string]any)["id"].(map[string]any))
	return copyOf
}

func (f websiteHTTPFixture) save(t *testing.T, expected int, headline string) *httptest.ResponseRecorder {
	t.Helper()
	draft := f.workbench(t)["draft"].(map[string]any)
	content := contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["hero_title"] = headline })
	return f.call(http.MethodPut, "/api/v1/admin/website", map[string]any{"expected_draft_revision": expected, "content": content}, f.tokenAdminA)
}

func TestWebsitePermissions(t *testing.T) {
	f := newWebsiteHTTPFixture(t)
	draft := f.workbench(t)["draft"]

	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/admin/website", nil},
		{http.MethodPut, "/api/v1/admin/website", map[string]any{"expected_draft_revision": 1, "content": draft}},
		{http.MethodPost, "/api/v1/admin/website/publish", map[string]any{"expected_draft_revision": 1, "expected_published_revision": 0}},
		{http.MethodGet, "/api/v1/admin/website/history", nil},
	}
	denied := map[string]struct {
		token string
		want  int
	}{
		"anonymous":  {"", http.StatusUnauthorized},
		"student":    {f.tokenA, http.StatusForbidden},
		"operator":   {f.tokenOperatorA, http.StatusForbidden},
		"other-site": {f.tokenAdmB, http.StatusUnauthorized},
	}
	for _, r := range routes {
		for name, d := range denied {
			rec := f.call(r.method, r.path, r.body, d.token)
			if rec.Code != d.want {
				t.Errorf("%s %s as %s: status = %d, want %d", r.method, r.path, name, rec.Code, d.want)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("%s %s as %s: Cache-Control = %q, want no-store", r.method, r.path, name, got)
			}
		}
		if rec := f.call(r.method, r.path, r.body, f.tokenAdminA); rec.Code != http.StatusOK && rec.Code != http.StatusConflict {
			t.Errorf("%s %s as admin: status = %d body = %s", r.method, r.path, rec.Code, rec.Body.String())
		}
	}

	var rows int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM website_content WHERE tenant_id = $1`, f.tenantB.ID).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("a denied request created %d rows for the other tenant (err %v)", rows, err)
	}
}

func TestWebsitePublicIsolation(t *testing.T) {
	f := newWebsiteHTTPFixture(t)
	public := func(token string) *httptest.ResponseRecorder {
		return f.call(http.MethodGet, "/api/v1/website?cms=draft&preview=1", nil, token)
	}

	rec := public("")
	if rec.Code != http.StatusNotFound || errorBody(t, rec).Code != "website_not_published" {
		t.Fatalf("before the first publication: %d %s, want 404 website_not_published", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("404 Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	if rec := f.save(t, 1, "RAHASIA DRAF <script>alert(1)</script>"); rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	for _, token := range []string{"", f.tokenAdminA} {
		if rec := public(token); rec.Code != http.StatusNotFound {
			t.Fatalf("a saved draft is public: %d %s", rec.Code, rec.Body.String())
		}
	}

	pub := f.call(http.MethodPost, "/api/v1/admin/website/publish", map[string]any{"expected_draft_revision": 2, "expected_published_revision": 0}, f.tokenAdminA)
	if pub.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", pub.Code, pub.Body.String())
	}

	if rec := f.save(t, 2, "DRAF BERIKUTNYA"); rec.Code != http.StatusOK {
		t.Fatalf("second save: %d %s", rec.Code, rec.Body.String())
	}
	for _, token := range []string{"", f.tokenAdminA} {
		rec := public(token)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("published read: %d, Cache-Control %q, body %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
		}
		body := decodeMap(t, rec)
		if len(body) != 3 || body["content"] == nil || body["published_revision"] != float64(1) || body["published_at"] == nil {
			t.Errorf("public keys = %v, want exactly content, published_revision, published_at", body)
		}
		for _, leak := range []string{"DRAF BERIKUTNYA", "updated_by", "published_by", "draft", "history", f.adminA.ID.String()} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Errorf("the public body leaks %q: %s", leak, rec.Body.String())
			}
		}
		title := body["content"].(map[string]any)["locales"].(map[string]any)["id"].(map[string]any)["home"].(map[string]any)["hero_title"]
		if title != "RAHASIA DRAF <script>alert(1)</script>" {
			t.Errorf("published title = %q, want the stored text exactly as saved", title)
		}
	}

	hist := f.call(http.MethodGet, "/api/v1/admin/website/history", nil, f.tokenAdminA)
	var list struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(hist.Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.Items[0]["published_by"] != f.adminA.ID.String() {
		t.Errorf("history = %s (%v), want one record by the admin", hist.Body.String(), err)
	}
}

func TestWebsiteStrictBody(t *testing.T) {
	f := newWebsiteHTTPFixture(t)
	draft := f.workbench(t)["draft"].(map[string]any)
	put := func(body any) *httptest.ResponseRecorder {
		return f.call(http.MethodPut, "/api/v1/admin/website", body, f.tokenAdminA)
	}
	req := func(content any) map[string]any {
		return map[string]any{"expected_draft_revision": 1, "content": content}
	}
	expect := func(name string, rec *httptest.ResponseRecorder, status int, code, field string) {
		t.Helper()
		if rec.Code != status {
			t.Errorf("%s: status = %d, want %d (%s)", name, rec.Code, status, rec.Body.String())
			return
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control = %q", name, rec.Header().Get("Cache-Control"))
		}
		body := errorBody(t, rec)
		if string(body.Code) != code {
			t.Errorf("%s: code = %q, want %q", name, body.Code, code)
		}
		if field != "" && body.Fields[field] == "" {
			t.Errorf("%s: fields = %v, want %q", name, body.Fields, field)
		}
	}

	expect("client tenant", put(map[string]any{"expected_draft_revision": 1, "content": draft, "tenant_id": f.tenantB.ID.String()}), 422, "validation_failed", "body")
	expect("client role", put(map[string]any{"expected_draft_revision": 1, "content": draft, "role": "admin"}), 422, "validation_failed", "body")
	expect("unknown nested field", put(req(contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["tagline"] = "x" }))), 422, "validation_failed", "body")
	expect("unknown locale", put(req(func() any {
		c := contentWith(draft, func(map[string]any) {})
		c["locales"].(map[string]any)["fr"] = c["locales"].(map[string]any)["id"]
		return c
	}())), 422, "website_content_invalid", "locales.fr")
	expect("unknown slot", put(req(contentWith(draft, func(id map[string]any) {
		id["programs"].(map[string]any)["sma"] = id["programs"].(map[string]any)["tni"]
	}))), 422, "website_content_invalid", "locales.id.programs.sma")
	expect("absent optional field", put(req(contentWith(draft, func(id map[string]any) {
		delete(id["programs"].(map[string]any)["tni"].(map[string]any), "schedule")
	}))), 422, "website_content_invalid", "locales.id.programs.tni.schedule")
	expect("null optional field", put(req(contentWith(draft, func(id map[string]any) {
		id["programs"].(map[string]any)["polri"].(map[string]any)["contact"] = nil
	}))), 422, "website_content_invalid", "locales.id.programs.polri.contact")
	expect("wrong key casing", put(req(contentWith(draft, func(id map[string]any) {
		home := id["home"].(map[string]any)
		home["HERO_TITLE"] = home["hero_title"]
		delete(home, "hero_title")
	}))), 422, "validation_failed", "body")
	expect("wrong top-level casing", put(map[string]any{"Expected_Draft_Revision": 1, "content": draft}), 422, "validation_failed", "body")
	both := put(req(contentWith(draft, func(id map[string]any) {
		delete(id["programs"].(map[string]any)["tni"].(map[string]any), "fees")
		id["home"].(map[string]any)["hero_body"] = strings.Repeat("a", 400)
	})))
	expect("missing and too long together", both, 422, "website_content_invalid", "locales.id.programs.tni.fees")
	if fields := errorBody(t, both).Fields; fields["locales.id.home.hero_body"] == "" {
		t.Errorf("missing and too long together: fields = %v, want hero_body too", fields)
	}
	if got, want := errorBody(t, both).Fields["locales.id.programs.tni.fees"], websiteReason(service.ReasonWebsiteRequired); got != want {
		t.Errorf("missing optional field reason = %q, want %q", got, want)
	}
	expect("blank required field", put(req(contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["hero_title"] = "   " }))), 422, "website_content_invalid", "locales.id.home.hero_title")
	expect("one over the limit", put(req(contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["hero_title"] = strings.Repeat("a", 91) }))), 422, "website_content_invalid", "locales.id.home.hero_title")
	expect("missing revision", put(map[string]any{"content": draft}), 422, "validation_failed", "expected_draft_revision")
	expect("malformed JSON", f.raw(http.MethodPut, "/api/v1/admin/website", `{"expected_draft_revision": 1,`, f.tokenAdminA), 422, "validation_failed", "body")
	expect("trailing data", f.raw(http.MethodPost, "/api/v1/admin/website/publish", `{"expected_draft_revision":1,"expected_published_revision":0} {}`, f.tokenAdminA), 422, "validation_failed", "body")
	expect("publish unknown field", f.call(http.MethodPost, "/api/v1/admin/website/publish", map[string]any{"expected_draft_revision": 1, "expected_published_revision": 0, "published_by": uuid.NewString()}, f.tokenAdminA), 422, "validation_failed", "body")

	huge := f.raw(http.MethodPut, "/api/v1/admin/website", `{"expected_draft_revision":1,"padding":"`+strings.Repeat("a", 64<<10)+`"}`, f.tokenAdminA)
	expect("over 64 KiB", huge, 413, "payload_too_large", "")

	for _, q := range []string{"limit=0", "limit=51", "limit=abc", "cursor=%25%25"} {
		expect("history "+q, f.call(http.MethodGet, "/api/v1/admin/website/history?"+q, nil, f.tokenAdminA), 422, "validation_failed", strings.SplitN(q, "=", 2)[0])
	}

	// Limits are inclusive, and length counts code points: 90 emoji are 360 bytes.
	for name, title := range map[string]string{"90 ASCII": strings.Repeat("a", 90), "90 emoji": strings.Repeat("😀", 90)} {
		rev := f.workbench(t)["draft_revision"]
		rec := f.call(http.MethodPut, "/api/v1/admin/website", map[string]any{"expected_draft_revision": rev, "content": contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["hero_title"] = title })}, f.tokenAdminA)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d body = %s", name, rec.Code, rec.Body.String())
		}
	}

	stale := f.call(http.MethodPut, "/api/v1/admin/website", map[string]any{"expected_draft_revision": 1, "content": contentWith(draft, func(id map[string]any) { id["home"].(map[string]any)["hero_title"] = "stale" })}, f.tokenAdminA)
	expect("stale save", stale, 409, "website_revision_conflict", "")
	expect("stale publish", f.call(http.MethodPost, "/api/v1/admin/website/publish", map[string]any{"expected_draft_revision": 1, "expected_published_revision": 5}, f.tokenAdminA), 409, "website_revision_conflict", "")

	var other int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM website_content WHERE tenant_id = $1`, f.tenantB.ID).Scan(&other); err != nil || other != 0 {
		t.Errorf("a client-supplied tenant wrote %d rows for tenant B (err %v)", other, err)
	}
}
