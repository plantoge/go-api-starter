package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

func TestSuccess_Envelope(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Success(c, 200, fiber.Map{"id": "abc"})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	var body struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Success {
		t.Error("success = false, want true")
	}
	if body.Data["id"] != "abc" {
		t.Errorf("data.id = %v, want abc", body.Data["id"])
	}
}

func TestError_AppError_UsesDeclaredStatus(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Error(c, "req-123", apperror.NotFound("tidak ditemukan"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Success {
		t.Error("success = true, want false")
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code = %q, want NOT_FOUND", body.Error.Code)
	}
	if body.RequestID != "req-123" {
		t.Errorf("request_id = %q, want req-123", body.RequestID)
	}
}

func TestError_UnknownError_Becomes500WithoutLeakingDetail(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		return Error(c, "req-456", errors.New("leaked secret detail"))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Message == "leaked secret detail" {
		t.Error("internal error message leaked to client response")
	}
}

func TestError_RouteNotFound_Returns404NotInternalError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return Error(c, "req-404", err)
		},
	})
	app.Get("/x", func(c *fiber.Ctx) error {
		return Success(c, 200, nil)
	})

	// Sengaja nembak path yang nggak punya route terdaftar, biar fiber
	// sendiri yang ngelempar *fiber.Error{Code: 404} sebelum handler ini
	// sempat jalan.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Success {
		t.Error("success = true, want false")
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code = %q, want NOT_FOUND", body.Error.Code)
	}
}

func TestError_Internal_LogsCauseServerSide(t *testing.T) {
	// Tangkap keluaran slog
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, nil)
	logger := slog.New(handler)
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	app := fiber.New()
	dbErr := errors.New("connection refused")
	app.Get("/x", func(c *fiber.Ctx) error {
		return Error(c, "req-789", apperror.Internal(dbErr))
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	// Pastikan status responsnya benar dan penyebab error-nya nggak bocor
	if resp.StatusCode != 500 {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
	var body struct {
		Success   bool   `json:"success"`
		RequestID string `json:"request_id"`
		Error     struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Success {
		t.Error("success = true, want false")
	}
	if body.RequestID != "req-789" {
		t.Errorf("request_id = %q, want req-789", body.RequestID)
	}
	if body.Error.Message == "connection refused" {
		t.Error("cause leaked to client response")
	}

	// Pastikan penyebabnya tercatat di log server bareng request_id-nya
	logOutput := buf.String()
	if logOutput == "" {
		t.Error("expected log output, got empty")
	}
	if !bytes.Contains(buf.Bytes(), []byte("connection refused")) {
		t.Error("cause not found in log output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("req-789")) {
		t.Error("request_id not found in log output")
	}
}
