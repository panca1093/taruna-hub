package infra

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/config"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	smtpDialTimeout  = 10 * time.Second
	smtpTotalTimeout = 30 * time.Second
)

// SMTPSender speaks SMTP explicitly, so a QUIT failure after acceptance is never a delivery failure.
type SMTPSender struct {
	cfg    config.SMTPConfig
	logger *slog.Logger
}

func NewSMTPSender(cfg config.SMTPConfig, logger *slog.Logger) *SMTPSender {
	if logger == nil {
		logger = slog.Default()
	}
	return &SMTPSender{cfg: cfg, logger: logger}
}

// NewSenders is the whole P02 channel set: a real SMTP transport and an unconfigured WhatsApp adapter.
func NewSenders(cfg config.SMTPConfig, logger *slog.Logger) map[string]service.Sender {
	return map[string]service.Sender{
		string(model.ChannelEmail):    NewSMTPSender(cfg, logger),
		string(model.ChannelWhatsApp): UnavailableSender{},
	}
}

// UnavailableSender is the WhatsApp adapter: it reports itself unconfigured and never claims delivery.
type UnavailableSender struct{}

func (UnavailableSender) Configured() bool { return false }

func (UnavailableSender) Send(context.Context, service.Message) error {
	return &service.DeliveryError{Class: service.ClassPermanent, Reason: "channel is not configured"}
}

func (s *SMTPSender) Configured() bool { return s.cfg.Host != "" }

func (s *SMTPSender) Send(ctx context.Context, msg service.Message) error {
	if err := s.validate(msg); err != nil {
		return err
	}

	conn, err := (&net.Dialer{Timeout: smtpDialTimeout}).DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, s.cfg.Port))
	if err != nil {
		return transient("smtp: connection failed")
	}
	defer conn.Close()

	// Abort the session if the caller gives up; raw is captured because conn is reassigned below.
	raw := conn
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = raw.Close()
		case <-stop:
		}
	}()

	if s.cfg.TLS == config.SMTPTLSImplicit {
		tlsConn := tls.Client(conn, s.tlsConfig())
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return handshakeClass(err)
		}
		conn = tlsConn
	}
	if err := conn.SetDeadline(time.Now().Add(smtpTotalTimeout)); err != nil {
		return transient("smtp: connection failed")
	}

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return transient("smtp: connection failed")
	}
	defer c.Close()

	if err := c.Hello("localhost"); err != nil {
		return replyClass(err)
	}
	if s.cfg.TLS == config.SMTPTLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			// Never fall back to cleartext on a server that cannot upgrade.
			return permanent("smtp: server does not offer STARTTLS")
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return handshakeClass(err)
		}
	}
	if s.cfg.Username != "" {
		if err := c.Auth(refusingPlainAuth{inner: smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)}); err != nil {
			return authClass(err)
		}
	}
	if err := c.Mail(s.cfg.From); err != nil {
		return replyClass(err)
	}
	if err := c.Rcpt(msg.Recipient); err != nil {
		return replyClass(err)
	}

	body, err := s.message(msg)
	if err != nil {
		return err
	}

	// Past this point the message may already be in the provider's hands, so failures are uncertain.
	w, err := c.Data()
	if err != nil {
		return replyClass(err)
	}
	if _, err := w.Write(body); err != nil {
		return uncertain()
	}
	if err := w.Close(); err != nil {
		return dataClass(err)
	}
	// A QUIT failure after acceptance is not a delivery failure.
	_ = c.Quit()
	return nil
}

func (s *SMTPSender) tlsConfig() *tls.Config {
	return &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
}

func (s *SMTPSender) validate(msg service.Message) error {
	for name, value := range map[string]string{
		"sender":    s.cfg.From,
		"recipient": msg.Recipient,
		"subject":   msg.Subject,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return permanent("smtp: invalid " + name)
		}
	}
	if strings.TrimSpace(msg.Recipient) == "" || strings.TrimSpace(s.cfg.From) == "" {
		return permanent("smtp: invalid recipient")
	}
	return nil
}

func (s *SMTPSender) message(msg service.Message) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", uuid.NewString(), s.messageIDDomain())
	fmt.Fprintf(&b, "From: %s\r\n", s.cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", msg.Recipient)
	// Word-encoding keeps the subject on one header line whatever the template produces.
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.Body)
	return []byte(b.String()), nil
}

func (s *SMTPSender) messageIDDomain() string {
	if _, domain, ok := strings.Cut(s.cfg.From, "@"); ok && domain != "" {
		return domain
	}
	return s.cfg.Host
}

func transient(reason string) error {
	return &service.DeliveryError{Class: service.ClassTransient, Reason: reason}
}

func permanent(reason string) error {
	return &service.DeliveryError{Class: service.ClassPermanent, Reason: reason}
}

func uncertain() error {
	return &service.DeliveryError{Class: service.ClassUncertain, Reason: "smtp: delivery outcome uncertain"}
}

// replyClass keeps the transient/permanent class of a reply but never its text.
func replyClass(err error) error {
	return classify(err, false)
}

// authClass keeps the class of an AUTH reply but never its text.
func authClass(err error) error {
	if code, ok := replyCode(err); ok {
		if code >= 500 {
			return permanent("smtp: authentication failed")
		}
		return transient("smtp: transient failure")
	}
	var refusal *authRefusal
	if errors.As(err, &refusal) {
		// The mechanism itself declined to send credentials, which no retry can change.
		return permanent("smtp: authentication failed")
	}
	return transient("smtp: transient failure")
}

// refusingPlainAuth tags PlainAuth's reply-less refusal so authClass can make it permanent.
type refusingPlainAuth struct{ inner smtp.Auth }

func (a refusingPlainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	mech, resp, err := a.inner.Start(server)
	if err != nil {
		return "", nil, &authRefusal{err: err}
	}
	return mech, resp, nil
}

func (a refusingPlainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	return a.inner.Next(fromServer, more)
}

type authRefusal struct{ err error }

func (e *authRefusal) Error() string { return e.err.Error() }
func (e *authRefusal) Unwrap() error { return e.err }

// handshakeClass fails closed on a certificate or a 5xx reply, and retries a 4xx or transport error.
func handshakeClass(err error) error {
	var certErr *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	if errors.As(err, &certErr) || errors.As(err, &unknownAuthority) ||
		errors.As(err, &hostname) || errors.As(err, &invalidCertificate) {
		return permanent("smtp: tls handshake failed")
	}
	if code, ok := replyCode(err); ok && code >= 500 {
		return permanent("smtp: tls handshake failed")
	}
	return transient("smtp: transient failure")
}

// replyCode extracts a numeric SMTP reply code without keeping any of its text.
func replyCode(err error) (int, bool) {
	var tp *textproto.Error
	if errors.As(err, &tp) {
		return tp.Code, true
	}
	return 0, false
}

// dataClass treats a definite reply after DATA as definite; only a lost reply is uncertain.
func dataClass(err error) error {
	return classify(err, true)
}

func classify(err error, afterData bool) error {
	var tp *textproto.Error
	if errors.As(err, &tp) {
		if tp.Code >= 400 && tp.Code < 500 {
			return transient("smtp: transient failure")
		}
		if tp.Code >= 500 {
			return permanent("smtp: permanent rejection (code " + strconv.Itoa(tp.Code) + ")")
		}
	}
	if afterData {
		return uncertain()
	}
	return transient("smtp: transient failure")
}
