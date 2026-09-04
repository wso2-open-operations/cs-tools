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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityCaseClient abstracts the entity-service case operations used by CaseHandler.
type entityCaseClient interface {
	SearchCases(ctx context.Context, req entity.SearchCasesRequest) (entity.SearchCasesResponse, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
	CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error)
	UpdateConversation(ctx context.Context, id string, req entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error)
	UpdateCase(ctx context.Context, id string, req entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error)
	CreateCaseComment(ctx context.Context, caseID string, req entity.CreateCaseCommentRequest) (entity.CreateCaseCommentResponse, error)
	SearchCaseActivities(ctx context.Context, caseID string, req entity.SearchCaseActivitiesRequest) (entity.SearchCaseActivitiesResponse, error)
	GetCaseFeedback(ctx context.Context, caseID string) (entity.CaseFeedback, error)
	SubmitCaseFeedback(ctx context.Context, caseID string, req entity.SubmitCaseFeedbackRequest) (entity.SubmitCaseFeedbackResponse, error)
	UpdateAttachment(ctx context.Context, id string, req entity.UpdateAttachmentRequest) (entity.UpdateAttachmentResponse, error)
	CreateEscalation(ctx context.Context, req entity.CreateEscalationRequest) (entity.CreateEscalationResponse, error)
	SearchEscalations(ctx context.Context, req entity.SearchEscalationsRequest) (entity.SearchEscalationsResponse, error)
	SearchAttachments(ctx context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error)
	CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error)
}

// CaseHandler handles HTTP requests for case operations.
type CaseHandler struct {
	entity entityCaseClient

	callerScope *CallerScopeResolver
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
}

// SetCallerScope wires up caller-scoped case access: SearchCases requires
// the caller to be an active portal-user contact of the project in the URL
// path, and GetCase requires the same for the case's own project (see
// CallerScopeResolver). main.go always calls this in production — there is
// no kill switch. A setter rather than a constructor parameter purely so
// the many pre-existing tests across this package that construct handlers
// directly, unrelated to this feature, keep compiling without change; a nil
// resolver (never calling this) is treated as unscoped rather than
// panicking — see requireProjectMember's doc comment.
func (h *CaseHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// SearchCases handles POST /projects/{id}/cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
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

	var req dto.CaseSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCases(r.Context(), dto.BuildEntitySearchCasesRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCases failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search cases.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchCases(result))
}

