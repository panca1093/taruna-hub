package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

// newOrderService is the real service over the fixture database; it panics because a fixture cannot recover.
func newOrderService(f tenantFixture, assets *service.AssetService) *service.OrderService {
	notifications, err := service.NewNotificationService(repository.NewNotificationRepository(f.pool), noJobs{}, nil,
		handlerSealKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return service.NewOrderService(f.pool, repository.NewOrderRepository(f.pool), repository.NewProductRepository(f.pool),
		newWalletService(f), assets, notifications,
		service.BankAccount{Bank: "Bank Uji", Number: "1234567890", Holder: "Taruna Eduka"})
}

func decodeOrder(t *testing.T, rec *httptest.ResponseRecorder) orderResponse {
	t.Helper()
	var body orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an order: %v", rec.Body.String(), err)
	}
	return body
}

func decodeOrders(t *testing.T, rec *httptest.ResponseRecorder) ordersResponse {
	t.Helper()
	var body ordersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an order list: %v", rec.Body.String(), err)
	}
	return body
}

func decodeEnrollments(t *testing.T, rec *httptest.ResponseRecorder) enrollmentsResponse {
	t.Helper()
	var body enrollmentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an enrollment list: %v", rec.Body.String(), err)
	}
	return body
}

func checkoutBody(productID, method string, extra map[string]any) map[string]any {
	body := map[string]any{
		"product_id": productID, "payment_method": method, "idempotency_key": "key-" + uuid.NewString()[:12], "expected_price": "100000.00",
	}
	for key, value := range extra {
		body[key] = value
	}
	return body
}

type orderHTTPFixture struct {
	assetHTTPFixture
	e       http.Handler
	program programResponse
	product productResponse
}

func newOrderHTTPFixture(t *testing.T) orderHTTPFixture {
	t.Helper()
	f := orderHTTPFixture{assetHTTPFixture: newAssetHTTPFixture(t)}
	f.e = f.server(t, time.Minute)
	f.program = questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	tryout := publishedTryout(t, f.e, f.hostA, f.tokenAdminA, f.program)
	f.product = createProduct(t, f.e, f.hostA, f.tokenAdminA,
		productBody(f.program, []string{tryout.ID.String()}, map[string]any{"price": "100000.00"}))
	return f
}

