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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	defaultCommentLimit = 20
	maxCommentLimit     = 100
)

// entityConversationClient abstracts the entity service conversation operations.
type entityConversationClient interface {
	SearchComments(ctx context.Context, body []byte) ([]byte, error)
}

// ConversationHandler handles HTTP requests for conversation operations.
type ConversationHandler struct {
	entity entityConversationClient
}

// NewConversationHandler creates a ConversationHandler backed by the given entity client.
func NewConversationHandler(entity entityConversationClient) *ConversationHandler {
	return &ConversationHandler{entity: entity}
}

// GetConversationMessages handles GET /conversations/{id}/messages.
// Builds a POST /comments/search payload with referenceType=conversation and forwards it.
func (h *ConversationHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
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

	limit := defaultCommentLimit
	offset := 0

	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxCommentLimit {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 100")
			return
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	payload, err := json.Marshal(map[string]any{
		"referenceId":   id,
		"referenceType": "conversation",
		"pagination": map[string]int{
			"limit":  limit,
			"offset": offset,
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.SearchComments(r.Context(), payload)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "conversationID", id, "err", err)
		mapUpstreamError(w, err, "Failed to retrieve conversation messages.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
