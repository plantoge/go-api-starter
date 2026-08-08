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
		pool.Exec("DROP SCHEMA " + name + " CASCADE")
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

func TestWithTenant_NoTenantInContext_ReturnsError(t *testing.T) {
	pool := testsupport.OpenTestDB(t)
	db := database.NewDB(pool)

	err := db.WithTenant(context.Background(), func(tx *sqlx.Tx) error {
		t.Fatal("fn should not run without tenant context")
		return nil
	})
	if err == nil {
		t.Fatal("WithTenant() = nil, want error when context has no TenantInfo")
	}
}
