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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// Package handler serves the public Knowledge Base API: read-only access to
// published articles and active knowledge bases. It is unauthenticated by design,
// so every handler treats its input as hostile: ids are validated before they
// reach an upstream URL, the article state filter is forced server-side, and
// responses are built from whitelist DTOs rather than relayed upstream JSON.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/apierror"
)

// Error messages returned to callers. Upstream bodies and statuses are never
// relayed; these are the only strings a client can see.
const (
	ErrMsgNotFound      = "The requested resource was not found."
	ErrMsgBadRequest    = "Invalid request payload."
	ErrMsgTooLarge      = "Request body too large."
	ErrMsgInvalidID     = "Invalid identifier: expected a UUID."
	ErrMsgInternal      = "An internal server error occurred. Please try again later."
	ErrMsgUnavailable   = "The knowledge base is temporarily unavailable."
	ErrMsgUpstreamSlow  = "The knowledge base took too long to respond."
	errMsgUnsupportedBy = "Unsupported sortBy value."
)

// errorBody is the JSON error payload, the {"message": "..."} envelope
// every route answers with.
type errorBody struct {
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorBody{Message: message})
}

// writeJSONValue marshals v and writes the result as a JSON response.
func writeJSONValue(w http.ResponseWriter, statusCode int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

// Health handles GET /health, a pure liveness probe.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// NotFound is the catch-all for unregistered paths, so they get the JSON envelope.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, ErrMsgNotFound)
}

// writeUpstreamError maps an entity-service failure to a generic response. The
// upstream status and body are logged, never relayed. On a by-id lookup (byID)
// an upstream 404 (no such article) or 400 (id rejected as malformed) both
// become the portal's 404; ids are validated before the call, so a 400 here
// means the article cannot exist. Everything else is a 502, or 504 when the
// upstream call timed out.
func writeUpstreamError(w http.ResponseWriter, r *http.Request, op string, err error, byID bool) {
	var apiErr *apierror.Error
	switch {
	case errors.As(err, &apiErr):
		if byID && (apiErr.StatusCode == http.StatusNotFound || apiErr.StatusCode == http.StatusBadRequest) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "entity-service returned an error",
			"op", op, "upstreamStatus", apiErr.StatusCode, "upstreamBody", apiErr.Body)
		writeError(w, http.StatusBadGateway, ErrMsgUnavailable)
	case errors.Is(err, context.DeadlineExceeded):
		slog.ErrorContext(r.Context(), "entity-service call timed out", "op", op, "err", err)
		writeError(w, http.StatusGatewayTimeout, ErrMsgUpstreamSlow)
	default:
		slog.ErrorContext(r.Context(), "entity-service call failed", "op", op, "err", err)
		writeError(w, http.StatusBadGateway, ErrMsgUnavailable)
	}
}
