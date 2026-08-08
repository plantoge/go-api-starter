package permission_test

import (
	"context"
	"testing"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/testsupport"
)

func TestSync_SeedsAllPermissions(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	added, err := permission.Sync(context.Background(), pool.DB, schema)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if added != len(permission.All) {
		t.Errorf("added = %d, want %d", added, len(permission.All))
	}

	var count int
	if err := pool.Get(&count, "SELECT count(*) FROM "+schema+".permissions"); err != nil {
		t.Fatalf("count permissions: %v", err)
	}
	if count != len(permission.All) {
		t.Errorf("permissions table has %d rows, want %d", count, len(permission.All))
	}
}

func TestSync_IsIdempotent(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(pool.DB, schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}

	if _, err := permission.Sync(context.Background(), pool.DB, schema); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	added, err := permission.Sync(context.Background(), pool.DB, schema)
	if err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if added != 0 {
		t.Errorf("second Sync added %d permissions, want 0 (already synced)", added)
	}
}
