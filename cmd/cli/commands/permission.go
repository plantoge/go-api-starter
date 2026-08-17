package commands

import (
	"context"
	"flag"
	"fmt"

	"go-api-starter/internal/config"
	"go-api-starter/internal/permission"
)

func init() {
	Register("permission sync", cmdPermissionSync)
}

func cmdPermissionSync(args []string) error {
	fs := flag.NewFlagSet("permission sync", flag.ExitOnError)
	all := fs.Bool("all", false, "sinkronkan semua tenant")
	tenantCode := fs.String("tenant", "", "sinkronkan satu tenant saja, berdasarkan kode")
	fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, *all, *tenantCode) // helper bareng dari migrate.go
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("tidak ada tenant yang cocok")
	}

	for _, tgt := range targets {
		added, err := permission.Sync(context.Background(), db, tgt.schemaName)
		if err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s +%d permission\n", tgt.code, added)
	}
	return nil
}
