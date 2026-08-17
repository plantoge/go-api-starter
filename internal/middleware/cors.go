package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS ngerakit middleware CORS bawaan Fiber dari daftar origin yang
// diizinkan di konfigurasi. Nggak ada wildcard: API ini pakai bearer token
// (AllowCredentials: true), dan browser jelas-jelas nolak kombinasi origin
// wildcard dengan credentials.
func CORS(allowedOrigins []string) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Join(allowedOrigins, ","),
		AllowHeaders:     "Content-Type,Authorization",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	})
}
