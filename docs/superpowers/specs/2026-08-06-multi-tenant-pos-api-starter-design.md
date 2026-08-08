# Multi-Tenant API Starter (Schema-per-Tenant) — Design

**Tanggal:** 2026-08-06 (revisi 2026-08-07)
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
| Hash password | bcrypt, cost 12 |
| Logging | `log/slog` (stdlib), format JSON |
| Dokumentasi API | `swaggo/swag` (OpenAPI di-generate dari komentar handler) |
| Hot reload (dev) | `air` |
| Testing | PostgreSQL lokal (bukan Docker) |
| Deploy | Binary + systemd + Caddy, **tanpa Docker** |

**Docker tidak dipakai sama sekali** — tidak untuk development, testing, maupun
produksi. Go menghasilkan binary statis, sehingga masalah yang biasanya diselesaikan
Docker (kecocokan runtime, dependency) tidak berlaku di sini.

Pilihan bcrypt di atas Argon2id: Argon2id secara teori lebih tahan serangan GPU, tapi
punya tiga parameter yang salah setel justru melemahkan atau menghabiskan RAM saat
banyak login bersamaan. bcrypt hanya punya satu parameter, dan cost 12 (~250ms per
verifikasi) sudah lebih dari cukup untuk kasus ini.

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
      user/                # staf tenant  (contoh pola tenant-scoped)
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
`FROM acme_corp.users`), sehingga modul bisa disalin apa adanya.

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
5. Seed permissions + role default (`owner` saja)
6. Buat user owner pertama
7. UPDATE status → active
```

Langkah 2–7 berada dalam **satu transaksi**. PostgreSQL mendukung DDL transaksional,
jadi kegagalan di langkah mana pun membatalkan seluruhnya — tidak ada tenant
setengah jadi.

## Siklus Hidup Tenant

Empat kondisi, masing-masing punya arti berbeda:

| Kondisi | Arti | Login | Data |
|---|---|---|---|
| `provisioning` | Sedang dibuat | Ditolak | Belum lengkap |
| `active` | Normal | Diizinkan | Utuh |
| `suspended` | Dibekukan sementara (mis. nunggak) | Ditolak | Utuh, bisa diaktifkan lagi |
| soft deleted (`deleted_at` terisi) | Berhenti berlangganan | Ditolak | **Schema masih ada di disk** |

Poin penting: soft delete hanya menyembunyikan baris di `platform.tenants`. Schema
tenant beserta seluruh isinya tetap ada dan tetap memakan ruang disk.

Penghapusan permanen dilakukan lewat perintah terpisah yang eksplisit:

```
cli tenant purge --code=acme_corp --confirm
```

Perintah ini menjalankan `DROP SCHEMA` dan **hanya berjalan bila tenant sudah
soft-deleted lebih dari 30 hari**. Jeda itu adalah jaring pengaman: `DROP SCHEMA`
tidak bisa dibatalkan, dan satu salah ketik nama tenant berarti data satu client
hilang permanen.

Alur lengkapnya: `active` → `suspended` → soft delete → (≥30 hari) → purge manual.

## Migrasi

| | Lokasi | Tabel versi | Kapan |
|---|---|---|---|
| Platform | `migration/platform/` | `platform.schema_migrations` | Sekali saat deploy |
| Tenant | `migration/tenant/` | `<schema>.schema_migrations` | Saat provisioning + deploy ke semua tenant |

Migrasi platform paling pertama bertugas menjalankan
`CREATE SCHEMA IF NOT EXISTS platform`, sebelum tabel apa pun dibuat.

Perintah CLI:

```
cli migrate platform up
cli migrate tenant up --all
cli migrate tenant up --tenant=acme_corp
cli migrate status                       # versi tiap tenant, tandai yang tertinggal

cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=...
cli tenant list
cli tenant suspend --code=acme_corp
cli tenant activate --code=acme_corp
cli tenant delete --code=acme_corp        # soft delete
cli tenant purge --code=acme_corp --confirm