func (f orderHTTPFixture) fund(t *testing.T, student model.User, amount string) {
	t.Helper()
	rec := sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/students/"+student.ID.String()+"/wallet/adjustments",
		adjustmentBody("credit", amount, "Saldo uji", "fund-"+uuid.NewString()[:8]), f.tokenAdminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("fund student: status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func (f orderHTTPFixture) post(path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, body, token)
}

func (f orderHTTPFixture) get(path, token string) *httptest.ResponseRecorder {
	return getWithToken(f.e, f.hostA, path, token)
}

func TestWalletCheckoutEndpointRolesAndOwnership(t *testing.T) {
	f := newOrderHTTPFixture(t)
	f.fund(t, f.studentA, "250000.00")
	productID := f.product.ID.String()

	if rec := f.post("/api/v1/orders", checkoutBody(productID, "wallet", nil), ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous checkout: status = %d, want 401", rec.Code)
	}
	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		if rec := f.post("/api/v1/orders", checkoutBody(productID, "wallet", nil), token); rec.Code != http.StatusForbidden {
			t.Errorf("%s checkout: status = %d, want 403", name, rec.Code)
		}
		if rec := f.get("/api/v1/enrollments", token); rec.Code != http.StatusForbidden {
			t.Errorf("%s enrollments: status = %d, want 403", name, rec.Code)
		}
	}
	if rec := f.get("/api/v1/enrollments", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous enrollments: status = %d, want 401", rec.Code)
	}
	if rec := f.get("/api/v1/admin/orders", f.tokenA); rec.Code != http.StatusForbidden {
		t.Errorf("student on the staff list: status = %d, want 403", rec.Code)
	}

	body := checkoutBody(productID, "wallet", map[string]any{"idempotency_key": "retry-key-0001"})
	rec := f.post("/api/v1/orders", body, f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("wallet checkout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	order := decodeOrder(t, rec)
	if order.Status != "confirmed" || order.PaymentMethod != "wallet" || order.Price != "100000.00" ||
		!strings.HasPrefix(order.Code, "ORD-") || order.StudentID != f.studentA.ID {
		t.Fatalf("order = %+v, want a confirmed wallet order of student A", order)
	}
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cache)
	}

	again := f.post("/api/v1/orders", body, f.tokenA)
	if again.Code != http.StatusOK || decodeOrder(t, again).ID != order.ID {
		t.Errorf("retry: status = %d body = %s, want 200 with the same order", again.Code, again.Body.String())
	}
	if wallet := decodeWallet(t, f.get("/api/v1/wallet", f.tokenA)); wallet.Balance != "150000.00" {
		t.Errorf("balance = %s, want one debit", wallet.Balance)
	}

	owned := f.post("/api/v1/orders", checkoutBody(productID, "wallet", nil), f.tokenA)
	if owned.Code != http.StatusConflict || errorBody(t, owned).Code != httpx.CodeAlreadyOwned {
		t.Errorf("repurchase: status = %d body = %s, want 409 already_owned", owned.Code, owned.Body.String())
	}

	poor := f.post("/api/v1/orders", checkoutBody(productID, "wallet", nil), f.tokenSecond)
	if poor.Code != http.StatusUnprocessableEntity || errorBody(t, poor).Code != httpx.CodeInsufficientBalance {
		t.Errorf("no balance: status = %d body = %s, want 422 insufficient_balance", poor.Code, poor.Body.String())
	}

	// Student B sees nothing of student A's.
	if list := decodeOrders(t, f.get("/api/v1/orders", f.tokenSecond)); list.Total != 0 {
		t.Errorf("student B lists %d orders, want 0", list.Total)
	}
	if rec := f.get("/api/v1/orders/"+order.ID.String(), f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("student B reads A's order: status = %d, want 404", rec.Code)
	}
	if rec := f.get("/api/v1/orders/not-a-uuid", f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed order id: status = %d, want 404", rec.Code)
	}
	if holds := decodeEnrollments(t, f.get("/api/v1/enrollments", f.tokenSecond)); len(holds.Enrollments) != 0 {
		t.Errorf("student B holds %d enrollments, want 0", len(holds.Enrollments))
	}
	holds := decodeEnrollments(t, f.get("/api/v1/enrollments", f.tokenA))
	if len(holds.Enrollments) != 1 || holds.Enrollments[0].TryoutID.String() != f.product.Tryouts[0].ID.String() {
		t.Errorf("student A enrollments = %+v, want the purchased tryout", holds.Enrollments)
	}
}

func TestCheckoutEndpointValidation(t *testing.T) {
	f := newOrderHTTPFixture(t)
	f.fund(t, f.studentA, "250000.00")
	productID := f.product.ID.String()
	proof := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof")

	cases := []struct {
		name   string
		body   map[string]any
		status int
		field  string
	}{
		{"unknown method", checkoutBody(productID, "paypal", nil), http.StatusUnprocessableEntity, "payment_method"},
		{"manual without a proof", checkoutBody(productID, "manual", nil), http.StatusUnprocessableEntity, "proof_asset_id"},
		{"wallet with a proof", checkoutBody(productID, "wallet", map[string]any{"proof_asset_id": proof.ID.String()}), http.StatusUnprocessableEntity, "proof_asset_id"},
		{"malformed product id", checkoutBody("nope", "wallet", nil), http.StatusUnprocessableEntity, "product_id"},
		{"short key", checkoutBody(productID, "wallet", map[string]any{"idempotency_key": "x"}), http.StatusUnprocessableEntity, "idempotency_key"},
		{"unknown field", checkoutBody(productID, "wallet", map[string]any{"price": "1.00"}), http.StatusUnprocessableEntity, "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.post("/api/v1/orders", tc.body, f.tokenA)
			if rec.Code != tc.status || errorBody(t, rec).Fields[tc.field] == "" {
				t.Errorf("status = %d body = %s, want %d with a message on %s", rec.Code, rec.Body.String(), tc.status, tc.field)
			}
		})
	}
	if rec := f.post("/api/v1/orders", checkoutBody(uuid.NewString(), "wallet", nil), f.tokenA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown product: status = %d, want 404", rec.Code)
	}
}

