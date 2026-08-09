# Multi-Tenant API Starter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Golang multi-tenant API starterpack (schema-per-tenant PostgreSQL) with platform+tenant auth, full RBAC, a tenant-scoped example module (`user`), CLI tooling for provisioning/migration, and a Docker-free deployment path — reusable as the foundation for future projects (first use case: a POS system built in a separate repo).

**Architecture:** Fiber v2 HTTP handlers stay thin and convert everything to `context.Context` immediately (never pass `*fiber.Ctx` past the handler layer). Services hold business logic and know nothing about HTTP. Repositories run SQL via sqlx against connections scoped per-request to a tenant's schema through `SET LOCAL search_path` inside a transaction (`database.WithTenant`). Platform data (tenants, platform admins) lives in its own `platform` schema on a separately pinned connection pool.

**Tech Stack:** Go 1.25, Fiber v2, PostgreSQL, sqlx + pgx (`stdlib` driver), golang-migrate (`iofs` embedded source), golang-jwt/jwt/v5, bcrypt, go-playground/validator/v10, log/slog (JSON), oklog/ulid/v2 (request IDs), swaggo/swag, air (dev hot reload). No Docker anywhere — dev, test, and deploy all run PostgreSQL and the Go binary directly on the host.

## Global Constraints

- Module path: `go-api-starter` (no VCS host prefix — private starter, not yet published).
- Go version: 1.25 minimum (raised from the original 1.22 during Task 4 — golang.org/x/crypto/sys/text at current releases require 1.25; pinning older transitive crypto deps to preserve 1.22 was judged a worse tradeoff than raising the floor) (`go.mod`).
- `*fiber.Ctx` must never be passed into a service or repository function — handlers convert to `context.Context` (via `c.UserContext()`, populated by middleware) before calling `service.*`.
- All tenant data access goes through `database.WithTenant(ctx, fn)` — no repository may hold a bare `*sqlx.DB` for tenant tables.
- Every table gets `created_at, updated_at, deleted_at, created_by, updated_by, deleted_by` **except**: join tables (`role_permissions`, `user_roles` — `created_at`+`created_by` only, hard-deleted) and log/token tables (`refresh_tokens`, `login_attempts` — `created_at` only).
- IDs are UUIDs generated in Go (`uuid.New()`), never DB-side defaults — `SET LOCAL search_path` replaces (not appends to) the path, so unqualified calls to extension functions inside a tenant transaction cannot be relied upon.
- Every list endpoint's `sort` parameter must be validated against an explicit per-repository whitelist map — never interpolated from the request unchecked.
- Passwords hashed with bcrypt, cost 12. Minimum 8 characters, checked against a short common-password blocklist.
- JWT carries `sub`, `scope` (`platform`|`tenant`), `tenant_id` (tenant scope only), `exp`. Never carries permissions.
- Response envelope: `{"success": true, "data": ...}` / `{"success": false, "error": {"code","message","details"}, "request_id": ...}`. Validation errors put `field -> []message` in `details`, using the JSON field name.
- All HTTP routes live under `/api/v1`.
- No Docker. No testcontainers — integration tests run against a local PostgreSQL `app_test` database, config from `.env.test`.
- **Language convention:** Go identifiers (functions, types, variables, packages) stay in English — that's the idiomatic Go convention and keeps the codebase compatible with its all-English dependencies (Fiber, sqlx, etc.). Everything a human reads that isn't an identifier — code comments, CLI `fmt.Println`/`fmt.Printf` output, log messages, and API error/response messages — is written in Bahasa Indonesia. Every code snippet below that has an English comment or English CLI-facing string should be translated to Indonesian when actually typed into a file; only the snippets that happen to already be pure Indonesian prose (most `apperror.*` messages already are) are copy-ready as-is. Example:
  ```go
  // English (as written in this plan's snippets — translate before saving):
  // Login is the one place tenant identity is established without a token.
  fmt.Println("tenant provisioned:")

  // Indonesian (what the actual file should contain):
  // Login adalah satu-satunya tempat identitas tenant ditetapkan tanpa token.
  fmt.Println("tenant berhasil di-provisioning:")
  ```

---

## File Structure

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
        tenant/{model.go,dto.go,repository.go,service.go}   # no handler.go — see Task 13 note
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
    (written per-task alongside the code they document)
