package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/apperror"
)

var schemaNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

// ValidSchemaName reports whether name is safe to interpolate into SQL as
// an identifier. Checked once at provisioning time, and again here,
// defensively, every time WithTenant is about to use it.
func ValidSchemaName(name string) bool {
	return schemaNameRe.MatchString(name)
}

// quoteIdentifier double-quotes a Postgres identifier, escaping embedded
// quotes. A small local helper instead of a lib/pq dependency pulled in
// only for QuoteIdentifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type TenantInfo struct {
	TenantID   uuid.UUID
	SchemaName string
}

type Actor struct {
	UserID uuid.UUID
	Scope  string // "platform" | "tenant"
}

type tenantCtxKey struct{}
type actorCtxKey struct{}

func WithTenantInfo(ctx context.Context, info TenantInfo) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, info)
}

func TenantFromContext(ctx context.Context) (TenantInfo, bool) {
	info, ok := ctx.Value(tenantCtxKey{}).(TenantInfo)
	return info, ok
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorCtxKey{}).(Actor)
	return actor, ok
}

type DB struct {
	Pool *sqlx.DB
}

func NewDB(pool *sqlx.DB) *DB {
	return &DB{Pool: pool}
}

// WithTenant runs fn inside a transaction whose search_path is pinned, via
// SET LOCAL, to the schema of the tenant found in ctx. SET LOCAL only lasts
// for the transaction — commit or rollback both clear it automatically —
// so the connection always returns to the pool clean, with no manual reset
// step that could be forgotten on an error path.
func (db *DB) WithTenant(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	info, ok := TenantFromContext(ctx)
	if !ok {
		return apperror.Internal(fmt.Errorf("WithTenant called without tenant context"))
	}
	if !ValidSchemaName(info.SchemaName) {
		return apperror.Internal(fmt.Errorf("invalid schema name %q", info.SchemaName))
	}

	tx, err := db.Pool.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Internal(fmt.Errorf("begin tx: %w", err))
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	// Deliberately pins search_path to the tenant schema alone, excluding
	// "public" — unqualified extension objects conventionally installed
	// there (pgcrypto, uuid-ossp, citext) will NOT resolve inside
	// WithTenant. This is an intentional isolation choice, not an
	// oversight; revisit if/when tenant migrations need those extensions.
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO "+quoteIdentifier(info.SchemaName)); err != nil {
		return apperror.Internal(fmt.Errorf("set search_path: %w", err))
	}

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperror.Internal(fmt.Errorf("commit tx: %w", err))
	}
	return nil
}
