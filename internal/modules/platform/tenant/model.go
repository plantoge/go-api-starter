package tenant

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID          uuid.UUID  `db:"id"`
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	SchemaName  string     `db:"schema_name"`
	Status      string     `db:"status"`
	SuspendedAt *time.Time `db:"suspended_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}
