package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TenantRecord struct {
	TenantID      uuid.UUID
	SchemaName    string
	Status        string // provisioning | active | suspended
	SchemaVersion uint
	SchemaDirty   bool
}

// TenantLookup is implemented by the platform tenant service (Task 13).
// Declared here, on the consumer side, so this package never imports
// modules/platform/tenant.
type TenantLookup interface {
	FindByID(ctx context.Context, tenantID uuid.UUID) (TenantRecord, error)
}

type cacheEntry struct {
	record  TenantRecord
	expires time.Time
}

// TenantResolver caches tenant lookups for ttl so RequireTenant doesn't hit
// the database on every request. This is the "asumsi single instance" cache
// from the design spec: pencabutan/perubahan status only becomes visible
// on other instances after ttl elapses (or immediately on this instance if
// Invalidate is called) — acceptable as long as the app runs as one
// process; a multi-instance deployment would need this moved to Redis.
type TenantResolver struct {
	lookup TenantLookup
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[uuid.UUID]cacheEntry
}

func NewTenantResolver(lookup TenantLookup, ttl time.Duration) *TenantResolver {
	return &TenantResolver{lookup: lookup, ttl: ttl, cache: make(map[uuid.UUID]cacheEntry)}
}

func (r *TenantResolver) Resolve(ctx context.Context, tenantID uuid.UUID) (TenantRecord, error) {
	r.mu.RLock()
	entry, ok := r.cache[tenantID]
	r.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.record, nil
	}

	rec, err := r.lookup.FindByID(ctx, tenantID)
	if err != nil {
		return TenantRecord{}, err
	}

	r.mu.Lock()
	r.cache[tenantID] = cacheEntry{record: rec, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	return rec, nil
}

// Invalidate drops tenantID from the cache. Call this whenever a tenant's
// status changes (suspend/activate/delete, Task 13) so the change is
// visible on this instance's very next request instead of waiting out ttl.
func (r *TenantResolver) Invalidate(tenantID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}
