# Multi-Tenant API Starter (Schema-per-Tenant) — Design

**Tanggal:** 2026-08-06
**Status:** Disetujui

## Ringkasan

Backend API Golang yang melayani banyak client (tenant) dari satu deployment, dengan
isolasi data lewat **satu schema PostgreSQL per tenant**. Repo ini adalah
*starterpack*: berisi fondasi lengkap plus modul `user` sebagai contoh pola, tanpa
domain bisnis POS di dalamnya. Kasus pakai pertamanya adalah aplikasi Point of Sales,
yang dibangun di repo terpisah hasil clone.

## Stack

| Komponen | Pilihan |
|---|---|
| HTTP framework | Fiber v2 |
| Database | PostgreSQL |
| Akses data | sqlx (driver pgx) |
| Migrasi | golang-migrate |
| Auth | JWT (access + refresh token) |
| Logging | `log/slog` (stdlib), format JSON |
| Testing | testcontainers untuk integration test |

### Catatan soal Fiber

Fiber dibangun di atas `fasthttp`, yang me-*reuse* objek context antar request. Kalau
`*fiber.Ctx` disimpan atau dioper ke goroutine, isinya bisa berubah menjadi milik
request lain — dan pada sistem multi-tenant itu berarti kebocoran data antar tenant.

**Aturan wajib:** `*fiber.Ctx` berhenti di layer handler. Handler menyalin yang
dibutuhkan ke `context.Context` standar; service dan repository hanya mengenal
`context.Context`. Efek sampingnya menguntungkan — business logic jadi
framework-agnostic dan bisa diuji tanpa menyalakan server.

## Arsitektur

### Layering

```
handler     → tahu HTTP, satu-satunya yang menyentuh *fiber.Ctx
   ↓ context.Context + DTO
service     → business logic, nol pengetahuan soal HTTP
   ↓ context.Context + domain model
repository  → SQL via sqlx
```

Dependensi hanya mengarah ke dalam.

### Struktur direktori

```
cmd/
  api/main.go              # HTTP server
  cli/main.go              # provisioning tenant, migrasi, seed
internal/
  config/                  # env → struct, divalidasi saat boot
  database/
    postgres.go            # pool koneksi
    tenant.go              # WithTenant: resolusi schema per request
  migration/
    platform/              # migrasi schema platform
    tenant/                # migrasi untuk SETIAP schema tenant
    runner.go
  middleware/
    auth.go                # verifikasi JWT, pisahkan scope platform/tenant
    tenant.go              # tenant dari claim → context
    rbac.go                # RequirePermission(...)
    recover.go, logger.go, requestid.go, cors.go
  modules/
    platform/
      tenant/              # CRUD tenant + provisioning
      user/                # admin sistem
      auth/
    tenant/
      auth/                # login, refresh, logout
      user/                # staf toko  (contoh pola tenant-scoped)
      role/                # RBAC
  shared/
    response/              # format response JSON seragam
    apperror/              # tipe error domain → HTTP status
    validator/
    pagination/
    querybuilder/          # helper soft-delete & filter
pkg/                       # utility generik
docs/
```

Setiap modul berisi berkas yang seragam: `handler.go`, `service.go`,
`repository.go`, `dto.go`, `model.go`. Menambah modul = menyalin satu folder.

### Konvensi sub-grup domain

Mulai datar (`modules/tenant/product/`). Setelah `modules/tenant/` berisi 7–8 modul
dan mulai sulit dinavigasi, baru kelompokkan ke map domain:

```
modules/tenant/logistik/product/
modules/tenant/sales/order/
```

Map domain hanyalah folder — modul sesungguhnya tetap unit terkecil.

**Aturan lintas modul:** modul tidak boleh memanggil repository modul lain. Interaksi
selalu lewat *service*. Contoh: `sales/order` mengurangi stok dengan memanggil
`logistik/stock` service, bukan menembak tabel `stock` langsung.

## Isolasi Multi-Tenant

### Identifikasi tenant

Tenant ditentukan dari **claim di JWT**, bukan dari header atau subdomain. Endpoint
login menerima `tenant_code` di body; setelah itu seluruh request memakai
`tenant_id` di token.

JWT hanya membawa `tenant_id`. **Nama schema selalu dilihat dari tabel
`platform.tenants`**, tidak pernah diambil dari input user (di-cache in-memory,
diinvalidasi saat data tenant berubah).

### Scoping schema

Semua akses data tenant melewati `WithTenant`, yang menjalankan
`SET LOCAL search_path` di dalam transaksi:

