package database

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/config"
)

// NewPool opens the main application connection pool, used for tenant-
// scoped queries via WithTenant. search_path is left at the connection
// default; WithTenant pins it per-transaction.
func NewPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

// NewPlatformPool opens a pool pinned to search_path=platform on every
// connection, so platform repositories write unqualified table names
// ("FROM tenants") and never need WithTenant.
func NewPlatformPool(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("pgx", cfg.PlatformDSN())
	if err != nil {
		return nil, fmt.Errorf("connect to platform schema: %w", err)
	}
	configurePool(db, cfg)
	return db, nil
}

func configurePool(db *sqlx.DB, cfg config.DBConfig) {
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}
