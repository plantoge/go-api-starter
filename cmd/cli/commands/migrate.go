package commands

import (
	"database/sql"
	"flag"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go-api-starter/internal/config"
	"go-api-starter/internal/migration"
)

func init() {
	Register("migrate platform up", cmdMigratePlatformUp)
	Register("migrate tenant up", cmdMigrateTenantUp)
	Register("migrate status", cmdMigrateStatus)
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

type tenantTarget struct {
	code       string
	schemaName string
}

func tenantSchemasToMigrate(db *sql.DB, all bool, code string) ([]tenantTarget, error) {
	query := "SELECT code, schema_name FROM platform.tenants WHERE deleted_at IS NULL"
	var args []any
	if !all {
		if code == "" {
			return nil, fmt.Errorf("pass --all or --tenant=<code>")
		}
		query += " AND code = $1"
		args = append(args, code)
	}
	query += " ORDER BY code"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var targets []tenantTarget
	for rows.Next() {
		var t tenantTarget
		if err := rows.Scan(&t.code, &t.schemaName); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func cmdMigrateTenantUp(args []string) error {
	fs := flag.NewFlagSet("migrate tenant up", flag.ExitOnError)
	all := fs.Bool("all", false, "apply to every tenant")
	tenantCode := fs.String("tenant", "", "apply to a single tenant by code")
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

	targets, err := tenantSchemasToMigrate(db, *all, *tenantCode)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no matching tenants found")
	}

	done := 0
	for _, tgt := range targets {
		fmt.Printf("migrating %s ... ", tgt.code)
		if err := migration.MigrateTenantUp(db, tgt.schemaName); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("tenant %s: %w (stopped after %d of %d tenants)",
				tgt.code, err, done, len(targets))
		}
		fmt.Println("ok")
		done++
	}
	return nil
}

func cmdMigrateStatus(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openRawDB(cfg.DB.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	targets, err := tenantSchemasToMigrate(db, true, "")
	if err != nil {
		return err
	}

	latest := migration.LatestTenantVersion()
	for _, tgt := range targets {
		version, dirty, err := migration.TenantSchemaVersion(db, tgt.schemaName)
		if err != nil {
			fmt.Printf("%-20s ERROR: %v\n", tgt.code, err)
			continue
		}
		status := "up to date"
		switch {
		case dirty:
			status = "DIRTY — needs manual repair"
		case version < latest:
			status = fmt.Sprintf("BEHIND (at %d, latest %d)", version, latest)
		}
		fmt.Printf("%-20s %s\n", tgt.code, status)
	}
	return nil
}
