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
	all := fs.Bool("all", false, "sync every tenant")
	tenantCode := fs.String("tenant", "", "sync a single tenant by code")
	fs.Parse(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, *all, *tenantCode) // shared helper from migrate.go
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no matching tenants found")
	}

	for _, tgt := range targets {
		added, err := permission.Sync(context.Background(), db, tgt.schemaName)
		if err != nil {
			return fmt.Errorf("tenant %s: %w", tgt.code, err)
		}
		fmt.Printf("%-20s +%d permission(s)\n", tgt.code, added)
	}
	return nil
}
