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
	"log/slog"
	"net/http"
)

// lookupsClient abstracts the backing-system lookup operations used by
// LookupsHandler.
type lookupsClient interface {
	GetProductList(ctx context.Context) ([]string, error)
	GetABTTeamList(ctx context.Context) ([]string, error)
}

// LookupsHandler handles HTTP requests for SupportPortalLite's small
// standalone dropdown lookups, delegating to the backing system service.
type LookupsHandler struct {
	servicenow  lookupsClient
	accessGuard *AccessGuard
}

// NewLookupsHandler creates a LookupsHandler backed by the given
// The backing system client. accessGuard enforces PermViewerAccess, SupportPortalLite's
// blanket audience gate.
func NewLookupsHandler(sn lookupsClient, accessGuard *AccessGuard) *LookupsHandler {
	return &LookupsHandler{servicenow: sn, accessGuard: accessGuard}
}

// GetProducts handles GET /products.
func (h *LookupsHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	products, err := h.servicenow.GetProductList(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetProductList failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve products.")
		return
	}

	writeJSONValue(w, http.StatusOK, products)
}

// GetABTTeams handles GET /abt-teams.
func (h *LookupsHandler) GetABTTeams(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	teams, err := h.servicenow.GetABTTeamList(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "servicenow GetABTTeamList failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve ABT teams.")
		return
	}

	writeJSONValue(w, http.StatusOK, teams)
}
