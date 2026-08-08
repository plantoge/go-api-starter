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

**Stack Teknologi:** Go 1.22, Fiber v2, PostgreSQL, sqlx + pgx (driver `stdlib`), golang-migrate (sumber embedded `iofs`), golang-jwt/jwt/v5, bcrypt, go-playground/validator/v10, log/slog (JSON), oklog/ulid/v2 (request ID), swaggo/swag, air (hot reload saat development). Tanpa Docker sama sekali — development, testing, dan deploy semuanya menjalankan PostgreSQL dan binary Go langsung di host.

## Batasan Global

- Module path: `go-api-starter` (tanpa prefix host VCS — starter privat, belum dipublikasikan).
- Versi Go: minimal 1.22 (`go.mod`).
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

Edit `go.mod`, pastikan direktif `go` berbunyi `go 1.22`.

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
