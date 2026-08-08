package tenant_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	"go-api-starter/internal/testsupport"
)

// setupTenantService gives every test in this file a fresh Service wired
// to real platform.tenants + a raw *sql.DB for provisioning transactions.
func setupTenantService(t *testing.T) (*platformtenant.Service, *platformtenant.Repository) {
	t.Helper()
	rawPool := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawPool.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() { rawPool.Exec("TRUNCATE platform.tenants CASCADE") })

	platformDB := testsupport.OpenTestPlatformDB(t)
	repo := platformtenant.NewRepository(platformDB, rawPool.DB)
	resolver := middleware.NewTenantResolver(repo, time.Minute)
	svc := platformtenant.NewService(repo, rawPool.DB, resolver)
	return svc, repo
}

func TestProvision_CreatesActiveTenantWithOwner(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "acme_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })

	result, err := svc.Provision(ctx, platformtenant.ProvisionInput{
		Code: code, Name: "Acme Test", OwnerEmail: "owner@example.com",
	})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if result.Tenant.Status != "active" {
		t.Errorf("Tenant.Status = %q, want active", result.Tenant.Status)
	}
	if result.OwnerPassword == "" {
		t.Error("OwnerPassword is empty")
	}
}

func TestProvision_DuplicateCode_ReturnsConflict(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "dup_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })

	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "First", OwnerEmail: "a@example.com"}); err != nil {
		t.Fatalf("first Provision: %v", err)
	}
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "Second", OwnerEmail: "b@example.com"}); err == nil {
		t.Fatal("second Provision with the same code succeeded, want conflict")
	}
}

func TestSuspendAndActivate_ChangeStatus(t *testing.T) {
	svc, repo := setupTenantService(t)
	ctx := context.Background()
	code := "susp_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if err := svc.Suspend(ctx, code); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	t2, err := repo.FindByCode(ctx, code)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if t2.Status != "suspended" {
		t.Errorf("Status = %q, want suspended", t2.Status)
	}

	if err := svc.Activate(ctx, code); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t3, err := repo.FindByCode(ctx, code)
	if err != nil {
		t.Fatalf("FindByCode: %v", err)
	}
	if t3.Status != "active" {
		t.Errorf("Status = %q, want active", t3.Status)
	}
}

func TestPurge_RefusesBeforeThirtyDays(t *testing.T) {
	svc, _ := setupTenantService(t)
	ctx := context.Background()
	code := "purge1_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	if _, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if err := svc.Delete(ctx, code); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if err := svc.Purge(ctx, code); err == nil {
		t.Fatal("Purge() succeeded immediately after soft delete, want refusal (< 30 days)")
	}
}

func TestFindByID_ReportsSchemaVersion(t *testing.T) {
	svc, repo := setupTenantService(t)
	ctx := context.Background()
	code := "ver_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropSchemaIfExists(t, code) })
	result, err := svc.Provision(ctx, platformtenant.ProvisionInput{Code: code, Name: "X", OwnerEmail: "x@example.com"})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	tenantID, err := uuid.Parse(result.Tenant.ID)
	if err != nil {
		t.Fatalf("parse tenant id: %v", err)
	}
	record, err := repo.FindByID(ctx, tenantID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if record.SchemaVersion != migration.LatestTenantVersion() {
		t.Errorf("SchemaVersion = %d, want %d (freshly provisioned tenant should be at the latest version)",
			record.SchemaVersion, migration.LatestTenantVersion())
	}
	if record.SchemaDirty {
		t.Error("SchemaDirty = true for a freshly provisioned tenant")
	}
}

func dropSchemaIfExists(t *testing.T, schemaName string) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	pool.Exec(`DROP SCHEMA IF EXISTS "` + schemaName + `" CASCADE`)
}
