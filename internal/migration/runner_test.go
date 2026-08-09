package migration_test

import (
	"testing"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/testsupport"
)

func TestMigratePlatformUp_CreatesSchemaAndIsIdempotent(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	pool.Exec("DROP SCHEMA IF EXISTS platform CASCADE")
	t.Cleanup(func() { pool.Exec("DROP SCHEMA IF EXISTS platform CASCADE") })

	db := pool.DB

	if err := migration.MigratePlatformUp(db); err != nil {
		t.Fatalf("first MigratePlatformUp: %v", err)
	}
	if err := migration.MigratePlatformUp(db); err != nil {
		t.Fatalf("second MigratePlatformUp (should be a no-op): %v", err)
	}

	var exists bool
	err := pool.Get(&exists, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'platform' AND table_name = 'tenants'
	)`)
	if err != nil {
		t.Fatalf("check tenants table: %v", err)
	}
	if !exists {
		t.Error("platform.tenants table was not created")
	}
}

func TestLatestPlatformVersion_MatchesFileCount(t *testing.T) {
	if got := migration.LatestPlatformVersion(); got != 5 {
		t.Errorf("LatestPlatformVersion() = %d, want 5 (five platform migration files)", got)
	}
}

func TestMigrateTenantUp_AndVersionReporting(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := pool.DB

	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })

	if err := migration.MigrateTenantUp(testsupport.TestDSN(), schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	version, dirty, err := migration.TenantSchemaVersion(db, schema)
	if err != nil {
		t.Fatalf("TenantSchemaVersion: %v", err)
	}
	if dirty {
		t.Error("schema reported dirty after a clean migration")
	}
	if version != migration.LatestTenantVersion() {
		t.Errorf("version = %d, want latest %d", version, migration.LatestTenantVersion())
	}
}

func TestTenantMigrationFiles_ReturnsAllFilesInOrder(t *testing.T) {
	files, err := migration.TenantMigrationFiles()
	if err != nil {
		t.Fatalf("TenantMigrationFiles: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files, want 4", len(files))
	}
	for i, f := range files {
		if f.Version != uint(i+1) {
			t.Errorf("files[%d].Version = %d, want %d (files must be in ascending order)", i, f.Version, i+1)
		}
		if f.SQL == "" {
			t.Errorf("files[%d].SQL is empty", i)
		}
	}
}