```go
func (db *DB) WithTenant(ctx context.Context, fn func(*sqlx.Tx) error) error {
    schema, ok := TenantFromContext(ctx)
    if !ok {
        return apperror.ErrNoTenantContext
    }

    tx, err := db.BeginTxx(ctx, nil)
    if err != nil { return err }
    defer tx.Rollback()

    if _, err := tx.ExecContext(ctx,
        "SET LOCAL search_path TO "+pq.QuoteIdentifier(schema)); err != nil {
        return err
    }

    if err := fn(tx); err != nil { return err }
    return tx.Commit()
}
```

`SET LOCAL` otomatis hilang saat transaksi berakhir — commit maupun rollback —
sehingga koneksi kembali ke pool dalam keadaan bersih tanpa perlu reset manual.
Pendekatan `SET` biasa + reset manual ditolak karena satu jalur error yang terlewat
sudah cukup untuk membocorkan data antar tenant.

Konsekuensi: repository tenant menerima `*sqlx.Tx`, bukan `*sqlx.DB`. Efek
sampingnya menguntungkan — operasi multi-tabel otomatis atomik.

Repository menulis query tanpa prefiks schema (`FROM users`, bukan
`FROM toko_aba.users`), sehingga modul bisa disalin apa adanya.

### Pertahanan berlapis

1. Nama schema divalidasi regex `^[a-z][a-z0-9_]{2,49}$` saat provisioning, dan
   di-*quote* dengan `pq.QuoteIdentifier` saat dipakai.
2. Nama schema berasal dari `platform.tenants`, tidak pernah dari input user.
3. Middleware tenant menolak request tanpa tenant context sebelum mencapai handler.
4. Repository platform memakai koneksi terpisah yang di-pin ke `search_path = platform`.

## Provisioning Tenant

Satu fungsi `TenantService.Provision()` dipakai bersama oleh CLI dan (nanti)
endpoint admin. CLI dibangun lebih dulu; endpoint self-service menyusul tanpa
mengubah logic.

```
1. Validasi & normalisasi kode tenant → nama schema
2. INSERT platform.tenants (status: provisioning)
3. CREATE SCHEMA
4. Jalankan seluruh migrasi tenant ke schema tersebut
5. Seed permissions + role default (owner, cashier)
6. Buat user owner pertama
7. UPDATE status → active
```

Langkah 2–7 berada dalam **satu transaksi**. PostgreSQL mendukung DDL transaksional,
jadi kegagalan di langkah mana pun membatalkan seluruhnya — tidak ada tenant
setengah jadi.

## Migrasi

| | Lokasi | Tabel versi | Kapan |
|---|---|---|---|
| Platform | `migration/platform/` | `platform.schema_migrations` | Sekali saat deploy |
| Tenant | `migration/tenant/` | `<schema>.schema_migrations` | Saat provisioning + deploy ke semua tenant |

Perintah CLI:

```
cli migrate platform up
cli migrate tenant up --all
cli migrate tenant up --tenant=toko_aba
cli tenant create --code=toko_aba --name="Toko ABA" --owner-email=...
cli tenant list
cli token cleanup            # hapus refresh token kedaluwarsa
```

`--all` berjalan berurutan dan **berhenti di kegagalan pertama**, melaporkan tenant
mana yang sudah dan belum diproses.

## Autentikasi

### Dua alur terpisah

```
POST /api/v1/admin/auth/login    { email, password }
POST /api/v1/auth/login          { tenant_code, email, password }
```

Alur tenant: cari tenant dari `tenant_code` → pastikan `status = active` → buka
schema → verifikasi user → terbitkan token.

### Isi token

```json
{
  "sub": "user-uuid",
  "scope": "tenant",
  "tenant_id": "tenant-uuid",
  "exp": 1234567890
}
```

`scope` (`platform` | `tenant`) adalah pemisah keras. `RequirePlatform()` menolak apa
pun yang bukan `scope: platform`, dan `RequireTenant()` sebaliknya — meskipun kunci
penandatangannya sama.

**Permission tidak dimasukkan ke token.** Permission yang dicabut akan tetap sakti
sampai token kedaluwarsa kalau ditanam di dalamnya. Sebagai gantinya, permission
dibaca dari database saat request, dengan cache in-memory TTL 1 menit yang
diinvalidasi saat role user berubah.

### Refresh token

Disimpan **ter-hash** (SHA-256) di `<schema>.refresh_tokens` bersama `user_id`,
`expires_at`, `revoked_at`, `user_agent`, `ip`. Yang dikirim ke client adalah nilai
acak 32 byte, bukan JWT.

- Access token: 15 menit
- Refresh token: 7 hari
- Logout menandai `revoked_at`
- Tanpa rotation (bisa ditambahkan nanti tanpa mengubah struktur)

## Model User

Dua entitas berbeda yang kebetulan sama-sama bernama "user":

