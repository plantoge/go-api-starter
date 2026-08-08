package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)

// RegisterHealth attaches liveness and readiness endpoints. /health never
// touches the database — it only proves the process is alive and
// responding. /health/ready additionally pings the database, so a load
// balancer or orchestrator can tell "running" apart from "actually able to
// serve requests."
func RegisterHealth(app *fiber.App, pool *sqlx.DB) {
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		if err := pool.PingContext(c.Context()); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not ready"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
}
