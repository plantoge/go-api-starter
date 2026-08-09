# Menambahkan Modul

Setiap modul tenant-scoped mengikuti bentuk
`internal/modules/tenant/user/` — salin folder itu sebagai titik awal.

## Bentuk file

```
modules/tenant/<name>/
  model.go       # struct baris DB, tag `db:"..."`
  dto.go         # struct request/response, tag `json:"..."` + `validate:"..."`
  repository.go  # SQL via sqlx, selalu lewat database.WithTenant
  service.go     # business logic, context.Context saja, tanpa tipe HTTP
  handler.go     # Fiber saja, konversi ke/dari DTO, tidak pernah menyentuh SQL
```

## Langkah-langkah

1. **Tambahkan migrasi.** Buat
   `internal/migration/tenant/0000N_create_<name>_table.up.sql` (dan
   `.down.sql`), mengikuti `database-conventions.md` untuk kolom audit dan
   soft delete. Naikkan nomor versinya.
2. **Tulis `model.go`** — satu struct per tabel, tag `db:"..."` yang persis
   sama dengan nama kolomnya.
3. **Tulis `repository.go`** — setiap method membungkus query-nya dalam
   `db.WithTenant(ctx, func(tx *sqlx.Tx) error {...})`. Deklarasikan
   whitelist `Sortable map[string]string` untuk apa pun yang boleh dipakai
   mengurutkan di endpoint list — jangan pernah menerima nilai query `sort`
   mentah.
4. **Tulis `dto.go`** — struct request dengan tag `validate:"..."`, sebuah
   struct `View` untuk response, dan mapper `toView`.
5. **Tulis `service.go`** — hanya bergantung pada repository dan
   `context.Context`. Hashing password, pemanggilan eksternal, atau
   pemanggilan lintas modul (lewat *service* modul lain, tidak pernah lewat
   repository-nya) ditempatkan di sini.
6. **Tulis `handler.go`** — parse body, panggil `validator.Validate`,
   panggil service, terjemahkan hasilnya dengan `response.Success` /
   `response.Error`. Jangan pernah menulis `c.Status(...)` langsung.
7. **Tambahkan konstanta permission** di `internal/permission/constants.go`
   untuk resource baru (`<name>.view`, `<name>.create`, ...), mengikuti pola
   `user.*` yang sudah ada.
8. **Sambungkan route** di `internal/server/router.go`, di dalam grup
   `tenantAPI`, masing-masing di balik
   `middleware.RequirePermission(permission.YourConstant, deps.PermissionCache)`.
9. **Tambahkan handler ke `server.Dependencies`** dan buat instance-nya di
   `cmd/api/main.go`.
10. **Jalankan `cli permission sync --all`** setelah deploy, supaya tenant
    yang sudah ada mendapatkan baris permission yang baru.

## Pengelompokan sub-modul

Biarkan `modules/tenant/` tetap rata sampai isinya mencapai 7–8 modul dan
navigasinya mulai menyulitkan. Setelah itu kelompokkan berdasarkan domain,
misalnya `modules/tenant/sales/order/`,
`modules/tenant/inventory/product/` — bentuk modulnya sendiri tidak
berubah, hanya path-nya. **Modul tidak boleh memanggil repository modul
lain secara langsung** — interaksi lintas modul selalu lewat *service*
modul tersebut.
