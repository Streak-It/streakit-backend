package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/response"
)

// ConfigureValidation makes gin report JSON/form field names in validation
// errors and rejects unknown JSON fields. Call once at startup.
func ConfigureValidation() {
	binding.EnableDecoderDisallowUnknownFields = true
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(f reflect.StructField) string {
			for _, tag := range []string{"json", "form"} {
				name, _, _ := strings.Cut(f.Tag.Get(tag), ",")
				if name != "" && name != "-" {
					return name
				}
			}
			return f.Name
		})
	}
}

// bindJSON binds and validates the body; on failure it writes the error
// response and returns false.
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.Error(c, bindError(err))
		return false
	}
	return true
}

func bindQuery(c *gin.Context, dst any) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		response.Error(c, bindError(err))
		return false
	}
	return true
}

// pathID parses the :id path parameter.
func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, apperror.Field("id", "must be a valid UUID"))
		return uuid.Nil, false
	}
	return id, true
}

func bindError(err error) error {
	var (
		verrs     validator.ValidationErrors
		typeErr   *json.UnmarshalTypeError
		syntaxErr *json.SyntaxError
		timeErr   *time.ParseError
		maxErr    *http.MaxBytesError
	)
	switch {
	case errors.As(err, &verrs):
		fields := make([]apperror.FieldError, len(verrs))
		for i, fe := range verrs {
			fields[i] = apperror.FieldError{Field: fieldPath(fe), Message: fieldMessage(fe)}
		}
		return apperror.Validation("Validation failed", fields...)
	case errors.As(err, &maxErr):
		return apperror.Validation("Request body is too large")
	case errors.Is(err, io.EOF):
		return apperror.Validation("Request body is required")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return apperror.Validation("Malformed JSON body")
	case errors.As(err, &typeErr):
		return apperror.Validation("Validation failed", apperror.FieldError{
			Field: typeErr.Field, Message: "must be of type " + typeErr.Type.String(),
		})
	case errors.As(err, &timeErr):
		return apperror.Validation("Validation failed", apperror.FieldError{
			Field: "", Message: "timestamps must be RFC 3339, e.g. 2026-12-20T19:30:00Z",
		})
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		name := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return apperror.Validation("Validation failed", apperror.FieldError{Field: name, Message: "is not allowed"})
	default:
		return apperror.Validation(err.Error())
	}
}

// fieldPath drops the root struct name and embedded struct names:
// "UserListQuery.PageQuery.page_size" → "page_size". Wire names are all
// lower-case, so any segment starting upper-case is a Go type name.
func fieldPath(fe validator.FieldError) string {
	parts := strings.Split(fe.Namespace(), ".")
	out := parts[:0]
	for _, p := range parts[1:] {
		if p != "" && !unicode.IsUpper(rune(p[0])) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fe.Field()
	}
	return strings.Join(out, ".")
}

func fieldMessage(fe validator.FieldError) string {
	isString := fe.Kind() == reflect.String
	switch fe.Tag() {
	case "required":
		return "required"
	case "email":
		return "must be a valid email address"
	case "uuid":
		return "must be a valid UUID"
	case "url":
		return "must be a valid URL"
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	case "max":
		if isString {
			return fmt.Sprintf("must be at most %s characters", fe.Param())
		}
		if fe.Kind() == reflect.Slice {
			return fmt.Sprintf("must have at most %s items", fe.Param())
		}
		return "must be <= " + fe.Param()
	case "min":
		if isString {
			return fmt.Sprintf("must be at least %s characters", fe.Param())
		}
		return "must be >= " + fe.Param()
	default:
		return "is invalid"
	}
}
