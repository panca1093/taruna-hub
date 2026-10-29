package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// newTopupService is the real service over the fixture database; it panics because a fixture cannot recover.
func newTopupService(f tenantFixture, assets *service.AssetService) *service.TopupService {
	notifications, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil,
		handlerSealKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return service.NewTopupService(f.pool, repository.NewTopupRepository(f.pool), newWalletService(f), assets, notifications)
}

func decodeTopup(t *testing.T, rec *httptest.ResponseRecorder) topupResponse {
	t.Helper()
	var body topupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a top-up: %v", rec.Body.String(), err)
	}
	return body
}

func decodeTopups(t *testing.T, rec *httptest.ResponseRecorder) topupsResponse {
	t.Helper()
	var body topupsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a top-up list: %v", rec.Body.String(), err)
	}
	return body
}

type topupHTTPFixture struct {
	assetHTTPFixture
	e http.Handler
}

func newTopupHTTPFixture(t *testing.T) topupHTTPFixture {
	t.Helper()
	f := topupHTTPFixture{assetHTTPFixture: newAssetHTTPFixture(t)}
	f.e = f.server(t, time.Minute)
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), `DELETE FROM tenant_settings WHERE tenant_id = ANY($1)`,
			[]uuid.UUID{f.tenantA.ID, f.tenantB.ID}); err != nil {
			t.Errorf("cleanup tenant settings: %v", err)
		}
	})
	return f
}

func (f topupHTTPFixture) post(path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, body, token)
}

func (f topupHTTPFixture) get(path, token string) *httptest.ResponseRecorder {
	return getWithToken(f.e, f.hostA, path, token)
}

func (f topupHTTPFixture) request(t *testing.T, token, amount string) topupResponse {
	t.Helper()
	proof := f.uploadPNG(t, f.e, f.hostA, token, "payment_proof")
	rec := f.post("/api/v1/topups", topupBody(amount, proof.ID.String(), nil), token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create top-up: status = %d body = %s", rec.Code, rec.Body.String())
	}
	return decodeTopup(t, rec)
}

func topupBody(amount, proofID string, extra map[string]any) map[string]any {
	body := map[string]any{"amount": amount, "idempotency_key": "key-" + uuid.NewString()[:12], "proof_asset_id": proofID}
	for key, value := range extra {
		body[key] = value
	}
	return body
}

func TestTopupEndpointsRolesAndOwnership(t *testing.T) {
	f := newTopupHTTPFixture(t)
	proof := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof")
	create := topupBody("50000", proof.ID.String(), nil)

	if rec := f.post("/api/v1/topups", create, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous create: status = %d, want 401", rec.Code)
	}
	if rec := f.get("/api/v1/topups", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous list: status = %d, want 401", rec.Code)
	}
	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		if rec := f.post("/api/v1/topups", create, token); rec.Code != http.StatusForbidden {
			t.Errorf("%s create: status = %d, want 403", name, rec.Code)
		}
		if rec := f.get("/api/v1/topups", token); rec.Code != http.StatusForbidden {
			t.Errorf("%s student list: status = %d, want 403", name, rec.Code)
		}
	}

	rec := f.post("/api/v1/topups", create, f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d body = %s", rec.Code, rec.Body.String())
	}
	topup := decodeTopup(t, rec)
	if topup.Status != "pending" || topup.Method != "manual" || topup.Amount != "50000.00" ||
		!strings.HasPrefix(topup.Code, "TUP-") || topup.StudentID != f.studentA.ID || topup.ProofAssetID == nil {
		t.Fatalf("top-up = %+v, want a pending manual request of student A with its proof", topup)
	}
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cache)
	}
	if wallet := decodeWallet(t, f.get("/api/v1/wallet", f.tokenA)); wallet.Balance != "0.00" {
		t.Errorf("balance after a pending request = %s, want 0.00", wallet.Balance)
	}

	again := f.post("/api/v1/topups", create, f.tokenA)
	if again.Code != http.StatusOK || decodeTopup(t, again).ID != topup.ID {
		t.Errorf("retry: status = %d body = %s, want 200 with the same request", again.Code, again.Body.String())
	}
	changed := topupBody("60000", proof.ID.String(), map[string]any{"idempotency_key": create["idempotency_key"]})
	if rec := f.post("/api/v1/topups", changed, f.tokenA); rec.Code != http.StatusConflict {
		t.Errorf("same key, other amount: status = %d, want 409", rec.Code)
	}

	// Student B sees nothing of student A's, and a malformed id is a plain 404.
	if list := decodeTopups(t, f.get("/api/v1/topups", f.tokenSecond)); list.Total != 0 || len(list.Topups) != 0 {
		t.Errorf("student B lists %d requests, want 0", list.Total)
	}
	if rec := f.get("/api/v1/topups/"+topup.ID.String(), f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("student B reads A's request: status = %d, want 404", rec.Code)
	}
	if rec := f.get("/api/v1/topups/not-a-uuid", f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed id: status = %d, want 404", rec.Code)
	}
	if rec := f.get("/api/v1/topups/"+topup.ID.String(), f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("student A reads own request: status = %d, want 200", rec.Code)
	}
	if list := decodeTopups(t, f.get("/api/v1/topups", f.tokenA)); list.Total != 1 || list.MinTopup != "10000.00" {
		t.Errorf("student A list = %+v, want one request and the default minimum", list)
	}

	// The staff routes are closed to students and anonymous callers.
	for _, path := range []string{"/api/v1/admin/topups", "/api/v1/admin/topups/" + topup.ID.String()} {
		if rec := f.get(path, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student GET %s: status = %d, want 403", path, rec.Code)
		}
		if rec := f.get(path, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s: status = %d, want 401", path, rec.Code)
		}
	}
	for _, action := range []string{"approve", "reject"} {
		path := "/api/v1/admin/topups/" + topup.ID.String() + "/" + action
		if rec := f.post(path, map[string]string{"reason": "x"}, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student %s: status = %d, want 403", action, rec.Code)
		}
	}
}

