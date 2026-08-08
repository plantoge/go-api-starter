package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/middleware"
)

type fakePermissionChecker struct {
	calls   int32
	allowed bool
	err     error
}

func (f *fakePermissionChecker) HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.allowed, f.err
}

func TestPermissionCache_CachesWithinTTL(t *testing.T) {
	fake := &fakePermissionChecker{allowed: true}
	cache := middleware.NewPermissionCache(fake, time.Minute)
	tenantID, userID := uuid.New(), uuid.New()

	for i := 0; i < 3; i++ {
		if _, err := cache.HasPermission(context.Background(), tenantID, userID, "user.view"); err != nil {
			t.Fatalf("HasPermission: %v", err)
		}
	}
	if fake.calls != 1 {
		t.Errorf("checker called %d times, want 1 (should be cached)", fake.calls)
	}
}

func TestPermissionCache_DifferentPermissionsAreIndependent(t *testing.T) {
	fake := &fakePermissionChecker{allowed: true}
	cache := middleware.NewPermissionCache(fake, time.Minute)
	tenantID, userID := uuid.New(), uuid.New()

	cache.HasPermission(context.Background(), tenantID, userID, "user.view")
	cache.HasPermission(context.Background(), tenantID, userID, "user.create")

	if fake.calls != 2 {
		t.Errorf("checker called %d times, want 2 (different permissions must not share a cache slot)", fake.calls)
	}
}

func TestRequirePermission_AllowsWhenPermitted(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	tenantFake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(tenantFake, time.Minute)
	permCache := middleware.NewPermissionCache(&fakePermissionChecker{allowed: true}, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", middleware.RequirePermission("user.view", permCache), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRequirePermission_RejectsWhenNotPermitted(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	tenantFake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(tenantFake, time.Minute)
	permCache := middleware.NewPermissionCache(&fakePermissionChecker{allowed: false}, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", middleware.RequirePermission("user.delete", permCache), func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePermission allowed a request the checker denied")
	}
}