| | `platform.users` | `<schema>.users` |
|---|---|---|
| Siapa | Admin sistem | Staf toko: owner, kasir, gudang |
| Wewenang | Kelola tenant, provisioning, monitoring | Data di tenant-nya sendiri |
| Login | `/admin/auth/login` | `/auth/login` (+ `tenant_code`) |
| Scope JWT | `platform` | `tenant` |

Keduanya sekaligus berfungsi sebagai contoh pola di starterpack: satu non-tenant,
satu tenant-scoped.

## RBAC

Tabel per schema tenant:

```
users
roles              (id, name, is_system)
permissions        (id, code, group)
role_permissions   (role_id, permission_id)
user_roles         (user_id, role_id)
```

Permission didefinisikan sebagai konstanta Go (`const ProductCreate = "product.create"`)
dan disinkronkan ke tabel `permissions` saat provisioning maupun migrasi. **Sumber
kebenarannya kode, bukan database** — menambah permission cukup dengan menambah
konstanta, tanpa migrasi manual di puluhan tenant.

Role `owner` bertanda `is_system = true`: tidak bisa dihapus dan selalu memiliki
seluruh permission. Ini diterapkan lewat *bypass* di pengecekan, bukan lewat baris di
`role_permissions`, supaya permission baru otomatis dimiliki owner tanpa backfill.

Pemakaian di handler:

```go
r.Post("/products",
    mw.RequireTenant(),
    mw.RequirePermission(perm.ProductCreate),
    h.Create)
```

## Konvensi Tabel

Setiap tabel — platform maupun tenant — wajib memiliki:

```sql
created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at  TIMESTAMPTZ NULL
created_by  UUID NULL
updated_by  UUID NULL
deleted_by  UUID NULL
```

Konsekuensi yang wajib ditangani:

- **Soft delete.** Setiap query menyaring `WHERE deleted_at IS NULL`. Karena mudah
  terlupa, `shared/querybuilder` dan repository dasar menyertakan filter ini secara
  default.
- **Unique index parsial.** `UNIQUE(email) WHERE deleted_at IS NULL`, supaya nilai
  milik baris terhapus bisa dipakai ulang.
- `updated_at` diisi trigger PostgreSQL (satu fungsi, dipasang ke tiap tabel saat
  migrasi).
- Kolom `*_by` diisi dari user di `context.Context`.

## Response & Error

Sukses:

```json
{ "success": true, "data": {}, "meta": { "page": 1, "total": 100 } }
```

Gagal:

```json
{
  "success": false,
  "error": { "code": "VALIDATION_ERROR", "message": "...", "details": {} },
  "request_id": "01HQ..."
}
```

Service mengembalikan `*apperror.Error` bertipe (`NotFound`, `Conflict`,
`Validation`, `Forbidden`, `Internal`). Satu error handler Fiber di paling atas
menerjemahkannya menjadi status HTTP dan body — handler tidak pernah menulis
`c.Status(404)` sendiri.

Error internal tidak pernah membocorkan detail ke client: yang keluar hanya
`request_id`, sementara pesan asli dan stack trace masuk ke log dengan `request_id`
yang sama.

Logging `log/slog` format JSON, dengan `request_id` dan `tenant_id` otomatis melekat
di setiap baris log dalam satu request.

## Testing

| Lapis | Cara | Yang diuji |
|---|---|---|
| Unit | Mock repository | Business logic di service |
| Integration | testcontainers (PostgreSQL asli) | Repository, migrasi, provisioning |
| E2E | Fiber test app + DB asli | Login → akses → RBAC |

Integration test wajib memakai database asli. Yang paling ingin dibuktikan adalah
perilaku `search_path` dan isolasi schema, dan itu tidak mungkin diuji dengan mock.

**Test terpenting di repo ini:** dua tenant dengan data serupa, memastikan query
tenant A tidak pernah melihat baris milik tenant B — termasuk saat terjadi error dan
rollback di tengah transaksi.

## Operasional

- Konfigurasi lewat env var, divalidasi saat boot (gagal cepat bila ada yang kurang).
- `/health` (liveness) dan `/health/ready` (cek koneksi DB).
- Graceful shutdown.
- Docker Compose untuk PostgreSQL saat development.
- Makefile untuk perintah sehari-hari.

## Di Luar Cakupan

Hal-hal berikut sengaja tidak dibangun sekarang, dan struktur di atas tidak
menghalangi penambahannya nanti:

- Endpoint admin self-service untuk provisioning (logic-nya sudah reusable)
- Refresh token rotation
- Domain bisnis POS (product, sale, inventory) — dibangun di repo terpisah
- Rate limiting, observability/tracing