func TestTopupProofAccess(t *testing.T) {
	f := newTopupHTTPFixture(t)
	topup := f.request(t, f.tokenA, "50000")
	url := "/api/v1/assets/" + topup.ProofAssetID.String() + "/url"

	for name, token := range map[string]string{"owner": f.tokenA, "admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		if rec := f.get(url, token); rec.Code != http.StatusOK {
			t.Errorf("%s reads the proof: status = %d body = %s, want 200", name, rec.Code, rec.Body.String())
		}
	}
	if rec := f.get(url, f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("another student reads the proof: status = %d, want 404", rec.Code)
	}

	// Another student's upload cannot back my request, and neither can a used one.
	foreign := f.uploadPNG(t, f.e, f.hostA, f.tokenSecond, "payment_proof")
	if rec := f.post("/api/v1/topups", topupBody("50000", foreign.ID.String(), nil), f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("foreign proof: status = %d, want 422", rec.Code)
	}
	if rec := f.post("/api/v1/topups", topupBody("50000", topup.ProofAssetID.String(), nil), f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("reused proof: status = %d, want 422", rec.Code)
	}
}

func TestTopupCreateValidationAndMinimum(t *testing.T) {
	f := newTopupHTTPFixture(t)
	proof := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof").ID.String()

	cases := []struct {
		name   string
		body   map[string]any
		field  string
		status int
	}{
		{"below the default minimum", topupBody("9999", proof, nil), "amount", http.StatusUnprocessableEntity},
		{"fraction", topupBody("50000.50", proof, nil), "amount", http.StatusUnprocessableEntity},
		{"zero", topupBody("0", proof, nil), "amount", http.StatusUnprocessableEntity},
		{"negative", topupBody("-50000", proof, nil), "amount", http.StatusUnprocessableEntity},
		{"exponent", topupBody("5e4", proof, nil), "amount", http.StatusUnprocessableEntity},
		{"short key", topupBody("50000", proof, map[string]any{"idempotency_key": "x"}), "idempotency_key", http.StatusUnprocessableEntity},
		{"no proof", map[string]any{"amount": "50000", "idempotency_key": "key-" + uuid.NewString()[:12]}, "proof_asset_id", http.StatusUnprocessableEntity},
		{"malformed proof", topupBody("50000", "nope", nil), "proof_asset_id", http.StatusUnprocessableEntity},
		{"unknown field", topupBody("50000", proof, map[string]any{"bogus": "gateway"}), "body", http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.post("/api/v1/topups", tc.body, f.tokenA)
			if rec.Code != tc.status {
				t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), tc.status)
			}
			if body := errorBody(t, rec); body.Fields[tc.field] == "" {
				t.Errorf("fields = %v, want a reason on %q", body.Fields, tc.field)
			}
		})
	}

	// The Admin minimum is what the server enforces, and the student page can read it.
	settings := settingsBody{ScoringWeights: weightsBody{Easy: 1, Medium: 2, Hard: 3}, MinTopup: "25000"}
	if rec := sendTryoutJSON(f.e, http.MethodPut, f.hostA, "/api/v1/admin/settings", settings, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Fatalf("save settings: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if list := decodeTopups(t, f.get("/api/v1/topups", f.tokenA)); list.MinTopup != "25000.00" {
		t.Errorf("student minimum = %s, want 25000.00", list.MinTopup)
	}
	if rec := f.post("/api/v1/topups", topupBody("24999", proof, nil), f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("below the saved minimum: status = %d, want 422", rec.Code)
	}
	if rec := f.post("/api/v1/topups", topupBody("25000", proof, nil), f.tokenA); rec.Code != http.StatusCreated {
		t.Errorf("at the saved minimum: status = %d body = %s, want 201", rec.Code, rec.Body.String())
	}
	if rec := sendTryoutJSON(f.e, http.MethodPut, f.hostA, "/api/v1/admin/settings", settings, f.tokenOperatorA); rec.Code != http.StatusForbidden {
		t.Errorf("operator saving the minimum: status = %d, want 403", rec.Code)
	}
}

