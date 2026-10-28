package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestKecermatanProductCheckoutOverHTTPAddsCredits(t *testing.T) {
	f := newKecHTTPFixture(t)
	programID := f.test.ProgramID.String()
	body := map[string]any{
		"type": "kecermatan", "title": "Paket Kecermatan " + uuid.NewString()[:6], "description": "3 attempt", "program_id": programID,
		"price": "20000.00", "is_active": true, "kecermatan_test_id": f.test.ID.String(), "included_attempts": 3,
	}
	rec := f.call(http.MethodPost, "/api/v1/products", body, f.tokenAdminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create kecermatan product = %d %s, want 201", rec.Code, rec.Body.String())
	}
	product := decodeInto[productResponse](t, rec)
	if product.Type != "kecermatan" || product.IncludedAttempts == nil || *product.IncludedAttempts != 3 || len(product.Tryouts) != 0 {
		t.Fatalf("product = %+v, want a kecermatan product of 3 attempts", product)
	}
	body["included_attempts"] = 0
	body["title"] = "Paket Nol"
	if rec := f.call(http.MethodPost, "/api/v1/products", body, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("zero attempts = %d, want 422", rec.Code)
	}

	catalog := decodeInto[publicProductsResponse](t, f.call(http.MethodGet, "/api/v1/catalog/products?type=kecermatan", nil, ""))
	if catalog.Total != 1 || catalog.Products[0].Kecermatan == nil || catalog.Products[0].Kecermatan.KolomCount != 10 {
		t.Fatalf("catalog = %+v, want the kecermatan product with its structure", catalog)
	}
	assertNoSecretKeys(t, "catalog", mustBody(t, f.call(http.MethodGet, "/api/v1/catalog/products?type=kecermatan", nil, "")),
		"seed", "generation_seed", "missing", "columns")

	if rec := f.call(http.MethodPost, "/api/v1/students/"+f.studentA.ID.String()+"/wallet/adjustments",
		adjustmentBody("credit", "50000.00", "Saldo uji", "kec-"+uuid.NewString()[:8]), f.tokenAdminA); rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("fund wallet = %d %s", rec.Code, rec.Body.String())
	}
	checkout := map[string]any{
		"product_id": product.ID.String(), "payment_method": "wallet", "expected_price": "20000.00",
		"idempotency_key": "kec-order-" + uuid.NewString()[:8],
	}
	rec = f.call(http.MethodPost, "/api/v1/orders", checkout, f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("checkout = %d %s, want 201", rec.Code, rec.Body.String())
	}
	order := decodeInto[orderResponse](t, rec)
	if order.Status != "confirmed" || order.IncludedAttempts == nil || *order.IncludedAttempts != 3 || order.KecermatanTestID == nil {
		t.Errorf("order = %+v, want a confirmed kecermatan order", order)
	}
	if rec := f.call(http.MethodPost, "/api/v1/orders", checkout, f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("replayed checkout = %d, want 200", rec.Code)
	}
	tests := decodeInto[kecStudentTestsResponse](t, f.call(http.MethodGet, "/api/v1/my/kecermatan-tests", nil, f.tokenA))
	if len(tests.Tests) != 1 || tests.Tests[0].AttemptsPurchased != 3 || tests.Tests[0].AttemptsRemaining != 3 {
		t.Errorf("my tests = %+v, want 3 attempts after a purchase and its replay", tests.Tests)
	}
	// The tryout enrollments list never mixes in a kecermatan row.
	if enrolled := decodeInto[enrollmentsResponse](t, f.call(http.MethodGet, "/api/v1/enrollments", nil, f.tokenA)); len(enrolled.Enrollments) != 0 {
		t.Errorf("enrollments = %+v, want only tryout holdings", enrolled.Enrollments)
	}
}

func mustBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