func TestManualCheckoutAndReviewEndpoints(t *testing.T) {
	f := newOrderHTTPFixture(t)
	productID := f.product.ID.String()

	bank := f.get("/api/v1/payments/bank-transfer", f.tokenA)
	if bank.Code != http.StatusOK || !strings.Contains(bank.Body.String(), `"account_number":"1234567890"`) {
		t.Errorf("bank transfer: status = %d body = %s, want the configured account", bank.Code, bank.Body.String())
	}
	if rec := f.get("/api/v1/payments/bank-transfer", f.tokenAdminA); rec.Code != http.StatusForbidden {
		t.Errorf("staff on the bank account: status = %d, want 403", rec.Code)
	}

	foreign := f.uploadPNG(t, f.e, f.hostA, f.tokenSecond, "payment_proof")
	rec := f.post("/api/v1/orders", checkoutBody(productID, "manual", map[string]any{"proof_asset_id": foreign.ID.String()}), f.tokenA)
	if rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["proof_asset_id"] == "" {
		t.Errorf("another student's proof: status = %d body = %s, want 422 on proof_asset_id", rec.Code, rec.Body.String())
	}

	proof := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof")
	rec = f.post("/api/v1/orders", checkoutBody(productID, "manual", map[string]any{"proof_asset_id": proof.ID.String()}), f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("manual checkout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	order := decodeOrder(t, rec)
	if order.Status != "pending" || order.ProofAssetID == nil || *order.ProofAssetID != proof.ID {
		t.Fatalf("order = %+v, want pending with the proof", order)
	}
	if holds := decodeEnrollments(t, f.get("/api/v1/enrollments", f.tokenA)); len(holds.Enrollments) != 0 {
		t.Error("a pending order opened access")
	}

	// Staff review it; the proof is read through the existing re-authorized download.
	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		list := decodeOrders(t, f.get("/api/v1/admin/orders?status=pending", token))
		if list.Total < 1 || list.Orders[0].ID != order.ID {
			t.Errorf("%s pending list = %+v, want the order", name, list)
		}
	}
	if rec := f.get("/api/v1/admin/orders/"+order.ID.String(), f.tokenOperatorA); rec.Code != http.StatusOK {
		t.Errorf("staff order detail: status = %d", rec.Code)
	}
	if rec := f.get("/api/v1/admin/orders?status=paid", f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown status filter: status = %d, want 422", rec.Code)
	}
	if rec := f.get("/api/v1/assets/"+proof.ID.String()+"/url", f.tokenOperatorA); rec.Code != http.StatusOK {
		t.Errorf("staff proof download: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := f.get("/api/v1/assets/"+proof.ID.String()+"/url", f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("another student's proof download: status = %d, want 404", rec.Code)
	}
	if rec := f.get("/api/v1/admin/orders/not-a-uuid", f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("malformed staff order id: status = %d, want 404", rec.Code)
	}

	confirmPath := "/api/v1/admin/orders/" + order.ID.String() + "/confirm"
	cancelPath := "/api/v1/admin/orders/" + order.ID.String() + "/cancel"
	for _, path := range []string{confirmPath, cancelPath} {
		if rec := f.post(path, map[string]any{"reason": "x"}, f.tokenA); rec.Code != http.StatusForbidden {
			t.Errorf("student on %s: status = %d, want 403", path, rec.Code)
		}
		if rec := f.post(path, map[string]any{"reason": "x"}, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous on %s: status = %d, want 401", path, rec.Code)
		}
	}
	if rec := f.post(cancelPath, map[string]any{"reason": "  "}, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity ||
		errorBody(t, rec).Fields["reason"] == "" {
		t.Errorf("cancel without a reason: status = %d body = %s, want 422 on reason", rec.Code, rec.Body.String())
	}

	confirmed := f.post(confirmPath, nil, f.tokenOperatorA)
	if confirmed.Code != http.StatusOK || decodeOrder(t, confirmed).Status != "confirmed" {
		t.Fatalf("confirm: status = %d body = %s", confirmed.Code, confirmed.Body.String())
	}
	if rec := f.post(confirmPath, nil, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Errorf("repeated confirm: status = %d, want 200", rec.Code)
	}
	if rec := f.post(cancelPath, map[string]any{"reason": "Terlambat"}, f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("cancel after confirm: status = %d, want 409", rec.Code)
	}
	if holds := decodeEnrollments(t, f.get("/api/v1/enrollments", f.tokenA)); len(holds.Enrollments) != 1 {
		t.Errorf("student A holds %d tryouts after confirmation, want 1", len(holds.Enrollments))
	}

	// A second order is cancelled with a reason the student can read, and opens nothing for student B.
	proofB := f.uploadPNG(t, f.e, f.hostA, f.tokenSecond, "payment_proof")
	second := decodeOrder(t, f.post("/api/v1/orders", checkoutBody(productID, "manual", map[string]any{"proof_asset_id": proofB.ID.String()}), f.tokenSecond))
	cancelled := f.post("/api/v1/admin/orders/"+second.ID.String()+"/cancel", map[string]any{"reason": "Bukti tidak sesuai"}, f.tokenOperatorA)
	if cancelled.Code != http.StatusOK || decodeOrder(t, cancelled).Status != "cancelled" {
		t.Fatalf("cancel: status = %d body = %s", cancelled.Code, cancelled.Body.String())
	}
	mine := decodeOrder(t, f.get("/api/v1/orders/"+second.ID.String(), f.tokenSecond))
	if mine.CancelReason != "Bukti tidak sesuai" {
		t.Errorf("cancel reason = %q, want the staff reason", mine.CancelReason)
	}
	if rec := f.post("/api/v1/admin/orders/"+second.ID.String()+"/confirm", nil, f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("confirm after cancel: status = %d, want 409", rec.Code)
	}
	if holds := decodeEnrollments(t, f.get("/api/v1/enrollments", f.tokenSecond)); len(holds.Enrollments) != 0 {
		t.Error("a cancelled order opened access")
	}
}

func TestProductSaleWindowOverHTTP(t *testing.T) {
	f := newOrderHTTPFixture(t)
	f.fund(t, f.studentA, "250000.00")
	program := f.program
	tryout := publishedTryout(t, f.e, f.hostA, f.tokenAdminA, program)

	from := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	until := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	bad := f.post("/api/v1/products", productBody(program, []string{tryout.ID.String()},
		map[string]any{"active_from": from, "active_until": until}), f.tokenAdminA)
	if bad.Code != http.StatusUnprocessableEntity || errorBody(t, bad).Fields["active_from"] == "" {
		t.Errorf("window ending before it starts: status = %d body = %s, want 422 on active_from", bad.Code, bad.Body.String())
	}

	future := createProduct(t, f.e, f.hostA, f.tokenAdminA,
		productBody(program, []string{tryout.ID.String()}, map[string]any{"active_from": from}))
	if future.ActiveFrom == nil {
		t.Fatal("the product does not echo active_from")
	}
	if catalog := f.get("/api/v1/catalog/products", ""); strings.Contains(catalog.Body.String(), future.ID.String()) {
		t.Error("the catalog lists a product before its sale window")
	}
	rec := f.post("/api/v1/orders", checkoutBody(future.ID.String(), "wallet", nil), f.tokenA)
	if rec.Code != http.StatusNotFound {
		t.Errorf("checkout before the window: status = %d body = %s, want 404", rec.Code, rec.Body.String())
	}
}

func TestCheckoutPriceChangedAndPendingOrderCodes(t *testing.T) {
	f := newOrderHTTPFixture(t)
	f.fund(t, f.studentA, "250000.00")
	productID := f.product.ID.String()

	stale := f.post("/api/v1/orders", checkoutBody(productID, "wallet", map[string]any{"expected_price": "90000.00"}), f.tokenA)
	if stale.Code != http.StatusConflict || errorBody(t, stale).Code != httpx.CodePriceChanged {
		t.Errorf("stale price: status = %d body = %s, want 409 price_changed", stale.Code, stale.Body.String())
	}
	missing := checkoutBody(productID, "wallet", nil)
	delete(missing, "expected_price")
	if rec := f.post("/api/v1/orders", missing, f.tokenA); rec.Code != http.StatusUnprocessableEntity || errorBody(t, rec).Fields["expected_price"] == "" {
		t.Errorf("no expected_price: status = %d body = %s, want 422 on expected_price", rec.Code, rec.Body.String())
	}
	if wallet := decodeWallet(t, f.get("/api/v1/wallet", f.tokenA)); wallet.Balance != "250000.00" {
		t.Errorf("balance = %s, want nothing debited", wallet.Balance)
	}

	first := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof")
	if rec := f.post("/api/v1/orders", checkoutBody(productID, "manual", map[string]any{"proof_asset_id": first.ID.String()}), f.tokenA); rec.Code != http.StatusCreated {
		t.Fatalf("manual checkout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	second := f.uploadPNG(t, f.e, f.hostA, f.tokenA, "payment_proof")
	for _, body := range []map[string]any{
		checkoutBody(productID, "manual", map[string]any{"proof_asset_id": second.ID.String()}),
		checkoutBody(productID, "wallet", nil),
	} {
		rec := f.post("/api/v1/orders", body, f.tokenA)
		if rec.Code != http.StatusConflict || errorBody(t, rec).Code != httpx.CodeOrderPending {
			t.Errorf("%v: status = %d body = %s, want 409 order_pending", body["payment_method"], rec.Code, rec.Body.String())
		}
	}
}
