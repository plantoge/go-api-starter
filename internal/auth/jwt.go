package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	ScopePlatform = "platform"
	ScopeTenant   = "tenant"
)

type Claims struct {
	jwt.RegisteredClaims
	Scope    string `json:"scope"`
	TenantID string `json:"tenant_id,omitempty"`
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL}
}

// IssueAccessToken signs a short-lived access token. tenantID must be
// non-nil for ScopeTenant and nil for ScopePlatform — each side's auth
// service (Tasks 12 and 16) enforces which is which; this just carries
// whatever it's given.
func (tm *TokenManager) IssueAccessToken(userID uuid.UUID, scope string, tenantID *uuid.UUID) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tm.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Scope: scope,
	}
	if tenantID != nil {
		claims.TenantID = tenantID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

// Verify parses and validates tokenString: signature and expiry only. It
// never inspects permissions — permissions are deliberately not carried in
// the token (see Task 15) — so a verified token proves who the caller is
// and which scope they hold, nothing more.
func (tm *TokenManager) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
