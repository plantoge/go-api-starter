package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/database"
	"go-api-starter/internal/middleware"
	"go-api-starter/internal/ratelimit"
)

// TenantLookup ngubah tenant_code jadi informasi yang cukup buat mencoba
// login. Dideklarasikan di sini, di sisi pemakai, dan diimplementasikan
// sama modules/platform/tenant.Repository (Task 14).
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

// dummyPasswordHash itu hash bcrypt yang sah dari password yang nggak
// dipakai dan nggak mungkin ditebak. Login selalu manggil VerifyPassword
// tepat sekali — ke hash user aslinya kalau ketemu dan aktif, atau ke hash
// tetap ini kalau nggak — jadi lama waktu respons nggak pernah ngebocorin
// mana email yang ada/aktif dan mana yang cuma salah password. Polanya dan
// alasannya sama persis kayak service auth admin platform di Task 12;
// ditulis ulang di sini karena beda package.
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

	// Login satu-satunya tempat identitas tenant ditentukan tanpa lewat
	// RequireTenant duluan — wong token-nya memang belum ada — jadi
	// TenantInfo dimasukin ke ctx secara manual sebelum pakai WithTenant.
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
		return nil // user nggak ketemu bukan berarti transaksinya gagal — cuma login yang nggak sah
	})
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	// Catatan perbaikan (dibuat barengan dengan perbaikan serupa di Task
	// 12, alasannya sama): VerifyPassword selalu dipanggil — ke hash asli
	// kalau user-nya ketemu dan aktif, ke dummyPasswordHash kalau nggak —
	// biar ongkos baris ini sama di semua jalur. Tanpa ini, short-circuit
	// && bakal ngelewatin panggilan bcrypt tiap kali user-nya nggak
	// ketemu atau nggak aktif, dan lama respons jadi ngebocorin email mana
	// yang terdaftar. Package ini punya dummyPasswordHash dan
	// mustHashDummyPassword sendiri (polanya sama kayak service auth
	// platform di Task 12, tapi bukan variabel yang sama — beda package).
	activeFound := found && u.IsActive
	hashToCheck := dummyPasswordHash
	if activeFound {
		hashToCheck = u.PasswordHash
	}
	passwordOK := appauth.VerifyPassword(hashToCheck, req.Password)
	valid := activeFound && passwordOK
	// Kalau pencatatan percobaannya sendiri gagal, perlindungan brute-force
	// buat email/tenant ini diam-diam melemah — makanya tetap di-log.
	// Login-nya sendiri tetap diputuskan dengan benar, cuma penghitung
	// rate limit-nya yang mungkin kurang dari kenyataan.
	if err := s.rateLimiter.Record(ctx, appauth.ScopeTenant, &tenantRec.TenantID, req.Email, valid); err != nil {
		slog.Error("rate limiter record failed", "scope", appauth.ScopeTenant, "tenant_id", tenantRec.TenantID, "error", err)
	}
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
