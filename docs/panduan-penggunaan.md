# Panduan Penggunaan Fitur

Dokumen ini menjelaskan **cara memakai** setiap fitur yang sudah ada di
starter ini — endpoint HTTP, perintah CLI, dan perilaku yang menempel pada
keduanya (autentikasi, RBAC, rate limit, pagination, format error).

Untuk pemasangan awal lihat [getting-started.md](getting-started.md); untuk
alasan di balik desainnya lihat [multi-tenancy.md](multi-tenancy.md),
[database-conventions.md](database-conventions.md), dan
[adding-a-module.md](adding-a-module.md).

---

## 1. Dua bidang yang perlu dibedakan

Aplikasi ini punya dua "dunia" yang sengaja dipisah keras:

| | **Platform** | **Tenant** |
|---|---|---|
| Siapa | Admin pengelola aplikasi | Staf milik satu tenant |
| Data | Schema `platform` | Schema per tenant (mis. `acme_corp`) |
| Scope token | `platform` | `tenant` |
| Prefix endpoint | `/api/v1/admin/...` | `/api/v1/...` |
| Kelola tenant | Lewat CLI (`cmd/cli`) | — |

Token dari satu scope **tidak pernah** diterima di scope lain, walaupun
ditandatangani dengan `JWT_SECRET` yang sama. Access token bawaan scope
`tenant` juga membawa klaim `tenant_id`, dan itulah yang menentukan schema
mana yang dibaca — bukan header atau parameter apa pun dari klien.

> Catatan: pengelolaan tenant (buat, suspend, hapus) saat ini **hanya
> tersedia lewat CLI**. Middleware `middleware.RequirePlatform` sudah ada
> dan siap dipakai, tetapi belum ada endpoint HTTP admin selain login /
> refresh / logout.

---

## 2. Format respons

### Sukses

```json
{ "success": true, "data": { ... } }
```

Endpoint list menambahkan `meta`:

```json
{
  "success": true,
  "data": [ ... ],
  "meta": { "page": 1, "limit": 20, "total": 42, "total_pages": 3 }
}
```

### Error

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "data yang dikirim tidak valid",
    "details": { "email": ["format email tidak valid"] }
  },
  "request_id": "01J8Z9..."
}
```

`details` selalu ada (objek kosong bila tidak relevan), dan kuncinya adalah
**nama field JSON**, bukan nama field Go — jadi bisa dipetakan langsung ke
input di frontend.

### Daftar kode error

| `code` | HTTP | Kapan muncul |
|---|---|---|
| `VALIDATION_ERROR` | 422 | Body tidak lolos validasi, atau `sort`/`order` tidak dikenal |
| `UNAUTHORIZED` | 401 | Kredensial salah, token hilang/tidak valid, refresh token kadaluarsa |
| `FORBIDDEN` | 403 | Scope token salah, tenant tidak aktif, permission kurang |
| `NOT_FOUND` | 404 | Resource tidak ada, atau rute tidak terdaftar |
| `CONFLICT` | 409 | Duplikasi (email/kode tenant), atau aturan lifecycle tenant |
| `TENANT_MIGRATION_PENDING` | 409 | Schema tenant tertinggal dari binary yang berjalan |
| `RATE_LIMITED` | 429 | Percobaan login gagal melebihi batas |
| `INTERNAL_ERROR` | 500 | Bug atau kegagalan tak terduga |

Error internal **tidak pernah** membocorkan penyebab aslinya ke klien —
detailnya hanya masuk log server, dikaitkan lewat `request_id`.

### `request_id`

Setiap request diberi ULID dan dikembalikan di header `X-Request-ID`. Id
yang sama muncul di body error dan di setiap baris log request. Saat
melaporkan bug, sertakan nilai ini.

---

## 3. Autentikasi admin platform

Semua endpoint di bawah `/api/v1/admin/auth`, tanpa perlu token
(kecuali tentu saja token yang dikirim di body untuk refresh/logout).

### Login

```powershell
curl -X POST http://localhost:8080/api/v1/admin/auth/login `
  -H "Content-Type: application/json" `
  -d "{\"email\":\"admin@app.test\",\"password\":\"<password>\"}"
```

