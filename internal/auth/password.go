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

// commonPasswords daftar pendek password yang panjangnya lolos pengecekan
// tapi tetap gampang banget ditebak. Sengaja dibikin kecil — syarat
// panjang sudah ngerjain sebagian besar tugasnya, ini cuma nangkep yang
// paling kelewat jelas.
var commonPasswords = map[string]bool{
	"password":  true,
	"12345678":  true,
	"123456789": true,
	"qwertyui":  true,
	"11111111":  true,
	"password1": true,
	"letmein11": true,
}

// ValidatePasswordStrength nerapin aturan password di starter ini:
// minimal 8 karakter dan nggak masuk daftar password pasaran. Sengaja
// nggak ada kewajiban huruf besar/angka/simbol — aturan begitu sudah
// banyak dibuktikan cuma bikin orang milih pola yang ketebak atau nyatet
// passwordnya, tanpa bikin lebih susah ditebak dibanding sekadar bikin
// lebih panjang.
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
