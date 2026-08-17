package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// Sortable whitelist kolom yang boleh dipakai endpoint list buat
// mengurutkan data, dan dibaca lewat pagination.Params.OrderByClause —
// bukan dari nilai query mentah. Tiru pola ini di tiap modul ber-scope
// tenant yang baru.
var Sortable = map[string]string{
	"created_at": "created_at",
	"name":       "name",
	"email":      "email",
}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func wrapDBErr(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.NotFound("user tidak ditemukan")
	}
	return apperror.Internal(err)
}

// Catatan perbaikan (dibuat saat Task 17 dikerjakan): versi awal fungsi
// ini nganggep semua kegagalan INSERT sebagai "email sudah dipakai" —
// persis bug salah klasifikasi yang ketemu dan dibenerin di review Task 14
// buat penyiapan tenant. Akibatnya tabel yang hilang, koneksi yang putus,
// atau kegagalan nyata apa pun bakal dilaporin sebagai email duplikat, dan
// penyebab aslinya ketutup.
//
// Modul ini template yang bakal disalin ke tiap modul ber-scope tenant di
// masa depan, jadi benerin polanya di sini — daripada dibiarkan lolos lalu
// ikut tersalin ke mana-mana — jauh lebih penting ketimbang di modul yang
// berdiri sendiri. Pengecekannya pakai teknik yang sama dengan Task 14,
// yaitu baca kode error asli dari Postgres, bukan main asumsi.
func (r *Repository) Create(ctx context.Context, u User) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, $4, $5, $6)`,
			u.ID, u.Email, u.PasswordHash, u.Name, u.IsActive, actor.UserID)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("email sudah dipakai")
		}
		// Kalau yang gagal justru WithTenant-nya (misal context tenant nggak
		// ada, atau nama schema-nya nggak valid), dia sudah ngasih
		// *apperror.Error yang terklasifikasi. wrapDBErr ngelewatin itu apa
		// adanya, nggak dibungkus lagi jadi apperror.Internal kedua — soalnya
		// pembungkusan ganda malah ngehapus klasifikasi aslinya.
		return wrapDBErr(fmt.Errorf("gagal menyimpan user: %w", err))
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		return tx.Get(&u, `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	})
	if err != nil {
		return User{}, wrapDBErr(err)
	}
	return u, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int, orderBy string) ([]User, int, error) {
	var users []User
	var total int
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		query := `SELECT * FROM users WHERE deleted_at IS NULL ORDER BY ` + orderBy + ` LIMIT $1 OFFSET $2`
		if err := tx.Select(&users, query, limit, offset); err != nil {
			return err
		}
		return tx.Get(&total, `SELECT count(*) FROM users WHERE deleted_at IS NULL`)
	})
	if err != nil {
		return nil, 0, wrapDBErr(err)
	}
	return users, total, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, name *string, isActive *bool) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.Exec(
			`UPDATE users SET name = COALESCE($2, name), is_active = COALESCE($3, is_active), updated_by = $4
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, name, isActive, actor.UserID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperror.NotFound("user tidak ditemukan")
		}
		return nil
	})
	return wrapDBErr(err)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	actor, _ := database.ActorFromContext(ctx)
	err := r.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.Exec(
			`UPDATE users SET deleted_at = now(), deleted_by = $2 WHERE id = $1 AND deleted_at IS NULL`,
			id, actor.UserID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return apperror.NotFound("user tidak ditemukan")
		}
		return nil
	})
	return wrapDBErr(err)
}
