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

// ValidSchemaName ngecek apakah name aman ditempel ke SQL sebagai
// identifier. Dicek sekali waktu tenant disiapkan, lalu dicek lagi di sini
// sebagai lapis pengaman tiap kali WithTenant mau memakainya.
func ValidSchemaName(name string) bool {
	return schemaNameRe.MatchString(name)
}

// quoteIdentifier ngasih tanda kutip ganda ke identifier Postgres, sambil
// nge-escape kutip yang ada di dalamnya. Sengaja dibikin helper kecil di
// sini daripada narik dependency lib/pq cuma demi QuoteIdentifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type TenantInfo struct {
	TenantID   uuid.UUID
	SchemaName string
}

type Actor struct {
	UserID uuid.UUID
	// Scope isinya "platform" atau "tenant" buat pemanggil yang login lewat
	// HTTP, atau "cli" buat penanda pelaku tetap (UserID: uuid.Nil) yang
	// dipakai perintah CLI. Penanda itu gunanya ngisi kolom audit *_by pada
	// operasi yang memang nggak punya user login beneran (lihat
	// cmd/cli/commands/tenant.go dan admin.go).
	Scope string
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

// WithTenant jalanin fn di dalam satu transaksi yang search_path-nya
// dikunci pakai SET LOCAL ke schema milik tenant yang ada di ctx. SET
// LOCAL cuma berlaku selama transaksi — commit maupun rollback
// sama-sama otomatis ngebersihinnya — jadi koneksinya selalu balik ke pool
// dalam keadaan bersih, tanpa perlu langkah reset manual yang bisa saja
// kelupaan di jalur error.
func (db *DB) WithTenant(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	info, ok := TenantFromContext(ctx)
	if !ok {
		return apperror.Internal(fmt.Errorf("WithTenant dipanggil tanpa context tenant"))
	}
	if !ValidSchemaName(info.SchemaName) {
		return apperror.Internal(fmt.Errorf("nama schema tidak valid: %q", info.SchemaName))
	}

	tx, err := db.Pool.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Internal(fmt.Errorf("gagal memulai transaksi: %w", err))
	}
	defer tx.Rollback() //nolint:errcheck // nggak ngefek apa-apa kalau sudah commit

	// search_path sengaja dikunci cuma ke schema tenant, tanpa "public".
	// Artinya objek extension yang biasanya dipasang di sana (pgcrypto,
	// uuid-ossp, citext) NGGAK bakal kebaca di dalam WithTenant kalau
	// ditulis tanpa prefix schema. Ini pilihan isolasi yang disengaja,
	// bukan kelupaan; tinjau lagi kalau nanti migrasi tenant memang butuh
	// extension tersebut.
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO "+quoteIdentifier(info.SchemaName)); err != nil {
		return apperror.Internal(fmt.Errorf("gagal menyetel search_path: %w", err))
	}

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperror.Internal(fmt.Errorf("gagal commit transaksi: %w", err))
	}
	return nil
}
