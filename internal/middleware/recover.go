package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

// Recover converts a panic anywhere downstream into a normal
// apperror.Internal error instead of crashing the process, logging the
// stack trace server-side. The client only ever sees request_id —
// response.Error never exposes panic detail.
func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				LoggerFromCtx(c).Error("panic recovered",
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()),
				)
				err = apperror.Internal(fmt.Errorf("panic: %v", r))
			}
		}()
		return c.Next()
	}
}
