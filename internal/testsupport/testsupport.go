// Package testsupport nyediain helper buat test yang butuh koneksi
// PostgreSQL sungguhan. Package ini cuma boleh di-import dari file
// _test.go — dia bergantung ke "testing", yang nggak pantas ikut kebawa ke
// binary yang dirilis.
package testsupport

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// requireTestDB ngecek apakah REQUIRE_TEST_DB diisi nilai yang berarti
// "iya". Kalau cuma dicek pakai os.Getenv(...) != "", nilai
// REQUIRE_TEST_DB=0 atau =false bakal ikut dianggap "aktif", dan itu bikin
// kaget. Pakai strconv.ParseBool, nilai mematikan yang biasa
// (0/false/f/...) punya arti sesuai harapan, dan apa pun yang nggak bisa
// dibaca (termasuk kalau nggak diisi) dianggap mati.
func requireTestDB() bool {
	b, _ := strconv.ParseBool(os.Getenv("REQUIRE_TEST_DB"))
	return b
}

// TestDSN ngasih string koneksi yang bakal dipakai OpenTestDB, tanpa
// beneran nyambung. Berguna buat pemanggil (misalnya
// migration.MigrateTenantUp) yang butuh string DSN, bukan *sqlx.DB yang
// sudah terbuka — umpamanya karena mau buka koneksi sendiri yang khusus
// diarahkan ke satu schema.
func TestDSN() string {
	_ = godotenv.Load(findEnvTestFile())
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)
}

// OpenTestDB nyambung ke database app_test sesuai .env.test di root repo.
// Kalau PostgreSQL lokal nggak bisa dijangkau, test pemanggilnya di-skip,
// bukan digagalkan — jadi `go test ./...` tetap bisa dipakai di mesin yang
// belum nyiapin database. Mesin yang sudah nyiapin tetap dapat cakupan
// test penuh.
func OpenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := TestDSN()

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		if requireTestDB() {
			t.Fatalf("REQUIRE_TEST_DB aktif tapi database test PostgreSQL lokal nggak bisa dijangkau: %v", err)
		}
		t.Skipf("dilewati: database test PostgreSQL lokal nggak bisa dijangkau (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// OpenTestPlatformDB itu OpenTestDB dengan search_path dikunci ke
// "platform", niru database.NewPlatformPool di produksi. Dipakai buat
// nge-test kode yang nulis nama tabel tanpa prefix ("FROM login_attempts")
// karena memang mengandaikan koneksinya sudah dikunci ke platform.
func OpenTestPlatformDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := TestDSN() + "&search_path=platform"

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		if requireTestDB() {
			t.Fatalf("REQUIRE_TEST_DB aktif tapi database test PostgreSQL lokal nggak bisa dijangkau: %v", err)
		}
		t.Skipf("dilewati: database test PostgreSQL lokal nggak bisa dijangkau (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// RandomSchemaName ngasih nama macam "test_a1b2c3d4" yang lolos regex nama
// schema tenant, buat test yang butuh schema sekali pakai. Sumber acaknya
// langsung dari math/rand tingkat package (otomatis di-seed sejak Go
// 1.20), bukan bikin sumber baru tiap panggilan — cara kedua itu bisa
// ngasih nama kembar kalau dipanggil beruntun di mesin yang jamnya kurang
// halus.
func RandomSchemaName() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "test_" + string(b)
}

// SeedInSchema jalanin query penyiapan data uji dengan search_path dikunci
// ke schema tertentu, buat test yang perlu nyisipin baris langsung tanpa
// lewat database.WithTenant.
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findEnvTestFile() string {
	// Catatan perbaikan: tiga kandidat awalnya cuma naik sampai 3 tingkat,
	// yang nggak pernah nyampe root repo kalau dipanggil dari package
	// sedalam 4 tingkat (misalnya internal/modules/tenant/auth). Akibatnya
	// package-package itu diam-diam balik ke nilai bawaan
	// DB_USER/DB_PASSWORD, bukan nilai asli dari .env.test. Sekarang
	// dipanjangin sampai 5 tingkat; package terdalam di repo ini ada di
	// tingkat 4.
	candidates := []string{".env.test", "../../.env.test", "../../../.env.test", "../../../../.env.test", "../../../../../.env.test"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ".env.test"
}
