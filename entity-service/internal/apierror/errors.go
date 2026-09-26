// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Package apierror provides shared HTTP error types and response helpers
// used across all handlers.
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

// ValidationError signals a caller-side input problem that should be
// reported as HTTP 400. Use errors.As in handlers to distinguish it from
// infrastructure errors.
type ValidationError struct {
	Msg string
}

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError signals that the requested resource does not exist and should
// be reported as HTTP 404. The Msg is safe to log and return to the caller.
type NotFoundError struct {
	Msg string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string { return e.Msg }

// ServiceUnavailableError signals that a downstream dependency is temporarily
// unavailable and should be reported as HTTP 503. Log Msg server-side but
// return only a generic message to the caller.
type ServiceUnavailableError struct {
	Msg string
}

// Error implements the error interface.
func (e *ServiceUnavailableError) Error() string { return e.Msg }

// UnauthorizedError signals that the caller is not authenticated and should be
// reported as HTTP 401.
type UnauthorizedError struct {
	Msg string
}

// Error implements the error interface.
func (e *UnauthorizedError) Error() string { return e.Msg }

// ForbiddenError signals that the caller is authenticated but not permitted to
// access the resource and should be reported as HTTP 403.
type ForbiddenError struct {
	Msg string
}

// Error implements the error interface.
func (e *ForbiddenError) Error() string { return e.Msg }

// ConflictError signals that the request conflicts with the current state of
// the resource and should be reported as HTTP 409.
type ConflictError struct {
	Msg string
}

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.Msg }

// TooManyRequestsError signals that the caller is repeating an operation
// faster than its own cooldown allows and should be reported as HTTP 429. It
// is a caller-pacing decision this service makes deliberately (e.g. the
// invitation-resend cooldown), never a downstream rate limit passed through,
// so Msg is safe to return: it is meant to tell the caller how long to wait.
type TooManyRequestsError struct {
	Msg string
}

// Error implements the error interface.
func (e *TooManyRequestsError) Error() string { return e.Msg }

// DownstreamError signals that a downstream dependency rejected the request
// with a status this service does not map to a more specific code, but that it
// supplied a reason worth returning. It is reported as HTTP 500 — the status is
// unchanged from the generic case — with Msg as the message instead of the
// opaque "internal server error" literal.
//
// Msg must already be a caller-safe reason extracted from the downstream error
// envelope, never a raw response body: it is returned to the API caller.
type DownstreamError struct {
	Msg string
}

// Error implements the error interface.
func (e *DownstreamError) Error() string { return e.Msg }

// WriteJSON writes an ErrorResponse JSON body with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: status, Message: msg})
}
