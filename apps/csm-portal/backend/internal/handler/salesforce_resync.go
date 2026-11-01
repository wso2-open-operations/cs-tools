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
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entitySalesforceResyncClient abstracts the entity service calls used by
// SalesforceResyncHandler.
type entitySalesforceResyncClient interface {
	PreviewSalesforceResync(ctx context.Context, body []byte) ([]byte, error)
	ApplySalesforceResync(ctx context.Context, body []byte) ([]byte, error)
}

// SalesforceResyncHandler handles the admin-only Salesforce resync routes: a
// read-only comparison of one account's or project's Salesforce data with the
// database, and the overwrite of the components the admin chose. It is a thin
// pass-through: the entity service owns the comparison, the admin check and the
// per-target lock. PermAdmin on the route is the gate here; authorization is
// resolved from platform data, never from token claims.
//
// A resync is synchronous and can take about a minute, so it runs close to the
// shared limits (ENTITY_SERVICE_TIMEOUT for the upstream call and
// REST_WRITE_TIMEOUT for this response, both 60s by default); raise both
// together if resyncs start timing out.
type SalesforceResyncHandler struct {
	entity entitySalesforceResyncClient
}

// NewSalesforceResyncHandler creates a SalesforceResyncHandler backed by the
// given entity client.
func NewSalesforceResyncHandler(entity entitySalesforceResyncClient) *SalesforceResyncHandler {
	return &SalesforceResyncHandler{entity: entity}
}

// Preview handles POST /salesforce/resync/preview.
func (h *SalesforceResyncHandler) Preview(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, false, "Failed to compare the Salesforce data.")
}

// Apply handles POST /salesforce/resync/apply.
func (h *SalesforceResyncHandler) Apply(w http.ResponseWriter, r *http.Request) {
	h.forward(w, r, true, "Failed to apply the Salesforce data.")
}

// forward validates the request lightly and passes it to the entity service.
// The entity service validates the scope, the target and the component keys in
// full; this layer only rejects what could not possibly be valid.
func (h *SalesforceResyncHandler) forward(w http.ResponseWriter, r *http.Request, apply bool, fallbackMsg string) {
	w.Header().Set("Cache-Control", "no-store")

	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req struct {
		Scope      string   `json:"scope"`
		ID         string   `json:"id"`
		Components []string `json:"components"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Scope != "account" && req.Scope != "project" {
		writeError(w, http.StatusBadRequest, "scope must be account or project.")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "id is required.")
		return
	}
	if apply && len(req.Components) == 0 {
		writeError(w, http.StatusBadRequest, "components must list at least one component.")
		return
	}

	var (
		result []byte
		err    error
	)
	if apply {
		result, err = h.entity.ApplySalesforceResync(r.Context(), body)
	} else {
		result, err = h.entity.PreviewSalesforceResync(r.Context(), body)
	}
	if err != nil {
		// The request body is deliberately not logged: it carries record ids.
		slog.ErrorContext(r.Context(), "entity Salesforce resync failed", "userID", user.UserID, "apply", apply, "err", summarizeErr(err))
		// mapUpstreamError, not the Generic one: a 409 (a resync is already
		// running for the target) and the entity service's validation 400 carry
		// a reason the admin needs to read; 403 and 404 keep their fixed messages.
		mapUpstreamError(w, err, fallbackMsg)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
