package middleware_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go-api-starter/internal/middleware"
)

type fakeTenantLookup struct {
	calls  int32
	record middleware.TenantRecord
	err    error
}

func (f *fakeTenantLookup) FindByID(ctx context.Context, tenantID uuid.UUID) (middleware.TenantRecord, error) {
	atomic.AddInt32(&f.calls, 1)
	return f.record, f.err
}

func TestTenantResolver_CachesWithinTTL(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := resolver.Resolve(context.Background(), tenantID); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
	if fake.calls != 1 {
		t.Errorf("underlying lookup called %d times, want 1 (should be served from cache)", fake.calls)
	}
}

func TestTenantResolver_RefetchesAfterTTL(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, 10*time.Millisecond)

	resolver.Resolve(context.Background(), tenantID)
	time.Sleep(20 * time.Millisecond)
	resolver.Resolve(context.Background(), tenantID)

	if fake.calls != 2 {
		t.Errorf("underlying lookup called %d times, want 2 (TTL should have expired)", fake.calls)
	}
}

func TestTenantResolver_InvalidateForcesRefetch(t *testing.T) {
	tenantID := uuid.New()
	fake := &fakeTenantLookup{record: middleware.TenantRecord{TenantID: tenantID, SchemaName: "acme_corp", Status: "active"}}
	resolver := middleware.NewTenantResolver(fake, time.Minute)

	resolver.Resolve(context.Background(), tenantID)
	resolver.Invalidate(tenantID)
	resolver.Resolve(context.Background(), tenantID)

	if fake.calls != 2 {
		t.Errorf("underlying lookup called %d times, want 2 (Invalidate should force a refetch)", fake.calls)
	}
}
