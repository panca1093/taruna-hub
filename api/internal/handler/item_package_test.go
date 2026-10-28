package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
)

type packageFixture struct {
	assetHTTPFixture
	e http.Handler
}

func newPackageFixture(t *testing.T) packageFixture {
	t.Helper()
	f := packageFixture{assetHTTPFixture: newAssetHTTPFixture(t)}
	f.e = f.server(t, time.Minute)
	return f
}

type uploadPart struct {
	field, name string
	data        []byte
}

func elementsCall(t *testing.T, e http.Handler, host, token, path string, values map[string][]string, parts ...uploadPart) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for field, vs := range values {
		for _, v := range vs {
			if err := w.WriteField(field, v); err != nil {
				t.Fatalf("field %s: %v", field, err)
			}
		}
	}
	for _, p := range parts {
		part, err := w.CreateFormFile(p.field, p.name)
		if err != nil {
			t.Fatalf("part %s: %v", p.name, err)
		}
		if _, err := part.Write(p.data); err != nil {
			t.Fatalf("write %s: %v", p.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(e, req)
}

func (f packageFixture) create(t *testing.T, host, token, kind, name string) itemPackageResponse {
	t.Helper()
	rec := sendTryoutJSON(f.e, http.MethodPost, host, "/api/v1/item-packages", map[string]string{"type": kind, "name": name}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s package = %d %s, want 201", kind, rec.Code, rec.Body.String())
	}
	return decodePackage(t, rec)
}

func decodePackage(t *testing.T, rec *httptest.ResponseRecorder) itemPackageResponse {
	t.Helper()
	var body itemPackageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a package: %v", rec.Body.String(), err)
	}
	return body
}

func decodeDetail(t *testing.T, rec *httptest.ResponseRecorder) itemPackageDetailResponse {
	t.Helper()
	var body itemPackageDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a package detail: %v", rec.Body.String(), err)
	}
	return body
}

func decodePreview(t *testing.T, rec *httptest.ResponseRecorder) itemElementPreviewResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("preview = %d %s, want 200", rec.Code, rec.Body.String())
	}
	var body itemElementPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a preview: %v", rec.Body.String(), err)
	}
	return body
}

func decodeCommit(t *testing.T, rec *httptest.ResponseRecorder) itemElementCommitResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("commit = %d %s, want 200", rec.Code, rec.Body.String())
	}
	var body itemElementCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a commit: %v", rec.Body.String(), err)
	}
	return body
}

func (f packageFixture) detail(t *testing.T, host, token string, id uuid.UUID, query string) itemPackageDetailResponse {
	t.Helper()
	rec := sendTryoutJSON(f.e, http.MethodGet, host, "/api/v1/item-packages/"+id.String()+query, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("get package = %d %s, want 200", rec.Code, rec.Body.String())
	}
	return decodeDetail(t, rec)
}

func (f packageFixture) symbols(t *testing.T, id uuid.UUID, text string) itemElementCommitResponse {
	t.Helper()
	path := "/api/v1/item-packages/" + id.String() + "/elements/commit"
	return decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, path, map[string][]string{"symbols": {text}}))
}

func previewPath(id uuid.UUID) string {
	return "/api/v1/item-packages/" + id.String() + "/elements/preview"
}
func commitPath(id uuid.UUID) string {
	return "/api/v1/item-packages/" + id.String() + "/elements/commit"
}
func packagePath(id uuid.UUID) string { return "/api/v1/item-packages/" + id.String() }
func samplePath(id uuid.UUID) string {
	return "/api/v1/item-packages/" + id.String() + "/sample-column"
}
func elementPath(p, e uuid.UUID) string { return packagePath(p) + "/elements/" + e.String() }

func reasonsOf(p itemElementPreviewResponse) map[string]string {
	out := map[string]string{}
	for _, item := range p.Items {
		out[item.Label] = item.Reason
	}
	return out
}

