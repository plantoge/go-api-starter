package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/middleware"
)

func TestLogger_LogsRequestSummaryWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(base))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	wantID := resp.Header.Get("X-Request-ID")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode log line: %v (raw: %s)", err, buf.String())
	}
	if line["request_id"] != wantID {
		t.Errorf("logged request_id = %v, want %v", line["request_id"], wantID)
	}
	if line["method"] != "GET" {
		t.Errorf("logged method = %v, want GET", line["method"])
	}
	if _, ok := line["duration_ms"]; !ok {
		t.Error("log line missing duration_ms")
	}
}

func TestLoggerFromContext_AvailableInsideHandler(t *testing.T) {
	base := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(base))
	var got *slog.Logger
	app.Get("/x", func(c *fiber.Ctx) error {
		got = middleware.LoggerFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got == nil {
		t.Fatal("LoggerFromContext returned nil inside the handler")
	}
}
