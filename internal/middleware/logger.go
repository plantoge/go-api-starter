package middleware

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

const localsKeyLogger = "logger"

type loggerCtxKey struct{}

// Logger nempelin *slog.Logger per request (yang sudah ditandai
// request_id) ke locals fiber sekaligus ke context.Context milik request,
// lalu nulis satu baris log terstruktur begitu request selesai. Middleware
// setelahnya (tenant resolver, Task 11) bisa nambahin info ke logger itu —
// misalnya tenant_id — lewat SetLoggerInCtx. Pas middleware ini nulis
// baris ringkasannya, c.Next() sudah balik duluan, jadi yang kepakai
// otomatis logger versi paling lengkap.
func Logger(base *slog.Logger) fiber.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(c *fiber.Ctx) error {
		start := time.Now()
		SetLoggerInCtx(c, base.With("request_id", RequestIDFromCtx(c)))

		err := c.Next()

		LoggerFromCtx(c).Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", statusFor(c, err),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
}

// statusFor ngasih tahu status yang beneran bakal dipakai buat menjawab
// request ini. Di jalur sukses (err == nil), handler sudah nyetel status
// lewat c.Status()/c.SendStatus(), jadi c.Response().StatusCode() memang
// akurat.
//
// Di jalur error ceritanya beda: pada titik ini ErrorHandler global punya
// Fiber belum jalan — dia baru jalan setelah semua middleware app.Use()
// selesai — jadi objek respons masih megang status bawaan. Makanya status
// aslinya diambil dari error-nya langsung, pakai pemetaan yang sama persis
// dengan response.Error: Status yang tertulis di *apperror.Error, atau 500
// buat error jenis lain.
func statusFor(c *fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Status
	}
	return 500
}

// SetLoggerInCtx nyimpen l di locals fiber (buat handler yang megang
// *fiber.Ctx) sekaligus di context request (buat service yang cuma megang
// context.Context). Dasarnya selalu c.UserContext(), nggak pernah
// context.Background(), biar nilai yang sudah ditempel middleware lain
// (misalnya info tenant) nggak kebuang.
func SetLoggerInCtx(c *fiber.Ctx, l *slog.Logger) {
	c.Locals(localsKeyLogger, l)
	c.SetUserContext(context.WithValue(c.UserContext(), loggerCtxKey{}, l))
}

func LoggerFromCtx(c *fiber.Ctx) *slog.Logger {
	if l, ok := c.Locals(localsKeyLogger).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// LoggerFromContext versi context.Context dari LoggerFromCtx, buat service
// yang memang cuma kebagian context.Context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
