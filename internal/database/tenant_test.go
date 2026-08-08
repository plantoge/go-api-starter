package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/database"
	"go-api-starter/internal/testsupport"
)

func createTestSchema(t *testing.T, pool *sqlx.DB, name string) {
	t.Helper()
	if _, err := pool.Exec("CREATE SCHEMA " + name); err != nil {
		t.Fatalf("create schema %s: %v", name, err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec("DROP SCHEMA " + name + " CASCADE"); err != nil {
			t.Logf("cleanup: drop schema %s: %v", name, err)
		}
	})
	if _, err := pool.Exec("CREATE TABLE " + name + ".widgets (id serial primary key, name text)"); err != nil {
		t.Fatalf("create table in %s: %v", name, err)
	}
}

func TestWithTenant_IsolatesSchemas(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := database.NewDB(pool)

	schemaA := testsupport.RandomSchemaName()
	schemaB := testsupport.RandomSchemaName()
	createTestSchema(t, pool, schemaA)
	createTestSchema(t, pool, schemaB)

	ctxA := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaA,
	})
	ctxB := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaB,
	})

	err := db.WithTenant(ctxA, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("INSERT INTO widgets (name) VALUES ($1)", "from-a")
		return err
	})
	if err != nil {
		t.Fatalf("insert into schema A: %v", err)
	}

	err = db.WithTenant(ctxB, func(tx *sqlx.Tx) error {
		var count int
		if err := tx.Get(&count, "SELECT count(*) FROM widgets"); err != nil {
			return err
		}
		if count != 0 {
			t.Errorf("schema B sees %d row(s), want 0 — schema A's insert leaked across search_path", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query schema B: %v", err)
	}
}

func TestWithTenant_RollbackClearsSearchPath(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	pool.SetMaxOpenConns(1) // force both calls below onto the same physical connection
	db := database.NewDB(pool)

	schemaA := testsupport.RandomSchemaName()
	schemaB := testsupport.RandomSchemaName()
	createTestSchema(t, pool, schemaA)
	createTestSchema(t, pool, schemaB)

	ctxA := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaA,
	})
	_ = db.WithTenant(ctxA, func(tx *sqlx.Tx) error {
		return errors.New("forced failure")
	})

	ctxB := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaB,
	})
	var current string
	err := db.WithTenant(ctxB, func(tx *sqlx.Tx) error {
		return tx.Get(&current, "SELECT current_schema()")
	})
	if err != nil {
		t.Fatalf("WithTenant(schemaB): %v", err)
	}
	if current != schemaB {
		t.Errorf("current_schema() = %q, want %q — search_path leaked across a rolled-back transaction on a reused connection", current, schemaB)
	}
}

// TestWithTenant_CommitClearsSearchPath guards specifically against a
// regression to plain SET (instead of SET LOCAL): plain SET would also
// pass TestWithTenant_RollbackClearsSearchPath (SET is transactional on
// rollback too), but a *committed* transaction using plain SET leaves
// search_path pinned on the physical connection for whichever query
// reuses it next. SET LOCAL is discarded at commit as well as rollback,
// so after a successful WithTenant call the pool's own search_path must
// be back at the connection default, not the tenant schema.
func TestWithTenant_CommitClearsSearchPath(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	pool.SetMaxOpenConns(1) // force the check below onto the same physical connection
	db := database.NewDB(pool)

	schemaA := testsupport.RandomSchemaName()
	createTestSchema(t, pool, schemaA)

	ctxA := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID: uuid.New(), SchemaName: schemaA,
	})
	err := db.WithTenant(ctxA, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("INSERT INTO widgets (name) VALUES ($1)", "from-a")
		return err
	})
	if err != nil {
		t.Fatalf("WithTenant(schemaA) commit path: %v", err)
	}

	var current string
	if err := pool.Get(&current, "SHOW search_path"); err != nil {
		t.Fatalf("SHOW search_path: %v", err)
	}
	if current == schemaA {
		t.Errorf("search_path = %q after commit, want connection default — SET LOCAL search_path leaked past commit onto the pooled connection", current)
	}
}

func TestWithTenant_NoTenantInContext_ReturnsError(t *testing.T) {
	// No live DB needed: WithTenant returns before ever touching db.Pool
	// when the context carries no TenantInfo, so a nil pool is safe here
	// and lets this test run (and pass) without a database.
	db := database.NewDB(nil)

	err := db.WithTenant(context.Background(), func(tx *sqlx.Tx) error {
		t.Fatal("fn should not run without tenant context")
		return nil
	})
	if err == nil {
		t.Fatal("WithTenant() = nil, want error when context has no TenantInfo")
	}
}

func TestWithTenant_InvalidSchemaName_ReturnsError(t *testing.T) {
	// Same reasoning as above: the invalid-schema-name check happens
	// before db.Pool is ever touched, so a nil pool is safe and this
	// test runs without a database.
	db := database.NewDB(nil)

	ctx := database.WithTenantInfo(context.Background(), database.TenantInfo{
		TenantID:   uuid.New(),
		SchemaName: `bad"; DROP SCHEMA public CASCADE; --`,
	})

	err := db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		t.Fatal("fn should not run with an invalid schema name")
		return nil
	})
	if err == nil {
		t.Fatal("WithTenant() = nil, want error when TenantInfo.SchemaName is invalid")
	}
}
