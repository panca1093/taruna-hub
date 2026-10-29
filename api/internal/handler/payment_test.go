package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const handlerServerKey = "SB-handler-key"

// handlerGateway is Midtrans in memory for the HTTP tests.
type handlerGateway struct {
	mu         sync.Mutex
	configured bool
	statuses   map[string]service.ProviderStatus
	created    map[string]bool
}

func newHandlerGateway() *handlerGateway {
	return &handlerGateway{configured: true, statuses: map[string]service.ProviderStatus{}, created: map[string]bool{}}
}

func (g *handlerGateway) Configured() bool { return g.configured }

func (g *handlerGateway) CreateTransaction(_ context.Context, req service.SnapRequest) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.created[req.ProviderOrderID] {
		return "", service.ErrGatewayDuplicate
	}
	g.created[req.ProviderOrderID] = true
	return "https://pay.example/" + req.ProviderOrderID, nil
}

func (g *handlerGateway) Status(_ context.Context, id string) (service.ProviderStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if st, ok := g.statuses[id]; ok {
		return st, nil
	}
	return service.ProviderStatus{}, service.ErrGatewayNotFound
}

func (g *handlerGateway) VerifySignature(n service.Notification) bool {
	return service.SignatureMatches(service.MidtransSignature(n.OrderID, n.StatusCode, n.GrossAmount, handlerServerKey), n.SignatureKey)
}

func (g *handlerGateway) set(id, status, gross string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.statuses[id] = service.ProviderStatus{OrderID: id, StatusCode: "200", GrossAmount: gross, TransactionStatus: status,
		Raw: []byte(`{"order_id":"` + id + `","transaction_status":"` + status + `"}`)}
}

type paymentHTTPFixture struct {
	walletHTTPFixture
	e       http.Handler
	product productResponse
}

func newPaymentHTTPFixture(t *testing.T) paymentHTTPFixture {
	t.Helper()
	f := paymentHTTPFixture{walletHTTPFixture: newWalletHTTPFixture(t)}
	f.e = f.echo(t)
	program := questionCurriculum(t, f.e, f.hostA, f.tokenAdminA)
	tryout := publishedTryout(t, f.e, f.hostA, f.tokenAdminA, program)
	f.product = createProduct(t, f.e, f.hostA, f.tokenAdminA,
		productBody(program, []string{tryout.ID.String()}, map[string]any{"price": "100000.00"}))
	return f
}

func (f paymentHTTPFixture) post(path string, body any, token string) *httptest.ResponseRecorder {
	return sendTryoutJSON(f.e, http.MethodPost, f.hostA, path, body, token)
}

func (f paymentHTTPFixture) get(path, token string) *httptest.ResponseRecorder {
	return getWithToken(f.e, f.hostA, path, token)
}

// webhook posts a signed notification with no session; the handler never reads the host.
func (f paymentHTTPFixture) webhook(id, status, gross, signature string) *httptest.ResponseRecorder {
	if signature == "" {
		signature = service.MidtransSignature(id, "200", gross, handlerServerKey)
	}
	return sendTryoutJSON(f.e, http.MethodPost, f.hostA, "/api/v1/payments/midtrans/notification", map[string]string{
		"order_id": id, "status_code": "200", "gross_amount": gross, "transaction_status": status, "signature_key": signature,
	}, "")
}

func (f paymentHTTPFixture) gatewayOrder(t *testing.T, token string) (orderResponse, string) {
	t.Helper()
	rec := f.post("/api/v1/orders", checkoutBody(f.product.ID.String(), "gateway", nil), token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("gateway checkout: status = %d body = %s", rec.Code, rec.Body.String())
	}
	order := decodeOrder(t, rec)
	var id string
	if err := f.pool.QueryRow(context.Background(), `SELECT provider_order_id FROM payment_attempts WHERE order_id = $1`, order.ID).Scan(&id); err != nil {
		t.Fatalf("read provider order id: %v", err)
	}
	return order, id
}

func TestGatewayMethodAvailability(t *testing.T) {
	f := newPaymentHTTPFixture(t)
	if rec := f.get("/api/v1/payments/gateway", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: status = %d, want 401", rec.Code)
	}
	for name, token := range map[string]string{"admin": f.tokenAdminA, "operator": f.tokenOperatorA} {
		if rec := f.get("/api/v1/payments/gateway", token); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", name, rec.Code)
		}
	}
	if rec := f.get("/api/v1/payments/gateway", f.tokenA); rec.Code != http.StatusOK {
		t.Errorf("student: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	f.gateway.configured = false
	if rec := f.get("/api/v1/payments/gateway", f.tokenA); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured: status = %d, want 503", rec.Code)
	}
	rec := f.post("/api/v1/orders", checkoutBody(f.product.ID.String(), "gateway", nil), f.tokenA)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unconfigured checkout: status = %d, want 503", rec.Code)
	}
	f.gateway.configured = true
}

