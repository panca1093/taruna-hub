package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
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

// S3Config is S3-compatible object storage: MinIO locally, GCS through its S3 interoperability in
// production. An empty Endpoint means the asset routes answer 503 and nothing else is affected.
type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	Region    string
}

// BankConfig is where manual transfers are sent; the one tenant's account, never taken from a request.
// Without all three fields manual checkout answers 503 and nothing else is affected.
type BankConfig struct {
	Name          string
	AccountNumber string
	AccountHolder string
}

// MidtransConfig is the one platform-owned merchant account (D16). An empty ServerKey means the gateway method is
// simply not offered. Sandbox is the default; production must be asked for.
type MidtransConfig struct {
	ServerKey  string
	Production bool
	// NotificationURL overrides the dashboard's notification URL for each transaction; optional.
	NotificationURL string
}

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
	// RedisURL holds the auth throttles: disposable counters, so losing Redis only resets them.
	RedisURL      string
	CORSOrigins   []string
	ShutdownGrace time.Duration
	// TenantSlug is the one tenant this deployment serves (D17).
	TenantSlug string
	SMTP       SMTPConfig
	S3         S3Config
	Bank       BankConfig
	Midtrans   MidtransConfig
	// NotificationSealKey seals the codes and reset links queued for delivery. Required everywhere.
	NotificationSealKey []byte
	// AppURL is the web origin that links in emails point at; never taken from a request.
	AppURL string
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
		TenantSlug:    strings.ToLower(env("TENANT_SLUG", "")),
		SMTP: SMTPConfig{
			Host:     env("SMTP_HOST", ""),
			Port:     env("SMTP_PORT", "587"),
			Username: env("SMTP_USERNAME", ""),
			Password: env("SMTP_PASSWORD", ""),
			From:     env("SMTP_FROM", ""),
			TLS:      env("SMTP_TLS", SMTPTLSStartTLS),
		},
	}
	c.Bank = BankConfig{
		Name:          env("BANK_NAME", ""),
		AccountNumber: env("BANK_ACCOUNT_NUMBER", ""),
		AccountHolder: env("BANK_ACCOUNT_HOLDER", ""),
	}
	c.Midtrans = MidtransConfig{
		ServerKey:       env("MIDTRANS_SERVER_KEY", ""),
		NotificationURL: env("MIDTRANS_NOTIFICATION_URL", ""),
	}
	if raw := env("MIDTRANS_IS_PRODUCTION", ""); raw != "" {
		production, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MIDTRANS_IS_PRODUCTION must be true or false, got %q", raw)
		}
		c.Midtrans.Production = production
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	c.RedisURL = env("REDIS_URL", "")
	if c.RedisURL == "" {
		if !c.IsDevelopment() {
			return Config{}, fmt.Errorf("REDIS_URL is required outside development")
		}
		c.RedisURL = "redis://localhost:6380/0"
	}
	s3, err := loadS3(c.IsDevelopment())
	if err != nil {
		return Config{}, err
	}
	c.S3 = s3
	if c.TenantSlug == "" {
		return Config{}, fmt.Errorf("TENANT_SLUG is required")
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

	c.AppURL = strings.TrimRight(env("APP_URL", ""), "/")
	if c.AppURL == "" {
		if !c.IsDevelopment() {
			return Config{}, fmt.Errorf("APP_URL is required outside development")
		}
		c.AppURL = "http://localhost:3000"
	}
	appURL, err := url.Parse(c.AppURL)
	if err != nil || appURL.Host == "" || appURL.Path != "" || appURL.RawQuery != "" || appURL.Fragment != "" {
		return Config{}, fmt.Errorf("APP_URL must be an origin such as https://example.com, got %q", c.AppURL)
	}
	if appURL.Scheme != "https" && !(appURL.Scheme == "http" && c.IsDevelopment()) {
		return Config{}, fmt.Errorf("APP_URL must be https outside development, got %q", c.AppURL)
	}
	return c, nil
}

func loadS3(development bool) (S3Config, error) {
	s := S3Config{
		Endpoint:  env("S3_ENDPOINT", ""),
		AccessKey: env("S3_ACCESS_KEY", ""),
		SecretKey: env("S3_SECRET_KEY", ""),
		Bucket:    env("S3_BUCKET", ""),
		Region:    env("S3_REGION", ""),
		UseSSL:    !development,
	}
	if raw := env("S3_USE_SSL", ""); raw != "" {
		ssl, err := strconv.ParseBool(raw)
		if err != nil {
			return S3Config{}, fmt.Errorf("S3_USE_SSL must be true or false, got %q", raw)
		}
		s.UseSSL = ssl
	}
	if s.Endpoint != "" && (s.AccessKey == "" || s.SecretKey == "" || s.Bucket == "") {
		return S3Config{}, fmt.Errorf("S3_ACCESS_KEY, S3_SECRET_KEY and S3_BUCKET are required when S3_ENDPOINT is set")
	}
	return s, nil
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
