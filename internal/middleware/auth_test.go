package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
)

func TestRequirePlatform_AcceptsPlatformToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	userID := uuid.New()
	token, _ := tm.IssueAccessToken(userID, auth.ScopePlatform, nil)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	var seenActor database.Actor
	app.Get("/x", func(c *fiber.Ctx) error {
		seenActor, _ = database.ActorFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if seenActor.UserID != userID {
		t.Errorf("actor.UserID = %v, want %v", seenActor.UserID, userID)
	}
}

func TestRequirePlatform_RejectsTenantScopeToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePlatform accepted a tenant-scope token")
	}
}

func TestRequirePlatform_RejectsMissingToken(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequirePlatform(tm))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequirePlatform accepted a request with no token")
	}
}

func TestRequireTenant_AcceptsActiveTenant_SetsTenantContext(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	userID := uuid.New()
	token, _ := tm.IssueAccessToken(userID, auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	var seenTenant database.TenantInfo
	app.Get("/x", func(c *fiber.Ctx) error {
		seenTenant, _ = database.TenantFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if seenTenant.SchemaName != "acme_corp" {
		t.Errorf("tenant.SchemaName = %q, want acme_corp", seenTenant.SchemaName)
	}
}

func TestRequireTenant_RejectsSuspendedTenant(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "suspended", SchemaVersion: 4,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequireTenant accepted a suspended tenant")
	}
}

func TestRequireTenant_RejectsBehindMigrationVersion(t *testing.T) {
	tm := auth.NewTokenManager("secret", time.Minute)
	tenantID := uuid.New()
	token, _ := tm.IssueAccessToken(uuid.New(), auth.ScopeTenant, &tenantID)

	fake := &fakeTenantLookup{record: middleware.TenantRecord{
		TenantID: tenantID, SchemaName: "acme_corp", Status: "active", SchemaVersion: 1,
	}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.RequireTenant(tm, resolver))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode == 200 {
		t.Error("RequireTenant accepted a tenant whose schema is behind the binary's compiled-in migrations")
	}
}
