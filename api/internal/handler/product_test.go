package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func sendProductJSON(e http.Handler, method, host, path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(e, method, host, path, body, token)
}

func decodeProduct(t *testing.T, rec *httptest.ResponseRecorder) productResponse {
	t.Helper()
	var body productResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a product: %v", rec.Body.String(), err)
	}
	return body
}

func decodeProducts(t *testing.T, rec *httptest.ResponseRecorder) productsResponse {
	t.Helper()
	var body productsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a product list: %v", rec.Body.String(), err)
	}
	return body
}

func decodePublicProducts(t *testing.T, rec *httptest.ResponseRecorder) publicProductsResponse {
	t.Helper()
	var body publicProductsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a catalog: %v", rec.Body.String(), err)
	}
	return body
}

func decodePublicProduct(t *testing.T, rec *httptest.ResponseRecorder) publicProductResponse {
	t.Helper()
	var body publicProductResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a catalog product: %v", rec.Body.String(), err)
	}
	return body
}

func decodeSummary(t *testing.T, rec *httptest.ResponseRecorder) catalogSummaryResponse {
	t.Helper()
	var body catalogSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a catalog summary: %v", rec.Body.String(), err)
	}
	return body
}

// productBody is one valid product over the caller's tryouts; `extra` may overwrite any field.
func productBody(program programResponse, tryoutIDs []string, extra map[string]any) map[string]any {
	ids := make([]any, 0, len(tryoutIDs))
	for _, id := range tryoutIDs {
		ids = append(ids, id)
	}
	productType := model.ProductIndividual
	if len(tryoutIDs) > 1 {
		productType = model.ProductBundle
	}
	body := map[string]any{
		"type": productType, "title": "Paket " + uuid.NewString()[:8], "description": "Paket latihan",
		"program_id": program.ID.String(), "price": "150000", "original_price": nil,
		"is_active": true, "active_until": nil, "tryout_ids": ids,
	}
	for key, value := range extra {
		body[key] = value
	}
	return body
}

func publishedTryout(t *testing.T, e http.Handler, host, token string, program programResponse) tryoutResponse {
	t.Helper()
	question := createTryoutQuestion(t, e, host, token, program)
	created := decodeTryout(t, sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts",
		tryoutBody(program, question.ID.String()), token))
	rec := sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts/"+created.ID.String()+"/publish", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish tryout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeTryout(t, rec)
}

