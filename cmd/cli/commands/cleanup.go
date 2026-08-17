package commands

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
)

func init() {
	Register("cleanup", cmdCleanup)
}

const loginAttemptRetention = 30 * 24 * time.Hour

func cmdCleanup(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`DELETE FROM platform.refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < now()`); err != nil {
		return fmt.Errorf("gagal membersihkan refresh_tokens platform: %w", err)
	}
	cutoff := time.Now().Add(-loginAttemptRetention)
	if _, err := db.Exec(`DELETE FROM platform.login_attempts WHERE attempted_at < $1`, cutoff); err != nil {
		return fmt.Errorf("gagal membersihkan login_attempts platform: %w", err)
	}
	fmt.Println("platform: sudah dibersihkan")

	targets, err := tenantSchemasToMigrate(db, true, "") // helper bareng dari migrate.go
	if err != nil {
		return err
	}
	for _, tgt := range targets {
		if err := cleanupTenantSchema(db, tgt.schemaName); err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s sudah dibersihkan\n", tgt.code)
	}
	return nil
}

func cleanupTenantSchema(db *sql.DB, schemaName string) error {
	if !database.ValidSchemaName(schemaName) {
		return fmt.Errorf("nama schema tidak valid: %q", schemaName)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`SET LOCAL search_path TO "` + schemaName + `"`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM refresh_tokens WHERE revoked_at IS NOT NULL OR expires_at < now()`); err != nil {
		return err
	}
	return tx.Commit()
}
