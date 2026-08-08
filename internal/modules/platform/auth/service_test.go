package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/migration"
	platformauth "go-api-starter/internal/modules/platform/auth"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

func setupService(t *testing.T) (*platformauth.Service, string) {
	t.Helper()
	rawDB := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawDB.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() {
		rawDB.Exec("TRUNCATE platform.refresh_tokens, platform.users, platform.login_attempts CASCADE")
	})

	platformDB := testsupport.OpenTestPlatformDB(t)

	plainPassword := "correct-horse-battery"
	hash, err := appauth.HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	_, err = platformDB.Exec(
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, $2, $3, $4, true)`,
		uuid.New(), "admin@example.com", hash, "Admin")
	if err != nil {
		t.Fatalf("insert admin user: %v", err)
	}

	tokens := appauth.NewTokenManager("test-secret", 15*time.Minute)
	rateLimiter := ratelimit.NewLoginAttemptService(platformDB, 5, 15*time.Minute)
	svc := platformauth.NewService(platformDB, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	return svc, plainPassword
}

func TestLogin_ValidCredentials_ReturnsTokens(t *testing.T) {
	svc, password := setupService(t)

	res, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("Login() returned an empty token")
	}
}

func TestLogin_WrongPassword_ReturnsUnauthorized(t *testing.T) {
	svc, _ := setupService(t)

	if _, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: "wrong"}); err == nil {
		t.Fatal("Login() with a wrong password succeeded")
	}
}

func TestRefresh_ValidToken_IssuesNewAccessToken(t *testing.T) {
	svc, password := setupService(t)
	login, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	res, err := svc.Refresh(context.Background(), platformauth.RefreshRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("Refresh() returned an empty access token")
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, password := setupService(t)
	login, err := svc.Login(context.Background(), platformauth.LoginRequest{Email: "admin@example.com", Password: password})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := svc.Logout(context.Background(), platformauth.LogoutRequest{RefreshToken: login.RefreshToken}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), platformauth.RefreshRequest{RefreshToken: login.RefreshToken}); err == nil {
		t.Error("Refresh() succeeded with a revoked refresh token")
	}
}
