package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestRequestID_SetsHeaderAndLocal(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	var seen string
	app.Get("/x", func(c *fiber.Ctx) error {
		seen = middleware.RequestIDFromCtx(c)
		return c.SendStatus(200)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	header := resp.Header.Get("X-Request-ID")
	if header == "" {
		t.Error("X-Request-ID header not set")
	}
	if seen != header {
		t.Errorf("RequestIDFromCtx() = %q, want it to match response header %q", seen, header)
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp1, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	resp2, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))

	if resp1.Header.Get("X-Request-ID") == resp2.Header.Get("X-Request-ID") {
		t.Error("two requests got the same request ID")
	}
}
