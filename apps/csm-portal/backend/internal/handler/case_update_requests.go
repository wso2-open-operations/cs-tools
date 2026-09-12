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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// requestUpdateCategory identifies which fixed-message set a "request update"
// nudge is drawn from. It is always server-determined from the case's own
// type/engagementType — the caller never supplies it — because the wording is
// specific to the kind of work the case represents (a generic case vs. a
// migration engagement) and must not be spoofable from the request body.
type requestUpdateCategory string

const (
	requestUpdateCategoryGeneric   requestUpdateCategory = "generic"
	requestUpdateCategoryMigration requestUpdateCategory = "migration"
)

// requestUpdateStage identifies which of the three fixed reminder stages (or
// a caller-supplied custom message) to post.
type requestUpdateStage string

const (
	requestUpdateStageFirst  requestUpdateStage = "first"
	requestUpdateStageSecond requestUpdateStage = "second"
	requestUpdateStageFinal  requestUpdateStage = "final"
	requestUpdateStageCustom requestUpdateStage = "custom"
)

// requestUpdateTemplates holds the fixed HTML message for every
// (category, stage) pair. Content is reviewed copy: do not paraphrase or
// reformat it.
var requestUpdateTemplates = map[requestUpdateCategory]map[requestUpdateStage]string{
	requestUpdateCategoryGeneric: {
		requestUpdateStageFirst:  `<p>Hi Team,</p><p>Please let us know if you need further assistance regarding this.</p><p>Thanks and Regards,<br>WSO2 Team.</p>`,
		requestUpdateStageSecond: `<p>Hi Team,</p><p>Please let us know if you need further assistance regarding this or if this case is good to close.</p><p>Thanks and Regards,<br>WSO2 Team.</p>`,
		requestUpdateStageFinal:  `<p>Hi Team,</p><p>As we haven't received a response to our previous follow-ups, we will proceed with placing this case on hold.</p><p>The Technical Owner and Account Manager have been informed.</p><p>Thanks &amp; Regards,<br>WSO2 Team.</p>`,
	},
	requestUpdateCategoryMigration: {
		requestUpdateStageFirst:  `<p>Hi Team,</p><p>Kindly share a progress update on the migration process.</p><p>Thanks &amp; Regards,<br>WSO2 Team</p>`,
		requestUpdateStageSecond: `<p>Hi Team,</p><p>Kindly share a progress update on the migration process.</p><p>If you are currently facing any blockers, delays, or technical issues please do not hesitate to contact us.</p><p>Thanks &amp; Regards,<br>WSO2 Team</p>`,
		requestUpdateStageFinal:  `<p>Hi Team,</p><p>As we haven't received a response to our previous follow-ups, we will proceed with placing this migration ticket on hold.</p><p>If you plan to resume the migration, please open a related case and reference this ticket so we can continue from there.</p><p>The Technical Owner and Account Manager have been informed.</p><p>Thanks &amp; Regards,<br>WSO2 Team</p>`,
	},
}

// requestUpdateRequest is the body of POST /cases/{id}/request-update.
type requestUpdateRequest struct {
	Stage         requestUpdateStage `json:"stage"`
	CustomContent string             `json:"customContent"`
}

// requestUpdateTemplateResponse is the response shape of
// GET /case-update-request-templates: both categories' full stage->content maps.
type requestUpdateTemplateResponse struct {
	Generic   map[requestUpdateStage]string `json:"generic"`
	Migration map[requestUpdateStage]string `json:"migration"`
}

// RequestCaseUpdate handles POST /cases/{id}/request-update. It posts a
// customer-visible comment nudging the customer for a response, using one of
// three fixed reminder templates (first/second/final) or a caller-supplied
// custom message. Only available while the case is in awaiting_info or
// solution_proposed — the states where the ball is meant to be in the
// customer's court.
func (h *CaseHandler) RequestCaseUpdate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var req requestUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	var content string
	switch req.Stage {
	case requestUpdateStageFirst, requestUpdateStageSecond, requestUpdateStageFinal:
		if req.CustomContent != "" {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
	case requestUpdateStageCustom:
		if strings.TrimSpace(req.CustomContent) == "" {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		content = req.CustomContent
	default:
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	current, err := h.entity.GetCase(r.Context(), caseID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during request-update guard", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to request an update.")
		return
	}
	var currentCase struct {
		State            string `json:"state"`
		Type             string `json:"type"`
		EngagementType   string `json:"engagementType"`
		AssignedEngineer *struct {
			ID *string `json:"id"`
		} `json:"assignedEngineer"`
	}
	if err := json.Unmarshal(current, &currentCase); err != nil {
		slog.ErrorContext(r.Context(), "failed to parse case state for request-update guard", "userID", user.UserID, "caseID", caseID, "err", err)
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	if currentCase.State != caseStateAwaitingInfo && currentCase.State != caseStateSolutionProposed {
		writeError(w, http.StatusConflict, ErrMsgRequestUpdateNotAllowed)
		return
	}

	// Ownership check, mirroring CreateCaseComment's guard for a public
	// (non-work_note) comment: this endpoint always posts a customer-visible
	// comment, so only the case's assigned engineer may trigger it. Resolved
	// here, after the state gate, so the extra lookup is only paid on a
	// request that would otherwise be accepted.
	currentUserID := h.resolveCurrentUserID(r, user)
	if currentUserID == "" {
		// The caller's identity could not be established, so ownership cannot
		// be decided either way: fail closed, but as a server-side failure
		// rather than a misleading "you are not the assignee".
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	if currentCase.AssignedEngineer == nil || currentCase.AssignedEngineer.ID == nil || *currentCase.AssignedEngineer.ID != currentUserID {
		writeError(w, http.StatusForbidden, ErrMsgCommentNotOwnCase)
		return
	}

	// engagementType is compared case-insensitively because it is NOT
	// normalized before reaching this layer: entity-service's CaseView.EngagementType
	// carries ServiceNow's raw choice-field display label unmodified (e.g.
	// literally "Migration", capitalized) — unlike State/WorkState, which go
	// through explicit lowering functions before reaching the domain layer. A
	// strict-case compare would silently misclassify every real migration
	// case as generic, so case-insensitive is required here, not just safer.
	category := requestUpdateCategoryGeneric
	if currentCase.Type == "engagement" && strings.EqualFold(currentCase.EngagementType, "migration") {
		category = requestUpdateCategoryMigration
	}

	if req.Stage != requestUpdateStageCustom {
		content = requestUpdateTemplates[category][req.Stage]
	}

	commentBody, err := json.Marshal(map[string]string{
		"type":    "comment",
		"content": content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	result, err := h.entity.CreateCaseComment(r.Context(), caseID, commentBody)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateCaseComment failed during request-update", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to request an update.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetCaseUpdateRequestTemplates handles GET /case-update-request-templates.
// Returns the fixed reminder message catalogue for both categories, with no
// case-specific logic or upstream call — the caller picks a stage from
// whichever category applies once RequestCaseUpdate has told it which case
// the case falls into (or the frontend infers it from the case it already has
// loaded).
func (h *CaseHandler) GetCaseUpdateRequestTemplates(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	writeJSONValue(w, http.StatusOK, requestUpdateTemplateResponse{
		Generic:   requestUpdateTemplates[requestUpdateCategoryGeneric],
		Migration: requestUpdateTemplates[requestUpdateCategoryMigration],
	})
}
