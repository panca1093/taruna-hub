package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	midtransTimeout   = 10 * time.Second
	midtransMaxBody   = 1 << 20
	snapSandbox       = "https://app.sandbox.midtrans.com"
	snapProduction    = "https://app.midtrans.com"
	statusSandbox     = "https://api.sandbox.midtrans.com"
	statusProduction  = "https://api.midtrans.com"
	midtransNotFound  = "404"
	midtransDuplicate = "has already been taken"
)

// Midtrans talks to Snap (create) and the Status API. Without a server key it is simply unavailable.
type Midtrans struct {
	serverKey       string
	snapBase        string
	statusBase      string
	notificationURL string
	client          *http.Client
}

func NewMidtrans(cfg config.MidtransConfig) *Midtrans {
	m := &Midtrans{
		serverKey: cfg.ServerKey, notificationURL: cfg.NotificationURL,
		snapBase: snapSandbox, statusBase: statusSandbox, client: &http.Client{Timeout: midtransTimeout},
	}
	if cfg.Production {
		m.snapBase, m.statusBase = snapProduction, statusProduction
	}
	return m
}

// WithBases points both APIs at another origin; tests use it to talk to a local fake.
func (m *Midtrans) WithBases(snap, status string) *Midtrans {
	m.snapBase, m.statusBase = snap, status
	return m
}

func (m *Midtrans) Configured() bool { return m != nil && m.serverKey != "" }

func (m *Midtrans) VerifySignature(n service.Notification) bool {
	if !m.Configured() || n.SignatureKey == "" {
		return false
	}
	return service.SignatureMatches(service.MidtransSignature(n.OrderID, n.StatusCode, n.GrossAmount, m.serverKey), n.SignatureKey)
}

type snapBody struct {
	TransactionDetails struct {
		OrderID     string `json:"order_id"`
		GrossAmount int64  `json:"gross_amount"`
	} `json:"transaction_details"`
	ItemDetails []struct {
		ID       string `json:"id"`
		Price    int64  `json:"price"`
		Quantity int    `json:"quantity"`
		Name     string `json:"name"`
	} `json:"item_details"`
	CustomerDetails struct {
		FirstName string `json:"first_name"`
		Email     string `json:"email,omitempty"`
		Phone     string `json:"phone,omitempty"`
	} `json:"customer_details"`
	Expiry struct {
		Unit     string `json:"unit"`
		Duration int    `json:"duration"`
	} `json:"expiry"`
	Callbacks struct {
		Finish string `json:"finish"`
	} `json:"callbacks"`
}

// CreateTransaction opens a Snap payment page. A network failure, timeout or 5xx is ambiguous: the provider may
// have created it, so the caller retries with the same order id.
func (m *Midtrans) CreateTransaction(ctx context.Context, req service.SnapRequest) (string, error) {
	whole, _, _ := strings.Cut(req.Amount, ".")
	var amount int64
	if _, err := fmt.Sscanf(whole, "%d", &amount); err != nil || amount < 1 {
		return "", fmt.Errorf("%w: amount %q", service.ErrGatewayRejected, req.Amount)
	}
	var body snapBody
	body.TransactionDetails.OrderID = req.ProviderOrderID
	body.TransactionDetails.GrossAmount = amount
	body.ItemDetails = append(body.ItemDetails, struct {
		ID       string `json:"id"`
		Price    int64  `json:"price"`
		Quantity int    `json:"quantity"`
		Name     string `json:"name"`
	}{ID: req.ProviderOrderID, Price: amount, Quantity: 1, Name: req.ItemName})
	body.CustomerDetails.FirstName, body.CustomerDetails.Email, body.CustomerDetails.Phone = req.CustomerName, req.Email, req.Phone
	body.Expiry.Unit, body.Expiry.Duration = "minutes", req.ExpiryMinutes
	body.Callbacks.Finish = req.FinishURL
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.snapBase+"/snap/v1/transactions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if m.notificationURL != "" {
		httpReq.Header.Set("X-Override-Notification", m.notificationURL)
	}
	status, data, err := m.do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", service.ErrGatewayAmbiguous, err)
	}
	switch {
	case status == http.StatusCreated || status == http.StatusOK:
		var out struct {
			RedirectURL string `json:"redirect_url"`
		}
		if err := json.Unmarshal(data, &out); err != nil || out.RedirectURL == "" {
			return "", fmt.Errorf("%w: unreadable create response", service.ErrGatewayAmbiguous)
		}
		return out.RedirectURL, nil
	case status >= 500 || status == http.StatusRequestTimeout || status == http.StatusTooManyRequests:
		return "", fmt.Errorf("%w: status %d", service.ErrGatewayAmbiguous, status)
	case strings.Contains(string(data), midtransDuplicate):
		return "", service.ErrGatewayDuplicate
	}
	return "", fmt.Errorf("%w: status %d", service.ErrGatewayRejected, status)
}

// Status asks the provider for the truth about one transaction. Midtrans answers a missing transaction with an
// HTTP 200 whose body status_code is 404.
func (m *Midtrans) Status(ctx context.Context, providerOrderID string) (service.ProviderStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		m.statusBase+"/v2/"+url.PathEscape(providerOrderID)+"/status", nil)
	if err != nil {
		return service.ProviderStatus{}, err
	}
	status, data, err := m.do(httpReq)
	if err != nil {
		return service.ProviderStatus{}, fmt.Errorf("%w: %v", service.ErrGatewayAmbiguous, err)
	}
	if status == http.StatusNotFound {
		return service.ProviderStatus{}, service.ErrGatewayNotFound
	}
	if status != http.StatusOK {
		return service.ProviderStatus{}, fmt.Errorf("%w: status %d", service.ErrGatewayAmbiguous, status)
	}
	var out struct {
		StatusCode        string `json:"status_code"`
		TransactionID     string `json:"transaction_id"`
		OrderID           string `json:"order_id"`
		GrossAmount       string `json:"gross_amount"`
		TransactionStatus string `json:"transaction_status"`
		FraudStatus       string `json:"fraud_status"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return service.ProviderStatus{}, fmt.Errorf("%w: unreadable status response", service.ErrGatewayAmbiguous)
	}
	if out.StatusCode == midtransNotFound {
		return service.ProviderStatus{}, service.ErrGatewayNotFound
	}
	return service.ProviderStatus{
		OrderID: out.OrderID, TransactionID: out.TransactionID, StatusCode: out.StatusCode, GrossAmount: out.GrossAmount,
		TransactionStatus: out.TransactionStatus, FraudStatus: out.FraudStatus, Raw: data,
	}, nil
}

func (m *Midtrans) do(req *http.Request) (int, []byte, error) {
	req.SetBasicAuth(m.serverKey, "")
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, midtransMaxBody))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}
