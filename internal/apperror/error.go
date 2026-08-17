package apperror

type Code string

const (
	CodeValidation             Code = "VALIDATION_ERROR"
	CodeNotFound               Code = "NOT_FOUND"
	CodeConflict               Code = "CONFLICT"
	CodeForbidden              Code = "FORBIDDEN"
	CodeUnauthorized           Code = "UNAUTHORIZED"
	CodeInternal               Code = "INTERNAL_ERROR"
	CodeTenantMigrationPending Code = "TENANT_MIGRATION_PENDING"
	CodeRateLimited            Code = "RATE_LIMITED"
)

// Error itu satu-satunya tipe error yang dikembalikan semua fungsi
// service. Lapisan HTTP (response.Error) yang tahu cara ngubahnya jadi
// status code dan body JSON, jadi handler nggak pernah perlu nulis
// c.Status(...) sendiri.
type Error struct {
	Code    Code
	Message string
	Details map[string][]string
	Status  int
	cause   error
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func NotFound(msg string) *Error {
	return &Error{Code: CodeNotFound, Message: msg, Status: 404}
}

func Conflict(msg string) *Error {
	return &Error{Code: CodeConflict, Message: msg, Status: 409}
}

func Forbidden(msg string) *Error {
	return &Error{Code: CodeForbidden, Message: msg, Status: 403}
}

func Unauthorized(msg string) *Error {
	return &Error{Code: CodeUnauthorized, Message: msg, Status: 401}
}

func RateLimited(msg string) *Error {
	return &Error{Code: CodeRateLimited, Message: msg, Status: 429}
}

func TenantMigrationPending() *Error {
	return &Error{
		Code:    CodeTenantMigrationPending,
		Message: "tenant ini punya migrasi yang belum diterapkan dan harus diselesaikan dulu",
		Status:  409,
	}
}

func Validation(details map[string][]string) *Error {
	return &Error{
		Code:    CodeValidation,
		Message: "data yang dikirim tidak valid",
		Details: details,
		Status:  422,
	}
}

func Internal(cause error) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: "terjadi kesalahan pada server",
		Status:  500,
		cause:   cause,
	}
}
