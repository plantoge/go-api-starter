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
	Status        string // provisioning | active | suspended (nilainya tetap dalam bahasa Inggris karena disimpan apa adanya di database)
	SchemaVersion uint
	SchemaDirty   bool
}

// TenantLookup diimplementasikan oleh service tenant platform (Task 13).
// Dideklarasikan di sini, di sisi pemakai, biar package ini nggak pernah
// perlu meng-import modules/platform/tenant.
type TenantLookup interface {
	FindByID(ctx context.Context, tenantID uuid.UUID) (TenantRecord, error)
}

type cacheEntry struct {
	record  TenantRecord
	expires time.Time
}

// TenantResolver nyimpen hasil pencarian tenant selama ttl, biar
// RequireTenant nggak nembak database tiap request. Ini cache dengan
// "asumsi single instance" seperti di spec desain: pencabutan atau
// perubahan status baru kelihatan di instance lain setelah ttl habis (atau
// langsung di instance ini kalau Invalidate dipanggil). Aman selama
// aplikasi jalan sebagai satu proses; kalau nanti dideploy multi-instance,
// bagian ini mesti dipindah ke Redis.
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

// Invalidate ngebuang tenantID dari cache. Panggil ini tiap kali status
// tenant berubah (suspend/activate/delete, Task 13) biar perubahannya
// kebaca di request berikutnya, nggak perlu nunggu ttl habis.
func (r *TenantResolver) Invalidate(tenantID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}
