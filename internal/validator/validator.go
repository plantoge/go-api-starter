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
	// Pakai nama field JSON (misalnya "email"), bukan nama field struct Go
	// (misalnya "Email"), biar pemetaan error di sisi klien pas.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// Validate jalanin validasi berdasarkan tag struct pada s. Balikannya nil
// kalau s valid; kalau nggak, *apperror.Error dengan Details yang dikunci
// pakai nama field JSON, biar frontend bisa naruh tiap pesan di bawah
// input yang tepat.
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
