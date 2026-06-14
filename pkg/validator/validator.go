package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = func() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return v
}()

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func Validate(s any) []FieldError {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return nil
	}

	errs := make([]FieldError, len(ve))
	for i, e := range ve {
		errs[i] = FieldError{
			Field:   e.Field(),
			Message: message(e),
		}
	}
	return errs
}

func message(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email", e.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
	case "eqfield":
		return fmt.Sprintf("%s must match %s", e.Field(), e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", e.Field(), e.Param())
	case "gte":
		return fmt.Sprintf("%s must be at least %s", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s is invalid", e.Field())
	}
}
