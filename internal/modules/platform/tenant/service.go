package tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

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

// Provision nyiapin tenant baru dari nol sampai jadi: validasi kodenya,
// bikin schema, nerapin semua migrasi tenant, nanam permission dan role
// owner, bikin user owner, lalu nandain tenant-nya aktif. Semuanya di
// dalam satu *sql.Tx, jadi kalau ada satu langkah yang gagal nggak ada
// sisa apa pun — DDL di PostgreSQL ikut transaksional, jadi CREATE SCHEMA
// pun ikut ke-rollback.
//
// Migrasi tenant diterapkan dengan cara baca file yang sama persis dengan
// yang dipakai golang-migrate (migration.TenantMigrationFiles), lalu
// SQL-nya dijalankan langsung di transaksi ini. Soalnya golang-migrate
// ngurus koneksinya sendiri dan nggak bisa ikut transaksi pemanggil.
// Setelah itu schema_migrations ditulis manual, dengan format yang persis
// sama kayak punya golang-migrate, biar `cli migrate tenant up` dan
// `status` (Task 7) tetap akurat buat tenant ini.
func (s *Service) Provision(ctx context.Context, in ProvisionInput) (ProvisionResult, error) {
	code, err := normalizeCode(in.Code)
	if err != nil {
		return ProvisionResult{}, err
	}
	if !database.ValidSchemaName(code) {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("kode %q setelah dinormalkan bukan nama schema yang valid", code))
	}

	// Tenant yang sudah di-soft delete tapi belum di-purge itu schema-nya
	// masih nangkring di disk, padahal kodenya sudah bebas dipakai baris
	// baru (tenants_code_key itu unique index parsial dengan syarat
	// deleted_at IS NULL). Tanpa pengecekan ini, Provision bakal lolos di
	// INSERT bawah, lalu gagal jauh di dalam transaksi pas CREATE SCHEMA
	// dengan error "already exists" yang bikin bingung. Dicek di depan biar
	// yang muncul konflik yang jelas.
	//
	// Baris dengan kode yang sama tapi statusnya active/provisioning
	// sengaja NGGAK ditolak di sini — biar jatuh ke INSERT di bawah, karena
	// di situlah constraint keunikan yang sesungguhnya berada dan
	// klasifikasi error-nya jadi benar.
	if existing, findErr := s.repo.FindByCodeIncludingDeleted(ctx, code); findErr == nil && existing.DeletedAt != nil {
		return ProvisionResult{}, apperror.Conflict("kode tenant ini pernah dipakai dan belum di-purge — tunggu purge selesai atau gunakan kode lain")
	}

	tx, err := s.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return ProvisionResult{}, apperror.Internal(err)
	}
	defer tx.Rollback()

	actor, _ := database.ActorFromContext(ctx)
	tenantID := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO platform.tenants (id, code, name, schema_name, status, created_by) VALUES ($1, $2, $3, $4, 'provisioning', $5)`,
		tenantID, code, in.Name, code, actor.UserID); err != nil {
		// Yang boleh dianggap kode duplikat cuma pelanggaran unique
		// constraint beneran di tenants_code_key (SQLSTATE 23505). Selain
		// itu ya memang error Internal sungguhan.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ProvisionResult{}, apperror.Conflict("kode tenant sudah dipakai")
		}
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("gagal menyimpan baris tenant: %w", err))
	}

	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA "`+code+`"`); err != nil {
		return ProvisionResult{}, apperror.Internal(fmt.Errorf("gagal membuat schema: %w", err))
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
			return ProvisionResult{}, apperror.Internal(fmt.Errorf("gagal menerapkan migrasi tenant %d: %w", f.Version, err))
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
	// owner dapat semua permission lewat jalan pintas is_system pas
	// pengecekan (Task 15), bukan dari baris di role_permissions — jadi
	// nggak perlu loop penanaman data di sini.

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
		`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, 'Owner', true, $4)`,
		ownerID, in.OwnerEmail, ownerHash, actor.UserID); err != nil {
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

// Purge ngedrop schema milik tenant yang sudah di-soft delete, permanen.
// Dia bakal nolak kalau tenant-nya belum di-soft delete selama minimal
// purgeMinAge. DROP SCHEMA nggak bisa dibatalkan, dan jeda inilah
// satu-satunya jaring pengaman kalau kode tenant sampai salah ketik dan
// data klien orang kehapus semua.
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
		return apperror.Internal(fmt.Errorf("nama schema tidak valid: %q", t.SchemaName))
	}

	tx, err := s.rawDB.BeginTx(ctx, nil)
	if err != nil {
		return apperror.Internal(err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DROP SCHEMA IF EXISTS "`+t.SchemaName+`" CASCADE`); err != nil {
		return apperror.Internal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM platform.tenants WHERE id = $1`, t.ID); err != nil {
		return apperror.Internal(err)
	}
	if err := tx.Commit(); err != nil {
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
