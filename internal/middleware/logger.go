package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
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
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
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
