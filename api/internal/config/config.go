package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env         string
	Port        string
	DatabaseURL string
	CORSOrigins []string
	ShutdownGrace time.Duration
}

func Load() (Config, error) {
	c := Config{
		Env:           env("APP_ENV", "development"),
		Port:          env("PORT", "8081"),
		DatabaseURL:   env("DATABASE_URL", ""),
		CORSOrigins:   split(env("CORS_ORIGINS", "http://localhost:3000")),
		ShutdownGrace: 10 * time.Second,
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func (c Config) IsProduction() bool { return c.Env == "production" }

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
