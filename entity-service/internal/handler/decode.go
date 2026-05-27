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
	"io"
	"log"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// decodeRequest decodes a JSON request body into dst, enforcing unknown-field
// rejection, a 1 MiB body cap, and no trailing data after the JSON object.
// Returns false and writes the error response if decoding fails.
func decodeRequest[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		apierror.WriteJSON(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		apierror.WriteJSON(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// writeServiceError maps a service-layer error to the appropriate HTTP response.
// Full error details are logged server-side for debugging; the client always
// receives a safe, generic message — never raw infrastructure details.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ve  *apierror.ValidationError
		nfe *apierror.NotFoundError
		sue *apierror.ServiceUnavailableError
	)
	switch {
	case errors.As(err, &ve):
		// 400 – caller-supplied input is invalid; the message is safe to return.
		log.Printf("Bad request: %s %s: %s", r.Method, r.URL.Path, ve.Msg)
		apierror.WriteJSON(w, http.StatusBadRequest, ve.Msg)

	case errors.As(err, &nfe):
		// 404 – resource not found; message is safe to return.
		log.Printf("Not found: %s %s: %s", r.Method, r.URL.Path, nfe.Msg)
		apierror.WriteJSON(w, http.StatusNotFound, nfe.Msg)

	case errors.As(err, &sue):
		// 503 – downstream dependency unavailable; log details, return generic message.
		log.Printf("Service unavailable: %s %s: %s", r.Method, r.URL.Path, sue.Msg)
		apierror.WriteJSON(w, http.StatusServiceUnavailable, "service temporarily unavailable, please try again later")

	case errors.Is(err, context.DeadlineExceeded):
		// 408 – request timed out waiting for a downstream call or DB query.
		log.Printf("Request timeout: %s %s", r.Method, r.URL.Path)
		apierror.WriteJSON(w, http.StatusRequestTimeout, "request timed out")

	case errors.Is(err, context.Canceled):
		// Client closed the connection — log it and return nothing; the response is already gone.
		log.Printf("Request canceled: %s %s", r.Method, r.URL.Path)

	default:
		// 500 – unexpected infrastructure error (DB, network, etc.).
		// Log the full error for debugging; the client only receives the generic message.
		log.Printf("Internal error: %s %s: %v", r.Method, r.URL.Path, err)
		apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
	}
}