```

Each `modules/**` package is `handler.go` (Fiber, `*fiber.Ctx` only) → `service.go` (business logic, `context.Context` only) → `repository.go` (sqlx, `*sqlx.Tx`) → `dto.go` (request/response structs) → `model.go` (DB row structs). This shape is what Task 17's `user` module demonstrates for future modules to copy.

---

### Task 1: Project scaffolding & Go module

**Files:**
- Create: `go.mod`
- Create: `.gitattributes`
- Create: `.env.example`
- Create: `.env.test.example`
- Create: `Makefile`

**Interfaces:**
- Produces: a buildable empty module other tasks add packages into.

- [ ] **Step 1: Initialize the module**

Run:
```bash
cd D:/app/go-api-starter
go mod init go-api-starter
```
Expected: creates `go.mod` with `module go-api-starter` and a `go` directive.

- [ ] **Step 2: Set Go version floor**

Edit `go.mod`, ensure the `go` directive reads `go 1.25`.

- [ ] **Step 3: Add `.gitattributes` to force LF on scripts and SQL**

Create `.gitattributes`:
```
* text=auto eol=lf
*.sh text eol=lf
*.sql text eol=lf
Makefile text eol=lf
```

- [ ] **Step 4: Write `.env.example`**

Create `.env.example`:
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

- [ ] **Step 5: Write `.env.test.example`**

Create `.env.test.example`:
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

- [ ] **Step 6: Write the Makefile**

Create `Makefile`:
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

- [ ] **Step 7: Verify the module builds**

Run: `go build ./...`
Expected: succeeds with no output (no packages yet, exit code 0).

- [ ] **Step 8: Commit**

```bash
git add go.mod .gitattributes .env.example .env.test.example Makefile
git commit -m "chore: scaffold Go module, env templates, Makefile"
```

---

### Task 2: Config loading + validation

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `config.Config` struct: `DB DBConfig`, `App AppConfig`, `JWT JWTConfig`, `Login LoginConfig`, `CORS CORSConfig`
  - `config.Load() (*Config, error)` — reads env vars (loads `.env` first if present), validates, returns one error listing every missing/invalid field.
  - `(c DBConfig) DSN() string` — pgx connection string.
  - `(c DBConfig) PlatformDSN() string` — same, with `search_path=platform` pinned.

- [ ] **Step 1: Add dependency**

Run: `go get github.com/joho/godotenv@latest`

- [ ] **Step 2: Write the failing test**

Create `internal/config/config_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL — package does not compile (`Load` undefined).

- [ ] **Step 4: Implement config.go**

Create `internal/config/config.go`:
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS — all three tests pass.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config
git commit -m "feat: env-based config loading with fail-fast validation"
```

---

### Task 3: Shared error type & response envelope

**Files:**
- Create: `internal/apperror/error.go`
- Create: `internal/response/response.go`
- Test: `internal/apperror/error_test.go`
- Test: `internal/response/response_test.go`

**Interfaces:**
- Produces:
  - `apperror.Code` (string type) and constants: `CodeValidation`, `CodeNotFound`, `CodeConflict`, `CodeForbidden`, `CodeUnauthorized`, `CodeInternal`, `CodeTenantMigrationPending`, `CodeRateLimited`.
  - `apperror.Error struct { Code Code; Message string; Details map[string][]string; Status int }` implementing `error`, plus `Unwrap() error`.
  - Constructors: `apperror.NotFound(msg string) *Error`, `apperror.Conflict(msg string) *Error`, `apperror.Validation(details map[string][]string) *Error`, `apperror.Forbidden(msg string) *Error`, `apperror.Unauthorized(msg string) *Error`, `apperror.RateLimited(msg string) *Error`, `apperror.TenantMigrationPending() *Error`, `apperror.Internal(cause error) *Error`.
  - `response.Success(c *fiber.Ctx, status int, data any) error`
  - `response.SuccessList(c *fiber.Ctx, status int, data any, meta response.Meta) error`
  - `response.Meta struct { Page, Limit, Total, TotalPages int }`
  - `response.Error(c *fiber.Ctx, requestID string, err error) error` — maps `*apperror.Error` to its declared status/body; any other error becomes a logged 500 with only `request_id` exposed.

- [ ] **Step 1: Add Fiber dependency**

Run: `go get github.com/gofiber/fiber/v2@latest`

- [ ] **Step 2: Write the failing apperror test**

Create `internal/apperror/error_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/apperror/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 4: Implement error.go**

Create `internal/apperror/error.go`:
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

// Error is the one error type every service function returns. The HTTP
// layer (response.Error) knows how to turn it into a status code and JSON
// body without the handler ever writing c.Status(...) itself.
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

- [ ] **Step 5: Run apperror tests to verify they pass**

Run: `go test ./internal/apperror/... -v`
Expected: PASS.

- [ ] **Step 6: Write the failing response test**

Create `internal/response/response_test.go`:
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

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/response/... -v`
Expected: FAIL — package does not compile (`Success`, `Error` undefined).

- [ ] **Step 8: Implement response.go**

Create `internal/response/response.go`:
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

// Error renders err as the standard error envelope. *apperror.Error values
// use their declared status/code/details verbatim. Anything else (a bug, a
// driver error that leaked past a service) becomes a 500 whose body carries
// only request_id — the real error is logged, never sent to the client.
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

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/response/... -v`
Expected: PASS — all three tests pass.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum internal/apperror internal/response
git commit -m "feat: typed app errors and uniform JSON response envelope"
```

---

### Task 4: Validator + Pagination helpers

**Files:**
- Create: `internal/validator/validator.go`
- Create: `internal/pagination/pagination.go`
- Test: `internal/validator/validator_test.go`
- Test: `internal/pagination/pagination_test.go`

**Interfaces:**
- Produces:
  - `validator.Validate(s any) *apperror.Error` — runs go-playground/validator, returns `nil` if valid, else `apperror.Validation(details)` keyed by each field's JSON tag name.
  - `pagination.Params struct { Page, Limit int; Sort, Order string }`
  - `pagination.Parse(c *fiber.Ctx, sortable map[string]string, defaultSort string) (Params, *apperror.Error)` — reads `page`, `limit`, `sort`, `order` query params; clamps `limit` to `[1,100]`; rejects `sort` not in `sortable` and `order` not `asc`/`desc` with `apperror.Validation`.
  - `(p Params) Offset() int`
  - `(p Params) OrderByClause(sortable map[string]string) string` — e.g. `"created_at DESC"`, built only from the whitelisted column name, never the raw query value.
  - `pagination.BuildMeta(page, limit, total int) response.Meta`

- [ ] **Step 1: Add validator dependency**

Run: `go get github.com/go-playground/validator/v10@latest`

- [ ] **Step 2: Write the failing validator test**

Create `internal/validator/validator_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/validator/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 4: Implement validator.go**

Create `internal/validator/validator.go`:
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
	// Report the JSON field name (e.g. "email") instead of the Go struct
	// field name (e.g. "Email") so client-side error mapping is exact.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// Validate runs struct tag validation on s. It returns nil when s is valid,
// otherwise an *apperror.Error with Details keyed by JSON field name so the
// frontend can place each message under the right input.
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

- [ ] **Step 5: Run validator tests to verify they pass**

Run: `go test ./internal/validator/... -v`
Expected: PASS.

- [ ] **Step 6: Write the failing pagination test**

Create `internal/pagination/pagination_test.go`:
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

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/pagination/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 8: Implement pagination.go**

Create `internal/pagination/pagination.go`:
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

// Parse reads page/limit/sort/order query params. sort must be a key in
// sortable (the caller's whitelist of columns that are safe to interpolate
// into ORDER BY) — anything else is rejected. This is the only place a raw
// query value is allowed near a sort column name; every repository must go
// through it rather than reading c.Query("sort") itself.
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

// OrderByClause returns "<column> ASC|DESC" using ONLY the whitelisted SQL
// column name from sortable[p.Sort] — never the raw p.Sort value itself —
// so it is always safe to append directly to a query string.
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

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/pagination/... -v`
Expected: PASS — all 5 tests pass.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum internal/validator internal/pagination
git commit -m "feat: request validation and whitelist-safe pagination"
```

---

### Task 5: Database — connection pools, `WithTenant`, context helpers

This is the single most safety-critical piece of the whole starter: it's what turns "one Postgres database" into "hard-isolated per-tenant schemas." Get this task's test green before building anything on top of it.

**Files:**
- Create: `internal/database/postgres.go`
- Create: `internal/database/tenant.go`
- Create: `internal/testsupport/testsupport.go` (test-only helper package — **never** imported from non-test code; it pulls in `testing`)
- Test: `internal/database/tenant_test.go`

**Interfaces:**
- Produces:
  - `database.NewPool(cfg config.DBConfig) (*sqlx.DB, error)` — general pool, search_path left at the connection default.
  - `database.NewPlatformPool(cfg config.DBConfig) (*sqlx.DB, error)` — pool pinned to `search_path=platform` on every connection.
  - `database.TenantInfo struct { TenantID uuid.UUID; SchemaName string }`
  - `database.Actor struct { UserID uuid.UUID; Scope string }`
  - `database.WithTenantInfo(ctx, TenantInfo) context.Context`, `database.TenantFromContext(ctx) (TenantInfo, bool)`
  - `database.WithActor(ctx, Actor) context.Context`, `database.ActorFromContext(ctx) (Actor, bool)`
  - `database.ValidSchemaName(name string) bool`
  - `database.DB struct { Pool *sqlx.DB }`, `database.NewDB(pool *sqlx.DB) *DB`
  - `(db *DB) WithTenant(ctx context.Context, fn func(tx *sqlx.Tx) error) error`
  - `testsupport.OpenTestDB(t *testing.T) *sqlx.DB` — skips the test if no local PostgreSQL is reachable.
  - `testsupport.RandomSchemaName() string`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/jmoiron/sqlx@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/google/uuid@latest
```

- [ ] **Step 2: Create the test-only database database `app_test`**

Run (one-time, manual, on your local PostgreSQL):
```bash
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```
(Adjust the role name to whatever `.env.test`'s `DB_USER` is. If the `app` role doesn't exist yet: `psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"` first, matching `.env.test`.)

Copy `.env.test.example` to `.env.test` in the repo root and adjust credentials if needed.

- [ ] **Step 3: Write the test support helper**

Create `internal/testsupport/testsupport.go`:
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

- [ ] **Step 4: Write the failing test**

Create `internal/database/tenant_test.go`:
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

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./internal/database/... -v`
Expected: FAIL — package does not compile (`database.NewDB` etc. undefined).

- [ ] **Step 6: Implement postgres.go**

Create `internal/database/postgres.go`:
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

- [ ] **Step 7: Implement tenant.go**

Create `internal/database/tenant.go`:
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

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/database/... -v`
Expected: PASS — `TestWithTenant_IsolatesSchemas`, `TestWithTenant_RollbackClearsSearchPath`, `TestWithTenant_NoTenantInContext_ReturnsError` all pass against your local `app_test` database. (If PostgreSQL isn't reachable, they SKIP instead of failing — go set that up per Step 2 before continuing.)

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/database internal/testsupport
git commit -m "feat: tenant-scoped DB pool with SET LOCAL search_path isolation"
```

---

### Task 6: CLI skeleton + migration runner + platform migrations

**Design note on schema creation:** golang-migrate's postgres driver expects the target schema to already exist when it opens (it creates its `schema_migrations` tracking table there immediately, and will not create the schema itself). So schema creation is **not** its own migration file — `MigratePlatformUp` issues `CREATE SCHEMA IF NOT EXISTS platform` directly before handing off to golang-migrate, and tenant schema creation happens explicitly during provisioning (Task 13), before any tenant migration runs.

**Files:**
- Create: `cmd/cli/main.go`
- Create: `cmd/cli/commands/registry.go`
- Create: `cmd/cli/commands/migrate.go`
- Create: `internal/migration/runner.go`
- Create: `internal/migration/platform_fs.go`
- Create: `internal/migration/platform/000001_create_updated_at_trigger_fn.up.sql`
- Create: `internal/migration/platform/000001_create_updated_at_trigger_fn.down.sql`
- Create: `internal/migration/platform/000002_create_tenants_table.up.sql`
- Create: `internal/migration/platform/000002_create_tenants_table.down.sql`
- Create: `internal/migration/platform/000003_create_platform_users_table.up.sql`
- Create: `internal/migration/platform/000003_create_platform_users_table.down.sql`
- Create: `internal/migration/platform/000004_create_platform_refresh_tokens_table.up.sql`
- Create: `internal/migration/platform/000004_create_platform_refresh_tokens_table.down.sql`
- Create: `internal/migration/platform/000005_create_login_attempts_table.up.sql`
- Create: `internal/migration/platform/000005_create_login_attempts_table.down.sql`
- Test: `internal/migration/runner_test.go`

**Interfaces:**
- Produces:
  - `commands.Register(name string, h commands.Handler)`, `commands.Lookup(name string) (Handler, bool)`, `commands.Names() []string`, `type Handler func(args []string) error`
  - `migration.MigratePlatformUp(db *sql.DB) error`
  - `migration.LatestPlatformVersion() uint`
  - CLI command: `cli migrate platform up`

- [ ] **Step 1: Add golang-migrate**

Run: `go get github.com/golang-migrate/migrate/v4`

- [ ] **Step 2: Write the CLI command registry**

Create `cmd/cli/commands/registry.go`:
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

- [ ] **Step 3: Write the CLI entrypoint**

Create `cmd/cli/main.go`:
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

- [ ] **Step 4: Write the failing migration runner test**

Create `internal/migration/runner_test.go`:
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

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./internal/migration/... -v`
Expected: FAIL — package does not compile (`migration.MigratePlatformUp` undefined; also no `platform/*.sql` files yet for `//go:embed` to find).

- [ ] **Step 6: Write the platform migration SQL files**

Create `internal/migration/platform/000001_create_updated_at_trigger_fn.up.sql`:
```sql
CREATE OR REPLACE FUNCTION platform.trigger_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Create `internal/migration/platform/000001_create_updated_at_trigger_fn.down.sql`:
```sql
DROP FUNCTION IF EXISTS platform.trigger_set_updated_at();
```

Create `internal/migration/platform/000002_create_tenants_table.up.sql`:
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

Create `internal/migration/platform/000002_create_tenants_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.tenants;
```

Create `internal/migration/platform/000003_create_platform_users_table.up.sql`:
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

Create `internal/migration/platform/000003_create_platform_users_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.users;
```

Create `internal/migration/platform/000004_create_platform_refresh_tokens_table.up.sql`:
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

Create `internal/migration/platform/000004_create_platform_refresh_tokens_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.refresh_tokens;
```

Create `internal/migration/platform/000005_create_login_attempts_table.up.sql`:
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

Create `internal/migration/platform/000005_create_login_attempts_table.down.sql`:
```sql
DROP TABLE IF EXISTS platform.login_attempts;
```

- [ ] **Step 7: Implement the embed + runner (platform half)**

Create `internal/migration/platform_fs.go`:
```go
package migration

import "embed"

//go:embed platform/*.sql
var PlatformFS embed.FS
```

Create `internal/migration/runner.go`:

**Amendment (added during Task 14 execution):** `TenantSchemaVersion` (below)
originally instantiated a full `*migrate.Migrate`/postgres driver via
`newMigrate`/`WithInstance` just to read the current version. That driver
checks out a **dedicated `*sql.Conn` from the pool and holds it until an
explicit `Close()`** — which nothing on this read path ever called. Task 14
made this function back `middleware.TenantLookup`, i.e. it now runs on
every `RequireTenant` cache miss (Task 11) — so every cache miss leaked one
pooled connection permanently, exhausting the pool under load. Fixed by
querying `schema_migrations` directly instead of going through
golang-migrate's own machinery; `MigrateTenantUp` below (the rarer,
CLI-only, migration-*applying* path) is unaffected and still uses
`newMigrate` as before.

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

	"go-api-starter/internal/database"
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

- [ ] **Step 8: Write the CLI migrate-platform command**

Create `cmd/cli/commands/migrate.go`:
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
	fmt.Println("platform migrations applied")
	return nil
}
```

- [ ] **Step 9: Run tests to verify they pass**

Run: `go test ./internal/migration/... -v`
Expected: PASS — both tests pass against your local `app_test` database.

- [ ] **Step 10: Manually verify the CLI command**

Run: `go run ./cmd/cli migrate platform up`
Expected: prints `platform migrations applied`. Run it a second time — same output, no error (idempotent).

- [ ] **Step 11: Commit**

```bash
git add go.mod go.sum cmd/cli internal/migration
git commit -m "feat: CLI skeleton and platform schema migrations"
```

---

### Task 7: Tenant migrations

Tenant schema creation itself is **not** a migration file here either — provisioning (Task 13) issues `CREATE SCHEMA` directly before applying these, for the same golang-migrate reason as Task 6.

**Files:**
- Create: `internal/migration/tenant_fs.go`
- Create: `internal/migration/tenant/000001_create_updated_at_trigger_fn.up.sql` (+ `.down.sql`)
- Create: `internal/migration/tenant/000002_create_users_table.up.sql` (+ `.down.sql`)
- Create: `internal/migration/tenant/000003_create_roles_permissions_tables.up.sql` (+ `.down.sql`)
- Create: `internal/migration/tenant/000004_create_refresh_tokens_table.up.sql` (+ `.down.sql`)
- Modify: `internal/migration/runner.go` — add tenant-side functions
- Modify: `cmd/cli/commands/migrate.go` — add `migrate tenant up` and `migrate status`
- Test: Modify `internal/migration/runner_test.go` — add tenant coverage

**Interfaces:**
- Produces:
  - `migration.MigrateTenantUp(db *sql.DB, schemaName string) error`
  - `migration.TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error)`
  - `migration.LatestTenantVersion() uint`
  - `migration.TenantMigrationFiles() ([]MigrationFile, error)` — consumed by Task 13's provisioning.
  - CLI commands: `cli migrate tenant up --all`, `cli migrate tenant up --tenant=<code>`, `cli migrate status`

- [ ] **Step 1: Write the tenant migration SQL files**

Create `internal/migration/tenant/000001_create_updated_at_trigger_fn.up.sql`:
```sql
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

Create `internal/migration/tenant/000001_create_updated_at_trigger_fn.down.sql`:
```sql
DROP FUNCTION IF EXISTS trigger_set_updated_at();
```

Create `internal/migration/tenant/000002_create_users_table.up.sql`:
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

Create `internal/migration/tenant/000002_create_users_table.down.sql`:
```sql
DROP TABLE IF EXISTS users;
```

Create `internal/migration/tenant/000003_create_roles_permissions_tables.up.sql`:
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

Create `internal/migration/tenant/000003_create_roles_permissions_tables.down.sql`:
```sql
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
```

Create `internal/migration/tenant/000004_create_refresh_tokens_table.up.sql`:
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

Create `internal/migration/tenant/000004_create_refresh_tokens_table.down.sql`:
```sql
DROP TABLE IF EXISTS refresh_tokens;
```

- [ ] **Step 2: Add the tenant embed**

Create `internal/migration/tenant_fs.go`:
```go
package migration

import "embed"

//go:embed tenant/*.sql
var TenantFS embed.FS
```

- [ ] **Step 3: Extend the failing test**

Modify `internal/migration/runner_test.go` — add these two tests to the existing file (keep the two from Task 6):
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

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/migration/... -v`
Expected: FAIL — `migration.MigrateTenantUp` etc. undefined.

- [ ] **Step 5: Extend the runner**

**Amendment (added after this plan's implementation was complete, once the
suite was finally run against a real PostgreSQL database instead of only
ever skipping): the original `MigrateTenantUp` below is a serious bug.**
golang-migrate's `postgres.Config.SchemaName` option (used in `newMigrate`)
only scopes where golang-migrate stores its OWN `schema_migrations`
tracking table — it never sets `search_path` on the connection, so it does
nothing to scope the migration FILES' own SQL bodies. Tenant migration
files are intentionally written unqualified (no schema prefix), since the
tenant's schema name isn't known until provisioning time — that design
relies entirely on `search_path` being correctly pinned before the files
execute. Without an explicit fix, every tenant migration applied through
this function silently ran against `public` instead of the intended
tenant schema — invisible in every dry run because this function was never
once exercised against a live database until after all 22 tasks and the
final whole-branch review were already complete. A second tenant migrated
this way collided immediately with "relation already exists" against the
first tenant's tables in `public` — that's how it was caught. Platform
migrations are unaffected: their SQL fully qualifies every object with a
`platform.` prefix, so `MigratePlatformUp` never depended on `search_path`
in the first place. Tenant *provisioning* (`Provision`, Task 14) is also
unaffected — it applies migrations by hand inside its own transaction with
an explicit `SET LOCAL search_path`, not via this function at all. Only
`MigrateTenantUp` — i.e. `cli migrate tenant up`, the command that applies
*new* migrations to *already-provisioned* tenants — was broken.

The fix: `MigrateTenantUp` now takes a DSN string, not an already-open
`*sql.DB`, and opens its own dedicated, single-connection `*sql.DB` scoped
to the tenant's schema via the connection's `options` parameter (the
`options=-c search_path=<schema>` technique — verified working against
this project's actual pgx/v5 driver before being adopted). Every caller
that previously passed an open `db`/`pool.DB` now passes the DSN string
instead (`cfg.DB.DSN()` in the CLI, `testsupport.TestDSN()` — a new
exported helper — in tests).

Modify `internal/migration/runner.go` — append these functions at the end of the file:
```go
// MigrateTenantUp applies every unapplied migration under tenant/ to the
// given tenant schema, which must already exist (provisioning creates it
// before calling this). dsn (not an already-open *sql.DB) is required
// because this opens its own dedicated, single-connection *sql.DB scoped
// to schemaName via the connection's "options" parameter — a shared
// pooled *sql.DB can't be safely re-scoped per call. See the amendment
// note above for why this is necessary.
func MigrateTenantUp(dsn string, schemaName string) error {
	scopedDB, err := openScopedDB(dsn, schemaName)
	if err != nil {
		return fmt.Errorf("connect (scoped to %s): %w", schemaName, err)
	}
	defer scopedDB.Close()

	m, err := newMigrate(scopedDB, TenantFS, "tenant", schemaName)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// openScopedDB opens a dedicated, single-connection *sql.DB whose
// search_path is pinned to schemaName via the connection's "options"
// parameter (equivalent to `SET search_path TO schemaName` applied to
// every statement on this connection). Pinned to exactly one physical
// connection (SetMaxOpenConns(1)) so the setting can never be silently
// dropped by handing a later statement to a different pooled connection
// that never had it set. Callers must Close() the returned db when done.
func openScopedDB(dsn, schemaName string) (*sql.DB, error) {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	scopedDSN := dsn + sep + "options=-c%20search_path%3D" + url.QueryEscape(schemaName)
	db, err := sql.Open("pgx", scopedDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// TenantSchemaVersion reports the migration version currently applied to
// schemaName, and whether it's dirty (failed mid-migration). version is 0
// with dirty=false if no migration has ever been applied. schemaName must
// already have been provisioned (its schema_migrations table exists) —
// every caller of this function only reaches it for a tenant that
// Provision (Task 14) already created and migrated.
func TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error) {
	if !database.ValidSchemaName(schemaName) {
		return 0, false, fmt.Errorf("invalid schema name %q", schemaName)
	}
	query := `SELECT version, dirty FROM "` + schemaName + `".schema_migrations LIMIT 1`
	var v int64
	var d bool
	err = db.QueryRow(query).Scan(&v, &d)
	switch {
	case err == sql.ErrNoRows:
		return 0, false, nil
	case err != nil:
		return 0, false, err
	default:
		return uint(v), d, nil
	}
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

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/migration/... -v`
Expected: PASS — all four tests pass.

- [ ] **Step 7: Add the CLI tenant-migration commands**

Modify `cmd/cli/commands/migrate.go` — add to the `init()` block and append these functions:
```go
func init() {
	Register("migrate platform up", cmdMigratePlatformUp)
	Register("migrate tenant up", cmdMigrateTenantUp)
	Register("migrate status", cmdMigrateStatus)
}
```
(Replace the existing single-line `init()` from Task 6 with this three-line version.)

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

Add `"flag"` to the import block at the top of `cmd/cli/commands/migrate.go`.

- [ ] **Step 8: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 9: Commit**

```bash
git add internal/migration cmd/cli/commands/migrate.go
git commit -m "feat: tenant schema migrations and cli migrate tenant/status commands"
```

---

### Task 8: Password hashing + JWT + refresh token helpers

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/refreshtoken.go`
- Test: `internal/auth/password_test.go`
- Test: `internal/auth/jwt_test.go`
- Test: `internal/auth/refreshtoken_test.go`

**Interfaces:**
- Produces:
  - `auth.BcryptCost = 12`, `auth.HashPassword(plain string) (string, error)`, `auth.VerifyPassword(hash, plain string) bool`
  - `auth.ValidatePasswordStrength(plain string) *apperror.Error`
  - `auth.ScopePlatform = "platform"`, `auth.ScopeTenant = "tenant"`
  - `auth.Claims struct { jwt.RegisteredClaims; Scope string; TenantID string }`
  - `auth.NewTokenManager(secret string, accessTTL time.Duration) *TokenManager`
  - `(tm *TokenManager) IssueAccessToken(userID uuid.UUID, scope string, tenantID *uuid.UUID) (string, error)`
  - `(tm *TokenManager) Verify(tokenString string) (*Claims, error)`
  - `auth.GenerateRefreshToken() (plain string, hash string, err error)`
  - `auth.HashRefreshToken(plain string) string`

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get golang.org/x/crypto/bcrypt
go get github.com/golang-jwt/jwt/v5
```

- [ ] **Step 2: Write the failing password test**

Create `internal/auth/password_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/auth/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 4: Implement password.go**

Create `internal/auth/password.go`:
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

- [ ] **Step 5: Run password tests to verify they pass**

Run: `go test ./internal/auth/... -run Password -v`
Expected: PASS — all four tests pass.

- [ ] **Step 6: Write the failing JWT test**

Create `internal/auth/jwt_test.go`:
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

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/auth/... -run Token -v`
Expected: FAIL — `NewTokenManager` undefined.

- [ ] **Step 8: Implement jwt.go**

Create `internal/auth/jwt.go`:
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

- [ ] **Step 9: Run JWT tests to verify they pass**

Run: `go test ./internal/auth/... -run Token -v`
Expected: PASS — all four tests pass.

- [ ] **Step 10: Write the failing refresh token test**

Create `internal/auth/refreshtoken_test.go`:
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

- [ ] **Step 11: Run test to verify it fails**

Run: `go test ./internal/auth/... -run RefreshToken -v`
Expected: FAIL — `GenerateRefreshToken` undefined.

- [ ] **Step 12: Implement refreshtoken.go**

Create `internal/auth/refreshtoken.go`:
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

- [ ] **Step 13: Run all auth tests to verify they pass**

Run: `go test ./internal/auth/... -v`
Expected: PASS — all 9 tests across the three files pass.

- [ ] **Step 14: Commit**

```bash
git add go.mod go.sum internal/auth
git commit -m "feat: password hashing, JWT issuing/verification, refresh tokens"
```

---

### Task 9: Core middleware — request ID, structured logging, recover, CORS

**Design note on context composition:** every middleware that needs to enrich the request's `context.Context` must build on top of the *current* `c.UserContext()`, never `context.Background()` — otherwise it silently discards whatever an earlier middleware (like this task's logger) already attached. This convention is set here and Task 11's tenant resolver relies on it.

**Files:**
- Create: `internal/middleware/requestid.go`
- Create: `internal/middleware/logger.go`
- Create: `internal/middleware/recover.go`
- Create: `internal/middleware/cors.go`
- Test: `internal/middleware/requestid_test.go`
- Test: `internal/middleware/logger_test.go`
- Test: `internal/middleware/recover_test.go`
- Test: `internal/middleware/cors_test.go`

**Interfaces:**
- Produces:
  - `middleware.RequestID() fiber.Handler`, `middleware.RequestIDFromCtx(c *fiber.Ctx) string`
  - `middleware.Logger(base *slog.Logger) fiber.Handler`
  - `middleware.SetLoggerInCtx(c *fiber.Ctx, l *slog.Logger)`, `middleware.LoggerFromCtx(c *fiber.Ctx) *slog.Logger`, `middleware.LoggerFromContext(ctx context.Context) *slog.Logger`
  - `middleware.Recover() fiber.Handler`
  - `middleware.CORS(allowedOrigins []string) fiber.Handler`

- [ ] **Step 1: Add ULID dependency**

Run: `go get github.com/oklog/ulid/v2`

- [ ] **Step 2: Write the failing request ID test**

Create `internal/middleware/requestid_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/middleware/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 4: Implement requestid.go**

Create `internal/middleware/requestid.go`:
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

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/middleware/... -run RequestID -v`
Expected: PASS.

- [ ] **Step 6: Write the failing logger test**

Create `internal/middleware/logger_test.go`:
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

- [ ] **Step 7: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run Logger -v`
Expected: FAIL — `middleware.Logger` undefined.

- [ ] **Step 8: Implement logger.go**

Create `internal/middleware/logger.go`:

**Amendment (added during Task 9 execution):** the original version of this
file read `c.Response().StatusCode()` unconditionally after `c.Next()`
returned. That's wrong for every error/panic response: Fiber's global
`ErrorHandler` — the thing that actually writes the real status code to the
response — only runs after the *entire* `app.Use()` middleware chain has
already unwound, one level above any individual middleware. No reordering
of `Logger`/`Recover`/`RequestID` fixes this, since all three sit inside
that same chain. The fix below has `Logger` resolve the status from the
returned error itself (reusing the same `*apperror.Error.Status` field
`response.Error` already uses) whenever `err != nil`, and only trusts
`c.Response().StatusCode()` on the success path where no ErrorHandler
rewrite happens.

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

- [ ] **Step 9: Run test to verify it passes**

Run: `go test ./internal/middleware/... -run Logger -v`
Expected: PASS.

- [ ] **Step 10: Write the failing recover test**

Create `internal/middleware/recover_test.go`:
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

- [ ] **Step 11: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run Recover -v`
Expected: FAIL — `middleware.Recover` undefined.

- [ ] **Step 12: Implement recover.go**

Create `internal/middleware/recover.go`:
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

- [ ] **Step 13: Run test to verify it passes**

Run: `go test ./internal/middleware/... -run Recover -v`
Expected: PASS.

- [ ] **Step 14: Write the failing CORS test**

Create `internal/middleware/cors_test.go`:
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

- [ ] **Step 15: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run CORS -v`
Expected: FAIL — `middleware.CORS` undefined.

- [ ] **Step 16: Implement cors.go**

Create `internal/middleware/cors.go`:
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

- [ ] **Step 17: Run all middleware tests to verify they pass**

Run: `go test ./internal/middleware/... -v`
Expected: PASS — all 10 tests across the four files pass.

- [ ] **Step 18: Commit**

```bash
git add go.mod go.sum internal/middleware
git commit -m "feat: request ID, structured request logging, panic recovery, CORS"
```

---

### Task 10: Login rate limiting

`login_attempts` lives in the `platform` schema regardless of whether the attempt was a platform or tenant login — this service always talks to the platform-pinned pool, never `WithTenant`.

**Files:**
- Create: `internal/ratelimit/loginattempt.go`
- Modify: `internal/testsupport/testsupport.go` — add `OpenTestPlatformDB`
- Test: `internal/ratelimit/loginattempt_test.go`

**Interfaces:**
- Produces:
  - `ratelimit.NewLoginAttemptService(platformDB *sqlx.DB, maxAttempts int, window time.Duration) *LoginAttemptService`
  - `(s *LoginAttemptService) Check(ctx context.Context, scope string, tenantID *uuid.UUID, email string) error` — returns `apperror.RateLimited` when blocked, `nil` otherwise.
  - `(s *LoginAttemptService) Record(ctx context.Context, scope string, tenantID *uuid.UUID, email string, success bool) error`
  - `testsupport.OpenTestPlatformDB(t *testing.T) *sqlx.DB` — like `OpenTestDB` but pinned to `search_path=platform`, mirroring `database.NewPlatformPool`.

- [ ] **Step 1: Add the platform-pinned test helper**

Modify `internal/testsupport/testsupport.go` — add this function alongside `OpenTestDB`:
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

- [ ] **Step 2: Write the failing test**

Create `internal/ratelimit/loginattempt_test.go`:
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

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ratelimit/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 4: Implement loginattempt.go**

Create `internal/ratelimit/loginattempt.go`:
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

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ratelimit/... -v`
Expected: PASS — all four tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ratelimit internal/testsupport
git commit -m "feat: login attempt rate limiting backed by platform.login_attempts"
```

---

### Task 11: Auth middleware — `RequirePlatform` / `RequireTenant`, tenant resolver cache, migration guard

**Design note:** `TenantLookup` is declared here, on the consumer side, so `internal/middleware` never imports `modules/platform/tenant` — Task 13 implements the interface instead. The same lookup call reports both tenant status and tenant schema migration version in one round trip, because both are cached together with the same TTL and both gate the same decision ("can this request proceed against this tenant's data").

**Files:**
- Create: `internal/middleware/tenantresolver.go`
- Create: `internal/middleware/auth.go`
- Test: `internal/middleware/tenantresolver_test.go`
- Test: `internal/middleware/auth_test.go`

**Interfaces:**
- Produces:
  - `middleware.TenantRecord struct { TenantID uuid.UUID; SchemaName string; Status string; SchemaVersion uint; SchemaDirty bool }`
  - `middleware.TenantLookup interface { FindByID(ctx, tenantID uuid.UUID) (TenantRecord, error) }` — implemented by Task 13.
  - `middleware.NewTenantResolver(lookup TenantLookup, ttl time.Duration) *TenantResolver`
  - `(r *TenantResolver) Resolve(ctx, tenantID uuid.UUID) (TenantRecord, error)`
  - `(r *TenantResolver) Invalidate(tenantID uuid.UUID)`
  - `middleware.RequirePlatform(tm *auth.TokenManager) fiber.Handler`
  - `middleware.RequireTenant(tm *auth.TokenManager, resolver *TenantResolver) fiber.Handler`

- [ ] **Step 1: Write the failing tenant resolver test**

Create `internal/middleware/tenantresolver_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run TenantResolver -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement tenantresolver.go**

Create `internal/middleware/tenantresolver.go`:
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

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/middleware/... -run TenantResolver -v`
Expected: PASS — all three tests pass.

- [ ] **Step 5: Write the failing auth middleware test**

Create `internal/middleware/auth_test.go`:
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

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run RequirePlatform -v` and `go test ./internal/middleware/... -run RequireTenant -v`
Expected: FAIL — `middleware.RequirePlatform` / `RequireTenant` undefined.

- [ ] **Step 7: Implement auth.go**

Create `internal/middleware/auth.go`:
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

- [ ] **Step 8: Run all middleware tests to verify they pass**

Run: `go test ./internal/middleware/... -v`
Expected: PASS — every test in the package passes, including the 8 new ones from this task.

- [ ] **Step 9: Commit**

```bash
git add internal/middleware
git commit -m "feat: platform/tenant auth middleware with cached tenant resolution and migration guard"
```

---

### Task 12: Platform admin module — bootstrap + auth (login/refresh/logout)

**Files:**
- Create: `internal/modules/platform/auth/dto.go`
- Create: `internal/modules/platform/auth/service.go`
- Create: `internal/modules/platform/auth/handler.go`
- Create: `cmd/cli/commands/admin.go`
- Test: `internal/modules/platform/auth/service_test.go`

**Interfaces:**
- Produces:
  - `platformauth.LoginRequest { Email, Password string }`, `RefreshRequest { RefreshToken string }`, `LogoutRequest { RefreshToken string }`
  - `platformauth.LoginResponse { AccessToken, RefreshToken string; ExpiresIn int }`
  - `platformauth.NewService(platformDB *sqlx.DB, tokens *auth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service`
  - `(s *Service) Login(ctx, LoginRequest) (LoginResponse, error)`, `Refresh(ctx, RefreshRequest) (LoginResponse, error)`, `Logout(ctx, LogoutRequest) error`
  - `platformauth.NewHandler(svc *Service) *Handler` with `Login`, `Refresh`, `Logout` methods (`func(c *fiber.Ctx) error`), wired to routes in Task 18.
  - CLI command: `cli admin create --email=<e> --name=<n>` (bootstrap, no auth required).

- [ ] **Step 1: Write the failing service test**

Create `internal/modules/platform/auth/service_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/platform/auth/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement dto.go**

Create `internal/modules/platform/auth/dto.go`:
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

- [ ] **Step 4: Implement service.go**

Create `internal/modules/platform/auth/service.go`:
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

// dummyPasswordHash is a valid bcrypt hash of an unused, unguessable
// password. Login always runs exactly one VerifyPassword call — against
// the real user's hash when found, or against this fixed hash when not —
// so response timing never reveals whether an email exists/is active
// versus exists-with-a-wrong-password. (Amendment, added during Task 12
// execution: the original version below skipped VerifyPassword entirely
// via && short-circuiting whenever the user wasn't found or was inactive,
// creating a measurable timing side-channel for email enumeration.)
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
	// Always call VerifyPassword — on the real hash if found, on a fixed
	// dummy hash otherwise — so this line's cost (dominated by bcrypt) is
	// the same on every path regardless of whether the user exists.
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

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/modules/platform/auth/... -v`
Expected: PASS — all four tests pass.

- [ ] **Step 6: Implement the handler**

Create `internal/modules/platform/auth/handler.go`:
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

- [ ] **Step 7: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 8: Write the admin bootstrap CLI command**

Create `cmd/cli/commands/admin.go`:
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

	fmt.Println("admin created:")
	fmt.Println("  email:   ", *email)
	fmt.Println("  password:", password)
	fmt.Println("(this password is shown once — change it after your first login)")
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

- [ ] **Step 9: Manually verify**

Run:
```bash
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
```
Expected: prints a generated password. Copy it — Task 18 will let you actually log in with it once the HTTP server exists.

- [ ] **Step 10: Commit**

```bash
git add internal/modules/platform/auth cmd/cli/commands/admin.go
git commit -m "feat: platform admin auth (login/refresh/logout) and bootstrap CLI command"
```

---

### Task 13: Permission constants + sync mechanism + CLI

Built before tenant provisioning (next task) because provisioning needs `permission.All` to seed a brand new tenant's `permissions` table.

**Files:**
- Create: `internal/permission/constants.go`
- Create: `internal/permission/sync.go`
- Create: `cmd/cli/commands/permission.go`
- Test: `internal/permission/sync_test.go`

**Interfaces:**
- Produces:
  - `permission.UserView`, `UserCreate`, `UserUpdate`, `UserDelete`, `RoleView`, `RoleManage` (string constants)
  - `permission.All []string` — every constant above; the source of truth Task 14's provisioning seeds from.
  - `permission.Sync(ctx context.Context, db *sql.DB, schemaName string) (added int, err error)`
  - CLI command: `cli permission sync --all` / `--tenant=<code>`

- [ ] **Step 1: Write the failing test**

Create `internal/permission/sync_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/permission/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement constants.go**

Create `internal/permission/constants.go`:
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

- [ ] **Step 4: Implement sync.go**

Create `internal/permission/sync.go`:
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

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/permission/... -v`
Expected: PASS — both tests pass.

- [ ] **Step 6: Write the CLI command**

Create `cmd/cli/commands/permission.go`:
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

- [ ] **Step 7: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 8: Commit**

```bash
git add internal/permission cmd/cli/commands/permission.go
git commit -m "feat: permission constants and cli permission sync command"
```

---

### Task 14: Platform tenant module — provisioning, CRUD, CLI

**Design note — no `handler.go`:** the spec puts a self-service HTTP endpoint for provisioning explicitly **out of scope** for this starter ("endpoint admin self-service ... menyusul"). This task builds the reusable `Service` the future endpoint will call, and exposes it now only through the CLI. `modules/platform/tenant/` therefore has no `handler.go` — that arrives later, wired to this same `Service`, with zero changes to it.

**Files:**
- Create: `internal/modules/platform/tenant/model.go`
- Create: `internal/modules/platform/tenant/dto.go`
- Create: `internal/modules/platform/tenant/repository.go`
- Create: `internal/modules/platform/tenant/service.go`
- Create: `cmd/cli/commands/tenant.go`
- Test: `internal/modules/platform/tenant/service_test.go`

**Interfaces:**
- Produces:
  - `tenant.Tenant` (DB row struct), `tenant.ProvisionInput`, `tenant.ProvisionResult`, `tenant.TenantView`
  - `tenant.NewRepository(platformDB *sqlx.DB, rawDB *sql.DB) *Repository`
  - `(r *Repository) FindByID(ctx, uuid.UUID) (middleware.TenantRecord, error)` — **implements `middleware.TenantLookup`** (Task 11).
  - `(r *Repository) Create/FindByCode/FindByCodeIncludingDeleted/List/UpdateStatus/SoftDelete`
  - `tenant.NewService(repo *Repository, rawDB *sql.DB, resolver *middleware.TenantResolver) *Service`
  - `(s *Service) Provision(ctx, ProvisionInput) (ProvisionResult, error)`
  - `(s *Service) Suspend/Activate/Delete(ctx, code string) error`
  - `(s *Service) Purge(ctx, code string) error`
  - `(s *Service) List(ctx, pagination.Params) ([]TenantView, response.Meta, error)`
  - CLI: `cli tenant create --code= --name= --owner-email=`, `cli tenant list`, `cli tenant suspend --code=`, `cli tenant activate --code=`, `cli tenant delete --code=`, `cli tenant purge --code= --confirm`

- [ ] **Step 1: Write the failing test**

Create `internal/modules/platform/tenant/service_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/platform/tenant/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement model.go and dto.go**

Create `internal/modules/platform/tenant/model.go`:
```go
package tenant

import (
	"time"

	"github.com/google/uuid"
)

// Amendment (added during Task 14 execution): the original version of
// this struct had 9 fields, but platform.tenants (000002_create_tenants_
// table.up.sql) has 12 columns — it also carries created_by, updated_by,
// deleted_by. Every repository method below queries `SELECT *`, and sqlx
// runs in strict mode by default (no .Unsafe() call anywhere in this
// codebase) — a SELECT * into a struct missing destination columns fails
// every single call with "missing destination name created_by in *Tenant"
// at runtime. That broke every read path in this module, including
// Repository.FindByID, which is the middleware.TenantLookup implementation
// the auth middleware (Task 11) depends on for every tenant request.
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
	CreatedBy   *uuid.UUID `db:"created_by"`
	UpdatedBy   *uuid.UUID `db:"updated_by"`
	DeletedBy   *uuid.UUID `db:"deleted_by"`
}
```

Create `internal/modules/platform/tenant/dto.go`:
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

- [ ] **Step 4: Implement repository.go**

Create `internal/modules/platform/tenant/repository.go`:
```go
package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
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

// Amendment (added during Task 14 execution): added ORDER BY created_at
// DESC LIMIT 1. This query has no code-uniqueness constraint to lean on —
// deleted_at IS NULL isn't part of the WHERE clause here — so once a
// tenant is purged (Purge below now hard-deletes its row) and a new
// tenant re-provisions the same code, this can no longer return more than
// one candidate row for that code. Before this fix, an ORDER-less SELECT
// with more than one matching row (which could happen before the Purge
// fix below, when a purged tenant's row was left behind) could
// nondeterministically return the wrong one — including a live
// re-provisioned tenant instead of the intended old one — which is
// dangerous given this feeds Purge's DROP SCHEMA.
func (r *Repository) FindByCodeIncludingDeleted(ctx context.Context, code string) (Tenant, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t,
		`SELECT * FROM tenants WHERE code = $1 ORDER BY created_at DESC LIMIT 1`, code)
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
//
// Amendment (added during Task 14 execution): the original version mapped
// every error (including a transient connection failure) to
// apperror.NotFound, which made a real outage indistinguishable from "this
// tenant genuinely doesn't exist" at the auth-middleware layer. Now only
// sql.ErrNoRows maps to NotFound; anything else is a real Internal error.
// Also added the same ValidSchemaName check WithTenant (Task 5) and Purge
// use before t.SchemaName reaches SQL, for defense-in-depth consistency.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (middleware.TenantRecord, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE id = $1 AND deleted_at IS NULL`, id)
	if err == sql.ErrNoRows {
		return middleware.TenantRecord{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}

	if !database.ValidSchemaName(t.SchemaName) {
		return middleware.TenantRecord{}, apperror.Internal(fmt.Errorf("invalid schema name %q", t.SchemaName))
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

- [ ] **Step 5: Implement service.go**

Create `internal/modules/platform/tenant/service.go`:
```go
package tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

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

	// Amendment (added during Task 14 execution): a soft-deleted (but not
	// yet purged) tenant still owns its schema on disk even though its
	// code is free again for a new row (tenants_code_key is a partial
	// unique index on deleted_at IS NULL). Without this check, Provision
	// would pass the INSERT below, then fail deep inside the transaction
	// at CREATE SCHEMA with a confusing "already exists" error. Checking
	// up front surfaces a clear conflict instead. An active/provisioning
	// row with this code is deliberately NOT rejected here — it falls
	// through to the INSERT below, which is where the real uniqueness
	// constraint lives and gets classified correctly.
	if existing, findErr := s.repo.FindByCodeIncludingDeleted(ctx, code); findErr == nil && existing.DeletedAt != nil {
		return ProvisionResult{}, apperror.Conflict("kode tenant ini pernah dipakai dan belum di-purge — tunggu purge selesai atau gunakan kode lain")
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
		// Amendment (added during Task 14 execution): the original code
		// mapped every insert error to "kode tenant sudah dipakai",
		// including a missing table, a lost connection, or any other real
		// failure — hiding the actual cause from both logs and the
		// operator. Only a genuine unique-constraint violation on
		// tenants_code_key (SQLSTATE 23505) is a duplicate code; anything
		// else is a real Internal error.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ProvisionResult{}, apperror.Conflict("kode tenant sudah dipakai")
		}
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("insert tenant row: %w", err))
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
//
// Amendment (added during Task 14 execution): the original version
// dropped the schema but left the platform.tenants row behind. Because
// tenants_code_key/tenants_schema_name_key are partial unique indexes
// (WHERE deleted_at IS NULL), that stale row made its code reusable —
// re-provisioning the same code, then a second (accidental or delayed)
// Purge call on that code, could destroy the newly re-provisioned, LIVE
// tenant's schema instead of the original one, because
// FindByCodeIncludingDeleted had no way to distinguish them. Purge now
// hard-deletes the row in the same transaction as the DROP SCHEMA — both
// are DDL/DML PostgreSQL can roll back together — so a purged code can
// never again resolve to more than one row, closing that path entirely
// (paired with the ORDER BY fix on FindByCodeIncludingDeleted above,
// which is now also redundant as a safety net rather than load-bearing).
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

	tx, err := s.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Internal(err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DROP SCHEMA IF EXISTS "`+t.SchemaName+`" CASCADE`); err != nil {
		return apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM platform.tenants WHERE id = $1`, t.ID); err != nil {
		return apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
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

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/modules/platform/tenant/... -v`
Expected: PASS — all five tests pass. (These are slow — each provisions a real schema with four tenant migrations — a few seconds total is normal.)

- [ ] **Step 7: Write the CLI tenant commands**

Create `cmd/cli/commands/tenant.go`:
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

	fmt.Println("tenant provisioned:")
	fmt.Println("  code:          ", result.Tenant.Code)
	fmt.Println("  owner email:   ", *ownerEmail)
	fmt.Println("  owner password:", result.OwnerPassword)
	fmt.Println("(this password is shown once — change it after first login)")
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
	fmt.Println("tenant permanently purged")
	return nil
}
```

- [ ] **Step 8: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 9: Manually verify end to end**

Run:
```bash
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acmecorp.test
go run ./cmd/cli tenant list
go run ./cmd/cli tenant suspend --code=acme_corp
go run ./cmd/cli tenant list
go run ./cmd/cli tenant activate --code=acme_corp
```
Expected: each command prints a sensible result; `tenant list` shows the status changing between `active` and `suspended`.

- [ ] **Step 10: Commit**

```bash
git add internal/modules/platform/tenant cmd/cli/commands/tenant.go
git commit -m "feat: tenant provisioning service, lifecycle management, and cli tenant commands"
```

---

### Task 15: RBAC — `RequirePermission` middleware, permission cache, tenant role service

Two halves, same split as Task 11: `middleware.PermissionChecker` is declared consumer-side (`internal/middleware`); `modules/tenant/role` implements it against real tenant tables. This is also where the `modules/tenant/role` package from the File Structure gets built — it wasn't a separate task because its only job is being this interface's implementation.

**Files:**
- Create: `internal/middleware/rbac.go`
- Create: `internal/modules/tenant/role/service.go`
- Test: `internal/middleware/rbac_test.go`
- Test: `internal/modules/tenant/role/service_test.go`

**Interfaces:**
- Produces:
  - `middleware.PermissionChecker interface { HasPermission(ctx, userID uuid.UUID, permCode string) (bool, error) }`
  - `middleware.NewPermissionCache(checker PermissionChecker, ttl time.Duration) *PermissionCache`
  - `(c *PermissionCache) HasPermission(ctx, tenantID, userID uuid.UUID, permCode string) (bool, error)`
  - `middleware.RequirePermission(permCode string, cache *PermissionCache) fiber.Handler` — must run after `RequireTenant`.
  - `role.NewService(db *database.DB) *Service` — **implements `middleware.PermissionChecker`**.

- [ ] **Step 1: Write the failing RBAC middleware test**

Create `internal/middleware/rbac_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/middleware/... -run "PermissionCache|RequirePermission" -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement rbac.go**

Create `internal/middleware/rbac.go`:
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/middleware/... -run "PermissionCache|RequirePermission" -v`
Expected: PASS — all four tests pass.

- [ ] **Step 5: Write the failing role service test**

Create `internal/modules/tenant/role/service_test.go`:
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

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/modules/tenant/role/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 7: Implement service.go**

Create `internal/modules/tenant/role/service.go`:
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

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/modules/tenant/role/... -v`
Expected: PASS — both tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/middleware internal/modules/tenant/role
git commit -m "feat: RBAC middleware with permission cache and tenant role service"
```

---

### Task 16: Tenant auth module — login (with `tenant_code`), refresh, logout

**Design notes:**
- Every tenant-auth request body carries `tenant_code` — including refresh and logout — because a bare refresh token doesn't say which tenant's schema its row lives in, and there's no token yet to carry that (that's exactly what login is establishing). The client keeps `tenant_code` alongside its tokens.
- Login is the one place tenant identity is established **without** a prior `RequireTenant` pass — there's no token yet — so the service injects `database.TenantInfo` into `ctx` itself, by hand, before calling `WithTenant`.
- The tenant lookup interface returns `middleware.TenantRecord` (reused, not a new type) so this module doesn't need its own tenant-shape struct.

**Files:**
- Create: `internal/modules/tenant/auth/dto.go`
- Create: `internal/modules/tenant/auth/service.go`
- Create: `internal/modules/tenant/auth/handler.go`
- Modify: `internal/modules/platform/tenant/repository.go` — add `FindRecordByCode`
- Modify: `internal/testsupport/testsupport.go` — add exported `SeedInSchema`
- Test: `internal/modules/tenant/auth/service_test.go`

**Interfaces:**
- Produces:
  - `tenantauth.LoginRequest { TenantCode, Email, Password string }`, `RefreshRequest { TenantCode, RefreshToken string }`, `LogoutRequest { TenantCode, RefreshToken string }`
  - `tenantauth.LoginResponse { AccessToken, RefreshToken string; ExpiresIn int }`
  - `tenantauth.TenantLookup interface { FindRecordByCode(ctx, code string) (middleware.TenantRecord, error) }`
  - `tenantauth.NewService(db *database.DB, tenants TenantLookup, tokens *auth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service`
  - `(s *Service) Login/Refresh/Logout` — same shape as Task 12's platform equivalent.
  - `tenantauth.NewHandler(svc *Service) *Handler` with `Login`, `Refresh`, `Logout` (wired to routes in Task 18).
  - `(r *tenant.Repository) FindRecordByCode(ctx, code string) (middleware.TenantRecord, error)` — implements `tenantauth.TenantLookup`.
  - `testsupport.SeedInSchema(t, pool *sqlx.DB, schema, query string, args ...any)`

- [ ] **Step 1: Add the shared seed helper to testsupport**

Modify `internal/testsupport/testsupport.go` — add:
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

- [ ] **Step 2: Add `FindRecordByCode` to the platform tenant repository**

Modify `internal/modules/platform/tenant/repository.go` — add this method, reusing `FindByCode` and the same migration-version lookup `FindByID` already does:
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

- [ ] **Step 3: Run existing tests to verify nothing broke**

Run: `go test ./internal/modules/platform/tenant/... -v`
Expected: PASS — the five tests from Task 14 still pass; this was an additive change.

- [ ] **Step 4: Write the failing service test**

Create `internal/modules/tenant/auth/service_test.go`:
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

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./internal/modules/tenant/auth/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 6: Implement dto.go**

Create `internal/modules/tenant/auth/dto.go`:
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

- [ ] **Step 7: Implement service.go**

Create `internal/modules/tenant/auth/service.go`:
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

// dummyPasswordHash is a valid bcrypt hash of an unused, unguessable
// password. Login always runs exactly one VerifyPassword call — against
// the real user's hash when found and active, or against this fixed hash
// when not — so response timing never reveals whether an email
// exists/is active versus exists-with-a-wrong-password. Same pattern and
// same rationale as Task 12's platform admin auth service, duplicated
// here because it's a different package.
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

	// Amendment (added alongside the identical Task 12 fix, for the same
	// reason): always call VerifyPassword — on the real hash if found and
	// active, on the fixed dummyPasswordHash otherwise — so this line's
	// cost is the same on every path. Without this, && short-circuiting
	// skips the bcrypt call whenever the user isn't found/inactive,
	// letting response timing leak which emails exist. This package
	// defines its own dummyPasswordHash/mustHashDummyPassword (same
	// pattern as Task 12's platform auth service, but not the same
	// variable — different package) — add them alongside Service below.
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

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/modules/tenant/auth/... -v`
Expected: PASS — all four tests pass.

- [ ] **Step 9: Implement the handler**

Create `internal/modules/tenant/auth/handler.go`:
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

- [ ] **Step 10: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 11: Commit**

```bash
git add internal/modules/tenant/auth internal/modules/platform/tenant/repository.go internal/testsupport
git commit -m "feat: tenant auth (login/refresh/logout) with tenant_code resolution"
```

---

### Task 17: Tenant `user` module — the example pattern

This is the module every future tenant-scoped module (`product`, `order`, whatever a project turned on this starter needs) copies. Every convention the starter established gets demonstrated here: `WithTenant` for every query, soft delete, `*_by` audit columns from `database.ActorFromContext`, the pagination sort whitelist, and the handler → service → repository shape.

**Files:**
- Create: `internal/modules/tenant/user/model.go`
- Create: `internal/modules/tenant/user/dto.go`
- Create: `internal/modules/tenant/user/repository.go`
- Create: `internal/modules/tenant/user/service.go`
- Create: `internal/modules/tenant/user/handler.go`
- Test: `internal/modules/tenant/user/repository_test.go`

**Interfaces:**
- Produces:
  - `user.User` (DB row struct), `user.CreateRequest`, `user.UpdateRequest`, `user.View`
  - `user.Sortable map[string]string` — the whitelist template for every future module.
  - `user.NewRepository(db *database.DB) *Repository` with `Create/FindByID/List/Update/Delete`
  - `user.NewService(repo *Repository) *Service` with `Create/Get/List/Update/Delete`
  - `user.NewHandler(svc *Service) *Handler` with `Create/Get/List/Update/Delete` (`func(c *fiber.Ctx) error`), wired to routes in Task 18 behind `RequireTenant` + `RequirePermission`.

- [ ] **Step 1: Write the failing repository test**

Create `internal/modules/tenant/user/repository_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/modules/tenant/user/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement model.go and dto.go**

Create `internal/modules/tenant/user/model.go`:
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

Create `internal/modules/tenant/user/dto.go`:
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

- [ ] **Step 4: Implement repository.go**

Create `internal/modules/tenant/user/repository.go`:
```go
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

// Amendment (added during Task 17 execution): the original version of
// this function collapsed every INSERT failure into "email sudah dipakai"
// — the same misclassification bug Task 14's review found and fixed for
// tenant provisioning (a missing table, a lost connection, or any other
// real failure would have been misreported as a duplicate email, hiding
// the actual cause). Because this module is the copy-paste template every
// future tenant-scoped module is built from, fixing the pattern here
// (rather than letting it ship and get copied forward) matters more than
// in an isolated module — this is checked via the same actual-Postgres-
// error-code technique Task 14 uses, not a broadened assumption.
func (r *Repository) Create(ctx context.Context, u User) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
			u.ID, u.Email, u.PasswordHash, u.Name, u.IsActive, actor.UserID)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("email sudah dipakai")
		}
		return apperror.Internal(fmt.Errorf("insert user: %w", err))
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

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/modules/tenant/user/... -v`
Expected: PASS — all six tests pass.

- [ ] **Step 6: Implement service.go**

Create `internal/modules/tenant/user/service.go`:
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

- [ ] **Step 7: Implement handler.go**

Create `internal/modules/tenant/user/handler.go`:
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

- [ ] **Step 8: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 9: Commit**

```bash
git add internal/modules/tenant/user
git commit -m "feat: tenant user module — the example CRUD pattern for future modules"
```

---

### Task 18: HTTP server wiring — router, health checks, graceful shutdown, `main.go`

Full route-behavior testing (auth → RBAC → tenant isolation, end to end over real HTTP) is deliberately left to Task 20's integration test, which exercises this exact router. This task's own test only covers the health endpoints, which are simple enough to verify in isolation; everything else is verified by building and by the manual smoke test in Step 5.

**Files:**
- Create: `internal/server/router.go`
- Create: `internal/server/health.go`
- Create: `cmd/api/main.go`
- Test: `internal/server/health_test.go`

**Interfaces:**
- Produces:
  - `server.Dependencies` struct — every handler and cache `NewRouter` needs.
  - `server.NewRouter(deps Dependencies, corsOrigins []string) *fiber.App`
  - `server.RegisterHealth(app *fiber.App, pool *sqlx.DB)`
  - Routes: `POST /api/v1/admin/auth/{login,refresh,logout}`, `POST /api/v1/auth/{login,refresh,logout}`, `POST|GET /api/v1/users`, `GET|PATCH|DELETE /api/v1/users/:id` (all five behind `RequireTenant` + `RequirePermission`).

- [ ] **Step 1: Write the failing health test**

Create `internal/server/health_test.go`:
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

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/... -v`
Expected: FAIL — package does not compile.

- [ ] **Step 3: Implement health.go**

Create `internal/server/health.go`:
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

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/... -v`
Expected: PASS — both tests pass.

- [ ] **Step 5: Implement router.go**

Create `internal/server/router.go`:
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

- [ ] **Step 6: Implement main.go**

Create `cmd/api/main.go`:
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

- [ ] **Step 7: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 8: Manually smoke-test the whole stack**

Run:
```bash
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acmecorp.test
go run ./cmd/api
```
In another terminal:
```bash
curl http://localhost:8080/health
curl http://localhost:8080/health/ready
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"tenant_code":"acme_corp","email":"owner@acmecorp.test","password":"<owner password from tenant create>"}'
```
Expected: `/health` and `/health/ready` return `{"status":"ok"}` / `{"status":"ready"}`; login returns `access_token`, `refresh_token`, `expires_in`. Copy the `access_token` and confirm RBAC works — the owner (`is_system` bypass) can list users:
```bash
curl http://localhost:8080/api/v1/users -H "Authorization: Bearer <access_token>"
```
Expected: `{"success":true,"data":[...],"meta":{...}}` — at least the owner user created during provisioning.

- [ ] **Step 9: Commit**

```bash
git add internal/server cmd/api
git commit -m "feat: HTTP server wiring — router, health checks, graceful shutdown"
```

---

### Task 19: `cli cleanup` command

Deletes expired/revoked refresh tokens (platform and every tenant schema) and old `login_attempts` rows (platform only — tenant schemas never got their own `login_attempts` table; the spec keeps rate-limit history in one place regardless of scope). Meant to run on a schedule via systemd timer (wired up in Task 21). Tenant purge's 30-day safety window is already built into `Service.Purge` (Task 14) — nothing left to add there.

No automated test for this file, consistent with the other CLI command files in this plan (`migrate.go`, `admin.go`, `tenant.go`, `permission.go`) — their correctness rides on the already-tested services underneath; this task is verified manually.

**Files:**
- Create: `cmd/cli/commands/cleanup.go`

**Interfaces:**
- Produces: CLI command `cli cleanup`

- [ ] **Step 1: Implement cleanup.go**

Create `cmd/cli/commands/cleanup.go`:
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
	fmt.Println("platform: cleaned")

	targets, err := tenantSchemasToMigrate(db, true, "") // shared helper from migrate.go
	if err != nil {
		return err
	}
	for _, tgt := range targets {
		if err := cleanupTenantSchema(db, tgt.schemaName); err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s cleaned\n", tgt.code)
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

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: succeeds.

- [ ] **Step 3: Manually verify**

Run:
```bash
go run ./cmd/cli cleanup
```
Expected: prints `platform: cleaned` followed by one line per tenant. Running it against a freshly provisioned tenant with no expired tokens yet is a no-op — that's expected, not a bug.

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/commands/cleanup.go
git commit -m "feat: cli cleanup command for expired refresh tokens and login attempts"
```

---

### Task 20: Integration test — tenant isolation, end to end over real HTTP

The test the spec calls the most important one in the repo: two tenants with **the same** data (same email address, on purpose) verified never to cross-contaminate — provisioned for real, logged into for real, driven entirely through the real HTTP router built by Task 18. Task 5 already proved `WithTenant` itself can't leak `search_path` across a rollback at the database layer; this test proves the same guarantee holds through the full stack a real client actually talks to.

**Files:**
- Test: `internal/server/tenant_isolation_test.go`

**Interfaces:**
- Consumes everything built in Tasks 5–18: no new production code, this task is pure test.

- [ ] **Step 1: Write the test**

Create `internal/server/tenant_isolation_test.go`:
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

	// Amendment (added during Task 20 execution): positive control before
	// the negative check below. Without this, a 404 on the cross-tenant
	// fetch is ambiguous — it could mean "isolated" (the intended
	// guarantee) or "the route/handler is broken" (a false pass that
	// would look identical). Tenant A fetching its own user proves the
	// route and handler work at all, so the negative check that follows
	// is a real signal, not a coincidence.
	respOwnFetch, bodyOwnFetch := doJSON(t, app, http.MethodGet, "/api/v1/users/"+staffAID, nil, tokenA)
	if respOwnFetch.StatusCode != 200 {
		t.Fatalf("tenant A failed to fetch its own user by ID: status=%d body=%v", respOwnFetch.StatusCode, bodyOwnFetch)
	}

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

- [ ] **Step 2: Run the test**

Run: `go test ./internal/server/... -run TenantIsolation -v`
Expected: PASS. This test is slow (two full provisions, each running four tenant migrations) — several seconds is normal.

If it fails, **stop and treat it as the highest-priority bug in the whole codebase** — this is the one guarantee the entire architecture exists to provide.

- [ ] **Step 3: Run the full test suite**

Run: `go test ./... -v`
Expected: every test written across Tasks 2–20 passes.

- [ ] **Step 4: Commit**

```bash
git add internal/server/tenant_isolation_test.go
git commit -m "test: end-to-end tenant isolation across two tenants with identical data"
```

---

### Task 21: Deployment artifacts, `CLAUDE.md`, and documentation

Docs are written now, once, against the finished code — not earlier and incrementally against a moving target. That's a deliberate choice: the spec's rule is "documentation always matches the code," and writing it after Task 20 is the only point in this plan where that's true by construction. (`README.md` stays out of scope per the spec — it's written after the starter is used to build something real.)

**Files:**
- Create: `deploy/app-api.service`
- Create: `deploy/app-cleanup.service`
- Create: `deploy/app-cleanup.timer`
- Create: `deploy/pg-backup.service`
- Create: `deploy/pg-backup.timer`
- Create: `deploy/pg-backup.sh`
- Create: `deploy/Caddyfile`
- Create: `deploy/deploy.sh`
- Create: `CLAUDE.md`
- Create: `docs/getting-started.md`
- Create: `docs/adding-a-module.md`
- Create: `docs/multi-tenancy.md`
- Create: `docs/database-conventions.md`
- Create: `docs/deployment.md`

- [ ] **Step 1: Write the systemd units**

Create `deploy/app-api.service`:
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

Create `deploy/app-cleanup.service`:
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

Create `deploy/app-cleanup.timer`:
```ini
[Unit]
Description=Daily cli cleanup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Create `deploy/pg-backup.service`:
```ini
[Unit]
Description=PostgreSQL backup

[Service]
Type=oneshot
User=app
EnvironmentFile=/etc/app/api.env
ExecStart=/opt/app/deploy/pg-backup.sh
```

Create `deploy/pg-backup.timer`:
```ini
[Unit]
Description=Daily PostgreSQL backup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

- [ ] **Step 2: Write the backup script**

Create `deploy/pg-backup.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/backups/app-db"
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
FILE="$BACKUP_DIR/appdb-$TIMESTAMP.dump"

pg_dump -Fc -h "${DB_HOST:-localhost}" -U "${DB_USER:-app}" "${DB_NAME:-appdb}" > "$FILE"

# Keep 35 days locally; the real retention policy lives wherever this gets
# copied to off-server.
find "$BACKUP_DIR" -name "appdb-*.dump" -mtime +35 -delete

echo "backup written: $FILE"
echo "REMINDER: this backup is only useful if it is copied off this server"
echo "          (rsync/rclone to remote storage) and restore has been tested."
```

- [ ] **Step 3: Write the Caddy config**

Create `deploy/Caddyfile`:
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

- [ ] **Step 4: Write the deploy script**

Create `deploy/deploy.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

cd /opt/app

echo "==> pulling latest code"
git pull

echo "==> backing up current binaries"
cp -f api api.backup 2>/dev/null || true
cp -f cli cli.backup 2>/dev/null || true

echo "==> building"
go build -o api ./cmd/api
go build -o cli ./cmd/cli

echo "==> migrating platform schema"
./cli migrate platform up

echo "==> migrating tenant schemas"
./cli migrate tenant up --all

echo "==> syncing permissions"
./cli permission sync --all

echo "==> restarting service"
sudo systemctl restart app-api

echo "==> done"
```

Mark it executable:
```bash
chmod +x deploy/deploy.sh deploy/pg-backup.sh
```

- [ ] **Step 5: Write `CLAUDE.md`**

Create `CLAUDE.md`:
```markdown
# CLAUDE.md

Rules for working in this repository. Breaking any of these has a real
consequence — usually data leaking across tenants — not just a style nit.

## Multi-tenancy (see `docs/multi-tenancy.md` for the full explanation)

- **Every tenant data query goes through `database.WithTenant(ctx, fn)`.**
  No repository holds a bare `*sqlx.DB` for tenant tables. No exceptions.
- **`*fiber.Ctx` never leaves the handler layer.** Services and
  repositories take `context.Context` only. Fiber is built on `fasthttp`,
  which reuses request objects across requests — anything that escapes the
  handler with a live `*fiber.Ctx` reference can end up reading a
  different request's (possibly a different tenant's) data.
- **Every `sort` query parameter is validated against an explicit
  whitelist** (`pagination.Parse` + a module's `Sortable` map) before it's
  interpolated into `ORDER BY`. Never pass a raw query value into SQL,
  even indirectly.

## Module boundaries

- **A module never calls another module's repository directly.**
  Cross-module interaction goes through the other module's *service*.
- New tenant-scoped modules copy the shape of
  `internal/modules/tenant/user/` — see `docs/adding-a-module.md`.

## Errors and responses

- Services return `*apperror.Error`, never a bare `error` wrapping a
  driver error straight to the client.
- Handlers never call `c.Status(...)` themselves — `response.Success` /
  `response.Error` own that.

## Database

- IDs are generated in Go (`uuid.New()`), never as a DB-side default —
  see `docs/database-conventions.md` for why.
- New tables follow the audit-column convention in
  `docs/database-conventions.md`, unless they're a join table or a
  log/token table (the two documented exceptions).

## Testing

- Integration tests use `testsupport.OpenTestDB` / `OpenTestPlatformDB`,
  backed by a local `app_test` PostgreSQL database — never Docker, never
  testcontainers.
- Tests that create a schema must drop it in `t.Cleanup`.
```

- [ ] **Step 6: Write `docs/getting-started.md`**

Create `docs/getting-started.md`:
```markdown
# Getting Started (Windows development)

## 1. Install prerequisites

- Go 1.25+
- PostgreSQL, running locally
- `air` for hot reload: `go install github.com/air-verse/air@latest`

## 2. Create the databases

```powershell
psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE appdb OWNER app;"
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```

## 3. Configure environment

```powershell
copy .env.example .env
copy .env.test.example .env.test
```
Adjust `DB_PASSWORD` in both files if you used a different password above.

## 4. Run migrations and create your first admin

```powershell
go run ./cmd/cli migrate platform up
go run ./cmd/cli admin create --email=you@example.com --name="Your Name"
```
Copy the printed password — it's shown once.

## 5. Create your first tenant

```powershell
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test
```
Copy the printed owner password too.

## 6. Run the API

```powershell
air
```
or without hot reload:
```powershell
go run ./cmd/api
```

## 7. Try it

```powershell
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<owner password>\"}"
```

## Running tests

```powershell
go test ./...
```
Integration tests connect to `app_test` using `.env.test` and are skipped
automatically if PostgreSQL isn't reachable.
```

- [ ] **Step 7: Write `docs/adding-a-module.md`**

Create `docs/adding-a-module.md`:
```markdown
# Adding a Module

Every tenant-scoped module follows the shape of
`internal/modules/tenant/user/` — copy that folder as your starting point.

## File shape

```
modules/tenant/<name>/
  model.go       # DB row struct(s), `db:"..."` tags
  dto.go         # request/response structs, `json:"..."` + `validate:"..."` tags
  repository.go  # SQL via sqlx, always through database.WithTenant
  service.go     # business logic, context.Context only, no HTTP types
  handler.go     # Fiber only, converts to/from DTOs, never touches SQL
```

## Steps

1. **Add a migration.** Create
   `internal/migration/tenant/0000N_create_<name>_table.up.sql` (and
   `.down.sql`), following `database-conventions.md` for audit columns and
   soft delete. Bump the version number.
2. **Write `model.go`** — one struct per table, `db:"..."` tags matching
   column names exactly.
3. **Write `repository.go`** — every method wraps its query in
   `db.WithTenant(ctx, func(tx *sqlx.Tx) error {...})`. Declare a
   `Sortable map[string]string` whitelist for anything the list endpoint
   can sort by — never accept a raw `sort` query value.
4. **Write `dto.go`** — request structs with `validate:"..."` tags, a
   `View` struct for responses, and a `toView` mapper.
5. **Write `service.go`** — depends only on the repository and
   `context.Context`. Password hashing, external calls, or cross-module
   calls (via another module's *service*, never its repository) go here.
6. **Write `handler.go`** — parse the body, call `validator.Validate`,
   call the service, translate the result with `response.Success` /
   `response.Error`. Never write `c.Status(...)` directly.
7. **Add permission constants** in `internal/permission/constants.go` for
   the new resource (`<name>.view`, `<name>.create`, ...), following the
   existing `user.*` ones.
8. **Wire routes** in `internal/server/router.go`, inside the `tenantAPI`
   group, each behind
   `middleware.RequirePermission(permission.YourConstant, deps.PermissionCache)`.
9. **Add the handler to `server.Dependencies`** and construct it in
   `cmd/api/main.go`.
10. **Run `cli permission sync --all`** after deploying, so existing
    tenants get the new permission rows.

## Sub-grouping modules

Keep `modules/tenant/` flat until it holds 7–8 modules and navigation gets
hard. Then group by domain, e.g. `modules/tenant/sales/order/`,
`modules/tenant/inventory/product/` — the module itself doesn't change
shape, only its path. **Modules must not call another module's repository
directly** — cross-module interaction always goes through the other
module's *service*.
```

- [ ] **Step 8: Write `docs/multi-tenancy.md`**

Create `docs/multi-tenancy.md`:
```markdown
# Multi-Tenancy Rules

These two rules are what keep tenant A's data from ever appearing in
tenant B's response. Breaking either one is a data breach, not a bug.

## Rule 1: all tenant data access goes through `database.WithTenant`

```go
// Right
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1`, id)
	})
	return u, err
}
```
```go
// Wrong — bypasses tenant scoping entirely
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.pool.Get(&u, `SELECT * FROM users WHERE id = $1`, id) // which tenant?!
	return u, err
}
```
`WithTenant` runs your query inside a transaction with `SET LOCAL
search_path` pinned to the calling tenant's schema — resolved from
`context.Context`, never from a request parameter. `SET LOCAL` is
automatically cleared when the transaction ends (commit or rollback), so a
connection can never carry one tenant's schema into the next caller that
borrows it from the pool.

## Rule 2: `*fiber.Ctx` stops at the handler

Fiber is built on `fasthttp`, which **reuses** its context objects between
requests for performance. If a `*fiber.Ctx` (or anything derived from it)
escapes into a goroutine or gets stored anywhere, it can be silently
overwritten by a completely different request — including a different
tenant's — before it's read back.

```go
// Right — handler converts everything it needs before calling the service
func (h *Handler) Get(c *fiber.Ctx) error {
	id, _ := uuid.Parse(c.Params("id"))
	view, err := h.svc.Get(c.UserContext(), id) // context.Context only, from here down
	if err != nil {
		return response.Error(c, middleware.RequestIDFromCtx(c), err)
	}
	return response.Success(c, 200, view)
}
```
```go
// Wrong — never do this
func (h *Handler) Get(c *fiber.Ctx) error {
	go func() {
		log.Println(c.Params("id")) // c may belong to a different request by now
	}()
	return nil
}
```
Services and repositories only ever see `context.Context`. This isn't just
a safety rule — it also means business logic can be unit tested without
starting an HTTP server.

## What ends up in `context.Context`

Set by middleware, read via `database.TenantFromContext` /
`database.ActorFromContext`:
- `database.TenantInfo` — tenant ID + schema name, set by `RequireTenant`
- `database.Actor` — user ID + scope, set by `RequirePlatform` /
  `RequireTenant`

The one exception is login: there's no token yet to prove tenant identity,
so `modules/tenant/auth.Service.Login` constructs `TenantInfo` itself from
the resolved `tenant_code`, before calling `WithTenant`. Every other
tenant-scoped code path receives it already set by middleware.
```

- [ ] **Step 9: Write `docs/database-conventions.md`**

Create `docs/database-conventions.md`:
```markdown
# Database Conventions

## Every table (with two exceptions below)

```sql
created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at  TIMESTAMPTZ NULL
created_by  UUID NULL
updated_by  UUID NULL
deleted_by  UUID NULL
```

- `updated_at` is maintained by a per-schema trigger
  (`trigger_set_updated_at`) — attach it to every new table:
  ```sql
  CREATE TRIGGER set_updated_at
      BEFORE UPDATE ON your_table
      FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
  ```
- `*_by` columns are filled from `database.ActorFromContext(ctx)` in the
  repository layer, never left for the database to guess.
- IDs are `UUID PRIMARY KEY` with **no database default** — generate with
  `uuid.New()` in Go before inserting. (A `SET LOCAL search_path`
  transaction only searches the tenant's own schema, not `public`, so a
  DB-side default calling an extension function there would fail
  unpredictably. Generating in Go sidesteps the problem entirely.)

## Soft delete

Every query filters `WHERE deleted_at IS NULL` unless it's specifically
looking for deleted rows (like `tenant purge`'s 30-day check). Any column
with a uniqueness requirement needs a **partial** unique index so a
deleted row's value can be reused:
```sql
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;
```

## Exceptions

**Join tables** (`role_permissions`, `user_roles`): only `created_at` +
`created_by`. Access is revoked with a hard `DELETE`, not a soft delete —
stacking `deleted_at IS NULL` filters onto every permission check for no
benefit isn't worth it, and grant/revoke history belongs in logs, not in
the join table.

**Log/token tables** (`refresh_tokens`, `login_attempts`): only
`created_at`. They're append-only — nothing ever updates or soft-deletes a
row in them. A revoked refresh token is represented by its own
`revoked_at` column, not the standard `deleted_at`.

## Naming

- Tables: plural, `snake_case` (`users`, `role_permissions`).
- Columns: `snake_case`, no type prefixes (`email`, not `str_email`).
- Foreign keys: `<singular_table>_id` (`user_id`, `role_id`).
```

- [ ] **Step 10: Write `docs/deployment.md`**

Create `docs/deployment.md`:
```markdown
# Deployment

No Docker anywhere in this stack — Go compiles to one static binary, so
the server needs nothing beyond that binary, PostgreSQL, and Caddy.

## Layout on the server

```
/opt/app/
  api               # HTTP server binary
  cli               # CLI binary (migrations, provisioning, cleanup)
  api.backup        # previous binary, for instant rollback

/etc/app/api.env    # config, chmod 600
/etc/systemd/system/app-api.service
/etc/systemd/system/app-cleanup.service
/etc/systemd/system/app-cleanup.timer
```

## First-time setup

```bash
sudo useradd -r -s /usr/sbin/nologin app
sudo mkdir -p /opt/app /etc/app
sudo chown app:app /opt/app
git clone <your-repo-url> /opt/app
sudo cp deploy/app-api.service deploy/app-cleanup.service deploy/app-cleanup.timer /etc/systemd/system/
sudo cp .env.example /etc/app/api.env   # then edit it with real values
sudo chmod 600 /etc/app/api.env
sudo systemctl daemon-reload
sudo systemctl enable --now app-cleanup.timer
```

Install Caddy, then:
```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile   # edit the domain first
sudo systemctl reload caddy
```

### Ubuntu/Debian vs Rocky

Package manager differs (`apt install postgresql caddy` vs
`dnf install postgresql-server caddy`); everything else — systemd units,
the Go binary, the deploy script — is identical.

**Rocky-specific:** SELinux is enabled by default and blocks Caddy from
connecting to `localhost:8080` unless told otherwise:
```bash
sudo setsebool -P httpd_can_network_connect 1
```
Without this, requests through Caddy fail with a "Permission denied" that
has nothing obviously to do with SELinux — if Caddy can't reach the API on
Rocky, this is the first thing to check.

## Deploying an update

```bash
cd /opt/app
./deploy/deploy.sh
```
This pulls, backs up the current binaries, rebuilds, runs platform +
tenant migrations, syncs permissions, and restarts the service. Expect
1–2 seconds of downtime at the restart — schedule deploys outside business
hours for apps with fixed operating hours (like POS).

### Rollback

```bash
cp api.backup api && sudo systemctl restart app-api
```

## Backups

```bash
sudo cp deploy/pg-backup.service deploy/pg-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pg-backup.timer
```
This runs `pg_dump` daily and keeps 35 days locally. **Copy backups off
the server** (rsync/rclone to remote storage) — a backup on the same disk
that fails doesn't protect you. Test a restore periodically; an untested
backup is a guess, not a backup.

## Environment variables

See `.env.example` for the full list. All of them are readable/writable
without a rebuild — edit `/etc/app/api.env` and `systemctl restart app-api`.
```

- [ ] **Step 11: Commit**

```bash
git add deploy CLAUDE.md docs/getting-started.md docs/adding-a-module.md docs/multi-tenancy.md docs/database-conventions.md docs/deployment.md
git commit -m "docs: deployment artifacts, CLAUDE.md, and full documentation set"
```

---

### Task 22: OpenAPI documentation via `swaggo/swag`

Closes the one item from the spec's stack table with no task behind it yet: `swaggo/swag` generates OpenAPI docs from comments directly above each handler, so the docs can't drift far from the code without `swag init` visibly failing to compile against it.

**Files:**
- Modify: `cmd/api/main.go` — add general API annotations
- Modify: `internal/modules/platform/auth/handler.go` — add annotations
- Modify: `internal/modules/tenant/auth/handler.go` — add annotations
- Modify: `internal/modules/tenant/user/handler.go` — add annotations
- Modify: `internal/server/router.go` — serve the generated spec
- Modify: `Makefile` — add `make swagger`
- Create: `docs/swagger/` (generated, not hand-written)

- [ ] **Step 1: Install the swag CLI and add the runtime dependency**

Run:
```bash
go install github.com/swaggo/swag/cmd/swag@latest
go get github.com/gofiber/swagger
go get github.com/swaggo/swag
```

- [ ] **Step 2: Add general API annotations to `main.go`**

Modify `cmd/api/main.go` — add this comment block immediately above `package main`:

**Amendment (added during Task 22 execution):** the original block below
was missing the `@securitydefinitions.apikey` annotation. Five endpoints
(Task 22 Step 5, the tenant `users` handlers) carry `@Security BearerAuth`,
but without a matching `securityDefinitions` declaration somewhere in the
annotated source, `swag init` emits a Swagger 2.0 document that references
an undefined security scheme — invalid per spec, and in practice the
generated Swagger UI's "Authorize" button never appears, so protected
routes can't be exercised interactively from `/swagger/index.html`. The
three lines below (`@securitydefinitions.apikey`, `@in`, `@name`) are the
standard swaggo pattern for a bearer-token API and belong on the same
general-annotation block as `@title`/`@version`/`@BasePath`.

```go
// @title           App API
// @version         1.0
// @description     Multi-tenant API starter — schema-per-tenant PostgreSQL. Point of Sales is the first thing built on it, not a fixed part of it.
// @BasePath        /api/v1
// @securitydefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main
```

- [ ] **Step 3: Annotate the platform auth handlers**

Modify `internal/modules/platform/auth/handler.go` — add a comment block directly above each method:
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
(Replace the bare `func (h *Handler) Login(c *fiber.Ctx) error {` lines already in the file with these annotated versions — the method bodies underneath are unchanged from Task 12.)

- [ ] **Step 4: Annotate the tenant auth handlers**

Modify `internal/modules/tenant/auth/handler.go` the same way:
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

- [ ] **Step 5: Annotate the tenant user handlers**

Modify `internal/modules/tenant/user/handler.go`:
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

- [ ] **Step 6: Generate the spec**

Run:
```bash
swag init -g cmd/api/main.go -o docs/swagger
```
Expected: creates `docs/swagger/docs.go`, `swagger.json`, `swagger.yaml`.

- [ ] **Step 7: Serve it**

Modify `internal/server/router.go` — add to the import block:
```go
swaggerui "github.com/gofiber/swagger"
_ "go-api-starter/docs/swagger" // generated by `swag init`; registers the spec swaggerui reads
```
and add, right after `app := fiber.New(...)` and before the middleware chain:
```go
app.Get("/swagger/*", swaggerui.HandlerDefault)
```

- [ ] **Step 8: Add the Makefile target**

Modify `Makefile` — add:
```makefile
swagger:
	swag init -g cmd/api/main.go -o docs/swagger
```
and add `swagger` to the `.PHONY` line.

- [ ] **Step 9: Verify it builds and serves**

Run:
```bash
go build ./...
go run ./cmd/api
```
Then open `http://localhost:8080/swagger/index.html` in a browser.
Expected: the Swagger UI loads and lists every annotated endpoint, grouped by tag (`platform-auth`, `tenant-auth`, `users`).

- [ ] **Step 10: Commit**

```bash
git add cmd/api internal/modules internal/server docs/swagger Makefile go.mod go.sum
git commit -m "docs: OpenAPI documentation via swaggo/swag"
```

---

## Final Verification

- [ ] Run `go build ./...` — succeeds with no errors.
- [ ] Run `go vet ./...` — no warnings.
- [ ] Run `go test ./...` — every test from Tasks 2–20 passes.
- [ ] Run the manual smoke test from Task 18, Step 8, start to finish on a clean database.
- [ ] Open `http://localhost:8080/swagger/index.html` and confirm every endpoint is listed (Task 22).
- [ ] Confirm `git log --oneline` shows one commit per task, each with passing tests at that point in history.
