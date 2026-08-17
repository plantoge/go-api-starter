package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"go-api-starter/internal/database"
)

func newMigrate(db *sql.DB, fsys embed.FS, dir, schemaName string) (*migrate.Migrate, error) {
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca sumber migrasi %s: %w", dir, err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{
		SchemaName:      schemaName,
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return nil, fmt.Errorf("gagal menyiapkan driver migrasi untuk schema %s: %w", schemaName, err)
	}
	return migrate.NewWithInstance("iofs", src, schemaName, driver)
}

// MigratePlatformUp nerapin semua migrasi di folder platform/ yang belum
// dijalankan ke schema platform, sambil bikin schema-nya dulu kalau memang
// belum ada. (Driver postgres punya golang-migrate nuntut schema-nya sudah
// ada sebelum dia dibuka, jadi pembuatan schema nggak bisa ditaruh sebagai
// file migrasi.)
func MigratePlatformUp(db *sql.DB) error {
	if _, err := db.Exec("CREATE SCHEMA IF NOT EXISTS platform"); err != nil {
		return fmt.Errorf("gagal membuat schema platform: %w", err)
	}
	m, err := newMigrate(db, PlatformFS, "platform", "platform")
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// LatestPlatformVersion ngasih versi migrasi tertinggi di folder platform/
// yang ikut dikompilasi ke binary ini.
func LatestPlatformVersion() uint {
	return latestVersion(PlatformFS, "platform")
}

func latestVersion(fsys embed.FS, dir string) uint {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return 0
	}
	var max uint
	for _, e := range entries {
		underscore := strings.IndexByte(e.Name(), '_')
		if underscore <= 0 {
			continue
		}
		n, err := strconv.ParseUint(e.Name()[:underscore], 10, 64)
		if err != nil {
			continue
		}
		if uint(n) > max {
			max = uint(n)
		}
	}
	return max
}

// MigrationFile mewakili satu file .up.sql yang dijalankan berurutan.
// Dibuka ke luar supaya pemanggil yang harus nerapin migrasi manual di
// dalam transaksinya sendiri (penyiapan tenant, Task 13) bisa baca file
// yang persis sama dengan yang dipakai perintah `migrate` di CLI.
// golang-migrate sendiri nggak bisa ikut nebeng transaksi orang lain,
// soalnya dia ngurus koneksinya sendiri.
type MigrationFile struct {
	Version uint
	SQL     string
}