// tableCounts is what a preview or a sample must leave exactly as it found it.
func (f packageFixture) tableCounts(t *testing.T) string {
	t.Helper()
	var out string
	err := f.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM item_packages WHERE tenant_id = ANY($1))::text || '/' ||
		       (SELECT count(*) FROM item_package_revisions r JOIN item_packages p ON p.id = r.package_id WHERE p.tenant_id = ANY($1))::text || '/' ||
		       (SELECT count(*) FROM item_package_elements e JOIN item_packages p ON p.id = e.package_id WHERE p.tenant_id = ANY($1))::text || '/' ||
		       (SELECT count(*) FROM assets WHERE tenant_id = ANY($1))::text`,
		[]uuid.UUID{f.tenantA.ID, f.tenantB.ID}).Scan(&out)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return out
}

func TestItemPackageRoutesAllowAdminAndOperatorAndDenyEveryoneElse(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Peran")
	f.symbols(t, pkg.ID, "♠ ♥ ★")
	detail := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	element := detail.Elements[0].ID
	missing := uuid.New()

	routes := []struct {
		name, method, path string
		body               any
		multipart          bool
	}{
		{"list", http.MethodGet, "/api/v1/item-packages", nil, false},
		{"create", http.MethodPost, "/api/v1/item-packages", map[string]string{"type": "emoji", "name": "Peran-" + uuid.NewString()[:6]}, false},
		{"catalog", http.MethodGet, "/api/v1/item-packages/catalog", nil, false},
		{"template", http.MethodGet, "/api/v1/item-packages/symbol-template?format=csv", nil, false},
		{"get", http.MethodGet, packagePath(pkg.ID), nil, false},
		{"rename", http.MethodPut, packagePath(pkg.ID), map[string]string{"name": "Peran baru"}, false},
		{"preview", http.MethodPost, previewPath(pkg.ID), nil, true},
		{"commit", http.MethodPost, commitPath(pkg.ID), nil, true},
		{"sample", http.MethodPost, samplePath(pkg.ID), map[string]int{"item_length": 1, "options_per_soal": 2, "soal_per_kolom": 1}, false},
		{"remove", http.MethodDelete, elementPath(pkg.ID, element), nil, false},
		{"delete", http.MethodDelete, packagePath(missing), nil, false},
	}
	call := func(r struct {
		name, method, path string
		body               any
		multipart          bool
	}, token string) *httptest.ResponseRecorder {
		if r.multipart {
			return elementsCall(t, f.e, f.hostA, token, r.path, map[string][]string{"symbols": {"♦"}})
		}
		return sendTryoutJSON(f.e, r.method, f.hostA, r.path, r.body, token)
	}
	for _, r := range routes {
		t.Run(r.name, func(t *testing.T) {
			if rec := call(r, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("anonymous = %d, want 401", rec.Code)
			}
			if rec := call(r, f.tokenA); rec.Code != http.StatusForbidden {
				t.Errorf("student = %d %s, want 403", rec.Code, rec.Body.String())
			}
			for who, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
				rec := call(r, token)
				if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
					t.Errorf("%s = %d %s, want the route to run", who, rec.Code, rec.Body.String())
				}
				if rec.Header().Get("Cache-Control") != "no-store" {
					t.Errorf("%s: Cache-Control = %q, want no-store", who, rec.Header().Get("Cache-Control"))
				}
			}
		})
	}
}

func TestOperatorMayCreateFillAndDeleteAPackage(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenOperatorA, "symbol", "Dari operator")
	rec := elementsCall(t, f.e, f.hostA, f.tokenOperatorA, commitPath(pkg.ID), map[string][]string{"symbols": {"♠ ♥"}})
	if c := decodeCommit(t, rec); c.Added != 2 {
		t.Errorf("operator commit added %d, want 2", c.Added)
	}
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(pkg.ID), nil, f.tokenOperatorA); rec.Code != http.StatusNoContent {
		t.Errorf("operator delete = %d %s, want 204", rec.Code, rec.Body.String())
	}
}

func TestAnotherTenantsPackageIsNotFoundOnEveryRoute(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Milik A")
	f.symbols(t, pkg.ID, "♠ ♥ ★")
	element := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "").Elements[0].ID
	before := f.tableCounts(t)

	foreign := []struct {
		name   string
		status int
		rec    *httptest.ResponseRecorder
	}{
		{"get", 404, sendTryoutJSON(f.e, http.MethodGet, f.hostB, packagePath(pkg.ID), nil, f.tokenAdmB)},
		{"get revision", 404, sendTryoutJSON(f.e, http.MethodGet, f.hostB, packagePath(pkg.ID)+"?revision=1", nil, f.tokenAdmB)},
		{"rename", 404, sendTryoutJSON(f.e, http.MethodPut, f.hostB, packagePath(pkg.ID), map[string]string{"name": "Dicuri"}, f.tokenAdmB)},
		{"delete", 404, sendTryoutJSON(f.e, http.MethodDelete, f.hostB, packagePath(pkg.ID), nil, f.tokenAdmB)},
		{"preview", 404, elementsCall(t, f.e, f.hostB, f.tokenAdmB, previewPath(pkg.ID), map[string][]string{"symbols": {"x"}})},
		{"commit", 404, elementsCall(t, f.e, f.hostB, f.tokenAdmB, commitPath(pkg.ID), map[string][]string{"symbols": {"x"}})},
		{"remove", 404, sendTryoutJSON(f.e, http.MethodDelete, f.hostB, elementPath(pkg.ID, element), nil, f.tokenAdmB)},
		{"sample", 404, sendTryoutJSON(f.e, http.MethodPost, f.hostB, samplePath(pkg.ID), map[string]int{"item_length": 1, "options_per_soal": 2, "soal_per_kolom": 1}, f.tokenAdmB)},
	}
	for _, c := range foreign {
		if c.rec.Code != c.status {
			t.Errorf("%s from another tenant = %d %s, want %d", c.name, c.rec.Code, c.rec.Body.String(), c.status)
		}
	}
	if after := f.tableCounts(t); after != before {
		t.Errorf("rows %s after foreign calls, want %s: another tenant changed something", after, before)
	}

	list := sendTryoutJSON(f.e, http.MethodGet, f.hostB, "/api/v1/item-packages", nil, f.tokenAdmB)
	if strings.Contains(list.Body.String(), pkg.ID.String()) {
		t.Errorf("tenant B lists tenant A's package: %s", list.Body.String())
	}
	own := f.create(t, f.hostB, f.tokenAdmB, "symbol", "Milik A")
	if own.ID == pkg.ID {
		t.Errorf("the same name in another tenant must be its own package")
	}
}

func TestPackageNamesAreUniquePerTenantIgnoringCase(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Panah")
	other := f.create(t, f.hostA, f.tokenAdminA, "emoji", "Hewan")

	dup := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/item-packages", map[string]string{"type": "emoji", "name": "  pAnAh "}, f.tokenAdminA)
	if dup.Code != http.StatusConflict || errorBody(t, dup).Fields["name"] == "" {
		t.Errorf("duplicate name = %d %s, want 409 with fields.name", dup.Code, dup.Body.String())
	}
	clash := sendTryoutJSON(f.e, http.MethodPut, f.hostA, packagePath(other.ID), map[string]string{"name": "PANAH"}, f.tokenAdminA)
	if clash.Code != http.StatusConflict {
		t.Errorf("rename onto a taken name = %d, want 409", clash.Code)
	}
	renamed := sendTryoutJSON(f.e, http.MethodPut, f.hostA, packagePath(pkg.ID), map[string]string{"name": "Panah baru"}, f.tokenAdminA)
	if renamed.Code != http.StatusOK || decodePackage(t, renamed).Name != "Panah baru" || decodePackage(t, renamed).Revision != 1 {
		t.Errorf("rename = %d %s, want 200, the new name and no new revision", renamed.Code, renamed.Body.String())
	}
	for name, body := range map[string]map[string]string{
		"bad type":   {"type": "mix", "name": "X"},
		"empty name": {"type": "symbol", "name": "  "},
		"long name":  {"type": "symbol", "name": strings.Repeat("n", 101)},
	} {
		if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/item-packages", body, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d %s, want 422", name, rec.Code, rec.Body.String())
		}
	}
	if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/item-packages", map[string]any{"type": "symbol", "name": "ok", "tenant_id": uuid.NewString()}, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a client-supplied tenant_id = %d, want 422: the tenant is never read from the body", rec.Code)
	}
}

func TestSymbolPreviewChangesNothingAndCommitWritesOneFrozenRevision(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Simbol")
	before := f.tableCounts(t)

	text := "♠ ♥ ★ 😀 ab ♠"
	preview := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), map[string][]string{"symbols": {text}}))
	if preview.Accepted != 3 || preview.Rejected != 3 {
		t.Errorf("preview accepted %d rejected %d, want 3 and 3: %+v", preview.Accepted, preview.Rejected, preview.Items)
	}
	reasons := reasonsOf(preview)
	if reasons["😀"] != "emoji_in_symbol" || reasons["ab"] != "symbol_multi" || reasons["♠"] != "duplicate_in_upload" && reasons["♠"] != "" {
		t.Errorf("reasons = %v", reasons)
	}
	for _, item := range preview.Items {
		if !item.Accepted && item.Message == "" {
			t.Errorf("%q rejected with no guidance message", item.Label)
		}
		if item.Reason == "emoji_in_symbol" && !strings.Contains(item.Message, "Emoji") {
			t.Errorf("emoji guidance = %q, want it to point at the emoji package", item.Message)
		}
	}
	if after := f.tableCounts(t); after != before {
		t.Fatalf("rows %s after a preview, want %s", after, before)
	}
	if d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, ""); d.Package.Revision != 1 || len(d.Elements) != 0 {
		t.Fatalf("package after preview = revision %d with %d elements, want untouched", d.Package.Revision, len(d.Elements))
	}

	commit := f.symbols(t, pkg.ID, text)
	if commit.Added != 3 || commit.Revision != 2 || commit.Rejected != 2 || commit.AlreadyPresent != 1 {
		t.Errorf("commit = %+v, want 3 added at revision 2, 2 rejected, 1 repeated", commit)
	}
	again := f.symbols(t, pkg.ID, text)
	if again.Added != 0 || again.Revision != 2 || again.AlreadyPresent != 4 {
		t.Errorf("retry = %+v, want nothing added, revision still 2, 4 already present", again)
	}

	current := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	if current.Package.ElementCount != 3 || len(current.Elements) != 3 || current.Package.Revision != 2 {
		t.Errorf("current = revision %d, %d elements, want 3 at revision 2", current.Package.Revision, len(current.Elements))
	}
	if first := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "?revision=1"); len(first.Elements) != 0 || first.Revision != 1 {
		t.Errorf("revision 1 = %d elements at %d, want the frozen empty revision", len(first.Elements), first.Revision)
	}
	for _, bad := range []string{"?revision=3", "?revision=0", "?revision=abc"} {
		if rec := sendTryoutJSON(f.e, http.MethodGet, f.hostA, packagePath(pkg.ID)+bad, nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("GET %s = %d, want 422", bad, rec.Code)
		}
	}
}

func TestConcurrentIdenticalCommitsWriteEachElementOnce(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Serentak")

	var wg sync.WaitGroup
	added := make([]int, 6)
	for at := range added {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), map[string][]string{"symbols": {"♠ ♥ ★ ▲"}})
			if rec.Code == http.StatusOK {
				var c itemElementCommitResponse
				_ = json.Unmarshal(rec.Body.Bytes(), &c)
				added[at] = c.Added
			}
		}()
	}
	wg.Wait()
	total := 0
	for _, n := range added {
		total += n
	}
	d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	if total != 4 || len(d.Elements) != 4 || d.Package.Revision != 2 {
		t.Errorf("added %d in total, package has %d elements at revision %d; want 4 at revision 2", total, len(d.Elements), d.Package.Revision)
	}
}

func TestSymbolFilesAndPasteGoThroughTheSameRules(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Dari berkas")

	csvData := []byte("simbol\n♠\n♥\n😀\n")
	rec := elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil, uploadPart{"file", "s.csv", csvData})
	if c := decodeCommit(t, rec); c.Added != 2 || c.Rejected != 1 {
		t.Errorf("csv commit = %+v, want 2 added and the emoji rejected", c)
	}
	txt := elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil, uploadPart{"file", "s.txt", []byte("♥ ◆ ◇\n")})
	if c := decodeCommit(t, txt); c.Added != 2 || c.AlreadyPresent != 1 {
		t.Errorf("txt commit = %+v, want 2 added and the repeated heart present", c)
	}

	bad := elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil, uploadPart{"file", "s.png", pngBytes(t, 2, 2)})
	if bad.Code != http.StatusUnprocessableEntity || errorBody(t, bad).Fields["file"] == "" {
		t.Errorf("a PNG as a symbol file = %d %s, want 422 on fields.file", bad.Code, bad.Body.String())
	}
	wrongKind := elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), map[string][]string{"ids": {"1f34e"}})
	if wrongKind.Code != http.StatusUnprocessableEntity || errorBody(t, wrongKind).Fields["elements"] == "" {
		t.Errorf("emoji ids into a symbol package = %d %s, want 422 on fields.elements", wrongKind.Code, wrongKind.Body.String())
	}
	empty := elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil)
	if empty.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty upload = %d, want 422", empty.Code)
	}

	for _, format := range []string{"csv", "xlsx", ""} {
		rec := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/item-packages/symbol-template?format="+format, nil, f.tokenAdminA)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "template-simbol.") {
			t.Errorf("template %q = %d %q, want a download", format, rec.Code, rec.Header().Get("Content-Disposition"))
		}
	}
	if rec := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/item-packages/symbol-template?format=docx", nil, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("template docx = %d, want 422", rec.Code)
	}
}

func TestEmojiIsCatalogOnlyAndShapesAreBuiltIn(t *testing.T) {
	f := newPackageFixture(t)

	catalog := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/item-packages/catalog", nil, f.tokenAdminA)
	var body itemCatalogResponse
	if err := json.Unmarshal(catalog.Body.Bytes(), &body); err != nil || len(body.Emoji) == 0 || len(body.Shapes) != 7 || len(body.SymbolPalette) == 0 {
		t.Fatalf("catalog = %s (%v), want emoji, 7 shapes and a palette", catalog.Body.String(), err)
	}
	if !strings.Contains(body.EmojiAttribution, "Twemoji") || !strings.Contains(body.EmojiAttribution, "CC-BY 4.0") {
		t.Errorf("attribution = %q, want the Twemoji CC-BY 4.0 credit", body.EmojiAttribution)
	}
	for _, e := range body.Emoji {
		if _, err := os.Stat("../../../web/public/emoji/" + e.ID + ".svg"); err != nil {
			t.Errorf("catalog emoji %s is not bundled: %v", e.ID, err)
		}
	}

	pkg := f.create(t, f.hostA, f.tokenAdminA, "emoji", "Hewan")
	ids := map[string][]string{"ids": {"1f436", "1f431", "1f600-1f3fd", "😀", "1f436"}}
	preview := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), ids))
	if preview.Accepted != 2 || preview.Rejected != 3 {
		t.Errorf("emoji preview = %+v, want 2 catalog ids accepted", preview)
	}
	typed := elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), map[string][]string{"symbols": {"😀"}})
	if typed.Code != http.StatusUnprocessableEntity {
		t.Errorf("a typed emoji into an emoji package = %d, want 422: the catalog is the only source", typed.Code)
	}
	if c := decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), ids)); c.Added != 2 {
		t.Errorf("emoji commit = %+v, want 2 added", c)
	}
	d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	if len(d.Elements) != 2 || d.Elements[0].Value != "1f436" || d.Elements[0].AssetID != nil {
		t.Errorf("elements = %+v, want emoji ids as values and no asset", d.Elements)
	}

	shapes := f.create(t, f.hostA, f.tokenAdminA, "shape", "Bangun")
	rec := elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(shapes.ID), map[string][]string{"ids": {"lingkaran", "kotak", "bintang"}})
	if c := decodeCommit(t, rec); c.Added != 2 || c.Rejected != 1 {
		t.Errorf("shape commit = %+v, want 2 added and the unknown shape rejected", c)
	}
}

func zipOf(t *testing.T, entries map[string][]byte, order ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range order {
		part, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(entries[name])
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImagePackageFromZipPreviewsFirstThenStoresOnlyAcceptedFiles(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "image", "Gambar")
	a, b := pngBytes(t, 6, 6), pngBytes(t, 7, 7)
	archive := zipOf(t, map[string][]byte{
		"a.png": a, "copy-of-a.png": a, "b.png": b, "logo.svg": []byte("<svg/>"), "../evil.png": pngBytes(t, 8, 8),
		"inner.zip": zipOf(t, map[string][]byte{"x.png": a}, "x.png"), "broken.png": a[:30],
	}, "a.png", "copy-of-a.png", "b.png", "logo.svg", "../evil.png", "inner.zip", "broken.png")
	parts := []uploadPart{{"files", "pack.zip", archive}}
	before := f.tableCounts(t)
	puts := f.store.puts.Load()

	preview := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil, parts...))
	want := map[string]string{
		"a.png": "", "b.png": "", "copy-of-a.png": "duplicate_in_upload", "logo.svg": "image_type",
		"evil.png": "path_traversal", "inner.zip": "nested_archive", "broken.png": "image_corrupt",
	}
	for label, reason := range want {
		got, ok := reasonsOf(preview)[label]
		if !ok || got != reason {
			t.Errorf("preview %q = %q (present %v), want %q", label, got, ok, reason)
		}
	}
	if preview.Accepted != 2 || preview.Rejected != 5 {
		t.Errorf("preview accepted %d rejected %d, want 2 and 5", preview.Accepted, preview.Rejected)
	}
	if after := f.tableCounts(t); after != before || f.store.puts.Load() != puts {
		t.Fatalf("a preview changed rows %s -> %s or wrote %d objects", before, after, f.store.puts.Load()-puts)
	}

	commit := decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil, parts...))
	if commit.Added != 2 || commit.Rejected != 4 || commit.AlreadyPresent != 1 || commit.Revision != 2 {
		t.Errorf("commit = %+v, want 2 added, 4 rejected, 1 repeated, revision 2", commit)
	}
	if got := f.store.puts.Load() - puts; got != 2 {
		t.Errorf("%d objects stored, want only the 2 accepted images", got)
	}
	var finalAssets, stagedAssets int
	if err := f.pool.QueryRow(context.Background(), `
		SELECT count(*) FILTER (WHERE status = 'final'), count(*) FILTER (WHERE status = 'staged')
		  FROM assets WHERE tenant_id = $1 AND purpose = 'item_image'`, f.tenantA.ID).Scan(&finalAssets, &stagedAssets); err != nil {
		t.Fatal(err)
	}
	if finalAssets != 2 || stagedAssets != 0 {
		t.Errorf("item_image assets final=%d staged=%d, want 2 and 0", finalAssets, stagedAssets)
	}

	d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	if len(d.Elements) != 2 {
		t.Fatalf("package has %d elements, want 2", len(d.Elements))
	}
	for _, e := range d.Elements {
		if e.AssetID == nil || e.Value != "" {
			t.Errorf("element %+v, want an asset id and no value", e)
		}
		link := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/assets/"+e.AssetID.String()+"/url", nil, f.tokenAdminA)
		if link.Code != http.StatusOK {
			t.Errorf("asset link for %s = %d, want 200 for staff on a final asset", e.Label, link.Code)
		}
	}
	if strings.Contains(sendTryoutJSON(f.e, http.MethodGet, f.hostA, packagePath(pkg.ID), nil, f.tokenAdminA).Body.String(), "object_key") {
		t.Errorf("the detail leaks an object key")
	}

	again := decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil, parts...))
	if again.Added != 0 || again.Revision != 2 || f.store.puts.Load()-puts != 2 {
		t.Errorf("retry = %+v after %d stores, want nothing added and no new object", again, f.store.puts.Load()-puts)
	}

	other := f.create(t, f.hostA, f.tokenAdminA, "image", "Gambar lain")
	same := decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(other.ID), nil, uploadPart{"files", "renamed.png", a}))
	if same.Added != 1 {
		t.Errorf("the same image in another package = %+v, want it accepted: duplicates are per package", same)
	}
}

func TestImagePackageWithPlainFilesAndFrozenAssetsSurviveRemoval(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "image", "Berkas biasa")
	rec := elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil,
		uploadPart{"files", "one.png", pngBytes(t, 3, 3)}, uploadPart{"files", "two.png", pngBytes(t, 4, 4)},
		uploadPart{"files", "vector.svg", []byte("<svg/>")}, uploadPart{"files", "huge.png", pngBytes(t, 4097, 5)})
	if c := decodeCommit(t, rec); c.Added != 2 || c.Rejected != 2 {
		t.Fatalf("commit = %+v, want 2 added and svg + oversize-dimension rejected", c)
	}
	d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")
	removed := d.Elements[0]

	rm := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, elementPath(pkg.ID, removed.ID), nil, f.tokenAdminA)
	if rm.Code != http.StatusOK || decodePackage(t, rm).Revision != 3 || decodePackage(t, rm).ElementCount != 1 {
		t.Fatalf("remove = %d %s, want 200 at revision 3 with 1 element", rm.Code, rm.Body.String())
	}
	frozen := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "?revision=2")
	if len(frozen.Elements) != 2 {
		t.Fatalf("revision 2 has %d elements, want both: a revision never changes", len(frozen.Elements))
	}
	var status string
	if err := f.pool.QueryRow(context.Background(), `SELECT status FROM assets WHERE id = $1`, removed.AssetID).Scan(&status); err != nil || status != "final" {
		t.Errorf("removed element's asset = %q (%v), want it kept final for the frozen revision", status, err)
	}
	if link := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/assets/"+removed.AssetID.String()+"/url", nil, f.tokenAdminA); link.Code != http.StatusOK {
		t.Errorf("link to a frozen asset = %d, want it still renderable", link.Code)
	}
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, elementPath(pkg.ID, removed.ID), nil, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("removing it twice = %d, want 404", rec.Code)
	}
}

func TestImageArchiveLevelLimitsRejectTheArchiveAndWriteNothing(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "image", "Batas")
	before := f.tableCounts(t)
	puts := f.store.puts.Load()

	many := map[string][]byte{}
	var names []string
	good := pngBytes(t, 3, 3)
	for at := 0; at < 201; at++ {
		name := fmt.Sprintf("f%03d.png", at)
		many[name], names = good, append(names, name)
	}
	bomb := map[string][]byte{}
	var bombNames []string
	for at := 0; at < 11; at++ {
		name := fmt.Sprintf("p%d.png", at)
		bomb[name], bombNames = make([]byte, 5<<20), append(bombNames, name)
	}
	cases := map[string]struct {
		data   []byte
		reason string
	}{
		"201 files":     {zipOf(t, many, names...), "archive_too_many_files"},
		"55 MB extract": {zipOf(t, bomb, bombNames...), "archive_extracted_too_large"},
		"not a zip":     {[]byte("PK\x03\x04garbage"), "archive_corrupt"},
	}
	for name, tc := range cases {
		preview := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil, uploadPart{"files", "pack.zip", tc.data}))
		if len(preview.Items) != 1 || preview.Items[0].Reason != tc.reason || preview.Accepted != 0 {
			t.Errorf("%s: preview = %+v, want the archive rejected with %s", name, preview, tc.reason)
		}
		commit := decodeCommit(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, commitPath(pkg.ID), nil, uploadPart{"files", "pack.zip", tc.data}))
		if commit.Added != 0 || commit.Revision != 1 {
			t.Errorf("%s: commit = %+v, want nothing written", name, commit)
		}
	}
	if after := f.tableCounts(t); after != before || f.store.puts.Load() != puts {
		t.Errorf("rows %s -> %s, %d objects stored; want nothing written for refused archives", before, after, f.store.puts.Load()-puts)
	}

	tooBig := elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil, uploadPart{"files", "pack.zip", make([]byte, 65<<20)})
	if tooBig.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a 65 MB body = %d, want 413", tooBig.Code)
	}
	overTen := decodePreview(t, elementsCall(t, f.e, f.hostA, f.tokenAdminA, previewPath(pkg.ID), nil, uploadPart{"files", "big.zip", make([]byte, 10<<20+5)}))
	if len(overTen.Items) != 1 || overTen.Items[0].Reason != "archive_too_large" {
		t.Errorf("an 11 MB zip = %+v, want archive_too_large", overTen)
	}
}

func TestPackageInUseCannotBeDeletedOrShrunkBelowATestsNeed(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Dipakai")
	f.symbols(t, pkg.ID, "♠ ♥ ★ ▲")
	current := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "")

	// P10 will write this row when a test pins a package; the table is the seam it plugs into.
	_, err := f.pool.Exec(context.Background(), `
		INSERT INTO kecermatan_test_packages (test_id, tenant_id, package_id, revision, required_count)
		VALUES ($1, $2, $3, $4, 3)`, uuid.New(), f.tenantA.ID, pkg.ID, current.Package.Revision)
	if err != nil {
		t.Fatalf("pin the package: %v", err)
	}

	list := sendTryoutJSON(f.e, http.MethodGet, f.hostA, "/api/v1/item-packages?type=symbol", nil, f.tokenAdminA)
	var packages itemPackagesResponse
	_ = json.Unmarshal(list.Body.Bytes(), &packages)
	var listed *itemPackageResponse
	for i := range packages.Packages {
		if packages.Packages[i].ID == pkg.ID {
			listed = &packages.Packages[i]
		}
	}
	if listed == nil || !listed.InUse || listed.RequiredCount != 3 || listed.ElementCount != 4 {
		t.Fatalf("listed = %+v, want in use, needing 3, holding 4", listed)
	}

	del := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(pkg.ID), nil, f.tokenAdminA)
	if del.Code != http.StatusConflict || errorBody(t, del).Code != httpx.Code("package_in_use") {
		t.Errorf("delete of a used package = %d %s, want 409 package_in_use", del.Code, del.Body.String())
	}
	if rec := sendTryoutJSON(f.e, http.MethodGet, f.hostA, packagePath(pkg.ID), nil, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Errorf("the used package is gone after a refused delete: %d", rec.Code)
	}

	first := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, elementPath(pkg.ID, current.Elements[0].ID), nil, f.tokenAdminA)
	if first.Code != http.StatusOK || decodePackage(t, first).ElementCount != 3 {
		t.Fatalf("shrinking 4 -> 3 = %d %s, want it allowed: the test still has what it needs", first.Code, first.Body.String())
	}
	second := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, elementPath(pkg.ID, current.Elements[1].ID), nil, f.tokenAdminA)
	if second.Code != http.StatusConflict || errorBody(t, second).Code != httpx.Code("package_too_small") {
		t.Errorf("shrinking 3 -> 2 = %d %s, want 409 package_too_small", second.Code, second.Body.String())
	}
	if d := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, ""); len(d.Elements) != 3 || d.Package.Revision != 3 {
		t.Errorf("after the refused shrink: %d elements at revision %d, want 3 at 3", len(d.Elements), d.Package.Revision)
	}
	pinned := f.detail(t, f.hostA, f.tokenAdminA, pkg.ID, "?revision="+fmt.Sprint(current.Package.Revision))
	if len(pinned.Elements) != 4 {
		t.Errorf("the pinned revision has %d elements, want the 4 it was frozen with", len(pinned.Elements))
	}

	_, rawErr := f.pool.Exec(context.Background(), `DELETE FROM item_packages WHERE id = $1`, pkg.ID)
	if pgErr, ok := rawErr.(*pgconn.PgError); !ok || pgErr.Code != "23503" {
		t.Errorf("a raw DELETE of a pinned package = %v, want the foreign key to refuse it (23503)", rawErr)
	}
	_, rawErr = f.pool.Exec(context.Background(), `DELETE FROM item_package_revisions WHERE package_id = $1 AND revision = $2`, pkg.ID, current.Package.Revision)
	if pgErr, ok := rawErr.(*pgconn.PgError); !ok || pgErr.Code != "23503" {
		t.Errorf("a raw DELETE of a pinned revision = %v, want the foreign key to refuse it (23503)", rawErr)
	}
}

func TestFrozenRevisionRowsCannotBeUpdated(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Beku")
	f.symbols(t, pkg.ID, "♠")
	for _, stmt := range []string{
		`UPDATE item_package_elements SET value = 'x' WHERE package_id = $1`,
		`UPDATE item_package_revisions SET element_count = 99 WHERE package_id = $1`,
	} {
		_, err := f.pool.Exec(context.Background(), stmt, pkg.ID)
		if pgErr, ok := err.(*pgconn.PgError); !ok || pgErr.Code != "23514" {
			t.Errorf("%s = %v, want the frozen-row trigger (23514)", stmt, err)
		}
	}
}

func TestUnusedPackageDeletesWithItsRevisions(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Sementara")
	f.symbols(t, pkg.ID, "♠ ♥")
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(pkg.ID), nil, f.tokenAdminA); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s, want 204", rec.Code, rec.Body.String())
	}
	var left int
	if err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM item_package_revisions WHERE package_id = $1) + (SELECT count(*) FROM item_package_elements WHERE package_id = $1)`, pkg.ID).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d revision/element rows left after the delete (%v)", left, err)
	}
	if rec := sendTryoutJSON(f.e, http.MethodDelete, f.hostA, packagePath(pkg.ID), nil, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", rec.Code)
	}
}

