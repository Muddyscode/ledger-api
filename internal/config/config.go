package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	JWTSecret     string
	AllowRegister bool
	Seed          bool
	SeedEmail     string
	SeedPassword  string
}

func FromEnv() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("LEDGER_DATABASE_URL"),
		HTTPAddr:      getenv("LEDGER_HTTP_ADDR", ":8080"),
		JWTSecret:     os.Getenv("LEDGER_JWT_SECRET"),
		AllowRegister: boolEnv("LEDGER_ALLOW_REGISTER", true),
		Seed:          boolEnv("LEDGER_SEED", false),
		SeedEmail:     getenv("LEDGER_SEED_EMAIL", "operator@ledger.local"),
		SeedPassword:  getenv("LEDGER_SEED_PASSWORD", "change-me-now-12"),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("LEDGER_DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("LEDGER_JWT_SECRET must be at least 32 characters")
	}
	if c.Seed && len(c.SeedPassword) < 12 {
		return Config{}, fmt.Errorf("LEDGER_SEED_PASSWORD must be at least 12 characters")
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func boolEnv(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
