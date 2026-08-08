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

// HasPermission implements middleware.PermissionChecker. A user holding a
// role with is_system=true (currently only "owner") is granted every
// permission via this bypass, rather than rows in role_permissions, so a
// newly added permission is automatically available to owners without a
// backfill migration.
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
