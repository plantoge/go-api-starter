package config

import (
	"errors"
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

// Error() wajib tetap satu baris: slog ngasih tanda kutip ke nilai atribut
// yang mengandung baris baru, jadi "\n"-nya bakal tercetak apa adanya,
// bukan ganti baris beneran. Multiline() yang versi buat terminal, dan
// cuma dia yang boleh mengandung baris baru.
func TestValidationError_Rendering(t *testing.T) {
	ve := &ValidationError{Problems: []string{"DB_HOST wajib diisi", "JWT_SECRET minimal 32 karakter"}}

	if strings.Contains(ve.Error(), "\n") {
		t.Errorf("Error() mengandung baris baru: %q", ve.Error())
	}
	for _, p := range ve.Problems {
		if !strings.Contains(ve.Error(), p) {
			t.Errorf("Error() tidak menyebut %q", p)
		}
		if !strings.Contains(ve.Multiline(), "  - "+p+"\n") {
			t.Errorf("Multiline() tidak memuat baris untuk %q", p)
		}
	}
}

func TestExplain_UsesMultilineForValidationError(t *testing.T) {
	ve := &ValidationError{Problems: []string{"DB_HOST wajib diisi"}}
	if got := Explain(ve); got != ve.Multiline() {
		t.Errorf("Explain() = %q, ingin bentuk Multiline()", got)
	}

	plain := errors.New("kegagalan lain")
	if got := Explain(plain); got != "kegagalan lain" {
		t.Errorf("Explain() = %q, ingin pesan asli apa adanya", got)
	}
}

func TestLoad_ReturnsValidationError(t *testing.T) {
	env := validEnv()
	delete(env, "DB_HOST")
	setEnv(t, env)

	_, err := Load()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Load() error bertipe %T, ingin *ValidationError", err)
	}
	if len(ve.Problems) == 0 {
		t.Error("ValidationError.Problems kosong")
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
