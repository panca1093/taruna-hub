package config

import "testing"

func TestMidtransConfig(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_URL", "redis://x:6379/0")
	t.Setenv("TENANT_SLUG", "taruna")
	t.Setenv("JWT_SECRET", validJWTSecret())
	t.Setenv("NOTIFICATION_SEAL_KEY", validSealKey())
	t.Setenv("MIDTRANS_SERVER_KEY", "")
	t.Setenv("MIDTRANS_IS_PRODUCTION", "")

	cfg, err := Load()
	if err != nil || cfg.Midtrans.ServerKey != "" || cfg.Midtrans.Production {
		t.Fatalf("defaults = %+v, %v; want no key and the sandbox", cfg.Midtrans, err)
	}
	t.Setenv("MIDTRANS_SERVER_KEY", "SB-key")
	t.Setenv("MIDTRANS_IS_PRODUCTION", "true")
	t.Setenv("MIDTRANS_NOTIFICATION_URL", "https://api.example/notify")
	cfg, err = Load()
	if err != nil || cfg.Midtrans.ServerKey != "SB-key" || !cfg.Midtrans.Production || cfg.Midtrans.NotificationURL != "https://api.example/notify" {
		t.Errorf("configured = %+v, %v", cfg.Midtrans, err)
	}
	t.Setenv("MIDTRANS_IS_PRODUCTION", "maybe")
	if _, err := Load(); err == nil {
		t.Error("an unparsable MIDTRANS_IS_PRODUCTION was accepted")
	}
}
