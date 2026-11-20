// Package response writes the uniform JSON envelope for every endpoint.
package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/niteshs1/streakit-api/internal/apperror"
	"github.com/niteshs1/streakit-api/internal/reqctx"
)

// Success is the envelope for successful responses.
type Success struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"OK"`
	Data    any    `json:"data"`
}

// ErrorBody is the machine-readable part of a failure.
type ErrorBody struct {
	Code   apperror.Code         `json:"code" example:"validation_error"`
	Fields []apperror.FieldError `json:"fields,omitempty"`
}

// Failure is the envelope for error responses.
type Failure struct {
	Success bool      `json:"success" example:"false"`
	Message string    `json:"message" example:"Validation failed"`
	Error   ErrorBody `json:"error"`
}

// Page is the list payload placed inside Success.Data.
type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page" example:"1"`
	PageSize int `json:"page_size" example:"20"`
	Total    int `json:"total" example:"0"`
}

// NewPage builds a Page and guarantees Items serialises as [] not null.
func NewPage[T any](items []T, page, pageSize, total int) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Page: page, PageSize: pageSize, Total: total}
}

// OK writes 200 with data.
func OK(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, Success{Success: true, Message: message, Data: data})
}

// Created writes 201 with data.
func Created(c *gin.Context, message string, data any) {
	c.JSON(http.StatusCreated, Success{Success: true, Message: message, Data: data})
}

// Error maps err to its HTTP status and writes the failure envelope.
// Internal errors are logged with their cause; clients get a generic message.
func Error(c *gin.Context, err error) {
	appErr := apperror.From(err)
	status := appErr.Code.HTTPStatus()

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "request failed",
			"request_id", reqctx.RequestID(c.Request.Context()),
			"error", err,
		)
		_ = c.Error(err)
	}

	body := ErrorBody{Code: appErr.Code}
	if appErr.Code == apperror.CodeValidation {
		body.Fields = appErr.Fields
	}

	c.AbortWithStatusJSON(status, Failure{
		Success: false,
		Message: appErr.Message,
		Error:   body,
	})
}

// Unavailable writes 503 for readiness failures, keeping diagnostic data.
func Unavailable(c *gin.Context, message string, data any) {
	c.JSON(http.StatusServiceUnavailable, Success{Success: false, Message: message, Data: data})
}

// NoRoute handles unknown routes.
func NoRoute(c *gin.Context) {
	Error(c, apperror.NotFound("Route"))
}

// NoMethod handles a known path with an unsupported method.
func NoMethod(c *gin.Context) {
	Error(c, &apperror.Error{Code: apperror.CodeMethodNotAllowed, Message: "Method not allowed"})
}

// Recovery converts panics into a 500 envelope and logs them.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.ErrorContext(c.Request.Context(), "panic recovered",
			"request_id", reqctx.RequestID(c.Request.Context()),
			"panic", recovered,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, Failure{
			Success: false,
			Message: "Internal server error",
			Error:   ErrorBody{Code: apperror.CodeInternal},
		})
	})
}
