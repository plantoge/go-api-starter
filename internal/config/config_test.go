package config

import (
	"strings"
	"testing"
	"time"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DB_HOST":              "localhost",
		"DB_PORT":              "5432",
		"DB_USER":              "app",
		"DB_PASSWORD":          "secret",
		"DB_NAME":              "appdb",
		"DB_SSLMODE":           "disable",
		"DB_MAX_OPEN_CONNS":    "25",
		"DB_MAX_IDLE_CONNS":    "5",
		"DB_CONN_MAX_LIFETIME": "5m",
		"APP_PORT":             "8080",
		"APP_ENV":              "development",
		"JWT_SECRET":           "01234567890123456789012345678901",
		"ACCESS_TOKEN_TTL":     "15m",
		"REFRESH_TOKEN_TTL":    "168h",
		"LOGIN_MAX_ATTEMPTS":   "5",
		"LOGIN_ATTEMPT_WINDOW": "15m",
		"SHUTDOWN_TIMEOUT":     "15s",
		"CORS_ALLOWED_ORIGINS": "http://localhost:5173,https://acme.com",
	}
}

func TestLoad_Valid(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want localhost", cfg.DB.Host)
	}
	if cfg.JWT.AccessTokenTTL != 15*time.Minute {
		t.Errorf("JWT.AccessTokenTTL = %v, want 15m", cfg.JWT.AccessTokenTTL)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Errorf("CORS.AllowedOrigins = %v, want 2 entries", cfg.CORS.AllowedOrigins)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	env := validEnv()
	delete(env, "DB_HOST")
	delete(env, "JWT_SECRET")
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing required vars, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "DB_HOST") || !strings.Contains(msg, "JWT_SECRET") {
		t.Errorf("error %q does not mention both missing vars", msg)
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	env := validEnv()
	env["ACCESS_TOKEN_TTL"] = "not-a-duration"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid duration, got nil")
	}
}