func TestTopupReviewEndpoints(t *testing.T) {
	f := newTopupHTTPFixture(t)
	approved := f.request(t, f.tokenA, "50000")
	rejected := f.request(t, f.tokenA, "30000")
	untouched := f.request(t, f.tokenA, "20000")
	approve := func(id uuid.UUID, token string) *httptest.ResponseRecorder {
		return f.post("/api/v1/admin/topups/"+id.String()+"/approve", nil, token)
	}
	reject := func(id uuid.UUID, reason string, token string) *httptest.ResponseRecorder {
		return f.post("/api/v1/admin/topups/"+id.String()+"/reject", map[string]string{"reason": reason}, token)
	}

	if list := decodeTopups(t, f.get("/api/v1/admin/topups?status=pending", f.tokenOperatorA)); list.Total != 3 {
		t.Errorf("pending list total = %d, want 3", list.Total)
	}

	rec := approve(approved.ID, f.tokenOperatorA)
	if rec.Code != http.StatusOK || decodeTopup(t, rec).Status != "approved" {
		t.Fatalf("approve: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if again := approve(approved.ID, f.tokenAdminA); again.Code != http.StatusOK || decodeTopup(t, again).Status != "approved" {
		t.Errorf("repeated approve: status = %d body = %s, want 200", again.Code, again.Body.String())
	}
	if rec := reject(approved.ID, "salah", f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("reject an approved request: status = %d, want 409", rec.Code)
	}
	if wallet := decodeWallet(t, f.get("/api/v1/wallet", f.tokenA)); wallet.Balance != "50000.00" {
		t.Errorf("balance = %s, want one credit of 50000.00", wallet.Balance)
	}
	entries := decodeWalletEntries(t, f.get("/api/v1/wallet/entries", f.tokenA))
	if entries.Total != 1 || entries.Entries[0].Kind != "topup" || entries.Entries[0].SourceType != "topup" ||
		entries.Entries[0].SourceID != approved.ID.String() || entries.Entries[0].Amount != "50000.00" {
		t.Errorf("ledger = %+v, want one topup entry referencing the request", entries)
	}

	if rec := reject(rejected.ID, "  ", f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["reason"] == "" {
		t.Errorf("reject without a reason: status = %d body = %s, want 422 on reason", rec.Code, rec.Body.String())
	}
	if rec := reject(rejected.ID, strings.Repeat("a", 501), f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("reject with a long reason: status = %d, want 422", rec.Code)
	}
	rec = reject(rejected.ID, "Nominal tidak sesuai", f.tokenOperatorA)
	if got := decodeTopup(t, rec); rec.Code != http.StatusOK || got.Status != "rejected" || got.RejectReason != "Nominal tidak sesuai" {
		t.Fatalf("reject: status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec := approve(rejected.ID, f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("approve a rejected request: status = %d, want 409", rec.Code)
	}
	if wallet := decodeWallet(t, f.get("/api/v1/wallet", f.tokenA)); wallet.Balance != "50000.00" {
		t.Errorf("balance after a rejection = %s, want it unchanged", wallet.Balance)
	}

	// The student sees the outcome and the reason on their own list.
	byID := map[uuid.UUID]topupResponse{}
	for _, item := range decodeTopups(t, f.get("/api/v1/topups", f.tokenA)).Topups {
		byID[item.ID] = item
	}
	if byID[approved.ID].Status != "approved" || byID[rejected.ID].RejectReason != "Nominal tidak sesuai" || byID[untouched.ID].Status != "pending" {
		t.Errorf("student list = %+v, want approved, rejected with its reason and a pending one", byID)
	}

	// Staff filter by status and search by code; a bad filter is a 422.
	if list := decodeTopups(t, f.get("/api/v1/admin/topups?status=pending", f.tokenAdminA)); list.Total != 1 || list.Topups[0].ID != untouched.ID {
		t.Errorf("pending filter = %+v, want only the untouched request", list)
	}
	if list := decodeTopups(t, f.get("/api/v1/admin/topups?q="+rejected.Code, f.tokenAdminA)); list.Total != 1 || list.Topups[0].ID != rejected.ID {
		t.Errorf("search by code = %+v, want the rejected request", list)
	}
	if rec := f.get("/api/v1/admin/topups?status=paid", f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad status filter: status = %d, want 422", rec.Code)
	}
	if rec := f.get("/api/v1/admin/topups/"+uuid.NewString(), f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown request: status = %d, want 404", rec.Code)
	}
}
