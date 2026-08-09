package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

type fakeTenantLookup struct {
	record middleware.TenantRecord
}

func (f *fakeTenantLookup) FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error) {
	return f.record, nil
}

func setupTenantAuthService(t *testing.T) (*tenantauth.Service, string) {
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

	plainPassword := "correct-horse-battery"
	hash, err := appauth.HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	testsupport.SeedInSchema(t, pool, schema,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, 'staff@example.com', $2, 'Staff', true)`,
		uuid.New(), hash)

	db := database.NewDB(pool)
	platformPool := testsupport.OpenTestPlatformDB(t)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, 5, 15*time.Minute)
	lookup := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: uuid.New(), SchemaName: schema, Status: "active"}}

	tokens := appauth.NewTokenManager("test-secret", 15*time.Minute)
	svc := tenantauth.NewService(db, lookup, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	return svc, plainPassword
}

func TestLogin_ValidCredentials_ReturnsTokens(t *testing.T) {
	svc, password := setupTenantAuthService(t)

	res, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("Login() returned an empty token")
	}
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc, _ := setupTenantAuthService(t)

	if _, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: "wrong",
	}); err == nil {
		t.Fatal("Login() with a wrong password succeeded")
	}
}

func TestRefresh_ValidToken_IssuesNewAccessToken(t *testing.T) {
	svc, password := setupTenantAuthService(t)
	login, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	res, err := svc.Refresh(context.Background(), tenantauth.RefreshRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, password := setupTenantAuthService(t)
	login, err := svc.Login(context.Background(), tenantauth.LoginRequest{
		TenantCode: "acme_corp", Email: "staff@example.com", Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(context.Background(), tenantauth.LogoutRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), tenantauth.RefreshRequest{
		TenantCode: "acme_corp", RefreshToken: login.RefreshToken,
	}); err == nil {
		t.Error("Refresh() succeeded with a revoked refresh token")
	}
}
