package validator

import (
	"fmt"
	"reflect"
	"strings"

	pv "github.com/go-playground/validator/v10"
	"go-api-starter/internal/apperror"
)

var instance = newInstance()

func newInstance() *pv.Validate {
	v := pv.New()
	// Report the JSON field name (e.g. "email") instead of the Go struct
	// field name (e.g. "Email") so client-side error mapping is exact.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// Validate runs struct tag validation on s. It returns nil when s is valid,
// otherwise an *apperror.Error with Details keyed by JSON field name so the
// frontend can place each message under the right input.
func Validate(s any) *apperror.Error {
	err := instance.Struct(s)
	if err == nil {
		return nil
	}

	fieldErrs, ok := err.(pv.ValidationErrors)
	if !ok {
		return apperror.Validation(map[string][]string{"_": {err.Error()}})
	}

	details := make(map[string][]string)
	for _, fe := range fieldErrs {
		details[fe.Field()] = append(details[fe.Field()], message(fe))
	}
	return apperror.Validation(details)
}

func message(fe pv.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		return fmt.Sprintf("minimal %s karakter", fe.Param())
	case "max":
		return fmt.Sprintf("maksimal %s karakter", fe.Param())
	default:
		return fmt.Sprintf("tidak valid (%s)", fe.Tag())
	}
}
