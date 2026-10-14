package infra

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

func senderFor(t *testing.T, s *fakeSMTP, mutate func(*config.SMTPConfig)) *SMTPSender {
	t.Helper()
	host, port := s.hostPort(t)
	cfg := config.SMTPConfig{Host: host, Port: port, From: "no-reply@tarunaeduka.id", TLS: config.SMTPTLSNone}
	if mutate != nil {
		mutate(&cfg)
	}
	return NewSMTPSender(cfg, testLogger())
}

func testMessage() service.Message {
	return service.Message{Recipient: "student@example.com", Subject: "Taruna Eduka test message", Body: "Pesan uji dari Taruna Eduka.\n"}
}

func TestSMTPSenderDeliversPlainMessage(t *testing.T) {
	s := newFakeSMTP(t)
	sender := senderFor(t, s, nil)

	if err := sender.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	mails := s.messages()
	if len(mails) != 1 {
		t.Fatalf("messages = %d, want 1", len(mails))
	}
	for _, want := range []string{"To: student@example.com", "From: no-reply@tarunaeduka.id", "Subject: Taruna Eduka test message",
		"Content-Type: text/plain", "Date: ", "Message-ID: <"} {
		if !strings.Contains(mails[0], want) {
			t.Errorf("message is missing %q:\n%s", want, mails[0])
		}
	}
	if !strings.Contains(mails[0], "Pesan uji dari Taruna Eduka.") {
		t.Errorf("body missing:\n%s", mails[0])
	}
}

func TestSMTPSenderRejectsHeaderInjection(t *testing.T) {
	s := newFakeSMTP(t)
	ctx := context.Background()

	cases := map[string]func(*service.Message, *config.SMTPConfig){
		"recipient": func(m *service.Message, _ *config.SMTPConfig) {
			m.Recipient = "student@example.com\r\nBcc: attacker@example.com"
		},
		"subject": func(m *service.Message, _ *config.SMTPConfig) {
			m.Subject = "hello\r\nBcc: attacker@example.com"
		},
		"from": func(_ *service.Message, c *config.SMTPConfig) {
			c.From = "no-reply@tarunaeduka.id\r\nBcc: attacker@example.com"
		},
	}
	for name, mutate := range cases {
		cfg := config.SMTPConfig{Host: "", Port: "1025", From: "no-reply@tarunaeduka.id", TLS: config.SMTPTLSNone}
		msg := testMessage()
		mutate(&msg, &cfg)
		sender := NewSMTPSender(cfg, testLogger())

		err := sender.Send(ctx, msg)
		if err == nil {
			t.Fatalf("%s: send succeeded, want a permanent rejection", name)
		}
		if !service.IsPermanent(err) {
			t.Errorf("%s: class = %v, want permanent", name, err)
		}
	}
	if got := s.connectionCount(); got != 0 {
		t.Errorf("connections = %d, want 0: injection must be rejected before dialling", got)
	}
}

func TestSMTPStartTLSRequired(t *testing.T) {
	s := newFakeSMTP(t)
	// The server offers no STARTTLS extension at all.
	sender := senderFor(t, s, func(c *config.SMTPConfig) { c.TLS = config.SMTPTLSStartTLS })

	err := sender.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("send succeeded against a server without STARTTLS, want a failure")
	}
	if !service.IsPermanent(err) {
		t.Errorf("class = %v, want permanent: the session must close rather than send cleartext", err)
	}
	if got := s.messageCount(); got != 0 {
		t.Errorf("messages = %d, want 0: never fall back to cleartext", got)
	}
}

func TestSMTPBadCertificateRejected(t *testing.T) {
	s := newFakeSMTP(t)
	s.tlsCfg = &tls.Config{Certificates: []tls.Certificate{selfSignedTLS(t)}}
	s.advertiseSTARTTLS = true

	sender := senderFor(t, s, func(c *config.SMTPConfig) { c.TLS = config.SMTPTLSStartTLS })
	err := sender.Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("send succeeded over an untrusted certificate, want a failure")
	}
	if !service.IsPermanent(err) {
		t.Errorf("class = %v, want permanent", err)
	}
	if got := s.messageCount(); got != 0 {
		t.Errorf("messages = %d, want 0", got)
	}
}

// A 250 after the dot is acceptance; a later QUIT failure must not turn it into a retry.
func TestSMTPQuitErrorAfter250IsSuccess(t *testing.T) {
	s := newFakeSMTP(t)
	s.dropAfterReply = true

	sender := senderFor(t, s, nil)
	if err := sender.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got := s.messageCount(); got != 1 {
		t.Fatalf("messages = %d, want 1", got)
	}
}

