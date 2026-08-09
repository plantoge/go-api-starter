package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

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
			errs = append(errs, key+" is required")
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
			errs = append(errs, key+" must be an integer: "+err.Error())
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
			errs = append(errs, key+" must be a duration (e.g. 15m): "+err.Error())
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
		errs = append(errs, "JWT_SECRET must be at least 32 characters")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
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