Body: `email` (wajib, format email), `password` (wajib).

Respons:

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "0f3a...",
    "expires_in": 900
  }
}
```

`expires_in` dalam detik, mengikuti `ACCESS_TOKEN_TTL`. Refresh token
berumur `REFRESH_TOKEN_TTL` dan disimpan di database hanya sebagai hash —
nilai aslinya cuma ditampilkan sekali di respons ini.

Login yang gagal selalu membalas pesan yang sama (`email atau password
salah`) baik email-nya tidak ada, akun non-aktif, maupun password salah.
Waktu prosesnya pun dibuat sama, supaya tidak bisa dipakai menebak email
mana yang terdaftar.

### Refresh

```powershell
curl -X POST http://localhost:8080/api/v1/admin/auth/refresh `
  -H "Content-Type: application/json" `
  -d "{\"refresh_token\":\"<refresh token>\"}"
```

Menghasilkan **access token baru**. Refresh token **tidak dirotasi** —
nilai yang sama dikembalikan dan tetap berlaku sampai kadaluarsa atau
dicabut lewat logout.

### Logout

```powershell
curl -X POST http://localhost:8080/api/v1/admin/auth/logout `
  -H "Content-Type: application/json" `
  -d "{\"refresh_token\":\"<refresh token>\"}"
```

Mencabut refresh token (`revoked_at` diisi). Access token yang sudah
terlanjur terbit **tetap berlaku sampai kadaluarsa** — itulah sebabnya
`ACCESS_TOKEN_TTL` dibuat pendek (bawaan 15 menit).

---

## 4. Autentikasi staf tenant

Sama seperti platform, tetapi tanpa prefix `/admin` dan **setiap request
wajib menyertakan `tenant_code`** — karena sebelum login belum ada token
yang bisa memberi tahu tenant mana yang dimaksud.

### Login

```powershell
curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<password>\"}"
```

Body: `tenant_code`, `email`, `password` (semuanya wajib).

Respons identik dengan login platform, hanya saja access token-nya membawa
`scope=tenant` dan `tenant_id`.

Tenant yang statusnya bukan `active` (mis. `suspended`) akan ditolak dengan
pesan generik `tenant_code, email, atau password salah` — status tenant
sengaja tidak dibocorkan lewat halaman login.

### Refresh

```powershell
curl -X POST http://localhost:8080/api/v1/auth/refresh `
  -H "Content-Type: application/json" `
  -d "{\"tenant_code\":\"acme_corp\",\"refresh_token\":\"<refresh token>\"}"
```

### Logout

```powershell
curl -X POST http://localhost:8080/api/v1/auth/logout `
  -H "Content-Type: application/json" `
  -d "{\"tenant_code\":\"acme_corp\",\"refresh_token\":\"<refresh token>\"}"
```

### Memakai access token

Semua endpoint tenant selain `/auth/*` memerlukan header:

```
Authorization: Bearer <access_token>
```

Yang diperiksa middleware, berurutan:

1. Token ada, tanda tangannya benar, belum kadaluarsa → jika tidak: `401`.
2. `scope` harus `tenant` → jika token platform: `403 FORBIDDEN`.
3. Tenant dari klaim `tenant_id` masih ada → jika tidak: `401`.
4. Status tenant `active` → jika `suspended`: `403 tenant tidak aktif`.
5. Schema tenant sudah semigrasi dengan binary → jika tertinggal atau
   `dirty`: `409 TENANT_MIGRATION_PENDING` (jalankan
   `cli migrate tenant up`, lihat §8).

