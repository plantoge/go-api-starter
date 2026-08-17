package database

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/config"
)

// NewPool buka pool koneksi utama aplikasi, yang dipakai buat query
// ber-scope tenant lewat WithTenant. search_path-nya dibiarkan di nilai
// bawaan koneksi; WithTenant yang nanti nguncinya per transaksi.
func NewPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

// NewPlatformPool buka pool yang tiap koneksinya dikunci ke
// search_path=platform. Jadi repository platform bisa nulis nama tabel
// tanpa prefix ("FROM tenants") dan nggak pernah butuh WithTenant.
func NewPlatformPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.PlatformDSN())
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke schema platform: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

func configurePool(db *sqlx.DB, cfg config.DBConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}
