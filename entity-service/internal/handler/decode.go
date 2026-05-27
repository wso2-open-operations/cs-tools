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
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *apierror.ValidationError
	switch {
	case errors.As(err, &ve):
		apierror.WriteJSON(w, http.StatusBadRequest, ve.Error())
	case errors.Is(err, context.DeadlineExceeded):
		apierror.WriteJSON(w, http.StatusRequestTimeout, "request timeout")
	case errors.Is(err, context.Canceled):
		w.WriteHeader(499)
		log.Printf("request canceled: %s %s", r.Method, r.URL.Path)
	default:
		apierror.WriteJSON(w, http.StatusInternalServerError, "internal server error")
	}
}
