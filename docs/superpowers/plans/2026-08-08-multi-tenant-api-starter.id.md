# Rencana Implementasi: Multi-Tenant API Starter

> **Catatan:** Ini adalah terjemahan Bahasa Indonesia dari
> `2026-08-08-multi-tenant-api-starter.md`. Kode (Go/SQL/bash/ini/yaml),
> nama file, dan perintah dibiarkan **apa adanya** — itu yang nanti benar-benar
> ditulis ke file. Yang diterjemahkan hanya penjelasan, judul, catatan desain,
> dan instruksi langkah kerja. Kalau ada perbedaan makna, versi Inggris yang
> jadi acuan resmi.

> **Untuk pekerja agentic:** SUB-SKILL WAJIB: Gunakan superpowers:subagent-driven-development (disarankan) atau superpowers:executing-plans untuk mengerjakan rencana ini tugas demi tugas. Setiap langkah pakai sintaks checkbox (`- [ ]`) untuk pelacakan progres.

**Tujuan:** Membangun starterpack API multi-tenant berbahasa Golang (PostgreSQL satu schema per tenant) lengkap dengan auth platform+tenant, RBAC penuh, satu modul contoh tenant-scoped (`user`), tooling CLI untuk provisioning/migrasi, dan jalur deployment tanpa Docker — dipakai ulang sebagai fondasi project-project berikutnya (kasus pakai pertama: sistem POS yang dibangun di repo terpisah).

**Arsitektur:** Handler HTTP Fiber v2 dibuat setipis mungkin dan langsung mengonversi semuanya jadi `context.Context` (tidak pernah mengoper `*fiber.Ctx` melewati layer handler). Service memegang business logic dan sama sekali tidak tahu soal HTTP. Repository menjalankan SQL lewat sqlx terhadap koneksi yang di-scope per-request ke schema satu tenant lewat `SET LOCAL search_path` di dalam transaksi (`database.WithTenant`). Data platform (tenant, admin platform) hidup di schema `platform` miliknya sendiri, di pool koneksi yang di-pin terpisah.

**Stack Teknologi:** Go 1.25, Fiber v2, PostgreSQL, sqlx + pgx (driver `stdlib`), golang-migrate (sumber embedded `iofs`), golang-jwt/jwt/v5, bcrypt, go-playground/validator/v10, log/slog (JSON), oklog/ulid/v2 (request ID), swaggo/swag, air (hot reload saat development). Tanpa Docker sama sekali — development, testing, dan deploy semuanya menjalankan PostgreSQL dan binary Go langsung di host.

## Batasan Global

- Module path: `go-api-starter` (tanpa prefix host VCS — starter privat, belum dipublikasikan).
- Versi Go: minimal 1.25 (dinaikkan dari 1.22 semula saat Tugas 4 — golang.org/x/crypto/sys/text versi terkini butuh 1.25; mengunci dependency crypto lama demi bertahan di 1.22 dinilai trade-off yang lebih buruk daripada menaikkan floor) (`go.mod`).
- `*fiber.Ctx` tidak boleh pernah dioper ke fungsi service atau repository — handler mengonversinya ke `context.Context` (lewat `c.UserContext()`, yang diisi middleware) sebelum memanggil `service.*`.
- Semua akses data tenant lewat `database.WithTenant(ctx, fn)` — tidak ada repository yang boleh memegang `*sqlx.DB` polos untuk tabel tenant.
- Setiap tabel punya `created_at, updated_at, deleted_at, created_by, updated_by, deleted_by` **kecuali**: tabel penghubung (`role_permissions`, `user_roles` — cuma `created_at`+`created_by`, hard-delete) dan tabel log/token (`refresh_tokens`, `login_attempts` — cuma `created_at`).
- ID berupa UUID yang dibuat di Go (`uuid.New()`), tidak pernah pakai default sisi database — `SET LOCAL search_path` **mengganti** (bukan menambah) path, jadi pemanggilan fungsi extension tanpa qualifier di dalam transaksi tenant tidak bisa diandalkan.
- Parameter `sort` di setiap endpoint list wajib divalidasi lewat peta whitelist per-repository yang eksplisit — tidak pernah diinterpolasi langsung dari request.
- Password di-hash pakai bcrypt, cost 12. Minimal 8 karakter, dicek terhadap daftar pendek password umum.
- JWT membawa `sub`, `scope` (`platform`|`tenant`), `tenant_id` (khusus scope tenant), `exp`. Tidak pernah membawa permission.
- Bentuk response: `{"success": true, "data": ...}` / `{"success": false, "error": {"code","message","details"}, "request_id": ...}`. Error validasi menaruh `field -> []pesan` di `details`, memakai nama field versi JSON.
- Semua rute HTTP ada di bawah `/api/v1`.
- Tanpa Docker. Tanpa testcontainers — integration test jalan terhadap database PostgreSQL lokal `app_test`, config dari `.env.test`.
- **Konvensi bahasa:** Identifier Go (nama fungsi, tipe, variabel, package) tetap Bahasa Inggris — itu konvensi idiomatic Go dan menjaga kompatibilitas dengan dependency-nya yang semuanya berbahasa Inggris (Fiber, sqlx, dll). Semua teks yang dibaca manusia dan bukan identifier — komentar kode, output CLI (`fmt.Println`/`fmt.Printf`), pesan log, dan pesan error/response API — ditulis dalam Bahasa Indonesia. Setiap potongan kode di bawah yang punya komentar atau string CLI berbahasa Inggris harus diterjemahkan ke Indonesia saat benar-benar diketik ke file; hanya potongan yang kebetulan sudah berupa teks Indonesia murni (kebanyakan pesan `apperror.*` sudah begitu) yang siap disalin apa adanya. Contoh:
  ```go
  // Bahasa Inggris (seperti tertulis di potongan kode rencana ini — terjemahkan dulu sebelum disimpan):
  // Login is the one place tenant identity is established without a token.
  fmt.Println("tenant provisioned:")

  // Bahasa Indonesia (yang seharusnya ada di file sungguhan):
  // Login adalah satu-satunya tempat identitas tenant ditetapkan tanpa token.
  fmt.Println("tenant berhasil di-provisioning:")
  ```

---

## Struktur Berkas

```
go-api-starter/
  go.mod
  .gitattributes
  .env.example
  .env.test.example
  Makefile
  CLAUDE.md
  cmd/
    api/main.go
    cli/
      main.go
      commands/
        registry.go
        migrate.go
        admin.go
        tenant.go
        permission.go
        cleanup.go
  internal/
    config/
      config.go
    apperror/
      error.go
    response/
      response.go
    validator/
      validator.go
    pagination/
      pagination.go
    database/
      postgres.go
      tenant.go
    migration/
      runner.go
      platform_fs.go
      tenant_fs.go
      platform/*.sql
      tenant/*.sql
    auth/
      password.go
      jwt.go
      refreshtoken.go
    middleware/
      requestid.go
      logger.go
      recover.go
      cors.go
      auth.go
      tenantresolver.go
      rbac.go
    ratelimit/
      loginattempt.go
    permission/
      constants.go
    modules/
      platform/
        auth/{dto.go,service.go,handler.go}
        tenant/{model.go,dto.go,repository.go,service.go}   # tanpa handler.go — lihat catatan Tugas 13
      tenant/
        auth/{dto.go,service.go,handler.go}
        user/{model.go,dto.go,repository.go,service.go,handler.go}
        role/{model.go,repository.go,service.go}
    server/
      router.go
      health.go
  deploy/
    app-api.service
    Caddyfile
    deploy.sh
  docs/
    (ditulis per-tugas, bersamaan dengan kode yang didokumentasikannya)
```

Setiap paket `modules/**` berbentuk `handler.go` (Fiber, hanya `*fiber.Ctx`) → `service.go` (business logic, hanya `context.Context`) → `repository.go` (sqlx, `*sqlx.Tx`) → `dto.go` (struct request/response) → `model.go` (struct baris DB). Bentuk inilah yang didemonstrasikan modul `user` di Tugas 17 sebagai contoh untuk modul-modul berikutnya.

---

### Tugas 1: Scaffolding project & modul Go

**Berkas:**
- Buat: `go.mod`
- Buat: `.gitattributes`
- Buat: `.env.example`
- Buat: `.env.test.example`
- Buat: `Makefile`

**Antarmuka:**
- Menghasilkan: modul kosong yang bisa di-build, tempat tugas-tugas lain menambahkan paket.

- [ ] **Langkah 1: Inisialisasi modul**

Jalankan:
```bash
cd D:/app/go-api-starter
go mod init go-api-starter
```
Hasil: membuat `go.mod` dengan `module go-api-starter` dan direktif `go`.

- [ ] **Langkah 2: Tetapkan versi minimal Go**

Edit `go.mod`, pastikan direktif `go` berbunyi `go 1.25`.

- [ ] **Langkah 3: Tambah `.gitattributes` supaya script dan SQL selalu LF**

Buat `.gitattributes`:
```
* text=auto eol=lf
*.sh text eol=lf
*.sql text eol=lf
Makefile text eol=lf
```

- [ ] **Langkah 4: Tulis `.env.example`**

Buat `.env.example`:
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=app
DB_PASSWORD=changeme
DB_NAME=appdb
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m
APP_PORT=8080
APP_ENV=development
JWT_SECRET=change-this-to-a-random-64-char-string
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
LOGIN_MAX_ATTEMPTS=5
LOGIN_ATTEMPT_WINDOW=15m
SHUTDOWN_TIMEOUT=15s
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

- [ ] **Langkah 5: Tulis `.env.test.example`**

Buat `.env.test.example`:
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=app
DB_PASSWORD=changeme
DB_NAME=app_test
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m
APP_PORT=8080
APP_ENV=test
JWT_SECRET=test-secret-not-for-production
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
LOGIN_MAX_ATTEMPTS=5
LOGIN_ATTEMPT_WINDOW=15m
SHUTDOWN_TIMEOUT=15s
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

- [ ] **Langkah 6: Tulis Makefile**

Buat `Makefile`:
```makefile
.PHONY: run build build-cli test test-integration migrate-platform migrate-tenant

run:
	air

build:
	go build -o bin/api ./cmd/api

build-cli:
	go build -o bin/cli ./cmd/cli

test:
	go test ./... -short

test-integration:
	go test ./... -run Integration -v

migrate-platform:
	go run ./cmd/cli migrate platform up

migrate-tenant:
	go run ./cmd/cli migrate tenant up --all
```

- [ ] **Langkah 7: Pastikan modul bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil tanpa output (belum ada paket, exit code 0).

- [ ] **Langkah 8: Commit**

```bash
git add go.mod .gitattributes .env.example .env.test.example Makefile
git commit -m "chore: scaffold Go module, env templates, Makefile"
```

---

### Tugas 2: Config loading + validasi

**Berkas:**
- Buat: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Antarmuka:**
- Menghasilkan:
  - Struct `config.Config`: `DB DBConfig`, `App AppConfig`, `JWT JWTConfig`, `Login LoginConfig`, `CORS CORSConfig`
  - `config.Load() (*Config, error)` — membaca env var (memuat `.env` dulu kalau ada), validasi, mengembalikan satu error berisi daftar semua field yang kurang/tidak valid.
  - `(c DBConfig) DSN() string` — connection string pgx.
  - `(c DBConfig) PlatformDSN() string` — sama, dengan `search_path=platform` di-pin.

- [ ] **Langkah 1: Tambah dependency**

Jalankan: `go get github.com/joho/godotenv@latest`

- [ ] **Langkah 2: Tulis test yang gagal**

Buat `internal/config/config_test.go`:
```go
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
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/config/... -v`
Hasil: FAIL — paket tidak compile (`Load` belum ada).

- [ ] **Langkah 4: Implementasikan config.go**

Buat `internal/config/config.go`:
```go
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

// Load membaca konfigurasi dari environment proses (memuat file .env lokal
// dulu kalau ada) lalu memvalidasinya. Semua masalah dikumpulkan dan
// dikembalikan sekaligus, jadi deploy yang salah konfigurasi gagal satu
// kali dengan daftar lengkap, bukan satu env var per satu kegagalan.
func Load() (*Config, error) {
	_ = godotenv.Load() // opsional; deploy sungguhan set env var langsung

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
			errs = append(errs, key+" harus berupa angka: "+err.Error())
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
			errs = append(errs, key+" harus berupa durasi (mis. 15m): "+err.Error())
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

	if len(errs) > 0 {
		return nil, fmt.Errorf("konfigurasi tidak valid:\n  - %s", strings.Join(errs, "\n  - "))
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
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/config/... -v`
Hasil: PASS — ketiga test lolos.

- [ ] **Langkah 6: Commit**

```bash
git add go.mod go.sum internal/config
git commit -m "feat: env-based config loading with fail-fast validation"
```

---

### Tugas 3: Tipe error bersama & amplop response

**Berkas:**
- Buat: `internal/apperror/error.go`
- Buat: `internal/response/response.go`
- Test: `internal/apperror/error_test.go`
- Test: `internal/response/response_test.go`

**Antarmuka:**
- Menghasilkan:
  - Tipe `apperror.Code` (string) dan konstanta: `CodeValidation`, `CodeNotFound`, `CodeConflict`, `CodeForbidden`, `CodeUnauthorized`, `CodeInternal`, `CodeTenantMigrationPending`, `CodeRateLimited`.
  - `apperror.Error struct { Code Code; Message string; Details map[string][]string; Status int }` yang mengimplementasikan `error`, plus `Unwrap() error`.
  - Konstruktor: `apperror.NotFound(msg string) *Error`, `apperror.Conflict(msg string) *Error`, `apperror.Validation(details map[string][]string) *Error`, `apperror.Forbidden(msg string) *Error`, `apperror.Unauthorized(msg string) *Error`, `apperror.RateLimited(msg string) *Error`, `apperror.TenantMigrationPending() *Error`, `apperror.Internal(cause error) *Error`.
  - `response.Success(c *fiber.Ctx, status int, data any) error`
  - `response.SuccessList(c *fiber.Ctx, status int, data any, meta response.Meta) error`
  - Struct `response.Meta { Page, Limit, Total, TotalPages int }`
  - `response.Error(c *fiber.Ctx, requestID string, err error) error` — memetakan `*apperror.Error` ke status/body yang sudah ditetapkan; error lain jadi 500 yang dicatat log, hanya `request_id` yang ditampilkan ke client.

- [ ] **Langkah 1: Tambah dependency Fiber**

Jalankan: `go get github.com/gofiber/fiber/v2@latest`

- [ ] **Langkah 2: Tulis test apperror yang gagal**

Buat `internal/apperror/error_test.go`:
```go
package apperror

import (
	"errors"
	"testing"
)

func TestConstructors_StatusAndCode(t *testing.T) {
	cases := []struct {
		name       string
		err        *Error
		wantCode   Code
		wantStatus int
	}{
		{"NotFound", NotFound("missing"), CodeNotFound, 404},
		{"Conflict", Conflict("dup"), CodeConflict, 409},
		{"Forbidden", Forbidden("no"), CodeForbidden, 403},
		{"Unauthorized", Unauthorized("no token"), CodeUnauthorized, 401},
		{"RateLimited", RateLimited("slow down"), CodeRateLimited, 429},
		{"TenantMigrationPending", TenantMigrationPending(), CodeTenantMigrationPending, 409},
		{"Validation", Validation(map[string][]string{"email": {"required"}}), CodeValidation, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.wantCode {
				t.Errorf("Code = %v, want %v", tc.err.Code, tc.wantCode)
			}
			if tc.err.Status != tc.wantStatus {
				t.Errorf("Status = %v, want %v", tc.err.Status, tc.wantStatus)
			}
		})
	}
}

func TestInternal_WrapsCause(t *testing.T) {
	cause := errors.New("db exploded")
	err := Internal(cause)

	if err.Status != 500 {
		t.Errorf("Status = %v, want 500", err.Status)
	}
	if !errors.Is(err, cause) {
		t.Error("Internal(cause) does not unwrap to cause")
	}
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	var _ error = NotFound("x")
}
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/apperror/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 4: Implementasikan error.go**

Buat `internal/apperror/error.go`:
```go
package apperror

type Code string

const (
	CodeValidation             Code = "VALIDATION_ERROR"
	CodeNotFound               Code = "NOT_FOUND"
	CodeConflict               Code = "CONFLICT"
	CodeForbidden              Code = "FORBIDDEN"
	CodeUnauthorized           Code = "UNAUTHORIZED"
	CodeInternal               Code = "INTERNAL_ERROR"
	CodeTenantMigrationPending Code = "TENANT_MIGRATION_PENDING"
	CodeRateLimited            Code = "RATE_LIMITED"
)

// Error adalah satu-satunya tipe error yang dikembalikan setiap fungsi
// service. Layer HTTP (response.Error) tahu cara mengubahnya jadi status
// code dan body JSON tanpa handler pernah menulis c.Status(...) sendiri.
type Error struct {
	Code    Code
	Message string
	Details map[string][]string
	Status  int
	cause   error
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func NotFound(msg string) *Error {
	return &Error{Code: CodeNotFound, Message: msg, Status: 404}
}

func Conflict(msg string) *Error {
	return &Error{Code: CodeConflict, Message: msg, Status: 409}
}

func Forbidden(msg string) *Error {
	return &Error{Code: CodeForbidden, Message: msg, Status: 403}
}

func Unauthorized(msg string) *Error {
	return &Error{Code: CodeUnauthorized, Message: msg, Status: 401}
}

func RateLimited(msg string) *Error {
	return &Error{Code: CodeRateLimited, Message: msg, Status: 429}
}

func TenantMigrationPending() *Error {
	return &Error{
		Code:    CodeTenantMigrationPending,
		Message: "tenant ini punya migrasi yang belum diterapkan dan harus diselesaikan dulu",
		Status:  409,
	}
}

func Validation(details map[string][]string) *Error {
	return &Error{
		Code:    CodeValidation,
		Message: "data yang dikirim tidak valid",
		Details: details,
		Status:  422,
	}
}

func Internal(cause error) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: "terjadi kesalahan pada server",
		Status:  500,
		cause:   cause,
	}
}
```

- [ ] **Langkah 5: Jalankan test apperror, pastikan lolos**

Jalankan: `go test ./internal/apperror/... -v`
Hasil: PASS.

- [ ] **Langkah 6: Tulis test response yang gagal**

Buat `internal/response/response_test.go`:
```go
package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

func TestSuccess_Envelope(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Success(c, 200, fiber.Map{"id": "abc"})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	var body struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Success {
		t.Error("success = false, want true")
	}
	if body.Data["id"] != "abc" {
		t.Errorf("data.id = %v, want abc", body.Data["id"])
	}
}

