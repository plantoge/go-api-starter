package middleware

import (
	"crypto/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

const localsKeyRequestID = "request_id"

// RequestID ngasih ULID ke tiap request (bisa diurutkan berdasarkan waktu,
// dan unik tanpa perlu koordinasi), lalu nampilin lewat header respons
// X-Request-ID dan RequestIDFromCtx. Id inilah yang jadi penghubung tiap
// respons error dengan baris lognya.
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
