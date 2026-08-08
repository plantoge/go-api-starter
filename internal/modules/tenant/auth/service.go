package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/ratelimit"
)

// TenantLookup resolves a tenant_code to enough info to attempt a login.
// Declared here, consumer-side, and implemented by
// modules/platform/tenant.Repository (Task 14).
type TenantLookup interface {
	FindRecordByCode(ctx context.Context, code string) (middleware.TenantRecord, error)
}

type tenantUser struct {
	ID           uuid.UUID `db:"id"`
	PasswordHash string    `db:"password_hash"`
	IsActive     bool      `db:"is_active"`
}

type Service struct {
	db          *database.DB
	tenants     TenantLookup
	tokens      *appauth.TokenManager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rateLimiter *ratelimit.LoginAttemptService
}

func NewService(db *database.DB, tenants TenantLookup, tokens *appauth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service {
	return &Service{db: db, tenants: tenants, tokens: tokens, accessTTL: accessTTL, refreshTTL: refreshTTL, rateLimiter: rateLimiter}
}

// dummyPasswordHash is a valid bcrypt hash of an unused, unguessable
// password. Login always runs exactly one VerifyPassword call — against
// the real user's hash when found and active, or against this fixed hash
// when not — so response timing never reveals whether an email
// exists/is active versus exists-with-a-wrong-password. Same pattern and
// same rationale as Task 12's platform admin auth service, duplicated
// here because it's a different package.
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := appauth.HashPassword("not-a-real-password-used-only-for-constant-time-comparison")
	if err != nil {
		panic(err)
	}
	return hash
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil || tenantRec.Status != "active" {
		return LoginResponse{}, apperror.Unauthorized("tenant_code, email, atau password salah")
	}

	if err := s.rateLimiter.Check(ctx, appauth.ScopeTenant, &tenantRec.TenantID, req.Email); err != nil {
		return LoginResponse{}, err
	}

	// Login is the one place tenant identity is established without a
	// prior RequireTenant pass — there's no token yet — so TenantInfo is
	// injected into ctx by hand before using WithTenant.
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	var u tenantUser
	found := false
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		getErr := tx.Get(&u, `SELECT id, password_hash, is_active FROM users WHERE email = $1 AND deleted_at IS NULL`, req.Email)
		if getErr == nil {
			found = true
		}
		return nil // a missing user isn't a transaction failure — just an invalid login
	})
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	// Amendment (added alongside the identical Task 12 fix, for the same
	// reason): always call VerifyPassword — on the real hash if found and
	// active, on the fixed dummyPasswordHash otherwise — so this line's
	// cost is the same on every path. Without this, && short-circuiting
	// skips the bcrypt call whenever the user isn't found/inactive,
	// letting response timing leak which emails exist. This package
	// defines its own dummyPasswordHash/mustHashDummyPassword (same
	// pattern as Task 12's platform auth service, but not the same
	// variable — different package) — add them alongside Service below.
	activeFound := found && u.IsActive
	hashToCheck := dummyPasswordHash
	if activeFound {
		hashToCheck = u.PasswordHash
	}
	passwordOK := appauth.VerifyPassword(hashToCheck, req.Password)
	valid := activeFound && passwordOK
	s.rateLimiter.Record(ctx, appauth.ScopeTenant, &tenantRec.TenantID, req.Email, valid)
	if !valid {
		return LoginResponse{}, apperror.Unauthorized("tenant_code, email, atau password salah")
	}

	return s.issueTokens(tenantCtx, tenantRec.TenantID, u.ID)
}

func (s *Service) issueTokens(tenantCtx context.Context, tenantID, userID uuid.UUID) (LoginResponse, error) {
	access, err := s.tokens.IssueAccessToken(userID, appauth.ScopeTenant, &tenantID)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	plain, hash, err := appauth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		_, execErr := tx.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
			uuid.New(), userID, hash, time.Now().Add(s.refreshTTL))
		return execErr
	})
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	return LoginResponse{AccessToken: access, RefreshToken: plain, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (LoginResponse, error) {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil || tenantRec.Status != "active" {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	hash := appauth.HashRefreshToken(req.RefreshToken)
	var row struct {
		UserID    uuid.UUID  `db:"user_id"`
		ExpiresAt time.Time  `db:"expires_at"`
		RevokedAt *time.Time `db:"revoked_at"`
	}
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		return tx.Get(&row, `SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash)
	})
	if err != nil {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	if row.RevokedAt != nil || time.Now().After(row.ExpiresAt) {
		return LoginResponse{}, apperror.Unauthorized("refresh token sudah tidak berlaku")
	}

	access, err := s.tokens.IssueAccessToken(row.UserID, appauth.ScopeTenant, &tenantRec.TenantID)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	return LoginResponse{AccessToken: access, RefreshToken: req.RefreshToken, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	tenantRec, err := s.tenants.FindRecordByCode(ctx, req.TenantCode)
	if err != nil {
		return apperror.Unauthorized("tenant tidak ditemukan")
	}
	tenantCtx := database.WithTenantInfo(ctx, database.TenantInfo{
		TenantID: tenantRec.TenantID, SchemaName: tenantRec.SchemaName,
	})

	hash := appauth.HashRefreshToken(req.RefreshToken)
	err = s.db.WithTenant(tenantCtx, func(tx *sqlx.Tx) error {
		_, execErr := tx.Exec(`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
		return execErr
	})
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
