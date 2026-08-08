package apperror

import (
	"errors"
	"testing"
)

func TestConstructors_StatusAndCode(t *testing.T) {
	cases := []struct {
		name       string
		err        *Error
		wantCode   Code
		wantStatus int
	}{
		{"NotFound", NotFound("missing"), CodeNotFound, 404},
		{"Conflict", Conflict("dup"), CodeConflict, 409},
		{"Forbidden", Forbidden("no"), CodeForbidden, 403},
		{"Unauthorized", Unauthorized("no token"), CodeUnauthorized, 401},
		{"RateLimited", RateLimited("slow down"), CodeRateLimited, 429},
		{"TenantMigrationPending", TenantMigrationPending(), CodeTenantMigrationPending, 409},
		{"Validation", Validation(map[string][]string{"email": {"required"}}), CodeValidation, 422},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.wantCode {
				t.Errorf("Code = %v, want %v", tc.err.Code, tc.wantCode)
			}
			if tc.err.Status != tc.wantStatus {
				t.Errorf("Status = %v, want %v", tc.err.Status, tc.wantStatus)
			}
		})
	}
}

func TestInternal_WrapsCause(t *testing.T) {
	cause := errors.New("db exploded")
	err := Internal(cause)

	if err.Status != 500 {
		t.Errorf("Status = %v, want 500", err.Status)
	}
	if !errors.Is(err, cause) {
		t.Error("Internal(cause) does not unwrap to cause")
	}
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	var _ error = NotFound("x")
}
