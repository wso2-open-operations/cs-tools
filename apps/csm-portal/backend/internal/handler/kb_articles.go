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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityKBArticleClient abstracts the entity service kb_article operations used by KBArticleHandler.
type entityKBArticleClient interface {
	CreateKBArticle(ctx context.Context, body []byte) ([]byte, error)
	GetKBArticle(ctx context.Context, id string) ([]byte, error)
	SearchKBArticles(ctx context.Context, body []byte) ([]byte, error)
	PatchKBArticleState(ctx context.Context, id string, body []byte) ([]byte, error)
	SearchKBManagers(ctx context.Context, body []byte) ([]byte, error)
	PatchKBArticleContent(ctx context.Context, id string, body []byte) ([]byte, error)
	ListKnowledgeBases(ctx context.Context) ([]byte, error)
	CreateKnowledgeBase(ctx context.Context, body []byte) ([]byte, error)
	UpdateKnowledgeBaseName(ctx context.Context, id string, body []byte) ([]byte, error)
	SetKnowledgeBaseActive(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateKBManager(ctx context.Context, body []byte) ([]byte, error)
	DeleteKBManager(ctx context.Context, body []byte) error
	GetUserMe(ctx context.Context) ([]byte, error)
	DeleteKBArticle(ctx context.Context, id string) error
	ListKBArticleHistory(ctx context.Context, id string) ([]byte, error)
}

// KBArticleHandler handles HTTP requests for KB article operations, delegating to the
// entity service for data access.
type KBArticleHandler struct {
	entity entityKBArticleClient
}

// NewKBArticleHandler creates a KBArticleHandler backed by the given entity client.
func NewKBArticleHandler(entity entityKBArticleClient) *KBArticleHandler {
	return &KBArticleHandler{entity: entity}
}

// CreateKBArticle handles POST /kb-articles.
// Forwards the request body directly to the entity service and returns 201 on success.
func (h *KBArticleHandler) CreateKBArticle(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateKBArticle(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateKBArticle failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to create KB article.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetKBArticle handles GET /kb-articles/{id}.
func (h *KBArticleHandler) GetKBArticle(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetKBArticle failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to fetch KB article.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchKBArticles handles POST /kb-articles/search.
func (h *KBArticleHandler) SearchKBArticles(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchKBArticles(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchKBArticles failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search KB articles.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// kbArticleSummary is the minimal shape read back from GetKBArticle, just
// enough to decide who is allowed to make the requested transition.
type kbArticleSummary struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	AuthorID        string `json:"authorId"`
	State           string `json:"state"`
}

// kbManagerSearchResult is the minimal shape read back from SearchKBManagers.
type kbManagerSearchResult struct {
	Managers []struct {
		UserID string `json:"userId"`
	} `json:"managers"`
}

// kbArticleStateDecisionPayload is decoded here only to validate the allowed
// action names before forwarding; the entity service performs the real
// state-machine validation via trg_kb_article_valid_transition.
type kbArticleStateDecisionPayload struct {
	State             string  `json:"state"`
	RejectionComment  *string `json:"rejectionComment,omitempty"`
}

// PatchKBArticleState handles PATCH /kb-articles/{id}/state.
// Accepts state: "pending_review" | "draft" | "published" | "retired".
//
// Enforces who may make each transition:
//
//	draft           -> pending_review : author only (submit)
//	pending_review  -> draft          : manager only (reject)
//	pending_review  -> published      : manager only (approve)
//	published       -> draft          : author only (edit)
//	published       -> retired        : manager only (retire)
//
// The entity service's DB trigger still independently rejects any
// structurally-illegal transition regardless of this check.
func (h *KBArticleHandler) PatchKBArticleState(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload kbArticleStateDecisionPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	switch payload.State {
	case "draft", "pending_review", "published", "retired":
		// valid
	default:
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Fetch the article to learn its current state, author, and KB --
	// required to know which role rule applies to this specific transition.
	currentRaw, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetKBArticle failed (permission check)", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update KB article state.")
		return
	}
	var current kbArticleSummary
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		slog.ErrorContext(r.Context(), "decode kb article failed (permission check)", "userID", user.UserID, "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	requiresManager := (current.State == "pending_review" && payload.State == "draft") || // reject
		(current.State == "pending_review" && payload.State == "published") || // approve
		(current.State == "published" && payload.State == "retired") // retire

	requiresAuthor := (current.State == "draft" && payload.State == "pending_review") || // submit
		(current.State == "published" && payload.State == "draft") // edit published

	if requiresAuthor {
		myID, err := h.currentPostgresUserID(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "resolve current user failed (permission check)", "userID", user.UserID, "id", id, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to update KB article state.")
			return
		}
		if myID != current.AuthorID {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
	}

	if requiresManager {
		myID, err := h.currentPostgresUserID(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "resolve current user failed (permission check)", "userID", user.UserID, "id", id, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to update KB article state.")
			return
		}
		searchBody, err := json.Marshal(map[string]string{
			"knowledgeBaseId": current.KnowledgeBaseID,
			"userId":          myID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		managersRaw, err := h.entity.SearchKBManagers(r.Context(), searchBody)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity SearchKBManagers failed (permission check)", "userID", user.UserID, "id", id, "err", err)
			mapUpstreamErrorGeneric(w, err, "Failed to update KB article state.")
			return
		}
		var managers kbManagerSearchResult
		if err := json.Unmarshal(managersRaw, &managers); err != nil {
			slog.ErrorContext(r.Context(), "decode kb managers failed (permission check)", "userID", user.UserID, "id", id, "err", err)
			writeError(w, http.StatusInternalServerError, ErrMsgInternal)
			return
		}
		if len(managers.Managers) == 0 {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
	}

	finalUserID, err := h.currentPostgresUserID(r.Context())
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to update KB article state.")
		return
	}
	var statePayload map[string]any
	if err := json.Unmarshal(body, &statePayload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	statePayload["updatedBy"] = finalUserID
	body, err = json.Marshal(statePayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.PatchKBArticleState(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PatchKBArticleState failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamError(w, err, "Failed to update KB article state.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PatchKBArticleContent handles PATCH /kb-articles/{id}.
// Only the article's author may edit it, and only while it's a draft --
// the entity service's UPDATE ... WHERE state = 'draft' silently no-ops
// (0 rows affected, surfaced as a 404-ish "not found") if it isn't.
func (h *KBArticleHandler) PatchKBArticleContent(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	currentRaw, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		mapUpstreamError(w, err, "Failed to update KB article.")
		return
	}
	var current kbArticleSummary
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	myID, err := h.currentPostgresUserID(r.Context())
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to update KB article.")
		return
	}
	if myID != current.AuthorID {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	var contentPayload map[string]any
	if err := json.Unmarshal(body, &contentPayload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	contentPayload["updatedBy"] = myID
	body, err = json.Marshal(contentPayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.PatchKBArticleContent(r.Context(), id, body)
	if err != nil {
		mapUpstreamError(w, err, "Failed to update KB article.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ListKnowledgeBases handles GET /knowledge-bases.
func (h *KBArticleHandler) ListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.ListKnowledgeBases(r.Context())
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to load knowledge bases.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// currentPostgresUserID resolves the caller's Postgres users.id by calling
// GET /users/me, which looks the row up by email -- the same identity space
// KB article authorId/reviewerId values live in. user.UserID (the JWT
// "userid" claim) is a different identifier (Asgardeo's sub) and must never
// be compared directly against those columns.
func (h *KBArticleHandler) currentPostgresUserID(ctx context.Context) (string, error) {
	raw, err := h.entity.GetUserMe(ctx)
	if err != nil {
		return "", err
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		return "", err
	}
	return me.ID, nil
}

// DeleteKBArticle handles DELETE /kb-articles/{id}.
// Author may delete their own draft or pending_review article; a manager of
// the article's knowledge base may also delete it while pending_review.
// Published/retired articles are never deletable (entity service enforces
// this too, as a second line of defense).
func (h *KBArticleHandler) DeleteKBArticle(w http.ResponseWriter, r *http.Request) {
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

	currentRaw, err := h.entity.GetKBArticle(r.Context(), id)
	if err != nil {
		mapUpstreamError(w, err, "Failed to delete KB article.")
		return
	}
	var current kbArticleSummary
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	myID, err := h.currentPostgresUserID(r.Context())
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to delete KB article.")
		return
	}

	isAuthor := myID == current.AuthorID
	allowed := false
	switch current.State {
	case "draft":
		allowed = isAuthor
	case "pending_review":
		if isAuthor {
			allowed = true
		} else {
			searchBody, err := json.Marshal(map[string]string{
				"knowledgeBaseId": current.KnowledgeBaseID,
				"userId":          myID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrMsgInternal)
				return
			}
			managersRaw, err := h.entity.SearchKBManagers(r.Context(), searchBody)
			if err != nil {
				mapUpstreamErrorGeneric(w, err, "Failed to delete KB article.")
				return
			}
			var managers kbManagerSearchResult
			if err := json.Unmarshal(managersRaw, &managers); err == nil && len(managers.Managers) > 0 {
				allowed = true
			}
		}
	}

	if !allowed {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	if err := h.entity.DeleteKBArticle(r.Context(), id); err != nil {
		mapUpstreamError(w, err, "Failed to delete KB article.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListKBArticleHistory handles GET /kb-articles/{id}/history.
func (h *KBArticleHandler) ListKBArticleHistory(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.entity.ListKBArticleHistory(r.Context(), id)
	if err != nil {
		mapUpstreamError(w, err, "Failed to load article history.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// kbManagerRecord mirrors one entry of the entity service's
// SearchKBManagers response -- only the field this handler needs.
type kbManagerRecord struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
}

type kbManagerSearchResultFull struct {
	Managers []kbManagerRecord `json:"managers"`
}

// ListMyManagedKnowledgeBases handles GET /kb-managers/my-knowledge-bases.
// Returns the ids of every knowledge base the current user is a manager of,
// used by the frontend's "To Review" tab to show only articles the caller
// can actually act on.
func (h *KBArticleHandler) ListMyManagedKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	myID, err := h.currentPostgresUserID(r.Context())
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to load managed knowledge bases.")
		return
	}

	searchBody, err := json.Marshal(map[string]string{"userId": myID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	raw, err := h.entity.SearchKBManagers(r.Context(), searchBody)
	if err != nil {
		mapUpstreamErrorGeneric(w, err, "Failed to load managed knowledge bases.")
		return
	}

	var result kbManagerSearchResultFull
	if err := json.Unmarshal(raw, &result); err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	ids := make([]string, 0, len(result.Managers))
	for _, m := range result.Managers {
		ids = append(ids, m.KnowledgeBaseID)
	}

	respBody, err := json.Marshal(map[string]any{"knowledgeBaseIds": ids})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	writeJSON(w, http.StatusOK, respBody)
}