func TestError_AppError_UsesDeclaredStatus(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Error(c, "req-123", apperror.NotFound("tidak ditemukan"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Success {
		t.Error("success = true, want false")
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code = %q, want NOT_FOUND", body.Error.Code)
	}
	if body.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", body.RequestID)
	}
}

func TestError_UnknownError_Becomes500WithoutLeakingDetail(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Error(c, "req-456", errors.New("leaked secret detail"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Message == "leaked secret detail" {
		t.Error("internal error message leaked to client response")
	}
}
```

- [ ] **Langkah 7: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/response/... -v`
Hasil: FAIL — paket tidak compile (`Success`, `Error` belum ada).

- [ ] **Langkah 8: Implementasikan response.go**

Buat `internal/response/response.go`:
```go
package response

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func Success(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

func SuccessList(c *fiber.Ctx, status int, data any, meta Meta) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"meta":    meta,
	})
}

// Error merender err menjadi amplop error standar. Nilai *apperror.Error
// memakai status/code/details yang sudah ditetapkan apa adanya. Error lain
// (bug, error driver yang lolos dari service) jadi 500 yang body-nya hanya
// membawa request_id — error aslinya dicatat log, tidak pernah dikirim ke
// client.
func Error(c *fiber.Ctx, requestID string, err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		body := fiber.Map{
			"code":    appErr.Code,
			"message": appErr.Message,
		}
		if appErr.Details != nil {
			body["details"] = appErr.Details
		} else {
			body["details"] = fiber.Map{}
		}
		return c.Status(appErr.Status).JSON(fiber.Map{
			"success":    false,
			"error":      body,
			"request_id": requestID,
		})
	}

	slog.Error("unhandled error", "request_id", requestID, "error", err)
	return c.Status(500).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    apperror.CodeInternal,
			"message": "terjadi kesalahan pada server",
			"details": fiber.Map{},
		},
		"request_id": requestID,
	})
}
```

- [ ] **Langkah 9: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/response/... -v`
Hasil: PASS — ketiga test lolos.

- [ ] **Langkah 10: Commit**

```bash
git add go.mod go.sum internal/apperror internal/response
git commit -m "feat: typed app errors and uniform JSON response envelope"
```

---

### Tugas 4: Helper Validator + Pagination

**Berkas:**
- Buat: `internal/validator/validator.go`
- Buat: `internal/pagination/pagination.go`
- Test: `internal/validator/validator_test.go`
- Test: `internal/pagination/pagination_test.go`

**Antarmuka:**
- Menghasilkan:
  - `validator.Validate(s any) *apperror.Error` — menjalankan go-playground/validator, mengembalikan `nil` kalau valid, kalau tidak `apperror.Validation(details)` dengan key nama tag JSON tiap field.
  - Struct `pagination.Params { Page, Limit int; Sort, Order string }`
  - `pagination.Parse(c *fiber.Ctx, sortable map[string]string, defaultSort string) (Params, *apperror.Error)` — membaca query param `page`, `limit`, `sort`, `order`; `limit` di-clamp ke `[1,100]`; menolak `sort` di luar `sortable` dan `order` selain `asc`/`desc` dengan `apperror.Validation`.
  - `(p Params) Offset() int`
  - `(p Params) OrderByClause(sortable map[string]string) string` — misalnya `"created_at DESC"`, dibangun hanya dari nama kolom yang di-whitelist, tidak pernah dari nilai query mentah.
  - `pagination.BuildMeta(page, limit, total int) response.Meta`

- [ ] **Langkah 1: Tambah dependency validator**

Jalankan: `go get github.com/go-playground/validator/v10@latest`

- [ ] **Langkah 2: Tulis test validator yang gagal**

Buat `internal/validator/validator_test.go`:
```go
package validator

import "testing"

type signupInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

func TestValidate_AllValid_ReturnsNil(t *testing.T) {
	in := signupInput{Email: "a@b.com", Password: "longenough"}
	if err := Validate(in); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidate_Invalid_ReturnsDetailsByJSONName(t *testing.T) {
	in := signupInput{Email: "not-an-email", Password: "short"}
	err := Validate(in)
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if _, ok := err.Details["email"]; !ok {
		t.Errorf("Details missing key 'email', got %v", err.Details)
	}
	if _, ok := err.Details["password"]; !ok {
		t.Errorf("Details missing key 'password', got %v", err.Details)
	}
}
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/validator/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 4: Implementasikan validator.go**

Buat `internal/validator/validator.go`:
```go
package validator

import (
	"fmt"
	"reflect"
	"strings"

	pv "github.com/go-playground/validator/v10"
	"go-api-starter/internal/apperror"
)

var instance = newInstance()

func newInstance() *pv.Validate {
	v := pv.New()
	// Laporkan nama field versi JSON (mis. "email"), bukan nama field
	// struct Go (mis. "Email"), supaya pemetaan error di sisi client tepat.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// Validate menjalankan validasi struct tag pada s. Mengembalikan nil kalau
// s valid, kalau tidak *apperror.Error dengan Details ber-key nama field
// versi JSON, supaya frontend bisa menaruh tiap pesan di bawah input yang
// tepat.
func Validate(s any) *apperror.Error {
	err := instance.Struct(s)
	if err == nil {
		return nil
	}

	fieldErrs, ok := err.(pv.ValidationErrors)
	if !ok {
		return apperror.Validation(map[string][]string{"_": {err.Error()}})
	}

	details := make(map[string][]string)
	for _, fe := range fieldErrs {
		details[fe.Field()] = append(details[fe.Field()], message(fe))
	}
	return apperror.Validation(details)
}

func message(fe pv.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		return fmt.Sprintf("minimal %s karakter", fe.Param())
	case "max":
		return fmt.Sprintf("maksimal %s karakter", fe.Param())
	default:
		return fmt.Sprintf("tidak valid (%s)", fe.Tag())
	}
}
```

- [ ] **Langkah 5: Jalankan test validator, pastikan lolos**

Jalankan: `go test ./internal/validator/... -v`
Hasil: PASS.

- [ ] **Langkah 6: Tulis test pagination yang gagal**

Buat `internal/pagination/pagination_test.go`:
```go
package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

var userSortable = map[string]string{
	"created_at": "created_at",
	"name":       "name",
}

func parseWith(t *testing.T, target string) (Params, *fiber.Ctx) {
	t.Helper()
	app := fiber.New()
	var got Params
	var gotErr error
	app.Get("/x", func(c *fiber.Ctx) error {
		p, appErr := Parse(c, userSortable, "created_at")
		got = p
		if appErr != nil {
			gotErr = appErr
		}
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, target, nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_ = gotErr
	return got, nil
}

func TestParse_Defaults(t *testing.T) {
	p, _ := parseWith(t, "/x")
	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}
	if p.Limit != 20 {
		t.Errorf("Limit = %d, want 20", p.Limit)
	}
	if p.Sort != "created_at" {
		t.Errorf("Sort = %q, want created_at", p.Sort)
	}
	if p.Order != "desc" {
		t.Errorf("Order = %q, want desc", p.Order)
	}
}

func TestParse_LimitClampedAt100(t *testing.T) {
	p, _ := parseWith(t, "/x?limit=500")
	if p.Limit != 100 {
		t.Errorf("Limit = %d, want clamped to 100", p.Limit)
	}
}

func TestParse_RejectsSortNotInWhitelist(t *testing.T) {
	app := fiber.New()
	var appErr *fiber.Error
	_ = appErr
	var gotIsNil bool
	app.Get("/x", func(c *fiber.Ctx) error {
		_, err := Parse(c, userSortable, "created_at")
		gotIsNil = err == nil
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, "/x?sort=password_hash", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if gotIsNil {
		t.Error("Parse() accepted a sort column outside the whitelist")
	}
}

func TestOffset(t *testing.T) {
	p := Params{Page: 3, Limit: 20}
	if got := p.Offset(); got != 40 {
		t.Errorf("Offset() = %d, want 40", got)
	}
}

func TestOrderByClause_UsesWhitelistedColumnOnly(t *testing.T) {
	p := Params{Sort: "name", Order: "asc"}
	if got := p.OrderByClause(userSortable); got != "name ASC" {
		t.Errorf("OrderByClause() = %q, want %q", got, "name ASC")
	}
}
```

- [ ] **Langkah 7: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/pagination/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 8: Implementasikan pagination.go**

Buat `internal/pagination/pagination.go`:
```go
package pagination

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
	"go-api-starter/internal/response"
)

type Params struct {
	Page  int
	Limit int
	Sort  string
	Order string
}

// Parse membaca query param page/limit/sort/order. sort harus berupa key
// di sortable (whitelist kolom milik pemanggil yang aman diinterpolasi ke
// ORDER BY) — selain itu ditolak. Ini satu-satunya tempat nilai query
// mentah diizinkan dekat nama kolom sortir; setiap repository wajib lewat
// sini, bukan membaca c.Query("sort") sendiri.
func Parse(c *fiber.Ctx, sortable map[string]string, defaultSort string) (Params, *apperror.Error) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	sort := c.Query("sort", defaultSort)
	if _, ok := sortable[sort]; !ok {
		return Params{}, apperror.Validation(map[string][]string{
			"sort": {"kolom sortir tidak dikenal"},
		})
	}

	order := strings.ToLower(c.Query("order", "desc"))
	if order != "asc" && order != "desc" {
		return Params{}, apperror.Validation(map[string][]string{
			"order": {"harus 'asc' atau 'desc'"},
		})
	}

	return Params{Page: page, Limit: limit, Sort: sort, Order: order}, nil
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// OrderByClause mengembalikan "<kolom> ASC|DESC" HANYA memakai nama kolom
// SQL yang di-whitelist dari sortable[p.Sort] — tidak pernah nilai mentah
// p.Sort itu sendiri — jadi selalu aman ditempel langsung ke query string.
func (p Params) OrderByClause(sortable map[string]string) string {
	col := sortable[p.Sort]
	return col + " " + strings.ToUpper(p.Order)
}

func BuildMeta(page, limit, total int) response.Meta {
	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}
	return response.Meta{Page: page, Limit: limit, Total: total, TotalPages: totalPages}
}
```

- [ ] **Langkah 9: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/pagination/... -v`
Hasil: PASS — kelima test lolos.

- [ ] **Langkah 10: Commit**

```bash
git add go.mod go.sum internal/validator internal/pagination
git commit -m "feat: request validation and whitelist-safe pagination"
```

---

### Tugas 5: Database — connection pool, `WithTenant`, context helper

Ini bagian paling krusial dari seluruh starter dari sisi keamanan: inilah yang mengubah "satu database Postgres" menjadi "schema per-tenant yang terisolasi ketat." Pastikan test tugas ini hijau dulu sebelum membangun apa pun di atasnya.

**Berkas:**
- Buat: `internal/database/postgres.go`
- Buat: `internal/database/tenant.go`
- Buat: `internal/testsupport/testsupport.go` (paket helper khusus test — **tidak pernah** di-import dari kode non-test; dia menarik `testing`)
- Test: `internal/database/tenant_test.go`

**Antarmuka:**
- Menghasilkan:
  - `database.NewPool(cfg config.DBConfig) (*sqlx.DB, error)` — pool umum, search_path dibiarkan default koneksi.
  - `database.NewPlatformPool(cfg config.DBConfig) (*sqlx.DB, error)` — pool yang di-pin ke `search_path=platform` di setiap koneksi.
  - Struct `database.TenantInfo { TenantID uuid.UUID; SchemaName string }`
  - Struct `database.Actor { UserID uuid.UUID; Scope string }`
  - `database.WithTenantInfo(ctx, TenantInfo) context.Context`, `database.TenantFromContext(ctx) (TenantInfo, bool)`
  - `database.WithActor(ctx, Actor) context.Context`, `database.ActorFromContext(ctx) (Actor, bool)`
  - `database.ValidSchemaName(name string) bool`
  - Struct `database.DB { Pool *sqlx.DB }`, `database.NewDB(pool *sqlx.DB) *DB`
  - `(db *DB) WithTenant(ctx context.Context, fn func(tx *sqlx.Tx) error) error`
  - `testsupport.OpenTestDB(t *testing.T) *sqlx.DB` — skip test kalau PostgreSQL lokal tidak bisa diakses.
  - `testsupport.RandomSchemaName() string`

- [ ] **Langkah 1: Tambah dependency**

Jalankan:
```bash
go get github.com/jmoiron/sqlx@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/google/uuid@latest
```

- [ ] **Langkah 2: Buat database khusus test `app_test`**

Jalankan (sekali, manual, di PostgreSQL lokal kamu):
```bash
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```
(Sesuaikan nama role dengan `DB_USER` di `.env.test`. Kalau role `app` belum ada: `psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"` dulu, samakan dengan `.env.test`.)

Salin `.env.test.example` jadi `.env.test` di root repo dan sesuaikan kredensialnya kalau perlu.

- [ ] **Langkah 3: Tulis helper test support**

Buat `internal/testsupport/testsupport.go`:
```go
// Package testsupport provides helpers for tests that need a real
// PostgreSQL connection. It must only ever be imported from _test.go
// files — it depends on "testing", which has no place in a shipped binary.
package testsupport

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// OpenTestDB connects to the app_test database described by .env.test at
// the repo root. It skips (not fails) the calling test when no local
// PostgreSQL is reachable, so `go test ./...` stays usable on a machine
// that hasn't set one up — the machine that has gets full coverage.
func OpenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	_ = godotenv.Load(findEnvTestFile())

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("skipping: no local PostgreSQL test database reachable (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// RandomSchemaName returns a name like "test_a1b2c3d4" that satisfies the
// tenant schema-name regex, for tests that need their own throwaway schema.
func RandomSchemaName() string {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[src.Intn(len(letters))]
	}
	return "test_" + string(b)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findEnvTestFile() string {
	candidates := []string{".env.test", "../../.env.test", "../../../.env.test"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ".env.test"
}
```

- [ ] **Langkah 4: Tulis test yang gagal**

Buat `internal/database/tenant_test.go`:
```go
package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/database"
	"go-api-starter/internal/testsupport"
)

func createTestSchema(t *testing.T, pool *sqlx.DB, name string) {
	t.Helper()
	if _, err := pool.Exec("CREATE SCHEMA " + name); err != nil {
		t.Fatalf("create schema %s: %v", name, err)
	}
	t.Cleanup(func() {
		pool.Exec("DROP SCHEMA " + name + " CASCADE")
	})
	if _, err := pool.Exec("CREATE TABLE " + name + ".widgets (id serial primary key, name text)"); err != nil {
		t.Fatalf("create table in %s: %v", name, err)
	}
}

func TestWithTenant_IsolatesSchemas(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := database.NewDB(pool)

	schemaA := testsupport.RandomSchemaName()
	schemaB := testsupport.RandomSchemaName()
	createTestSchema(t, pool, schemaA)
	createTestSchema(t, pool, schemaB)

	ctxA := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaA,
	})
	ctxB := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaB,
	})

	err := db.WithTenant(ctxA, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("INSERT INTO widgets (name) VALUES ($1)", "from-a")
		return err
	})
	if err != nil {
		t.Fatalf("insert into schema A: %v", err)
	}

	err = db.WithTenant(ctxB, func(tx *sqlx.Tx) error {
		var count int
		if err := tx.Get(&count, "SELECT count(*) FROM widgets"); err != nil {
			return err
		}
		if count != 0 {
			t.Errorf("schema B sees %d row(s), want 0 — schema A's insert leaked across search_path", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query schema B: %v", err)
	}
}

func TestWithTenant_RollbackClearsSearchPath(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	pool.SetMaxOpenConns(1) // force both calls below onto the same physical connection
	db := database.NewDB(pool)

	schemaA := testsupport.RandomSchemaName()
	schemaB := testsupport.RandomSchemaName()
	createTestSchema(t, pool, schemaA)
	createTestSchema(t, pool, schemaB)

	ctxA := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaA,
	})
	_ = db.WithTenant(ctxA, func(tx *sqlx.Tx) error {
		return errors.New("forced failure")
	})

	ctxB := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaB,
	})
	var current string
	err := db.WithTenant(ctxB, func(tx *sqlx.Tx) error {
		return tx.Get(&current, "SELECT current_schema()")
	})
	if err != nil {
		t.Fatalf("WithTenant(schemaB): %v", err)
	}
	if current != schemaB {
		t.Errorf("current_schema() = %q, want %q — search_path leaked across a rolled-back transaction on a reused connection", current, schemaB)
	}
}

func TestWithTenant_NoTenantInContext_ReturnsError(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := database.NewDB(pool)

	err := db.WithTenant(context.Background(), func(tx *sqlx.Tx) error {
		t.Fatal("fn should not run without tenant context")
		return nil
	})
	if err == nil {
		t.Fatal("WithTenant() = nil, want error when context has no TenantInfo")
	}
}
```

- [ ] **Langkah 5: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/database/... -v`
Hasil: FAIL — paket tidak compile (`database.NewDB` dkk. belum ada).

- [ ] **Langkah 6: Implementasikan postgres.go**

Buat `internal/database/postgres.go`:
```go
package database

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/config"
)

// NewPool opens the main application connection pool, used for tenant-
// scoped queries via WithTenant. search_path is left at the connection
// default; WithTenant pins it per-transaction.
func NewPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

// NewPlatformPool opens a pool pinned to search_path=platform on every
// connection, so platform repositories write unqualified table names
// ("FROM tenants") and never need WithTenant.
func NewPlatformPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.PlatformDSN())
	if err != nil {
		return nil, fmt.Errorf("connect to platform schema: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

func configurePool(db *sqlx.DB, cfg config.DBConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}
```

- [ ] **Langkah 7: Implementasikan tenant.go**

Buat `internal/database/tenant.go`:
```go
package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/apperror"
)

var schemaNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

// ValidSchemaName reports whether name is safe to interpolate into SQL as
// an identifier. Checked once at provisioning time, and again here,
// defensively, every time WithTenant is about to use it.
func ValidSchemaName(name string) bool {
	return schemaNameRe.MatchString(name)
}

// quoteIdentifier double-quotes a Postgres identifier, escaping embedded
// quotes. A small local helper instead of a lib/pq dependency pulled in
// only for QuoteIdentifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type TenantInfo struct {
	TenantID   uuid.UUID
	SchemaName string
}

type Actor struct {
	UserID uuid.UUID
	Scope  string // "platform" | "tenant"
}

type tenantCtxKey struct{}
type actorCtxKey struct{}

func WithTenantInfo(ctx context.Context, info TenantInfo) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, info)
}

func TenantFromContext(ctx context.Context) (TenantInfo, bool) {
	info, ok := ctx.Value(tenantCtxKey{}).(TenantInfo)
	return info, ok
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorCtxKey{}).(Actor)
	return actor, ok
}

type DB struct {
	Pool *sqlx.DB
}

func NewDB(pool *sqlx.DB) *DB {
	return &DB{Pool: pool}
}

// WithTenant runs fn inside a transaction whose search_path is pinned, via
// SET LOCAL, to the schema of the tenant found in ctx. SET LOCAL only lasts
// for the transaction — commit or rollback both clear it automatically —
// so the connection always returns to the pool clean, with no manual reset
// step that could be forgotten on an error path.
func (db *DB) WithTenant(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	info, ok := TenantFromContext(ctx)
	if !ok {
		return apperror.Internal(fmt.Errorf("WithTenant called without tenant context"))
	}
	if !ValidSchemaName(info.SchemaName) {
		return apperror.Internal(fmt.Errorf("invalid schema name %q", info.SchemaName))
	}

	tx, err := db.Pool.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Internal(fmt.Errorf("begin tx: %w", err))
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO "+quoteIdentifier(info.SchemaName)); err != nil {
		return apperror.Internal(fmt.Errorf("set search_path: %w", err))
	}

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperror.Internal(fmt.Errorf("commit tx: %w", err))
	}
	return nil
}
```

- [ ] **Langkah 8: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/database/... -v`
Hasil: PASS — `TestWithTenant_IsolatesSchemas`, `TestWithTenant_RollbackClearsSearchPath`, `TestWithTenant_NoTenantInContext_ReturnsError` semua lolos terhadap database `app_test` lokal kamu. (Kalau PostgreSQL tidak bisa diakses, hasilnya SKIP, bukan gagal — siapkan dulu sesuai Langkah 2 sebelum lanjut.)

- [ ] **Langkah 9: Commit**

```bash
git add go.mod go.sum internal/database internal/testsupport
git commit -m "feat: tenant-scoped DB pool with SET LOCAL search_path isolation"
```

---

### Tugas 6: Skeleton CLI + migration runner + migrasi platform

**Catatan desain soal pembuatan schema:** driver postgres milik golang-migrate mengharap schema tujuan sudah ada saat dia dibuka (dia langsung bikin tabel pelacak `schema_migrations` di situ, dan tidak akan membuat schema-nya sendiri). Jadi pembuatan schema **bukan** migration file tersendiri — `MigratePlatformUp` menjalankan `CREATE SCHEMA IF NOT EXISTS platform` langsung sebelum menyerahkan kendali ke golang-migrate, dan pembuatan schema tenant dilakukan eksplisit saat provisioning (Tugas 14), sebelum migrasi tenant mana pun dijalankan.

**Berkas:**
- Buat: `cmd/cli/main.go`
- Buat: `cmd/cli/commands/registry.go`
- Buat: `cmd/cli/commands/migrate.go`
- Buat: `internal/migration/runner.go`
- Buat: `internal/migration/platform_fs.go`
- Buat: `internal/migration/platform/000001_create_updated_at_trigger_fn.up.sql`
- Buat: `internal/migration/platform/000001_create_updated_at_trigger_fn.down.sql`
- Buat: `internal/migration/platform/000002_create_tenants_table.up.sql`
- Buat: `internal/migration/platform/000002_create_tenants_table.down.sql`
- Buat: `internal/migration/platform/000003_create_platform_users_table.up.sql`
- Buat: `internal/migration/platform/000003_create_platform_users_table.down.sql`
- Buat: `internal/migration/platform/000004_create_platform_refresh_tokens_table.up.sql`
- Buat: `internal/migration/platform/000004_create_platform_refresh_tokens_table.down.sql`
- Buat: `internal/migration/platform/000005_create_login_attempts_table.up.sql`
- Buat: `internal/migration/platform/000005_create_login_attempts_table.down.sql`
- Test: `internal/migration/runner_test.go`

**Antarmuka:**
- Menghasilkan:
  - `commands.Register(name string, h commands.Handler)`, `commands.Lookup(name string) (Handler, bool)`, `commands.Names() []string`, tipe `Handler func(args []string) error`
  - `migration.MigratePlatformUp(db *sql.DB) error`
  - `migration.LatestPlatformVersion() uint`
  - Perintah CLI: `cli migrate platform up`

- [ ] **Langkah 1: Tambah golang-migrate**

Jalankan: `go get github.com/golang-migrate/migrate/v4`

- [ ] **Langkah 2: Tulis registry perintah CLI**

Buat `cmd/cli/commands/registry.go`:
```go
package commands

import "sort"

type Handler func(args []string) error

var registry = map[string]Handler{}

func Register(name string, h Handler) {
	registry[name] = h
}

func Lookup(name string) (Handler, bool) {
	h, ok := registry[name]
	return h, ok
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Langkah 3: Tulis entrypoint CLI**

Buat `cmd/cli/main.go`:
```go
package main

import (
	"fmt"
	"os"
	"strings"

	"go-api-starter/cmd/cli/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmdKey, rest := splitCommand(os.Args[1:])
	handler, ok := commands.Lookup(cmdKey)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmdKey)
		printUsage()
		os.Exit(1)
	}

	if err := handler(rest); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// splitCommand takes the leading non-flag arguments as the command name
// (e.g. "migrate tenant up") and returns the rest as flags, so callers can
// run `cli migrate tenant up --tenant=acme_corp`.
func splitCommand(args []string) (cmdKey string, rest []string) {
	var parts []string
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "--") {
		parts = append(parts, args[i])
		i++
	}
	return strings.Join(parts, " "), args[i:]
}

func printUsage() {
	fmt.Println("usage: cli <command> [flags]")
	fmt.Println("\navailable commands:")
	for _, name := range commands.Names() {
		fmt.Println("  " + name)
	}
}
```

- [ ] **Langkah 4: Tulis test migration runner yang gagal**

Buat `internal/migration/runner_test.go`:
```go
package migration_test

import (
	"testing"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/testsupport"
)

