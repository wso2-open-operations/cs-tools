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
)

// entityUserMeClient is the subset of internal/entity.CustomerEntityClient
// this file needs.
type entityUserMeClient interface {
	GetUserMe(ctx context.Context) ([]byte, error)
}

// UserInfoView is the portal response for GET /user-info.
type UserInfoView struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// UserInfoHandler handles HTTP requests for the caller's own name,
// delegating to entity-service — the same GET /users/me call UsersHandler.GetMe
// already makes, since first/last name already live on entity-service's own
// user table (no separate employee-info lookup needed for them).
type UserInfoHandler struct {
	entity      entityUserMeClient
	accessGuard *AccessGuard
}

// NewUserInfoHandler creates a UserInfoHandler backed by the given
// entity client. accessGuard enforces PermViewerAccess, SupportPortalLite's
// blanket audience gate.
func NewUserInfoHandler(entity entityUserMeClient, accessGuard *AccessGuard) *UserInfoHandler {
	return &UserInfoHandler{entity: entity, accessGuard: accessGuard}
}

// GetUserInfo handles GET /user-info: returns the caller's own first/last
// name, resolved from entity-service.
func (h *UserInfoHandler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	raw, err := h.entity.GetUserMe(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve user info.")
		return
	}

	var resp entityUserMeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		slog.ErrorContext(r.Context(), "entity GetUserMe: parse response failed", "userID", user.UserID, "err", summarizeErr(err))
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	writeJSONValue(w, http.StatusOK, UserInfoView{
		FirstName: derefStr(resp.FirstName),
		LastName:  resp.LastName,
	})
}
