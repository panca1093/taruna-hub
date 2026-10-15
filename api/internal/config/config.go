package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"
)

// SMTPConfig is the email transport; an empty Host means email is simply unavailable.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	TLS      string
}

type Config struct {
	Env                string
	Port               string
	DatabaseURL        string
	CORSOrigins        []string
	ShutdownGrace      time.Duration
	BaseDomain         string
	TrustForwardedHost bool
	SMTP               SMTPConfig
	// NotificationSealKey seals the codes and reset links queued for delivery. Required everywhere.
	NotificationSealKey []byte
	// WebScheme and WebPort place links in emails on <slug>.<BaseDomain>; never taken from a request.
	WebScheme string
	WebPort   string
	// JWTSecret signs access tokens. Required everywhere: an unauthenticated API is not a mode.
	JWTSecret []byte
}

// MinJWTSecretBytes is the shortest acceptable HMAC key; 32 bytes is the SHA-256 block a signer needs.
const MinJWTSecretBytes = 32

func Load() (Config, error) {
	c := Config{
		Env:           env("APP_ENV", "development"),
		Port:          env("PORT", "8081"),
		DatabaseURL:   env("DATABASE_URL", ""),
		CORSOrigins:   split(env("CORS_ORIGINS", "http://localhost:3000")),
		ShutdownGrace: 10 * time.Second,
		BaseDomain:    strings.ToLower(env("BASE_DOMAIN", "")),
		// Only true behind a proxy that strips client-sent X-Forwarded-Host and sets its own.
		TrustForwardedHost: env("TRUST_FORWARDED_HOST", "") == "true",
		SMTP: SMTPConfig{
			Host:     env("SMTP_HOST", ""),
			Port:     env("SMTP_PORT", "587"),
			Username: env("SMTP_USERNAME", ""),
			Password: env("SMTP_PASSWORD", ""),
			From:     env("SMTP_FROM", ""),
			TLS:      env("SMTP_TLS", SMTPTLSStartTLS),
		},
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if c.BaseDomain == "" {
		if !c.IsDevelopment() {
			return Config{}, fmt.Errorf("BASE_DOMAIN is required outside development")
		}
		c.BaseDomain = "localhost"
	}
	if err := c.SMTP.validate(c.IsDevelopment()); err != nil {
		return Config{}, err
	}
	// No environment may start without a signing key: an unauthenticated API is not a mode.
	rawSecret := env("JWT_SECRET", "")
	if rawSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required (generate one with: openssl rand -base64 32)")
	}
	raw, err := base64.StdEncoding.DecodeString(rawSecret)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_SECRET is not valid base64: %w", err)
	}
	if len(raw) < MinJWTSecretBytes {
		return Config{}, fmt.Errorf("JWT_SECRET must decode to at least %d bytes, got %d", MinJWTSecretBytes, len(raw))
	}
	c.JWTSecret = raw
	// Registration codes and reset links are sealed secrets, so signup cannot work without the key.
	rawSeal := env("NOTIFICATION_SEAL_KEY", "")
	if rawSeal == "" {
		return Config{}, fmt.Errorf("NOTIFICATION_SEAL_KEY is required (generate one with: openssl rand -base64 32)")
	}
	key, err := base64.StdEncoding.DecodeString(rawSeal)
	if err != nil {
		return Config{}, fmt.Errorf("NOTIFICATION_SEAL_KEY is not valid base64: %w", err)
	}
	if len(key) != service.SealKeyBytes {
		return Config{}, fmt.Errorf("NOTIFICATION_SEAL_KEY must decode to exactly %d bytes, got %d", service.SealKeyBytes, len(key))
	}
	c.NotificationSealKey = key

	c.WebScheme, c.WebPort = "https", ""
	if c.IsDevelopment() {
		c.WebScheme, c.WebPort = "http", "3000"
	}
	c.WebScheme = env("PUBLIC_WEB_SCHEME", c.WebScheme)
	c.WebPort = env("PUBLIC_WEB_PORT", c.WebPort)
	if c.WebScheme != "https" && !(c.WebScheme == "http" && c.IsDevelopment()) {
		return Config{}, fmt.Errorf("PUBLIC_WEB_SCHEME must be https outside development, got %q", c.WebScheme)
	}
	if c.WebPort != "" {
		if port, err := strconv.Atoi(c.WebPort); err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("PUBLIC_WEB_PORT must be a port number, got %q", c.WebPort)
		}
	}
	return c, nil
}

func (s SMTPConfig) validate(development bool) error {
	if port, err := strconv.Atoi(s.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("SMTP_PORT must be a port number, got %q", s.Port)
	}
	switch s.TLS {
	case SMTPTLSStartTLS, SMTPTLSImplicit:
	case SMTPTLSNone:
		// Cleartext is a development-only convenience; anything else must upgrade.
		if !development {
			return fmt.Errorf("SMTP_TLS=none is only allowed when APP_ENV=development")
		}
		// PlainAuth never sends a password to a remote cleartext host, so this pairing can only fail.
		if s.Username != "" && !isLoopbackHost(s.Host) {
			return fmt.Errorf("SMTP_USERNAME over SMTP_TLS=none is only allowed against a loopback SMTP_HOST")
		}
	default:
		return fmt.Errorf("SMTP_TLS must be one of %s, %s, %s", SMTPTLSStartTLS, SMTPTLSImplicit, SMTPTLSNone)
	}
	if s.Host != "" && s.From == "" {
		return fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func (c Config) IsProduction() bool  { return c.Env == "production" }
func (c Config) IsDevelopment() bool { return c.Env == "development" }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func split(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
