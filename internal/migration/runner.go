package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func newMigrate(db *sql.DB, fsys embed.FS, dir, schemaName string) (*migrate.Migrate, error) {
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migration source %s: %w", dir, err)
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{
		SchemaName:      schemaName,
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return nil, fmt.Errorf("migration driver for schema %s: %w", schemaName, err)
	}
	return migrate.NewWithInstance("iofs", src, schemaName, driver)
}

// MigratePlatformUp applies every unapplied migration under platform/ to
// the platform schema, creating that schema first if it doesn't exist yet
// (golang-migrate's postgres driver requires the schema to already exist
// before it opens, so schema creation can't be a migration file itself).
func MigratePlatformUp(db *sql.DB) error {
	if _, err := db.Exec("CREATE SCHEMA IF NOT EXISTS platform"); err != nil {
		return fmt.Errorf("create platform schema: %w", err)
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

// LatestPlatformVersion returns the highest migration version compiled
// into this binary under platform/.
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

// MigrationFile is one applied-in-order .up.sql file, exposed so callers
// (tenant provisioning, Task 13) that need to apply migrations by hand
// inside their own transaction — something golang-migrate itself can't
// join, since it manages its own connection — can read the exact same
// files the CLI's `migrate` commands use.
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
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files = append(files, MigrationFile{Version: uint(version), SQL: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}