Hasil lookup tenant di-cache 1 menit di memori proses. Artinya perubahan
status tenant (suspend/activate/delete) baru terasa paling lama 1 menit
kemudian — kecuali dilakukan lewat service yang sama, yang langsung
membatalkan cache-nya. Cache ini per-proses, jadi deployment multi-instance
perlu dipindahkan ke Redis.

---

## 5. Rate limit login

Berlaku untuk login platform maupun tenant.

- Dihitung per kombinasi (scope, tenant, email), hanya percobaan **gagal**.
- Batas: `LOGIN_MAX_ATTEMPTS` percobaan dalam `LOGIN_ATTEMPT_WINDOW`
  (bawaan 5 kali dalam 15 menit).
- Saat terlampaui: `429 RATE_LIMITED` — pemeriksaan password bahkan tidak
  dijalankan.
- Blokir hilang sendiri begitu percobaan lama keluar dari jendela waktu;
  tidak ada perintah "unblock". Untuk melepas paksa, hapus baris terkait
  dari `platform.login_attempts`.

Riwayat percobaan disimpan 30 hari lalu dibersihkan oleh `cli cleanup`
(§8).

---

## 6. Manajemen user tenant

Modul contoh sekaligus template untuk modul tenant baru. Semua endpoint di
bawah `/api/v1/users`, wajib bearer token scope `tenant`, dan masing-masing
menuntut permission tertentu.

| Method | Path | Permission | Sukses |
|---|---|---|---|
| `POST` | `/api/v1/users` | `user.create` | `201` |
| `GET` | `/api/v1/users` | `user.view` | `200` (+`meta`) |
| `GET` | `/api/v1/users/:id` | `user.view` | `200` |
| `PATCH` | `/api/v1/users/:id` | `user.update` | `200` |
| `DELETE` | `/api/v1/users/:id` | `user.delete` | `200` |

### Buat user

```powershell
curl -X POST http://localhost:8080/api/v1/users `
  -H "Authorization: Bearer <access_token>" `
  -H "Content-Type: application/json" `
  -d "{\"email\":\"kasir@acme.test\",\"password\":\"rahasia123\",\"name\":\"Kasir 1\"}"
```

Aturan password: minimal 8 karakter dan tidak termasuk daftar password
terlalu umum (`password`, `12345678`, dan sejenisnya). **Tidak ada**
kewajiban huruf besar/angka/simbol — panjang dianggap lebih efektif
daripada aturan yang mendorong pola tertebak.

Email ganda → `409 CONFLICT` dengan pesan `email sudah dipakai`.

Respons `201`:

```json
{
  "success": true,
  "data": {
    "id": "3f1c...",
    "email": "kasir@acme.test",
    "name": "Kasir 1",
    "is_active": true
  }
}
```

User baru dibuat **tanpa role**. Sampai diberi role, ia bisa login tapi
akan kena `403` di setiap endpoint yang butuh permission. Pemberian role
saat ini belum punya endpoint — lakukan langsung lewat SQL pada tabel
`user_roles` di schema tenant terkait.

### Daftar user

```powershell
curl "http://localhost:8080/api/v1/users?page=1&limit=20&sort=name&order=asc" `
  -H "Authorization: Bearer <access_token>"
```

Query parameter:

| Param | Bawaan | Keterangan |
|---|---|---|
| `page` | `1` | Nilai < 1 dinormalkan menjadi 1 |
| `limit` | `20` | Dibatasi maksimum **100**; nilai < 1 dikembalikan ke 20 |
| `sort` | `created_at` | Hanya `created_at`, `name`, `email` |
| `order` | `desc` | Hanya `asc` atau `desc` |

`sort` di luar whitelist → `422` dengan `details.sort`. `order` selain
`asc`/`desc` → `422` dengan `details.order`. Whitelist ini bukan sekadar
validasi input: nilai `sort` tidak pernah masuk ke `ORDER BY` secara
langsung, yang dipakai adalah nama kolom dari peta whitelist-nya.

