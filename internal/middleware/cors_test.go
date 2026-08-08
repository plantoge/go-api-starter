package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS([]string{"http://localhost:5173"}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the configured origin", got)
	}
}

func TestCORS_RejectsUnconfiguredOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.CORS([]string{"http://localhost:5173"}))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got == "http://evil.example.com" {
		t.Error("CORS allowed an unconfigured origin")
	}
}