func TestSampleColumnIsEphemeralAndRespectsThePoolRules(t *testing.T) {
	f := newPackageFixture(t)
	pkg := f.create(t, f.hostA, f.tokenAdminA, "symbol", "Contoh")
	f.symbols(t, pkg.ID, "♠ ♥ ★ ▲ ● ■ ◆ ▼ ○ □ ◇ △")
	before := f.tableCounts(t)

	rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, samplePath(pkg.ID), map[string]int{"item_length": 2, "options_per_soal": 5, "soal_per_kolom": 8}, f.tokenAdminA)
	if rec.Code != http.StatusOK {
		t.Fatalf("sample = %d %s, want 200", rec.Code, rec.Body.String())
	}
	var column sampleColumnResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &column); err != nil || len(column.Soal) != 8 {
		t.Fatalf("sample = %s (%v), want 8 soal", rec.Body.String(), err)
	}
	for at, soal := range column.Soal {
		if len(soal.Items) != 5 {
			t.Fatalf("soal %d has %d items, want 5", at, len(soal.Items))
		}
		seen := map[uuid.UUID]bool{}
		for _, item := range soal.Items {
			if len(item.Elements) != 2 {
				t.Fatalf("soal %d has an item of %d elements, want 2", at, len(item.Elements))
			}
			for _, e := range item.Elements {
				if seen[e.ID] {
					t.Errorf("soal %d repeats element %s: elements must be distinct within a soal", at, e.Label)
				}
				seen[e.ID] = true
			}
		}
	}
	if after := f.tableCounts(t); after != before {
		t.Errorf("rows %s after a sample, want %s: the sample stores nothing", after, before)
	}

	tooSmall := sendTryoutJSON(f.e, http.MethodPost, f.hostA, samplePath(pkg.ID), map[string]int{"item_length": 5, "options_per_soal": 5, "soal_per_kolom": 1}, f.tokenAdminA)
	if tooSmall.Code != http.StatusUnprocessableEntity || errorBody(t, tooSmall).Fields["elements"] == "" {
		t.Errorf("25 elements needed from 12 = %d %s, want 422 on fields.elements", tooSmall.Code, tooSmall.Body.String())
	}
	for name, body := range map[string]map[string]int{
		"length 0":   {"item_length": 0, "options_per_soal": 2, "soal_per_kolom": 1},
		"length 6":   {"item_length": 6, "options_per_soal": 2, "soal_per_kolom": 1},
		"one option": {"item_length": 1, "options_per_soal": 1, "soal_per_kolom": 1},
		"no soal":    {"item_length": 1, "options_per_soal": 2, "soal_per_kolom": 0},
		"101 soal":   {"item_length": 1, "options_per_soal": 2, "soal_per_kolom": 101},
	} {
		if rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, samplePath(pkg.ID), body, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, want 422", name, rec.Code)
		}
	}
}
