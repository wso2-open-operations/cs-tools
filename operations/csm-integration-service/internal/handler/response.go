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
	"regexp"

	"github.com/wso2-open-operations/cs-tools/operations/csm-integration-service/internal/apierror"
)

// uuidRe validates a path-id segment as a UUID before it is forwarded upstream.
var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent memory DoS.
const maxRequestBodyBytes = 1 << 20

// Error message constants matching apps/csm-portal/backend's error vocabulary.
const (
	ErrMsgUnauthorized = "You are not authorized to perform this action. Please try again."
	ErrMsgForbidden    = "Access to the requested resource is forbidden!"
	ErrMsgNotFound     = "The requested resource was not found!"
	ErrMsgBadRequest   = "Invalid request payload."
	ErrMsgTooLarge     = "Request body too large."
	ErrMsgInternal     = "An internal server error occurred. Please try again later."
	ErrMsgInvalidUUID  = "Invalid UUID format."
	errMsgReadBody     = "Failed to read request body."
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

// mapUpstreamError translates an upstream service error to an HTTP response.
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
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		case http.StatusConflict, http.StatusUnprocessableEntity:
			writeError(w, apiErr.StatusCode, apiErr.Body)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			writeError(w, http.StatusServiceUnavailable, fallbackMsg)
		default:
			writeError(w, http.StatusInternalServerError, fallbackMsg)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackMsg)
}
