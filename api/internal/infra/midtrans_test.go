package infra

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func midtransAgainst(t *testing.T, handler http.HandlerFunc) *Midtrans {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewMidtrans(config.MidtransConfig{ServerKey: "SB-key", NotificationURL: "https://api.example/notify"}).WithBases(server.URL, server.URL)
}

func TestMidtransIsUnavailableWithoutAKey(t *testing.T) {
	if NewMidtrans(config.MidtransConfig{}).Configured() {
		t.Error("a gateway without a server key claims to be configured")
	}
	if !NewMidtrans(config.MidtransConfig{ServerKey: "k"}).Configured() {
		t.Error("a gateway with a server key is not configured")
	}
}

func TestMidtransSignature(t *testing.T) {
	m := NewMidtrans(config.MidtransConfig{ServerKey: "SB-key"})
	good := service.MidtransSignature("TE-1", "200", "100000.00", "SB-key")
	if len(good) != 128 {
		t.Fatalf("signature length = %d, want a hex SHA512", len(good))
	}
	n := service.Notification{ProviderStatus: service.ProviderStatus{OrderID: "TE-1", StatusCode: "200", GrossAmount: "100000.00"}, SignatureKey: good}
	if !m.VerifySignature(n) {
		t.Error("a valid signature was refused")
	}
	for name, mutate := range map[string]func(*service.Notification){
		"amount": func(n *service.Notification) { n.GrossAmount = "1.00" },
		"order":  func(n *service.Notification) { n.OrderID = "TE-2" },
		"code":   func(n *service.Notification) { n.StatusCode = "201" },
		"empty":  func(n *service.Notification) { n.SignatureKey = "" },
	} {
		bad := n
		mutate(&bad)
		if m.VerifySignature(bad) {
			t.Errorf("%s tampering was accepted", name)
		}
	}
}

func TestMidtransCreateTransaction(t *testing.T) {
	var got map[string]any
	var auth, override string
	m := midtransAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		auth, override = r.Header.Get("Authorization"), r.Header.Get("X-Override-Notification")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"t","redirect_url":"https://pay/redirect"}`))
	})
	url, err := m.CreateTransaction(context.Background(), service.SnapRequest{
		ProviderOrderID: "TE-1", Amount: "100000.00", ItemName: "Paket", CustomerName: "Naila", FinishURL: "https://app/student/orders", ExpiryMinutes: 1440,
	})
	if err != nil || url != "https://pay/redirect" {
		t.Fatalf("create = %q, %v", url, err)
	}
	details := got["transaction_details"].(map[string]any)
	if details["order_id"] != "TE-1" || details["gross_amount"] != float64(100000) {
		t.Errorf("transaction_details = %v", details)
	}
	if !strings.HasPrefix(auth, "Basic ") || override != "https://api.example/notify" {
		t.Errorf("auth %q override %q", auth, override)
	}
}

func TestMidtransCreateClassifiesFailures(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   error
	}{
		"duplicate": {http.StatusBadRequest, `{"error_messages":["transaction_details.order_id has already been taken"]}`, service.ErrGatewayDuplicate},
		"rejected":  {http.StatusUnauthorized, `{"error_messages":["Access denied"]}`, service.ErrGatewayRejected},
		"server":    {http.StatusBadGateway, `oops`, service.ErrGatewayAmbiguous},
		"garbled":   {http.StatusCreated, `not json`, service.ErrGatewayAmbiguous},
	}
	for name, c := range cases {
		m := midtransAgainst(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		})
		if _, err := m.CreateTransaction(context.Background(), service.SnapRequest{ProviderOrderID: "TE-1", Amount: "1000.00"}); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	dead := NewMidtrans(config.MidtransConfig{ServerKey: "k"}).WithBases("http://127.0.0.1:1", "http://127.0.0.1:1")
	if _, err := dead.CreateTransaction(context.Background(), service.SnapRequest{ProviderOrderID: "TE-1", Amount: "1000.00"}); !errors.Is(err, service.ErrGatewayAmbiguous) {
		t.Errorf("unreachable provider = %v, want ErrGatewayAmbiguous", err)
	}
}

func TestMidtransStatus(t *testing.T) {
	m := midtransAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/TE-1/status":
			_, _ = w.Write([]byte(`{"status_code":"200","transaction_id":"x","order_id":"TE-1","gross_amount":"100000.00","transaction_status":"settlement","fraud_status":"accept"}`))
		case "/v2/TE-2/status":
			_, _ = w.Write([]byte(`{"status_code":"404","status_message":"Transaction doesn't exist."}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	st, err := m.Status(context.Background(), "TE-1")
	if err != nil || st.TransactionStatus != "settlement" || st.GrossAmount != "100000.00" || st.FraudStatus != "accept" || len(st.Raw) == 0 {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if _, err := m.Status(context.Background(), "TE-2"); !errors.Is(err, service.ErrGatewayNotFound) {
		t.Errorf("missing transaction = %v, want ErrGatewayNotFound", err)
	}
	if _, err := m.Status(context.Background(), "TE-3"); !errors.Is(err, service.ErrGatewayAmbiguous) {
		t.Errorf("provider error = %v, want ErrGatewayAmbiguous", err)
	}
}
