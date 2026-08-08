package server

import (
	"github.com/gofiber/fiber/v2"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/middleware"
	platformauth "go-api-starter/internal/modules/platform/auth"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/response"
)

// Dependencies collects every constructed handler and cache NewRouter
// needs. Built once in cmd/api/main.go, kept here as a single struct so
// main.go's job stays "construct dependencies, hand them to NewRouter"
// rather than router logic and dependency construction tangled together.
type Dependencies struct {
	Tokens          *auth.TokenManager
	TenantResolver  *middleware.TenantResolver
	PermissionCache *middleware.PermissionCache
	PlatformAuth    *platformauth.Handler
	TenantAuth      *tenantauth.Handler
	TenantUser      *tenantuser.Handler
}

// NewRouter builds the full Fiber app: global middleware, then every route
// under /api/v1. Health endpoints are registered separately via
// RegisterHealth (main.go calls both).
func NewRouter(deps Dependencies, corsOrigins []string) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return response.Error(c, middleware.RequestIDFromCtx(c), err)
		},
	})

	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(nil))
	app.Use(middleware.Recover())
	app.Use(middleware.CORS(corsOrigins))

	api := app.Group("/api/v1")

	admin := api.Group("/admin")
	admin.Post("/auth/login", deps.PlatformAuth.Login)
	admin.Post("/auth/refresh", deps.PlatformAuth.Refresh)
	admin.Post("/auth/logout", deps.PlatformAuth.Logout)

	api.Post("/auth/login", deps.TenantAuth.Login)
	api.Post("/auth/refresh", deps.TenantAuth.Refresh)
	api.Post("/auth/logout", deps.TenantAuth.Logout)

	tenantAPI := api.Group("", middleware.RequireTenant(deps.Tokens, deps.TenantResolver))

	users := tenantAPI.Group("/users")
	users.Post("/", middleware.RequirePermission(permission.UserCreate, deps.PermissionCache), deps.TenantUser.Create)
	users.Get("/", middleware.RequirePermission(permission.UserView, deps.PermissionCache), deps.TenantUser.List)
	users.Get("/:id", middleware.RequirePermission(permission.UserView, deps.PermissionCache), deps.TenantUser.Get)
	users.Patch("/:id", middleware.RequirePermission(permission.UserUpdate, deps.PermissionCache), deps.TenantUser.Update)
	users.Delete("/:id", middleware.RequirePermission(permission.UserDelete, deps.PermissionCache), deps.TenantUser.Delete)

	return app
}
