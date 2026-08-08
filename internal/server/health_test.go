package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/server"
	"go-api-starter/internal/testsupport"
)

func TestHealth_AlwaysOK(t *testing.T) {
	app := fiber.New()
	pool := testsupport.OpenTestDB(t)
	server.RegisterHealth(app, pool)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHealthReady_OKWhenDBReachable(t *testing.T) {
	app := fiber.New()
	pool := testsupport.OpenTestDB(t)
	server.RegisterHealth(app, pool)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
