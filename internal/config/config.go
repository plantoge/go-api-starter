package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// ValidationError collects every configuration problem at once, so a
// misconfigured deploy fails once with a complete list instead of one env
// var per attempt.
//
// It renders two ways because the readers differ. Error() is the form that
// ends up in structured logs: a single line, because slog's handlers quote
// any value containing newlines — an embedded "\n" would be printed
// literally as two characters rather than breaking the line. Multiline() is
// for a terminal, where one problem per line is far easier to scan.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "konfigurasi tidak valid: " + strings.Join(e.Problems, "; ")
}

func (e *ValidationError) Multiline() string {
	var b strings.Builder
	b.WriteString("Konfigurasi tidak valid — perbaiki dulu sebelum menjalankan aplikasi:\n")
	for _, p := range e.Problems {
		b.WriteString("  - ")
		b.WriteString(p)
		b.WriteString("\n")
	}
	b.WriteString("\nSalin .env.example menjadi .env lalu isi nilainya — lihat docs/getting-started.md.")
	return b.String()
}

// Explain picks the most readable rendering of an error for terminal
// output. Both cmd/api and cmd/cli use it so a configuration failure looks
// the same whichever binary hit it.
func Explain(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Multiline()
	}
	return err.Error()
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

func (c DBConfig) PlatformDSN() string {
	return c.DSN() + "&search_path=platform"
}

type AppConfig struct {
	Port            int
	Env             string
	ShutdownTimeout time.Duration
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type LoginConfig struct {
	MaxAttempts   int
	AttemptWindow time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type Config struct {
	DB    DBConfig
	App   AppConfig
	JWT   JWTConfig
	Login LoginConfig
	CORS  CORSConfig
}

// Load reads configuration from the process environment (loading a local
// .env file first, if one exists) and validates it. Every problem is
// collected and returned together so a misconfigured deploy fails once,
// with a complete list, instead of one env var at a time.
func Load() (*Config, error) {
	_ = godotenv.Load() // optional; real deploys set env vars directly

	var errs []string
	req := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			errs = append(errs, key+" wajib diisi")
		}
		return v
	}
	reqInt := func(key string) int {
		raw := req(key)
		if raw == "" {
			return 0
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s harus berupa bilangan bulat, dapat %q", key, raw))
		}
		return n
	}
	reqDuration := func(key string) time.Duration {
		raw := req(key)
		if raw == "" {
			return 0
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s harus berupa durasi seperti 15m, 168h, atau 30s, dapat %q", key, raw))
		}
		return d
	}

	cfg := &Config{
		DB: DBConfig{
			Host:            req("DB_HOST"),
			Port:            reqInt("DB_PORT"),
			User:            req("DB_USER"),
			Password:        req("DB_PASSWORD"),
			Name:            req("DB_NAME"),
			SSLMode:         req("DB_SSLMODE"),
			MaxOpenConns:    reqInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    reqInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: reqDuration("DB_CONN_MAX_LIFETIME"),
		},
		App: AppConfig{
			Port:            reqInt("APP_PORT"),
			Env:             req("APP_ENV"),
			ShutdownTimeout: reqDuration("SHUTDOWN_TIMEOUT"),
		},
		JWT: JWTConfig{
			Secret:          req("JWT_SECRET"),
			AccessTokenTTL:  reqDuration("ACCESS_TOKEN_TTL"),
			RefreshTokenTTL: reqDuration("REFRESH_TOKEN_TTL"),
		},
		Login: LoginConfig{
			MaxAttempts:   reqInt("LOGIN_MAX_ATTEMPTS"),
			AttemptWindow: reqDuration("LOGIN_ATTEMPT_WINDOW"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(req("CORS_ALLOWED_ORIGINS")),
		},
	}

	// A short/weak JWT_SECRET is a real production risk, not just a style
	// nit — anyone who can guess or brute-force it can forge access
	// tokens for any user. .env.example ships a placeholder value that
	// looks random but is publicly known (it's in version control); this
	// check exists mainly to catch a deploy that copied .env.example
	// verbatim without ever editing it, rather than to enforce true
	// entropy (which a length check can't measure anyway).
	if cfg.JWT.Secret != "" && len(cfg.JWT.Secret) < 32 {
		errs = append(errs, "JWT_SECRET minimal 32 karakter")
	}

	if len(errs) > 0 {
		return nil, &ValidationError{Problems: errs}
	}
	return cfg, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
