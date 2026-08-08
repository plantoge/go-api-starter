package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correcthorsebattery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "correcthorsebattery") {
		t.Error("VerifyPassword() = false for the correct password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("VerifyPassword() = true for a wrong password")
	}
}

func TestValidatePasswordStrength_TooShort(t *testing.T) {
	if err := ValidatePasswordStrength("short1"); err == nil {
		t.Error("expected error for a password under 8 characters")
	}
}

func TestValidatePasswordStrength_CommonPassword(t *testing.T) {
	if err := ValidatePasswordStrength("password"); err == nil {
		t.Error("expected error for a common password")
	}
	if err := ValidatePasswordStrength("123456789"); err == nil {
		t.Error("expected error for a common password")
	}
}

func TestValidatePasswordStrength_Valid(t *testing.T) {
	if err := ValidatePasswordStrength("a-reasonably-unique-passphrase"); err != nil {
		t.Errorf("ValidatePasswordStrength() = %v, want nil", err)
	}
}