func readUpFiles(fsys embed.FS, dir string) ([]MigrationFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var files []MigrationFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		underscore := strings.IndexByte(name, '_')
		if underscore <= 0 {
			continue
		}
		version, err := strconv.ParseUint(name[:underscore], 10, 64)
		if err != nil {
			continue
		}
		content, err := fsys.ReadFile(dir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca %s: %w", name, err)
		}
		files = append(files, MigrationFile{Version: uint(version), SQL: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}

// MigrateTenantUp nerapin semua migrasi di folder tenant/ yang belum
// dijalankan ke schema tenant yang dituju. Schema-nya harus sudah ada —
// proses penyiapan tenant yang bikin duluan sebelum manggil fungsi ini.
//
// Catatan perbaikan: opsi konfigurasi SchemaName di driver postgres punya
// golang-migrate cuma ngatur letak tabel pencatat schema_migrations
// miliknya sendiri. Dia sama sekali nggak nyetel search_path di koneksi,
// jadi isi SQL di file migrasinya sendiri nggak ikut terarahkan.
//
// Padahal file migrasi tenant sengaja ditulis tanpa prefix schema, karena
// nama schema tenant baru ketahuan pas penyiapan. Desain itu sepenuhnya
// bergantung pada search_path yang sudah dikunci benar sebelum file-nya
// dijalankan. Sebelum diperbaiki, semua migrasi tenant diam-diam jalan di
// "public", bukan di schema tenant yang dimaksud. Ini nggak kelihatan
// sampai fungsinya beneran dicoba ke PostgreSQL sungguhan — dan itu nggak
// pernah kejadian sampai titik ini, karena semua test yang butuh database
// selama ini cuma di-skip. Ketahuannya pas tenant kedua dimigrasi dengan
// cara yang sama dan langsung tabrakan "relation already exists". Migrasi
// platform nggak kena masalah ini, soalnya SQL-nya nulis prefix
// "platform." di tiap objek, jadi memang nggak pernah bergantung ke
// search_path.
//
// Yang diminta di sini dsn, bukan *sql.DB yang sudah terbuka, karena
// perbaikannya buka *sql.DB sendiri dengan satu koneksi khusus yang
// diarahkan ke schemaName lewat parameter "options" di koneksi. *sql.DB
// hasil pooling yang dipakai bareng nggak bisa diarahkan ulang per
// panggilan dengan aman.
func MigrateTenantUp(dsn string, schemaName string) error {
	scopedDB, err := openScopedDB(dsn, schemaName)
	if err != nil {
		return fmt.Errorf("gagal terhubung ke database (diarahkan ke %s): %w", schemaName, err)
	}
	defer scopedDB.Close()

	m, err := newMigrate(scopedDB, TenantFS, "tenant", schemaName)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// openScopedDB buka *sql.DB khusus berisi satu koneksi, yang
// search_path-nya dikunci ke schemaName lewat parameter "options" di
// koneksi. Efeknya setara `psql -c "SET search_path TO schemaName"` saat
// connect, dan berlaku ke tiap statement di koneksi itu.
//
// Sengaja dikunci ke tepat satu koneksi fisik (SetMaxOpenConns(1)) biar
// setelan search_path-nya nggak bisa diam-diam hilang gara-gara statement
// berikutnya dilempar ke koneksi lain di pool yang nggak pernah disetel.
// Pemanggil wajib manggil Close() setelah selesai — ini handle sekali
// pakai buat satu keperluan, bukan buat dipakai bareng atau dibiarkan
// hidup lama.
func openScopedDB(dsn, schemaName string) (*sql.DB, error) {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	scopedDSN := dsn + sep + "options=-c%20search_path%3D" + url.QueryEscape(schemaName)
	db, err := sql.Open("pgx", scopedDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// TenantSchemaVersion ngasih tahu versi migrasi yang sekarang terpasang di
// schemaName, plus apakah statusnya dirty (migrasi gagal di tengah jalan).
// Kalau belum ada migrasi yang pernah dijalankan, hasilnya version 0 dan
// dirty=false. schemaName harus sudah disiapkan lebih dulu (tabel
// schema_migrations-nya ada) — semua pemanggil fungsi ini cuma sampai ke
// sini buat tenant yang memang sudah dibuat dan dimigrasi sama Provision
// (Task 14).
func TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error) {
	if !database.ValidSchemaName(schemaName) {
		return 0, false, fmt.Errorf("nama schema tidak valid: %q", schemaName)
	}
	query := `SELECT version, dirty FROM "` + schemaName + `".schema_migrations LIMIT 1`
	var v int64
	var d bool
	err = db.QueryRow(query).Scan(&v, &d)
	switch {
	case err == sql.ErrNoRows:
		return 0, false, nil
	case err != nil:
		return 0, false, err
	default:
		return uint(v), d, nil
	}
}

// LatestTenantVersion ngasih versi migrasi tertinggi di folder tenant/
// yang ikut dikompilasi ke binary ini — versi yang seharusnya dipakai tiap
// schema tenant. Dipakai `cli migrate status` dan penjaga
// TENANT_MIGRATION_PENDING saat aplikasi jalan (Task 11).
func LatestTenantVersion() uint {
	return latestVersion(TenantFS, "tenant")
}

// TenantMigrationFiles ngasih SQL up-migration di folder tenant/, urut
// berdasarkan versi, buat pemanggil yang harus nerapin migrasi di dalam
// transaksi yang sudah dia buka sendiri. golang-migrate ngurus koneksinya
// sendiri dan nggak bisa ikut transaksi orang lain, jadi penyiapan tenant
// (Task 13) baca dan jalanin file-file ini langsung, bukan manggil
// MigrateTenantUp.
func TenantMigrationFiles() ([]MigrationFile, error) {
	return readUpFiles(TenantFS, "tenant")
}
