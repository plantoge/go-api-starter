package middleware

import (
	"crypto/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

const localsKeyRequestID = "request_id"

// RequestID assigns a ULID to every request (sortable by time, unique
// without coordination) and exposes it via the X-Request-ID response
// header and RequestIDFromCtx — the id every error response and log line
// ties back to.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var idStr string
		if id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader); err == nil {
			idStr = id.String()
		} else {
			idStr = time.Now().Format("20060102150405.000000000")
		}
		c.Locals(localsKeyRequestID, idStr)
		c.Set("X-Request-ID", idStr)
		return c.Next()
	}
}

func RequestIDFromCtx(c *fiber.Ctx) string {
	id, _ := c.Locals(localsKeyRequestID).(string)
	return id
}