cli admin create --email=... --name=...   # admin platform pertama (bootstrap)
cli permission sync --all                # sinkron konstanta permission ke tenant
cli cleanup                              # hapus refresh token & login_attempts kedaluwarsa
```

### Bootstrap admin pertama

Ada masalah ayam-dan-telur: membuat admin platform memerlukan login sebagai admin,
padahal `platform.users` masih kosong sehingga tidak ada yang bisa login. Tanpa jalan
keluar, sistem tidak bisa dipakai sama sekali setelah dipasang.

`cli admin create` berjalan tanpa autentikasi — aksesnya dijaga oleh kepemilikan
shell di server, bukan oleh token. Password dibuat acak dan dicetak sekali ke layar,
lalu wajib diganti saat login pertama.

`--all` berjalan berurutan dan **berhenti di kegagalan pertama**, melaporkan tenant
mana yang sudah dan belum diproses.

### Pengaman versi schema

Karena `--all` bisa berhenti di tengah, sebagian tenant dapat tertinggal versi
sementara kode aplikasi sudah mengasumsikan versi terbaru. Tanpa pengaman, gejalanya
muncul sebagai error SQL yang menyesatkan seperti `column does not exist`.

Pengamannya: versi schema tiap tenant dibaca dan di-cache saat tenant pertama kali
diakses. Bila versinya lebih rendah dari versi yang dibutuhkan kode (konstanta di
Go), request ditolak dengan error eksplisit `TENANT_MIGRATION_PENDING` — bukan
dibiarkan menabrak.

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

**Asumsi single instance.** Cache permission disimpan di memori proses. Bila
aplikasi kelak dijalankan lebih dari satu instance di belakang load balancer,
pencabutan permission di instance A tidak terlihat oleh instance B sampai TTL habis
— sehingga user yang aksesnya sudah dicabut masih bisa lolos hingga 1 menit. Selama
aplikasi berjalan satu instance, ini tidak pernah terjadi. Asumsi ini ditulis
eksplisit agar saat scaling nanti langsung terlihat bahwa cache harus dipindah ke
Redis lebih dulu.

### Refresh token

Disimpan **ter-hash** (SHA-256) bersama `user_id`, `expires_at`, `revoked_at`,
`user_agent`, `ip`. Yang dikirim ke client adalah nilai acak 32 byte, bukan JWT.

Ada **dua tabel terpisah**, satu untuk tiap jenis user:

- `platform.refresh_tokens` — untuk admin platform
- `<schema>.refresh_tokens` — untuk staf tenant

Ketentuan:

- Access token: 15 menit
- Refresh token: 7 hari
- Logout menandai `revoked_at`
- Tanpa rotation (bisa ditambahkan nanti tanpa mengubah struktur)

### Aturan password

Minimal 8 karakter. Tidak ada syarat kombinasi huruf besar, angka, atau simbol.

Aturan yang rumit terbukti mendorong orang membuat password yang justru mudah
ditebak (`Password1!`) atau menuliskannya di kertas. Panjang lebih menentukan
daripada variasi karakter.

Password baru diperiksa juga terhadap daftar pendek password paling umum
(`123456789`, `password`, dan sejenisnya) dan ditolak bila cocok.

### Rate limit login

Berlaku khusus untuk kedua endpoint login — bukan pembatasan trafik umum, melainkan
perlindungan dari percobaan tebak password.

Aturannya: **5 percobaan gagal per email dalam 15 menit** → percobaan berikutnya
ditolak sampai jendela waktunya lewat, bahkan bila password yang dimasukkan benar.

Percobaan dicatat di tabel `platform.login_attempts`, bukan di memori. Alasannya dua:
hitungan tetap berlaku setelah aplikasi restart, dan tabel ini sekaligus menjadi
riwayat login — berguna saat ada user melaporkan akunnya dipakai orang lain, karena
waktu dan IP percobaan bisa ditelusuri.

Bebannya kecil (satu baris per percobaan, dibaca lewat index untuk 15 menit
terakhir), dan barisnya dibersihkan berkala lewat `cli cleanup`.

Ini tidak berpengaruh sama sekali pada sesi yang sudah berjalan.

## Model User

Dua entitas berbeda yang kebetulan sama-sama bernama "user":

| | `platform.users` | `<schema>.users` |
|---|---|---|
| Siapa | Admin sistem | Staf tenant (role dibuat sesuai kebutuhan project turunan) |
| Wewenang | Kelola tenant, provisioning, monitoring | Data di tenant-nya sendiri |
| Login | `/admin/auth/login` | `/auth/login` (+ `tenant_code`) |
| Scope JWT | `platform` | `tenant` |

Keduanya sekaligus berfungsi sebagai contoh pola di starterpack: satu non-tenant,
satu tenant-scoped.

## Tabel Schema Platform

```sql
tenants
  id             UUID PK
  code           TEXT UNIQUE      -- "acme_corp", dipakai saat login
  name           TEXT             -- "Acme Corp"
  schema_name    TEXT UNIQUE      -- "acme_corp"
  status         TEXT             -- provisioning | active | suspended
  suspended_at   TIMESTAMPTZ NULL
  + kolom audit standar

