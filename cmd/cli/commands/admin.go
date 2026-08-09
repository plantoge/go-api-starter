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

// cmdAdminCreate solves the chicken-and-egg problem of the very first
// platform admin: creating one normally requires being logged in as one,
// but platform.users starts empty. This command runs with no auth check
// at all — its safety comes from requiring shell access to the server, not
// from a token — and prints a random password once, which must be changed
// on first login.
func cmdAdminCreate(args []string) error {
	fs := flag.NewFlagSet("admin create", flag.ExitOnError)
	email := fs.String("email", "", "admin email (required)")
	name := fs.String("name", "", "admin name (required)")
	fs.Parse(args)

	if *email == "" || *name == "" {
		return fmt.Errorf("both --email and --name are required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.NewPlatformPool(cfg.DB)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
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

	// Bootstrap has no prior admin to be an actor, so this uses the same
	// fixed CLI sentinel (uuid.Nil, scope "cli") as the other CLI commands
	// that mutate audit-column-bearing rows — see database.Actor.Scope.
	_, err = pool.Exec(
		`INSERT INTO users (id, email, password_hash, name, is_active, created_by) VALUES ($1, $2, $3, $4, true, $5)`,
		uuid.New(), *email, hash, *name, uuid.Nil)
	if err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}

	fmt.Println("admin created:")
	fmt.Println("  email:   ", *email)
	fmt.Println("  password:", password)
	fmt.Println("(this password is shown once — change it after your first login)")
	return nil
}

func generateRandomPassword() (string, error) {
	// Reuses the same crypto/rand 32-byte generator as refresh tokens
	// rather than adding a second random-string helper.
	plain, _, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	return plain[:20], nil
}
