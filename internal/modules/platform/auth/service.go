package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go-api-starter/internal/apperror"
	appauth "go-api-starter/internal/auth"
	"go-api-starter/internal/ratelimit"
)

type adminUser struct {
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	IsActive     bool      `db:"is_active"`
}

type Service struct {
	db          *sqlx.DB // pool yang dikunci ke schema platform
	tokens      *appauth.TokenManager
	accessTTL   time.Duration
	refreshTTL  time.Duration
	rateLimiter *ratelimit.LoginAttemptService
}

func NewService(platformDB *sqlx.DB, tokens *appauth.TokenManager, accessTTL, refreshTTL time.Duration, rateLimiter *ratelimit.LoginAttemptService) *Service {
	return &Service{db: platformDB, tokens: tokens, accessTTL: accessTTL, refreshTTL: refreshTTL, rateLimiter: rateLimiter}
}

// dummyPasswordHash itu hash bcrypt yang sah dari sebuah password yang
// nggak dipakai dan nggak mungkin ditebak. Login selalu manggil
// VerifyPassword tepat sekali — ke hash user aslinya kalau ketemu, atau ke
// hash tetap ini kalau nggak — jadi lama waktu respons nggak pernah
// ngebocorin mana email yang ada/aktif dan mana yang cuma salah password.
//
// (Catatan perbaikan saat Task 12 dikerjakan: versi awalnya kelewat
// manggil VerifyPassword gara-gara short-circuit && tiap kali user-nya
// nggak ketemu atau nggak aktif. Itu bikin celah timing yang beneran bisa
// diukur buat nyari tahu email mana yang terdaftar.)
var dummyPasswordHash = mustHashDummyPassword()

func mustHashDummyPassword() string {
	hash, err := appauth.HashPassword("not-a-real-password-used-only-for-constant-time-comparison")
	if err != nil {
		panic(err)
	}
	return hash
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	if err := s.rateLimiter.Check(ctx, appauth.ScopePlatform, nil, req.Email); err != nil {
		return LoginResponse{}, err
	}

	var u adminUser
	err := s.db.GetContext(ctx, &u,
		`SELECT id, email, password_hash, is_active FROM users WHERE email = $1 AND deleted_at IS NULL`,
		req.Email)
	found := err == nil && u.IsActive

	hashToCheck := dummyPasswordHash
	if found {
		hashToCheck = u.PasswordHash
	}
	// VerifyPassword selalu dipanggil — ke hash asli kalau user-nya ketemu,
	// ke hash dummy tetap kalau nggak — biar ongkos baris ini (yang
	// didominasi bcrypt) sama di semua jalur, terlepas dari ada tidaknya
	// user tersebut.
	passwordOK := appauth.VerifyPassword(hashToCheck, req.Password)
	valid := found && passwordOK

	// Kalau pencatatan percobaannya sendiri gagal, perlindungan brute-force
	// buat email/scope ini diam-diam melemah — makanya tetap di-log.
	// Login-nya sendiri tetap diputuskan dengan benar, cuma penghitung
	// rate limit-nya yang mungkin kurang dari kenyataan.
	if err := s.rateLimiter.Record(ctx, appauth.ScopePlatform, nil, req.Email, valid); err != nil {
		slog.Error("rate limiter record failed", "scope", appauth.ScopePlatform, "error", err)
	}
	if !valid {
		return LoginResponse{}, apperror.Unauthorized("email atau password salah")
	}

	return s.issueTokens(ctx, u.ID)
}

func (s *Service) issueTokens(ctx context.Context, userID uuid.UUID) (LoginResponse, error) {
	access, err := s.tokens.IssueAccessToken(userID, appauth.ScopePlatform, nil)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	plain, hash, err := appauth.GenerateRefreshToken()
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		uuid.New(), userID, hash, time.Now().Add(s.refreshTTL))
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}

	return LoginResponse{AccessToken: access, RefreshToken: plain, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (LoginResponse, error) {
	hash := appauth.HashRefreshToken(req.RefreshToken)

	var row struct {
		UserID    uuid.UUID  `db:"user_id"`
		ExpiresAt time.Time  `db:"expires_at"`
		RevokedAt *time.Time `db:"revoked_at"`
	}
	err := s.db.GetContext(ctx, &row,
		`SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash)
	if err != nil {
		return LoginResponse{}, apperror.Unauthorized("refresh token tidak valid")
	}
	if row.RevokedAt != nil || time.Now().After(row.ExpiresAt) {
		return LoginResponse{}, apperror.Unauthorized("refresh token sudah tidak berlaku")
	}

	// Nggak ada rotasi (sesuai spec: refresh token nggak dirotasi tiap
	// dipakai) — refresh token yang sama tetap berlaku, yang diterbitkan
	// cuma access token baru.
	access, err := s.tokens.IssueAccessToken(row.UserID, appauth.ScopePlatform, nil)
	if err != nil {
		return LoginResponse{}, apperror.Internal(err)
	}
	return LoginResponse{AccessToken: access, RefreshToken: req.RefreshToken, ExpiresIn: int(s.accessTTL.Seconds())}, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	hash := appauth.HashRefreshToken(req.RefreshToken)
	_, err := s.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return apperror.Internal(err)
	}
	return nil
}
