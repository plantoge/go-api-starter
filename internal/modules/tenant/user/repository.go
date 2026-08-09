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

// Sortable is the whitelist of columns the list endpoint may sort by,
// consumed through pagination.Params.OrderByClause — never the raw query
// value. Copy this pattern for every new tenant-scoped module.
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

// Amendment (added during Task 17 execution): the original version of
// this function collapsed every INSERT failure into "email sudah dipakai"
// — the same misclassification bug Task 14's review found and fixed for
// tenant provisioning (a missing table, a lost connection, or any other
// real failure would have been misreported as a duplicate email, hiding
// the actual cause). Because this module is the copy-paste template every
// future tenant-scoped module is built from, fixing the pattern here
// (rather than letting it ship and get copied forward) matters more than
// in an isolated module — this is checked via the same actual-Postgres-
// error-code technique Task 14 uses, not a broadened assumption.
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
		// If WithTenant itself failed (e.g. missing tenant context, invalid
		// schema name), it already returns a classified *apperror.Error —
		// wrapDBErr passes that through unchanged instead of double-wrapping
		// it in another apperror.Internal, which would erase the original
		// classification (e.g. turning a caller-error Internal into a
		// second, less specific Internal).
		return wrapDBErr(fmt.Errorf("insert user: %w", err))
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
