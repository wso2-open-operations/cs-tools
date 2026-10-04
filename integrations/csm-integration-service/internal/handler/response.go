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
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/apierror"
)

// uuidRe validates a path-id segment as a UUID before it is forwarded upstream.
var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// Error message constants matching apps/csm-portal/backend's error vocabulary.
const (
	ErrMsgUnauthorized    = "You are not authorized to perform this action. Please try again."
	ErrMsgForbidden       = "Access to the requested resource is forbidden!"
	ErrMsgNotFound        = "The requested resource was not found!"
	ErrMsgBadRequest      = "Invalid request payload."
	ErrMsgTooLarge        = "Request body too large."
	ErrMsgInternal        = "An internal server error occurred. Please try again later."
	ErrMsgInvalidUUID     = "Invalid UUID format."
	errMsgReadBody        = "Failed to read request body."
	ErrMsgContentRequired = "The 'content' field is required."
	ErrMsgLabelRequired   = "The 'label' field is required."
	ErrMsgRateLimited     = "Too many requests. Please retry later."
	ErrMsgSyncNotArray    = "The request body must be a JSON array of product-vulnerability records."
	ErrMsgSyncEmpty       = "At least one product-vulnerability record is required; an empty set is not accepted."
)

// errorBody is the JSON error payload format.
type errorBody struct {
	Message string `json:"message"`
}

// writeError writes a JSON error response: {"message": "..."}.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorBody{Message: message})
}

// writeJSON writes a raw JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data) // #nosec G705 -- Content-Type: application/json already set; SecurityHeaders middleware adds X-Content-Type-Options: nosniff
}

// mapUpstreamError translates an upstream service error to an HTTP response:
//   - 401/403/404: same status, fixed message
//   - 400/409/422: same status; the upstream's own message when it is safe to
//     show a caller (see safeUpstreamMessage), else a fixed fallback
//   - 429: 429, with the upstream's Retry-After passed through when well-formed
//   - 408/504: 504 (the upstream did not answer in time)
//   - 502/503: 503 (the upstream is unavailable)
//   - anything else, including transport failures: 500
func mapUpstreamError(w http.ResponseWriter, err error, fallbackMsg string) {
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
			writeError(w, http.StatusBadRequest, safeUpstreamMessage(apiErr.Body, ErrMsgBadRequest))
		case http.StatusConflict, http.StatusUnprocessableEntity:
			writeError(w, apiErr.StatusCode, safeUpstreamMessage(apiErr.Body, fallbackMsg))
		case http.StatusTooManyRequests:
			if ra := strings.TrimSpace(apiErr.RetryAfter); validRetryAfter(ra) {
				w.Header().Set("Retry-After", ra)
			}
			writeError(w, http.StatusTooManyRequests, ErrMsgRateLimited)
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			writeError(w, http.StatusGatewayTimeout, fallbackMsg)
		case http.StatusBadGateway, http.StatusServiceUnavailable:
			writeError(w, http.StatusServiceUnavailable, fallbackMsg)
		default:
			writeError(w, http.StatusInternalServerError, fallbackMsg)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackMsg)
}

// maxRetryAfterDigits bounds a delta-seconds Retry-After value; anything longer
// is not a plausible retry hint and is dropped rather than forwarded.
const maxRetryAfterDigits = 8

// validRetryAfter reports whether v is a well-formed Retry-After value: either
// delta-seconds (digits only) or an HTTP-date. Anything else is not forwarded.
func validRetryAfter(v string) bool {
	if v == "" || len(v) > 64 || strings.ContainsAny(v, "\r\n") {
		return false
	}
	if len(v) <= maxRetryAfterDigits && strings.Trim(v, "0123456789") == "" {
		return true
	}
	_, err := http.ParseTime(v)
	return err == nil
}

// maxPassThroughMsgLen caps a passed-through upstream error message.
const maxPassThroughMsgLen = 1024

// internalDetailRe matches database or driver detail that must never reach a caller.
var internalDetailRe = regexp.MustCompile(`(?i)key \(|table "|violates|constraint|sqlstate|\bpgx?\b|\bpq:|\bsql\b|character varying|invalid input syntax`)

// safeUpstreamMessage returns the message from an upstream {"message": ...}
// error body when it is written for clients, or fallback when it is missing,
// oversized, multi-line, or looks like internal database or driver detail.
func safeUpstreamMessage(body, fallback string) string {
	var env errorBody
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		return fallback
	}
	msg := strings.TrimSpace(env.Message)
	if msg == "" || len(msg) > maxPassThroughMsgLen || strings.ContainsAny(msg, "\r\n\t") || internalDetailRe.MatchString(msg) {
		return fallback
	}
	return msg
}

// summarizeErr returns a short, log-safe description of err: the upstream status
// code for a typed *apierror.Error (never its Body, which may carry upstream
// response data not meant for logs), or a bounded prefix of err.Error() otherwise.
func summarizeErr(err error) string {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("upstream status %d", apiErr.StatusCode)
	}
	const maxLen = 200
	msg := err.Error()
	if len(msg) > maxLen {
		msg = msg[:maxLen]
	}
	return msg
}
