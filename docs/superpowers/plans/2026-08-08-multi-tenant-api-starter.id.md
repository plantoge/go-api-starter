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
```go
package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
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
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
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
