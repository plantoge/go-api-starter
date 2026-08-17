package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
)

// RegisterHealth nempelin endpoint liveness dan readiness. /health sama
// sekali nggak nyentuh database — cuma buktiin prosesnya hidup dan mau
// menjawab. /health/ready sekalian nge-ping database, jadi load balancer
// atau orchestrator bisa bedain "prosesnya jalan" sama "beneran siap
// melayani request".
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
