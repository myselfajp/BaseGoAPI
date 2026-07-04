// Package apperror provides a typed application error that carries an HTTP
// status code and a client-facing message. It is the Go equivalent of
// FastAPI's HTTPException: services raise it, and the HTTP layer translates it
// into a JSON response.
package apperror

import "net/http"

// AppError represents a handled error with an associated HTTP status code.
type AppError struct {
	Status  int
	Message string
}

// Error implements the error interface.
func (e *AppError) Error() string { return e.Message }

// New builds an AppError with an explicit status code.
func New(status int, message string) *AppError {
	return &AppError{Status: status, Message: message}
}

// Convenience constructors for the status codes used across the application.

func BadRequest(message string) *AppError   { return New(http.StatusBadRequest, message) }
func Unauthorized(message string) *AppError { return New(http.StatusUnauthorized, message) }
func Forbidden(message string) *AppError    { return New(http.StatusForbidden, message) }
func NotFound(message string) *AppError     { return New(http.StatusNotFound, message) }
func Conflict(message string) *AppError     { return New(http.StatusConflict, message) }
func Internal(message string) *AppError     { return New(http.StatusInternalServerError, message) }