users                             -- admin sistem
  id, email (unik), password_hash, name, is_active
  + kolom audit standar

refresh_tokens                    -- untuk admin platform
  id, user_id, token_hash, expires_at, revoked_at, user_agent, ip
  created_at

login_attempts                    -- rate limit login (platform & tenant)
  id, scope, tenant_id NULL, email, attempted_at, success
  created_at
```

`code` dan `schema_name` dipisah meski awalnya bernilai sama: `code` adalah
identitas yang dipakai client saat login dan boleh berubah, sedangkan `schema_name`
terikat ke struktur database dan tidak boleh berubah selamanya.

### Pengecualian lain: tabel log & token

`refresh_tokens` dan `login_attempts` (di kedua schema) hanya memakai `created_at`,
di luar enam kolom audit standar. Keduanya adalah tabel *append-only*: baris tidak
pernah diedit atau dihapus lewat aksi user, jadi `updated_at`/`updated_by`/
`deleted_at`/`deleted_by` tidak punya makna. Status token dicabut sudah diwakili
`revoked_at`, bukan `deleted_at`.

## RBAC

Tabel per schema tenant:

```
users
roles              (id, name, is_system)
permissions        (id, code, group)
role_permissions   (role_id, permission_id)
user_roles         (user_id, role_id)
```

Permission didefinisikan sebagai konstanta Go
(`const ProductCreate = "product.create"`). **Sumber kebenarannya kode, bukan
database.**

### Sinkronisasi permission

Konstanta Go tidak dikenal oleh file migrasi `.sql`, jadi permission baru tidak bisa
ikut menyebar lewat migrasi biasa. Tanpa mekanisme khusus, menambah
`const SaleVoid = "sale.void"` hari ini tidak akan pernah sampai ke tenant yang sudah
ada — dan role di tenant lama tidak akan pernah bisa diberi permission itu.

Mekanismenya adalah perintah tersendiri:

```
cli permission sync --all
cli permission sync --tenant=acme_corp
```

Perintah ini membaca seluruh konstanta permission di kode, lalu `INSERT` yang belum
ada di tabel `permissions` tiap tenant. Sifatnya *idempotent* — aman dijalankan
berulang kali.

Permission yang konstantanya dihapus dari kode **tidak** ikut dihapus dari database
(baris di `role_permissions` bisa masih menunjuk ke sana). Pembersihannya manual dan
disengaja.

Perintah ini masuk ke urutan deploy, tepat setelah migrasi tenant.

Role default saat provisioning hanya `owner`. Tidak ada role bawaan lain — nama
seperti `cashier` bersifat POS, sedangkan starter ini harus netral terhadap domain.
Tiap project turunan membuat role sendiri sesuai kebutuhannya.

Tabel penghubung (`role_permissions`, `user_roles`) **dikecualikan dari soft
delete** — lihat Konvensi Tabel di bawah.

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

### Pengecualian: tabel penghubung

`role_permissions` dan `user_roles` hanya memakai `created_at` + `created_by`.
Pencabutan dilakukan dengan `DELETE` biasa, bukan soft delete.

Alasannya: soft delete pada relasi membuat setiap pengecekan permission menumpuk
filter `deleted_at IS NULL` berlapis tanpa manfaat nyata — riwayat pemberian dan
pencabutan akses lebih tepat dicatat di log daripada di tabel relasi.

### Aturan kolom `*_by` lintas schema

Kolom `*_by` di tabel tenant **selalu merujuk user di schema tenant yang sama**.

Masalahnya muncul saat provisioning: user owner pertama dibuat oleh admin platform,
yang datanya ada di `platform.users` — schema berbeda. UUID lintas schema tidak bisa
dijamin lewat foreign key, dan saat dibaca akan tampak seperti data rusak (UUID yang
tidak ditemukan di tabel manapun dalam schema itu).

Aturannya: bila pelakunya admin platform atau proses sistem, kolom `*_by` diisi
`NULL`, dan identitas pelaku dicatat di log. Dengan begitu `NULL` punya arti yang
konsisten — "bukan orang di dalam tenant ini".

## Response & Error

Sukses:

```json
{ "success": true, "data": {} }
```

Untuk daftar berpaginasi, bentuk `meta` mengikuti format yang ditetapkan di bagian
Pagination (`page`, `limit`, `total`, `total_pages`) — bukan bentuk bebas per modul.

Gagal:

```json
{
  "success": false,
  "error": { "code": "VALIDATION_ERROR", "message": "...", "details": {} },
  "request_id": "01HQ..."
}
```

Khusus `VALIDATION_ERROR`, `details` berisi peta nama field ke daftar pesan,
sehingga frontend bisa menaruh pesan tepat di bawah field yang salah:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Data yang dikirim tidak valid",
    "details": {
      "email": ["format email tidak valid"],
      "password": ["minimal 8 karakter"]
    }
  },
  "request_id": "01HQ..."
}
```

