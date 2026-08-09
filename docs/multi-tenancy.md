# Aturan Multi-Tenancy

Dua aturan di bawah inilah yang mencegah data tenant A muncul di response
tenant B. Melanggar salah satunya adalah kebocoran data, bukan sekadar bug.

## Aturan 1: semua akses data tenant lewat `database.WithTenant`

```go
// Benar
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	})
	return u, err
}
```
```go
// Salah — melewati tenant scoping sepenuhnya
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.pool.Get(&u, `SELECT * FROM users WHERE id = $1`, id) // tenant yang mana?!
	return u, err
}
```
`WithTenant` menjalankan query di dalam sebuah transaksi dengan `SET LOCAL
search_path` yang dipatok ke schema milik tenant pemanggil — diambil dari
`context.Context`, tidak pernah dari parameter request. `SET LOCAL` otomatis
dibersihkan saat transaksi berakhir (commit atau rollback), sehingga sebuah
koneksi tidak pernah bisa membawa schema satu tenant ke pemanggil berikutnya
yang meminjam koneksi itu dari pool.

## Aturan 2: `*fiber.Ctx` berhenti di handler

Fiber dibangun di atas `fasthttp`, yang **memakai ulang** object context-nya
antar request demi performa. Kalau sebuah `*fiber.Ctx` (atau apa pun yang
diturunkan darinya) lolos ke dalam goroutine atau disimpan di suatu tempat,
isinya bisa diam-diam tertimpa oleh request yang sama sekali berbeda —
termasuk request milik tenant lain — sebelum sempat dibaca kembali.

```go
// Benar — handler mengonversi semua yang dibutuhkan sebelum memanggil service
func (h *Handler) Get(c *fiber.Ctx) error {
	id, _ := uuid.Parse(c.Params("id"))
	view, err := h.svc.Get(c.UserContext(), id) // mulai dari sini ke bawah: context.Context saja
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
		log.Println(c.Params("id")) // c bisa jadi sudah milik request lain sekarang
	}()
	return nil
}
```
Service dan repository hanya boleh melihat `context.Context`. Ini bukan
sekadar aturan keamanan — efek sampingnya, business logic bisa di-unit test
tanpa menjalankan HTTP server.

## Apa saja yang ada di dalam `context.Context`

Diisi oleh middleware, dibaca lewat `database.TenantFromContext` /
`database.ActorFromContext`:
- `database.TenantInfo` — ID tenant + nama schema, diisi oleh `RequireTenant`
- `database.Actor` — ID user + scope, diisi oleh `RequirePlatform` /
  `RequireTenant`

Satu-satunya pengecualian adalah login: pada titik itu belum ada token yang
membuktikan identitas tenant, jadi `modules/tenant/auth.Service.Login`
menyusun `TenantInfo` sendiri dari `tenant_code` yang sudah diresolusi,
sebelum memanggil `WithTenant`. Semua jalur kode tenant-scoped lainnya
menerimanya dalam keadaan sudah diisi oleh middleware.