func TestMigratePlatformUp_CreatesSchemaAndIsIdempotent(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	pool.Exec("DROP SCHEMA IF EXISTS platform CASCADE")
	t.Cleanup(func() { pool.Exec("DROP SCHEMA IF EXISTS platform CASCADE") })

	db := pool.DB

	if err := migration.MigratePlatformUp(db); err != nil {
		t.Fatalf("first MigratePlatformUp: %v", err)
	}
	if err := migration.MigratePlatformUp(db); err != nil {
		t.Fatalf("second MigratePlatformUp (should be a no-op): %v", err)
	}

	var exists bool
	err := pool.Get(&exists, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'platform' AND table_name = 'tenants'
	)`)
	if err != nil {
		t.Fatalf("check tenants table: %v", err)
	}
	if !exists {
		t.Error("platform.tenants table was not created")
	}
}

func TestLatestPlatformVersion_MatchesFileCount(t *testing.T) {
	if got := migration.LatestPlatformVersion(); got != 5 {
		t.Errorf("LatestPlatformVersion() = %d, want 5 (five platform migration files)", got)
	}
}
```

- [ ] **Langkah 5: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/migration/... -v`
Hasil: FAIL — paket tidak compile (`migration.MigratePlatformUp` belum ada; juga belum ada berkas `platform/*.sql` untuk ditemukan `//go:embed`).

- [ ] **Langkah 6: Tulis berkas SQL migrasi platform**

Buat `internal/migration/platform/000001_create_updated_at_trigger_fn.up.sql`:
```sql
CREATE OR REPLACE FUNCTION platform.trigger_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Buat `internal/migration/platform/000001_create_updated_at_trigger_fn.down.sql`:
```sql
DROP FUNCTION IF EXISTS platform.trigger_set_updated_at();
```

Buat `internal/migration/platform/000002_create_tenants_table.up.sql`:
```sql
CREATE TABLE platform.tenants (
    id            UUID PRIMARY KEY,
    code          TEXT NOT NULL,
    name          TEXT NOT NULL,
    schema_name   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'provisioning'
                  CHECK (status IN ('provisioning', 'active', 'suspended')),
    suspended_at  TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ NULL,
    created_by    UUID NULL,
    updated_by    UUID NULL,
    deleted_by    UUID NULL
);

CREATE UNIQUE INDEX tenants_code_key ON platform.tenants (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_schema_name_key ON platform.tenants (schema_name) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON platform.tenants
    FOR EACH ROW EXECUTE FUNCTION platform.trigger_set_updated_at();
```

Buat `internal/migration/platform/000002_create_tenants_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.tenants;
```

Buat `internal/migration/platform/000003_create_platform_users_table.up.sql`:
```sql
CREATE TABLE platform.users (
    id             UUID PRIMARY KEY,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    name           TEXT NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ NULL,
    created_by     UUID NULL,
    updated_by     UUID NULL,
    deleted_by     UUID NULL
);

CREATE UNIQUE INDEX platform_users_email_key ON platform.users (email) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON platform.users
    FOR EACH ROW EXECUTE FUNCTION platform.trigger_set_updated_at();
```

Buat `internal/migration/platform/000003_create_platform_users_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.users;
```

Buat `internal/migration/platform/000004_create_platform_refresh_tokens_table.up.sql`:
```sql
CREATE TABLE platform.refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES platform.users (id),
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NULL,
    user_agent  TEXT NULL,
    ip          TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX platform_refresh_tokens_hash_key ON platform.refresh_tokens (token_hash);
CREATE INDEX platform_refresh_tokens_user_id_idx ON platform.refresh_tokens (user_id);
```

Buat `internal/migration/platform/000004_create_platform_refresh_tokens_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.refresh_tokens;
```

Buat `internal/migration/platform/000005_create_login_attempts_table.up.sql`:
```sql
CREATE TABLE platform.login_attempts (
    id           UUID PRIMARY KEY,
    scope        TEXT NOT NULL CHECK (scope IN ('platform', 'tenant')),
    tenant_id    UUID NULL,
    email        TEXT NOT NULL,
    success      BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX login_attempts_lookup_idx
    ON platform.login_attempts (scope, tenant_id, email, attempted_at DESC);
```

Buat `internal/migration/platform/000005_create_login_attempts_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.login_attempts;
```

- [ ] **Langkah 7: Implementasikan embed + runner (bagian platform)**

Buat `internal/migration/platform_fs.go`:
```go
package migration

import "embed"

//go:embed platform/*.sql
var PlatformFS embed.FS
```

Buat `internal/migration/runner.go`:
```go
package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func newMigrate(db *sql.DB, fsys embed.FS, dir, schemaName string) (*migrate.Migrate, error) {
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migration source %s: %w", dir, err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{
		SchemaName:      schemaName,
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return nil, fmt.Errorf("migration driver for schema %s: %w", schemaName, err)
	}
	return migrate.NewWithInstance("iofs", src, schemaName, driver)
}

// MigratePlatformUp applies every unapplied migration under platform/ to
// the platform schema, creating that schema first if it doesn't exist yet
// (golang-migrate's postgres driver requires the schema to already exist
// before it opens, so schema creation can't be a migration file itself).
func MigratePlatformUp(db *sql.DB) error {
	if _, err := db.Exec("CREATE SCHEMA IF NOT EXISTS platform"); err != nil {
		return fmt.Errorf("create platform schema: %w", err)
	}
	m, err := newMigrate(db, PlatformFS, "platform", "platform")
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// LatestPlatformVersion returns the highest migration version compiled
// into this binary under platform/.
func LatestPlatformVersion() uint {
	return latestVersion(PlatformFS, "platform")
}

func latestVersion(fsys embed.FS, dir string) uint {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return 0
	}
	var max uint
	for _, e := range entries {
		underscore := strings.IndexByte(e.Name(), '_')
		if underscore <= 0 {
			continue
		}
		n, err := strconv.ParseUint(e.Name()[:underscore], 10, 64)
		if err != nil {
			continue
		}
		if uint(n) > max {
			max = uint(n)
		}
	}
	return max
}

// MigrationFile is one applied-in-order .up.sql file, exposed so callers
// (tenant provisioning, Task 13) that need to apply migrations by hand
// inside their own transaction — something golang-migrate itself can't
// join, since it manages its own connection — can read the exact same
// files the CLI's `migrate` commands use.
type MigrationFile struct {
	Version uint
	SQL     string
}

func readUpFiles(fsys embed.FS, dir string) ([]MigrationFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var files []MigrationFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		underscore := strings.IndexByte(name, '_')
		if underscore <= 0 {
			continue
		}
		version, err := strconv.ParseUint(name[:underscore], 10, 64)
		if err != nil {
			continue
		}
		content, err := fsys.ReadFile(dir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files = append(files, MigrationFile{Version: uint(version), SQL: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}
```

- [ ] **Langkah 8: Tulis perintah CLI migrate-platform**

Buat `cmd/cli/commands/migrate.go`:
```go
package commands

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go-api-starter/internal/config"
	"go-api-starter/internal/migration"
)

func init() {
	Register("migrate platform up", cmdMigratePlatformUp)
}

func openRawDB(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

func cmdMigratePlatformUp(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	if err := migration.MigratePlatformUp(db); err != nil {
		return fmt.Errorf("migrate platform: %w", err)
	}
	fmt.Println("migrasi platform berhasil diterapkan")
	return nil
}
```

- [ ] **Langkah 9: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/migration/... -v`
Hasil: PASS — kedua test lolos terhadap database `app_test` lokal kamu.

- [ ] **Langkah 10: Verifikasi manual perintah CLI**

Jalankan: `go run ./cmd/cli migrate platform up`
Hasil: mencetak `migrasi platform berhasil diterapkan`. Jalankan sekali lagi — hasil sama, tanpa error (idempotent).

- [ ] **Langkah 11: Commit**

```bash
git add go.mod go.sum cmd/cli internal/migration
git commit -m "feat: CLI skeleton and platform schema migrations"
```

---

### Tugas 7: Migrasi tenant

Pembuatan schema tenant juga **bukan** migration file di sini — provisioning (Tugas 14) menjalankan `CREATE SCHEMA` langsung sebelum menerapkan migrasi-migrasi ini, dengan alasan golang-migrate yang sama seperti Tugas 6.

**Berkas:**
- Buat: `internal/migration/tenant_fs.go`
- Buat: `internal/migration/tenant/000001_create_updated_at_trigger_fn.up.sql` (+ `.down.sql`)
- Buat: `internal/migration/tenant/000002_create_users_table.up.sql` (+ `.down.sql`)
- Buat: `internal/migration/tenant/000003_create_roles_permissions_tables.up.sql` (+ `.down.sql`)
- Buat: `internal/migration/tenant/000004_create_refresh_tokens_table.up.sql` (+ `.down.sql`)
- Ubah: `internal/migration/runner.go` — tambah fungsi sisi tenant
- Ubah: `cmd/cli/commands/migrate.go` — tambah `migrate tenant up` dan `migrate status`
- Test: Ubah `internal/migration/runner_test.go` — tambah cakupan tenant

**Antarmuka:**
- Menghasilkan:
  - `migration.MigrateTenantUp(db *sql.DB, schemaName string) error`
  - `migration.TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error)`
  - `migration.LatestTenantVersion() uint`
  - `migration.TenantMigrationFiles() ([]MigrationFile, error)` — dipakai provisioning di Tugas 14.
  - Perintah CLI: `cli migrate tenant up --all`, `cli migrate tenant up --tenant=<code>`, `cli migrate status`

- [ ] **Langkah 1: Tulis berkas SQL migrasi tenant**

Buat `internal/migration/tenant/000001_create_updated_at_trigger_fn.up.sql`:
```sql
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Buat `internal/migration/tenant/000001_create_updated_at_trigger_fn.down.sql`:
```sql
DROP FUNCTION IF EXISTS trigger_set_updated_at();
```

Buat `internal/migration/tenant/000002_create_users_table.up.sql`:
```sql
CREATE TABLE users (
    id             UUID PRIMARY KEY,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    name           TEXT NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ NULL,
    created_by     UUID NULL,
    updated_by     UUID NULL,
    deleted_by     UUID NULL
);

CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
```

Buat `internal/migration/tenant/000002_create_users_table.down.sql`:
```sql
DROP TABLE IF EXISTS users;
```

Buat `internal/migration/tenant/000003_create_roles_permissions_tables.up.sql`:
```sql
CREATE TABLE roles (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    created_by  UUID NULL,
    updated_by  UUID NULL,
    deleted_by  UUID NULL
);

CREATE UNIQUE INDEX roles_name_key ON roles (name) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE permissions (
    id          UUID PRIMARY KEY,
    code        TEXT NOT NULL,
    "group"     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    created_by  UUID NULL,
    updated_by  UUID NULL,
    deleted_by  UUID NULL
);

CREATE UNIQUE INDEX permissions_code_key ON permissions (code) WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE role_permissions (
    role_id        UUID NOT NULL REFERENCES roles (id),
    permission_id  UUID NOT NULL REFERENCES permissions (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by     UUID NULL,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users (id),
    role_id     UUID NOT NULL REFERENCES roles (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  UUID NULL,
    PRIMARY KEY (user_id, role_id)
);
```

Buat `internal/migration/tenant/000003_create_roles_permissions_tables.down.sql`:
```sql
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
```

Buat `internal/migration/tenant/000004_create_refresh_tokens_table.up.sql`:
```sql
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users (id),
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ NULL,
    user_agent  TEXT NULL,
    ip          TEXT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX refresh_tokens_hash_key ON refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);
```

Buat `internal/migration/tenant/000004_create_refresh_tokens_table.down.sql`:
```sql
DROP TABLE IF EXISTS refresh_tokens;
```

- [ ] **Langkah 2: Tambah embed tenant**

Buat `internal/migration/tenant_fs.go`:
```go
package migration

import "embed"

//go:embed tenant/*.sql
var TenantFS embed.FS
```

- [ ] **Langkah 3: Perluas test yang gagal**

Ubah `internal/migration/runner_test.go` — tambahkan dua test ini ke berkas yang sudah ada (pertahankan dua test dari Tugas 6):
```go
func TestMigrateTenantUp_AndVersionReporting(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := pool.DB

	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })

	if err := migration.MigrateTenantUp(db, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	version, dirty, err := migration.TenantSchemaVersion(db, schema)
	if err != nil {
		t.Fatalf("TenantSchemaVersion: %v", err)
	}
	if dirty {
		t.Error("schema reported dirty after a clean migration")
	}
	if version != migration.LatestTenantVersion() {
		t.Errorf("version = %d, want latest %d", version, migration.LatestTenantVersion())
	}
}

func TestTenantMigrationFiles_ReturnsAllFilesInOrder(t *testing.T) {
	files, err := migration.TenantMigrationFiles()
	if err != nil {
		t.Fatalf("TenantMigrationFiles: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files, want 4", len(files))
	}
	for i, f := range files {
		if f.Version != uint(i+1) {
			t.Errorf("files[%d].Version = %d, want %d (files must be in ascending order)", i, f.Version, i+1)
		}
		if f.SQL == "" {
			t.Errorf("files[%d].SQL is empty", i)
		}
	}
}
```

- [ ] **Langkah 4: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/migration/... -v`
Hasil: FAIL — `migration.MigrateTenantUp` dkk. belum ada.

- [ ] **Langkah 5: Perluas runner**

Ubah `internal/migration/runner.go` — tambahkan fungsi-fungsi ini di akhir berkas:
```go
// MigrateTenantUp applies every unapplied migration under tenant/ to the
// given tenant schema, which must already exist (provisioning creates it
// before calling this).
func MigrateTenantUp(db *sql.DB, schemaName string) error {
	m, err := newMigrate(db, TenantFS, "tenant", schemaName)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// TenantSchemaVersion reports the migration version currently applied to
// schemaName, and whether it's dirty (failed mid-migration). version is 0
// with dirty=false if no migration has ever been applied.
func TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error) {
	m, err := newMigrate(db, TenantFS, "tenant", schemaName)
	if err != nil {
		return 0, false, err
	}
	v, d, err := m.Version()
	if err == migrate.ErrNilVersion {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, d, nil
}

// LatestTenantVersion returns the highest migration version compiled into
// this binary under tenant/ — the version every tenant schema is expected
// to be at. Used by cli migrate status and by the runtime
// TENANT_MIGRATION_PENDING guard (Task 11).
func LatestTenantVersion() uint {
	return latestVersion(TenantFS, "tenant")
}

// TenantMigrationFiles returns tenant/'s up-migration SQL, in version
// order, for callers that must apply migrations inside their own already-
// open transaction — golang-migrate manages its own connection and can't
// join a caller's transaction, so tenant provisioning (Task 13) reads and
// execs these directly instead of calling MigrateTenantUp.
func TenantMigrationFiles() ([]MigrationFile, error) {
	return readUpFiles(TenantFS, "tenant")
}
```

- [ ] **Langkah 6: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/migration/... -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 7: Tambah perintah CLI migrasi tenant**

Ubah `cmd/cli/commands/migrate.go` — tambahkan ke blok `init()` dan tambahkan fungsi-fungsi ini:
```go
func init() {
	Register("migrate platform up", cmdMigratePlatformUp)
	Register("migrate tenant up", cmdMigrateTenantUp)
	Register("migrate status", cmdMigrateStatus)
}
```
(Ganti `init()` satu baris dari Tugas 6 dengan versi tiga baris ini.)

```go
type tenantTarget struct {
	code       string
	schemaName string
}

func tenantSchemasToMigrate(db *sql.DB, all bool, code string) ([]tenantTarget, error) {
	query := "SELECT code, schema_name FROM platform.tenants WHERE deleted_at IS NULL"
	var args []any
	if !all {
		if code == "" {
			return nil, fmt.Errorf("pass --all or --tenant=<code>")
		}
		query += " AND code = $1"
		args = append(args, code)
	}
	query += " ORDER BY code"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var targets []tenantTarget
	for rows.Next() {
		var t tenantTarget
		if err := rows.Scan(&t.code, &t.schemaName); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func cmdMigrateTenantUp(args []string) error {
	fs := flag.NewFlagSet("migrate tenant up", flag.ExitOnError)
	all := fs.Bool("all", false, "apply to every tenant")
	tenantCode := fs.String("tenant", "", "apply to a single tenant by code")
	fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, *all, *tenantCode)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no matching tenants found")
	}

	done := 0
	for _, tgt := range targets {
		fmt.Printf("migrating %s ... ", tgt.code)
		if err := migration.MigrateTenantUp(db, tgt.schemaName); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("tenant %s: %w (stopped after %d of %d tenants)",
				tgt.code, err, done, len(targets))
		}
		fmt.Println("ok")
		done++
	}
	return nil
}

func cmdMigrateStatus(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, true, "")
	if err != nil {
		return err
	}

	latest := migration.LatestTenantVersion()
	for _, tgt := range targets {
		version, dirty, err := migration.TenantSchemaVersion(db, tgt.schemaName)
		if err != nil {
			fmt.Printf("%-20s ERROR: %v\n", tgt.code, err)
			continue
		}
		status := "up to date"
		switch {
		case dirty:
			status = "DIRTY — needs manual repair"
		case version < latest:
			status = fmt.Sprintf("BEHIND (at %d, latest %d)", version, latest)
		}
		fmt.Printf("%-20s %s\n", tgt.code, status)
	}
	return nil
}
```

Tambahkan `"flag"` ke blok import di atas `cmd/cli/commands/migrate.go`.

- [ ] **Langkah 8: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 9: Commit**

```bash
git add internal/migration cmd/cli/commands/migrate.go
git commit -m "feat: tenant schema migrations and cli migrate tenant/status commands"
```

---

### Tugas 8: Hashing password + JWT + helper refresh token

**Berkas:**
- Buat: `internal/auth/password.go`
- Buat: `internal/auth/jwt.go`
- Buat: `internal/auth/refreshtoken.go`
- Test: `internal/auth/password_test.go`
- Test: `internal/auth/jwt_test.go`
- Test: `internal/auth/refreshtoken_test.go`

**Antarmuka:**
- Menghasilkan:
  - `auth.BcryptCost = 12`, `auth.HashPassword(plain string) (string, error)`, `auth.VerifyPassword(hash, plain string) bool`
  - `auth.ValidatePasswordStrength(plain string) *apperror.Error`
  - `auth.ScopePlatform = "platform"`, `auth.ScopeTenant = "tenant"`
  - Struct `auth.Claims { jwt.RegisteredClaims; Scope string; TenantID string }`
  - `auth.NewTokenManager(secret string, accessTTL time.Duration) *TokenManager`
  - `(tm *TokenManager) IssueAccessToken(userID uuid.UUID, scope string, tenantID *uuid.UUID) (string, error)`
  - `(tm *TokenManager) Verify(tokenString string) (*Claims, error)`
  - `auth.GenerateRefreshToken() (plain string, hash string, err error)`
  - `auth.HashRefreshToken(plain string) string`

- [ ] **Langkah 1: Tambah dependency**

Jalankan:
```bash
go get golang.org/x/crypto/bcrypt
go get github.com/golang-jwt/jwt/v5
```

- [ ] **Langkah 2: Tulis test password yang gagal**

Buat `internal/auth/password_test.go`:
```go
package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correcthorsebattery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correcthorsebattery") {
		t.Error("VerifyPassword() = false for the correct password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("VerifyPassword() = true for a wrong password")
	}
}

func TestValidatePasswordStrength_TooShort(t *testing.T) {
	if err := ValidatePasswordStrength("short1"); err == nil {
		t.Error("expected error for a password under 8 characters")
	}
}

func TestValidatePasswordStrength_CommonPassword(t *testing.T) {
	if err := ValidatePasswordStrength("password"); err == nil {
		t.Error("expected error for a common password")
	}
	if err := ValidatePasswordStrength("123456789"); err == nil {
		t.Error("expected error for a common password")
	}
}

func TestValidatePasswordStrength_Valid(t *testing.T) {
	if err := ValidatePasswordStrength("a-reasonably-unique-passphrase"); err != nil {
		t.Errorf("ValidatePasswordStrength() = %v, want nil", err)
	}
}
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/auth/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 4: Implementasikan password.go**

Buat `internal/auth/password.go`:
```go
package auth

import (
	"golang.org/x/crypto/bcrypt"

	"go-api-starter/internal/apperror"
)

const BcryptCost = 12

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// commonPasswords is a short blocklist of passwords long enough to pass
// the length check but still trivially guessable. Intentionally small —
// the length requirement does most of the work; this just catches the
// most obvious misses.
var commonPasswords = map[string]bool{
	"password":  true,
	"12345678":  true,
	"123456789": true,
	"qwertyui":  true,
	"11111111":  true,
	"password1": true,
	"letmein11": true,
}

// ValidatePasswordStrength enforces the starter's password policy: at
// least 8 characters, not on the common-password blocklist. Deliberately
// no uppercase/digit/symbol requirement — those rules are well documented
// to push people toward predictable patterns or writing passwords down,
// without meaningfully raising guess-resistance over plain length.
func ValidatePasswordStrength(plain string) *apperror.Error {
	details := map[string][]string{}
	if len(plain) < 8 {
		details["password"] = append(details["password"], "minimal 8 karakter")
	}
	if commonPasswords[plain] {
		details["password"] = append(details["password"], "password terlalu umum, gunakan yang lain")
	}
	if len(details) > 0 {
		return apperror.Validation(details)
	}
	return nil
}
```

- [ ] **Langkah 5: Jalankan test password, pastikan lolos**

Jalankan: `go test ./internal/auth/... -run Password -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 6: Tulis test JWT yang gagal**

Buat `internal/auth/jwt_test.go`:
```go
package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndVerifyAccessToken_TenantScope(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := tm.IssueAccessToken(userID, ScopeTenant, &tenantID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := tm.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
	if claims.Scope != ScopeTenant {
		t.Errorf("Scope = %q, want %q", claims.Scope, ScopeTenant)
	}
	if claims.TenantID != tenantID.String() {
		t.Errorf("TenantID = %q, want %q", claims.TenantID, tenantID.String())
	}
}

func TestIssueAndVerifyAccessToken_PlatformScope_NoTenantID(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	userID := uuid.New()

	token, err := tm.IssueAccessToken(userID, ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := tm.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Scope != ScopePlatform {
		t.Errorf("Scope = %q, want %q", claims.Scope, ScopePlatform)
	}
	if claims.TenantID != "" {
		t.Errorf("TenantID = %q, want empty for platform scope", claims.TenantID)
	}
}

func TestVerify_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	tm1 := NewTokenManager("secret-one", 15*time.Minute)
	tm2 := NewTokenManager("secret-two", 15*time.Minute)

	token, err := tm1.IssueAccessToken(uuid.New(), ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := tm2.Verify(token); err == nil {
		t.Error("Verify() accepted a token signed with a different secret")
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret", -1*time.Minute) // already expired
	token, err := tm.IssueAccessToken(uuid.New(), ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := tm.Verify(token); err == nil {
		t.Error("Verify() accepted an expired token")
	}
}
```

- [ ] **Langkah 7: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/auth/... -run Token -v`
Hasil: FAIL — `NewTokenManager` belum ada.

- [ ] **Langkah 8: Implementasikan jwt.go**

Buat `internal/auth/jwt.go`:
```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	ScopePlatform = "platform"
	ScopeTenant   = "tenant"
)

type Claims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope"`
	TenantID string `json:"tenant_id,omitempty"`
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL}
}

// IssueAccessToken signs a short-lived access token. tenantID must be
// non-nil for ScopeTenant and nil for ScopePlatform — each side's auth
// service (Tasks 12 and 16) enforces which is which; this just carries
// whatever it's given.
func (tm *TokenManager) IssueAccessToken(userID uuid.UUID, scope string, tenantID *uuid.UUID) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tm.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Scope: scope,
	}
	if tenantID != nil {
		claims.TenantID = tenantID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

// Verify parses and validates tokenString: signature and expiry only. It
// never inspects permissions — permissions are deliberately not carried in
// the token (see Task 15) — so a verified token proves who the caller is
// and which scope they hold, nothing more.
func (tm *TokenManager) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
```

- [ ] **Langkah 9: Jalankan test JWT, pastikan lolos**

Jalankan: `go test ./internal/auth/... -run Token -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 10: Tulis test refresh token yang gagal**

Buat `internal/auth/refreshtoken_test.go`:
```go
package auth

import "testing"

func TestGenerateRefreshToken_UniqueAndHashMatches(t *testing.T) {
	plain1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	plain2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if plain1 == plain2 {
		t.Error("two calls returned the same plain token")
	}
	if hash1 == hash2 {
		t.Error("two calls returned the same hash")
	}
	if HashRefreshToken(plain1) != hash1 {
		t.Error("HashRefreshToken(plain1) does not match the hash returned alongside it")
	}
}
```

- [ ] **Langkah 11: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/auth/... -run RefreshToken -v`
Hasil: FAIL — `GenerateRefreshToken` belum ada.

- [ ] **Langkah 12: Implementasikan refreshtoken.go**

Buat `internal/auth/refreshtoken.go`:
```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateRefreshToken returns a fresh random refresh token. plain is sent
// to the client; hash is stored in the database — never the plain value
// itself, so a stolen database dump can't be replayed as a valid refresh
// token.
func GenerateRefreshToken() (plain string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashRefreshToken(plain), nil
}

func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Langkah 13: Jalankan semua test auth, pastikan lolos**

Jalankan: `go test ./internal/auth/... -v`
Hasil: PASS — semua 9 test di ketiga berkas lolos.

- [ ] **Langkah 14: Commit**

```bash
git add go.mod go.sum internal/auth
git commit -m "feat: password hashing, JWT issuing/verification, refresh tokens"
```

---

### Tugas 9: Middleware inti — request ID, logging terstruktur, recover, CORS

**Catatan desain soal komposisi context:** setiap middleware yang perlu memperkaya `context.Context` request wajib membangunnya di atas `c.UserContext()` yang *sedang berjalan*, tidak pernah dari `context.Background()` — kalau tidak, dia diam-diam membuang apa pun yang sudah ditempel middleware sebelumnya (seperti logger di tugas ini). Konvensi ini ditetapkan di sini dan diandalkan oleh tenant resolver Tugas 11.

**Berkas:**
- Buat: `internal/middleware/requestid.go`
- Buat: `internal/middleware/logger.go`
- Buat: `internal/middleware/recover.go`
- Buat: `internal/middleware/cors.go`
- Test: `internal/middleware/requestid_test.go`
- Test: `internal/middleware/logger_test.go`
- Test: `internal/middleware/recover_test.go`
- Test: `internal/middleware/cors_test.go`

**Antarmuka:**
- Menghasilkan:
  - `middleware.RequestID() fiber.Handler`, `middleware.RequestIDFromCtx(c *fiber.Ctx) string`
  - `middleware.Logger(base *slog.Logger) fiber.Handler`
  - `middleware.SetLoggerInCtx(c *fiber.Ctx, l *slog.Logger)`, `middleware.LoggerFromCtx(c *fiber.Ctx) *slog.Logger`, `middleware.LoggerFromContext(ctx context.Context) *slog.Logger`
  - `middleware.Recover() fiber.Handler`
  - `middleware.CORS(allowedOrigins []string) fiber.Handler`

- [ ] **Langkah 1: Tambah dependency ULID**

Jalankan: `go get github.com/oklog/ulid/v2`

- [ ] **Langkah 2: Tulis test request ID yang gagal**

Buat `internal/middleware/requestid_test.go`:
```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestRequestID_SetsHeaderAndLocal(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	var seen string
	app.Get("/x", func(c *fiber.Ctx) error {
		seen = middleware.RequestIDFromCtx(c)
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	header := resp.Header.Get("X-Request-ID")
	if header == "" {
		t.Error("X-Request-ID header not set")
	}
	if seen != header {
		t.Errorf("RequestIDFromCtx() = %q, want it to match response header %q", seen, header)
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp1, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	resp2, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))

	if resp1.Header.Get("X-Request-ID") == resp2.Header.Get("X-Request-ID") {
		t.Error("two requests got the same request ID")
	}
}
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 4: Implementasikan requestid.go**

Buat `internal/middleware/requestid.go`:
```go
package middleware

import (
	"crypto/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

const localsKeyRequestID = "request_id"

// RequestID assigns a ULID to every request (sortable by time, unique
// without coordination) and exposes it via the X-Request-ID response
// header and RequestIDFromCtx — the id every error response and log line
// ties back to.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var idStr string
		if id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader); err == nil {
			idStr = id.String()
		} else {
			idStr = time.Now().Format("20060102150405.000000000")
		}
		c.Locals(localsKeyRequestID, idStr)
		c.Set("X-Request-ID", idStr)
		return c.Next()
	}
}

func RequestIDFromCtx(c *fiber.Ctx) string {
	id, _ := c.Locals(localsKeyRequestID).(string)
	return id
}
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -run RequestID -v`
Hasil: PASS.

- [ ] **Langkah 6: Tulis test logger yang gagal**

Buat `internal/middleware/logger_test.go`:
```go
package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestLogger_LogsRequestSummaryWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(base))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	wantID := resp.Header.Get("X-Request-ID")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode log line: %v (raw: %s)", err, buf.String())
	}
	if line["request_id"] != wantID {
		t.Errorf("logged request_id = %v, want %v", line["request_id"], wantID)
	}
	if line["method"] != "GET" {
		t.Errorf("logged method = %v, want GET", line["method"])
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Error("log line missing duration_ms")
	}
}

func TestLoggerFromContext_AvailableInsideHandler(t *testing.T) {
	base := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(base))
	var got *slog.Logger
	app.Get("/x", func(c *fiber.Ctx) error {
		got = middleware.LoggerFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got == nil {
		t.Fatal("LoggerFromContext returned nil inside the handler")
	}
}
```

- [ ] **Langkah 7: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run Logger -v`
Hasil: FAIL — `middleware.Logger` belum ada.

- [ ] **Langkah 8: Implementasikan logger.go**

Buat `internal/middleware/logger.go`:

**Amandemen (ditambahkan saat eksekusi Tugas 9):** versi awal file ini
membaca `c.Response().StatusCode()` tanpa syarat setelah `c.Next()`
kembali. Itu salah untuk setiap respons error/panic: `ErrorHandler` global
Fiber — yang sebenarnya menulis kode status asli ke respons — baru
berjalan setelah *seluruh* rantai middleware `app.Use()` selesai
di-unwind, satu level di atas middleware individual mana pun. Mengubah
urutan `Logger`/`Recover`/`RequestID` tidak memperbaiki ini, karena
ketiganya berada dalam rantai yang sama. Perbaikan di bawah membuat
`Logger` menentukan status dari error yang dikembalikan (memakai ulang
field `*apperror.Error.Status` yang sama yang sudah dipakai
`response.Error`) setiap kali `err != nil`, dan hanya memercayai
`c.Response().StatusCode()` pada jalur sukses di mana tidak ada penulisan
ulang oleh ErrorHandler.

```go
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

const localsKeyLogger = "logger"

type loggerCtxKey struct{}

// Logger attaches a per-request *slog.Logger (tagged with request_id) to
// both fiber locals and the request's context.Context, then logs one
// structured line per request after it completes. Downstream middleware
// (the tenant resolver, Task 11) enriches the logger further — e.g. adding
// tenant_id — via SetLoggerInCtx; by the time this middleware logs its
// summary line, c.Next() has already returned, so it picks up whatever the
// final, most-enriched logger is.
func Logger(base *slog.Logger) fiber.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(c *fiber.Ctx) error {
		start := time.Now()
		SetLoggerInCtx(c, base.With("request_id", RequestIDFromCtx(c)))

		err := c.Next()

		LoggerFromCtx(c).Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", statusFor(c, err),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
}

// statusFor reports the status this request will actually be answered
// with. On the success path (err == nil), the handler already set it via
// c.Status()/c.SendStatus(), so c.Response().StatusCode() is accurate. On
// the error path, Fiber's global ErrorHandler hasn't run yet at this point
// in the middleware chain — it runs strictly after every app.Use()
// middleware returns — so the response object still holds its default
// status. Resolve the real status from the error itself instead, using the
// exact same mapping response.Error uses: an *apperror.Error's declared
// Status, or 500 for anything else.
func statusFor(c *fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Status
	}
	return 500
}

// SetLoggerInCtx stores l in both fiber locals (for handlers holding a
// *fiber.Ctx) and the request context (for services holding only a
// context.Context). It builds on c.UserContext(), never
// context.Background(), so it never discards values other middleware
// already attached (e.g. tenant info).
func SetLoggerInCtx(c *fiber.Ctx, l *slog.Logger) {
	c.Locals(localsKeyLogger, l)
	c.SetUserContext(context.WithValue(c.UserContext(), loggerCtxKey{}, l))
}

func LoggerFromCtx(c *fiber.Ctx) *slog.Logger {
	if l, ok := c.Locals(localsKeyLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// LoggerFromContext is the context.Context-based counterpart of
// LoggerFromCtx, for services that only ever see a context.Context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
```

- [ ] **Langkah 9: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -run Logger -v`
Hasil: PASS.

- [ ] **Langkah 10: Tulis test recover yang gagal**

Buat `internal/middleware/recover_test.go`:
```go
package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/response"
)

func TestRecover_ConvertsPanicToInternal500(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return response.Error(c, middleware.RequestIDFromCtx(c), err)
		},
	})
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	app.Use(middleware.Recover())
	app.Get("/x", func(c *fiber.Ctx) error {
		panic("boom")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestRecover_PassesThroughNormalRequests(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	app.Use(middleware.Recover())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(204) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204 (no panic, should pass through untouched)", resp.StatusCode)
	}
}
```

- [ ] **Langkah 11: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run Recover -v`
Hasil: FAIL — `middleware.Recover` belum ada.

- [ ] **Langkah 12: Implementasikan recover.go**

Buat `internal/middleware/recover.go`:
```go
package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

// Recover converts a panic anywhere downstream into a normal
// apperror.Internal error instead of crashing the process, logging the
// stack trace server-side. The client only ever sees request_id —
// response.Error never exposes panic detail.
func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				LoggerFromCtx(c).Error("panic recovered",
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()),
				)
				err = apperror.Internal(fmt.Errorf("panic: %v", r))
			}
		}()
		return c.Next()
	}
}
```

- [ ] **Langkah 13: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -run Recover -v`
Hasil: PASS.

- [ ] **Langkah 14: Tulis test CORS yang gagal**

Buat `internal/middleware/cors_test.go`:
```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS([]string{"http://localhost:5173"}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the configured origin", got)
	}
}

func TestCORS_RejectsUnconfiguredOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS([]string{"http://localhost:5173"}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "http://evil.example.com" {
		t.Error("CORS allowed an unconfigured origin")
	}
}
```

- [ ] **Langkah 15: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run CORS -v`
Hasil: FAIL — `middleware.CORS` belum ada.

- [ ] **Langkah 16: Implementasikan cors.go**

Buat `internal/middleware/cors.go`:
```go
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS builds Fiber's CORS middleware from the configured allowed origins.
// No wildcard — this API uses bearer tokens (AllowCredentials: true), and
// browsers reject a wildcard origin combined with credentials outright.
func CORS(allowedOrigins []string) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(allowedOrigins, ","),
		AllowHeaders:     "Content-Type,Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	})
}
```

- [ ] **Langkah 17: Jalankan semua test middleware, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -v`
Hasil: PASS — semua 10 test di keempat berkas lolos.

- [ ] **Langkah 18: Commit**

```bash
git add go.mod go.sum internal/middleware
git commit -m "feat: request ID, structured request logging, panic recovery, CORS"
```

---

### Tugas 10: Rate limit login

`login_attempts` hidup di schema `platform`, tidak peduli apakah percobaan login itu untuk platform atau tenant — service ini selalu bicara ke pool yang di-pin ke platform, tidak pernah lewat `WithTenant`.

**Berkas:**
- Buat: `internal/ratelimit/loginattempt.go`
- Ubah: `internal/testsupport/testsupport.go` — tambah `OpenTestPlatformDB`
- Test: `internal/ratelimit/loginattempt_test.go`

**Antarmuka:**
- Menghasilkan:
  - `ratelimit.NewLoginAttemptService(platformDB *sqlx.DB, maxAttempts int, window time.Duration) *LoginAttemptService`
  - `(s *LoginAttemptService) Check(ctx context.Context, scope string, tenantID *uuid.UUID, email string) error` — mengembalikan `apperror.RateLimited` kalau diblokir, `nil` kalau tidak.
  - `(s *LoginAttemptService) Record(ctx context.Context, scope string, tenantID *uuid.UUID, email string, success bool) error`
  - `testsupport.OpenTestPlatformDB(t *testing.T) *sqlx.DB` — seperti `OpenTestDB` tapi di-pin ke `search_path=platform`, mencerminkan `database.NewPlatformPool`.

- [ ] **Langkah 1: Tambah helper test yang di-pin ke platform**

Ubah `internal/testsupport/testsupport.go` — tambahkan fungsi ini di sebelah `OpenTestDB`:
```go
// OpenTestPlatformDB is OpenTestDB with search_path pinned to "platform",
// mirroring database.NewPlatformPool in production — for tests of code
// that expects to write unqualified names ("FROM login_attempts") because
// it assumes a platform-pinned connection.
func OpenTestPlatformDB(t *testing.T) *sqlx.DB {
	t.Helper()
	_ = godotenv.Load(findEnvTestFile())

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&search_path=platform",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("skipping: no local PostgreSQL test database reachable (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
```

- [ ] **Langkah 2: Tulis test yang gagal**

Buat `internal/ratelimit/loginattempt_test.go`:
```go
package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

func setupLoginAttempts(t *testing.T) *ratelimit.LoginAttemptService {
	t.Helper()
	rawDB := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawDB.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() { rawDB.Exec("TRUNCATE platform.login_attempts") })

	platformDB := testsupport.OpenTestPlatformDB(t)
	return ratelimit.NewLoginAttemptService(platformDB, 5, 15*time.Minute)
}

func TestCheck_AllowsUnderLimit(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := svc.Record(ctx, "tenant", nil, "user@example.com", false); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err != nil {
		t.Errorf("Check() = %v, want nil at 4 failed attempts (limit is 5)", err)
	}
}

func TestCheck_BlocksAtLimit(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := svc.Record(ctx, "tenant", nil, "user@example.com", false); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err == nil {
		t.Fatal("Check() = nil, want RATE_LIMITED at 5 failed attempts")
	}
}

func TestCheck_DifferentEmailsAreIndependent(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Record(ctx, "tenant", nil, "a@example.com", false)
	}

	if err := svc.Check(ctx, "tenant", nil, "b@example.com"); err != nil {
		t.Errorf("Check() for a different email = %v, want nil", err)
	}
}

func TestCheck_SuccessfulAttemptsDoNotCount(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Record(ctx, "tenant", nil, "user@example.com", true)
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err != nil {
		t.Errorf("Check() = %v, want nil — only failed attempts should count", err)
	}
}
```

- [ ] **Langkah 3: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/ratelimit/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 4: Implementasikan loginattempt.go**

Buat `internal/ratelimit/loginattempt.go`:
```go
package ratelimit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/apperror"
)

type LoginAttemptService struct {
	db          *sqlx.DB // pinned to the platform schema
	maxAttempts int
	window      time.Duration
}

func NewLoginAttemptService(platformDB *sqlx.DB, maxAttempts int, window time.Duration) *LoginAttemptService {
	return &LoginAttemptService{db: platformDB, maxAttempts: maxAttempts, window: window}
}

// Check returns apperror.RateLimited if email — scoped to scope and, for
// tenant logins, tenantID — has failed to log in maxAttempts times or more
// within the last window. Called before verifying credentials, so a
// blocked caller never even reaches the password check.
func (s *LoginAttemptService) Check(ctx context.Context, scope string, tenantID *uuid.UUID, email string) error {
	const query = `
		SELECT count(*) FROM login_attempts
		WHERE scope = $1 AND email = $2 AND success = false
		  AND attempted_at > now() - ($3 * interval '1 second')
		  AND tenant_id IS NOT DISTINCT FROM $4`
	var count int
	err := s.db.GetContext(ctx, &count, query, scope, email, s.window.Seconds(), tenantIDValue(tenantID))
	if err != nil {
		return apperror.Internal(err)
	}
	if count >= s.maxAttempts {
		return apperror.RateLimited("terlalu banyak percobaan login, coba lagi nanti")
	}
	return nil
}

// Record logs one login attempt, successful or not. Every login flow calls
// this exactly once per attempt, after Check and after verifying (or
// failing to verify) credentials.
func (s *LoginAttemptService) Record(ctx context.Context, scope string, tenantID *uuid.UUID, email string, success bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO login_attempts (id, scope, tenant_id, email, success) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), scope, tenantIDValue(tenantID), email, success,
	)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func tenantIDValue(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/ratelimit/... -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 6: Commit**

```bash
git add internal/ratelimit internal/testsupport
git commit -m "feat: login attempt rate limiting backed by platform.login_attempts"
```

---

### Tugas 11: Middleware auth — `RequirePlatform` / `RequireTenant`, cache tenant resolver, pengaman migrasi

**Catatan desain:** `TenantLookup` dideklarasikan di sini, di sisi konsumen, jadi `internal/middleware` tidak pernah meng-import `modules/platform/tenant` — Tugas 14 yang mengimplementasikan interface-nya. Satu panggilan lookup yang sama melaporkan status tenant sekaligus versi migrasi schema tenant dalam satu round trip, karena keduanya di-cache bersama dengan TTL yang sama dan sama-sama menjaga keputusan yang sama ("bolehkah request ini lanjut terhadap data tenant ini").

**Berkas:**
- Buat: `internal/middleware/tenantresolver.go`
- Buat: `internal/middleware/auth.go`
- Test: `internal/middleware/tenantresolver_test.go`
- Test: `internal/middleware/auth_test.go`

**Antarmuka:**
- Menghasilkan:
  - Struct `middleware.TenantRecord { TenantID uuid.UUID; SchemaName string; Status string; SchemaVersion uint; SchemaDirty bool }`
  - `middleware.TenantLookup interface { FindByID(ctx, tenantID uuid.UUID) (TenantRecord, error) }` — diimplementasikan Tugas 14.
  - `middleware.NewTenantResolver(lookup TenantLookup, ttl time.Duration) *TenantResolver`
  - `(r *TenantResolver) Resolve(ctx, tenantID uuid.UUID) (TenantRecord, error)`
  - `(r *TenantResolver) Invalidate(tenantID uuid.UUID)`
  - `middleware.RequirePlatform(tm *auth.TokenManager) fiber.Handler`
  - `middleware.RequireTenant(tm *auth.TokenManager, resolver *TenantResolver) fiber.Handler`

- [ ] **Langkah 1: Tulis test tenant resolver yang gagal**

Buat `internal/middleware/tenantresolver_test.go`:
```go
package middleware_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go-api-starter/internal/middleware"
)

type fakeTenantLookup struct {
	calls  int32
	record middleware.TenantRecord
	err    error
}

func (f *fakeTenantLookup) FindByID(ctx context.Context, tenantID uuid.UUID) (middleware.TenantRecord, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.record, f.err
}

func TestTenantResolver_CachesWithinTTL(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := resolver.Resolve(context.Background(), tenantID); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
	if fake.calls != 1 {
		t.Errorf("underlying lookup called %d times, want 1 (should be served from cache)", fake.calls)
	}
}

func TestTenantResolver_RefetchesAfterTTL(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, 10*time.Millisecond)

	resolver.Resolve(context.Background(), tenantID)
	time.Sleep(20 * time.Millisecond)
	resolver.Resolve(context.Background(), tenantID)

	if fake.calls != 2 {
		t.Errorf("underlying lookup called %d times, want 2 (TTL should have expired)", fake.calls)
	}
}

func TestTenantResolver_InvalidateForcesRefetch(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	resolver.Resolve(context.Background(), tenantID)
	resolver.Invalidate(tenantID)
	resolver.Resolve(context.Background(), tenantID)

	if fake.calls != 2 {
		t.Errorf("underlying lookup called %d times, want 2 (Invalidate should force a refetch)", fake.calls)
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run TenantResolver -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan tenantresolver.go**

Buat `internal/middleware/tenantresolver.go`:
```go
package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TenantRecord struct {
	TenantID      uuid.UUID
	SchemaName    string
	Status        string // provisioning | active | suspended
	SchemaVersion uint
	SchemaDirty   bool
}

// TenantLookup is implemented by the platform tenant service (Task 13).
// Declared here, on the consumer side, so this package never imports
// modules/platform/tenant.
type TenantLookup interface {
	FindByID(ctx context.Context, tenantID uuid.UUID) (TenantRecord, error)
}

type cacheEntry struct {
	record  TenantRecord
	expires time.Time
}

// TenantResolver caches tenant lookups for ttl so RequireTenant doesn't hit
// the database on every request. This is the "asumsi single instance" cache
// from the design spec: pencabutan/perubahan status only becomes visible
// on other instances after ttl elapses (or immediately on this instance if
// Invalidate is called) — acceptable as long as the app runs as one
// process; a multi-instance deployment would need this moved to Redis.
type TenantResolver struct {
	lookup TenantLookup
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[uuid.UUID]cacheEntry
}

func NewTenantResolver(lookup TenantLookup, ttl time.Duration) *TenantResolver {
	return &TenantResolver{lookup: lookup, ttl: ttl, cache: make(map[uuid.UUID]cacheEntry)}
}

func (r *TenantResolver) Resolve(ctx context.Context, tenantID uuid.UUID) (TenantRecord, error) {
	r.mu.RLock()
	entry, ok := r.cache[tenantID]
	r.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.record, nil
	}

	rec, err := r.lookup.FindByID(ctx, tenantID)
	if err != nil {
		return TenantRecord{}, err
	}

	r.mu.Lock()
	r.cache[tenantID] = cacheEntry{record: rec, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	return rec, nil
}

// Invalidate drops tenantID from the cache. Call this whenever a tenant's
// status changes (suspend/activate/delete, Task 13) so the change is
// visible on this instance's very next request instead of waiting out ttl.
func (r *TenantResolver) Invalidate(tenantID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}
```

- [ ] **Langkah 4: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -run TenantResolver -v`
Hasil: PASS — ketiga test lolos.

- [ ] **Langkah 5: Tulis test auth middleware yang gagal**

Buat `internal/middleware/auth_test.go`:
```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
)

func TestRequirePlatform_AcceptsPlatformToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	userID := uuid.New()
	token, _ := tm.IssueAccessToken(userID, auth.ScopePlatform, nil)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	var seenActor database.Actor
	app.Get("/x", func(c *fiber.Ctx) error {
		seenActor, _ = database.ActorFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if seenActor.UserID != userID {
		t.Errorf("actor.UserID = %v, want %v", seenActor.UserID, userID)
	}
}

func TestRequirePlatform_RejectsTenantScopeToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePlatform accepted a tenant-scope token")
	}
}

func TestRequirePlatform_RejectsMissingToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePlatform accepted a request with no token")
	}
}

func TestRequireTenant_AcceptsActiveTenant_SetsTenantContext(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	userID := uuid.New()
	token, _ := tm.IssueAccessToken(userID, auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	var seenTenant database.TenantInfo
	app.Get("/x", func(c *fiber.Ctx) error {
		seenTenant, _ = database.TenantFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if seenTenant.SchemaName != "acme_corp" {
		t.Errorf("tenant.SchemaName = %q, want acme_corp", seenTenant.SchemaName)
	}
}

func TestRequireTenant_RejectsSuspendedTenant(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "suspended", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequireTenant accepted a suspended tenant")
	}
}

func TestRequireTenant_RejectsBehindMigrationVersion(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 1,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequireTenant accepted a tenant whose schema is behind the binary's compiled-in migrations")
	}
}
```

- [ ] **Langkah 6: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run RequirePlatform -v` dan `go test ./internal/middleware/... -run RequireTenant -v`
Hasil: FAIL — `middleware.RequirePlatform` / `RequireTenant` belum ada.

- [ ] **Langkah 7: Implementasikan auth.go**

Buat `internal/middleware/auth.go`:
```go
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
)

func bearerToken(c *fiber.Ctx) (string, bool) {
	h := c.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimPrefix(h, prefix), true
}

// RequirePlatform verifies the request carries a valid access token with
// scope=platform and sets the actor in the request context. A tenant-scope
// token is rejected outright, even though both are signed with the same
// secret — scope is a hard partition, not just a hint.
func RequirePlatform(tm *auth.TokenManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr, ok := bearerToken(c)
		if !ok {
			return apperror.Unauthorized("token tidak ditemukan")
		}
		claims, err := tm.Verify(tokenStr)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		if claims.Scope != auth.ScopePlatform {
			return apperror.Forbidden("token ini bukan untuk admin platform")
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}

		c.SetUserContext(database.WithActor(c.UserContext(), database.Actor{
			UserID: userID, Scope: auth.ScopePlatform,
		}))
		return c.Next()
	}
}

// RequireTenant verifies the request carries a valid access token with
// scope=tenant, resolves the tenant's schema/status/migration version
// through resolver, rejects non-active tenants and schemas whose
// migrations haven't caught up with this binary (TENANT_MIGRATION_PENDING),
// and sets both actor and tenant info in the request context — everything
// database.WithTenant needs downstream.
func RequireTenant(tm *auth.TokenManager, resolver *TenantResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr, ok := bearerToken(c)
		if !ok {
			return apperror.Unauthorized("token tidak ditemukan")
		}
		claims, err := tm.Verify(tokenStr)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		if claims.Scope != auth.ScopeTenant {
			return apperror.Forbidden("token ini bukan untuk staf tenant")
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}

		record, err := resolver.Resolve(c.UserContext(), tenantID)
		if err != nil {
			return apperror.Unauthorized("tenant tidak ditemukan")
		}
		if record.Status != "active" {
			return apperror.Forbidden("tenant tidak aktif")
		}
		if record.SchemaDirty || record.SchemaVersion < migration.LatestTenantVersion() {
			return apperror.TenantMigrationPending()
		}

		ctx := c.UserContext()
		ctx = database.WithActor(ctx, database.Actor{UserID: userID, Scope: auth.ScopeTenant})
		ctx = database.WithTenantInfo(ctx, database.TenantInfo{TenantID: tenantID, SchemaName: record.SchemaName})
		c.SetUserContext(ctx)

		SetLoggerInCtx(c, LoggerFromContext(ctx).With("tenant_id", tenantID.String()))
		return c.Next()
	}
}
```

- [ ] **Langkah 8: Jalankan semua test middleware, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -v`
Hasil: PASS — semua test di paket ini lolos, termasuk 8 test baru dari tugas ini.

- [ ] **Langkah 9: Commit**

```bash
git add internal/middleware
git commit -m "feat: platform/tenant auth middleware with cached tenant resolution and migration guard"
```

---

### Tugas 12: Modul admin platform — bootstrap + auth (login/refresh/logout)

**Berkas:**
- Buat: `internal/modules/platform/auth/dto.go`
- Buat: `internal/modules/platform/auth/service.go`
- Buat: `internal/modules/platform/auth/handler.go`
- Buat: `cmd/cli/commands/admin.go`
- Test: `internal/modules/platform/auth/service_test.go`

**Antarmuka:**
- Menghasilkan:
  - `platformauth.LoginRequest { Email, Password string }`, `RefreshRequest { RefreshToken string }`, `LogoutRequest { RefreshToken string }`
  - `platformauth.LoginResponse { AccessToken, RefreshToken string; ExpiresIn int }`
  - `platformauth.NewService(platformDB *sqlx.DB, tokens *auth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service`
  - `(s *Service) Login(ctx, LoginRequest) (LoginResponse, error)`, `Refresh(ctx, RefreshRequest) (LoginResponse, error)`, `Logout(ctx, LogoutRequest) error`
  - `platformauth.NewHandler(svc *Service) *Handler` dengan method `Login`, `Refresh`, `Logout` (`func(c *fiber.Ctx) error`), disambungkan ke rute di Tugas 18.
  - Perintah CLI: `cli admin create --email=<e> --name=<n>` (bootstrap, tanpa auth).

- [ ] **Langkah 1: Tulis test service yang gagal**

Buat `internal/modules/platform/auth/service_test.go`:
```go
package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/migration"
	platformauth "go-api-starter/internal/modules/platform/auth"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

func setupService(t *testing.T) (*platformauth.Service, string) {
	t.Helper()
	rawDB := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawDB.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() {
		rawDB.Exec("TRUNCATE platform.refresh_tokens, platform.users, platform.login_attempts CASCADE")
	})

	platformDB := testsupport.OpenTestPlatformDB(t)

	plainPassword := "correct-horse-battery"
	hash, err := appauth.HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	_, err = platformDB.Exec(
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, $2, $3, $4, true)`,
		uuid.New(), "admin@example.com", hash, "Admin")
	if err != nil {
		t.Fatalf("insert admin user: %v", err)
	}

	tokens := appauth.NewTokenManager("test-secret", 15*time.Minute)
	rateLimiter := ratelimit.NewLoginAttemptService(platformDB, 5, 15*time.Minute)
	svc := platformauth.NewService(platformDB, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	return svc, plainPassword
}

func TestLogin_ValidCredentials_ReturnsTokens(t *testing.T) {
	svc, password := setupService(t)

	res, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("Login() returned an empty token")
	}
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc, _ := setupService(t)

	if _, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: "wrong"}); err == nil {
		t.Fatal("Login() with a wrong password succeeded")
	}
}

func TestRefresh_ValidToken_IssuesNewAccessToken(t *testing.T) {
	svc, password := setupService(t)
	login, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	res, err := svc.Refresh(context.Background(), platformauth.RefreshRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, password := setupService(t)
	login, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(context.Background(), platformauth.LogoutRequest{RefreshToken: login.RefreshToken}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), platformauth.RefreshRequest{RefreshToken: login.RefreshToken}); err == nil {
		t.Error("Refresh() succeeded with a revoked refresh token")
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/modules/platform/auth/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan dto.go**

Buat `internal/modules/platform/auth/dto.go`:
```go
package auth

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
```

- [ ] **Langkah 4: Implementasikan service.go**

Buat `internal/modules/platform/auth/service.go`:
```go
package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/ratelimit"
)

type adminUser struct {
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	IsActive     bool      `db:"is_active"`
}

type Service struct {
	db          *sqlx.DB // platform-pinned pool
	tokens      *appauth.TokenManager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rateLimiter *ratelimit.LoginAttemptService
}

func NewService(platformDB *sqlx.DB, tokens *appauth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service {
	return &Service{db: platformDB, tokens: tokens, accessTTL: accessTTL, refreshTTL: refreshTTL, rateLimiter: rateLimiter}
}

// dummyPasswordHash adalah hash bcrypt yang valid dari password acak yang
// tidak pernah dipakai. Login selalu memanggil VerifyPassword tepat satu
// kali — terhadap hash user asli jika ditemukan, atau terhadap hash tetap
// ini jika tidak — sehingga durasi respons tidak pernah membocorkan apakah
// sebuah email ada/aktif dibanding ada-tapi-password-salah. (Amandemen,
// ditambahkan saat eksekusi Tugas 12: versi awal di bawah ini melewati
// VerifyPassword sepenuhnya lewat short-circuit && setiap kali user tidak
// ditemukan atau tidak aktif, menciptakan celah timing yang bisa dipakai
// untuk enumerasi email.)
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := appauth.HashPassword("not-a-real-password-used-only-for-constant-time-comparison")
	if err != nil {
		panic(err)
	}
	return hash
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	if err := s.rateLimiter.Check(ctx, appauth.ScopePlatform, nil, req.Email); err != nil {
		return LoginResponse{}, err
	}

	var u adminUser
	err := s.db.GetContext(ctx, &u,
		`SELECT id, email, password_hash, is_active FROM users WHERE email = $1 AND deleted_at IS NULL`,
		req.Email)
	found := err == nil && u.IsActive

	hashToCheck := dummyPasswordHash
	if found {
		hashToCheck = u.PasswordHash
	}
	// Selalu panggil VerifyPassword — terhadap hash asli jika ditemukan,
	// terhadap hash dummy tetap jika tidak — sehingga biaya baris ini
	// (didominasi oleh bcrypt) sama di setiap jalur terlepas dari apakah
	// user tersebut ada.
	passwordOK := appauth.VerifyPassword(hashToCheck, req.Password)
	valid := found && passwordOK

	s.rateLimiter.Record(ctx, appauth.ScopePlatform, nil, req.Email, valid)
	if !valid {
		return LoginResponse{}, apperror.Unauthorized("email atau password salah")
	}

	return s.issueTokens(ctx, u.ID)
}

func (s *Service) issueTokens(ctx context.Context, userID uuid.UUID) (LoginResponse, error) {
	access, err := s.tokens.IssueAccessToken(userID, appauth.ScopePlatform, nil)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	plain, hash, err := appauth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		uuid.New(), userID, hash, time.Now().Add(s.refreshTTL))
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	return LoginResponse{AccessToken: access, RefreshToken: plain, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (LoginResponse, error) {
	hash := appauth.HashRefreshToken(req.RefreshToken)

	var row struct {
		UserID    uuid.UUID  `db:"user_id"`
		ExpiresAt time.Time  `db:"expires_at"`
		RevokedAt *time.Time `db:"revoked_at"`
	}
	err := s.db.GetContext(ctx, &row,
		`SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash)
	if err != nil {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	if row.RevokedAt != nil || time.Now().After(row.ExpiresAt) {
		return LoginResponse{}, apperror.Unauthorized("refresh token sudah tidak berlaku")
	}

	// No rotation (spec: refresh tokens are not rotated on use) — the same
	// refresh token stays valid, only a new access token is issued.
	access, err := s.tokens.IssueAccessToken(row.UserID, appauth.ScopePlatform, nil)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	return LoginResponse{AccessToken: access, RefreshToken: req.RefreshToken, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	hash := appauth.HashRefreshToken(req.RefreshToken)
	_, err := s.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/modules/platform/auth/... -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 6: Implementasikan handler**

Buat `internal/modules/platform/auth/handler.go`:
```go
package auth

import (
	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/response"
	"go-api-starter/internal/validator"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func badBody(c *fiber.Ctx) error {
	return response.Error(c, middleware.RequestIDFromCtx(c),
		apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	res, err := h.svc.Login(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	res, err := h.svc.Refresh(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	if err := h.svc.Logout(c.UserContext(), req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil logout"})
}
```

- [ ] **Langkah 7: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 8: Tulis perintah CLI bootstrap admin**

Buat `cmd/cli/commands/admin.go`:
```go
package commands

import (
	"flag"
	"fmt"

	"github.com/google/uuid"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
)

func init() {
	Register("admin create", cmdAdminCreate)
}

// cmdAdminCreate solves the chicken-and-egg problem of the very first
// platform admin: creating one normally requires being logged in as one,
// but platform.users starts empty. This command runs with no auth check
// at all — its safety comes from requiring shell access to the server, not
// from a token — and prints a random password once, which must be changed
// on first login.
func cmdAdminCreate(args []string) error {
	fs := flag.NewFlagSet("admin create", flag.ExitOnError)
	email := fs.String("email", "", "admin email (required)")
	name := fs.String("name", "", "admin name (required)")
	fs.Parse(args)

	if *email == "" || *name == "" {
		return fmt.Errorf("both --email and --name are required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	password, err := generateRandomPassword()
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = pool.Exec(
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, $2, $3, $4, true)`,
		uuid.New(), *email, hash, *name)
	if err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}

	fmt.Println("admin berhasil dibuat:")
	fmt.Println("  email:   ", *email)
	fmt.Println("  password:", password)
	fmt.Println("(password ini hanya ditampilkan sekali — ganti setelah login pertama)")
	return nil
}

func generateRandomPassword() (string, error) {
	// Reuses the same crypto/rand 32-byte generator as refresh tokens
	// rather than adding a second random-string helper.
	plain, _, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	return plain[:20], nil
}
```

- [ ] **Langkah 9: Verifikasi manual**

Jalankan:
```bash
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
```
Hasil: mencetak password yang di-generate. Salin — Tugas 18 memungkinkan kamu benar-benar login memakainya begitu HTTP server sudah ada.

- [ ] **Langkah 10: Commit**

```bash
git add internal/modules/platform/auth cmd/cli/commands/admin.go
git commit -m "feat: platform admin auth (login/refresh/logout) and bootstrap CLI command"
```

---

### Tugas 13: Konstanta permission + mekanisme sync + CLI

Dibangun sebelum provisioning tenant (tugas berikutnya) karena provisioning butuh `permission.All` untuk mengisi tabel `permissions` tenant yang baru dibuat.

**Berkas:**
- Buat: `internal/permission/constants.go`
- Buat: `internal/permission/sync.go`
- Buat: `cmd/cli/commands/permission.go`
- Test: `internal/permission/sync_test.go`

**Antarmuka:**
- Menghasilkan:
  - `permission.UserView`, `UserCreate`, `UserUpdate`, `UserDelete`, `RoleView`, `RoleManage` (konstanta string)
  - `permission.All []string` — semua konstanta di atas; sumber kebenaran yang jadi dasar provisioning Tugas 14.
  - `permission.Sync(ctx context.Context, db *sql.DB, schemaName string) (added int, err error)`
  - Perintah CLI: `cli permission sync --all` / `--tenant=<code>`

- [ ] **Langkah 1: Tulis test yang gagal**

Buat `internal/permission/sync_test.go`:
```go
package permission_test

import (
	"context"
	"testing"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/testsupport"
)

func TestSync_SeedsAllPermissions(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	added, err := permission.Sync(context.Background(), pool.DB, schema)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if added != len(permission.All) {
		t.Errorf("added = %d, want %d", added, len(permission.All))
	}

	var count int
	if err := pool.Get(&count, "SELECT count(*) FROM "+schema+".permissions"); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if count != len(permission.All) {
		t.Errorf("permissions table has %d rows, want %d", count, len(permission.All))
	}
}

func TestSync_IsIdempotent(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	if _, err := permission.Sync(context.Background(), pool.DB, schema); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	added, err := permission.Sync(context.Background(), pool.DB, schema)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if added != 0 {
		t.Errorf("second Sync added %d permissions, want 0 (already synced)", added)
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/permission/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan constants.go**

Buat `internal/permission/constants.go`:
```go
package permission

const (
	UserView   = "user.view"
	UserCreate = "user.create"
	UserUpdate = "user.update"
	UserDelete = "user.delete"

	RoleView   = "role.view"
	RoleManage = "role.manage"
)

// All lists every permission constant above. Tenant provisioning (Task 14)
// seeds each of these into a brand new tenant's permissions table; Sync
// (sync.go) brings already-provisioned tenants up to date when a new
// constant is added later.
var All = []string{
	UserView, UserCreate, UserUpdate, UserDelete,
	RoleView, RoleManage,
}
```

- [ ] **Langkah 4: Implementasikan sync.go**

Buat `internal/permission/sync.go`:
```go
package permission

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// Sync inserts any permission in All that schemaName's permissions table
// doesn't already have, and reports how many were added. Idempotent — safe
// to run against every tenant on every deploy — because permission
// constants live in Go code, not in migration files, so a newly added
// constant needs an explicit push to reach tenants provisioned before it
// existed.
func Sync(ctx context.Context, db *sql.DB, schemaName string) (added int, err error) {
	if !database.ValidSchemaName(schemaName) {
		return 0, apperror.Internal(fmt.Errorf("invalid schema name %q", schemaName))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO "`+schemaName+`"`); err != nil {
		return 0, apperror.Internal(err)
	}

	existing := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT code FROM permissions`)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			return 0, apperror.Internal(err)
		}
		existing[code] = true
	}
	rows.Close()

	for _, code := range All {
		if existing[code] {
			continue
		}
		group := strings.SplitN(code, ".", 2)[0]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO permissions (id, code, "group") VALUES ($1, $2, $3)`,
			uuid.New(), code, group); err != nil {
			return added, apperror.Internal(err)
		}
		added++
	}

	if err := tx.Commit(); err != nil {
		return 0, apperror.Internal(err)
	}
	return added, nil
}
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/permission/... -v`
Hasil: PASS — kedua test lolos.

- [ ] **Langkah 6: Tulis perintah CLI**

Buat `cmd/cli/commands/permission.go`:
```go
package commands

import (
	"context"
	"flag"
	"fmt"

	"go-api-starter/internal/config"
	"go-api-starter/internal/permission"
)

func init() {
	Register("permission sync", cmdPermissionSync)
}

func cmdPermissionSync(args []string) error {
	fs := flag.NewFlagSet("permission sync", flag.ExitOnError)
	all := fs.Bool("all", false, "sync every tenant")
	tenantCode := fs.String("tenant", "", "sync a single tenant by code")
	fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, *all, *tenantCode) // shared helper from migrate.go
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no matching tenants found")
	}

	for _, tgt := range targets {
		added, err := permission.Sync(context.Background(), db, tgt.schemaName)
		if err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s +%d permission(s)\n", tgt.code, added)
	}
	return nil
}
```

- [ ] **Langkah 7: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 8: Commit**

```bash
git add internal/permission cmd/cli/commands/permission.go
git commit -m "feat: permission constants and cli permission sync command"
```

---

### Tugas 14: Modul tenant platform — provisioning, CRUD, CLI

**Catatan desain — tanpa `handler.go`:** spec dengan tegas menaruh endpoint HTTP self-service untuk provisioning **di luar cakupan** starter ini ("endpoint admin self-service ... menyusul"). Tugas ini membangun `Service` yang bisa dipakai ulang, yang nanti dipanggil endpoint tersebut, dan untuk sekarang mengeksposnya hanya lewat CLI. Karena itu `modules/platform/tenant/` tidak punya `handler.go` — itu menyusul nanti, disambungkan ke `Service` yang sama ini, tanpa perubahan apa pun padanya.

**Berkas:**
- Buat: `internal/modules/platform/tenant/model.go`
- Buat: `internal/modules/platform/tenant/dto.go`
- Buat: `internal/modules/platform/tenant/repository.go`
- Buat: `internal/modules/platform/tenant/service.go`
- Buat: `cmd/cli/commands/tenant.go`
- Test: `internal/modules/platform/tenant/service_test.go`

**Antarmuka:**
- Menghasilkan:
  - `tenant.Tenant` (struct baris DB), `tenant.ProvisionInput`, `tenant.ProvisionResult`, `tenant.TenantView`
  - `tenant.NewRepository(platformDB *sqlx.DB, rawDB *sql.DB) *Repository`
  - `(r *Repository) FindByID(ctx, uuid.UUID) (middleware.TenantRecord, error)` — **mengimplementasikan `middleware.TenantLookup`** (Tugas 11).
  - `(r *Repository) Create/FindByCode/FindByCodeIncludingDeleted/List/UpdateStatus/SoftDelete`
  - `tenant.NewService(repo *Repository, rawDB *sql.DB, resolver *middleware.TenantResolver) *Service`
  - `(s *Service) Provision(ctx, ProvisionInput) (ProvisionResult, error)`
  - `(s *Service) Suspend/Activate/Delete(ctx, code string) error`
  - `(s *Service) Purge(ctx, code string) error`
  - `(s *Service) List(ctx, pagination.Params) ([]TenantView, response.Meta, error)`
  - CLI: `cli tenant create --code= --name= --owner-email=`, `cli tenant list`, `cli tenant suspend --code=`, `cli tenant activate --code=`, `cli tenant delete --code=`, `cli tenant purge --code= --confirm`

- [ ] **Langkah 1: Tulis test yang gagal**

Buat `internal/modules/platform/tenant/service_test.go`:
```go
package tenant_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	"go-api-starter/internal/testsupport"
)

// setupTenantService gives every test in this file a fresh Service wired
// to real platform.tenants + a raw *sql.DB for provisioning transactions.
func setupTenantService(t *testing.T) (*platformtenant.Service, *platformtenant.Repository) {
	t.Helper()
	rawPool := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawPool.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() { rawPool.Exec("TRUNCATE platform.tenants CASCADE") })

	platformDB := testsupport.OpenTestPlatformDB(t)
	repo := platformtenant.NewRepository(platformDB, rawPool.DB)
	resolver := middleware.NewTenantResolver(repo, time.Minute)
	svc := platformtenant.NewService(repo, rawPool.DB, resolver)
	return svc, repo
}

func TestProvision_CreatesActiveTenantWithOwner(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "acme_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })

	result, err := svc.Provision(ctx, platformtenant.ProvisionInput{
		Code: code, Name: "Acme Test", OwnerEmail: "owner@example.com",
	})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if result.Tenant.Status != "active" {
		t.Errorf("Tenant.Status = %q, want active", result.Tenant.Status)
	}
	if result.OwnerPassword == "" {
		t.Error("OwnerPassword is empty")
	}
}

func TestProvision_DuplicateCode_ReturnsConflict(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "dup_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })

	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "First", OwnerEmail: "a@example.com"}); err != nil {
		t.Fatalf("first Provision: %v", err)
	}
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "Second", OwnerEmail: "b@example.com"}); err == nil {
		t.Fatal("second Provision with the same code succeeded, want conflict")
	}
}

func TestSuspendAndActivate_ChangeStatus(t *testing.T) {
	svc, repo := setupTenantService(t)
	ctx := context.Background()
	code := "susp_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if err := svc.Suspend(ctx, code); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	t2, err := repo.FindByCode(ctx, code)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if t2.Status != "suspended" {
		t.Errorf("Status = %q, want suspended", t2.Status)
	}

	if err := svc.Activate(ctx, code); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t3, err := repo.FindByCode(ctx, code)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if t3.Status != "active" {
		t.Errorf("Status = %q, want active", t3.Status)
	}
}

func TestPurge_RefusesBeforeThirtyDays(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "purge1_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if err := svc.Delete(ctx, code); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if err := svc.Purge(ctx, code); err == nil {
		t.Fatal("Purge() succeeded immediately after soft delete, want refusal (< 30 days)")
	}
}

func TestFindByID_ReportsSchemaVersion(t *testing.T) {
	svc, repo := setupTenantService(t)
	ctx := context.Background()
	code := "ver_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	result, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	tenantID, err := uuid.Parse(result.Tenant.ID)
	if err != nil {
		t.Fatalf("parse tenant id: %v", err)
	}
	record, err := repo.FindByID(ctx, tenantID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if record.SchemaVersion != migration.LatestTenantVersion() {
		t.Errorf("SchemaVersion = %d, want %d (freshly provisioned tenant should be at the latest version)",
			record.SchemaVersion, migration.LatestTenantVersion())
	}
	if record.SchemaDirty {
		t.Error("SchemaDirty = true for a freshly provisioned tenant")
	}
}

func dropSchemaIfExists(t *testing.T, schemaName string) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	pool.Exec(`DROP SCHEMA IF EXISTS "` + schemaName + `" CASCADE`)
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/modules/platform/tenant/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan model.go dan dto.go**

Buat `internal/modules/platform/tenant/model.go`:
```go
package tenant

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID          uuid.UUID  `db:"id"`
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	SchemaName  string     `db:"schema_name"`
	Status      string     `db:"status"`
	SuspendedAt *time.Time `db:"suspended_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}
```

Buat `internal/modules/platform/tenant/dto.go`:
```go
package tenant

type ProvisionInput struct {
	Code       string `json:"code" validate:"required,min=3,max=50"`
	Name       string `json:"name" validate:"required"`
	OwnerEmail string `json:"owner_email" validate:"required,email"`
}

type ProvisionResult struct {
	Tenant        TenantView `json:"tenant"`
	OwnerPassword string     `json:"owner_password"`
}

type TenantView struct {
	ID     string `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
```

- [ ] **Langkah 4: Implementasikan repository.go**

Buat `internal/modules/platform/tenant/repository.go`:
```go
package tenant

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
)

type Repository struct {
	db    *sqlx.DB // platform-pinned pool, for normal CRUD
	rawDB *sql.DB  // unqualified connection, for schema-scoped migration version checks
}

func NewRepository(platformDB *sqlx.DB, rawDB *sql.DB) *Repository {
	return &Repository{db: platformDB, rawDB: rawDB}
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Tenant, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE code = $1 AND deleted_at IS NULL`, code)
	if err == sql.ErrNoRows {
		return Tenant{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return Tenant{}, apperror.Internal(err)
	}
	return t, nil
}

func (r *Repository) FindByCodeIncludingDeleted(ctx context.Context, code string) (Tenant, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE code = $1`, code)
	if err == sql.ErrNoRows {
		return Tenant{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return Tenant{}, apperror.Internal(err)
	}
	return t, nil
}

// FindByID implements middleware.TenantLookup: it reports both the
// tenant's platform-side status and its schema's migration version in one
// call, because RequireTenant needs both to decide whether a request may
// proceed, and both are cached together under the same TTL.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (middleware.TenantRecord, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return middleware.TenantRecord{}, apperror.NotFound("tenant tidak ditemukan")
	}

	version, dirty, err := migration.TenantSchemaVersion(r.rawDB, t.SchemaName)
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}

	return middleware.TenantRecord{
		TenantID: t.ID, SchemaName: t.SchemaName, Status: t.Status,
		SchemaVersion: version, SchemaDirty: dirty,
	}, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int, orderBy string) ([]Tenant, int, error) {
	var tenants []Tenant
	query := `SELECT * FROM tenants WHERE deleted_at IS NULL ORDER BY ` + orderBy + ` LIMIT $1 OFFSET $2`
	if err := r.db.SelectContext(ctx, &tenants, query, limit, offset); err != nil {
		return nil, 0, apperror.Internal(err)
	}
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT count(*) FROM tenants WHERE deleted_at IS NULL`); err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return tenants, total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, suspendedAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET status = $2, suspended_at = $3 WHERE id = $1`, id, status, suspendedAt)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tenants SET deleted_at = now() WHERE id = $1`, id)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
```

- [ ] **Langkah 5: Implementasikan service.go**

Buat `internal/modules/platform/tenant/service.go`:
```go
package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/response"
)

type Service struct {
	repo     *Repository
	rawDB    *sql.DB
	resolver *middleware.TenantResolver
}

func NewService(repo *Repository, rawDB *sql.DB, resolver *middleware.TenantResolver) *Service {
	return &Service{repo: repo, rawDB: rawDB, resolver: resolver}
}

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

func normalizeCode(raw string) (string, error) {
	code := strings.ToLower(strings.TrimSpace(raw))
	code = strings.ReplaceAll(code, "-", "_")
	code = strings.ReplaceAll(code, " ", "_")
	if !codeRe.MatchString(code) {
		return "", apperror.Validation(map[string][]string{
			"code": {"kode tenant harus 3-50 karakter huruf kecil, angka, dan garis bawah"},
		})
	}
	return code, nil
}

func randomPassword() (string, error) {
	plain, _, err := appauth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	return plain[:20], nil
}

// Provision creates a brand new tenant end to end: validates the code,
// creates its schema, applies every tenant migration, seeds permissions
// and the owner role, creates the owner user, and marks the tenant active
// — all inside one *sql.Tx, so a failure at any step leaves nothing behind
// (PostgreSQL DDL is transactional, so even CREATE SCHEMA rolls back).
//
// Tenant migrations are applied by reading the same files golang-migrate
// itself uses (migration.TenantMigrationFiles) and exec'ing their SQL
// directly against this transaction, since golang-migrate manages its own
// connection and can't join a caller's transaction. Afterwards this writes
// schema_migrations by hand, in the exact format golang-migrate uses, so
// `cli migrate tenant up`/`status` (Task 7) stay accurate for this tenant.
func (s *Service) Provision(ctx context.Context, in ProvisionInput) (ProvisionResult, error) {
	code, err := normalizeCode(in.Code)
	if err != nil {
		return ProvisionResult{}, err
	}
	if !database.ValidSchemaName(code) {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("normalized code %q is not a valid schema name", code))
	}

	tx, err := s.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	defer tx.Rollback()

	tenantID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO platform.tenants (id, code, name, schema_name, status) VALUES ($1, $2, $3, $4, 'provisioning')`,
		tenantID, code, in.Name, code); err != nil {
		return ProvisionResult{}, apperror.Conflict("kode tenant sudah dipakai")
	}

	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA "`+code+`"`); err != nil {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("create schema: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO "`+code+`"`); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	files, err := migration.TenantMigrationFiles()
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	var latestVersion uint
	for _, f := range files {
		if _, err := tx.ExecContext(ctx, f.SQL); err != nil {
			return ProvisionResult{}, apperror.Internal(fmt.Errorf("apply tenant migration %d: %w", f.Version, err))
		}
		latestVersion = f.Version
	}
	if _, err := tx.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version bigint primary key, dirty boolean not null)`); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)`, latestVersion); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	for _, permCode := range permission.All {
		group := strings.SplitN(permCode, ".", 2)[0]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO permissions (id, code, "group") VALUES ($1, $2, $3)`,
			uuid.New(), permCode, group); err != nil {
			return ProvisionResult{}, apperror.Internal(err)
		}
	}

	ownerRoleID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'owner', true)`, ownerRoleID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	// owner gets every permission via the is_system bypass at check time
	// (Task 15), not rows in role_permissions — no seed loop needed here.

	ownerID := uuid.New()
	ownerPassword, err := randomPassword()
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	ownerHash, err := appauth.HashPassword(ownerPassword)
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, $2, $3, 'Owner', true)`,
		ownerID, in.OwnerEmail, ownerHash); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id, created_at) VALUES ($1, $2, now())`, ownerID, ownerRoleID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE platform.tenants SET status = 'active' WHERE id = $1`, tenantID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	if err := tx.Commit(); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	return ProvisionResult{
		Tenant:        TenantView{ID: tenantID.String(), Code: code, Name: in.Name, Status: "active"},
		OwnerPassword: ownerPassword,
	}, nil
}

func (s *Service) Suspend(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, t.ID, "suspended", &now); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

func (s *Service) Activate(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, t.ID, "active", nil); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

func (s *Service) Delete(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, t.ID); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

const purgeMinAge = 30 * 24 * time.Hour

// Purge permanently drops a soft-deleted tenant's schema. It refuses
// unless the tenant has been soft-deleted for at least purgeMinAge — DROP
// SCHEMA can't be undone, and this delay is the only safety net against a
// mistyped tenant code destroying a client's data outright.
func (s *Service) Purge(ctx context.Context, code string) error {
	t, err := s.repo.FindByCodeIncludingDeleted(ctx, code)
	if err != nil {
		return err
	}
	if t.DeletedAt == nil {
		return apperror.Conflict("tenant belum dihapus (soft delete) — hapus dulu sebelum purge")
	}
	if time.Since(*t.DeletedAt) < purgeMinAge {
		return apperror.Conflict("tenant baru bisa di-purge 30 hari setelah dihapus")
	}
	if !database.ValidSchemaName(t.SchemaName) {
		return apperror.Internal(fmt.Errorf("invalid schema name %q", t.SchemaName))
	}
	if _, err := s.rawDB.ExecContext(ctx, `DROP SCHEMA "`+t.SchemaName+`" CASCADE`); err != nil {
		return apperror.Internal(err)
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

var sortable = map[string]string{
	"created_at": "created_at",
	"code":       "code",
	"name":       "name",
}

func (s *Service) List(ctx context.Context, p pagination.Params) ([]TenantView, response.Meta, error) {
	tenants, total, err := s.repo.List(ctx, p.Limit, p.Offset(), p.OrderByClause(sortable))
	if err != nil {
		return nil, response.Meta{}, err
	}
	views := make([]TenantView, len(tenants))
	for i, t := range tenants {
		views[i] = TenantView{ID: t.ID.String(), Code: t.Code, Name: t.Name, Status: t.Status}
	}
	return views, pagination.BuildMeta(p.Page, p.Limit, total), nil
}
```

- [ ] **Langkah 6: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/modules/platform/tenant/... -v`
Hasil: PASS — kelima test lolos. (Test ini lambat — masing-masing melakukan provisioning schema sungguhan dengan empat migrasi tenant — total beberapa detik itu wajar.)

- [ ] **Langkah 7: Tulis perintah CLI tenant**

Buat `cmd/cli/commands/tenant.go`:
```go
package commands

import (
	"context"
	"flag"
	"fmt"

	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	"go-api-starter/internal/pagination"
)

func init() {
	Register("tenant create", cmdTenantCreate)
	Register("tenant list", cmdTenantList)
	Register("tenant suspend", cmdTenantSuspend)
	Register("tenant activate", cmdTenantActivate)
	Register("tenant delete", cmdTenantDelete)
	Register("tenant purge", cmdTenantPurge)
}

func newTenantService(cfg *config.Config) (*platformtenant.Service, func(), error) {
	platformDB, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		return nil, nil, fmt.Errorf("connect (platform pool): %w", err)
	}
	rawDB, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		platformDB.Close()
		return nil, nil, fmt.Errorf("connect (raw pool): %w", err)
	}

	repo := platformtenant.NewRepository(platformDB, rawDB)
	resolver := middleware.NewTenantResolver(repo, 0) // CLI never needs caching — one-shot process
	svc := platformtenant.NewService(repo, rawDB, resolver)

	cleanup := func() {
		platformDB.Close()
		rawDB.Close()
	}
	return svc, cleanup, nil
}

func cmdTenantCreate(args []string) error {
	fs := flag.NewFlagSet("tenant create", flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	name := fs.String("name", "", "tenant name (required)")
	ownerEmail := fs.String("owner-email", "", "first owner's email (required)")
	fs.Parse(args)

	if *code == "" || *name == "" || *ownerEmail == "" {
		return fmt.Errorf("--code, --name, and --owner-email are all required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := svc.Provision(context.Background(), platformtenant.ProvisionInput{
		Code: *code, Name: *name, OwnerEmail: *ownerEmail,
	})
	if err != nil {
		return err
	}

	fmt.Println("tenant berhasil di-provisioning:")
	fmt.Println("  code:          ", result.Tenant.Code)
	fmt.Println("  owner email:   ", *ownerEmail)
	fmt.Println("  owner password:", result.OwnerPassword)
	fmt.Println("(password ini hanya ditampilkan sekali — ganti setelah login pertama)")
	return nil
}

func cmdTenantList(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	views, _, err := svc.List(context.Background(), pagination.Params{
		Page: 1, Limit: 100, Sort: "created_at", Order: "desc",
	})
	if err != nil {
		return err
	}
	for _, v := range views {
		fmt.Printf("%-20s %-30s %s\n", v.Code, v.Name, v.Status)
	}
	return nil
}

func cmdTenantSuspend(args []string) error  { return tenantStatusCommand(args, "tenant suspend", (*platformtenant.Service).Suspend) }
func cmdTenantActivate(args []string) error { return tenantStatusCommand(args, "tenant activate", (*platformtenant.Service).Activate) }
func cmdTenantDelete(args []string) error   { return tenantStatusCommand(args, "tenant delete", (*platformtenant.Service).Delete) }

func tenantStatusCommand(args []string, name string, action func(*platformtenant.Service, context.Context, string) error) error {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := action(svc, context.Background(), *code); err != nil {
		return err
	}
	fmt.Println("ok")
	return nil
}

func cmdTenantPurge(args []string) error {
	fs := flag.NewFlagSet("tenant purge", flag.ExitOnError)
	code := fs.String("code", "", "tenant code (required)")
	confirm := fs.Bool("confirm", false, "required acknowledgement — this is irreversible")
	fs.Parse(args)
	if *code == "" {
		return fmt.Errorf("--code is required")
	}
	if !*confirm {
		return fmt.Errorf("pass --confirm to acknowledge this permanently deletes the tenant's data")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	svc, cleanup, err := newTenantService(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := svc.Purge(context.Background(), *code); err != nil {
		return err
	}
	fmt.Println("tenant berhasil di-purge secara permanen")
	return nil
}
```

- [ ] **Langkah 8: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 9: Verifikasi manual end-to-end**

Jalankan:
```bash
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acmecorp.test
go run ./cmd/cli tenant list
go run ./cmd/cli tenant suspend --code=acme_corp
go run ./cmd/cli tenant list
go run ./cmd/cli tenant activate --code=acme_corp
```
Hasil: tiap perintah mencetak hasil yang masuk akal; `tenant list` menunjukkan status berubah antara `active` dan `suspended`.

- [ ] **Langkah 10: Commit**

```bash
git add internal/modules/platform/tenant cmd/cli/commands/tenant.go
git commit -m "feat: tenant provisioning service, lifecycle management, and cli tenant commands"
```

---

### Tugas 15: RBAC — middleware `RequirePermission`, cache permission, service role tenant

Dua bagian, pembagian sama seperti Tugas 11: `middleware.PermissionChecker` dideklarasikan di sisi konsumen (`internal/middleware`); `modules/tenant/role` mengimplementasikannya terhadap tabel tenant sungguhan. Di sinilah paket `modules/tenant/role` dari Struktur Berkas dibangun — bukan tugas terpisah karena satu-satunya tugasnya adalah menjadi implementasi interface ini.

**Berkas:**
- Buat: `internal/middleware/rbac.go`
- Buat: `internal/modules/tenant/role/service.go`
- Test: `internal/middleware/rbac_test.go`
- Test: `internal/modules/tenant/role/service_test.go`

**Antarmuka:**
- Menghasilkan:
  - `middleware.PermissionChecker interface { HasPermission(ctx, userID uuid.UUID, permCode string) (bool, error) }`
  - `middleware.NewPermissionCache(checker PermissionChecker, ttl time.Duration) *PermissionCache`
  - `(c *PermissionCache) HasPermission(ctx, tenantID, userID uuid.UUID, permCode string) (bool, error)`
  - `middleware.RequirePermission(permCode string, cache *PermissionCache) fiber.Handler` — harus dijalankan setelah `RequireTenant`.
  - `role.NewService(db *database.DB) *Service` — **mengimplementasikan `middleware.PermissionChecker`**.

- [ ] **Langkah 1: Tulis test middleware RBAC yang gagal**

Buat `internal/middleware/rbac_test.go`:
```go
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/middleware"
)

type fakePermissionChecker struct {
	calls   int32
	allowed bool
	err     error
}

func (f *fakePermissionChecker) HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.allowed, f.err
}

func TestPermissionCache_CachesWithinTTL(t *testing.T) {
	fake := &fakePermissionChecker{allowed: true}
	cache := middleware.NewPermissionCache(fake, time.Minute)
	tenantID, userID := uuid.New(), uuid.New()

	for i := 0; i < 3; i++ {
		if _, err := cache.HasPermission(context.Background(), tenantID, userID, "user.view"); err != nil {
			t.Fatalf("HasPermission: %v", err)
		}
	}
	if fake.calls != 1 {
		t.Errorf("checker called %d times, want 1 (should be cached)", fake.calls)
	}
}

func TestPermissionCache_DifferentPermissionsAreIndependent(t *testing.T) {
	fake := &fakePermissionChecker{allowed: true}
	cache := middleware.NewPermissionCache(fake, time.Minute)
	tenantID, userID := uuid.New(), uuid.New()

	cache.HasPermission(context.Background(), tenantID, userID, "user.view")
	cache.HasPermission(context.Background(), tenantID, userID, "user.create")

	if fake.calls != 2 {
		t.Errorf("checker called %d times, want 2 (different permissions must not share a cache slot)", fake.calls)
	}
}

func TestRequirePermission_AllowsWhenPermitted(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	tenantFake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(tenantFake, time.Minute)
	permCache := middleware.NewPermissionCache(&fakePermissionChecker{allowed: true}, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", middleware.RequirePermission("user.view", permCache), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRequirePermission_RejectsWhenNotPermitted(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	tenantFake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(tenantFake, time.Minute)
	permCache := middleware.NewPermissionCache(&fakePermissionChecker{allowed: false}, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", middleware.RequirePermission("user.delete", permCache), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePermission allowed a request the checker denied")
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/middleware/... -run "PermissionCache|RequirePermission" -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan rbac.go**

Buat `internal/middleware/rbac.go`:
```go
package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// PermissionChecker is implemented by the tenant role service
// (modules/tenant/role, this task). Declared here, consumer-side, so this
// package never imports modules/tenant/role.
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error)
}

type permCacheKey struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	perm     string
}

type permCacheEntry struct {
	allowed bool
	expires time.Time
}

// PermissionCache wraps a PermissionChecker with a short TTL cache keyed
// by (tenant, user, permission) — same "asumsi single instance" caveat as
// TenantResolver (Task 11): a revoked permission is only guaranteed to
// take effect on this instance after ttl elapses.
type PermissionCache struct {
	checker PermissionChecker
	ttl     time.Duration
	mu      sync.RWMutex
	cache   map[permCacheKey]permCacheEntry
}

func NewPermissionCache(checker PermissionChecker, ttl time.Duration) *PermissionCache {
	return &PermissionCache{checker: checker, ttl: ttl, cache: make(map[permCacheKey]permCacheEntry)}
}

func (c *PermissionCache) HasPermission(ctx context.Context, tenantID, userID uuid.UUID, permCode string) (bool, error) {
	key := permCacheKey{tenantID: tenantID, userID: userID, perm: permCode}

	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.allowed, nil
	}

	allowed, err := c.checker.HasPermission(ctx, userID, permCode)
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	c.cache[key] = permCacheEntry{allowed: allowed, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return allowed, nil
}

// RequirePermission rejects the request unless the authenticated tenant
// user holds permCode. It must run after RequireTenant — it reads the
// tenant and actor info that only RequireTenant sets in the context.
func RequirePermission(permCode string, cache *PermissionCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		actor, ok := database.ActorFromContext(c.UserContext())
		if !ok {
			return apperror.Unauthorized("tidak terautentikasi")
		}
		tenant, ok := database.TenantFromContext(c.UserContext())
		if !ok {
			return apperror.Internal(fmt.Errorf("RequirePermission used without RequireTenant"))
		}

		allowed, err := cache.HasPermission(c.UserContext(), tenant.TenantID, actor.UserID, permCode)
		if err != nil {
			return apperror.Internal(err)
		}
		if !allowed {
			return apperror.Forbidden("tidak punya izin untuk aksi ini")
		}
		return c.Next()
	}
}
```

- [ ] **Langkah 4: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/middleware/... -run "PermissionCache|RequirePermission" -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 5: Tulis test role service yang gagal**

Buat `internal/modules/tenant/role/service_test.go`:
```go
package role_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
	"go-api-starter/internal/modules/tenant/role"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/testsupport"
)

func setupRoleService(t *testing.T) (*role.Service, database.TenantInfo, *sqlx.DB) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}
	if _, err := permission.Sync(context.Background(), pool.DB, schema); err != nil {
		t.Fatalf("permission.Sync: %v", err)
	}

	tenantInfo := database.TenantInfo{TenantID: uuid.New(), SchemaName: schema}
	return role.NewService(database.NewDB(pool)), tenantInfo, pool
}

// seedInSchema runs a fixture-setup query with search_path pinned to
// schema — tests seed rows directly rather than going through
// database.WithTenant themselves.
func seedInSchema(t *testing.T, pool *sqlx.DB, schema, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SET LOCAL search_path TO "` + schema + `"`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(query, args...); err != nil {
		t.Fatalf("seed query: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestHasPermission_OwnerBypass_AlwaysAllowed(t *testing.T) {
	svc, tenantInfo, pool := setupRoleService(t)
	ctx := database.WithTenantInfo(context.Background(), tenantInfo)

	userID, roleID := uuid.New(), uuid.New()
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'owner@x.com', 'hash', 'Owner', true)`, userID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'owner', true)`, roleID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	allowed, err := svc.HasPermission(ctx, userID, "some.permission.never.seeded")
	if err != nil {
		t.Fatalf("HasPermission: %v", err)
	}
	if !allowed {
		t.Error("owner (is_system role) was denied a permission it should bypass-allow")
	}
}

func TestHasPermission_NonOwner_RequiresExplicitGrant(t *testing.T) {
	svc, tenantInfo, pool := setupRoleService(t)
	ctx := database.WithTenantInfo(context.Background(), tenantInfo)

	userID, roleID := uuid.New(), uuid.New()
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'staff@x.com', 'hash', 'Staff', true)`, userID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'staff', false)`, roleID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	allowed, err := svc.HasPermission(ctx, userID, permission.UserView)
	if err != nil {
		t.Fatalf("HasPermission: %v", err)
	}
	if allowed {
		t.Error("non-owner role with no granted permission was allowed")
	}

	var permID uuid.UUID
	if err := pool.Get(&permID, `SELECT id FROM `+tenantInfo.SchemaName+`.permissions WHERE code = $1`, permission.UserView); err != nil {
		t.Fatalf("find permission id: %v", err)
	}
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permID)

	allowed, err = svc.HasPermission(ctx, userID, permission.UserView)
	if err != nil {
		t.Fatalf("HasPermission (after grant): %v", err)
	}
	if !allowed {
		t.Error("non-owner role was denied a permission explicitly granted to it")
	}
}
```

- [ ] **Langkah 6: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/modules/tenant/role/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 7: Implementasikan service.go**

Buat `internal/modules/tenant/role/service.go`:
```go
package role

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

type Service struct {
	db *database.DB
}

func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// HasPermission implements middleware.PermissionChecker. A user holding a
// role with is_system=true (currently only "owner") is granted every
// permission via this bypass, rather than rows in role_permissions, so a
// newly added permission is automatically available to owners without a
// backfill migration.
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	var allowed bool
	err := s.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		var isSystemOwner bool
		if err := tx.Get(&isSystemOwner, `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = $1 AND r.is_system = true
			)`, userID); err != nil {
			return err
		}
		if isSystemOwner {
			allowed = true
			return nil
		}

		return tx.Get(&allowed, `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id = ur.role_id
				JOIN permissions p ON p.id = rp.permission_id
				WHERE ur.user_id = $1 AND p.code = $2
			)`, userID, permCode)
	})
	if err != nil {
		return false, apperror.Internal(err)
	}
	return allowed, nil
}
```

- [ ] **Langkah 8: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/modules/tenant/role/... -v`
Hasil: PASS — kedua test lolos.

- [ ] **Langkah 9: Commit**

```bash
git add internal/middleware internal/modules/tenant/role
git commit -m "feat: RBAC middleware with permission cache and tenant role service"
```

---

### Tugas 16: Modul auth tenant — login (dengan `tenant_code`), refresh, logout

**Catatan desain:**
- Setiap body request auth-tenant membawa `tenant_code` — termasuk refresh dan logout — karena refresh token polos tidak menyebutkan schema tenant mana tempat barisnya berada, dan belum ada token yang bisa membawa info itu (itu justru yang sedang ditetapkan oleh login). Client menyimpan `tenant_code` bersama token-nya.
- Login adalah satu-satunya tempat identitas tenant ditetapkan **tanpa** melewati `RequireTenant` lebih dulu — belum ada token — jadi service menyuntikkan `database.TenantInfo` ke `ctx` sendiri, secara manual, sebelum memanggil `WithTenant`.
- Interface lookup tenant mengembalikan `middleware.TenantRecord` (dipakai ulang, bukan tipe baru) supaya modul ini tidak perlu struct bentuk-tenant sendiri.

**Berkas:**
- Buat: `internal/modules/tenant/auth/dto.go`
- Buat: `internal/modules/tenant/auth/service.go`
- Buat: `internal/modules/tenant/auth/handler.go`
- Ubah: `internal/modules/platform/tenant/repository.go` — tambah `FindRecordByCode`
- Ubah: `internal/testsupport/testsupport.go` — tambah `SeedInSchema` yang di-export
- Test: `internal/modules/tenant/auth/service_test.go`

**Antarmuka:**
- Menghasilkan:
  - `tenantauth.LoginRequest { TenantCode, Email, Password string }`, `RefreshRequest { TenantCode, RefreshToken string }`, `LogoutRequest { TenantCode, RefreshToken string }`
  - `tenantauth.LoginResponse { AccessToken, RefreshToken string; ExpiresIn int }`
  - `tenantauth.TenantLookup interface { FindRecordByCode(ctx, code string) (middleware.TenantRecord, error) }`
  - `tenantauth.NewService(db *database.DB, tenants TenantLookup, tokens *auth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service`
  - `(s *Service) Login/Refresh/Logout` — bentuknya sama dengan versi platform di Tugas 12.
  - `tenantauth.NewHandler(svc *Service) *Handler` dengan `Login`, `Refresh`, `Logout` (disambungkan ke rute di Tugas 18).
  - `(r *tenant.Repository) FindRecordByCode(ctx, code string) (middleware.TenantRecord, error)` — mengimplementasikan `tenantauth.TenantLookup`.
  - `testsupport.SeedInSchema(t, pool *sqlx.DB, schema, query string, args ...any)`

- [ ] **Langkah 1: Tambah helper seed bersama ke testsupport**

Ubah `internal/testsupport/testsupport.go` — tambahkan:
```go
// SeedInSchema runs a fixture-setup query with search_path pinned to
// schema, for tests that need to insert rows directly rather than going
// through database.WithTenant themselves.
func SeedInSchema(t *testing.T, pool *sqlx.DB, schema, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SET LOCAL search_path TO "`+schema+`"`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(query, args...); err != nil {
		t.Fatalf("seed query: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
```

- [ ] **Langkah 2: Tambah `FindRecordByCode` ke repository tenant platform**

Ubah `internal/modules/platform/tenant/repository.go` — tambahkan method ini, memakai ulang `FindByCode` dan lookup versi migrasi yang sama seperti `FindByID`:
```go
// FindRecordByCode is FindByCode plus a migration-version check, shaped as
// middleware.TenantRecord — the tenant/auth module's login flow (Task 16)
// uses this to resolve a tenant_code to enough info to attempt a login.
func (r *Repository) FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error) {
	t, err := r.FindByCode(ctx, code)
	if err != nil {
		return middleware.TenantRecord{}, err
	}
	version, dirty, err := migration.TenantSchemaVersion(r.rawDB, t.SchemaName)
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}
	return middleware.TenantRecord{
		TenantID: t.ID, SchemaName: t.SchemaName, Status: t.Status,
		SchemaVersion: version, SchemaDirty: dirty,
	}, nil
}
```

- [ ] **Langkah 3: Jalankan test yang ada, pastikan tidak ada yang rusak**

Jalankan: `go test ./internal/modules/platform/tenant/... -v`
Hasil: PASS — kelima test dari Tugas 14 tetap lolos; ini perubahan yang sifatnya menambah saja.

- [ ] **Langkah 4: Tulis test service yang gagal**

Buat `internal/modules/tenant/auth/service_test.go`:
```go
package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

type fakeTenantLookup struct {
	record middleware.TenantRecord
}

func (f *fakeTenantLookup) FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error) {
	return f.record, nil
}

func setupTenantAuthService(t *testing.T) (*tenantauth.Service, string) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	plainPassword := "correct-horse-battery"
	hash, err := appauth.HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	testsupport.SeedInSchema(t, pool, schema,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'staff@example.com', $2, 'Staff', true)`,
		uuid.New(), hash)

	db := database.NewDB(pool)
	platformPool := testsupport.OpenTestPlatformDB(t)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, 5, 15*time.Minute)
	lookup := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: uuid.New(), SchemaName: schema, Status: "active"}}

	tokens := appauth.NewTokenManager("test-secret", 15*time.Minute)
	svc := tenantauth.NewService(db, lookup, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	return svc, plainPassword
}

func TestLogin_ValidCredentials_ReturnsTokens(t *testing.T) {
	svc, password := setupTenantAuthService(t)

	res, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("Login() returned an empty token")
	}
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc, _ := setupTenantAuthService(t)

	if _, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: "wrong",
	}); err == nil {
		t.Fatal("Login() with a wrong password succeeded")
	}
}

func TestRefresh_ValidToken_IssuesNewAccessToken(t *testing.T) {
	svc, password := setupTenantAuthService(t)
	login, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	res, err := svc.Refresh(context.Background(), tenantauth.RefreshRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, password := setupTenantAuthService(t)
	login, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(context.Background(), tenantauth.LogoutRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), tenantauth.RefreshRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	}); err == nil {
		t.Error("Refresh() succeeded with a revoked refresh token")
	}
}
```

- [ ] **Langkah 5: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/modules/tenant/auth/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 6: Implementasikan dto.go**

Buat `internal/modules/tenant/auth/dto.go`:
```go
package auth

type LoginRequest struct {
	TenantCode string `json:"tenant_code" validate:"required"`
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequest struct {
	TenantCode   string `json:"tenant_code" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	TenantCode   string `json:"tenant_code" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}
```

- [ ] **Langkah 7: Implementasikan service.go**

Buat `internal/modules/tenant/auth/service.go`:
```go
package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/ratelimit"
)

// TenantLookup resolves a tenant_code to enough info to attempt a login.
// Declared here, consumer-side, and implemented by
// modules/platform/tenant.Repository (Task 14).
type TenantLookup interface {
	FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error)
}

type tenantUser struct {
	ID           uuid.UUID `db:"id"`
	PasswordHash string    `db:"password_hash"`
	IsActive     bool      `db:"is_active"`
}

type Service struct {
	db          *database.DB
	tenants     TenantLookup
	tokens      *appauth.TokenManager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rateLimiter *ratelimit.LoginAttemptService
}

func NewService(db *database.DB, tenants TenantLookup, tokens *appauth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service {
	return &Service{db: db, tenants: tenants, tokens: tokens, accessTTL: accessTTL, refreshTTL: refreshTTL, rateLimiter: rateLimiter}
}

// dummyPasswordHash adalah hash bcrypt yang valid dari password acak yang
// tidak pernah dipakai. Login selalu memanggil VerifyPassword tepat satu
// kali — terhadap hash user asli jika ditemukan dan aktif, atau terhadap
// hash tetap ini jika tidak — sehingga durasi respons tidak pernah
// membocorkan apakah sebuah email ada/aktif. Pola dan alasan yang sama
// dengan service auth admin platform (Tugas 12), diduplikasi di sini
// karena package-nya berbeda.
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := appauth.HashPassword("not-a-real-password-used-only-for-constant-time-comparison")
	if err != nil {
		panic(err)
	}
	return hash
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil || tenantRec.Status != "active" {
		return LoginResponse{}, apperror.Unauthorized("tenant_code, email, atau password salah")
	}

	if err := s.rateLimiter.Check(ctx, appauth.ScopeTenant, &tenantRec.TenantID, req.Email); err != nil {
		return LoginResponse{}, err
	}

	// Login is the one place tenant identity is established without a
	// prior RequireTenant pass — there's no token yet — so TenantInfo is
	// injected into ctx by hand before using WithTenant.
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	var u tenantUser
	found := false
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		getErr := tx.Get(&u, `SELECT id, password_hash, is_active FROM users WHERE email = $1 AND deleted_at IS NULL`, req.Email)
		if getErr == nil {
			found = true
		}
		return nil // a missing user isn't a transaction failure — just an invalid login
	})
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	// Amandemen (ditambahkan bersamaan dengan perbaikan Tugas 12 yang
	// identik, dengan alasan yang sama): selalu panggil VerifyPassword —
	// terhadap hash asli jika ditemukan dan aktif, terhadap
	// dummyPasswordHash tetap jika tidak — sehingga biaya baris ini sama
	// di setiap jalur. Tanpa ini, short-circuit && melewati pemanggilan
	// bcrypt setiap kali user tidak ditemukan/tidak aktif, membiarkan
	// durasi respons membocorkan email mana yang ada.
	activeFound := found && u.IsActive
	hashToCheck := dummyPasswordHash
	if activeFound {
		hashToCheck = u.PasswordHash
	}
	passwordOK := appauth.VerifyPassword(hashToCheck, req.Password)
	valid := activeFound && passwordOK
	s.rateLimiter.Record(ctx, appauth.ScopeTenant, &tenantRec.TenantID, req.Email, valid)
	if !valid {
		return LoginResponse{}, apperror.Unauthorized("tenant_code, email, atau password salah")
	}

	return s.issueTokens(tenantCtx, tenantRec.TenantID, u.ID)
}

func (s *Service) issueTokens(tenantCtx context.Context, tenantID, userID uuid.UUID) (LoginResponse, error) {
	access, err := s.tokens.IssueAccessToken(userID, appauth.ScopeTenant, &tenantID)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	plain, hash, err := appauth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		_, execErr := tx.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			uuid.New(), userID, hash, time.Now().Add(s.refreshTTL))
		return execErr
	})
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	return LoginResponse{AccessToken: access, RefreshToken: plain, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (LoginResponse, error) {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil || tenantRec.Status != "active" {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	hash := appauth.HashRefreshToken(req.RefreshToken)
	var row struct {
		UserID    uuid.UUID  `db:"user_id"`
		ExpiresAt time.Time  `db:"expires_at"`
		RevokedAt *time.Time `db:"revoked_at"`
	}
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		return tx.Get(&row, `SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash)
	})
	if err != nil {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	if row.RevokedAt != nil || time.Now().After(row.ExpiresAt) {
		return LoginResponse{}, apperror.Unauthorized("refresh token sudah tidak berlaku")
	}

	access, err := s.tokens.IssueAccessToken(row.UserID, appauth.ScopeTenant, &tenantRec.TenantID)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	return LoginResponse{AccessToken: access, RefreshToken: req.RefreshToken, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil {
		return apperror.Unauthorized("tenant tidak ditemukan")
	}
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	hash := appauth.HashRefreshToken(req.RefreshToken)
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		_, execErr := tx.Exec(`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
		return execErr
	})
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
```

- [ ] **Langkah 8: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/modules/tenant/auth/... -v`
Hasil: PASS — keempat test lolos.

- [ ] **Langkah 9: Implementasikan handler**

Buat `internal/modules/tenant/auth/handler.go`:
```go
package auth

import (
	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/response"
	"go-api-starter/internal/validator"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func badBody(c *fiber.Ctx) error {
	return response.Error(c, middleware.RequestIDFromCtx(c),
		apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	res, err := h.svc.Login(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	res, err := h.svc.Refresh(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, res)
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return badBody(c)
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}
	if err := h.svc.Logout(c.UserContext(), req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil logout"})
}
```

- [ ] **Langkah 10: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 11: Commit**

```bash
git add internal/modules/tenant/auth internal/modules/platform/tenant/repository.go internal/testsupport
git commit -m "feat: tenant auth (login/refresh/logout) with tenant_code resolution"
```

---

### Tugas 17: Modul tenant `user` — pola contoh

Ini modul yang disalin setiap modul tenant-scoped di masa depan (`product`, `order`, apa pun yang dibutuhkan project yang lahir dari starter ini). Setiap konvensi yang ditetapkan starter ini didemonstrasikan di sini: `WithTenant` untuk setiap query, soft delete, kolom audit `*_by` dari `database.ActorFromContext`, whitelist sort pagination, dan bentuk handler → service → repository.

**Berkas:**
- Buat: `internal/modules/tenant/user/model.go`
- Buat: `internal/modules/tenant/user/dto.go`
- Buat: `internal/modules/tenant/user/repository.go`
- Buat: `internal/modules/tenant/user/service.go`
- Buat: `internal/modules/tenant/user/handler.go`
- Test: `internal/modules/tenant/user/repository_test.go`

**Antarmuka:**
- Menghasilkan:
  - `user.User` (struct baris DB), `user.CreateRequest`, `user.UpdateRequest`, `user.View`
  - `user.Sortable map[string]string` — template whitelist untuk setiap modul di masa depan.
  - `user.NewRepository(db *database.DB) *Repository` dengan `Create/FindByID/List/Update/Delete`
  - `user.NewService(repo *Repository) *Service` dengan `Create/Get/List/Update/Delete`
  - `user.NewHandler(svc *Service) *Handler` dengan `Create/Get/List/Update/Delete` (`func(c *fiber.Ctx) error`), disambungkan ke rute di Tugas 18 di belakang `RequireTenant` + `RequirePermission`.

- [ ] **Langkah 1: Tulis test repository yang gagal**

Buat `internal/modules/tenant/user/repository_test.go`:
```go
package user_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/testsupport"
)

func setupUserRepo(t *testing.T) (*tenantuser.Repository, context.Context) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	db := database.NewDB(pool)
	ctx := database.WithTenantInfo(context.Background(), database.TenantInfo{TenantID: uuid.New(), SchemaName: schema})
	ctx = database.WithActor(ctx, database.Actor{UserID: uuid.New(), Scope: "tenant"})
	return tenantuser.NewRepository(db), ctx
}

func TestCreateAndFindByID(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	u := tenantuser.User{ID: uuid.New(), Email: "a@example.com", PasswordHash: "hash", Name: "A", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Email != "a@example.com" {
		t.Errorf("Email = %q, want a@example.com", got.Email)
	}
}

func TestCreate_DuplicateEmail_ReturnsConflict(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	u1 := tenantuser.User{ID: uuid.New(), Email: "dup@example.com", PasswordHash: "hash", Name: "A", IsActive: true}
	if err := repo.Create(ctx, u1); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	u2 := tenantuser.User{ID: uuid.New(), Email: "dup@example.com", PasswordHash: "hash", Name: "B", IsActive: true}
	if err := repo.Create(ctx, u2); err == nil {
		t.Fatal("second Create with duplicate email succeeded, want conflict")
	}
}

func TestFindByID_NotFound(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	if _, err := repo.FindByID(ctx, uuid.New()); err == nil {
		t.Fatal("FindByID() for a nonexistent id succeeded, want not-found error")
	}
}

func TestUpdate_ChangesNameAndIsActive(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	u := tenantuser.User{ID: uuid.New(), Email: "u@example.com", PasswordHash: "hash", Name: "Old", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "New"
	inactive := false
	if err := repo.Update(ctx, u.ID, &newName, &inactive); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "New" || got.IsActive != false {
		t.Errorf("got Name=%q IsActive=%v, want Name=New IsActive=false", got.Name, got.IsActive)
	}
}

func TestDelete_SoftDeletesAndHidesFromFindAndList(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	u := tenantuser.User{ID: uuid.New(), Email: "gone@example.com", PasswordHash: "hash", Name: "Gone", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.FindByID(ctx, u.ID); err == nil {
		t.Error("FindByID() found a soft-deleted user")
	}

	users, total, err := repo.List(ctx, 20, 0, "created_at DESC")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, lu := range users {
		if lu.ID == u.ID {
			t.Error("List() returned a soft-deleted user")
		}
	}
	if total != 0 {
		t.Errorf("total = %d, want 0 (the only user was soft-deleted)", total)
	}
}

func TestList_RespectsLimitAndOrder(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	for i := 0; i < 3; i++ {
		u := tenantuser.User{
			ID: uuid.New(), Email: uuid.New().String() + "@example.com",
			PasswordHash: "hash", Name: "User", IsActive: true,
		}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	users, total, err := repo.List(ctx, 2, 0, "created_at DESC")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2 (limit)", len(users))
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/modules/tenant/user/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan model.go dan dto.go**

Buat `internal/modules/tenant/user/model.go`:
```go
package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `db:"id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	Name         string     `db:"name"`
	IsActive     bool       `db:"is_active"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}
```

Buat `internal/modules/tenant/user/dto.go`:
```go
package user

type CreateRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required"`
}

type UpdateRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

type View struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

func toView(u User) View {
	return View{ID: u.ID.String(), Email: u.Email, Name: u.Name, IsActive: u.IsActive}
}
```

- [ ] **Langkah 4: Implementasikan repository.go**

Buat `internal/modules/tenant/user/repository.go`:
```go
package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// Sortable is the whitelist of columns the list endpoint may sort by,
// consumed through pagination.Params.OrderByClause — never the raw query
// value. Copy this pattern for every new tenant-scoped module.
var Sortable = map[string]string{
	"created_at": "created_at",
	"name":       "name",
	"email":      "email",
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func wrapDBErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.NotFound("user tidak ditemukan")
	}
	return apperror.Internal(err)
}

func (r *Repository) Create(ctx context.Context, u User) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
			u.ID, u.Email, u.PasswordHash, u.Name, u.IsActive, actor.UserID)
		return err
	})
	if err != nil {
		// A duplicate email is by far the most likely failure here, and
		// the unique partial index makes it unambiguous — reported as a
		// conflict rather than a generic 500.
		return apperror.Conflict("email sudah dipakai")
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	})
	if err != nil {
		return User{}, wrapDBErr(err)
	}
	return u, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int, orderBy string) ([]User, int, error) {
	var users []User
	var total int
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		query := `SELECT * FROM users WHERE deleted_at IS NULL ORDER BY ` + orderBy + ` LIMIT $1 OFFSET $2`
		if err := tx.Select(&users, query, limit, offset); err != nil {
			return err
		}
		return tx.Get(&total, `SELECT count(*) FROM users WHERE deleted_at IS NULL`)
	})
	if err != nil {
		return nil, 0, wrapDBErr(err)
	}
	return users, total, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, name *string, isActive *bool) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.Exec(
			`UPDATE users SET name = COALESCE($2, name), is_active = COALESCE($3, is_active), updated_by = $4
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, name, isActive, actor.UserID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperror.NotFound("user tidak ditemukan")
		}
		return nil
	})
	return wrapDBErr(err)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.Exec(
			`UPDATE users SET deleted_at = now(), deleted_by = $2 WHERE id = $1 AND deleted_at IS NULL`,
			id, actor.UserID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperror.NotFound("user tidak ditemukan")
		}
		return nil
	})
	return wrapDBErr(err)
}
```

- [ ] **Langkah 5: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/modules/tenant/user/... -v`
Hasil: PASS — keenam test lolos.

- [ ] **Langkah 6: Implementasikan service.go**

Buat `internal/modules/tenant/user/service.go`:
```go
package user

import (
	"context"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/response"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (View, error) {
	if err := appauth.ValidatePasswordStrength(req.Password); err != nil {
		return View{}, err
	}
	hash, err := appauth.HashPassword(req.Password)
	if err != nil {
		return View{}, err
	}
	u := User{ID: uuid.New(), Email: req.Email, PasswordHash: hash, Name: req.Name, IsActive: true}
	if err := s.repo.Create(ctx, u); err != nil {
		return View{}, err
	}
	return toView(u), nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (View, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return View{}, err
	}
	return toView(u), nil
}

func (s *Service) List(ctx context.Context, p pagination.Params) ([]View, response.Meta, error) {
	users, total, err := s.repo.List(ctx, p.Limit, p.Offset(), p.OrderByClause(Sortable))
	if err != nil {
		return nil, response.Meta{}, err
	}
	views := make([]View, len(users))
	for i, u := range users {
		views[i] = toView(u)
	}
	return views, pagination.BuildMeta(p.Page, p.Limit, total), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) error {
	return s.repo.Update(ctx, id, req.Name, req.IsActive)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Langkah 7: Implementasikan handler.go**

Buat `internal/modules/tenant/user/handler.go`:
```go
package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/response"
	"go-api-starter/internal/validator"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c),
			apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
	}
	if verr := validator.Validate(req); verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	view, err := h.svc.Create(c.UserContext(), req)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 201, view)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	view, err := h.svc.Get(c.UserContext(), id)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, view)
}

func (h *Handler) List(c *fiber.Ctx) error {
	params, verr := pagination.Parse(c, Sortable, "created_at")
	if verr != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), verr)
	}

	views, meta, err := h.svc.List(c.UserContext(), params)
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.SuccessList(c, 200, views, meta)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c),
			apperror.Validation(map[string][]string{"_": {"badan permintaan tidak valid"}}))
	}

	if err := h.svc.Update(c.UserContext(), id, req); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil diperbarui"})
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), apperror.NotFound("user tidak ditemukan"))
	}

	if err := h.svc.Delete(c.UserContext(), id); err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, fiber.Map{"message": "berhasil dihapus"})
}
```

- [ ] **Langkah 8: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 9: Commit**

```bash
git add internal/modules/tenant/user
git commit -m "feat: tenant user module — the example CRUD pattern for future modules"
```

---

### Tugas 18: Wiring server HTTP — router, health check, graceful shutdown, `main.go`

Test perilaku rute secara penuh (auth → RBAC → isolasi tenant, end-to-end lewat HTTP sungguhan) sengaja ditinggalkan untuk integration test Tugas 20, yang menjalankan router yang persis sama ini. Test milik tugas ini sendiri hanya mencakup endpoint health, yang cukup sederhana untuk diverifikasi terisolasi; sisanya diverifikasi lewat build dan smoke test manual di Langkah 5.

**Berkas:**
- Buat: `internal/server/router.go`
- Buat: `internal/server/health.go`
- Buat: `cmd/api/main.go`
- Test: `internal/server/health_test.go`

**Antarmuka:**
- Menghasilkan:
  - Struct `server.Dependencies` — setiap handler dan cache yang dibutuhkan `NewRouter`.
  - `server.NewRouter(deps Dependencies, corsOrigins []string) *fiber.App`
  - `server.RegisterHealth(app *fiber.App, pool *sqlx.DB)`
  - Rute: `POST /api/v1/admin/auth/{login,refresh,logout}`, `POST /api/v1/auth/{login,refresh,logout}`, `POST|GET /api/v1/users`, `GET|PATCH|DELETE /api/v1/users/:id` (kelima terakhir di belakang `RequireTenant` + `RequirePermission`).

- [ ] **Langkah 1: Tulis test health yang gagal**

Buat `internal/server/health_test.go`:
```go
package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/server"
	"go-api-starter/internal/testsupport"
)

func TestHealth_AlwaysOK(t *testing.T) {
	app := fiber.New()
	pool := testsupport.OpenTestDB(t)
	server.RegisterHealth(app, pool)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHealthReady_OKWhenDBReachable(t *testing.T) {
	app := fiber.New()
	pool := testsupport.OpenTestDB(t)
	server.RegisterHealth(app, pool)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
```

- [ ] **Langkah 2: Jalankan test, pastikan gagal**

Jalankan: `go test ./internal/server/... -v`
Hasil: FAIL — paket tidak compile.

- [ ] **Langkah 3: Implementasikan health.go**

Buat `internal/server/health.go`:
```go
package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)

// RegisterHealth attaches liveness and readiness endpoints. /health never
// touches the database — it only proves the process is alive and
// responding. /health/ready additionally pings the database, so a load
// balancer or orchestrator can tell "running" apart from "actually able to
// serve requests."
func RegisterHealth(app *fiber.App, pool *sqlx.DB) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		if err := pool.PingContext(c.Context()); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not ready"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
}
```

- [ ] **Langkah 4: Jalankan test, pastikan lolos**

Jalankan: `go test ./internal/server/... -v`
Hasil: PASS — kedua test lolos.

- [ ] **Langkah 5: Implementasikan router.go**

Buat `internal/server/router.go`:
```go
package server

import (
	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/middleware"
	platformauth "go-api-starter/internal/modules/platform/auth"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/response"
)

// Dependencies collects every constructed handler and cache NewRouter
// needs. Built once in cmd/api/main.go, kept here as a single struct so
// main.go's job stays "construct dependencies, hand them to NewRouter"
// rather than router logic and dependency construction tangled together.
type Dependencies struct {
	Tokens          *auth.TokenManager
	TenantResolver  *middleware.TenantResolver
	PermissionCache *middleware.PermissionCache
	PlatformAuth    *platformauth.Handler
	TenantAuth      *tenantauth.Handler
	TenantUser      *tenantuser.Handler
}

// NewRouter builds the full Fiber app: global middleware, then every route
// under /api/v1. Health endpoints are registered separately via
// RegisterHealth (main.go calls both).
func NewRouter(deps Dependencies, corsOrigins []string) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return response.Error(c, middleware.RequestIDFromCtx(c), err)
		},
	})

	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.Recover())
	app.Use(middleware.CORS(corsOrigins))

	api := app.Group("/api/v1")

	admin := api.Group("/admin")
	admin.Post("/auth/login", deps.PlatformAuth.Login)
	admin.Post("/auth/refresh", deps.PlatformAuth.Refresh)
	admin.Post("/auth/logout", deps.PlatformAuth.Logout)

	api.Post("/auth/login", deps.TenantAuth.Login)
	api.Post("/auth/refresh", deps.TenantAuth.Refresh)
	api.Post("/auth/logout", deps.TenantAuth.Logout)

	tenantAPI := api.Group("", middleware.RequireTenant(deps.Tokens, deps.TenantResolver))

	users := tenantAPI.Group("/users")
	users.Post("/", middleware.RequirePermission(permission.UserCreate, deps.PermissionCache), deps.TenantUser.Create)
	users.Get("/", middleware.RequirePermission(permission.UserView, deps.PermissionCache), deps.TenantUser.List)
	users.Get("/:id", middleware.RequirePermission(permission.UserView, deps.PermissionCache), deps.TenantUser.Get)
	users.Patch("/:id", middleware.RequirePermission(permission.UserUpdate, deps.PermissionCache), deps.TenantUser.Update)
	users.Delete("/:id", middleware.RequirePermission(permission.UserDelete, deps.PermissionCache), deps.TenantUser.Delete)

	return app
}
```

- [ ] **Langkah 6: Implementasikan main.go**

Buat `cmd/api/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	platformauth "go-api-starter/internal/modules/platform/auth"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantrole "go-api-starter/internal/modules/tenant/role"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(cfg.DB)
	if err != nil {
		slog.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	platformPool, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		slog.Error("platform db pool", "error", err)
		os.Exit(1)
	}
	defer platformPool.Close()

	db := database.NewDB(pool)
	tokens := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, cfg.Login.MaxAttempts, cfg.Login.AttemptWindow)

	// tenantRepo satisfies both middleware.TenantLookup (FindByID) and
	// tenantauth.TenantLookup (FindRecordByCode) — one repository, two
	// different lookup shapes for two different callers.
	tenantRepo := platformtenant.NewRepository(platformPool, pool.DB)
	tenantResolver := middleware.NewTenantResolver(tenantRepo, time.Minute)

	roleSvc := tenantrole.NewService(db)
	permCache := middleware.NewPermissionCache(roleSvc, time.Minute)

	platformAuthSvc := platformauth.NewService(platformPool, tokens, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, rateLimiter)
	tenantAuthSvc := tenantauth.NewService(db, tenantRepo, tokens, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, rateLimiter)

	userSvc := tenantuser.NewService(tenantuser.NewRepository(db))

	deps := server.Dependencies{
		Tokens:          tokens,
		TenantResolver:  tenantResolver,
		PermissionCache: permCache,
		PlatformAuth:    platformauth.NewHandler(platformAuthSvc),
		TenantAuth:      tenantauth.NewHandler(tenantAuthSvc),
		TenantUser:      tenantuser.NewHandler(userSvc),
	}
	app := server.NewRouter(deps, cfg.CORS.AllowedOrigins)
	server.RegisterHealth(app, pool)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.App.Port)
		slog.Info("listening", "addr", addr)
		if err := app.Listen(addr); err != nil {
			slog.Error("server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
```

- [ ] **Langkah 7: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 8: Smoke-test manual seluruh stack**

Jalankan:
```bash
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acmecorp.test
go run ./cmd/api
```
Di terminal lain:
```bash
curl http://localhost:8080/health
curl http://localhost:8080/health/ready
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"tenant_code":"acme_corp","email":"owner@acmecorp.test","password":"<password owner dari tenant create>"}'
```
Hasil: `/health` dan `/health/ready` mengembalikan `{"status":"ok"}` / `{"status":"ready"}`; login mengembalikan `access_token`, `refresh_token`, `expires_in`. Salin `access_token` dan pastikan RBAC bekerja — owner (lewat bypass `is_system`) bisa melihat daftar user:
```bash
curl http://localhost:8080/api/v1/users -H "Authorization: Bearer <access_token>"
```
Hasil: `{"success":true,"data":[...],"meta":{...}}` — minimal berisi user owner yang dibuat saat provisioning.

- [ ] **Langkah 9: Commit**

```bash
git add internal/server cmd/api
git commit -m "feat: HTTP server wiring — router, health checks, graceful shutdown"
```

---

### Tugas 19: Perintah `cli cleanup`

Menghapus refresh token yang kedaluwarsa/dicabut (platform dan setiap schema tenant) dan baris `login_attempts` yang lama (khusus platform — schema tenant tidak pernah punya tabel `login_attempts` sendiri; spec menjaga riwayat rate-limit di satu tempat saja, tidak peduli scope-nya). Dimaksudkan untuk dijalankan terjadwal lewat systemd timer (disambungkan di Tugas 21). Jendela pengaman 30-hari milik tenant purge sudah dibangun di `Service.Purge` (Tugas 14) — tidak ada yang perlu ditambahkan di sana.

Tidak ada test otomatis untuk berkas ini, konsisten dengan berkas perintah CLI lain di rencana ini (`migrate.go`, `admin.go`, `tenant.go`, `permission.go`) — kebenarannya bergantung pada service di baliknya yang sudah teruji; tugas ini diverifikasi manual.

**Berkas:**
- Buat: `cmd/cli/commands/cleanup.go`

**Antarmuka:**
- Menghasilkan: perintah CLI `cli cleanup`

- [ ] **Langkah 1: Implementasikan cleanup.go**

Buat `cmd/cli/commands/cleanup.go`:
```go
package commands

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
)

func init() {
	Register("cleanup", cmdCleanup)
}

const loginAttemptRetention = 30 * 24 * time.Hour

func cmdCleanup(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`DELETE FROM platform.refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < now()`); err != nil {
		return fmt.Errorf("clean platform refresh_tokens: %w", err)
	}
	cutoff := time.Now().Add(-loginAttemptRetention)
	if _, err := db.Exec(`DELETE FROM platform.login_attempts WHERE attempted_at < $1`, cutoff); err != nil {
		return fmt.Errorf("clean platform login_attempts: %w", err)
	}
	fmt.Println("platform: dibersihkan")

	targets, err := tenantSchemasToMigrate(db, true, "") // shared helper from migrate.go
	if err != nil {
		return err
	}
	for _, tgt := range targets {
		if err := cleanupTenantSchema(db, tgt.schemaName); err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s dibersihkan\n", tgt.code)
	}
	return nil
}

func cleanupTenantSchema(db *sql.DB, schemaName string) error {
	if !database.ValidSchemaName(schemaName) {
		return fmt.Errorf("invalid schema name %q", schemaName)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SET LOCAL search_path TO "` + schemaName + `"`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < now()`); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Langkah 2: Pastikan bisa di-build**

Jalankan: `go build ./...`
Hasil: berhasil.

- [ ] **Langkah 3: Verifikasi manual**

Jalankan:
```bash
go run ./cmd/cli cleanup
```
Hasil: mencetak `platform: dibersihkan` diikuti satu baris per tenant. Menjalankannya terhadap tenant yang baru saja di-provisioning dan belum punya token kedaluwarsa itu no-op — itu memang wajar, bukan bug.

- [ ] **Langkah 4: Commit**

```bash
git add cmd/cli/commands/cleanup.go
git commit -m "feat: cli cleanup command for expired refresh tokens and login attempts"
```

---

### Tugas 20: Integration test — isolasi tenant, end-to-end lewat HTTP sungguhan

Test yang oleh spec disebut paling penting di repo ini: dua tenant dengan data **yang sama** (alamat email yang sama, sengaja) dipastikan tidak pernah saling mencemari — di-provisioning sungguhan, login sungguhan, dijalankan sepenuhnya lewat router HTTP sungguhan yang dibangun Tugas 18. Tugas 5 sudah membuktikan `WithTenant` sendiri tidak bisa membocorkan `search_path` lintas rollback di layer database; test ini membuktikan jaminan yang sama berlaku lewat seluruh stack yang benar-benar diajak bicara client sungguhan.

**Berkas:**
- Test: `internal/server/tenant_isolation_test.go`

**Antarmuka:**
- Menggunakan semua yang dibangun di Tugas 5–18: tidak ada kode produksi baru, tugas ini murni test.

- [ ] **Langkah 1: Tulis test-nya**

Buat `internal/server/tenant_isolation_test.go`:
```go
package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	platformauth "go-api-starter/internal/modules/platform/auth"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantrole "go-api-starter/internal/modules/tenant/role"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/server"
	"go-api-starter/internal/testsupport"
)

func buildTestApp(t *testing.T) (*fiber.App, *platformtenant.Service) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(pool.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec("TRUNCATE platform.tenants, platform.login_attempts CASCADE")
	})

	platformPool := testsupport.OpenTestPlatformDB(t)
	db := database.NewDB(pool)
	tokens := auth.NewTokenManager("integration-test-secret", 15*time.Minute)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, 5, 15*time.Minute)

	tenantRepo := platformtenant.NewRepository(platformPool, pool.DB)
	tenantResolver := middleware.NewTenantResolver(tenantRepo, time.Minute)
	tenantSvc := platformtenant.NewService(tenantRepo, pool.DB, tenantResolver)

	roleSvc := tenantrole.NewService(db)
	permCache := middleware.NewPermissionCache(roleSvc, time.Minute)

	platformAuthSvc := platformauth.NewService(platformPool, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	tenantAuthSvc := tenantauth.NewService(db, tenantRepo, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	userSvc := tenantuser.NewService(tenantuser.NewRepository(db))

	deps := server.Dependencies{
		Tokens:          tokens,
		TenantResolver:  tenantResolver,
		PermissionCache: permCache,
		PlatformAuth:    platformauth.NewHandler(platformAuthSvc),
		TenantAuth:      tenantauth.NewHandler(tenantAuthSvc),
		TenantUser:      tenantuser.NewHandler(userSvc),
	}
	app := server.NewRouter(deps, []string{"http://localhost:5173"})
	return app, tenantSvc
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	reader := bytes.NewReader(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1) // -1: no timeout — these steps hit real Postgres
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, path, err)
	}
	var parsed map[string]any
	json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

func loginAsOwner(t *testing.T, app *fiber.App, tenantCode, email, password string) string {
	t.Helper()
	resp, body := doJSON(t, app, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenant_code": tenantCode, "email": email, "password": password,
	}, "")
	if resp.StatusCode != 200 {
		t.Fatalf("login for %s failed: status=%d body=%v", tenantCode, resp.StatusCode, body)
	}
	data := body["data"].(map[string]any)
	return data["access_token"].(string)
}

func TestTenantIsolation_AcrossTwoTenantsWithSimilarData(t *testing.T) {
	app, tenantSvc := buildTestApp(t)
	ctx := context.Background()

	codeA := "acme_" + testsupport.RandomSchemaName()
	codeB := "globex_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropTestSchema(t, codeA) })
	t.Cleanup(func() { dropTestSchema(t, codeB) })

	resA, err := tenantSvc.Provision(ctx, platformtenant.ProvisionInput{
		Code: codeA, Name: "Acme Corp", OwnerEmail: "owner@acme.test",
	})
	if err != nil {
		t.Fatalf("Provision tenant A: %v", err)
	}
	resB, err := tenantSvc.Provision(ctx, platformtenant.ProvisionInput{
		Code: codeB, Name: "Globex Corp", OwnerEmail: "owner@globex.test",
	})
	if err != nil {
		t.Fatalf("Provision tenant B: %v", err)
	}

	tokenA := loginAsOwner(t, app, codeA, "owner@acme.test", resA.OwnerPassword)
	tokenB := loginAsOwner(t, app, codeB, "owner@globex.test", resB.OwnerPassword)

	// Same email on purpose — if isolation ever breaks, a unique-index
	// collision or cross-visibility here is exactly how it would surface.
	respA, bodyA := doJSON(t, app, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "staff@example.com", "password": "correct-horse-battery", "name": "Staff A",
	}, tokenA)
	if respA.StatusCode != 201 {
		t.Fatalf("create user in tenant A failed: status=%d body=%v", respA.StatusCode, bodyA)
	}
	staffAID := bodyA["data"].(map[string]any)["id"].(string)

	respB, bodyB := doJSON(t, app, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "staff@example.com", "password": "correct-horse-battery", "name": "Staff B",
	}, tokenB)
	if respB.StatusCode != 201 {
		t.Fatalf("create user in tenant B (same email, different tenant) failed: status=%d body=%v", respB.StatusCode, bodyB)
	}

	_, listA := doJSON(t, app, http.MethodGet, "/api/v1/users?limit=100", nil, tokenA)
	assertNamesPresentAbsent(t, listA, []string{"Staff A"}, []string{"Staff B"})

	_, listB := doJSON(t, app, http.MethodGet, "/api/v1/users?limit=100", nil, tokenB)
	assertNamesPresentAbsent(t, listB, []string{"Staff B"}, []string{"Staff A"})

	// The strongest check: tenant B, holding tenant A's own user ID,
	// cannot fetch it — WithTenant scopes every query to the caller's
	// schema regardless of which ID is asked for.
	respCross, _ := doJSON(t, app, http.MethodGet, "/api/v1/users/"+staffAID, nil, tokenB)
	if respCross.StatusCode != 404 {
		t.Errorf("tenant B fetched tenant A's user by ID: status=%d, want 404", respCross.StatusCode)
	}
}

func assertNamesPresentAbsent(t *testing.T, body map[string]any, present, absent []string) {
	t.Helper()
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response has no data array: %v", body)
	}
	names := map[string]bool{}
	for _, item := range data {
		row := item.(map[string]any)
		names[row["name"].(string)] = true
	}
	for _, want := range present {
		if !names[want] {
			t.Errorf("expected %q in list, got names: %v", want, names)
		}
	}
	for _, unwanted := range absent {
		if names[unwanted] {
			t.Errorf("found %q in list — cross-tenant leak, got names: %v", unwanted, names)
		}
	}
}

func dropTestSchema(t *testing.T, schemaName string) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	pool.Exec(`DROP SCHEMA IF EXISTS "` + schemaName + `" CASCADE`)
}
```

- [ ] **Langkah 2: Jalankan test-nya**

Jalankan: `go test ./internal/server/... -run TenantIsolation -v`
Hasil: PASS. Test ini lambat (dua kali provisioning penuh, masing-masing menjalankan empat migrasi tenant) — beberapa detik itu wajar.

Kalau gagal, **berhenti dan perlakukan sebagai bug berprioritas tertinggi di seluruh codebase** — ini satu-satunya jaminan yang jadi alasan seluruh arsitektur ini ada.

- [ ] **Langkah 3: Jalankan seluruh test suite**

Jalankan: `go test ./... -v`
Hasil: setiap test yang ditulis di Tugas 2–20 lolos.

- [ ] **Langkah 4: Commit**

```bash
git add internal/server/tenant_isolation_test.go
git commit -m "test: end-to-end tenant isolation across two tenants with identical data"
```

---

### Tugas 21: Artefak deployment, `CLAUDE.md`, dan dokumentasi

Dokumentasi ditulis sekarang, sekali, terhadap kode yang sudah selesai — bukan lebih awal dan bertahap mengejar target yang terus bergerak. Ini pilihan sengaja: aturan spec adalah "dokumentasi harus selalu cocok dengan kode," dan menulisnya setelah Tugas 20 adalah satu-satunya titik di rencana ini di mana itu benar secara konstruksi. (`README.md` tetap di luar cakupan sesuai spec — ditulis setelah starter ini dipakai membangun sesuatu yang nyata.)

Berkas dokumentasi (`CLAUDE.md`, `docs/*.md`) di sini ditulis dalam Bahasa Indonesia — konsisten dengan konvensi bahasa di Batasan Global: ini murni teks yang dibaca manusia, tidak ada identifier Go di dalamnya, jadi tidak ada alasan menulisnya dalam Bahasa Inggris.

**Berkas:**
- Buat: `deploy/app-api.service`
- Buat: `deploy/app-cleanup.service`
- Buat: `deploy/app-cleanup.timer`
- Buat: `deploy/pg-backup.service`
- Buat: `deploy/pg-backup.timer`
- Buat: `deploy/pg-backup.sh`
- Buat: `deploy/Caddyfile`
- Buat: `deploy/deploy.sh`
- Buat: `CLAUDE.md`
- Buat: `docs/getting-started.md`
- Buat: `docs/adding-a-module.md`
- Buat: `docs/multi-tenancy.md`
- Buat: `docs/database-conventions.md`
- Buat: `docs/deployment.md`

- [ ] **Langkah 1: Tulis unit systemd**

Buat `deploy/app-api.service`:
```ini
[Unit]
Description=App API
After=network.target postgresql.service

[Service]
Type=simple
User=app
WorkingDirectory=/opt/app
EnvironmentFile=/etc/app/api.env
ExecStart=/opt/app/api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Buat `deploy/app-cleanup.service`:
```ini
[Unit]
Description=Run cli cleanup (expired tokens, old login attempts)

[Service]
Type=oneshot
User=app
WorkingDirectory=/opt/app
EnvironmentFile=/etc/app/api.env
ExecStart=/opt/app/cli cleanup
```

Buat `deploy/app-cleanup.timer`:
```ini
[Unit]
Description=Daily cli cleanup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Buat `deploy/pg-backup.service`:
```ini
[Unit]
Description=PostgreSQL backup

[Service]
Type=oneshot
User=app
EnvironmentFile=/etc/app/api.env
ExecStart=/opt/app/deploy/pg-backup.sh
```

Buat `deploy/pg-backup.timer`:
```ini
[Unit]
Description=Daily PostgreSQL backup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

- [ ] **Langkah 2: Tulis script backup**

Buat `deploy/pg-backup.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/backups/app-db"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
FILE="$BACKUP_DIR/appdb-$TIMESTAMP.dump"

pg_dump -Fc -h "${DB_HOST:-localhost}" -U "${DB_USER:-app}" "${DB_NAME:-appdb}" > "$FILE"

# Simpan 35 hari secara lokal; kebijakan retensi sesungguhnya ada di mana
# pun file ini disalin keluar server.
find "$BACKUP_DIR" -name "appdb-*.dump" -mtime +35 -delete

echo "backup tersimpan: $FILE"
echo "PENGINGAT: backup ini hanya berguna kalau disalin keluar dari server ini"
echo "           (rsync/rclone ke penyimpanan remote) dan proses restore-nya sudah diuji."
```

- [ ] **Langkah 3: Tulis config Caddy**

Buat `deploy/Caddyfile`:
```
{$APP_DOMAIN} {
    handle /api/* {
        reverse_proxy localhost:8080
    }
    handle {
        root * /var/www/frontend
        try_files {path} /index.html
        file_server
    }
}
```

- [ ] **Langkah 4: Tulis script deploy**

Buat `deploy/deploy.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

cd /opt/app

echo "==> menarik kode terbaru"
git pull

echo "==> membackup binary saat ini"
cp -f api api.backup 2>/dev/null || true
cp -f cli cli.backup 2>/dev/null || true

echo "==> build"
go build -o api ./cmd/api
go build -o cli ./cmd/cli

echo "==> migrasi schema platform"
./cli migrate platform up

echo "==> migrasi schema tenant"
./cli migrate tenant up --all

echo "==> sinkronisasi permission"
./cli permission sync --all

echo "==> restart service"
sudo systemctl restart app-api

echo "==> selesai"
```

Jadikan executable:
```bash
chmod +x deploy/deploy.sh deploy/pg-backup.sh
```

- [ ] **Langkah 5: Tulis `CLAUDE.md`**

Buat `CLAUDE.md`:
```markdown
# CLAUDE.md

Aturan untuk bekerja di repository ini. Melanggar salah satunya punya
konsekuensi nyata — biasanya data bocor lintas tenant — bukan sekadar
soal gaya penulisan.

## Multi-tenancy (lihat `docs/multi-tenancy.md` untuk penjelasan lengkap)

- **Setiap query data tenant lewat `database.WithTenant(ctx, fn)`.**
  Tidak ada repository yang memegang `*sqlx.DB` polos untuk tabel tenant.
  Tanpa pengecualian.
- **`*fiber.Ctx` tidak pernah keluar dari layer handler.** Service dan
  repository hanya menerima `context.Context`. Fiber dibangun di atas
  `fasthttp`, yang memakai ulang objek request antar request — apa pun
  yang lolos dari handler dengan referensi `*fiber.Ctx` yang masih hidup
  bisa berakhir membaca data request lain (mungkin milik tenant lain).
- **Setiap parameter query `sort` divalidasi terhadap whitelist eksplisit**
  (`pagination.Parse` + peta `Sortable` milik modul) sebelum diinterpolasi
  ke `ORDER BY`. Jangan pernah mengoper nilai query mentah ke SQL, bahkan
  secara tidak langsung.

## Batas modul

- **Satu modul tidak pernah memanggil repository modul lain secara
  langsung.** Interaksi lintas modul lewat *service* modul lain.
- Modul tenant-scoped baru menyalin bentuk
  `internal/modules/tenant/user/` — lihat `docs/adding-a-module.md`.

## Error dan response

- Service mengembalikan `*apperror.Error`, tidak pernah `error` polos
  yang membungkus error driver langsung ke client.
- Handler tidak pernah memanggil `c.Status(...)` sendiri —
  `response.Success` / `response.Error` yang memegang itu.

## Database

- ID dibuat di Go (`uuid.New()`), tidak pernah lewat default sisi DB —
  lihat `docs/database-conventions.md` untuk alasannya.
- Tabel baru mengikuti konvensi kolom audit di
  `docs/database-conventions.md`, kecuali tabel penghubung atau tabel
  log/token (dua pengecualian yang sudah didokumentasikan).

## Testing

- Integration test memakai `testsupport.OpenTestDB` / `OpenTestPlatformDB`,
  didukung database PostgreSQL lokal `app_test` — tidak pernah Docker,
  tidak pernah testcontainers.
- Test yang membuat schema wajib men-drop-nya di `t.Cleanup`.
```

- [ ] **Langkah 6: Tulis `docs/getting-started.md`**

Buat `docs/getting-started.md`:
```markdown
# Mulai Cepat (development di Windows)

## 1. Install prasyarat

- Go 1.25+
- PostgreSQL, jalan lokal
- `air` untuk hot reload: `go install github.com/air-verse/air@latest`

## 2. Buat database

```powershell
psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE appdb OWNER app;"
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```

## 3. Atur environment

```powershell
copy .env.example .env
copy .env.test.example .env.test
```
Sesuaikan `DB_PASSWORD` di kedua berkas kalau kamu pakai password berbeda di atas.

## 4. Jalankan migrasi dan buat admin pertama

```powershell
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Nama Kamu"
```
Salin password yang dicetak — hanya ditampilkan sekali.

## 5. Buat tenant pertama

```powershell
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test
```
Salin juga password owner yang dicetak.

## 6. Jalankan API

```powershell
air
```
atau tanpa hot reload:
```powershell
go run ./cmd/api
```

## 7. Coba

```powershell
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<password owner>\"}"
```

## Menjalankan test

```powershell
go test ./...
```
Integration test terhubung ke `app_test` memakai `.env.test` dan otomatis
di-skip kalau PostgreSQL tidak bisa diakses.
```

- [ ] **Langkah 7: Tulis `docs/adding-a-module.md`**

Buat `docs/adding-a-module.md`:
```markdown
# Menambah Modul

Setiap modul tenant-scoped mengikuti bentuk
`internal/modules/tenant/user/` — salin folder itu sebagai titik awal.

## Bentuk berkas

```
modules/tenant/<nama>/
  model.go       # struct baris DB, tag `db:"..."`
  dto.go         # struct request/response, tag `json:"..."` + `validate:"..."`
  repository.go  # SQL lewat sqlx, selalu lewat database.WithTenant
  service.go     # business logic, hanya context.Context, tanpa tipe HTTP
  handler.go     # hanya Fiber, konversi ke/dari DTO, tidak pernah menyentuh SQL
```

## Langkah-langkah

1. **Tambah migrasi.** Buat
   `internal/migration/tenant/0000N_create_<nama>_table.up.sql` (dan
   `.down.sql`), ikuti `database-conventions.md` untuk kolom audit dan
   soft delete. Naikkan nomor versinya.
2. **Tulis `model.go`** — satu struct per tabel, tag `db:"..."` cocok
   persis dengan nama kolom.
3. **Tulis `repository.go`** — setiap method membungkus query-nya dalam
   `db.WithTenant(ctx, func(tx *sqlx.Tx) error {...})`. Deklarasikan
   whitelist `Sortable map[string]string` untuk apa pun yang bisa
   di-sort di endpoint list — jangan pernah menerima nilai query `sort`
   mentah.
4. **Tulis `dto.go`** — struct request dengan tag `validate:"..."`,
   struct `View` untuk response, dan mapper `toView`.
5. **Tulis `service.go`** — hanya bergantung pada repository dan
   `context.Context`. Hashing password, panggilan eksternal, atau
   panggilan lintas modul (lewat *service* modul lain, tidak pernah
   repository-nya) masuk di sini.
6. **Tulis `handler.go`** — parse body, panggil `validator.Validate`,
   panggil service, terjemahkan hasilnya dengan `response.Success` /
   `response.Error`. Jangan pernah menulis `c.Status(...)` langsung.
7. **Tambah konstanta permission** di `internal/permission/constants.go`
   untuk resource baru (`<nama>.view`, `<nama>.create`, ...), mengikuti
   pola `user.*` yang sudah ada.
8. **Sambungkan rute** di `internal/server/router.go`, di dalam grup
   `tenantAPI`, masing-masing di belakang
   `middleware.RequirePermission(permission.KonstantaKamu, deps.PermissionCache)`.
9. **Tambahkan handler-nya ke `server.Dependencies`** dan konstruksi di
   `cmd/api/main.go`.
10. **Jalankan `cli permission sync --all`** setelah deploy, supaya
    tenant yang sudah ada dapat baris permission baru.

## Sub-grup modul

Biarkan `modules/tenant/` datar sampai berisi 7–8 modul dan navigasi
mulai sulit. Baru kelompokkan per domain, mis. `modules/tenant/sales/order/`,
`modules/tenant/inventory/product/` — modulnya sendiri tidak berubah
bentuk, cuma path-nya. **Modul tidak boleh memanggil repository modul
lain secara langsung** — interaksi lintas modul selalu lewat *service*
modul lain.
```

- [ ] **Langkah 8: Tulis `docs/multi-tenancy.md`**

Buat `docs/multi-tenancy.md`:
```markdown
# Aturan Multi-Tenancy

Dua aturan ini yang menjaga data tenant A tidak pernah muncul di
response tenant B. Melanggar salah satunya adalah kebocoran data,
bukan sekadar bug.

## Aturan 1: semua akses data tenant lewat `database.WithTenant`

```go
// Benar
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1`, id)
	})
	return u, err
}
```
```go
// Salah — melewati scoping tenant sepenuhnya
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.pool.Get(&u, `SELECT * FROM users WHERE id = $1`, id) // tenant yang mana?!
	return u, err
}
```
`WithTenant` menjalankan query kamu di dalam transaksi dengan `SET LOCAL
search_path` di-pin ke schema tenant si pemanggil — diambil dari
`context.Context`, tidak pernah dari parameter request. `SET LOCAL`
otomatis hilang saat transaksi selesai (commit maupun rollback), jadi
satu koneksi tidak pernah bisa membawa schema satu tenant ke pemanggil
berikutnya yang meminjamnya dari pool.

## Aturan 2: `*fiber.Ctx` berhenti di handler

Fiber dibangun di atas `fasthttp`, yang **memakai ulang** objek context-nya
antar request demi performa. Kalau `*fiber.Ctx` (atau apa pun turunannya)
lolos ke goroutine atau disimpan di mana pun, dia bisa diam-diam ditimpa
oleh request yang sama sekali berbeda — termasuk milik tenant lain —
sebelum sempat dibaca kembali.

```go
// Benar — handler mengonversi semua yang dibutuhkan sebelum memanggil service
func (h *Handler) Get(c *fiber.Ctx) error {
	id, _ := uuid.Parse(c.Params("id"))
	view, err := h.svc.Get(c.UserContext(), id) // hanya context.Context, dari sini ke bawah
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, view)
}
```
```go
// Salah — jangan pernah lakukan ini
func (h *Handler) Get(c *fiber.Ctx) error {
	go func() {
		log.Println(c.Params("id")) // c mungkin sudah jadi milik request lain saat ini
	}()
	return nil
}
```
Service dan repository hanya pernah melihat `context.Context`. Ini bukan
cuma soal keamanan — ini juga berarti business logic bisa di-unit-test
tanpa menyalakan server HTTP.

## Apa saja yang berakhir di `context.Context`

Diisi oleh middleware, dibaca lewat `database.TenantFromContext` /
`database.ActorFromContext`:
- `database.TenantInfo` — ID tenant + nama schema, diisi `RequireTenant`
- `database.Actor` — ID user + scope, diisi `RequirePlatform` /
  `RequireTenant`

Satu pengecualian adalah login: belum ada token yang membuktikan
identitas tenant, jadi `modules/tenant/auth.Service.Login` membuat
`TenantInfo` sendiri dari `tenant_code` yang sudah di-resolve, sebelum
memanggil `WithTenant`. Semua jalur kode tenant-scoped lainnya
menerimanya sudah diisi oleh middleware.
```

- [ ] **Langkah 9: Tulis `docs/database-conventions.md`**

Buat `docs/database-conventions.md`:
```markdown
# Konvensi Database

## Setiap tabel (dengan dua pengecualian di bawah)

```sql
created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at  TIMESTAMPTZ NULL
created_by  UUID NULL
updated_by  UUID NULL
deleted_by  UUID NULL
```

- `updated_at` dijaga oleh trigger per-schema
  (`trigger_set_updated_at`) — pasang di setiap tabel baru:
  ```sql
  CREATE TRIGGER set_updated_at
      BEFORE UPDATE ON nama_tabel_kamu
      FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
  ```
- Kolom `*_by` diisi dari `database.ActorFromContext(ctx)` di layer
  repository, tidak pernah dibiarkan ditebak database.
- ID berupa `UUID PRIMARY KEY` **tanpa default sisi database** — buat
  dengan `uuid.New()` di Go sebelum insert. (Transaksi dengan `SET LOCAL
  search_path` hanya mencari di schema tenant itu sendiri, bukan
  `public`, jadi default sisi DB yang memanggil fungsi extension di
  sana bisa gagal secara tidak terduga. Membuatnya di Go menghindari
  masalah ini sepenuhnya.)

## Soft delete

Setiap query memfilter `WHERE deleted_at IS NULL` kecuali memang sedang
mencari baris yang terhapus (seperti pengecekan 30-hari `tenant purge`).
Kolom mana pun yang punya syarat keunikan butuh unique index **parsial**
supaya nilai milik baris yang terhapus bisa dipakai ulang:
```sql
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;
```

## Pengecualian

**Tabel penghubung** (`role_permissions`, `user_roles`): hanya
`created_at` + `created_by`. Akses dicabut dengan `DELETE` biasa, bukan
soft delete — menumpuk filter `deleted_at IS NULL` di setiap pengecekan
permission tanpa manfaat itu tidak sepadan, dan riwayat
pemberian/pencabutan akses lebih pas ada di log, bukan di tabel
penghubung.

**Tabel log/token** (`refresh_tokens`, `login_attempts`): hanya
`created_at`. Sifatnya append-only — tidak ada baris di dalamnya yang
pernah di-update atau soft-delete. Refresh token yang dicabut
direpresentasikan lewat kolom `revoked_at` miliknya sendiri, bukan
`deleted_at` standar.

## Penamaan

- Tabel: bentuk jamak, `snake_case` (`users`, `role_permissions`).
- Kolom: `snake_case`, tanpa prefix tipe (`email`, bukan `str_email`).
- Foreign key: `<nama_tabel_tunggal>_id` (`user_id`, `role_id`).
```

- [ ] **Langkah 10: Tulis `docs/deployment.md`**

Buat `docs/deployment.md`:
```markdown
# Deployment

Tanpa Docker sama sekali di stack ini — Go compile jadi satu binary
statis, jadi server tidak butuh apa pun selain binary itu, PostgreSQL,
dan Caddy.

## Susunan di server

```
/opt/app/
  api               # binary HTTP server
  cli               # binary CLI (migrasi, provisioning, cleanup)
  api.backup        # binary sebelumnya, untuk rollback instan

/etc/app/api.env    # config, chmod 600
/etc/systemd/system/app-api.service
/etc/systemd/system/app-cleanup.service
/etc/systemd/system/app-cleanup.timer
```

## Setup pertama kali

```bash
sudo useradd -r -s /usr/sbin/nologin app
sudo mkdir -p /opt/app /etc/app
sudo chown app:app /opt/app
git clone <url-repo-kamu> /opt/app
sudo cp deploy/app-api.service deploy/app-cleanup.service deploy/app-cleanup.timer /etc/systemd/system/
sudo cp .env.example /etc/app/api.env   # lalu edit dengan nilai sungguhan
sudo chmod 600 /etc/app/api.env
sudo systemctl daemon-reload
sudo systemctl enable --now app-cleanup.timer
```

Install Caddy, lalu:
```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile   # edit domain-nya dulu
sudo systemctl reload caddy
```

### Ubuntu/Debian vs Rocky

Package manager beda (`apt install postgresql caddy` vs
`dnf install postgresql-server caddy`); selebihnya — unit systemd,
binary Go, script deploy — identik.

**Khusus Rocky:** SELinux aktif secara default dan memblokir Caddy
terhubung ke `localhost:8080` kecuali diizinkan:
```bash
sudo setsebool -P httpd_can_network_connect 1
```
Tanpa ini, request lewat Caddy gagal dengan "Permission denied" yang
kelihatannya tidak ada hubungannya dengan SELinux — kalau Caddy tidak
bisa menjangkau API di Rocky, ini yang pertama dicek.

## Deploy pembaruan

```bash
cd /opt/app
./deploy/deploy.sh
```
Ini menarik kode, membackup binary saat ini, build ulang, menjalankan
migrasi platform + tenant, sinkronisasi permission, dan restart service.
Perkirakan downtime 1–2 detik saat restart — jadwalkan deploy di luar
jam operasional untuk aplikasi dengan jam operasional tetap (seperti POS).

### Rollback

```bash
cp api.backup api && sudo systemctl restart app-api
```

## Backup

```bash
sudo cp deploy/pg-backup.service deploy/pg-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pg-backup.timer
```
Ini menjalankan `pg_dump` harian dan menyimpan 35 hari secara lokal.
**Salin backup keluar dari server** (rsync/rclone ke penyimpanan remote)
— backup di disk yang sama yang ikut rusak tidak melindungi apa pun. Uji
restore secara berkala; backup yang belum pernah diuji cuma tebakan,
bukan backup.

## Environment variable

Lihat `.env.example` untuk daftar lengkap. Semuanya bisa
dibaca/diubah tanpa build ulang — edit `/etc/app/api.env` lalu
`systemctl restart app-api`.
```

- [ ] **Langkah 11: Commit**

```bash
git add deploy CLAUDE.md docs/getting-started.md docs/adding-a-module.md docs/multi-tenancy.md docs/database-conventions.md docs/deployment.md
git commit -m "docs: deployment artifacts, CLAUDE.md, and full documentation set"
```

---

### Tugas 22: Dokumentasi OpenAPI lewat `swaggo/swag`

Menutup satu item dari tabel stack spec yang belum ada tugasnya: `swaggo/swag` men-generate dokumentasi OpenAPI dari komentar tepat di atas setiap handler, jadi dokumentasinya tidak bisa jauh menyimpang dari kode tanpa `swag init` gagal compile secara nyata.

**Catatan bahasa:** anotasi `@Summary` dkk. di bawah ini sengaja dibiarkan Bahasa Inggris — itu konvensi umum ekosistem OpenAPI/Swagger dan tetap konsisten dengan aturan "identifier & anotasi tooling tetap Inggris" di Batasan Global.

**Berkas:**
- Ubah: `cmd/api/main.go` — tambah anotasi API umum
- Ubah: `internal/modules/platform/auth/handler.go` — tambah anotasi
- Ubah: `internal/modules/tenant/auth/handler.go` — tambah anotasi
- Ubah: `internal/modules/tenant/user/handler.go` — tambah anotasi
- Ubah: `internal/server/router.go` — sajikan spec yang di-generate
- Ubah: `Makefile` — tambah `make swagger`
- Buat: `docs/swagger/` (hasil generate, bukan tulisan tangan)

- [ ] **Langkah 1: Install CLI swag dan tambah dependency runtime**

Jalankan:
```bash
go install github.com/swaggo/swag/cmd/swag@latest
go get github.com/gofiber/swagger
go get github.com/swaggo/swag
```

- [ ] **Langkah 2: Tambah anotasi API umum ke `main.go`**

Ubah `cmd/api/main.go` — tambahkan blok komentar ini tepat di atas `package main`:
```go
// @title           App API
// @version         1.0
// @description     Multi-tenant API starter — schema-per-tenant PostgreSQL. Point of Sales is the first thing built on it, not a fixed part of it.
// @BasePath        /api/v1
package main
```

- [ ] **Langkah 3: Anotasi handler auth platform**

Ubah `internal/modules/platform/auth/handler.go` — tambahkan blok komentar tepat di atas tiap method:
```go
// Login godoc
// @Summary      Admin login
// @Tags         platform-auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "credentials"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /admin/auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
```
```go
// Refresh godoc
// @Summary      Refresh an admin access token
// @Tags         platform-auth
// @Accept       json
// @Produce      json
// @Param        body body RefreshRequest true "refresh token"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /admin/auth/refresh [post]
func (h *Handler) Refresh(c *fiber.Ctx) error {
```
```go
// Logout godoc
// @Summary      Revoke an admin refresh token
// @Tags         platform-auth
// @Accept       json
// @Produce      json
// @Param        body body LogoutRequest true "refresh token"
// @Success      200 {object} map[string]any
// @Router       /admin/auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
```
(Ganti baris `func (h *Handler) Login(c *fiber.Ctx) error {` polos yang sudah ada di berkas dengan versi beranotasi ini — isi method di bawahnya tidak berubah dari Tugas 12.)

- [ ] **Langkah 4: Anotasi handler auth tenant**

Ubah `internal/modules/tenant/auth/handler.go` dengan cara yang sama:
```go
// Login godoc
// @Summary      Tenant staff login
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "tenant_code, credentials"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
```
```go
// Refresh godoc
// @Summary      Refresh a tenant access token
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body RefreshRequest true "tenant_code, refresh token"
// @Success      200 {object} LoginResponse
// @Failure      401 {object} map[string]any
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(c *fiber.Ctx) error {
```
```go
// Logout godoc
// @Summary      Revoke a tenant refresh token
// @Tags         tenant-auth
// @Accept       json
// @Produce      json
// @Param        body body LogoutRequest true "tenant_code, refresh token"
// @Success      200 {object} map[string]any
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
```

- [ ] **Langkah 5: Anotasi handler user tenant**

Ubah `internal/modules/tenant/user/handler.go`:
```go
// Create godoc
// @Summary      Create a tenant user
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body body CreateRequest true "new user"
// @Success      201 {object} View
// @Failure      422 {object} map[string]any
// @Router       /users [post]
func (h *Handler) Create(c *fiber.Ctx) error {
```
```go
// Get godoc
// @Summary      Get a tenant user by ID
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "user id"
// @Success      200 {object} View
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [get]
func (h *Handler) Get(c *fiber.Ctx) error {
```
```go
// List godoc
// @Summary      List tenant users
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        page query int false "page number"
// @Param        limit query int false "page size, max 100"
// @Param        sort query string false "created_at, name, or email"
// @Param        order query string false "asc or desc"
// @Success      200 {object} map[string]any
// @Router       /users [get]
func (h *Handler) List(c *fiber.Ctx) error {
```
```go
// Update godoc
// @Summary      Update a tenant user
// @Tags         users
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id path string true "user id"
// @Param        body body UpdateRequest true "fields to change"
// @Success      200 {object} map[string]any
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [patch]
func (h *Handler) Update(c *fiber.Ctx) error {
```
```go
// Delete godoc
// @Summary      Soft-delete a tenant user
// @Tags         users
// @Security     BearerAuth
// @Produce      json
// @Param        id path string true "user id"
// @Success      200 {object} map[string]any
// @Failure      404 {object} map[string]any
// @Router       /users/{id} [delete]
func (h *Handler) Delete(c *fiber.Ctx) error {
```

- [ ] **Langkah 6: Generate spec-nya**

Jalankan:
```bash
swag init -g cmd/api/main.go -o docs/swagger
```
Hasil: membuat `docs/swagger/docs.go`, `swagger.json`, `swagger.yaml`.

- [ ] **Langkah 7: Sajikan lewat HTTP**

Ubah `internal/server/router.go` — tambahkan ke blok import:
```go
swaggerui "github.com/gofiber/swagger"
_ "go-api-starter/docs/swagger" // generated by `swag init`; registers the spec swaggerui reads
```
dan tambahkan, tepat setelah `app := fiber.New(...)` dan sebelum rantai middleware:
```go
app.Get("/swagger/*", swaggerui.HandlerDefault)
```

- [ ] **Langkah 8: Tambah target Makefile**

Ubah `Makefile` — tambahkan:
```makefile
swagger:
	swag init -g cmd/api/main.go -o docs/swagger
```
dan tambahkan `swagger` ke baris `.PHONY`.

- [ ] **Langkah 9: Pastikan bisa di-build dan disajikan**

Jalankan:
```bash
go build ./...
go run ./cmd/api
```
Lalu buka `http://localhost:8080/swagger/index.html` di browser.
Hasil: Swagger UI muncul dan mendaftar setiap endpoint yang dianotasi, dikelompokkan per tag (`platform-auth`, `tenant-auth`, `users`).

- [ ] **Langkah 10: Commit**

```bash
git add cmd/api internal/modules internal/server docs/swagger Makefile go.mod go.sum
git commit -m "docs: OpenAPI documentation via swaggo/swag"
```

---

## Verifikasi Akhir

- [ ] Jalankan `go build ./...` — berhasil tanpa error.
- [ ] Jalankan `go vet ./...` — tanpa warning.
- [ ] Jalankan `go test ./...` — semua test dari Tugas 2–20 lolos.
- [ ] Jalankan smoke test manual dari Tugas 18 Langkah 8, dari awal sampai akhir di database bersih.
- [ ] Buka `http://localhost:8080/swagger/index.html` dan pastikan semua endpoint terdaftar (Tugas 22).
- [ ] Pastikan `git log --oneline` menunjukkan satu commit per tugas, masing-masing dengan test yang lolos pada titik itu di riwayatnya.
