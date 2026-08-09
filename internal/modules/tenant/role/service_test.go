package role_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
	"go-api-starter/internal/modules/tenant/role"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/testsupport"
)

func setupRoleService(t *testing.T) (*role.Service, database.TenantInfo, *sqlx.DB) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	schema := testsupport.RandomSchemaName()
	if _, err := pool.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { pool.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := migration.MigrateTenantUp(testsupport.TestDSN(), schema); err != nil {
		t.Fatalf("MigrateTenantUp: %v", err)
	}
	if _, err := permission.Sync(context.Background(), pool.DB, schema); err != nil {
		t.Fatalf("permission.Sync: %v", err)
	}

	tenantInfo := database.TenantInfo{TenantID: uuid.New(), SchemaName: schema}
	return role.NewService(database.NewDB(pool)), tenantInfo, pool
}

// seedInSchema runs a fixture-setup query with search_path pinned to
// schema — tests seed rows directly rather than going through
// database.WithTenant themselves.
func seedInSchema(t *testing.T, pool *sqlx.DB, schema, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SET LOCAL search_path TO "` + schema + `"`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(query, args...); err != nil {
		t.Fatalf("seed query: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestHasPermission_OwnerBypass_AlwaysAllowed(t *testing.T) {
	svc, tenantInfo, pool := setupRoleService(t)
	ctx := database.WithTenantInfo(context.Background(), tenantInfo)

	userID, roleID := uuid.New(), uuid.New()
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'owner@x.com', 'hash', 'Owner', true)`, userID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'owner', true)`, roleID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	allowed, err := svc.HasPermission(ctx, userID, "some.permission.never.seeded")
	if err != nil {
		t.Fatalf("HasPermission: %v", err)
	}
	if !allowed {
		t.Error("owner (is_system role) was denied a permission it should bypass-allow")
	}
}

func TestHasPermission_NonOwner_RequiresExplicitGrant(t *testing.T) {
	svc, tenantInfo, pool := setupRoleService(t)
	ctx := database.WithTenantInfo(context.Background(), tenantInfo)

	userID, roleID := uuid.New(), uuid.New()
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'staff@x.com', 'hash', 'Staff', true)`, userID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'staff', false)`, roleID)
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)

	allowed, err := svc.HasPermission(ctx, userID, permission.UserView)
	if err != nil {
		t.Fatalf("HasPermission: %v", err)
	}
	if allowed {
		t.Error("non-owner role with no granted permission was allowed")
	}

	var permID uuid.UUID
	if err := pool.Get(&permID, `SELECT id FROM `+tenantInfo.SchemaName+`.permissions WHERE code = $1`, permission.UserView); err != nil {
		t.Fatalf("find permission id: %v", err)
	}
	seedInSchema(t, pool, tenantInfo.SchemaName,
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permID)

	allowed, err = svc.HasPermission(ctx, userID, permission.UserView)
	if err != nil {
		t.Fatalf("HasPermission (after grant): %v", err)
	}
	if !allowed {
		t.Error("non-owner role was denied a permission explicitly granted to it")
	}
}
