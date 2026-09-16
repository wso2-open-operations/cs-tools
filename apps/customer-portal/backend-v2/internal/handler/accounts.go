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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityAccountClient abstracts the entity-service account operations used by AccountHandler.
type entityAccountClient interface {
	GetAccount(ctx context.Context, id string) (entity.AccountDetail, error)
}

// AccountHandler handles HTTP requests for account operations.
type AccountHandler struct {
	entity entityAccountClient
}

// NewAccountHandler creates an AccountHandler backed by the given entity client.
func NewAccountHandler(entity entityAccountClient) *AccountHandler {
	return &AccountHandler{entity: entity}
}

// GetAccount handles GET /accounts/{id}.
func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetAccount(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAccount failed", "userID", user.UserID, "accountID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve account.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapAccountDetails(result))
}
