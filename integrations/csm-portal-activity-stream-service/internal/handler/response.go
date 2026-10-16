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

package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
)

// Error message constants matching the customer-portal error vocabulary.
const (
	ErrMsgUnauthorized = "You are not authorized to perform this action. Please try again."
	ErrMsgForbidden    = "Access to the requested resource is forbidden!"
	ErrMsgNotFound     = "The requested resource was not found!"
	ErrMsgBadRequest   = "Invalid request payload."
	ErrMsgInternal     = "An internal server error occurred. Please try again later."
	ErrMsgInvalidUUID  = "Invalid UUID format."
	// ErrMsgTooManyStreams and ErrMsgStreamCapacity are the bodies of the
	// 429 / 503 the stream handler's connection limiter returns.
	ErrMsgTooManyStreams = "Too many open activity streams for this user. Close one and try again."
	ErrMsgStreamCapacity = "Live updates are at capacity right now. Please try again shortly."
	errMsgReadBody       = "Failed to read request body."
)

// errorBody is the JSON error payload format matching the customer-portal pattern.
type errorBody struct {
	Message string `json:"message"`
}

// writeError writes a JSON error response: {"message": "..."}.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorBody{Message: message})
}

// mapUpstreamErrorGeneric is mapUpstreamError's counterpart for every
// non-PATCH endpoint: 401/403/404 still translate to the fixed messages, but
// every other case — 400, 409, 422, 5xx, and unmapped statuses alike — falls
// back to fallbackMsg instead of echoing the upstream body to the caller.
// The full upstream reason (status + body) is still expected in the caller's
// own slog.ErrorContext(ctx, ..., "err", err) call — server-side logs are
// operator-facing, not caller-facing, so the detail this function withholds
// from the HTTP response is deliberately preserved there for debugging.
func mapUpstreamErrorGeneric(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		case http.StatusForbidden:
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
		case http.StatusNotFound:
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
		case http.StatusBadRequest:
			writeError(w, http.StatusBadRequest, fallbackMsg)
		case http.StatusConflict, http.StatusUnprocessableEntity:
			writeError(w, apiErr.StatusCode, fallbackMsg)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			writeError(w, http.StatusServiceUnavailable, fallbackMsg)
		default:
			writeError(w, http.StatusInternalServerError, fallbackMsg)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackMsg)
}
