package database_test

import (
	"testing"

	"go-api-starter/internal/database"
)

// TestValidSchemaName nguji ValidSchemaName secara langsung. Fungsi itu
// satu-satunya penghalang antara TenantInfo.SchemaName yang nilainya
// dikendalikan pemanggil dan penempelan mentah ke SQL di statement SET
// LOCAL milik WithTenant — jadi tiap kasus penolakan di sini penting.
// Fungsinya murni, nggak butuh koneksi database, jadi test ini selalu
// jalan dan nggak pernah di-skip.
func TestValidSchemaName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "tenant_a", true},
		{"valid with digits", "tenant_123", true},
		{"valid minimum length (3 chars)", "abc", true},
		{"valid maximum length (50 chars)", "a" + repeat("b", 49), true},

		{"rejects embedded double quote", `tenant"a`, false},
		{"rejects semicolon", "tenant;drop", false},
		{"rejects trailing-newline injection payload", "tenant_a\nDROP SCHEMA x", false},
		{"rejects bare trailing newline (RE2 $ = end of text, not before \\n)", "tenant_a\n", false},
		{"rejects uppercase", "Tenant_A", false},
		{"rejects leading digit", "1tenant", false},
		{"rejects leading underscore", "_tenant", false},
		{"rejects empty string", "", false},
		{"rejects over 50 chars", "a" + repeat("b", 50), false},
		{"rejects dotted name", "a.b", false},
		{"rejects embedded space", "tenant a", false},
		{"rejects embedded dash", "tenant-a", false},
		{"rejects too short (2 chars)", "ab", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := database.ValidSchemaName(tt.input)
			if got != tt.want {
				t.Errorf("ValidSchemaName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
