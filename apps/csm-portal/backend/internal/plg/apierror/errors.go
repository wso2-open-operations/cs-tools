// Package apierror provides shared HTTP error types and response helpers used
// across all handlers. Mirrors cs-tools/entity-service/internal/apierror so the
// error contract stays identical when this service is folded into that stack.
package apierror

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the JSON body returned for all error responses.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ValidationError signals a caller-side input problem, reported as HTTP 400.
type ValidationError struct{ Msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError signals that the requested resource does not exist (HTTP 404).
type NotFoundError struct{ Msg string }

// Error implements the error interface.
func (e *NotFoundError) Error() string { return e.Msg }

// ForbiddenError signals that the caller is authenticated but not permitted to
// access the resource and should be reported as HTTP 403.
type ForbiddenError struct{ Msg string }

// Error implements the error interface.
func (e *ForbiddenError) Error() string { return e.Msg }

// ConflictError signals that the request conflicts with current state (HTTP 409).
type ConflictError struct{ Msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.Msg }

// UnauthorizedError signals a missing or invalid caller identity (HTTP 401).
type UnauthorizedError struct{ Msg string }

// Error implements the error interface.
func (e *UnauthorizedError) Error() string { return e.Msg }

// ServiceUnavailableError signals a downstream dependency is down (HTTP 503).
type ServiceUnavailableError struct{ Msg string }

// Error implements the error interface.
func (e *ServiceUnavailableError) Error() string { return e.Msg }

// Validation is a convenience constructor for *ValidationError.
func Validation(msg string) error { return &ValidationError{Msg: msg} }

// NotFound is a convenience constructor for *NotFoundError.
func NotFound(msg string) error { return &NotFoundError{Msg: msg} }

// Forbidden is a convenience constructor for *ForbiddenError.
func Forbidden(msg string) error { return &ForbiddenError{Msg: msg} }

// Conflict is a convenience constructor for *ConflictError.
func Conflict(msg string) error { return &ConflictError{Msg: msg} }

// Unauthorized is a convenience constructor for *UnauthorizedError.
func Unauthorized(msg string) error { return &UnauthorizedError{Msg: msg} }

// WriteJSON writes an ErrorResponse JSON body with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: status, Message: msg})
}
