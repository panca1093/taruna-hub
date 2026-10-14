package config

import "testing"

func TestLoadRequiresBaseDomainOutsideDevelopment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("BASE_DOMAIN", "")

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