func createProduct(t *testing.T, e http.Handler, host, token string, body map[string]any) productResponse {
	t.Helper()
	rec := sendProductJSON(e, http.MethodPost, host, "/api/v1/products", body, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create product: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeProduct(t, rec)
}

func TestProductsIsolatedBetweenTenants(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	programA := questionCurriculum(t, e, hostA, adminA)
	tryoutA := publishedTryout(t, e, hostA, adminA, programA)
	mine := createProduct(t, e, hostA, adminA, productBody(programA, []string{tryoutA.ID.String()}, nil))

	programB := questionCurriculum(t, e, hostB, adminB)
	tryoutB := publishedTryout(t, e, hostB, adminB, programB)

	if other := decodeProducts(t, getWithToken(e, hostB, "/api/v1/products", adminB)); other.Total != 0 {
		t.Errorf("tenant B sees %d products, want none of tenant A's", other.Total)
	}
	if strings.Contains(getWithToken(e, hostB, "/api/v1/products", adminB).Body.String(), mine.ID.String()) {
		t.Error("tenant B's list carries tenant A's product id")
	}

	path := "/api/v1/products/" + mine.ID.String()
	if rec := getWithToken(e, hostB, path, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B GET on A's product: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	update := productBody(programB, []string{tryoutB.ID.String()}, nil)
	if rec := sendProductJSON(e, http.MethodPut, hostB, path, update, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B PUT on A's product: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := sendProductJSON(e, http.MethodDelete, hostB, path, nil, adminB); rec.Code != http.StatusNotFound {
		t.Errorf("B DELETE on A's product: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}

	foreign := productBody(programB, []string{tryoutA.ID.String()}, nil)
	wantError(t, "attach another tenant's tryout", sendProductJSON(e, http.MethodPost, hostB, "/api/v1/products", foreign, adminB),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "tryout_ids[0]")

	still := decodeProduct(t, getWithToken(e, hostA, path, adminA))
	if still.ID != mine.ID || still.Title != mine.Title || len(still.Tryouts) != 1 {
		t.Errorf("tenant A product = %+v, want the original untouched", still)
	}
}

func TestProductRoles(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	operator := curriculumToken(t, f, tenant, model.RoleOperator)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	program := questionCurriculum(t, e, host, admin)
	tryout := publishedTryout(t, e, host, admin, program)
	one := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))
	operatorPut := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))
	adminPut := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))
	operatorDelete := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))
	adminDelete := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))

	path := "/api/v1/products/" + one.ID.String()
	update := func(title string) map[string]any {
		body := productBody(program, []string{tryout.ID.String()}, nil)
		body["title"] = title
		return body
	}

	cases := []struct {
		method string
		path   string
		body   any
		who    string
		token  string
		want   int
	}{
		{"GET", "/api/v1/products", nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", "/api/v1/products", nil, "student", student, http.StatusForbidden},
		{"GET", "/api/v1/products", nil, "operator", operator, http.StatusOK},
		{"GET", "/api/v1/products", nil, "admin", admin, http.StatusOK},
		{"GET", path, nil, "anonymous", "", http.StatusUnauthorized},
		{"GET", path, nil, "student", student, http.StatusForbidden},
		{"GET", path, nil, "operator", operator, http.StatusOK},
		{"GET", path, nil, "admin", admin, http.StatusOK},
		{"POST", "/api/v1/products", productBody(program, []string{tryout.ID.String()}, nil), "anonymous", "", http.StatusUnauthorized},
		{"POST", "/api/v1/products", productBody(program, []string{tryout.ID.String()}, nil), "student", student, http.StatusForbidden},
		{"POST", "/api/v1/products", productBody(program, []string{tryout.ID.String()}, nil), "operator", operator, http.StatusCreated},
		{"POST", "/api/v1/products", productBody(program, []string{tryout.ID.String()}, nil), "admin", admin, http.StatusCreated},
		{"PUT", path, update("Judul operator"), "anonymous", "", http.StatusUnauthorized},
		{"PUT", path, update("Judul student"), "student", student, http.StatusForbidden},
		{"PUT", "/api/v1/products/" + operatorPut.ID.String(), update("Judul operator"), "operator", operator, http.StatusOK},
		{"PUT", "/api/v1/products/" + adminPut.ID.String(), update("Judul admin"), "admin", admin, http.StatusOK},
		{"DELETE", "/api/v1/products/" + operatorDelete.ID.String(), nil, "anonymous", "", http.StatusUnauthorized},
		{"DELETE", "/api/v1/products/" + operatorDelete.ID.String(), nil, "student", student, http.StatusForbidden},
		{"DELETE", "/api/v1/products/" + operatorDelete.ID.String(), nil, "operator", operator, http.StatusNoContent},
		{"DELETE", "/api/v1/products/" + adminDelete.ID.String(), nil, "admin", admin, http.StatusNoContent},
	}
	for _, tc := range cases {
		rec := sendProductJSON(e, tc.method, host, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s as %s: status = %d body = %s, want %d", tc.method, tc.path, tc.who, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestProductResponsesAreNoStore(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	tryout := publishedTryout(t, e, host, admin, program)
	created := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))
	path := "/api/v1/products/" + created.ID.String()

	invalid := productBody(program, []string{tryout.ID.String()}, nil)
	invalid["price"] = "1e3"

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"list", getWithToken(e, host, "/api/v1/products", admin)},
		{"list without a token", getWithToken(e, host, "/api/v1/products", "")},
		{"get", getWithToken(e, host, path, admin)},
		{"create", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", productBody(program, []string{tryout.ID.String()}, nil), admin)},
		{"create rejected", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", invalid, admin)},
		{"update", sendProductJSON(e, http.MethodPut, host, path, productBody(program, []string{tryout.ID.String()}, nil), admin)},
		{"delete missing", sendProductJSON(e, http.MethodDelete, host, "/api/v1/products/"+uuid.NewString(), nil, admin)},
		{"catalog list", getWithToken(e, host, "/api/v1/catalog/products", "")},
		{"catalog detail", getWithToken(e, host, "/api/v1/catalog/products/"+created.ID.String(), "")},
		{"catalog summary", getWithToken(e, host, "/api/v1/catalog/summary", "")},
		{"catalog programs", getWithToken(e, host, "/api/v1/catalog/programs", "")},
	}
	for _, tc := range cases {
		if got := tc.rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", tc.name, got)
		}
	}
}

