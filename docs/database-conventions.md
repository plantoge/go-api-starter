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

- `updated_at` dikelola oleh trigger per schema (`trigger_set_updated_at`) —
  pasang trigger ini di setiap tabel baru:
  ```sql
  CREATE TRIGGER set_updated_at
      BEFORE UPDATE ON your_table
      FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
  ```
- Kolom `*_by` diisi dari `database.ActorFromContext(ctx)` di lapisan
  repository, tidak pernah dibiarkan ditebak oleh database.
- **Kolom `*_by` tidak pernah diisi `NULL` oleh kode.** Aksi yang tidak
  punya user login tetap punya pelaku, hanya saja pelakunya bukan orang:
  operasi lifecycle platform-tenant lewat CLI
  (`cmd/cli/commands/tenant.go`) berjalan dengan actor sentinel tetap
  (`database.Actor{UserID: uuid.Nil, Scope: "cli"}`). Kalau kamu melihat
  `00000000-0000-0000-0000-000000000000` di kolom `*_by`, artinya aksi itu
  dilakukan lewat CLI atau proses sistem — bukan berarti jejak auditnya
  rusak.
- Konsekuensi yang gampang terlewat: `database.ActorFromContext` dipanggil
  dengan pola `actor, _ := ...` di seluruh repository, sehingga context
  tanpa actor pun menghasilkan `uuid.Nil`, bukan panic dan bukan `NULL`.
  Jadi `uuid.Nil` di kolom `*_by` berarti "bukan user di dalam tenant ini",
  titik — ia tidak membedakan CLI dari jalur yang lupa memasang actor.
- Kolom `*_by` di tabel tenant selalu merujuk user di schema tenant yang
  sama. Saat provisioning, user owner pertama dibuat oleh admin platform
  yang datanya ada di `platform.users` — schema berbeda, dan UUID lintas
  schema tidak bisa dijamin lewat foreign key. Karena itu jalur tersebut
  memakai sentinel yang sama, bukan UUID admin platform tersebut.
- ID bertipe `UUID PRIMARY KEY` **tanpa default di sisi database** —
  generate dengan `uuid.New()` di Go sebelum melakukan insert. (Transaksi
  dengan `SET LOCAL search_path` hanya mencari di schema milik tenant itu
  sendiri, bukan di `public`, sehingga default sisi DB yang memanggil fungsi
  extension di sana akan gagal secara tidak terduga. Membuat ID di Go
  menghindari masalah ini sepenuhnya.)

> Dokumen desain `docs/superpowers/specs/2026-08-06-...design.md` (bagian
> "Aturan kolom `*_by` lintas schema") menetapkan `NULL` untuk pelaku
> non-user. Keputusan itu diganti sentinel `uuid.Nil` saat implementasi.
> Spec adalah arsip bertanggal dan tidak diedit — **berkas inilah yang
> berlaku.**

## Soft delete

Setiap query memfilter `WHERE deleted_at IS NULL` kecuali query itu memang
sengaja mencari baris yang sudah dihapus (seperti pengecekan 30 hari pada
`tenant purge`).

**Tidak ada helper yang memaksakan filter ini.** Tidak ada paket
`querybuilder` maupun repository dasar di repo ini — dokumen desain
menyebutnya, implementasinya tidak dibangun. Setiap repository menulis
`deleted_at IS NULL` sendiri di setiap query, jadi ini hal yang harus
diperiksa mata saat review, bukan sesuatu yang dijamin oleh kode.

Kolom apa pun yang punya syarat keunikan membutuhkan unique index
**parsial** supaya nilai milik baris yang sudah dihapus bisa dipakai ulang:
```sql
CREATE UNIQUE INDEX users_email_key ON users (email) WHERE deleted_at IS NULL;
```

## Pengecualian

**Tabel join** (`role_permissions`, `user_roles`): hanya `created_at` +
`created_by`. Akses dicabut dengan `DELETE` permanen, bukan soft delete —
menumpuk filter `deleted_at IS NULL` di setiap pengecekan permission tanpa
manfaat nyata tidak sepadan, dan riwayat pemberian/pencabutan akses
tempatnya di log, bukan di tabel join.

**Tabel log/token** (`refresh_tokens`, `login_attempts`): hanya
`created_at`. Tabel ini bersifat append-only — tidak pernah ada baris yang
di-update atau di-soft-delete. Refresh token yang dicabut diwakili oleh
kolomnya sendiri, `revoked_at`, bukan `deleted_at` standar.

## Penamaan

- Tabel: bentuk jamak, `snake_case` (`users`, `role_permissions`).
- Kolom: `snake_case`, tanpa prefix tipe (`email`, bukan `str_email`).
- Foreign key: `<singular_table>_id` (`user_id`, `role_id`).
