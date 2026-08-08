// Package testsupport provides helpers for tests that need a real
// PostgreSQL connection. It must only ever be imported from _test.go
// files — it depends on "testing", which has no place in a shipped binary.
package testsupport

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// OpenTestDB connects to the app_test database described by .env.test at
// the repo root. It skips (not fails) the calling test when no local
// PostgreSQL is reachable, so `go test ./...` stays usable on a machine
// that hasn't set one up — the machine that has gets full coverage.
func OpenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	_ = godotenv.Load(findEnvTestFile())

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("skipping: no local PostgreSQL test database reachable (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// OpenTestPlatformDB is OpenTestDB with search_path pinned to "platform",
// mirroring database.NewPlatformPool in production — for tests of code
// that expects to write unqualified names ("FROM login_attempts") because
// it assumes a platform-pinned connection.
func OpenTestPlatformDB(t *testing.T) *sqlx.DB {
	t.Helper()
	_ = godotenv.Load(findEnvTestFile())

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&search_path=platform",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Skipf("skipping: no local PostgreSQL test database reachable (%v)", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// RandomSchemaName returns a name like "test_a1b2c3d4" that satisfies the
// tenant schema-name regex, for tests that need their own throwaway schema.
// Uses the package-level math/rand source directly (auto-seeded since
// Go 1.20) instead of reseeding a new source per call, which on coarse
// clock granularity could otherwise produce duplicate names on
// back-to-back calls.
func RandomSchemaName() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "test_" + string(b)
}

// SeedInSchema runs a fixture-setup query with search_path pinned to
// schema, for tests that need to insert rows directly rather than going
// through database.WithTenant themselves.
func SeedInSchema(t *testing.T, pool *sqlx.DB, schema, query string, args ...any) {
	t.Helper()
	tx, err := pool.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SET LOCAL search_path TO "`+schema+`"`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := tx.Exec(query, args...); err != nil {
		t.Fatalf("seed query: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findEnvTestFile() string {
	candidates := []string{".env.test", "../../.env.test", "../../../.env.test"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ".env.test"
}
