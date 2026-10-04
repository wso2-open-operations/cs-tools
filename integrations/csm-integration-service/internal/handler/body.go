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
	"io"
	"net/http"
)

// bodyPolicy says whether an operation accepts a request with no body.
type bodyPolicy int

const (
	// bodyRequired rejects an empty body with 400. Every write, and every
	// search whose upstream contract needs a payload, uses this.
	bodyRequired bodyPolicy = iota
	// bodyOptional forwards an empty body as "no filters". Used only by the
	// read-only searches whose openapi.yaml requestBody says required: false.
	bodyOptional
)

// readJSONBody reads the request body under the shared 1 MiB cap and checks
// that it is well-formed JSON. On any failure it writes the error response and
// returns ok=false, so a handler's whole prologue is:
//
//	body, ok := readJSONBody(w, r, bodyRequired)
//	if !ok {
//		return
//	}
//
// Responses: 413 when the body exceeds the cap, 400 when it cannot be read,
// is empty under bodyRequired, or is not valid JSON. Under bodyOptional an
// empty body returns (nil, true).
func readJSONBody(w http.ResponseWriter, r *http.Request, policy bodyPolicy) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if len(body) == 0 && policy == bodyOptional {
		return nil, true
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}
