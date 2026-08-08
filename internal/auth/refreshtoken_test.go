package auth

import "testing"

func TestGenerateRefreshToken_UniqueAndHashMatches(t *testing.T) {
	plain1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	plain2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if plain1 == plain2 {
		t.Error("two calls returned the same plain token")
	}
	if hash1 == hash2 {
		t.Error("two calls returned the same hash")
	}
	if HashRefreshToken(plain1) != hash1 {
		t.Error("HashRefreshToken(plain1) does not match the hash returned alongside it")
	}
}
