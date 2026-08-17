package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndVerifyAccessToken_TenantScope(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := tm.IssueAccessToken(userID, ScopeTenant, &tenantID)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := tm.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
	if claims.Scope != ScopeTenant {
		t.Errorf("Scope = %q, want %q", claims.Scope, ScopeTenant)
	}
	if claims.TenantID != tenantID.String() {
		t.Errorf("TenantID = %q, want %q", claims.TenantID, tenantID.String())
	}
}

func TestIssueAndVerifyAccessToken_PlatformScope_NoTenantID(t *testing.T) {
	tm := NewTokenManager("test-secret", 15*time.Minute)
	userID := uuid.New()

	token, err := tm.IssueAccessToken(userID, ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := tm.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Scope != ScopePlatform {
		t.Errorf("Scope = %q, want %q", claims.Scope, ScopePlatform)
	}
	if claims.TenantID != "" {
		t.Errorf("TenantID = %q, want empty for platform scope", claims.TenantID)
	}
}

func TestVerify_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	tm1 := NewTokenManager("secret-one", 15*time.Minute)
	tm2 := NewTokenManager("secret-two", 15*time.Minute)

	token, err := tm1.IssueAccessToken(uuid.New(), ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := tm2.Verify(token); err == nil {
		t.Error("Verify() accepted a token signed with a different secret")
	}
}

func TestVerify_RejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret", -1*time.Minute) // sudah kadaluarsa sejak awal
	token, err := tm.IssueAccessToken(uuid.New(), ScopePlatform, nil)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := tm.Verify(token); err == nil {
		t.Error("Verify() accepted an expired token")
	}
}
