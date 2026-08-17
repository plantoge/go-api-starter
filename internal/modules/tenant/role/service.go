package role

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

type Service struct {
	db *database.DB
}

func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

// HasPermission ngimplementasiin middleware.PermissionChecker. User yang
// megang role dengan is_system=true (sekarang cuma "owner") langsung
// dianggap punya semua permission lewat jalan pintas ini, bukan lewat
// baris di role_permissions. Jadi permission yang baru ditambahin otomatis
// kepegang owner tanpa perlu migrasi backfill.
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permCode string) (bool, error) {
	var allowed bool
	err := s.db.WithTenant(ctx, func(tx *sqlx.Tx) error {
		var isSystemOwner bool
		if err := tx.Get(&isSystemOwner, `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id
				WHERE ur.user_id = $1 AND r.is_system = true
			)`, userID); err != nil {
			return err
		}
		if isSystemOwner {
			allowed = true
			return nil
		}

		return tx.Get(&allowed, `
			SELECT EXISTS (
				SELECT 1 FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id = ur.role_id
				JOIN permissions p ON p.id = rp.permission_id
				WHERE ur.user_id = $1 AND p.code = $2
			)`, userID, permCode)
	})
	if err != nil {
		return false, apperror.Internal(err)
	}
	return allowed, nil
}
