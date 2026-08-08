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

// Logger attaches a per-request *slog.Logger (tagged with request_id) to
// both fiber locals and the request's context.Context, then logs one
// structured line per request after it completes. Downstream middleware
// (the tenant resolver, Task 11) enriches the logger further — e.g. adding
// tenant_id — via SetLoggerInCtx; by the time this middleware logs its
// summary line, c.Next() has already returned, so it picks up whatever the
// final, most-enriched logger is.
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

// statusFor reports the status this request will actually be answered
// with. On the success path (err == nil), the handler already set it via
// c.Status()/c.SendStatus(), so c.Response().StatusCode() is accurate. On
// the error path, Fiber's global ErrorHandler hasn't run yet at this point
// in the middleware chain — it runs strictly after every app.Use()
// middleware returns — so the response object still holds its default
// status. Resolve the real status from the error itself instead, using the
// exact same mapping response.Error uses: an *apperror.Error's declared
// Status, or 500 for anything else.
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

// SetLoggerInCtx stores l in both fiber locals (for handlers holding a
// *fiber.Ctx) and the request context (for services holding only a
// context.Context). It builds on c.UserContext(), never
// context.Background(), so it never discards values other middleware
// already attached (e.g. tenant info).
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

// LoggerFromContext is the context.Context-based counterpart of
// LoggerFromCtx, for services that only ever see a context.Context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
