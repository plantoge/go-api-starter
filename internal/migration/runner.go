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

// MigrateTenantUp applies every unapplied migration under tenant/ to the
// given tenant schema, which must already exist (provisioning creates it
// before calling this).
//
// Amendment: golang-migrate's postgres driver's SchemaName config option
// only scopes its own schema_migrations tracking table — it never sets
// search_path on the connection, so it does nothing to scope the migration
// files' own SQL bodies. Tenant migration files are intentionally written
// unqualified (no schema prefix), since the tenant's schema name isn't
// known until provisioning time — that design relies entirely on
// search_path being correctly pinned before the files execute. Without
// this fix, every tenant migration silently ran against "public" instead
// of the intended tenant schema: invisible until this function was
// actually exercised against a live PostgreSQL database (it never was,
// until this point — every DB-dependent test had only ever skipped). A
// second tenant migrated this way collided immediately with "relation
// already exists", which is how this was caught. Platform migrations are
// unaffected — their SQL fully qualifies every object with a "platform."
// prefix, so they never depended on search_path in the first place.
//
// dsn (not an already-open *sql.DB) is required here because the fix opens
// its own dedicated, single-connection *sql.DB scoped to schemaName via
// the connection's "options" parameter — a shared pooled *sql.DB can't be
// safely re-scoped per call.
func MigrateTenantUp(dsn string, schemaName string) error {
	scopedDB, err := openScopedDB(dsn, schemaName)
	if err != nil {
		return fmt.Errorf("connect (scoped to %s): %w", schemaName, err)
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

// openScopedDB opens a dedicated, single-connection *sql.DB whose
// search_path is pinned to schemaName via the connection's "options"
// parameter (equivalent to `psql -c "SET search_path TO schemaName"` at
// connect time, applied to every statement on this connection). Pinned to
// exactly one physical connection (SetMaxOpenConns(1)) so the search_path
// setting can never be silently dropped by handing a later statement to a
// different pooled connection that never had it set. Callers must Close()
// the returned db when done — it's a throwaway, single-purpose handle, not
// meant to be shared or long-lived.
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

// TenantSchemaVersion reports the migration version currently applied to
// schemaName, and whether it's dirty (failed mid-migration). version is 0
// with dirty=false if no migration has ever been applied. schemaName must
// already have been provisioned (its schema_migrations table exists) —
// every caller of this function only reaches it for a tenant that
// Provision (Task 14) already created and migrated.
func TenantSchemaVersion(db *sql.DB, schemaName string) (version uint, dirty bool, err error) {
	if !database.ValidSchemaName(schemaName) {
		return 0, false, fmt.Errorf("invalid schema name %q", schemaName)
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

// LatestTenantVersion returns the highest migration version compiled into
// this binary under tenant/ — the version every tenant schema is expected
// to be at. Used by cli migrate status and by the runtime
// TENANT_MIGRATION_PENDING guard (Task 11).
func LatestTenantVersion() uint {
	return latestVersion(TenantFS, "tenant")
}

// TenantMigrationFiles returns tenant/'s up-migration SQL, in version
// order, for callers that must apply migrations inside their own already-
// open transaction — golang-migrate manages its own connection and can't
// join a caller's transaction, so tenant provisioning (Task 13) reads and
// execs these directly instead of calling MigrateTenantUp.
func TenantMigrationFiles() ([]MigrationFile, error) {
	return readUpFiles(TenantFS, "tenant")
}