Nama field memakai penamaan yang sama persis dengan yang dikirim client (`snake_case`
di JSON), bukan nama field di struct Go. Validasi memakai `go-playground/validator`,
dan penerjemahan ke bentuk di atas dilakukan satu kali di `shared/validator`.

Service mengembalikan `*apperror.Error` bertipe (`NotFound`, `Conflict`,
`Validation`, `Forbidden`, `Internal`). Satu error handler Fiber di paling atas
menerjemahkannya menjadi status HTTP dan body — handler tidak pernah menulis
`c.Status(404)` sendiri.

Error internal tidak pernah membocorkan detail ke client: yang keluar hanya
`request_id`, sementara pesan asli dan stack trace masuk ke log dengan `request_id`
yang sama.

Logging `log/slog` format JSON, dengan `request_id` dan `tenant_id` otomatis melekat
di setiap baris log dalam satu request.

## Versi API & Pagination

### Versi API

Semua rute berada di bawah `/api/v1` sejak awal. Biayanya nol sekarang, dan mahal
bila baru dipasang belakangan — seluruh URL di frontend yang sudah berjalan harus
diubah.

Naik ke `v2` **hanya** bila ada perubahan yang merusak client lama:

| Perubahan | Merusak | Naik versi |
|---|---|---|
| Menambah field di response | Tidak | Tidak |
| Menambah endpoint | Tidak | Tidak |
| Menambah parameter opsional | Tidak | Tidak |
| Menghapus / mengganti nama field | **Ya** | Ya |
| Mengubah arti field | **Ya** | Ya |
| Menjadikan parameter wajib | **Ya** | Ya |

Selama frontend dan backend dikuasai satu pihak dan bisa dideploy bersamaan, `v1`
boleh diubah langsung tanpa membuat `v2`. Versi baru baru benar-benar diperlukan bila
ada client yang tidak bisa dipaksa memperbarui diri — misalnya aplikasi mobile yang
sudah terpasang di perangkat. Bila itu terjadi, `v1` dan `v2` berjalan berdampingan
dan `v1` diumumkan akan dihentikan dalam tenggat tertentu.

### Pagination

Memakai **offset**, bukan cursor:

