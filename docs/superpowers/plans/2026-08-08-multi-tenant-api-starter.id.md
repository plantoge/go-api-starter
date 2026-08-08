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
