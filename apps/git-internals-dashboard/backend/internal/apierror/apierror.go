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

// Package apierror defines the uniform JSON error envelope every non-2xx
// response returns (SPEC §6) and the helpers that write it, so no handler
// hand-rolls its own error shape.
package apierror

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Machine-readable error codes used across the API (SPEC §6).
const (
	CodeValidationFailed = "validation_failed"
	CodeNotFound         = "not_found"
	CodeSyncInProgress   = "sync_in_progress"
	CodeSyncTokenMissing = "sync_token_missing"
	CodeInternal         = "internal"
)

// envelope is the wire shape: {"error": {"code": "...", "message": "..."}}.
type envelope struct {
	Error detail `json:"error"`
}

type detail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write sends a JSON error envelope with the given HTTP status, code, and
// human-readable message.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: detail{Code: code, Message: message}})
}

// ValidationFailed writes a 400 validation_failed error with the given message.
func ValidationFailed(w http.ResponseWriter, message string) {
	Write(w, http.StatusBadRequest, CodeValidationFailed, message)
}

// NotFound writes a 404 not_found error with the given message.
func NotFound(w http.ResponseWriter, message string) {
	Write(w, http.StatusNotFound, CodeNotFound, message)
}

// Internal logs err with the given context and writes a fixed 500 response
// that never leaks err's detail to the caller (csm-portal convention).
func Internal(w http.ResponseWriter, r *http.Request, msg string, err error) {
	slog.ErrorContext(r.Context(), msg, "err", err)
	Write(w, http.StatusInternalServerError, CodeInternal, "internal server error")
}
