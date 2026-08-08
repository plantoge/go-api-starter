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
