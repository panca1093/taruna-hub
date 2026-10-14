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

// SMTP transport modes.
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
	// NotificationSealKey decrypts secret-bearing notification payloads; nil forbids them.
	NotificationSealKey []byte
}

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
	if raw := env("NOTIFICATION_SEAL_KEY", ""); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return Config{}, fmt.Errorf("NOTIFICATION_SEAL_KEY is not valid base64: %w", err)
		}
		if len(key) != service.SealKeyBytes {
			return Config{}, fmt.Errorf("NOTIFICATION_SEAL_KEY must decode to exactly %d bytes, got %d", service.SealKeyBytes, len(key))
		}
		c.NotificationSealKey = key
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
	default:
		return fmt.Errorf("SMTP_TLS must be one of %s, %s, %s", SMTPTLSStartTLS, SMTPTLSImplicit, SMTPTLSNone)
	}
	if s.Host != "" && s.From == "" {
		return fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
	}
	return nil
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
