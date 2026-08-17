package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/migration"
)

func bearerToken(c *fiber.Ctx) (string, bool) {
	h := c.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimPrefix(h, prefix), true
}

// RequirePlatform mastiin request bawa access token yang sah dengan
// scope=platform, lalu nyimpen pelakunya di context request. Token
// ber-scope tenant langsung ditolak, walaupun dua-duanya ditandatangani
// pakai secret yang sama — scope itu sekat keras, bukan sekadar petunjuk.
func RequirePlatform(tm *auth.TokenManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr, ok := bearerToken(c)
		if !ok {
			return apperror.Unauthorized("token tidak ditemukan")
		}
		claims, err := tm.Verify(tokenStr)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		if claims.Scope != auth.ScopePlatform {
			return apperror.Forbidden("token ini bukan untuk admin platform")
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}

		c.SetUserContext(database.WithActor(c.UserContext(), database.Actor{
			UserID: userID, Scope: auth.ScopePlatform,
		}))
		return c.Next()
	}
}

// RequireTenant mastiin request bawa access token sah dengan scope=tenant,
// nyari schema/status/versi migrasi tenant lewat resolver, nolak tenant
// yang nggak aktif dan schema yang migrasinya masih ketinggalan dari
// binary ini (TENANT_MIGRATION_PENDING), lalu nyimpen info pelaku dan
// tenant di context request — semua yang nanti dibutuhin
// database.WithTenant.
func RequireTenant(tm *auth.TokenManager, resolver *TenantResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr, ok := bearerToken(c)
		if !ok {
			return apperror.Unauthorized("token tidak ditemukan")
		}
		claims, err := tm.Verify(tokenStr)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		if claims.Scope != auth.ScopeTenant {
			return apperror.Forbidden("token ini bukan untuk staf tenant")
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}
		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			return apperror.Unauthorized("token tidak valid")
		}

		record, err := resolver.Resolve(c.UserContext(), tenantID)
		if err != nil {
			return apperror.Unauthorized("tenant tidak ditemukan")
		}
		if record.Status != "active" {
			return apperror.Forbidden("tenant tidak aktif")
		}
		if record.SchemaDirty || record.SchemaVersion < migration.LatestTenantVersion() {
			return apperror.TenantMigrationPending()
		}

		ctx := c.UserContext()
		ctx = database.WithActor(ctx, database.Actor{UserID: userID, Scope: auth.ScopeTenant})
		ctx = database.WithTenantInfo(ctx, database.TenantInfo{TenantID: tenantID, SchemaName: record.SchemaName})
		c.SetUserContext(ctx)

		SetLoggerInCtx(c, LoggerFromContext(ctx).With("tenant_id", tenantID.String()))
		return c.Next()
	}
}
