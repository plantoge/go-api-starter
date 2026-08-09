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
- Operasi lifecycle platform-tenant yang dijalankan lewat CLI
  (`cmd/cli/commands/tenant.go`) tidak punya user yang sedang login untuk
  diatribusikan, jadi operasi itu berjalan dengan actor sentinel tetap
  (`database.Actor{UserID: uuid.Nil, Scope: "cli"}`) — kalau kamu melihat
  `00000000-0000-0000-0000-000000000000` di kolom `*_by`, artinya aksi itu
  dilakukan lewat CLI, bukan berarti jejak auditnya rusak.
- ID bertipe `UUID PRIMARY KEY` **tanpa default di sisi database** —
  generate dengan `uuid.New()` di Go sebelum melakukan insert. (Transaksi
  dengan `SET LOCAL search_path` hanya mencari di schema milik tenant itu
  sendiri, bukan di `public`, sehingga default sisi DB yang memanggil fungsi
  extension di sana akan gagal secara tidak terduga. Membuat ID di Go
  menghindari masalah ini sepenuhnya.)

## Soft delete

Setiap query memfilter `WHERE deleted_at IS NULL` kecuali query itu memang
sengaja mencari baris yang sudah dihapus (seperti pengecekan 30 hari pada
`tenant purge`). Kolom apa pun yang punya syarat keunikan membutuhkan
unique index **parsial** supaya nilai milik baris yang sudah dihapus bisa
dipakai ulang:
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
