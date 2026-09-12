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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// Error message constants matching the customer-portal error vocabulary.
const (
	ErrMsgUnauthorized           = "You are not authorized to perform this action. Please try again."
	ErrMsgForbidden              = "Access to the requested resource is forbidden!"
	ErrMsgNotFound               = "The requested resource was not found!"
	ErrMsgBadRequest             = "Invalid request payload."
	ErrMsgTooLarge               = "Request body too large."
	ErrMsgInternal               = "An internal server error occurred. Please try again later."
	ErrMsgInvalidTransition      = "Invalid state transition."
	ErrMsgWorkStateNotAllowed    = "Work state can only be updated when the case is in progress."
	ErrMsgCommentNotAllowed      = "Comments can only be added when the case is in progress and the work state is ongoing."
	ErrMsgCommentNotOwnCase      = "Only the assigned engineer can add a public comment on this case."
	ErrMsgWorkNoteOnClosedCase   = "Work notes cannot be added to a closed case."
	ErrMsgCommentOnClosedCase    = "Comments cannot be added to a closed case."
	ErrMsgAttachmentOnClosedCase = "Attachments cannot be added to a closed case."
	ErrMsgAttachmentNotShareable = "This attachment is not available for direct download."
	// ErrMsgAttachmentStorageUnsupportedRef is returned by the direct-upload
	// (SFTPGo-backed) mint endpoint for a reference type whose attachments
	// cannot be persisted through that storage mode yet — the caller should
	// use the standard attachment upload instead.
	ErrMsgAttachmentStorageUnsupportedRef = "Direct-upload attachment storage is not available for this item type yet. Use the standard attachment upload instead."
	ErrMsgRequestUpdateNotAllowed         = "An update can only be requested while the case is awaiting info or has a proposed solution."
	ErrMsgInvalidUUID                     = "Invalid UUID format."
	errMsgReadBody                        = "Failed to read request body."
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

// writeJSON writes a raw JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data) // #nosec G705 -- Content-Type: application/json already set; SecurityHeaders middleware adds X-Content-Type-Options: nosniff
}

// writeJSONValue marshals v and writes the result as a JSON response.
func writeJSONValue(w http.ResponseWriter, statusCode int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, statusCode, data)
}

// mapUpstreamError translates an upstream service error to an HTTP response,
// mirroring the Ballerina getStatusCode pattern in the customer-portal. Use
// this only for PATCH/update endpoints: their 4xx failures are reliably a
// rejection of something in the payload the caller just sent (e.g. "Invalid
// state transition"), so the upstream reason is worth showing. Every other
// endpoint should use mapUpstreamErrorGeneric instead — most of them forward
// a request that's only partially validated at this layer, so a 4xx from
// upstream isn't necessarily something the caller could have avoided, and
// echoing it would just leak upstream/internal implementation detail (e.g. a
// JSON decoder's field names).
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
			writeError(w, http.StatusBadRequest, upstreamErrorMessageStrict(apiErr.Body, ErrMsgBadRequest))
		case http.StatusConflict, http.StatusUnprocessableEntity:
			writeError(w, apiErr.StatusCode, upstreamErrorMessage(apiErr.Body, fallbackMsg))
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			writeError(w, http.StatusServiceUnavailable, fallbackMsg)
		default:
			// The 4xx branches above surface the upstream reason because those
			// failures are caller-actionable: the caller can correct the request.
			// This branch is not. It catches 500 and every unenumerated status,
			// where the failure is internal to an upstream service and there is
			// nothing for the caller to act on. Its body is also not guaranteed
			// to be ours: this function is the error path for several clients,
			// including the identity provider's, and a body being a JSON object
			// with a message field proves its shape, not that its content is safe
			// to show a portal client. So return the generic fallback only; the
			// upstream detail is logged, not echoed.
			writeError(w, http.StatusInternalServerError, fallbackMsg)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackMsg)
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

// upstreamErrorMessage extracts the human-readable message from an upstream
// JSON error body shaped like {"message": "..."} (the entity service's error
// envelope). Passing the raw JSON body straight through as the outer
// {"message": ...} value would double-encode it into an escaped JSON string
// instead of the plain text callers expect, so it is parsed here first. Falls
// back to the raw body when it isn't a JSON object with a message field, and
// to fallbackMsg when the body is empty.
func upstreamErrorMessage(body string, fallbackMsg string) string {
	if body == "" {
		return fallbackMsg
	}
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	return body
}

// upstreamErrorMessageStrict is like upstreamErrorMessage but never falls back
// to the raw body: it only ever surfaces a value from body when it parses as
// a JSON object with a non-empty message field, and returns fallbackMsg for
// every other shape (empty, malformed JSON, or JSON without a message field).
// Used for status codes where the upstream body is not guaranteed to be a
// well-formed error envelope, so echoing it verbatim risks leaking upstream
// diagnostic text (or worse) straight to the API caller.
func upstreamErrorMessageStrict(body string, fallbackMsg string) string {
	if body == "" {
		return fallbackMsg
	}
	var parsed struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err == nil && parsed.Message != "" {
		return parsed.Message
	}
	return fallbackMsg
}

// summarizeErr returns a short, log-safe description of err: the upstream status
// code for a typed *apierror.Error (never its Body, which may carry upstream
// response data not meant for logs), or a fixed generic message otherwise — an
// unrecognized error can come from the underlying HTTP client (e.g. a
// net/url.Error), which stringifies with the full request URL including query
// parameters, so its raw text is not safe to log verbatim.
func summarizeErr(err error) string {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("upstream status %d", apiErr.StatusCode)
	}
	return "upstream request failed"
}
