package response

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func Success(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

func SuccessList(c *fiber.Ctx, status int, data any, meta Meta) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"meta":    meta,
	})
}

// Error renders err as the standard error envelope. *apperror.Error values
// use their declared status/code/details verbatim. Anything else (a bug, a
// driver error that leaked past a service) becomes a 500 whose body carries
// only request_id — the real error is logged, never sent to the client.
// Internal errors have their causes logged server-side regardless of whether
// they arrive as bare errors or as pre-wrapped *apperror.Error.
func Error(c *fiber.Ctx, requestID string, err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		// Log the cause for internal errors so they reach server-side logs.
		if appErr.Code == apperror.CodeInternal {
			slog.Error("internal error", "request_id", requestID, "error", appErr.Unwrap())
		}
		body := fiber.Map{
			"code":    appErr.Code,
			"message": appErr.Message,
		}
		if appErr.Details != nil {
			body["details"] = appErr.Details
		} else {
			body["details"] = fiber.Map{}
		}
		return c.Status(appErr.Status).JSON(fiber.Map{
			"success":    false,
			"error":      body,
			"request_id": requestID,
		})
	}

	// Fiber raises *fiber.Error for routing failures (no matching route,
	// method not allowed) before a handler ever runs, so these never pass
	// through a service and never become *apperror.Error. Render them with
	// their real status instead of falling through to the 500 branch below
	// — a plain "not found" shouldn't be logged and reported as an
	// unexpected server bug.
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) && fiberErr.Code == fiber.StatusNotFound {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    apperror.CodeNotFound,
				"message": "rute tidak ditemukan",
				"details": fiber.Map{},
			},
			"request_id": requestID,
		})
	}

	slog.Error("unhandled error", "request_id", requestID, "error", err)
	return c.Status(500).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    apperror.CodeInternal,
			"message": "terjadi kesalahan pada server",
			"details": fiber.Map{},
		},
		"request_id": requestID,
	})
}