func TestProductErrorEnvelope(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	tryout := publishedTryout(t, e, host, admin, program)
	createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))

	unknownField := productBody(program, []string{tryout.ID.String()}, map[string]any{"cover_from": "#fff"})
	wantError(t, "unknown field", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", unknownField, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "body")

	badPrice := productBody(program, []string{tryout.ID.String()}, map[string]any{"price": "1e3"})
	wantError(t, "price exponent", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", badPrice, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "price")

	emptyTitle := productBody(program, []string{tryout.ID.String()}, map[string]any{"title": "   "})
	wantError(t, "empty title", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", emptyTitle, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "title")

	missingProgram := productBody(program, []string{tryout.ID.String()}, map[string]any{"program_id": uuid.NewString()})
	wantError(t, "unknown program", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", missingProgram, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "program_id")

	twoTryouts := productBody(program, []string{tryout.ID.String(), uuid.NewString()}, map[string]any{"type": model.ProductIndividual})
	wantError(t, "individual with two tryouts", sendProductJSON(e, http.MethodPost, host, "/api/v1/products", twoTryouts, admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "tryout_ids")

	wantError(t, "another tenant's id shape", getWithToken(e, host, "/api/v1/products/"+uuid.NewString(), admin),
		http.StatusNotFound, httpx.CodeNotFound)

	if rec := getWithToken(e, host, "/api/v1/products/"+uuid.NewString(), admin); !strings.Contains(rec.Body.String(), productNotFound) {
		t.Errorf("404 message = %s, want it to say the product is gone", rec.Body.String())
	}

	wantError(t, "bad limit", getWithToken(e, host, "/api/v1/products?limit=abc", admin),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "limit")

	hidden := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, map[string]any{"is_active": false}))
	if rec := getWithToken(e, host, "/api/v1/catalog/products/"+hidden.ID.String(), ""); rec.Code != http.StatusNotFound {
		t.Errorf("catalog detail of an inactive product: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
}

func TestProductBodyIsBoundedBeforeItIsMapped(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	program := questionCurriculum(t, e, host, admin)

	item := `"` + uuid.NewString() + `",`
	body := `{"type":"bundle","title":"x","program_id":"` + program.ID.String() +
		`","price":"1000","tryout_ids":[` + strings.Repeat(item, 60000) + `"` + uuid.NewString() + `"]}`
	if len(body) < 1<<20 {
		t.Fatalf("the probe body is only %d bytes, so it cannot prove the cap", len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+admin)
	req.Host = host
	rec := serve(e, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized body: status = %d body = %s, want 422", rec.Code, rec.Body.String())
	}
	if got := len(errorEnvelope(t, rec).Error.Fields); got > 2 {
		t.Fatalf("oversized body produced %d field errors, want it refused before it is mapped", got)
	}
}

func TestCatalogIsPublicAndTenantResolved(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	programA := questionCurriculum(t, e, hostA, adminA)
	tryoutA := publishedTryout(t, e, hostA, adminA, programA)
	productA := createProduct(t, e, hostA, adminA, productBody(programA, []string{tryoutA.ID.String()}, nil))

	programB := questionCurriculum(t, e, hostB, adminB)
	tryoutB := publishedTryout(t, e, hostB, adminB, programB)
	productB := createProduct(t, e, hostB, adminB, productBody(programB, []string{tryoutB.ID.String()}, nil))

	// Sequential A/B/A/B anonymous reads: never a bleed in either direction.
	for round := 0; round < 2; round++ {
		for _, tc := range []struct {
			host      string
			slug      string
			productID string
			otherID   string
			code      string
		}{
			{hostA, tenantA.Slug, productA.ID.String(), productB.ID.String(), programA.Code},
			{hostB, tenantB.Slug, productB.ID.String(), productA.ID.String(), programB.Code},
		} {
			rec := getWithToken(e, tc.host, "/api/v1/catalog/products", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("anonymous catalog on %s: status = %d body = %s", tc.host, rec.Code, rec.Body.String())
			}
			page := decodePublicProducts(t, rec)
			if page.Total != 1 || len(page.Products) != 1 || page.Products[0].ID.String() != tc.productID {
				t.Fatalf("catalog on %s = %s, want only %s", tc.host, rec.Body.String(), tc.productID)
			}
			if page.Products[0].ProgramCode != tc.code || page.Products[0].ProgramName == "" {
				t.Errorf("catalog on %s carries program %q/%q, want its own branding",
					tc.host, page.Products[0].ProgramCode, page.Products[0].ProgramName)
			}
			if strings.Contains(rec.Body.String(), tc.otherID) {
				t.Errorf("catalog on %s carries the other tenant's product id", tc.host)
			}

			detail := getWithToken(e, tc.host, "/api/v1/catalog/products/"+tc.productID, "")
			if detail.Code != http.StatusOK {
				t.Errorf("anonymous detail on %s: status = %d body = %s", tc.host, detail.Code, detail.Body.String())
			}
			if rec := getWithToken(e, tc.host, "/api/v1/catalog/products/"+tc.otherID, ""); rec.Code != http.StatusNotFound {
				t.Errorf("detail of the other tenant's product on %s: status = %d, want 404", tc.host, rec.Code)
			}
			if summary := getWithToken(e, tc.host, "/api/v1/catalog/summary", ""); summary.Code != http.StatusOK {
				t.Errorf("anonymous summary on %s: status = %d body = %s", tc.host, summary.Code, summary.Body.String())
			}
		}
	}

	// No usable tenant: the same 404 as GET /tenant, for a missing and an inactive configured tenant.
	for _, tc := range []struct{ name, host string }{
		{"missing tenant", f.host(f.missingSlug)},
		{"inactive tenant", f.host(f.inactive.Slug)},
	} {
		for _, path := range []string{"/api/v1/catalog/products", "/api/v1/catalog/summary", "/api/v1/catalog/programs"} {
			rec := getWithToken(e, tc.host, path, "")
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s: status = %d body = %s, want 404", tc.name, path, rec.Code, rec.Body.String())
				continue
			}
			if code := errorEnvelope(t, rec).Error.Code; code != httpx.CodeNotFound {
				t.Errorf("%s %s: code = %q, want %q", tc.name, path, code, httpx.CodeNotFound)
			}
		}
	}
}

func TestCatalogNeverLeaks(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenantA, tenantB := f.active[0], f.active[1]
	hostA, hostB := f.host(tenantA.Slug), f.host(tenantB.Slug)
	adminA := curriculumToken(t, f, tenantA, model.RoleAdmin)
	adminB := curriculumToken(t, f, tenantB, model.RoleAdmin)

	programA := questionCurriculum(t, e, hostA, adminA)
	tryoutA := publishedTryout(t, e, hostA, adminA, programA)
	shown := createProduct(t, e, hostA, adminA, productBody(programA, []string{tryoutA.ID.String()}, nil))
	doomed := publishedTryout(t, e, hostA, adminA, programA)
	hidden := createProduct(t, e, hostA, adminA,
		productBody(programA, []string{doomed.ID.String()}, map[string]any{"title": "Paket Tersembunyi"}))
	if rec := sendTryoutJSON(e, http.MethodPost, hostA, "/api/v1/tryouts/"+doomed.ID.String()+"/unpublish", nil, adminA); rec.Code != http.StatusOK {
		t.Fatalf("unpublish the attached tryout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	inactive := createProduct(t, e, hostA, adminA,
		productBody(programA, []string{tryoutA.ID.String()}, map[string]any{"is_active": false, "title": "Paket Nonaktif"}))

	programB := questionCurriculum(t, e, hostB, adminB)
	tryoutB := publishedTryout(t, e, hostB, adminB, programB)
	foreign := createProduct(t, e, hostB, adminB, productBody(programB, []string{tryoutB.ID.String()}, nil))

	listBody := getWithToken(e, hostA, "/api/v1/catalog/products", "").Body.String()
	detailBody := getWithToken(e, hostA, "/api/v1/catalog/products/"+shown.ID.String(), "").Body.String()
	summaryBody := getWithToken(e, hostA, "/api/v1/catalog/summary", "").Body.String()
	errorBody := getWithToken(e, hostA, "/api/v1/catalog/products/"+uuid.NewString(), "").Body.String()

	if !strings.Contains(listBody, shown.ID.String()) {
		t.Fatalf("the catalog hides the product under test, so the check proves nothing: %s", listBody)
	}
	for name, body := range map[string]string{
		"list": listBody, "detail": detailBody, "summary": summaryBody, "error": errorBody,
	} {
		for _, leaked := range []string{`"correct"`, `"options"`, `"explanation"`, `"content"`, `"created_by"`,
			`"revision"`, `"first_attempt_at"`, `"created_at"`, `"updated_at"`, `"tenant_id"`} {
			if strings.Contains(body, leaked) {
				t.Errorf("%s carries %s: %s", name, leaked, body)
			}
		}
	}
	for name, body := range map[string]string{"list": listBody, "detail": detailBody, "summary": summaryBody} {
		for _, absent := range []string{hidden.ID.String(), inactive.ID.String(), foreign.ID.String(), "Paket Tersembunyi", "Paket Nonaktif"} {
			if strings.Contains(body, absent) {
				t.Errorf("%s carries %q, which is not publicly visible", name, absent)
			}
		}
	}
	if strings.Contains(listBody, tenantA.ID.String()) || strings.Contains(summaryBody, tenantA.ID.String()) {
		t.Error("a public body carries the tenant id")
	}
	if product := decodePublicProducts(t, getWithToken(e, hostA, "/api/v1/catalog/products", "")); len(product.Products) != 1 {
		t.Errorf("public list = %d products, want only the one visible product", len(product.Products))
	}
}

func TestCatalogHidesDeactivatedProductImmediately(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	tryout := publishedTryout(t, e, host, admin, program)
	created := createProduct(t, e, host, admin, productBody(program, []string{tryout.ID.String()}, nil))

	if page := decodePublicProducts(t, getWithToken(e, host, "/api/v1/catalog/products", "")); len(page.Products) != 1 {
		t.Fatalf("public list before deactivation = %d products, want 1", len(page.Products))
	}

	off := productBody(program, []string{tryout.ID.String()}, map[string]any{"is_active": false, "title": created.Title})
	if rec := sendProductJSON(e, http.MethodPut, host, "/api/v1/products/"+created.ID.String(), off, admin); rec.Code != http.StatusOK {
		t.Fatalf("deactivate: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}

	if page := decodePublicProducts(t, getWithToken(e, host, "/api/v1/catalog/products", "")); len(page.Products) != 0 {
		t.Errorf("public list right after deactivation = %d products, want 0 without a restart", len(page.Products))
	}
	if rec := getWithToken(e, host, "/api/v1/catalog/products/"+created.ID.String(), ""); rec.Code != http.StatusNotFound {
		t.Errorf("public detail right after deactivation: status = %d, want 404", rec.Code)
	}
	if summary := decodeSummary(t, getWithToken(e, host, "/api/v1/catalog/summary", "")); summary.Products != 0 {
		t.Errorf("summary still counts %d products after deactivation", summary.Products)
	}
}

func TestCatalogSummaryCountsOnlyVisible(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)
	shown := publishedTryout(t, e, host, admin, program)
	alsoOpen := publishedTryout(t, e, host, admin, program)
	if rec := sendTryoutJSON(e, http.MethodPost, host, "/api/v1/tryouts",
		tryoutBody(program, createTryoutQuestion(t, e, host, admin, program).ID.String()), admin); rec.Code != http.StatusCreated {
		t.Fatalf("create a draft tryout: status = %d body = %s", rec.Code, rec.Body.String())
	}

	createProduct(t, e, host, admin, productBody(program, []string{shown.ID.String()}, nil))
	createProduct(t, e, host, admin,
		productBody(program, []string{alsoOpen.ID.String()}, map[string]any{"is_active": false, "title": "Paket Nonaktif"}))

	summary := decodeSummary(t, getWithToken(e, host, "/api/v1/catalog/summary", ""))
	if summary.Programs != 1 {
		t.Errorf("programs = %d, want 1", summary.Programs)
	}
	if summary.Products != 1 {
		t.Errorf("products = %d, want only the visible one", summary.Products)
	}
	if summary.Tryouts != 2 {
		t.Errorf("tryouts = %d, want the two published open tryouts and not the draft", summary.Tryouts)
	}
}

func TestCatalogProgramsIsPublicAndFlat(t *testing.T) {
	f := newAuthFixture(t, false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)

	program := questionCurriculum(t, e, host, admin)

	rec := getWithToken(e, host, "/api/v1/catalog/programs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous programs: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"subjects"`) {
		t.Errorf("public programs carry the subject tree: %s", rec.Body.String())
	}
	var body publicProgramsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a program list: %v", rec.Body.String(), err)
	}
	if len(body.Programs) != 1 {
		t.Fatalf("programs = %d, want 1", len(body.Programs))
	}
	got := body.Programs[0]
	if got.ID != program.ID || got.Code != program.Code || got.Name != program.Name || got.Description != program.Description {
		t.Errorf("program = %+v, want the created %s", got, program.Code)
	}
}
