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

// entityCaseEscalationAccessClient is the entity-service calls the admin
// "Escalation access" page needs.
type entityCaseEscalationAccessClient interface {
	ListCaseDeescalators(ctx context.Context) ([]byte, error)
	PutCaseDeescalator(ctx context.Context, userID string) ([]byte, error)
	DeleteCaseDeescalator(ctx context.Context, userID string) ([]byte, error)
}

// CaseEscalationAccessHandler proxies who may de-escalate any case, besides the
// leads of the case's own ABT team: listing the people an admin named, adding
// and removing them. The routes are PermAdmin, and entity-service decides again
// from the caller's token, so this checks only the shape of the request.
type CaseEscalationAccessHandler struct {
	entity entityCaseEscalationAccessClient
}

// NewCaseEscalationAccessHandler creates a CaseEscalationAccessHandler.
func NewCaseEscalationAccessHandler(entity entityCaseEscalationAccessClient) *CaseEscalationAccessHandler {
	return &CaseEscalationAccessHandler{entity: entity}
}

// ListDeescalators handles GET /case-escalation-access/deescalators.
func (h *CaseEscalationAccessHandler) ListDeescalators(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.ListCaseDeescalators(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListCaseDeescalators failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who may de-escalate.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PutDeescalator handles PUT /case-escalation-access/deescalators/{userId}.
func (h *CaseEscalationAccessHandler) PutDeescalator(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "PutCaseDeescalator", "Failed to let that person de-escalate. Only active internal staff can be given this right.", h.entity.PutCaseDeescalator)
}

// DeleteDeescalator handles DELETE /case-escalation-access/deescalators/{userId}.
func (h *CaseEscalationAccessHandler) DeleteDeescalator(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, "DeleteCaseDeescalator", "Failed to remove that person's right to de-escalate.", h.entity.DeleteCaseDeescalator)
}

// change checks the path, forwards it and answers 204. Errors are mapped to
// fixed messages, never entity-service's own text; the one refusal worth
// explaining (a person who is not active internal staff) is in the PUT fallback.
func (h *CaseEscalationAccessHandler) change(w http.ResponseWriter, r *http.Request, op, fallbackMsg string,
	call func(ctx context.Context, userID string) ([]byte, error)) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	target := r.PathValue("userId")
	if !uuidRe.MatchString(target) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if _, err := call(r.Context(), target); err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, fallbackMsg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
