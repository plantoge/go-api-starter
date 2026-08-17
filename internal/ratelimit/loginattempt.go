package ratelimit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go-api-starter/internal/apperror"
)

type LoginAttemptService struct {
	db          *sqlx.DB // dikunci ke schema platform
	maxAttempts int
	window      time.Duration
}

func NewLoginAttemptService(platformDB *sqlx.DB, maxAttempts int, window time.Duration) *LoginAttemptService {
	return &LoginAttemptService{db: platformDB, maxAttempts: maxAttempts, window: window}
}

// Check balikin apperror.RateLimited kalau email tersebut — dihitung per
// scope, dan buat login tenant juga per tenantID — sudah gagal login
// sebanyak maxAttempts kali atau lebih dalam rentang window terakhir.
// Dipanggil sebelum kredensial diperiksa, jadi pemanggil yang kena blokir
// bahkan nggak sampai ke tahap pengecekan password.
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

// Record nyatet satu percobaan login, berhasil maupun gagal. Semua alur
// login manggil ini tepat sekali per percobaan: setelah Check, dan setelah
// kredensialnya selesai diperiksa (baik cocok maupun nggak).
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