User yang sudah di-soft delete tidak ikut terhitung maupun tampil.

### Ambil satu user

```powershell
curl http://localhost:8080/api/v1/users/<id> -H "Authorization: Bearer <access_token>"
```

`id` yang bukan UUID valid dibalas `404`, bukan `422` — supaya endpoint ini
tidak bisa dipakai membedakan "id salah format" dan "id tidak ada".

### Perbarui user

```powershell
curl -X PATCH http://localhost:8080/api/v1/users/<id> `
  -H "Authorization: Bearer <access_token>" `
  -H "Content-Type: application/json" `
  -d "{\"name\":\"Kasir Satu\",\"is_active\":false}"
```

Keduanya opsional — field yang tidak dikirim tidak diubah. Yang bisa
diubah hanya `name` dan `is_active`; **email dan password tidak bisa
diubah lewat endpoint ini**. Menonaktifkan user (`is_active: false`)
langsung memblokir login berikutnya, tapi access token yang sedang berjalan
tetap berlaku sampai kadaluarsa.

Respons: `{"success": true, "data": {"message": "berhasil diperbarui"}}`.

### Hapus user

```powershell
curl -X DELETE http://localhost:8080/api/v1/users/<id> -H "Authorization: Bearer <access_token>"
```

**Soft delete**: baris tetap ada dengan `deleted_at` dan `deleted_by`
terisi, hanya disaring dari semua query. Tidak ada endpoint restore —
kembalikan lewat SQL (`UPDATE users SET deleted_at = NULL ...`) bila perlu.

Menghapus user yang sudah terhapus → `404`.

---

## 7. RBAC (role dan permission)

### Permission yang tersedia

| Kode | Untuk |
|---|---|
| `user.view` | Melihat daftar/detail user |
| `user.create` | Membuat user |
| `user.update` | Mengubah user |
| `user.delete` | Menghapus user |
| `role.view` | Disediakan untuk modul role (belum dipakai endpoint) |
| `role.manage` | Disediakan untuk modul role (belum dipakai endpoint) |

Semua permission ini di-seed otomatis ke setiap tenant baru saat
provisioning. Untuk tenant yang sudah ada, permission baru ditambahkan
lewat `cli permission sync` (§8).

### Cara pengecekan bekerja

Permission **tidak** dititipkan di dalam JWT. Setiap request yang butuh izin
akan menanyakannya ke database tenant, lewat cache 1 menit di memori. Dua
konsekuensinya:

- Mencabut role berlaku cepat (maksimum ~1 menit), tanpa perlu menunggu
  token kadaluarsa.
- Cache-nya per-proses, sama seperti cache tenant — perlu diganti Redis
  bila jalan lebih dari satu instance.

### Role `owner`

Setiap tenant baru mendapat satu role `owner` dengan `is_system = true`,
dan user owner pertama otomatis diberi role itu. Role sistem **melewati**
tabel `role_permissions` sepenuhnya: pemiliknya dianggap punya semua
permission. Jadi ketika permission baru ditambahkan ke kode, owner langsung
mendapatkannya tanpa migrasi backfill.

Role non-sistem bekerja normal: izin diambil dari baris di
`role_permissions`. Belum ada endpoint untuk membuat role — kelola lewat
SQL di schema tenant sampai modul role dibangun.

---

## 8. Perintah CLI

Semua dijalankan dari root proyek. Konfigurasi dibaca dari `.env` yang
sama dengan API.

```powershell
go run ./cmd/cli <perintah> [flag]
```

Tanpa argumen, CLI menampilkan daftar perintah yang tersedia. Perintah
mana pun akan gagal lebih dulu dengan daftar lengkap masalah konfigurasi
bila `.env` belum benar.

### `admin create` — admin platform pertama

```powershell
go run ./cmd/cli admin create --email=admin@app.test --name="Admin Utama"
```

Menyelesaikan masalah ayam-dan-telur: membuat admin biasanya butuh login
sebagai admin, padahal `platform.users` awalnya kosong. Perintah ini
**tidak memeriksa autentikasi sama sekali** — pengamanannya adalah akses
shell ke server. Password acak dicetak **sekali**; salin sebelum menutup
terminal.

### `tenant create` — provisioning tenant baru

```powershell
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test
```

Ketiga flag wajib. `--code` dinormalkan (huruf kecil, spasi dan `-` jadi
`_`) lalu harus cocok dengan pola 3–50 karakter huruf kecil/angka/garis
bawah dan diawali huruf — karena kode ini juga dipakai sebagai nama schema.

Satu perintah ini melakukan seluruh rangkaian dalam **satu transaksi**:
membuat baris tenant, membuat schema, menerapkan semua migrasi tenant,
menanam permission, membuat role `owner`, membuat user owner, lalu menandai
tenant `active`. Kalau ada satu langkah gagal, tidak ada sisa apa pun —
termasuk schema-nya, karena DDL PostgreSQL ikut transaksional.

Password owner dicetak sekali, sama seperti `admin create`.

Kode yang sudah dipakai → `kode tenant sudah dipakai`. Kode milik tenant
yang sudah di-soft delete tapi belum di-purge → ditolak dengan pesan
terpisah, karena schema lamanya masih ada di disk.

### `tenant list`

```powershell
go run ./cmd/cli tenant list
```

Mencetak `code`, `name`, dan `status` untuk maksimal 100 tenant terbaru.

### `tenant suspend` / `tenant activate`

```powershell
go run ./cmd/cli tenant suspend --code=acme_corp
go run ./cmd/cli tenant activate --code=acme_corp
```

Suspend membuat semua login dan semua request tenant tersebut ditolak,
tanpa menyentuh datanya. Activate mengembalikannya.

### `tenant delete` — soft delete

```powershell
go run ./cmd/cli tenant delete --code=acme_corp
```

Menandai tenant terhapus. Schema dan seluruh isinya **masih utuh**, dan
kode tenant menjadi bebas dipakai lagi oleh baris baru — tapi belum bisa,
karena schema lamanya masih menempati nama itu (lihat `tenant create` di
atas). Untuk benar-benar melepaskannya, jalankan purge.

### `tenant purge` — hapus permanen

```powershell
go run ./cmd/cli tenant purge --code=acme_corp --confirm
```

`DROP SCHEMA ... CASCADE` plus penghapusan baris tenant. **Tidak bisa
dibatalkan.** Dua pengaman:

- `--confirm` wajib ada.
- Tenant harus sudah di-soft delete **minimal 30 hari**. Lebih cepat dari
  itu akan ditolak. Ini satu-satunya jaring pengaman terhadap kode tenant
  yang salah ketik.

### `migrate platform up`

```powershell
go run ./cmd/cli migrate platform up
```

Menerapkan migrasi schema `platform`. Jalankan sekali saat setup, lalu
setiap kali ada migrasi platform baru.

### `migrate tenant up`

```powershell
go run ./cmd/cli migrate tenant up --all
go run ./cmd/cli migrate tenant up --tenant=acme_corp
```

Salah satu flag wajib. Dijalankan berurutan per tenant dan **berhenti pada
kegagalan pertama**, dengan pesan yang menyebutkan berapa tenant sudah
selesai — sehingga aman diulang setelah masalahnya diperbaiki.

Tenant yang schema-nya tertinggal akan menolak seluruh request dengan
`TENANT_MIGRATION_PENDING` sampai perintah ini dijalankan.

### `migrate status`

```powershell
go run ./cmd/cli migrate status
```

Mencetak satu baris per tenant:

- `sudah paling baru` — versi schema sama dengan versi terbaru di binary.
- `TERTINGGAL (versi N, terbaru M)` — perlu `migrate tenant up`.
- `RUSAK — perlu diperbaiki manual` — migrasi pernah gagal di tengah jalan;
  perlu diperbaiki manual di database sebelum bisa dilanjutkan.

### `permission sync`

```powershell
go run ./cmd/cli permission sync --all
go run ./cmd/cli permission sync --tenant=acme_corp
```

Menyisipkan permission baru dari `internal/permission/constants.go` ke
tenant yang sudah ada (tenant baru sudah otomatis lengkap dari
provisioning). Aman diulang — mencetak jumlah permission yang benar-benar
ditambahkan per tenant. **Jalankan setiap kali menambah konstanta
permission baru**, kalau tidak, endpoint yang memakainya akan selalu `403`
untuk role non-sistem.

### `cleanup`

```powershell
go run ./cmd/cli cleanup
```

Perawatan rutin, aman dijalankan berkala:

- Menghapus refresh token platform yang sudah dicabut atau kadaluarsa.
- Menghapus `platform.login_attempts` yang lebih tua dari 30 hari.
- Menghapus refresh token kadaluarsa/dicabut di **setiap** schema tenant.

Untuk produksi sudah tersedia unit systemd `app-cleanup.service` dan
`app-cleanup.timer` di `deploy/` — lihat [deployment.md](deployment.md).

---

## 9. Endpoint operasional

### Health check

```powershell
curl http://localhost:8080/health
curl http://localhost:8080/health/ready
```

- `GET /health` → `{"status":"ok"}`. Tidak menyentuh database sama sekali —
  hanya membuktikan proses hidup dan menjawab. Pakai ini untuk liveness
  probe.
- `GET /health/ready` → `{"status":"ready"}` atau **503**
  `{"status":"not ready"}` bila ping database gagal. Pakai ini untuk
  readiness probe / load balancer.

Keduanya tidak memerlukan token dan tidak memakai prefix `/api/v1`.

### Dokumentasi Swagger

```
http://localhost:8080/swagger/index.html
```

Spesifikasinya digenerate dari anotasi di handler:

```powershell
make swagger      # atau: swag init -g cmd/api/main.go -o docs/swagger
```

Jangan mengedit isi `docs/swagger/` secara manual — ubah anotasi di source
Go lalu generate ulang.

---

## 10. Konfigurasi

Semua variabel di bawah **wajib** ada. Bila ada yang kurang atau formatnya
salah, aplikasi berhenti saat start dan mencetak **seluruh** masalah
sekaligus — bukan satu per satu tiap kali dicoba.

| Variabel | Contoh | Keterangan |
|---|---|---|
| `DB_HOST` | `localhost` | |
| `DB_PORT` | `5432` | |
| `DB_USER` | `app` | |
| `DB_PASSWORD` | `changeme` | |
| `DB_NAME` | `appdb` | |
| `DB_SSLMODE` | `disable` | `require` untuk produksi |
| `DB_MAX_OPEN_CONNS` | `25` | |
| `DB_MAX_IDLE_CONNS` | `5` | |
| `DB_CONN_MAX_LIFETIME` | `5m` | Format durasi Go |
| `APP_PORT` | `8080` | |
| `APP_ENV` | `development` | |
| `SHUTDOWN_TIMEOUT` | `15s` | Batas waktu graceful shutdown |
| `JWT_SECRET` | *(acak)* | **Minimal 32 karakter** |
| `ACCESS_TOKEN_TTL` | `15m` | Juga menjadi nilai `expires_in` |
| `REFRESH_TOKEN_TTL` | `168h` | |
| `LOGIN_MAX_ATTEMPTS` | `5` | |
| `LOGIN_ATTEMPT_WINDOW` | `15m` | |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Dipisah koma |

Catatan penting:

- **`JWT_SECRET`** harus benar-benar diganti. Nilai di `.env.example`
  memang terlihat acak, tetapi ada di version control alias publik.
  Pemeriksaan panjang 32 karakter dimaksudkan untuk menangkap deploy yang
  menyalin `.env.example` apa adanya.
- **CORS tidak menerima wildcard.** API ini memakai bearer token dengan
  `AllowCredentials: true`, dan browser menolak kombinasi wildcard +
  credentials. Daftarkan origin secara eksplisit. Header yang diizinkan:
  `Content-Type` dan `Authorization`; method: `GET, POST, PUT, PATCH,
  DELETE, OPTIONS`.
- Mengubah `JWT_SECRET` membatalkan semua access token yang beredar
  (refresh token tetap berlaku, karena disimpan sebagai hash di database,
  bukan ditandatangani).

---

## 11. Perintah Makefile

| Perintah | Setara dengan |
|---|---|
| `make run` | `air` (hot reload) |
| `make build` | `go build -o bin/api ./cmd/api` |
| `make build-cli` | `go build -o bin/cli ./cmd/cli` |
| `make test` | `go test ./... -short` |
| `make test-integration` | `REQUIRE_TEST_DB=1 go test ./... -v` |
| `make migrate-platform` | `cli migrate platform up` |
| `make migrate-tenant` | `cli migrate tenant up --all` |
| `make swagger` | Generate ulang `docs/swagger/` |

`make test` melewati test yang butuh database. `make test-integration`
bersifat fail-closed: test yang biasanya di-skip saat PostgreSQL tidak
terjangkau justru **gagal**. Jalankan yang kedua sebelum merge atau
deploy.

---

## 12. Alur lengkap dari nol

```powershell
# 1. Siapkan schema platform
go run ./cmd/cli migrate platform up