```
GET /api/v1/users?page=2&limit=20&sort=created_at&order=desc
```

- `limit` default 20, maksimum 100 (nilai di atas itu dipangkas, bukan ditolak)
- `page` dimulai dari 1

**`sort` wajib memakai daftar putih.** Nama kolom tidak bisa dikirim sebagai
parameter query seperti nilai biasa — ia harus disisipkan langsung ke teks SQL. Bila
nilainya diambil mentah dari URL, penyerang dapat menyisipkan potongan SQL dan
membaca data di luar haknya, **termasuk menembus ke schema tenant lain**. Ini
membatalkan seluruh isolasi tenant.

Karena itu tiap repository mendeklarasikan kolom yang boleh disortir:

```go
var userSortable = map[string]string{
    "created_at": "created_at",
    "name":       "name",
    "email":      "email",
}
```

Nilai `sort` di luar daftar ditolak dengan `VALIDATION_ERROR`, bukan diabaikan
diam-diam. `order` hanya menerima `asc` atau `desc`. Helper di `shared/pagination`
memaksa pemeriksaan ini sehingga tidak bisa terlewat.

Response memakai blok `meta` yang seragam:

```json
{
  "success": true,
  "data": [],
  "meta": { "page": 2, "limit": 20, "total": 137, "total_pages": 7 }
}
```

Offset dipilih karena mudah dipahami dan cocok dengan UI bernomor halaman. Cursor
lebih unggul pada dataset sangat besar, tapi tidak sepadan dengan kerumitannya di
sini. Keseragaman formatnya yang penting — bila tiap modul memakai gaya sendiri,
frontend harus menangani banyak bentuk berbeda.

## Testing

| Lapis | Cara | Yang diuji |
|---|---|---|
| Unit | Mock repository | Business logic di service |
| Integration | PostgreSQL lokal, database `app_test` | Repository, migrasi, provisioning |
| E2E | Fiber test app + DB asli | Login → akses → RBAC |

Integration test wajib memakai database asli. Yang paling ingin dibuktikan adalah
perilaku `search_path` dan isolasi schema, dan itu tidak mungkin diuji dengan mock.

Karena Docker tidak dipakai, testcontainers tidak berlaku. Sebagai gantinya test
menembak PostgreSQL yang terinstall di mesin, ke database khusus `app_test`. Setiap
test membuat schema tenant bernama acak lalu menghapusnya di akhir — isolasi tetap
terjaga dan test tetap bisa berjalan paralel, hanya instance PostgreSQL-nya yang
dipakai bersama.

Konsekuensinya: developer baru wajib memasang PostgreSQL lebih dulu sebelum bisa
menjalankan test. Ini dicatat di `docs/getting-started.md`.

Koneksi test dibaca dari berkas `.env.test` terpisah, agar tidak mungkin tertukar
dengan database development. `.env.test.example` ikut masuk git sebagai contoh;
`.env.test` sendiri diabaikan `.gitignore`.

**Test terpenting di repo ini:** dua tenant dengan data serupa, memastikan query
tenant A tidak pernah melihat baris milik tenant B — termasuk saat terjadi error dan
rollback di tengah transaksi.

## Konfigurasi

Lewat env var, divalidasi saat boot — aplikasi menolak start bila ada yang kurang
atau tidak valid, bukan gagal belakangan saat request pertama masuk.

Koneksi database ditulis **terpecah**, bukan sebagai satu URL:

```ini
DB_HOST=localhost
DB_PORT=5432
DB_USER=app
DB_PASSWORD=rahasia
DB_NAME=appdb
DB_SSLMODE=disable

DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m

APP_PORT=8080
APP_ENV=production
JWT_SECRET=...
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
LOGIN_MAX_ATTEMPTS=5
LOGIN_ATTEMPT_WINDOW=15m
SHUTDOWN_TIMEOUT=15s

CORS_ALLOWED_ORIGINS=https://acme.com
```

Hampir seluruh penyetelan sistem berada di sini, sehingga bisa diubah di server
tanpa build ulang — cukup edit berkas lalu restart.

