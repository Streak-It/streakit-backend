// Package apperror defines typed application errors with stable codes.
// Every layer returns these; the response package maps them to HTTP.
package apperror

import (
	"errors"
	"net/http"
)

// Code is a stable, machine-readable error identifier sent to clients.
type Code string

const (
	CodeValidation           Code = "validation_error"
	CodeInvalidCredentials   Code = "invalid_credentials"
	CodeUnauthorized         Code = "unauthorized"
	CodeTokenExpired         Code = "token_expired"
	CodeForbidden            Code = "forbidden"
	CodeNotFound             Code = "not_found"
	CodeMethodNotAllowed     Code = "method_not_allowed"
	CodeConflict             Code = "conflict"
	CodeUnsupportedMediaType Code = "unsupported_media_type"
	CodeRateLimited          Code = "rate_limited"
	CodeInternal             Code = "internal_error"
)

var httpStatus = map[Code]int{
	CodeValidation:           http.StatusUnprocessableEntity,
	CodeInvalidCredentials:   http.StatusUnauthorized,
	CodeUnauthorized:         http.StatusUnauthorized,
	CodeTokenExpired:         http.StatusUnauthorized,
	CodeForbidden:            http.StatusForbidden,
	CodeNotFound:             http.StatusNotFound,
	CodeMethodNotAllowed:     http.StatusMethodNotAllowed,
	CodeConflict:             http.StatusConflict,
	CodeUnsupportedMediaType: http.StatusUnsupportedMediaType,
	CodeRateLimited:          http.StatusTooManyRequests,
	CodeInternal:             http.StatusInternalServerError,
}

// HTTPStatus returns the HTTP status for a code (500 for unknown codes).
func (c Code) HTTPStatus() int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field" example:"email"`
	Message string `json:"message" example:"required"`
}

// Error is the single application error type.
type Error struct {
	Code    Code
	Message string
	Fields  []FieldError
	// Err is the underlying cause; logged, never sent to clients.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func Validation(message string, fields ...FieldError) *Error {
	if message == "" {
		message = "Validation failed"
	}
	return &Error{Code: CodeValidation, Message: message, Fields: fields}
}

// Field is a shorthand for a single-field validation error.
func Field(field, message string) *Error {
	return Validation("Validation failed", FieldError{Field: field, Message: message})
}

// InvalidCredentials always uses the same message to avoid account enumeration.
func InvalidCredentials() *Error {
	return &Error{Code: CodeInvalidCredentials, Message: "Invalid email or password"}
}

func Unauthorized(message string) *Error {
	if message == "" {
		message = "Authentication required"
	}
	return &Error{Code: CodeUnauthorized, Message: message}
}

func TokenExpired() *Error {
	return &Error{Code: CodeTokenExpired, Message: "Access token expired"}
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	return &Error{Code: CodeForbidden, Message: message}
}

func NotFound(entity string) *Error {
	return &Error{Code: CodeNotFound, Message: entity + " not found"}
}

func Conflict(message string) *Error {
	return &Error{Code: CodeConflict, Message: message}
}

func UnsupportedMediaType() *Error {
	return &Error{Code: CodeUnsupportedMediaType, Message: "Content-Type must be application/json"}
}

func RateLimited() *Error {
	return &Error{Code: CodeRateLimited, Message: "Too many attempts, please try again later"}
}

// Internal wraps an unexpected error. The cause is logged, not exposed.
func Internal(err error) *Error {
	return &Error{Code: CodeInternal, Message: "Internal server error", Err: err}
}

// From converts any error into an *Error; unknown errors become internal.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}

// HasCode reports whether err is an *Error with the given code.
func HasCode(err error, code Code) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}
