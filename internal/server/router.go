package server

import (
	"github.com/gofiber/fiber/v2"
	swaggerui "github.com/gofiber/swagger"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/middleware"
	platformauth "go-api-starter/internal/modules/platform/auth"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/response"

	_ "go-api-starter/docs/swagger" // hasil `swag init`; daftarin spec yang dibaca swaggerui
)

// Dependencies ngumpulin semua handler dan cache yang dibutuhin NewRouter.
// Dibangun sekali di cmd/api/main.go, ditaruh di satu struct biar tugas
// main.go tetap sebatas "siapkan dependency, serahkan ke NewRouter" —
// nggak campur aduk antara logika router dan perakitan dependency.
type Dependencies struct {
	Tokens          *auth.TokenManager
	TenantResolver  *middleware.TenantResolver
	PermissionCache *middleware.PermissionCache
	PlatformAuth    *platformauth.Handler
	TenantAuth      *tenantauth.Handler
	TenantUser      *tenantuser.Handler
}

// NewRouter ngerakit app Fiber sepenuhnya: middleware global dulu, baru
// semua route di bawah /api/v1. Endpoint health didaftarkan terpisah lewat
// RegisterHealth (main.go manggil dua-duanya).
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

	// Didaftarin setelah rantai middleware (bukan sebelumnya, kayak versi
	// awal) biar penanganan panic dan logging terstruktur juga kena ke
	// route ini, nggak diam-diam kelewat.
	app.Get("/swagger/*", swaggerui.HandlerDefault)

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
