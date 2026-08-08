package ratelimit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/apperror"
)

type LoginAttemptService struct {
	db          *sqlx.DB // pinned to the platform schema
	maxAttempts int
	window      time.Duration
}

func NewLoginAttemptService(platformDB *sqlx.DB, maxAttempts int, window time.Duration) *LoginAttemptService {
	return &LoginAttemptService{db: platformDB, maxAttempts: maxAttempts, window: window}
}

// Check returns apperror.RateLimited if email — scoped to scope and, for
// tenant logins, tenantID — has failed to log in maxAttempts times or more
// within the last window. Called before verifying credentials, so a
// blocked caller never even reaches the password check.
func (s *LoginAttemptService) Check(ctx context.Context, scope string, tenantID *uuid.UUID, email string) error {
	const query = `
		SELECT count(*) FROM login_attempts
		WHERE scope = $1 AND email = $2 AND success = false
		  AND attempted_at > now() - ($3 * interval '1 second')
		  AND tenant_id IS NOT DISTINCT FROM $4`
	var count int
	err := s.db.GetContext(ctx, &count, query, scope, email, s.window.Seconds(), tenantIDValue(tenantID))
	if err != nil {
		return apperror.Internal(err)
	}
	if count >= s.maxAttempts {
		return apperror.RateLimited("terlalu banyak percobaan login, coba lagi nanti")
	}
	return nil
}

// Record logs one login attempt, successful or not. Every login flow calls
// this exactly once per attempt, after Check and after verifying (or
// failing to verify) credentials.
func (s *LoginAttemptService) Record(ctx context.Context, scope string, tenantID *uuid.UUID, email string, success bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO login_attempts (id, scope, tenant_id, email, success) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), scope, tenantIDValue(tenantID), email, success,
	)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func tenantIDValue(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
