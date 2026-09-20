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

// Package handler wires HTTP routes to the domain packages: taxonomy,
// issues, metrics, sync, titles, and health. Every non-2xx response uses
// internal/apierror's envelope, with one documented exception: GET /readyz
// returns a status document (ready/not_ready/draining plus per-check
// detail) on both 200 and 503 rather than collapsing into {"error":{...}}
// — folding it into the error envelope would throw away the per-check
// detail that makes the endpoint worth having. Every other 2xx, and
// /readyz's own non-2xx, uses writeJSON below.
package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON marshals v and writes it with the given HTTP status. Used for
// every 2xx response and, as the one documented exception (see the package
// comment), for GET /readyz's 503.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
