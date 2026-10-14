package infra

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a throwaway SMTP server built from the standard library with scriptable replies.
type fakeSMTP struct {
	t    *testing.T
	ln   net.Listener
	addr string

	tlsCfg            *tls.Config
	implicitTLS       bool
	advertiseSTARTTLS bool
	authEnabled       bool
	authReplyCode     int
	// authReplyText is the provider's own text; tests use it to echo secrets back at the client.
	authReplyText string
	// startTLSReplyCode, when non-zero, is the reply to STARTTLS instead of 220.
	startTLSReplyCode int
	startTLSReplyText string

	// onMessage decides the final reply for one accepted message. drop means close without replying.
	onMessage func(msg string) (code int, text string, drop bool)
	// dropAfterReply hangs up right after a successful final reply, so a later QUIT fails.
	dropAfterReply bool
	// greetGate, when non-nil, is awaited after accept and before the 220 greeting.
	greetGate chan struct{}
	// messageSignal, when non-nil, receives one token per fully read message.
	messageSignal chan struct{}

	mu    sync.Mutex
	mails []string
	conns int
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	return newFakeSMTPOn(t, "127.0.0.1:0")
}

// newFakeSMTPOn lets a test choose the address, so a server can answer on a name PlainAuth distrusts.
func newFakeSMTPOn(t *testing.T, addr string) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen on %s: %v", addr, err)
	}
	s := &fakeSMTP{t: t, ln: ln, addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeSMTP) hostPort(t *testing.T) (string, string) {
	t.Helper()
	host, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		t.Fatalf("split %q: %v", s.addr, err)
	}
	return host, port
}

func (s *fakeSMTP) messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mails...)
}

// setOnMessage and handler guard the hook: tests swap the script while the server keeps serving.
func (s *fakeSMTP) setOnMessage(fn func(msg string) (code int, text string, drop bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMessage = fn
}

func (s *fakeSMTP) handler() func(msg string) (code int, text string, drop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.onMessage
}

func (s *fakeSMTP) messageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.mails)
}

func (s *fakeSMTP) connectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

func (s *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	s.conns++
	s.mu.Unlock()

	if s.implicitTLS {
		tc := tls.Server(conn, s.tlsCfg)
		if err := tc.Handshake(); err != nil {
			return
		}
		conn = tc
	}

	tp := textproto.NewConn(conn)
	defer tp.Close()

	if s.greetGate != nil {
		<-s.greetGate
	}
	if err := tp.PrintfLine("220 fake ESMTP"); err != nil {
		return
	}

	isTLS := s.implicitTLS
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "EHLO", "HELO":
			if err := writeEHLO(tp, s.tlsCfg != nil && s.advertiseSTARTTLS && !isTLS, s.authEnabled); err != nil {
				return
			}
		case "STARTTLS":
			if s.tlsCfg == nil || !s.advertiseSTARTTLS {
				if err := tp.PrintfLine("502 not supported"); err != nil {
					return
				}
				continue
			}
			if s.startTLSReplyCode != 0 {
				if err := tp.PrintfLine("%d %s", s.startTLSReplyCode, s.startTLSReplyText); err != nil {
					return
				}
				continue
			}
			if err := tp.PrintfLine("220 ready"); err != nil {
				return
			}
			tc := tls.Server(conn, s.tlsCfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			conn = tc
			tp = textproto.NewConn(conn)
			isTLS = true
		case "AUTH":
			if s.authReplyCode != 0 {
				text := s.authReplyText
				if text == "" {
					text = "authentication failed"
				}
				if err := tp.PrintfLine("%d %s", s.authReplyCode, text); err != nil {
					return
				}
				continue
			}
			if err := tp.PrintfLine("235 ok"); err != nil {
				return
			}
		case "MAIL", "RCPT":
			if err := tp.PrintfLine("250 ok"); err != nil {
				return
			}
		case "DATA":
			if err := tp.PrintfLine("354 go ahead"); err != nil {
				return
			}
			raw, _ := io.ReadAll(tp.DotReader())
			s.mu.Lock()
			s.mails = append(s.mails, string(raw))
			s.mu.Unlock()
			if s.messageSignal != nil {
				select {
				case s.messageSignal <- struct{}{}:
				default:
				}
			}
			code, text, drop := 250, "ok", false
			if h := s.handler(); h != nil {
				code, text, drop = h(string(raw))
			}
			if drop {
				return
			}
			if err := tp.PrintfLine("%d %s", code, text); err != nil {
				return
			}
			if s.dropAfterReply {
				return
			}
		case "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			if err := tp.PrintfLine("502 not implemented"); err != nil {
				return
			}
		}
	}
}

func writeEHLO(tp *textproto.Conn, starttls, auth bool) error {
	lines := []string{"250-fake"}
	if starttls {
		lines = append(lines, "250-STARTTLS")
	}
	if auth {
		lines = append(lines, "250-AUTH PLAIN LOGIN")
	}
	lines = append(lines, "250 8BITMIME")
	for _, l := range lines {
		if err := tp.PrintfLine("%s", l); err != nil {
			return err
		}
	}
	return nil
}

// selfSignedTLS produces a certificate no client trusts, which the bad-certificate test needs.
func selfSignedTLS(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost", "127.0.0.1"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