# 2. Admin platform pertama (catat password yang tercetak)
go run ./cmd/cli admin create --email=admin@app.test --name="Admin Utama"

# 3. Tenant pertama (catat password owner yang tercetak)
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test

# 4. Jalankan API
air

# 5. Login sebagai owner tenant
curl -X POST http://localhost:8080/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<password owner>\"}"

# 6. Pakai access token-nya (owner punya semua permission)
curl -X POST http://localhost:8080/api/v1/users `
  -H "Authorization: Bearer <access_token>" `
  -H "Content-Type: application/json" `
  -d "{\"email\":\"kasir@acme.test\",\"password\":\"rahasia123\",\"name\":\"Kasir 1\"}"

curl http://localhost:8080/api/v1/users -H "Authorization: Bearer <access_token>"
```

Setelah menambah migrasi tenant baru:

```powershell
go run ./cmd/cli migrate tenant up --all
go run ./cmd/cli migrate status
```

Setelah menambah konstanta permission baru:

```powershell
go run ./cmd/cli permission sync --all
```

---

## 13. Pemecahan masalah

| Gejala | Penyebab paling mungkin | Tindakan |
|---|---|---|
| Aplikasi berhenti saat start dengan daftar masalah | `.env` belum lengkap/benar | Perbaiki sesuai daftar yang dicetak (§10) |
| `409 TENANT_MIGRATION_PENDING` di semua endpoint | Schema tenant tertinggal | `cli migrate status` lalu `cli migrate tenant up` |
| `403 token ini bukan untuk staf tenant` | Memakai token platform di endpoint tenant | Login lewat `/api/v1/auth/login` |
| `403 tenant tidak aktif` | Tenant di-suspend | `cli tenant activate --code=...` |
| `403 tidak punya izin untuk aksi ini` | User belum punya role/permission | Beri role; setelah menambah permission baru jalankan `permission sync` |
| Perubahan status tenant/role belum terasa | Cache 1 menit di memori | Tunggu maksimal 1 menit |
| `429` saat login | Rate limit tercapai | Tunggu `LOGIN_ATTEMPT_WINDOW` berlalu (§5) |
| `500` tanpa detail | Memang disembunyikan dari klien | Cari `request_id` yang sama di log server |
| `404 rute tidak ditemukan` | Salah path/method | Bandingkan dengan Swagger di `/swagger/index.html` |
