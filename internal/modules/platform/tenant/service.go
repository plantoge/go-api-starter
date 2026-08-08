package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/migration"
	"go-api-starter/internal/pagination"
	"go-api-starter/internal/permission"
	"go-api-starter/internal/response"
)

type Service struct {
	repo     *Repository
	rawDB    *sql.DB
	resolver *middleware.TenantResolver
}

func NewService(repo *Repository, rawDB *sql.DB, resolver *middleware.TenantResolver) *Service {
	return &Service{repo: repo, rawDB: rawDB, resolver: resolver}
}

var codeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`)

func normalizeCode(raw string) (string, error) {
	code := strings.ToLower(strings.TrimSpace(raw))
	code = strings.ReplaceAll(code, "-", "_")
	code = strings.ReplaceAll(code, " ", "_")
	if !codeRe.MatchString(code) {
		return "", apperror.Validation(map[string][]string{
			"code": {"kode tenant harus 3-50 karakter huruf kecil, angka, dan garis bawah"},
		})
	}
	return code, nil
}

func randomPassword() (string, error) {
	plain, _, err := appauth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	return plain[:20], nil
}

// Provision creates a brand new tenant end to end: validates the code,
// creates its schema, applies every tenant migration, seeds permissions
// and the owner role, creates the owner user, and marks the tenant active
// — all inside one *sql.Tx, so a failure at any step leaves nothing behind
// (PostgreSQL DDL is transactional, so even CREATE SCHEMA rolls back).
//
// Tenant migrations are applied by reading the same files golang-migrate
// itself uses (migration.TenantMigrationFiles) and exec'ing their SQL
// directly against this transaction, since golang-migrate manages its own
// connection and can't join a caller's transaction. Afterwards this writes
// schema_migrations by hand, in the exact format golang-migrate uses, so
// `cli migrate tenant up`/`status` (Task 7) stay accurate for this tenant.
func (s *Service) Provision(ctx context.Context, in ProvisionInput) (ProvisionResult, error) {
	code, err := normalizeCode(in.Code)
	if err != nil {
		return ProvisionResult{}, err
	}
	if !database.ValidSchemaName(code) {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("normalized code %q is not a valid schema name", code))
	}

	tx, err := s.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	defer tx.Rollback()

	tenantID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO platform.tenants (id, code, name, schema_name, status) VALUES ($1, $2, $3, $4, 'provisioning')`,
		tenantID, code, in.Name, code); err != nil {
		return ProvisionResult{}, apperror.Conflict("kode tenant sudah dipakai")
	}

	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA "`+code+`"`); err != nil {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("create schema: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO "`+code+`"`); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	files, err := migration.TenantMigrationFiles()
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	var latestVersion uint
	for _, f := range files {
		if _, err := tx.ExecContext(ctx, f.SQL); err != nil {
			return ProvisionResult{}, apperror.Internal(fmt.Errorf("apply tenant migration %d: %w", f.Version, err))
		}
		latestVersion = f.Version
	}
	if _, err := tx.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version bigint primary key, dirty boolean not null)`); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)`, latestVersion); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	for _, permCode := range permission.All {
		group := strings.SplitN(permCode, ".", 2)[0]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO permissions (id, code, "group") VALUES ($1, $2, $3)`,
			uuid.New(), permCode, group); err != nil {
			return ProvisionResult{}, apperror.Internal(err)
		}
	}

	ownerRoleID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO roles (id, name, is_system) VALUES ($1, 'owner', true)`, ownerRoleID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	// owner gets every permission via the is_system bypass at check time
	// (Task 15), not rows in role_permissions — no seed loop needed here.

	ownerID := uuid.New()
	ownerPassword, err := randomPassword()
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	ownerHash, err := appauth.HashPassword(ownerPassword)
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name, is_active) VALUES ($1, $2, $3, 'Owner', true)`,
		ownerID, in.OwnerEmail, ownerHash); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id, created_at) VALUES ($1, $2, now())`, ownerID, ownerRoleID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE platform.tenants SET status = 'active' WHERE id = $1`, tenantID); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	if err := tx.Commit(); err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}

	return ProvisionResult{
		Tenant:        TenantView{ID: tenantID.String(), Code: code, Name: in.Name, Status: "active"},
		OwnerPassword: ownerPassword,
	}, nil
}

func (s *Service) Suspend(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, t.ID, "suspended", &now); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

func (s *Service) Activate(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, t.ID, "active", nil); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

func (s *Service) Delete(ctx context.Context, code string) error {
	t, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, t.ID); err != nil {
		return err
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

const purgeMinAge = 30 * 24 * time.Hour

// Purge permanently drops a soft-deleted tenant's schema. It refuses
// unless the tenant has been soft-deleted for at least purgeMinAge — DROP
// SCHEMA can't be undone, and this delay is the only safety net against a
// mistyped tenant code destroying a client's data outright.
func (s *Service) Purge(ctx context.Context, code string) error {
	t, err := s.repo.FindByCodeIncludingDeleted(ctx, code)
	if err != nil {
		return err
	}
	if t.DeletedAt == nil {
		return apperror.Conflict("tenant belum dihapus (soft delete) — hapus dulu sebelum purge")
	}
	if time.Since(*t.DeletedAt) < purgeMinAge {
		return apperror.Conflict("tenant baru bisa di-purge 30 hari setelah dihapus")
	}
	if !database.ValidSchemaName(t.SchemaName) {
		return apperror.Internal(fmt.Errorf("invalid schema name %q", t.SchemaName))
	}
	if _, err := s.rawDB.ExecContext(ctx, `DROP SCHEMA "`+t.SchemaName+`" CASCADE`); err != nil {
		return apperror.Internal(err)
	}
	s.resolver.Invalidate(t.ID)
	return nil
}

var sortable = map[string]string{
	"created_at": "created_at",
	"code":       "code",
	"name":       "name",
}

func (s *Service) List(ctx context.Context, p pagination.Params) ([]TenantView, response.Meta, error) {
	tenants, total, err := s.repo.List(ctx, p.Limit, p.Offset(), p.OrderByClause(sortable))
	if err != nil {
		return nil, response.Meta{}, err
	}
	views := make([]TenantView, len(tenants))
	for i, t := range tenants {
		views[i] = TenantView{ID: t.ID.String(), Code: t.Code, Name: t.Name, Status: t.Status}
	}
	return views, pagination.BuildMeta(p.Page, p.Limit, total), nil
}
