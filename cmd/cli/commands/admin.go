package commands

import (
	"flag"
	"fmt"

	"github.com/google/uuid"

	"go-api-starter/internal/auth"
	"go-api-starter/internal/config"
	"go-api-starter/internal/database"
)

func init() {
	Register("admin create", cmdAdminCreate)
}

// cmdAdminCreate ngatasin masalah ayam-dan-telur di admin platform
// pertama: mau bikin admin biasanya harus login dulu sebagai admin,
// padahal tabel platform.users awalnya masih kosong.
//
// Perintah ini sengaja nggak ngecek autentikasi sama sekali. Pengamanannya
// bukan token, tapi kenyataan bahwa orangnya harus punya akses shell ke
// server. Password acaknya dicetak sekali doang, dan wajib diganti pas
// login pertama.
func cmdAdminCreate(args []string) error {
	fs := flag.NewFlagSet("admin create", flag.ExitOnError)
	email := fs.String("email", "", "email admin (wajib diisi)")
	name := fs.String("name", "", "nama admin (wajib diisi)")
	fs.Parse(args)

	if *email == "" || *name == "" {
		return fmt.Errorf("--email dan --name dua-duanya wajib diisi")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		return fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	defer pool.Close()

	password, err := generateRandomPassword()
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	// Pas bootstrap belum ada admin lain yang bisa dicatat sebagai pelaku,
	// jadi dipakai penanda CLI yang tetap (uuid.Nil, scope "cli") — sama
	// kayak perintah CLI lain yang nyentuh baris berkolom audit. Detailnya
	// di database.Actor.Scope.
	_, err = pool.Exec(
		`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, $4, true, $5)`,
		uuid.New(), *email, hash, *name, uuid.Nil)
	if err != nil {
		return fmt.Errorf("gagal menyimpan user admin: %w", err)
	}

	fmt.Println("admin berhasil dibuat:")
	fmt.Println("  email:   ", *email)
	fmt.Println("  password:", password)
	fmt.Println("(password ini hanya ditampilkan sekali — ganti setelah login pertama)")
	return nil
}

func generateRandomPassword() (string, error) {
	// Pakai ulang generator crypto/rand 32 byte punya refresh token,
	// daripada bikin helper string acak kedua yang isinya sama aja.
	plain, _, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	return plain[:20], nil
}
