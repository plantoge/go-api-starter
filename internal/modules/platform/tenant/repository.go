package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
)

type Repository struct {
	db    *sqlx.DB // platform-pinned pool, for normal CRUD
	rawDB *sql.DB  // unqualified connection, for schema-scoped migration version checks
}

func NewRepository(platformDB *sqlx.DB, rawDB *sql.DB) *Repository {
	return &Repository{db: platformDB, rawDB: rawDB}
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Tenant, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE code = $1 AND deleted_at IS NULL`, code)
	if err == sql.ErrNoRows {
		return Tenant{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return Tenant{}, apperror.Internal(err)
	}
	return t, nil
}

func (r *Repository) FindByCodeIncludingDeleted(ctx context.Context, code string) (Tenant, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t,
		`SELECT * FROM tenants WHERE code = $1 ORDER BY created_at DESC LIMIT 1`, code)
	if err == sql.ErrNoRows {
		return Tenant{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return Tenant{}, apperror.Internal(err)
	}
	return t, nil
}

// FindByID implements middleware.TenantLookup: it reports both the
// tenant's platform-side status and its schema's migration version in one
// call, because RequireTenant needs both to decide whether a request may
// proceed, and both are cached together under the same TTL.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (middleware.TenantRecord, error) {
	var t Tenant
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tenants WHERE id = $1 AND deleted_at IS NULL`, id)
	if err == sql.ErrNoRows {
		return middleware.TenantRecord{}, apperror.NotFound("tenant tidak ditemukan")
	}
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}

	if !database.ValidSchemaName(t.SchemaName) {
		return middleware.TenantRecord{}, apperror.Internal(fmt.Errorf("invalid schema name %q", t.SchemaName))
	}
	version, dirty, err := migration.TenantSchemaVersion(r.rawDB, t.SchemaName)
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}

	return middleware.TenantRecord{
		TenantID: t.ID, SchemaName: t.SchemaName, Status: t.Status,
		SchemaVersion: version, SchemaDirty: dirty,
	}, nil
}

// FindRecordByCode is FindByCode plus a migration-version check, shaped as
// middleware.TenantRecord — the tenant/auth module's login flow (Task 16)
// uses this to resolve a tenant_code to enough info to attempt a login.
func (r *Repository) FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error) {
	t, err := r.FindByCode(ctx, code)
	if err != nil {
		return middleware.TenantRecord{}, err
	}
	version, dirty, err := migration.TenantSchemaVersion(r.rawDB, t.SchemaName)
	if err != nil {
		return middleware.TenantRecord{}, apperror.Internal(err)
	}
	return middleware.TenantRecord{
		TenantID: t.ID, SchemaName: t.SchemaName, Status: t.Status,
		SchemaVersion: version, SchemaDirty: dirty,
	}, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int, orderBy string) ([]Tenant, int, error) {
	var tenants []Tenant
	query := `SELECT * FROM tenants WHERE deleted_at IS NULL ORDER BY ` + orderBy + ` LIMIT $1 OFFSET $2`
	if err := r.db.SelectContext(ctx, &tenants, query, limit, offset); err != nil {
		return nil, 0, apperror.Internal(err)
	}
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT count(*) FROM tenants WHERE deleted_at IS NULL`); err != nil {
		return nil, 0, apperror.Internal(err)
	}
	return tenants, total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, suspendedAt *time.Time) error {
	actor, _ := database.ActorFromContext(ctx)
	_, err := r.db.ExecContext(ctx,
		`UPDATE tenants SET status = $2, suspended_at = $3, updated_by = $4 WHERE id = $1`,
		id, status, suspendedAt, actor.UserID)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	actor, _ := database.ActorFromContext(ctx)
	_, err := r.db.ExecContext(ctx,
		`UPDATE tenants SET deleted_at = now(), deleted_by = $2 WHERE id = $1`,
		id, actor.UserID)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
