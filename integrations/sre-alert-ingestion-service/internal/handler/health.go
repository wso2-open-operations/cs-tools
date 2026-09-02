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
	"net/http"
)

// healthPinger is the subset of internal/store.Store the health handler
// depends on.
type healthPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles GET /health: liveness/readiness backed by whether
// the buffer database is reachable — matching csm-integration-service's
// convention of a simple, dependency-checking health endpoint, with the one
// dependency this service actually has (the buffer database; Twilio and
// csm-integration-service are both expected to be unreachable at times by
// design and are not health-gating).
type HealthHandler struct {
	store healthPinger
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(store healthPinger) *HealthHandler {
	return &HealthHandler{store: store}
}

// Health handles GET /health.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Database unreachable.")
		return
	}
	w.WriteHeader(http.StatusOK)
}
