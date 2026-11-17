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

	"github.com/wso2-open-operations/cs-tools/operations/legacy-product-api/internal/apierror"
)

// ErrMsgInternal is the body of a failure with no more specific message.
const ErrMsgInternal = "An internal server error occurred. Please try again later."

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
