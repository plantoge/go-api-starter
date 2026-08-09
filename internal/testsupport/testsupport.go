// Package testsupport provides helpers for tests that need a real
// PostgreSQL connection. It must only ever be imported from _test.go
// files — it depends on "testing", which has no place in a shipped binary.
package testsupport

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// requireTestDB reports whether REQUIRE_TEST_DB is set to a truthy value.
// A plain os.Getenv(...) != "" check would treat REQUIRE_TEST_DB=0 or
// REQUIRE_TEST_DB=false as "set" (fail-closed) too, which is surprising —
// strconv.ParseBool gives the usual off-switch values (0/false/f/...) their
// expected meaning, and anything unparseable (including unset) is off.
func requireTestDB() bool {
	b, _ := strconv.ParseBool(os.Getenv("REQUIRE_TEST_DB"))
	return b
}

// TestDSN returns the connection string OpenTestDB would use to connect,
// without actually connecting — for callers (like migration.MigrateTenantUp)
// that need a DSN string rather than an already-open *sqlx.DB, e.g. to open
// their own dedicated, schema-scoped connection.
func TestDSN() string {
	_ = godotenv.Load(findEnvTestFile())
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getenv("DB_USER", "app"),
		getenv("DB_PASSWORD", "changeme"),
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_NAME", "app_test"),
		getenv("DB_SSLMODE", "disable"),
	)
}

// OpenTestDB connects to the app_test database described by .env.test at
// the repo root. It skips (not fails) the calling test when no local
// PostgreSQL is reachable, so `go test ./...` stays usable on a machine
// that hasn't set one up — the machine that has gets full coverage.
func OpenTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := TestDSN()

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		if requireTestDB() {
			t.Fatalf("REQUIRE_TEST_DB is set but no local PostgreSQL test database reachable: %v", err)
		}
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
	dsn := TestDSN() + "&search_path=platform"

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		if requireTestDB() {
			t.Fatalf("REQUIRE_TEST_DB is set but no local PostgreSQL test database reachable: %v", err)
		}
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
	// Amendment: the original three candidates topped out at 3 levels up,
	// which never reaches the repo root from a 4-levels-deep package (e.g.
	// internal/modules/tenant/auth) — those packages silently fell back to
	// the DB_USER/DB_PASSWORD defaults instead of .env.test's real values.
	// Extended to 5 levels; the deepest package in this repo is 4.
	candidates := []string{".env.test", "../../.env.test", "../../../.env.test", "../../../../.env.test", "../../../../../.env.test"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ".env.test"
}
