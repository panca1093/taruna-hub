package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func newNotificationServiceForTest(t *testing.T, sealKey []byte) *NotificationService {
	t.Helper()
	svc, err := NewNotificationService(nil, nil, nil, sealKey, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

func sealKeyOf(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

func TestRetryDelaySchedule(t *testing.T) {
	cases := map[int]time.Duration{
		0:  30 * time.Second,
		1:  30 * time.Second,
		2:  2 * time.Minute,
		3:  10 * time.Minute,
		4:  time.Hour,
		5:  6 * time.Hour,
		6:  6 * time.Hour,
		99: 6 * time.Hour,
		-3: 30 * time.Second,
	}
	for attempt, want := range cases {
		if got := RetryDelay(attempt); got != want {
			t.Errorf("RetryDelay(%d) = %s, want %s", attempt, got, want)
		}
	}
}

func TestMaskRecipient(t *testing.T) {
	cases := map[string]string{
		"a@example.com":  "a***@example.com",
		"student@x.test": "s***@x.test",
		"not-an-address": "***",
		"@example.com":   "***",
		"":               "***",
	}
	for in, want := range cases {
		if got := MaskRecipient(in); got != want {
			t.Errorf("MaskRecipient(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnqueueRejectsInvalidInput(t *testing.T) {
	svc := newNotificationServiceForTest(t, nil)
	ctx := context.Background()
	zero := uuid.Nil

	valid := func() NewNotification {
		return NewNotification{
			EventKey:  "event-" + uuid.NewString()[:8],
			Channel:   "email",
			Recipient: "student@example.com",
			Template:  "smtp-test",
		}
	}

	cases := map[string]func(*NewNotification){
		"zero-uuid tenant": func(n *NewNotification) { n.Scope = Scope{TenantID: &zero} },
		"unknown channel":  func(n *NewNotification) { n.Channel = "telegram" },
		"empty recipient":  func(n *NewNotification) { n.Recipient = "  " },
		"empty event key":  func(n *NewNotification) { n.EventKey = "" },
		"unknown template": func(n *NewNotification) { n.Template = "no-such-template" },
		"past expiry":      func(n *NewNotification) { at := time.Now().Add(-time.Minute); n.ExpiresAt = &at },
		"overlapping keys": func(n *NewNotification) {
			n.Params = map[string]string{"otp": "public"}
			n.SecretParams = map[string]string{"otp": "secret"}
		},
	}
	for name, mutate := range cases {
		n := valid()
		mutate(&n)
		if err := svc.Enqueue(ctx, nil, n); err == nil {
			t.Errorf("%s: Enqueue succeeded, want a rejection", name)
		}
	}

	// Secret params without a key fail before anything is written.
	n := valid()
	n.SecretParams = map[string]string{"otp": "123456"}
	if err := svc.Enqueue(ctx, nil, n); !errors.Is(err, ErrSealKeyRequired) {
		t.Errorf("secret params without a key: err = %v, want ErrSealKeyRequired", err)
	}
}

func TestTemplateRegistryIsTheSeamForLaterSlices(t *testing.T) {
	svc := newNotificationServiceForTest(t, nil)

	if err := svc.RegisterTemplate("smtp-test", "x", "y", ""); !errors.Is(err, ErrTemplateExists) {
		t.Errorf("re-registering an existing template: err = %v, want ErrTemplateExists", err)
	}
	// The P03b templates come built in, so a later slice's template stands in here.
	for _, name := range []string{TemplateEmailCode, TemplatePasswordReset} {
		if _, ok := svc.templates[name]; !ok {
			t.Errorf("built-in template %s is missing", name)
		}
	}
	if err := svc.RegisterTemplate("order-created", "Order {{.Get \"subject\"}}", "order: {{.Get \"order\"}}", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := svc.templates["order-created"]; !ok {
		t.Error("the registered template is not reachable by name")
	}
	if err := svc.RegisterTemplate("broken", "{{", "x", ""); err == nil {
		t.Error("a malformed template was accepted")
	}
	if err := svc.RegisterTemplate("broken-html", "x", "y", "{{define \"content\"}}"); err == nil {
		t.Error("a malformed HTML template was accepted")
	}
	if err := svc.RegisterTemplate("missing-blocks", "x", "y", `{{define "content"}}x{{end}}`); err == nil {
		t.Error("HTML missing blocks the layout needs was accepted")
	}
}

func TestHTMLWithoutAppURLOmitsLogoAndSiteLink(t *testing.T) {
	svc := newNotificationServiceForTest(t, nil)
	msg := renderForTest(t, svc, TemplateEmailCode, map[string]string{
		"tenant_name": "Taruna Eduka", "name": "Panca", "minutes": "10", "code": "561888",
	})
	if !strings.Contains(msg.HTML, "561888") {
		t.Errorf("HTML part lost the code:\n%s", msg.HTML)
	}
	for _, unwanted := range []string{"<img", "<a "} {
		if strings.Contains(msg.HTML, unwanted) {
			t.Errorf("HTML part without an app URL contains %q:\n%s", unwanted, msg.HTML)
		}
	}
}

func renderForTest(t *testing.T, svc *NotificationService, name string, params map[string]string) Message {
	t.Helper()
	n := model.Notification{ID: uuid.New(), Recipient: "student@example.com", Template: name, Params: params}
	data, err := svc.renderParams(n)
	if err != nil {
		t.Fatalf("%s: params: %v", name, err)
	}
	msg, err := svc.render(svc.templates[name], data, n)
	if err != nil {
		t.Fatalf("%s: render: %v", name, err)
	}
	return msg
}

func TestUserFacingTemplatesRenderTextAndHTML(t *testing.T) {
	svc, err := NewNotificationService(nil, nil, nil, nil, nil, WithAppURL("https://tarunaedu.com"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	base := map[string]string{"tenant_name": "Taruna Eduka", "name": "Panca", "minutes": "10"}
	cases := map[string]map[string]string{
		TemplateEmailCode:      {"code": "561888"},
		TemplatePasswordReset:  {"reset_url": "https://tarunaedu.com/reset-password#token=abc_-.def"},
		TemplateOrderConfirmed: {"code": "ORD-00042", "product": "Paket SKD"},
		TemplateOrderCancelled: {"code": "ORD-00043", "product": "Paket SKD", "reason": "Pembayaran kedaluwarsa"},
		TemplateTopupApproved:  {"code": "TOP-00007", "amount": "Rp50.000"},
		TemplateTopupRejected:  {"code": "TOP-00008", "amount": "Rp50.000", "reason": "Bukti transfer tidak terbaca"},
	}
	for name, extra := range cases {
		params := map[string]string{}
		for k, v := range base {
			params[k] = v
		}
		for k, v := range extra {
			params[k] = v
		}
		msg := renderForTest(t, svc, name, params)
		if msg.HTML == "" {
			t.Errorf("%s: no HTML part", name)
			continue
		}
		for _, v := range extra {
			if !strings.Contains(msg.Body, v) {
				t.Errorf("%s: text part is missing %q", name, v)
			}
			if !strings.Contains(msg.HTML, v) {
				t.Errorf("%s: HTML part is missing %q", name, v)
			}
		}
		for _, want := range []string{"https://tarunaedu.com/taruna-eduka-logo.png", "Halo Panca,", "mohon tidak dibalas"} {
			if !strings.Contains(msg.HTML, want) {
				t.Errorf("%s: HTML part is missing %q", name, want)
			}
		}
	}
	if msg := renderForTest(t, svc, templateNameSMTPTest, map[string]string{"subject": "x"}); msg.HTML != "" {
		t.Errorf("smtp-test grew an HTML part: %q", msg.HTML)
	}
}

func TestHTMLPartEscapesParams(t *testing.T) {
	svc := newNotificationServiceForTest(t, nil)
	msg := renderForTest(t, svc, TemplateOrderCancelled, map[string]string{
		"tenant_name": "Taruna Eduka", "name": `<script>alert(1)</script>`, "code": "ORD-1",
		"product": `<img src=x onerror=alert(1)>`, "reason": `"><b>x</b>`,
	})
	for _, raw := range []string{"<script>", "<img src=x", "<b>x</b>"} {
		if strings.Contains(msg.HTML, raw) {
			t.Errorf("HTML part contains unescaped %q", raw)
		}
	}
	if !strings.Contains(msg.HTML, "&lt;script&gt;") {
		t.Errorf("HTML part lost the escaped name:\n%s", msg.HTML)
	}
}

func TestSealNonceUniqueAndTamperDetected(t *testing.T) {
	sealer, err := NewSealer(sealKeyOf(7))
	if err != nil {
		t.Fatalf("new sealer: %v", err)
	}
	id := uuid.New()
	plaintext := []byte(`{"otp":"482913"}`)

	first, err := sealer.Seal(plaintext, id[:])
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	second, err := sealer.Seal(plaintext, id[:])
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Error("two seals of the same plaintext produced identical bytes; the nonce is not fresh")
	}
	if bytes.Contains(first, plaintext) {
		t.Error("the ciphertext contains the plaintext")
	}

	got, err := sealer.Open(first, id[:])
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("open = %q, %v; want the plaintext", got, err)
	}

	// Tampering, a different associated data and a truncated payload all fail closed.
	tampered := append([]byte(nil), first...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := sealer.Open(tampered, id[:]); !errors.Is(err, ErrSealOpen) {
		t.Errorf("tampered payload: err = %v, want ErrSealOpen", err)
	}
	other := uuid.New()
	if _, err := sealer.Open(first, other[:]); !errors.Is(err, ErrSealOpen) {
		t.Errorf("wrong associated data: err = %v, want ErrSealOpen", err)
	}
	if _, err := sealer.Open(first[:4], id[:]); !errors.Is(err, ErrSealOpen) {
		t.Errorf("truncated payload: err = %v, want ErrSealOpen", err)
	}
}

func TestSealerRejectsWrongKeyLength(t *testing.T) {
	for _, size := range []int{0, 16, 31, 33} {
		_, err := NewSealer(make([]byte, size))
		if !errors.Is(err, ErrInvalidSealKey) {
			t.Errorf("NewSealer(%d bytes): err = %v, want ErrInvalidSealKey", size, err)
		}
		if errors.Is(err, ErrInvalidNotification) {
			t.Errorf("NewSealer(%d bytes): err = %v reads as an invalid notification, want the seal-key sentinel", size, err)
		}
	}
	if _, err := NewSealer(sealKeyOf(1)); err != nil {
		t.Errorf("NewSealer(32 bytes): %v, want nil", err)
	}
}
