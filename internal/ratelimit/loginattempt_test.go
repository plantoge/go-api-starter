package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"go-api-starter/internal/migration"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/testsupport"
)

func setupLoginAttempts(t *testing.T) *ratelimit.LoginAttemptService {
	t.Helper()
	rawDB := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(rawDB.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() { rawDB.Exec("TRUNCATE platform.login_attempts") })

	platformDB := testsupport.OpenTestPlatformDB(t)
	return ratelimit.NewLoginAttemptService(platformDB, 5, 15*time.Minute)
}

func TestCheck_AllowsUnderLimit(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := svc.Record(ctx, "tenant", nil, "user@example.com", false); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err != nil {
		t.Errorf("Check() = %v, want nil at 4 failed attempts (limit is 5)", err)
	}
}

func TestCheck_BlocksAtLimit(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := svc.Record(ctx, "tenant", nil, "user@example.com", false); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err == nil {
		t.Fatal("Check() = nil, want RATE_LIMITED at 5 failed attempts")
	}
}

func TestCheck_DifferentEmailsAreIndependent(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Record(ctx, "tenant", nil, "a@example.com", false)
	}

	if err := svc.Check(ctx, "tenant", nil, "b@example.com"); err != nil {
		t.Errorf("Check() for a different email = %v, want nil", err)
	}
}

func TestCheck_SuccessfulAttemptsDoNotCount(t *testing.T) {
	svc := setupLoginAttempts(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Record(ctx, "tenant", nil, "user@example.com", true)
	}

	if err := svc.Check(ctx, "tenant", nil, "user@example.com"); err != nil {
		t.Errorf("Check() = %v, want nil — only failed attempts should count", err)
	}
}
