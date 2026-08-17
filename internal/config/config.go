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

// ValidationError ngumpulin semua masalah konfigurasi sekaligus, jadi
// deploy yang salah setelan gagal sekali dengan daftar lengkap — bukan
// satu env var tiap kali dicoba.
//
// Bentuk tampilannya ada dua, karena pembacanya beda. Error() dipakai buat
// log terstruktur: satu baris saja, soalnya handler slog bakal ngasih
// tanda kutip ke nilai yang mengandung baris baru — "\n" di dalamnya bakal
// tercetak apa adanya sebagai dua karakter, bukan ganti baris beneran.
// Multiline() dipakai di terminal, di mana satu masalah per baris jelas
// jauh lebih enak dibaca.
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

// Explain milih bentuk tampilan error yang paling enak dibaca di
// terminal. Dipakai cmd/api maupun cmd/cli, biar kegagalan konfigurasi
// kelihatan sama saja lewat binary mana pun.
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

// Load baca konfigurasi dari environment proses (kalau ada file .env
// lokal, itu dimuat duluan) lalu memvalidasinya. Semua masalah dikumpulin
// dan dikembalikan sekaligus, biar deploy yang salah setelan gagal sekali
// dengan daftar lengkap, bukan satu env var tiap kali.
func Load() (*Config, error) {
	_ = godotenv.Load() // opsional; deploy sungguhan nyetel env var langsung

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

	// JWT_SECRET yang pendek atau lemah itu risiko produksi beneran, bukan
	// sekadar masalah rapi-rapian: siapa pun yang bisa nebak atau
	// nge-brute-force-nya bisa bikin access token palsu atas nama user mana
	// pun. File .env.example emang bawa nilai contoh yang kelihatan acak
	// tapi sebenarnya sudah publik (ada di version control). Pengecekan ini
	// tujuannya lebih ke nangkep deploy yang nyalin .env.example bulat-bulat
	// tanpa pernah diubah, bukan buat ngukur keacakan asli — yang toh nggak
	// bisa diukur cuma dari panjangnya.
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