func TestSMTPSenderClassifiesReplies(t *testing.T) {
	ctx := context.Background()

	t.Run("5xx is permanent", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.onMessage = func(string) (int, string, bool) { return 550, "no such user", false }
		err := senderFor(t, s, nil).Send(ctx, testMessage())
		if !service.IsPermanent(err) {
			t.Fatalf("err = %v, want permanent", err)
		}
	})
	t.Run("4xx is transient", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.onMessage = func(string) (int, string, bool) { return 451, "try later", false }
		err := senderFor(t, s, nil).Send(ctx, testMessage())
		var de *service.DeliveryError
		if !errors.As(err, &de) || de.Class != service.ClassTransient {
			t.Fatalf("err = %v, want transient", err)
		}
	})
	t.Run("lost final reply is uncertain", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.onMessage = func(string) (int, string, bool) { return 0, "", true }
		err := senderFor(t, s, nil).Send(ctx, testMessage())
		var de *service.DeliveryError
		if !errors.As(err, &de) || de.Class != service.ClassUncertain {
			t.Fatalf("err = %v, want uncertain", err)
		}
		if got := s.messageCount(); got != 1 {
			t.Errorf("messages = %d, want 1: the body was already written", got)
		}
	})
	t.Run("535 is a permanent authentication failure", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.authEnabled = true
		s.authReplyCode = 535
		err := senderFor(t, s, func(c *config.SMTPConfig) { c.Username = "user"; c.Password = "hunter2" }).Send(ctx, testMessage())
		if !service.IsPermanent(err) {
			t.Fatalf("err = %v, want permanent", err)
		}
		if strings.Contains(err.Error(), "hunter2") {
			t.Errorf("error leaks the SMTP password: %q", err.Error())
		}
	})
	t.Run("a 4xx authentication reply is transient", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.authEnabled = true
		s.authReplyCode = 454
		err := senderFor(t, s, func(c *config.SMTPConfig) { c.Username = "user"; c.Password = "hunter2" }).Send(ctx, testMessage())
		var de *service.DeliveryError
		if !errors.As(err, &de) || de.Class != service.ClassTransient {
			t.Fatalf("err = %v, want transient", err)
		}
	})
	t.Run("a 4xx STARTTLS reply is transient", func(t *testing.T) {
		s := newFakeSMTP(t)
		s.tlsCfg = &tls.Config{Certificates: []tls.Certificate{selfSignedTLS(t)}}
		s.advertiseSTARTTLS = true
		s.startTLSReplyCode = 454
		s.startTLSReplyText = "try again later"
		err := senderFor(t, s, func(c *config.SMTPConfig) { c.TLS = config.SMTPTLSStartTLS }).Send(ctx, testMessage())
		var de *service.DeliveryError
		if !errors.As(err, &de) || de.Class != service.ClassTransient {
			t.Fatalf("err = %v, want transient", err)
		}
		if got := s.messageCount(); got != 0 {
			t.Errorf("messages = %d, want 0: the session was never encrypted", got)
		}
	})
}

// PlainAuth's reply-less refusal to send a password must be permanent, not retried for hours.
func TestSMTPCleartextAuthRefusalIsPermanent(t *testing.T) {
	// 127.0.0.2 is loopback but not one of the names PlainAuth trusts, so the refusal is real.
	s := newFakeSMTPOn(t, "127.0.0.2:0")
	s.authEnabled = true
	host, port := s.hostPort(t)

	cfg := config.SMTPConfig{Host: host, Port: port, From: "no-reply@tarunaeduka.id", TLS: config.SMTPTLSNone,
		Username: "relay", Password: "hunter2"}
	err := NewSMTPSender(cfg, testLogger()).Send(context.Background(), testMessage())
	if err == nil {
		t.Fatal("send succeeded over cleartext credentials, want the PlainAuth refusal")
	}
	if got := s.connectionCount(); got != 1 {
		t.Fatalf("connections = %d, want 1: the refusal must happen at AUTH, not at dial", got)
	}
	if !service.IsPermanent(err) {
		t.Errorf("class = %v, want permanent: a refusal is a configuration error, not a retry", err)
	}
	if err.Error() != "smtp: authentication failed" {
		t.Errorf("err = %q, want the fixed %q", err.Error(), "smtp: authentication failed")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("error leaks the SMTP password: %q", err.Error())
	}
	if got := s.messageCount(); got != 0 {
		t.Errorf("messages = %d, want 0: no body is sent without authentication", got)
	}
}
