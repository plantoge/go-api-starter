package permission

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"go-api-starter/internal/apperror"
	"go-api-starter/internal/database"
)

// Sync inserts any permission in All that schemaName's permissions table
// doesn't already have, and reports how many were added. Idempotent — safe
// to run against every tenant on every deploy — because permission
// constants live in Go code, not in migration files, so a newly added
// constant needs an explicit push to reach tenants provisioned before it
// existed.
func Sync(ctx context.Context, db *sql.DB, schemaName string) (added int, err error) {
	if !database.ValidSchemaName(schemaName) {
		return 0, apperror.Internal(fmt.Errorf("invalid schema name %q", schemaName))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO "`+schemaName+`"`); err != nil {
		return 0, apperror.Internal(err)
	}

	existing := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT code FROM permissions`)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			return 0, apperror.Internal(err)
		}
		existing[code] = true
	}
	rows.Close()

	for _, code := range All {
		if existing[code] {
			continue
		}
		group := strings.SplitN(code, ".", 2)[0]
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO permissions (id, code, "group") VALUES ($1, $2, $3)`,
			uuid.New(), code, group); err != nil {
			return added, apperror.Internal(err)
		}
		added++
	}

	if err := tx.Commit(); err != nil {
		return 0, apperror.Internal(err)
	}
	return added, nil
}
