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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// sanitizeLog strips CR/LF characters from a string to prevent log injection.
// Apply to every user-derived operand before passing it to a log call.
var sanitizeLog = strings.NewReplacer("\n", `\n`, "\r", `\r`).Replace

// decodeRequest decodes a JSON request body into dst, enforcing unknown-field
// rejection, a 1 MiB body cap, and no trailing data after the JSON object.
// Returns false and writes the error response if decoding fails.
func decodeRequest[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, decodeErrMsg(err))
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		apierror.WriteJSON(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return false
	}
	return true
}

// decodeErrMsg converts a JSON decode error into a human-readable message that
// is safe to return to the caller. Infrastructure details (e.g. raw Go type
// names) are replaced with user-friendly descriptions.
func decodeErrMsg(err error) string {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return "request body too large"
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("request body contains malformed JSON at position %d: %s", syntaxErr.Offset, syntaxErr.Error())
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		typeName := typeErr.Type.String()
		// Strip package prefix from named types (e.g. "domain.CaseResolutionCode" → "string").
		if typeErr.Type.Kind() == reflect.String {
			typeName = "string"
		} else if typeErr.Type.Kind() == reflect.Int || typeErr.Type.Kind() == reflect.Int64 {
			typeName = "integer"
		}
		return fmt.Sprintf("invalid value for field %q: expected %s", typeErr.Field, typeName)
	}
	// DisallowUnknownFields produces "json: unknown field "<name>"".
	msg := err.Error()
	if strings.HasPrefix(msg, "json: unknown field ") {
		field := strings.TrimPrefix(msg, "json: unknown field ")
		return fmt.Sprintf("unknown field %s in request body", field)
	}
	return "invalid request body"
}

// writeServiceError maps a service-layer error to the appropriate HTTP response.
// Full error details are logged server-side for debugging; the client always
// receives a safe, generic message — never raw infrastructure details.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve  *apierror.ValidationError
		nfe *apierror.NotFoundError
		sue *apierror.ServiceUnavailableError
		ue  *apierror.UnauthorizedError
		fe  *apierror.ForbiddenError
	)
	switch {
	case errors.As(err, &ve):
		// 400 – caller-supplied input is invalid; the message is safe to return.
		log.Printf("Bad request: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(ve.Msg)) // #nosec G706 -- path and message sanitized
		apierror.WriteJSON(w, http.StatusBadRequest, ve.Msg)

	case errors.As(err, &ue):
		// 401 – caller is not authenticated.
		log.Printf("Unauthorized: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(ue.Msg)) // #nosec G706 -- path and message sanitized
		apierror.WriteJSON(w, http.StatusUnauthorized, ue.Msg)

	case errors.As(err, &fe):
		// 403 – caller is authenticated but not permitted.
		log.Printf("Forbidden: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(fe.Msg)) // #nosec G706 -- path and message sanitized
		apierror.WriteJSON(w, http.StatusForbidden, fe.Msg)

	case errors.As(err, &nfe):
		// 404 – resource not found; message is safe to return.
		log.Printf("Not found: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(nfe.Msg)) // #nosec G706 -- path and message sanitized
		apierror.WriteJSON(w, http.StatusNotFound, nfe.Msg)

	case errors.As(err, &sue):
		// 503 – downstream dependency unavailable; log details, return generic message.
		log.Printf("Service unavailable: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(sue.Msg)) // #nosec G706 -- path and message sanitized
		apierror.WriteJSON(w, http.StatusServiceUnavailable, "service temporarily unavailable, please try again later")

	case errors.Is(err, context.DeadlineExceeded):
		// 408 – request timed out waiting for a downstream call or DB query.
		log.Printf("Request timeout: %s %s", r.Method, sanitizeLog(r.URL.Path)) // #nosec G706 -- path sanitized
		apierror.WriteJSON(w, http.StatusRequestTimeout, "request timed out")

	case errors.Is(err, context.Canceled):
		// Client closed the connection — log it and return nothing; the response is already gone.
		log.Printf("Request canceled: %s %s", r.Method, sanitizeLog(r.URL.Path)) // #nosec G706 -- path sanitized

	default:
		// 500 – unexpected infrastructure error (DB, network, etc.).
		// Log the full error for debugging; the client only receives the generic message.
		log.Printf("Internal error: %s %s: %s", r.Method, sanitizeLog(r.URL.Path), sanitizeLog(err.Error())) // #nosec G706 -- path and error sanitized
		apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
	}
}
