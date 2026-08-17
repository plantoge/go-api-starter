package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"go-api-starter/internal/apperror"
)

// Recover ngubah panic yang muncul di mana pun setelahnya jadi error
// apperror.Internal biasa, jadi prosesnya nggak ikut mati, sambil nyimpen
// stack trace-nya di log server. Klien cuma dapat request_id —
// response.Error nggak pernah ngasih bocoran isi panic-nya.
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