Alasan dipecah: lebih mudah dibaca dan diubah, dan password yang mengandung `@`,
`#`, atau `/` bisa ditulis apa adanya tanpa URL-encoding — sumber kesalahan yang
gejalanya hanya muncul sebagai `authentication failed`. Di dalam kode keenam nilai
DB digabung jadi satu connection string.

Pindah ke server PostgreSQL terpisah nanti cukup mengubah `DB_HOST` dan
`DB_SSLMODE`, tanpa perubahan kode.

## CORS

Saat development, React berjalan di `http://localhost:5173` sementara API di
`http://localhost:8080`. Browser menganggapnya dua asal berbeda dan **memblokir
seluruh request** bila CORS tidak disetel — hambatan yang muncul di menit pertama
menyambungkan frontend.

Daftar asal yang diizinkan dibaca dari `CORS_ALLOWED_ORIGINS` (dipisah koma):

- Development: `http://localhost:5173`
- Produksi: domain aplikasi, mis. `https://acme.com`

Wildcard `*` **tidak** dipakai, karena API ini memakai kredensial (token). Header
yang diizinkan dibatasi pada yang benar-benar dipakai: `Content-Type`,
`Authorization`.

Di produksi, frontend dan API berada di balik satu domain lewat Caddy, sehingga CORS
sebenarnya tidak terpakai — tetap disetel sebagai pengaman bila kelak dipisah ke
subdomain berbeda.

## Deployment

Tanpa Docker. Go menghasilkan satu binary statis, jadi server tidak perlu runtime
apa pun.

### Susunan di server

```
/opt/app/
  api                        # binary HTTP server
  cli                        # binary CLI (migrasi, provisioning)
  api.backup                 # binary versi sebelumnya, untuk rollback
  migrations/                # file .sql

/etc/app/api.env             # config, permission 600
/etc/systemd/system/app-api.service
```

Binary CLI dipisah dari API agar migrasi dan provisioning bisa dijalankan tanpa
menyentuh proses yang sedang melayani request.

### systemd

Aplikasi dijalankan sebagai service dengan `Restart=always`, `User=app` (bukan
root), dan log ke journald. Yang didapat: restart otomatis saat crash, start
otomatis saat server boot, dan `journalctl -u app-api -f` untuk membaca log.

### Caddy sebagai reverse proxy

Aplikasi hanya mendengarkan di `127.0.0.1:8080`, tidak pernah langsung terbuka ke
internet. Caddy berdiri di depan untuk TLS dan pembagian rute:

