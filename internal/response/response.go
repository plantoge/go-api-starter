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

// Error nulis err ke bentuk amplop error standar. Nilai *apperror.Error
// dipakai apa adanya: status, code, dan details-nya ikut yang tertulis di
// situ. Selain itu (bug, atau error driver yang lolos dari service) jadi
// 500 dengan body yang cuma berisi request_id — error aslinya masuk log,
// nggak pernah dikirim ke klien.
//
// Penyebab error internal selalu dicatat di sisi server, baik dia datang
// sebagai error polos maupun sudah dibungkus jadi *apperror.Error.
func Error(c *fiber.Ctx, requestID string, err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		// Catat penyebabnya buat error internal, biar tetap sampai ke log server.
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

	// Fiber ngelempar *fiber.Error buat kegagalan routing (route nggak
	// ketemu, method nggak diizinkan) bahkan sebelum handler jalan, jadi
	// error jenis ini nggak pernah lewat service dan nggak pernah jadi
	// *apperror.Error. Tampilkan pakai status aslinya, jangan dibiarkan
	// jatuh ke cabang 500 di bawah — "not found" biasa nggak pantas dicatat
	// dan dilaporkan sebagai bug server yang tak terduga.
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