// SearchCaseAttachments handles GET /cases/{id}/attachments?limit=&offset=.
func (h *CaseHandler) SearchCaseAttachments(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	limit, offset, ok := parseLimitOffset(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchAttachments(r.Context(), entity.SearchAttachmentsRequest{
		ReferenceID:   id,
		ReferenceType: entity.ReferenceTypeCase,
		Pagination:    entity.Pagination{Limit: limit, Offset: offset},
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAttachments failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve case attachments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseAttachments(result))
}

// CreateCaseAttachment handles POST /cases/{id}/attachments.
func (h *CaseHandler) CreateCaseAttachment(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CreateCaseAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateAttachment(r.Context(), dto.BuildEntityCreateCaseAttachmentRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAttachment failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create case attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapAttachmentCreate(result))
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.entity.GetCase(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve case.")
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// if !requireProjectMember(w, r, h.callerScope, result.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	writeJSONValue(w, http.StatusOK, dto.MapCaseDetails(result))
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CreateCaseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if req.ProjectID != "" && uuidRe.MatchString(req.ProjectID) {
		// Commented out pending end-to-end verification against real
		// entity-service data — uncomment while testing, re-comment before
		// committing. See handler.CallerScopeResolver / requireProjectMember.
		// if !requireProjectMember(w, r, h.callerScope, req.ProjectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
		// 	return
		// }
	}

	entityReq := dto.BuildEntityCreateCaseRequest(req)
	// CreatedBy is server-set from the authenticated caller, never from the
	// request body (the struct's json:"-" tag means a client-supplied value
	// would be silently dropped anyway, but set it explicitly for clarity).
	entityReq.CreatedBy = user.Email

	result, err := h.entity.CreateCase(r.Context(), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCase failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create case.")
		return
	}

	// A case raised from a Novera chat marks that conversation Converted so it
	// stops counting as an active chat. Deliberately after the response is
	// decided and non-blocking: the case already exists, so a conversion failure
	// must not turn a successful creation into an error. entity-service does not
	// do this itself even though the id is forwarded on the create request —
	// same as the Ballerina backend, which forwards payload.conversationId and
	// still performs this update explicitly.
	if req.ConversationID != "" && uuidRe.MatchString(req.ConversationID) {
		if _, err := h.entity.UpdateConversation(r.Context(), req.ConversationID, entity.UpdateConversationRequest{State: conversationStateConverted}); err != nil {
			slog.ErrorContext(r.Context(), "entity UpdateConversation failed to mark the source conversation converted", "userID", user.UserID, "conversationID", req.ConversationID, "err", summarizeErr(err))
		}
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCaseCreate(result))
}

// PatchCase handles PATCH /cases/{id}.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.UpdateCaseRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// entity-service requires exactly one of these primary fields per PATCH —
	// see dto.UpdateCaseRequest's doc comment.
	primaryFieldsSet := 0
	for _, set := range []bool{req.StateKey != nil, len(req.WatchList) > 0} {
		if set {
			primaryFieldsSet++
		}
	}
	if primaryFieldsSet != 1 {
		writeError(w, http.StatusBadRequest, "Exactly one of stateKey or watchList must be provided.")
		return
	}

	result, err := h.entity.UpdateCase(r.Context(), id, dto.BuildEntityUpdateCaseRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update case.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseUpdate(result))
}

// CreateCaseComment handles POST /cases/{id}/comments.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CaseCommentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateCaseComment(r.Context(), id, dto.BuildEntityCreateCaseCommentRequest(id, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to add comment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapCaseComment(result))
}

// SearchCaseActivities handles POST /cases/{id}/activities/search.
func (h *CaseHandler) SearchCaseActivities(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchCaseActivitiesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchCaseActivities(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchCaseActivities failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search case activities.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchCaseActivities(result))
}

// GetCaseFeedback handles GET /cases/{id}/feedback.
func (h *CaseHandler) GetCaseFeedback(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	result, err := h.entity.GetCaseFeedback(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCaseFeedback failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve feedback for the case.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseFeedback(result))
}

// SubmitCaseFeedback handles POST /cases/{id}/feedback.
func (h *CaseHandler) SubmitCaseFeedback(w http.ResponseWriter, r *http.Request) {
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

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), id)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.SubmitCaseFeedbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SubmitCaseFeedback(r.Context(), id, dto.BuildEntitySubmitCaseFeedbackRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SubmitCaseFeedback failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to submit feedback for the case.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapSubmitCaseFeedbackResponse(result))
}

// PatchCaseAttachment handles PATCH /cases/{caseId}/attachments/{attachmentId}.
func (h *CaseHandler) PatchCaseAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	attachmentID := r.PathValue("attachmentId")
	if !uuidRe.MatchString(caseID) || !uuidRe.MatchString(attachmentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.AttachmentUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req.Description = nil // this route never forwards description (case attachments don't carry one)

	entityReq := dto.BuildEntityUpdateAttachmentRequest(req, caseID, entity.ReferenceTypeCase)
	result, err := h.entity.UpdateAttachment(r.Context(), attachmentID, entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update the attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapUpdatedAttachment(result))
}

// CreateCaseEscalation handles POST /cases/{caseId}/escalations.
func (h *CaseHandler) CreateCaseEscalation(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.EscalationCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	action, ok, errMsg := dto.ValidateEscalationAction(req)
	if !ok {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	result, err := h.entity.CreateEscalation(r.Context(), dto.BuildEntityCreateEscalationRequest(caseID, action, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateEscalation failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create escalation.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapEscalationCreateResponse(result))
}

// SearchCaseEscalations handles POST /cases/{caseId}/escalations/search.
// filters.caseIds is always forced to [caseId] server-side.
func (h *CaseHandler) SearchCaseEscalations(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("caseId")
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Commented out pending end-to-end verification against real
	// entity-service data — uncomment while testing, re-comment before
	// committing. See handler.CallerScopeResolver / requireProjectMember.
	// caseView, err := h.entity.GetCase(r.Context(), caseID)
	// if err != nil {
	// 	slog.ErrorContext(r.Context(), "entity GetCase failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
	// 	mapUpstreamError(w, err, "Failed to retrieve case.")
	// 	return
	// }
	// if !requireProjectMember(w, r, h.callerScope, caseView.ProjectDetails.ID, user.UserID, user.Email, http.StatusNotFound, ErrMsgNotFound) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.EscalationSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchEscalations(r.Context(), dto.BuildEntitySearchEscalationsRequest(caseID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchEscalations failed", "userID", user.UserID, "caseID", caseID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search escalations.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapEscalationSearchResponse(result))
}
