# Memulai (pengembangan di Windows)

## 1. Pasang prasyarat

- Go 1.25+
- PostgreSQL, berjalan secara lokal
- `air` untuk hot reload: `go install github.com/air-verse/air@latest`

## 2. Buat database

```powershell
psql -U postgres -c "CREATE ROLE app LOGIN PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE appdb OWNER app;"
psql -U postgres -c "CREATE DATABASE app_test OWNER app;"
```

## 3. Konfigurasi environment

```powershell
copy .env.example .env
copy .env.test.example .env.test
```
Sesuaikan `DB_PASSWORD` di kedua file jika kamu memakai password yang
berbeda dari perintah di atas.

## 4. Jalankan migrasi dan buat admin pertama

                 Salin password yang tercetak — password itu hanya ditampilkan sekali.

## 5. Buat tenant pertama

```powershell
go run ./cmd/cli tenant create --code=acme_corp --name="Acme Corp" --owner-email=owner@acme.test
```
Salin juga password owner yang tercetak.

## 6. Jalankan API

```powershell
air
```
atau tanpa hot reload:
```powershell
go run ./cmd/api
```

## 7. Coba

```powershell
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"tenant_code\":\"acme_corp\",\"email\":\"owner@acme.test\",\"password\":\"<owner password>\"}"
```

## Menjalankan test

```powershell
go test ./...
```
Integration test terhubung ke `app_test` menggunakan `.env.test` dan otomatis
di-skip jika PostgreSQL tidak bisa dijangkau. Sebelum merge atau deploy,
jalankan `REQUIRE_TEST_DB=1 go test ./...` — mode fail-closed ini mengubah
skip tersebut menjadi kegagalan ketika database seharusnya tersedia,
sehingga bug yang hanya muncul terhadap database sungguhan ikut tertangkap.
