package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	platformauth "go-api-starter/internal/modules/platform/auth"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantrole "go-api-starter/internal/modules/tenant/role"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/server"
	"go-api-starter/internal/testsupport"
)

func buildTestApp(t *testing.T) (*fiber.App, *platformtenant.Service) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	if err := migration.MigratePlatformUp(pool.DB); err != nil {
		t.Fatalf("MigratePlatformUp: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec("TRUNCATE platform.tenants, platform.login_attempts CASCADE")
	})

	platformPool := testsupport.OpenTestPlatformDB(t)
	db := database.NewDB(pool)
	tokens := auth.NewTokenManager("integration-test-secret", 15*time.Minute)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, 5, 15*time.Minute)

	tenantRepo := platformtenant.NewRepository(platformPool, pool.DB)
	tenantResolver := middleware.NewTenantResolver(tenantRepo, time.Minute)
	tenantSvc := platformtenant.NewService(tenantRepo, pool.DB, tenantResolver)

	roleSvc := tenantrole.NewService(db)
	permCache := middleware.NewPermissionCache(roleSvc, time.Minute)

	platformAuthSvc := platformauth.NewService(platformPool, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	tenantAuthSvc := tenantauth.NewService(db, tenantRepo, tokens, 15*time.Minute, 168*time.Hour, rateLimiter)
	userSvc := tenantuser.NewService(tenantuser.NewRepository(db))

	deps := server.Dependencies{
		Tokens:          tokens,
		TenantResolver:  tenantResolver,
		PermissionCache: permCache,
		PlatformAuth:    platformauth.NewHandler(platformAuthSvc),
		TenantAuth:      tenantauth.NewHandler(tenantAuthSvc),
		TenantUser:      tenantuser.NewHandler(userSvc),
	}
	app := server.NewRouter(deps, []string{"http://localhost:5173"})
	return app, tenantSvc
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	reader := bytes.NewReader(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1) // -1: tanpa timeout — langkah-langkah ini nembak Postgres sungguhan
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, path, err)
	}
	var parsed map[string]any
	json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

func loginAsOwner(t *testing.T, app *fiber.App, tenantCode, email, password string) string {
	t.Helper()
	resp, body := doJSON(t, app, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"tenant_code": tenantCode, "email": email, "password": password,
	}, "")
	if resp.StatusCode != 200 {
		t.Fatalf("login for %s failed: status=%d body=%v", tenantCode, resp.StatusCode, body)
	}
	data := body["data"].(map[string]any)
	return data["access_token"].(string)
}

func TestTenantIsolation_AcrossTwoTenantsWithSimilarData(t *testing.T) {
	app, tenantSvc := buildTestApp(t)
	ctx := context.Background()

	codeA := "acme_" + testsupport.RandomSchemaName()
	codeB := "globex_" + testsupport.RandomSchemaName()
	t.Cleanup(func() { dropTestSchema(t, codeA) })
	t.Cleanup(func() { dropTestSchema(t, codeB) })

	resA, err := tenantSvc.Provision(ctx, platformtenant.ProvisionInput{
		Code: codeA, Name: "Acme Corp", OwnerEmail: "owner@acme.test",
	})
	if err != nil {
		t.Fatalf("Provision tenant A: %v", err)
	}
	resB, err := tenantSvc.Provision(ctx, platformtenant.ProvisionInput{
		Code: codeB, Name: "Globex Corp", OwnerEmail: "owner@globex.test",
	})
	if err != nil {
		t.Fatalf("Provision tenant B: %v", err)
	}

	tokenA := loginAsOwner(t, app, codeA, "owner@acme.test", resA.OwnerPassword)
	tokenB := loginAsOwner(t, app, codeB, "owner@globex.test", resB.OwnerPassword)

	// Sengaja pakai email yang sama — kalau isolasinya sampai jebol,
	// tabrakan unique index atau data yang saling kelihatan di sinilah
	// gejalanya bakal muncul.
	respA, bodyA := doJSON(t, app, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "staff@example.com", "password": "correct-horse-battery", "name": "Staff A",
	}, tokenA)
	if respA.StatusCode != 201 {
		t.Fatalf("create user in tenant A failed: status=%d body=%v", respA.StatusCode, bodyA)
	}
	staffAID := bodyA["data"].(map[string]any)["id"].(string)

	respB, bodyB := doJSON(t, app, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "staff@example.com", "password": "correct-horse-battery", "name": "Staff B",
	}, tokenB)
	if respB.StatusCode != 201 {
		t.Fatalf("create user in tenant B (same email, different tenant) failed: status=%d body=%v", respB.StatusCode, bodyB)
	}

	_, listA := doJSON(t, app, http.MethodGet, "/api/v1/users?limit=100", nil, tokenA)
	assertNamesPresentAbsent(t, listA, []string{"Staff A"}, []string{"Staff B"})

	_, listB := doJSON(t, app, http.MethodGet, "/api/v1/users?limit=100", nil, tokenB)
	assertNamesPresentAbsent(t, listB, []string{"Staff B"}, []string{"Staff A"})

	// Kontrol positif: tenant A ngambil user miliknya sendiri lewat ID
	// harus berhasil. Tanpa ini, pengecekan negatif di bawah jadi
	// ambigu antara "isolasinya jalan" dan "route/handler-nya rusak dan
	// selalu balikin 404".
	respOwn, bodyOwn := doJSON(t, app, http.MethodGet, "/api/v1/users/"+staffAID, nil, tokenA)
	if respOwn.StatusCode != 200 {
		t.Fatalf("tenant A fetching its own user by ID failed: status=%d body=%v", respOwn.StatusCode, bodyOwn)
	}

	// Pengecekan paling keras: tenant B yang megang ID user milik tenant A
	// tetap nggak bisa ngambil datanya — WithTenant ngunci tiap query ke
	// schema milik si pemanggil, apa pun ID yang diminta.
	respCross, _ := doJSON(t, app, http.MethodGet, "/api/v1/users/"+staffAID, nil, tokenB)
	if respCross.StatusCode != 404 {
		t.Errorf("tenant B fetched tenant A's user by ID: status=%d, want 404", respCross.StatusCode)
	}
}

func assertNamesPresentAbsent(t *testing.T, body map[string]any, present, absent []string) {
	t.Helper()
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("response has no data array: %v", body)
	}
	names := map[string]bool{}
	for _, item := range data {
		row := item.(map[string]any)
		names[row["name"].(string)] = true
	}
	for _, want := range present {
		if !names[want] {
			t.Errorf("expected %q in list, got names: %v", want, names)
		}
	}
	for _, unwanted := range absent {
		if names[unwanted] {
			t.Errorf("found %q in list — cross-tenant leak, got names: %v", unwanted, names)
		}
	}
}

func dropTestSchema(t *testing.T, schemaName string) {
	t.Helper()
	pool := testsupport.OpenTestDB(t)
	pool.Exec(`DROP SCHEMA IF EXISTS "` + schemaName + `" CASCADE`)
}