func TestWebhookNeedsOnlyAValidSignatureAndNeverTrustsTheReturnUrl(t *testing.T) {
	f := newPaymentHTTPFixture(t)
	order, id := f.gatewayOrder(t, f.tokenA)
	if order.Status != "pending" || order.PaymentMethod != "gateway" || order.RedirectURL == nil {
		t.Fatalf("order = %+v, want pending with the provider page", order)
	}
	// Returning from the provider (the student polling) grants nothing while the provider says pending.
	f.gateway.set(id, "pending", "100000.00")
	rec := f.get("/api/v1/orders/"+order.ID.String(), f.tokenA)
	if got := decodeOrder(t, rec); rec.Code != http.StatusOK || got.Status != "pending" {
		t.Fatalf("poll = %d %+v, want pending", rec.Code, got)
	}

	if rec := f.webhook(id, "settlement", "100000.00", "bogus"); rec.Code != http.StatusForbidden {
		t.Errorf("bad signature: status = %d, want 403", rec.Code)
	}
	if rec := f.webhook("TE-unknown", "settlement", "100000.00", ""); rec.Code != http.StatusOK {
		t.Errorf("unknown reference: status = %d, want 200 so the provider stops retrying", rec.Code)
	}
	if got := decodeOrder(t, f.get("/api/v1/orders/"+order.ID.String(), f.tokenA)); got.Status != "pending" {
		t.Fatalf("order = %s after refused notifications, want pending", got.Status)
	}

	f.gateway.set(id, "settlement", "100000.00")
	for i := 0; i < 3; i++ {
		if rec := f.webhook(id, "settlement", "100000.00", ""); rec.Code != http.StatusOK {
			t.Fatalf("settlement %d: status = %d body = %s", i, rec.Code, rec.Body.String())
		}
	}
	got := decodeOrder(t, f.get("/api/v1/orders/"+order.ID.String(), f.tokenA))
	if got.Status != "confirmed" || got.RedirectURL != nil {
		t.Errorf("order = %+v, want confirmed with no open page", got)
	}
	var enrollments int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM enrollments WHERE user_id = $1`, f.studentA.ID).Scan(&enrollments); err != nil || enrollments != 1 {
		t.Errorf("enrollments = %d, %v; want 1", enrollments, err)
	}
	// Another student cannot read it.
	if rec := f.get("/api/v1/orders/"+order.ID.String(), f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("foreign order: status = %d, want 404", rec.Code)
	}
}

func TestHeldPaymentFlowsToStaffAndIsResolvedOnce(t *testing.T) {
	f := newPaymentHTTPFixture(t)
	order, id := f.gatewayOrder(t, f.tokenA)
	if rec := f.post("/api/v1/admin/orders/"+order.ID.String()+"/cancel", map[string]string{"reason": "salah"}, f.tokenAdminA); rec.Code != http.StatusOK {
		t.Fatalf("cancel: status = %d body = %s", rec.Code, rec.Body.String())
	}
	f.gateway.set(id, "settlement", "100000.00")
	if rec := f.webhook(id, "settlement", "100000.00", ""); rec.Code != http.StatusOK {
		t.Fatalf("late settlement: status = %d", rec.Code)
	}
	detail := decodeOrder(t, f.get("/api/v1/admin/orders/"+order.ID.String(), f.tokenOperatorA))
	if detail.Status != "cancelled" || detail.PaymentException == nil || detail.PaymentException.Kind != "late_success" {
		t.Fatalf("order = %+v, want cancelled with a held late_success", detail)
	}
	path := "/api/v1/admin/payment-exceptions/" + detail.PaymentException.ID.String()

	if rec := f.get("/api/v1/admin/payment-exceptions", f.tokenA); rec.Code != http.StatusForbidden {
		t.Errorf("student list: status = %d, want 403", rec.Code)
	}
	if rec := f.get("/api/v1/admin/payment-exceptions", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous list: status = %d, want 401", rec.Code)
	}
	rec := f.get("/api/v1/admin/payment-exceptions?status=held", f.tokenOperatorA)
	var list paymentExceptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != http.StatusOK || list.Total != 1 || list.Exceptions[0].PaidAmount != "100000.00" {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.post(path+"/credit", map[string]string{"reason": "x"}, f.tokenA); rec.Code != http.StatusForbidden {
		t.Errorf("student credit: status = %d, want 403", rec.Code)
	}
	if rec := f.post(path+"/credit", map[string]string{"reason": " "}, f.tokenAdminA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("credit without a reason: status = %d, want 422", rec.Code)
	}
	if rec := f.post("/api/v1/admin/payment-exceptions/"+uuid.NewString()+"/credit", map[string]string{"reason": "x"}, f.tokenAdminA); rec.Code != http.StatusNotFound {
		t.Errorf("unknown exception: status = %d, want 404", rec.Code)
	}
	for _, token := range []string{f.tokenOperatorA, f.tokenAdminA} {
		if rec := f.post(path+"/credit", map[string]string{"reason": "pembayaran terlambat"}, token); rec.Code != http.StatusOK {
			t.Fatalf("credit: status = %d body = %s", rec.Code, rec.Body.String())
		}
	}
	if rec := f.post(path+"/refund", map[string]string{"reason": "sudah"}, f.tokenAdminA); rec.Code != http.StatusConflict {
		t.Errorf("refund after credit: status = %d, want 409", rec.Code)
	}
	var balance string
	if err := f.pool.QueryRow(context.Background(), `SELECT balance::text FROM users WHERE id = $1`, f.studentA.ID).Scan(&balance); err != nil || balance != "100000.00" {
		t.Errorf("balance = %s, %v; want one credit of 100000.00", balance, err)
	}
}

func TestGatewayTopupOverHTTP(t *testing.T) {
	f := newPaymentHTTPFixture(t)
	rec := f.post("/api/v1/topups", map[string]any{"method": "gateway", "amount": "50000", "idempotency_key": "topup-key-" + uuid.NewString()[:6]}, f.tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d body = %s", rec.Code, rec.Body.String())
	}
	topup := decodeTopup(t, rec)
	if topup.Method != "gateway" || topup.Status != "pending" || topup.RedirectURL == nil || topup.ProofAssetID != nil {
		t.Fatalf("top-up = %+v", topup)
	}
	var id string
	if err := f.pool.QueryRow(context.Background(), `SELECT provider_order_id FROM payment_attempts WHERE topup_id = $1`, topup.ID).Scan(&id); err != nil {
		t.Fatalf("provider id: %v", err)
	}
	f.gateway.set(id, "settlement", "50000.00")
	for i := 0; i < 2; i++ {
		if rec := f.webhook(id, "settlement", "50000.00", ""); rec.Code != http.StatusOK {
			t.Fatalf("settlement: status = %d", rec.Code)
		}
	}
	if got := decodeTopup(t, f.get("/api/v1/topups/"+topup.ID.String(), f.tokenA)); got.Status != "approved" {
		t.Errorf("top-up = %+v, want approved", got)
	}
	if rec := f.get("/api/v1/topups/"+topup.ID.String(), f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("foreign top-up: status = %d, want 404", rec.Code)
	}
	if rec := f.post("/api/v1/topups", map[string]any{"method": "bogus", "amount": "50000", "idempotency_key": "topup-key-zzzzzz"}, f.tokenA); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad method: status = %d, want 422", rec.Code)
	}
	var balance string
	if err := f.pool.QueryRow(context.Background(), `SELECT balance::text FROM users WHERE id = $1`, f.studentA.ID).Scan(&balance); err != nil || balance != "50000.00" {
		t.Errorf("balance = %s, %v", balance, err)
	}
}

func TestStaffCannotConfirmGatewayPaymentsAndStudentCanCancelThem(t *testing.T) {
	f := newPaymentHTTPFixture(t)
	order, _ := f.gatewayOrder(t, f.tokenA)
	rec := f.post("/api/v1/admin/orders/"+order.ID.String()+"/confirm", nil, f.tokenAdminA)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "gateway_managed") {
		t.Errorf("confirm gateway order: status = %d body = %s, want 409 gateway_managed", rec.Code, rec.Body.String())
	}
	rec = f.post("/api/v1/topups", map[string]any{"method": "gateway", "amount": "50000", "idempotency_key": "topup-key-" + uuid.NewString()[:6]}, f.tokenA)
	topup := decodeTopup(t, rec)
	rec = f.post("/api/v1/admin/topups/"+topup.ID.String()+"/approve", nil, f.tokenOperatorA)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "gateway_managed") {
		t.Errorf("approve gateway top-up: status = %d body = %s, want 409 gateway_managed", rec.Code, rec.Body.String())
	}

	if rec := f.post("/api/v1/orders/"+order.ID.String()+"/cancel", nil, f.tokenSecond); rec.Code != http.StatusNotFound {
		t.Errorf("foreign cancel: status = %d, want 404", rec.Code)
	}
	if rec := f.post("/api/v1/orders/"+order.ID.String()+"/cancel", nil, f.tokenAdminA); rec.Code != http.StatusForbidden {
		t.Errorf("staff on the student cancel: status = %d, want 403", rec.Code)
	}
	rec = f.post("/api/v1/orders/"+order.ID.String()+"/cancel", nil, f.tokenA)
	if got := decodeOrder(t, rec); rec.Code != http.StatusOK || got.Status != "cancelled" {
		t.Errorf("student cancel: status = %d body = %s", rec.Code, rec.Body.String())
	}
	rec = f.post("/api/v1/topups/"+topup.ID.String()+"/cancel", nil, f.tokenA)
	if got := decodeTopup(t, rec); rec.Code != http.StatusOK || got.Status != "rejected" {
		t.Errorf("student top-up cancel: status = %d body = %s", rec.Code, rec.Body.String())
	}
}
