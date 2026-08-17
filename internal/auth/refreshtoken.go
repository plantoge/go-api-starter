package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateRefreshToken bikin refresh token acak yang baru. Nilai plain
// dikirim ke klien, sedangkan yang disimpan di database cuma hash-nya —
// nggak pernah nilai plain-nya. Jadi kalau dump database sampai bocor,
// isinya nggak bisa dipakai ulang sebagai refresh token yang sah.
func GenerateRefreshToken() (plain string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, HashRefreshToken(plain), nil
}

func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
