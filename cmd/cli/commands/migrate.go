package commands

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go-api-starter/internal/config"
	"go-api-starter/internal/migration"
)

func init() {
	Register("migrate platform up", cmdMigratePlatformUp)
}

func openRawDB(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

func cmdMigratePlatformUp(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	if err := migration.MigratePlatformUp(db); err != nil {
		return fmt.Errorf("migrate platform: %w", err)
	}
	fmt.Println("platform migrations applied")
	return nil
}
