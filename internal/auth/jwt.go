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

// IssueAccessToken nandatanganin access token berumur pendek. tenantID
// wajib terisi buat ScopeTenant dan wajib nil buat ScopePlatform — yang
// nentuin mana yang mana adalah service auth di masing-masing sisi (Task
// 12 dan 16); fungsi ini cuma bawa apa yang dikasih.
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

// Verify mem-parsing dan memeriksa tokenString: sebatas tanda tangan dan
// masa berlaku. Dia nggak pernah ngecek permission — permission memang
// sengaja nggak dititipin di dalam token (lihat Task 15) — jadi token yang
// lolos verifikasi cuma membuktikan siapa pemanggilnya dan scope apa yang
// dia pegang, nggak lebih.
func (tm *TokenManager) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode penandatanganan token tidak sesuai")
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}
