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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entitySLAEngineClient is the entity-service calls the SLA engine admin
// action needs.
type entitySLAEngineClient interface {
	SearchActiveSLAStatusesManage(ctx context.Context, limit, offset, source string) ([]byte, error)
	RequestSLARecalculation(ctx context.Context) ([]byte, error)
}

// SLAEngineHandler proxies the CSM portal's "Recalculate SLA" admin action:
// a trigger that asks integrations/csm-notification-service to rebuild its
// own Redis-held SLA tracking from entity-service's durable record, and a
// read of the currently-active clocks to show after it. Both routes are
// PermAdmin at the route level (cmd/server/main.go); entity-service
// re-checks AUTH_ADMIN_ROLES itself (see that repo's own CLAUDE.md) -- this
// backend's own gate narrows who can even reach entity-service, it does not
// replace that check.
type SLAEngineHandler struct {
	entity entitySLAEngineClient
}

// NewSLAEngineHandler creates an SLAEngineHandler.
func NewSLAEngineHandler(entity entitySLAEngineClient) *SLAEngineHandler {
	return &SLAEngineHandler{entity: entity}
}

// SearchActiveSLAStatusesManage handles GET /sla-status/manage -- what the
// admin page polls to show which clocks are currently running, typically
// right after RequestRecalculation.
func (h *SLAEngineHandler) SearchActiveSLAStatusesManage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	q := r.URL.Query()
	result, err := h.entity.SearchActiveSLAStatusesManage(r.Context(), q.Get("limit"), q.Get("offset"), q.Get("source"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchActiveSLAStatusesManage failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load SLA clocks.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RequestRecalculation handles POST /sla-engine/recalculate. No request
// body. A 204 means the request was published, not that the rebuild on
// csm-notification-service's side has finished -- the webapp should poll
// SearchActiveSLAStatusesManage afterward to show progress.
func (h *SLAEngineHandler) RequestRecalculation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	if _, err := h.entity.RequestSLARecalculation(r.Context()); err != nil {
		slog.ErrorContext(r.Context(), "entity RequestSLARecalculation failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to start SLA recalculation.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
