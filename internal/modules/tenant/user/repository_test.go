package user_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/testsupport"
)

func setupUserRepo(t *testing.T) (*tenantuser.Repository, context.Context) {
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

	db := database.NewDB(pool)
	ctx := database.WithTenantInfo(context.Background(), database.TenantInfo{TenantID: uuid.New(), SchemaName: schema})
	ctx = database.WithActor(ctx, database.Actor{UserID: uuid.New(), Scope: "tenant"})
	return tenantuser.NewRepository(db), ctx
}

func TestCreateAndFindByID(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	u := tenantuser.User{ID: uuid.New(), Email: "a@example.com", PasswordHash: "hash", Name: "A", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Email != "a@example.com" {
		t.Errorf("Email = %q, want a@example.com", got.Email)
	}
}

func TestCreate_DuplicateEmail_ReturnsConflict(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	u1 := tenantuser.User{ID: uuid.New(), Email: "dup@example.com", PasswordHash: "hash", Name: "A", IsActive: true}
	if err := repo.Create(ctx, u1); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	u2 := tenantuser.User{ID: uuid.New(), Email: "dup@example.com", PasswordHash: "hash", Name: "B", IsActive: true}
	if err := repo.Create(ctx, u2); err == nil {
		t.Fatal("second Create with duplicate email succeeded, want conflict")
	}
}

func TestFindByID_NotFound(t *testing.T) {
	repo, ctx := setupUserRepo(t)

	if _, err := repo.FindByID(ctx, uuid.New()); err == nil {
		t.Fatal("FindByID() for a nonexistent id succeeded, want not-found error")
	}
}

func TestUpdate_ChangesNameAndIsActive(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	u := tenantuser.User{ID: uuid.New(), Email: "u@example.com", PasswordHash: "hash", Name: "Old", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "New"
	inactive := false
	if err := repo.Update(ctx, u.ID, &newName, &inactive); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "New" || got.IsActive != false {
		t.Errorf("got Name=%q IsActive=%v, want Name=New IsActive=false", got.Name, got.IsActive)
	}
}

func TestDelete_SoftDeletesAndHidesFromFindAndList(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	u := tenantuser.User{ID: uuid.New(), Email: "gone@example.com", PasswordHash: "hash", Name: "Gone", IsActive: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.FindByID(ctx, u.ID); err == nil {
		t.Error("FindByID() found a soft-deleted user")
	}

	users, total, err := repo.List(ctx, 20, 0, "created_at DESC")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, lu := range users {
		if lu.ID == u.ID {
			t.Error("List() returned a soft-deleted user")
		}
	}
	if total != 0 {
		t.Errorf("total = %d, want 0 (the only user was soft-deleted)", total)
	}
}

func TestList_RespectsLimitAndOrder(t *testing.T) {
	repo, ctx := setupUserRepo(t)
	for i := 0; i < 3; i++ {
		u := tenantuser.User{
			ID: uuid.New(), Email: uuid.New().String() + "@example.com",
			PasswordHash: "hash", Name: "User", IsActive: true,
		}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	users, total, err := repo.List(ctx, 2, 0, "created_at DESC")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2 (limit)", len(users))
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}
