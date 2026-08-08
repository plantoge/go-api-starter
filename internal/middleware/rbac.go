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

// PermissionChecker is implemented by the tenant role service
// (modules/tenant/role, this task). Declared here, consumer-side, so this
// package never imports modules/tenant/role.
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

// PermissionCache wraps a PermissionChecker with a short TTL cache keyed
// by (tenant, user, permission) — same "asumsi single instance" caveat as
// TenantResolver (Task 11): a revoked permission is only guaranteed to
// take effect on this instance after ttl elapses.
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

// RequirePermission rejects the request unless the authenticated tenant
// user holds permCode. It must run after RequireTenant — it reads the
// tenant and actor info that only RequireTenant sets in the context.
func RequirePermission(permCode string, cache *PermissionCache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		actor, ok := database.ActorFromContext(c.UserContext())
		if !ok {
			return apperror.Unauthorized("tidak terautentikasi")
		}
		tenant, ok := database.TenantFromContext(c.UserContext())
		if !ok {
			return apperror.Internal(fmt.Errorf("RequirePermission used without RequireTenant"))
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