```
acme.com {
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

Caddy dipilih di atas nginx karena mengurus sertifikat Let's Encrypt sepenuhnya
otomatis, termasuk perpanjangan. Reverse proxy tetap diperlukan meski server HTTP
Go sudah siap produksi, karena: frontend React dan backend harus berbagi port 443,
port di bawah 1024 butuh hak root (aplikasi sebaiknya berjalan sebagai user biasa),
dan file statis lebih efisien dilayani Caddy.

### Alur deploy

Kode di-*clone* di server; build dilakukan di server:

```bash
cd /opt/app
git pull
cp api api.backup                 # simpan versi lama untuk rollback
go build -o api ./cmd/api
go build -o cli ./cmd/cli
./cli migrate platform up
./cli migrate tenant up --all
./cli permission sync --all
sudo systemctl restart app-api
```

Dibungkus jadi `deploy.sh`. Rollback bila versi baru bermasalah:

```bash
cp api.backup api && sudo systemctl restart app-api
```

Migrasi sengaja dijalankan sebagai langkah terpisah, **bukan otomatis saat aplikasi
start** — agar tidak ada dua instance yang berebut menjalankan migrasi yang sama.

Restart menyebabkan downtime 1–2 detik. Untuk aplikasi dengan jam operasional
tertentu (seperti POS), deploy dilakukan di luar jam operasional itu. Zero-downtime
deploy tidak dibangun sekarang.

Build ulang hanya diperlukan bila kode `.go` berubah. Mengubah `api.env` atau
menambah tenant cukup restart / jalankan CLI.

Target OS: Linux (Ubuntu/Debian maupun Rocky). Dokumentasi mencakup keduanya,
termasuk catatan SELinux di Rocky yang secara default memblokir Caddy menghubungi
port lokal.

### Backup

Satu database menampung data transaksi seluruh client. Satu disk rusak berarti semua
client kehilangan data sekaligus, jadi backup bukan hal opsional di sini.

Ketentuan minimum:

- `pg_dump` seluruh database, dijadwalkan harian lewat systemd timer
- Hasil backup **disimpan di luar server aplikasi** — backup yang duduk di disk yang
  sama tidak melindungi dari kerusakan disk
- Simpan 7 harian + 4 mingguan
- **Uji pemulihan secara berkala.** Backup yang tidak pernah dicoba dipulihkan belum
  terbukti bisa dipakai.

Perintah dan berkas timer ditulis lengkap di `docs/deployment.md`.

### Development di Windows

Development dilakukan di Windows 11 dengan PostgreSQL terpasang lokal dan `air`
untuk hot reload. Karena target deploy adalah Linux, repo menyertakan
`.gitattributes` yang memaksa akhiran baris LF untuk `*.sh` dan `*.sql` — tanpa itu
script yang ditulis di Windows gagal di Linux dengan error `bad interpreter` yang
penyebabnya tidak terlihat.

## Operasional

- `/health` (liveness) dan `/health/ready` (cek koneksi DB).
- Graceful shutdown — berhenti menerima request baru, lalu menunggu request yang
  sedang berjalan sampai selesai agar tidak ada transaksi yang terputus di tengah
  jalan. Dibatasi
  `SHUTDOWN_TIMEOUT` (default 15 detik); lewat dari itu proses ditutup paksa. Tanpa
  batas ini, satu query yang menggantung membuat aplikasi tidak pernah mati dan
  akhirnya dibunuh paksa systemd — persis hal yang ingin dihindari.
- Makefile untuk perintah sehari-hari (`make run`, `make test`, `make build`).

## Dokumentasi

Dua hal yang harus dipisahkan:

- `docs/superpowers/specs/` — **arsip keputusan**, bertanggal, tidak diedit lagi saat
  kode berkembang. Nilainya justru pada keawetannya.
- `docs/` — **dokumentasi hidup**, wajib selalu cocok dengan kode.

Berkas di `docs/` ditulis **berbarengan dengan implementasinya**, bukan di muka —
dokumentasi untuk kode yang belum ada hampir pasti meleset lalu terlanjur dipercaya:

| Berkas | Isi |
|---|---|
| `getting-started.md` | Setup Windows, install PostgreSQL, migrasi, tenant pertama |
| `adding-a-module.md` | Langkah menambah modul baru + konvensi sub-grup domain |
| `multi-tenancy.md` | Aturan keselamatan: `WithTenant`, batas `*fiber.Ctx`, contoh salah |
| `database-conventions.md` | Kolom audit, soft delete, unique parsial, trigger, penamaan |
| `deployment.md` | systemd, Caddy, `deploy.sh`, env var, Ubuntu & Rocky |

`README.md` di root ditulis **setelah starter selesai**, bukan sekarang.

`CLAUDE.md` di root berisi aturan yang bila dilanggar berakibat serius —
`*fiber.Ctx` berhenti di handler, semua akses data tenant lewat `WithTenant`, modul
tidak memanggil repository modul lain — agar terbaca otomatis di setiap sesi, di
semua project turunan.

## Di Luar Cakupan

Hal-hal berikut sengaja tidak dibangun sekarang, dan struktur di atas tidak
menghalangi penambahannya nanti:

- Endpoint admin self-service untuk provisioning (logic-nya sudah reusable)
- GitHub Actions / CI-CD — deploy manual dulu, ditambahkan saat sudah terasa perlu
- Zero-downtime deploy
- Cache permission terdistribusi (Redis) — lihat asumsi single instance
- Refresh token rotation
- Domain bisnis POS (product, sale, inventory) — dibangun di repo terpisah
- Rate limiting, observability/tracing
