package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// PermissionChecker diimplementasikan oleh service role tenant
// (modules/tenant/role, task ini). Dideklarasikan di sini, di sisi
// pemakai, biar package ini nggak perlu meng-import modules/tenant/role.
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error)
}

type permCacheKey struct {
	tenantID uuid.UUID
	userID   uuid.UUID
	perm     string
}

type permCacheEntry struct {
	allowed bool
	expires time.Time
}

// PermissionCache ngebungkus PermissionChecker pakai cache ber-TTL pendek
// dengan kunci (tenant, user, permission). Catatan "asumsi single
// instance"-nya sama persis kayak TenantResolver (Task 11): permission
// yang dicabut baru dijamin berlaku di instance ini setelah ttl habis.
type PermissionCache struct {
	checker PermissionChecker
	ttl     time.Duration
	mu      sync.RWMutex
	cache   map[permCacheKey]permCacheEntry
}

func NewPermissionCache(checker PermissionChecker, ttl time.Duration) *PermissionCache {
	return &PermissionCache{checker: checker, ttl: ttl, cache: make(map[permCacheKey]permCacheEntry)}
}

func (c *PermissionCache) HasPermission(ctx context.Context, tenantID, userID uuid.UUID, permCode string) (bool, error) {
	key := permCacheKey{tenantID: tenantID, userID: userID, perm: permCode}

	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.allowed, nil
	}

	allowed, err := c.checker.HasPermission(ctx, userID, permCode)
	if err != nil {
		return false, err
	}

	c.mu.Lock()
	c.cache[key] = permCacheEntry{allowed: allowed, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return allowed, nil
}

// RequirePermission nolak request kecuali user tenant yang sudah login
// memang punya permCode. Wajib jalan setelah RequireTenant, soalnya dia
// baca info tenant dan pelaku yang cuma diisi sama RequireTenant di
// context.
func RequirePermission(permCode string, cache *PermissionCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		actor, ok := database.ActorFromContext(c.UserContext())
		if !ok {
			return apperror.Unauthorized("tidak terautentikasi")
		}
		tenant, ok := database.TenantFromContext(c.UserContext())
		if !ok {
			return apperror.Internal(fmt.Errorf("RequirePermission dipakai tanpa RequireTenant"))
		}

		allowed, err := cache.HasPermission(c.UserContext(), tenant.TenantID, actor.UserID, permCode)
		if err != nil {
			return apperror.Internal(err)
		}
		if !allowed {
			return apperror.Forbidden("tidak punya izin untuk aksi ini")
		}
		return c.Next()
	}
}
