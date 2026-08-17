// @title           App API
// @version         1.0
// @description     Starter API multi-tenant — PostgreSQL dengan satu schema per tenant. Point of Sales cuma hal pertama yang dibangun di atasnya, bukan bagian tetap dari starter ini.
// @BasePath        /api/v1
// @securitydefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	platformauth "go-api-starter/internal/modules/platform/auth"
	platformtenant "go-api-starter/internal/modules/platform/tenant"
	tenantauth "go-api-starter/internal/modules/tenant/auth"
	tenantrole "go-api-starter/internal/modules/tenant/role"
	tenantuser "go-api-starter/internal/modules/tenant/user"
	"go-api-starter/internal/ratelimit"
	"go-api-starter/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Sengaja di-print, bukan di-log. Kegagalan ini terjadi sebelum
		// proses punya logger, yang baca cuma manusia di depan terminal,
		// dan slog bakal nge-escape baris baru yang justru bikin daftarnya
		// enak dibaca.
		fmt.Fprintln(os.Stderr, config.Explain(err))
		os.Exit(1)
	}

	pool, err := database.NewPool(cfg.DB)
	if err != nil {
		slog.Error("db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	platformPool, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		slog.Error("platform db pool", "error", err)
		os.Exit(1)
	}
	defer platformPool.Close()

	db := database.NewDB(pool)
	tokens := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL)
	rateLimiter := ratelimit.NewLoginAttemptService(platformPool, cfg.Login.MaxAttempts, cfg.Login.AttemptWindow)

	// tenantRepo sekaligus memenuhi middleware.TenantLookup (FindByID) dan
	// tenantauth.TenantLookup (FindRecordByCode) — satu repository, dua
	// bentuk pencarian buat dua pemanggil yang beda.
	tenantRepo := platformtenant.NewRepository(platformPool, pool.DB)
	tenantResolver := middleware.NewTenantResolver(tenantRepo, time.Minute)

	roleSvc := tenantrole.NewService(db)
	permCache := middleware.NewPermissionCache(roleSvc, time.Minute)

	platformAuthSvc := platformauth.NewService(platformPool, tokens, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, rateLimiter)
	tenantAuthSvc := tenantauth.NewService(db, tenantRepo, tokens, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, rateLimiter)

	userSvc := tenantuser.NewService(tenantuser.NewRepository(db))

	deps := server.Dependencies{
		Tokens:          tokens,
		TenantResolver:  tenantResolver,
		PermissionCache: permCache,
		PlatformAuth:    platformauth.NewHandler(platformAuthSvc),
		TenantAuth:      tenantauth.NewHandler(tenantAuthSvc),
		TenantUser:      tenantuser.NewHandler(userSvc),
	}
	app := server.NewRouter(deps, cfg.CORS.AllowedOrigins)
	server.RegisterHealth(app, pool)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.App.Port)
		slog.Info("listening", "addr", addr)
		if err := app.Listen(addr); err != nil {
			slog.Error("server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
