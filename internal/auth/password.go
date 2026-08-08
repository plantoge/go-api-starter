package auth

import (
	"golang.org/x/crypto/bcrypt"

	"go-api-starter/internal/apperror"
)

const BcryptCost = 12

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// commonPasswords is a short blocklist of passwords long enough to pass
// the length check but still trivially guessable. Intentionally small —
// the length requirement does most of the work; this just catches the
// most obvious misses.
var commonPasswords = map[string]bool{
	"password":  true,
	"12345678":  true,
	"123456789": true,
	"qwertyui":  true,
	"11111111":  true,
	"password1": true,
	"letmein11": true,
}

// ValidatePasswordStrength enforces the starter's password policy: at
// least 8 characters, not on the common-password blocklist. Deliberately
// no uppercase/digit/symbol requirement — those rules are well documented
// to push people toward predictable patterns or writing passwords down,
// without meaningfully raising guess-resistance over plain length.
func ValidatePasswordStrength(plain string) *apperror.Error {
	details := map[string][]string{}
	if len(plain) < 8 {
		details["password"] = append(details["password"], "minimal 8 karakter")
	}
	if commonPasswords[plain] {
		details["password"] = append(details["password"], "password terlalu umum, gunakan yang lain")
	}
	if len(details) > 0 {
		return apperror.Validation(details)
	}
	return nil
}
