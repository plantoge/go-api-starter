package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/response"
)

func TestRecover_ConvertsPanicToInternal500(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return response.Error(c, middleware.RequestIDFromCtx(c), err)
		},
	})
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	app.Use(middleware.Recover())
	app.Get("/x", func(c *fiber.Ctx) error {
		panic("boom")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestRecover_PassesThroughNormalRequests(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	app.Use(middleware.Recover())
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(204) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204 (no panic, should pass through untouched)", resp.StatusCode)
	}
}
