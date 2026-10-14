package config

import (
	"encoding/base64"
	"testing"
)

func TestSMTPConfigValidation(t *testing.T) {
	base := func() {
		t.Setenv("APP_ENV", "production")
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("BASE_DOMAIN", "tarunaeduka.id")
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_TLS", "starttls")
		t.Setenv("SMTP_FROM", "")
		t.Setenv("NOTIFICATION_SEAL_KEY", "")
		// Every environment needs a signing key, so the negative cases below stay about SMTP.
		t.Setenv("JWT_SECRET", validJWTSecret())
	}

	t.Run("host without from", func(t *testing.T) {
		base()
		t.Setenv("SMTP_HOST", "smtp.example.com")
		if _, err := Load(); err == nil {
			t.Error("SMTP_HOST without SMTP_FROM was accepted, want an error")
		}
	})

	t.Run("bad port", func(t *testing.T) {
		base()
		t.Setenv("SMTP_PORT", "not-a-port")
		if _, err := Load(); err == nil {
			t.Error("an unparsable SMTP_PORT was accepted, want an error")
		}
	})

	t.Run("tls none outside development", func(t *testing.T) {
		base()
		t.Setenv("SMTP_TLS", "none")
		if _, err := Load(); err == nil {
			t.Error("SMTP_TLS=none outside development was accepted, want an error")
		}
		t.Setenv("APP_ENV", "development")
		cfg, err := Load()
		if err != nil || cfg.SMTP.TLS != SMTPTLSNone {
			t.Errorf("development: tls = %q, err = %v; want none, nil", cfg.SMTP.TLS, err)
		}
	})

	// PlainAuth cannot send a password to a remote cleartext server, so this pairing only ever fails.
	t.Run("cleartext credentials need a loopback host", func(t *testing.T) {
		base()
		t.Setenv("APP_ENV", "development")
		t.Setenv("SMTP_TLS", "none")
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_FROM", "no-reply@tarunaeduka.id")
		t.Setenv("SMTP_USERNAME", "relay")
		t.Setenv("SMTP_PASSWORD", "hunter2")
		if _, err := Load(); err == nil {
			t.Error("cleartext credentials against a remote host were accepted, want an error")
		}

		for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
			t.Setenv("SMTP_HOST", host)
			if _, err := Load(); err != nil {
				t.Errorf("cleartext credentials against %s: %v, want nil", host, err)
			}
		}

		// An unauthenticated cleartext relay stays legal: there is no credential to protect.
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_USERNAME", "")
		if _, err := Load(); err != nil {
			t.Errorf("cleartext without credentials: %v, want nil", err)
		}
	})

	t.Run("unknown tls mode", func(t *testing.T) {
		base()
		t.Setenv("SMTP_TLS", "maybe")
		if _, err := Load(); err == nil {
			t.Error("an unknown SMTP_TLS was accepted, want an error")
		}
	})

	t.Run("unset host is legal everywhere", func(t *testing.T) {
		base()
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unset SMTP_HOST: %v, want nil", err)
		}
		if cfg.SMTP.Host != "" || cfg.SMTP.Port != "587" || cfg.SMTP.TLS != SMTPTLSStartTLS {
			t.Errorf("defaults = %+v, want empty host, port 587, starttls", cfg.SMTP)
		}
	})

	t.Run("seal key must decode to 32 bytes", func(t *testing.T) {
		base()
		t.Setenv("NOTIFICATION_SEAL_KEY", "not base64!!")
		if _, err := Load(); err == nil {
			t.Error("a non-base64 seal key was accepted, want an error")
		}
		t.Setenv("NOTIFICATION_SEAL_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
		if _, err := Load(); err == nil {
			t.Error("a 16-byte seal key was accepted, want an error")
		}
		t.Setenv("NOTIFICATION_SEAL_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
		cfg, err := Load()
		if err != nil || len(cfg.NotificationSealKey) != 32 {
			t.Errorf("seal key = %d bytes, err = %v; want 32, nil", len(cfg.NotificationSealKey), err)
		}
	})
}

// validJWTSecret is the base64 of 32 bytes: the shape every environment's JWT_SECRET must have.
func validJWTSecret() string {
	return base64.StdEncoding.EncodeToString(make([]byte, MinJWTSecretBytes))
}

func TestConfigRequiresJWTSecret(t *testing.T) {
	base := func() {
		t.Setenv("APP_ENV", "development")
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("BASE_DOMAIN", "tarunaeduka.id")
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_TLS", "starttls")
		t.Setenv("SMTP_FROM", "")
		t.Setenv("NOTIFICATION_SEAL_KEY", "")
	}

	for _, env := range []string{"development", "staging", "production"} {
		t.Run(env+" without a secret", func(t *testing.T) {
			base()
			t.Setenv("APP_ENV", env)
			t.Setenv("JWT_SECRET", "")
			if _, err := Load(); err == nil {
				t.Errorf("%s without JWT_SECRET: err = nil, want an error", env)
			}
		})
	}

	t.Run("not base64", func(t *testing.T) {
		base()
		t.Setenv("JWT_SECRET", "not base64!!")
		if _, err := Load(); err == nil {
			t.Error("a non-base64 JWT_SECRET was accepted, want an error")
		}
	})

	t.Run("too short", func(t *testing.T) {
		base()
		t.Setenv("JWT_SECRET", base64.StdEncoding.EncodeToString(make([]byte, MinJWTSecretBytes-1)))
		if _, err := Load(); err == nil {
			t.Error("a 31-byte JWT_SECRET was accepted, want an error")
		}
	})

	t.Run("valid", func(t *testing.T) {
		base()
		t.Setenv("JWT_SECRET", validJWTSecret())
		cfg, err := Load()
		if err != nil || len(cfg.JWTSecret) != MinJWTSecretBytes {
			t.Errorf("JWTSecret = %d bytes, err = %v; want %d, nil", len(cfg.JWTSecret), err, MinJWTSecretBytes)
		}
	})
}

func TestLoadRequiresBaseDomainOutsideDevelopment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("BASE_DOMAIN", "")
	t.Setenv("JWT_SECRET", validJWTSecret())

	t.Setenv("APP_ENV", "development")
	cfg, err := Load()
	if err != nil || cfg.BaseDomain != "localhost" {
		t.Fatalf("development: BaseDomain = %q, err = %v; want localhost, nil", cfg.BaseDomain, err)
	}

	for _, env := range []string{"production", "staging"} {
		t.Setenv("APP_ENV", env)
		if _, err := Load(); err == nil {
			t.Errorf("%s without BASE_DOMAIN: err = nil, want error", env)
		}
	}

	t.Setenv("APP_ENV", "production")
	t.Setenv("BASE_DOMAIN", "TarunaEduka.id")
	cfg, err = Load()
	if err != nil || cfg.BaseDomain != "tarunaeduka.id" {
		t.Errorf("production: BaseDomain = %q, err = %v; want tarunaeduka.id, nil", cfg.BaseDomain, err)
	}
}
