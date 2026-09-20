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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityCaseClient abstracts the entity-service case operations used by CaseHandler.
type entityCaseClient interface {
	SearchCases(ctx context.Context, req entity.SearchCasesRequest) (entity.SearchCasesResponse, error)
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
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
}

// NewCaseHandler creates a CaseHandler backed by the given entity client.
func NewCaseHandler(entity entityCaseClient) *CaseHandler {
	return &CaseHandler{entity: entity}
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

// caseStateReader is the single entity-service call the closed-case guard
// needs. It is declared separately from entityCaseClient/entityAttachmentClient
// so both handlers can share one guard implementation.
type caseStateReader interface {
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
}

// caseIsClosed reports whether caseID names a case in the closed state.
//
// A closed case's attachments are read-only, and this backend is where that
// rule lives: entity-service still accepts the write, and the webapp only
// disables the upload/delete controls, so the direct-API path needs closing
// too. Used by CreateCaseAttachment, PatchCaseAttachment, and — via the
// attachment's own referenceId — AttachmentHandler.DeleteAttachment.
//
// The extra lookup deliberately fails open: an entity-service error leaves the
// operation to proceed rather than blocking a legitimate write on a failed
// guard read. That also covers DeleteAttachment's case, where referenceId may
// name a deployment or conversation instead of a case and GetCase 404s. This
// mirrors the Ballerina backend's
// `caseResponse is entity:CaseResponse && isCaseClosed(caseResponse)` guard,
// which ignores the error branch the same way.
func caseIsClosed(ctx context.Context, client caseStateReader, caseID string) bool {
	caseView, err := client.GetCase(ctx, caseID)
	if err != nil {
		return false
	}
	return dto.IsCaseStateClosed(caseView.State)
}

// CreateCaseAttachment handles POST /cases/{id}/attachments. Rejected with 400
// when the case is closed (see caseIsClosed).
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CreateCaseAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if caseIsClosed(r.Context(), h.entity, id) {
		slog.WarnContext(r.Context(), "rejected attachment create on a closed case", "userID", user.UserID, "caseID", id)
		writeError(w, http.StatusBadRequest, ErrMsgCaseClosedForAttachmentCreate)
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

	if req.ProjectID == "" || !uuidRe.MatchString(req.ProjectID) {
		writeError(w, http.StatusBadRequest, "Project ID is required and must be a valid UUID.")
		return
	}

	project, err := h.entity.GetProject(r.Context(), req.ProjectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed during CreateCase", "userID", user.UserID, "projectID", req.ProjectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	if isProjectSuspendedOrExpired(project) {
		slog.WarnContext(r.Context(), "attempted to create case for suspended or expired project", "userID", user.UserID, "projectID", req.ProjectID)
		writeError(w, http.StatusForbidden, "Cannot create cases for a suspended or contract-expired project.")
		return
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

	result, err := h.entity.GetCaseFeedback(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCaseFeedback failed", "userID", user.UserID, "caseID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve feedback for the case.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapCaseFeedback(result))
}

// SubmitCaseFeedback handles POST /cases/{id}/feedback. Rejected with 400
// when the case is not in the closed state.
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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.SubmitCaseFeedbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if caseView, err := h.entity.GetCase(r.Context(), id); err == nil && !dto.IsCaseStateClosed(caseView.State) {
		slog.WarnContext(r.Context(), "rejected feedback submission on a non-closed case", "userID", user.UserID, "caseID", id, "state", caseView.State)
		writeError(w, http.StatusBadRequest, ErrMsgCaseNotClosedForFeedback)
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
// referenceId/referenceType are injected server-side (caseId path param,
// ReferenceTypeCase). Only Name is read from the request body — Description
// is never wired through here, by design for this route (case attachments
// don't carry a description). Rejected with 400 when the case is closed (see
// caseIsClosed).
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

	if caseIsClosed(r.Context(), h.entity, caseID) {
		slog.WarnContext(r.Context(), "rejected attachment update on a closed case", "userID", user.UserID, "caseID", caseID, "attachmentID", attachmentID)
		writeError(w, http.StatusBadRequest, ErrMsgCaseClosedForAttachmentUpdate)
		return
	}

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

// isProjectSuspendedOrExpired checks if a project is suspended or its contract has ended.
func isProjectSuspendedOrExpired(project entity.ProjectDetailsView) bool {
	if project.ClosureState != nil && strings.EqualFold(strings.TrimSpace(*project.ClosureState), "suspended") {
		return true
	}
	if !project.EndDate.IsZero() {
		todayUTC := time.Now().UTC().Format("2006-01-02")
		endDateUTC := project.EndDate.UTC().Format("2006-01-02")
		return todayUTC > endDateUTC
	}
	return false
}

