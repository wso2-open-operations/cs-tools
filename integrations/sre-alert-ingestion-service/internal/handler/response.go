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
	"net/http"
)

// maxRequestBodyBytes caps incoming request bodies at 1 MiB to prevent
// memory DoS — matches csm-integration-service's own convention.
const maxRequestBodyBytes = 1 << 20

// Error message constants, matching csm-integration-service's error
// vocabulary where the same situation applies.
const (
	ErrMsgBadRequest = "Invalid request payload."
	ErrMsgTooLarge   = "Request body too large."
	ErrMsgInternal   = "An internal server error occurred. Please try again later."
	errMsgReadBody   = "Failed to read request body."
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

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}
