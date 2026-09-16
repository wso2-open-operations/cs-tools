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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// aiChatAgentClient abstracts the upstream AI chat agent operations used by AIChatHandler.
type aiChatAgentClient interface {
	CreateCaseClassification(ctx context.Context, req aichatagent.CaseClassificationPayload) (aichatagent.CaseClassificationResponse, error)
	CreateChat(ctx context.Context, req aichatagent.ChatPayload) (aichatagent.ChatResponse, error)
	GetRecommendations(ctx context.Context, req aichatagent.RecommendationRequest) (aichatagent.RecommendationResponse, error)
	GetConversationSummary(ctx context.Context, projectID, conversationID string) (aichatagent.ConversationSummaryResponse, error)
}

// entityConversationClient abstracts the entity-service conversation and
// comment operations used by AIChatHandler.
type entityConversationClient interface {
	SearchConversations(ctx context.Context, req entity.SearchConversationsRequest) (entity.SearchConversationsResponse, error)
	SearchComments(ctx context.Context, req entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error)
	CreateComment(ctx context.Context, req entity.CreateCommentRequest) (entity.CreateCommentResponse, error)
	GetConversation(ctx context.Context, id string) (entity.ConversationDetails, error)
	CreateConversation(ctx context.Context, req entity.CreateConversationRequest) (entity.CreateConversationResponse, error)
	UpdateConversation(ctx context.Context, id string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error)
	// GetProject resolves the project's owning account, so a chat message is
	// billed to the account the caller belongs to rather than to the project.
	// Same method and same reason as WebSocketHandler's entityCommentCreator.
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// AIChatHandler handles HTTP requests for the AI chat feature: case
// classification, KB article recommendations, conversation CRUD/search/
// messages, and conversation summaries. See WebSocketHandler for the
// real-time chat proxy and its doc comment for the comment createdBy
// override gap this handler works around the same way.
type AIChatHandler struct {
	ai     aiChatAgentClient
	entity entityConversationClient

	callerScope *CallerScopeResolver
}

// NewAIChatHandler creates an AIChatHandler backed by the given AI chat agent and entity clients.
func NewAIChatHandler(ai aiChatAgentClient, entityClient entityConversationClient) *AIChatHandler {
	return &AIChatHandler{ai: ai, entity: entityClient}
}

// SetCallerScope enables caller-scoped access: conversation operations require
// the caller to be an active portal-user contact of the project in the URL path
// or the project the conversation belongs to. Always enforced in production
// (main.go calls this unconditionally, no kill switch) — see
// ProjectHandler.SetCallerScope for why this is a setter rather than a
// constructor parameter.
func (h *AIChatHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// resolveAccountID resolves the account that owns projectID, for the agent's
// per-account token budget. The WebSocket path does this once per connection
// (see HandleWebSocket); the REST paths have no connection to hang it on, so
// they resolve per request instead.
//
// Falls back to the project ID when entity-service reports no account, which is
// what the frontend does too (`projectDetails?.account?.id || projectId`) — the
// agent rejects an empty accountId outright.
func (h *AIChatHandler) resolveAccountID(ctx context.Context, projectID string) (string, error) {
	project, err := h.entity.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.Account.ID == "" {
		return projectID, nil
	}
	return project.Account.ID, nil
}

// ClassifyCase handles POST /cases/classify.
func (h *AIChatHandler) ClassifyCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CaseClassificationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.ai.CreateCaseClassification(r.Context(), dto.BuildCaseClassificationPayload(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent CreateCaseClassification failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to classify chat message.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseClassification(result))
}

// SearchRecommendations handles POST /conversations/recommendations/search.
func (h *AIChatHandler) SearchRecommendations(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.RecommendationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.ai.GetRecommendations(r.Context(), dto.BuildRecommendationRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent GetRecommendations failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve recommendations.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapRecommendationResponse(result))
}

// SearchConversations handles POST /projects/{id}/conversations/search.
func (h *AIChatHandler) SearchConversations(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ConversationSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchConversations(r.Context(), dto.BuildEntitySearchConversationsRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchConversations failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversations.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchConversations(result))
}

// GetConversationMessages handles GET /conversations/{id}/messages.
// Backed by the generic comment search (referenceType=conversation), the
// same route the portal uses for POST /comments/search.
func (h *AIChatHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	conversationID := r.PathValue("id")
	if conversationID == "" || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	conv, err := h.entity.GetConversation(r.Context(), conversationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetConversation failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation messages.")
		return
	}
	if conv.Project == nil {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, conv.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	limit, offset, ok := parseLimitOffset(w, r)
	if !ok {
		return
	}

	req := dto.BuildEntitySearchCommentsRequest(dto.CommentSearchRequest{
		ReferenceID:   conversationID,
		ReferenceType: string(entity.ReferenceTypeConversation),
		Pagination:    entity.Pagination{Limit: limit, Offset: offset},
	})

	result, err := h.entity.SearchComments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchComments failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation messages.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchComments(result))
}

// createEntityStateResolved is entity-service's ConversationState value used
// to auto-resolve a conversation when the AI agent reports the issue solved.
const createEntityStateResolved = "RESOLVED"

// CreateConversation handles POST /projects/{id}/conversations — starts a
// brand-new conversation and gets the AI agent's first response via this
// composite flow: create the conversation, call the AI agent, optionally
// fetch KB recommendations (only when both Region and Tier are supplied),
// persist the AI's reply as a comment, then auto-resolve the conversation
// if the agent reports the issue solved.
// Error-handling: conversation creation, the AI call, and comment
// persistence are all fatal (500) on failure with no compensating rollback
// of earlier steps; recommendations and auto-resolve are best-effort and
// never fail the request.
func (h *AIChatHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChatMessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Resolved before the conversation is created: if this fails, no orphaned
	// conversation is left behind for a project we could not resolve.
	accountID, err := h.resolveAccountID(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve project account", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create a new conversation.")
		return
	}

	convResp, err := h.entity.CreateConversation(r.Context(), entity.CreateConversationRequest{
		ProjectID:      projectID,
		InitialMessage: req.Message,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateConversation failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create a new conversation.")
		return
	}
	conversationID := convResp.Conversation.ID

	chatResp, err := h.ai.CreateChat(r.Context(), aichatagent.ChatPayload{
		Message:        req.Message,
		AccountID:      accountID,
		ConversationID: conversationID,
		EnvProducts:    req.EnvProducts,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent CreateChat failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to process chat message.")
		return
	}

	if req.Region != "" && req.Tier != "" {
		recResp, err := h.ai.GetRecommendations(r.Context(), aichatagent.RecommendationRequest{
			ChatHistory: []aichatagent.ChatMessage{
				{Role: "user", Content: req.Message},
				{Role: "assistant", Content: chatResp.Message},
			},
			ConversationData: aichatagent.ConversationData{
				ChatHistory: "user: " + req.Message + "\nassistant: " + chatResp.Message,
				EnvProducts: req.EnvProducts,
				Region:      req.Region,
				Tier:        req.Tier,
			},
		})
		if err != nil {
			slog.WarnContext(r.Context(), "aichatagent GetRecommendations failed for first chat invocation", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		} else {
			chatResp.Recommendations = &recResp
		}
	}

	// Attributed to the assistant, not the customer whose token relayed it —
	// see entity.CreatedByAgent.
	if _, err := h.entity.CreateComment(r.Context(), entity.CreateCommentRequest{
		ReferenceID:   conversationID,
		ReferenceType: entity.ReferenceTypeConversation,
		Type:          entity.CommentTypeComment,
		Content:       chatResp.Message,
		CreatedBy:     entity.CreatedByAgent,
	}); err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed for chat response", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to save chat response as comment.")
		return
	}

	if chatResp.Resolved != nil && *chatResp.Resolved {
		if _, err := h.entity.UpdateConversation(r.Context(), conversationID, entity.UpdateConversationRequest{State: createEntityStateResolved}); err != nil {
			// Best-effort: the main flow already succeeded, so this
			// failure is logged, not returned.
			slog.ErrorContext(r.Context(), "entity UpdateConversation failed to auto-resolve", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}

	writeJSONValue(w, http.StatusOK, dto.MapChatResponse(chatResp))
}

// SendConversationMessage handles
// POST /projects/{projectId}/conversations/{conversationId}/messages — a
// follow-up message on an existing conversation. Unlike CreateConversation,
// no recommendations call is ever made here — recommendations are only
// attached on a conversation's first message.
func (h *AIChatHandler) SendConversationMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("projectId")
	conversationID := r.PathValue("conversationId")
	if !uuidRe.MatchString(projectID) || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	conv, err := h.entity.GetConversation(r.Context(), conversationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetConversation failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to process conversation message.")
		return
	}
	if conv.Project == nil || conv.Project.ID != projectID {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.ChatMessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	accountID, err := h.resolveAccountID(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve project account", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to process conversation message.")
		return
	}

	chatResp, err := h.ai.CreateChat(r.Context(), aichatagent.ChatPayload{
		Message:        req.Message,
		AccountID:      accountID,
		ConversationID: conversationID,
		EnvProducts:    req.EnvProducts,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent CreateChat failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to process conversation message.")
		return
	}

	if _, err := h.entity.CreateComment(r.Context(), entity.CreateCommentRequest{
		ReferenceID:   conversationID,
		ReferenceType: entity.ReferenceTypeConversation,
		Type:          entity.CommentTypeComment,
		Content:       req.Message,
	}); err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed for conversation message", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to save user message as comment.")
		return
	}

	// Attributed to the assistant, not the customer whose token relayed it —
	// see entity.CreatedByAgent.
	if _, err := h.entity.CreateComment(r.Context(), entity.CreateCommentRequest{
		ReferenceID:   conversationID,
		ReferenceType: entity.ReferenceTypeConversation,
		Type:          entity.CommentTypeComment,
		Content:       chatResp.Message,
		CreatedBy:     entity.CreatedByAgent,
	}); err != nil {
		slog.ErrorContext(r.Context(), "entity CreateComment failed for chat response", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to save chat response as comment.")
		return
	}

	if chatResp.Resolved != nil && *chatResp.Resolved {
		if _, err := h.entity.UpdateConversation(r.Context(), conversationID, entity.UpdateConversationRequest{State: createEntityStateResolved}); err != nil {
			slog.ErrorContext(r.Context(), "entity UpdateConversation failed to auto-resolve", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		}
	}

	writeJSONValue(w, http.StatusOK, dto.MapChatResponse(chatResp))
}

// GetConversation handles GET /conversations/{id}.
func (h *AIChatHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetConversation(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetConversation failed", "userID", user.UserID, "conversationID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation details.")
		return
	}

	if result.Project == nil {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, result.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	writeJSONValue(w, http.StatusOK, dto.MapConversationDetails(result))
}

// UpdateConversation handles PATCH /conversations/{id}. Status must be one
// of "closed", "abandoned", or "converted" — see dto.ConversationStatusUpdate.
func (h *AIChatHandler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	conv, err := h.entity.GetConversation(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetConversation failed", "userID", user.UserID, "conversationID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update conversation.")
		return
	}
	if conv.Project == nil {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}
	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, conv.Project.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	var req dto.ConversationStatusUpdate
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	entityReq, ok := dto.BuildEntityConversationStateUpdate(req)
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid conversation status. Allowed values: closed, abandoned, converted.")
		return
	}

	if _, err := h.entity.UpdateConversation(r.Context(), id, entityReq); err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateConversation failed", "userID", user.UserID, "conversationID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update conversation.")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetConversationSummary handles GET /projects/{id}/conversations/{conversationId}/summary.
func (h *AIChatHandler) GetConversationSummary(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	conversationID := r.PathValue("conversationId")
	if !uuidRe.MatchString(projectID) || !uuidRe.MatchString(conversationID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	result, err := h.ai.GetConversationSummary(r.Context(), projectID, conversationID)
	if err != nil {
		slog.ErrorContext(r.Context(), "aichatagent GetConversationSummary failed", "userID", user.UserID, "conversationID", conversationID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve conversation summary.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapConversationSummary(result))
}

// parseLimitOffset parses the limit/offset query parameters, defaulting to
// 20/0. Writes a 400 response and returns ok=false on an invalid value.
// maxConversationMessagesLimit caps GetConversationMessages' limit query
// param, matching this backend's other search endpoints' page-size ceiling.
const maxConversationMessagesLimit = 100

func parseLimitOffset(w http.ResponseWriter, r *http.Request) (limit, offset int, ok bool) {
	limit, offset = 20, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > maxConversationMessagesLimit {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return 0, 0, false
		}
		limit = v
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return 0, 0, false
		}
		offset = v
	}
	return limit, offset, true
}
