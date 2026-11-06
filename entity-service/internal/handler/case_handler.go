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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// maxCaseBodySize caps the CreateCase JSON body at 15 MiB. A case description can
// still carry base64 images pasted into it, and the generic maxRequestBodySize
// (1 MiB) is too small for that. Attachment files never travel in this body: they
// are uploaded afterwards through the stored-attachment routes.
const maxCaseBodySize = int64(15 << 20)

// maxTagSearchLimit caps POST /tags/search results. Tag search backs a
// type-ahead, not a paged browse, so the ceiling is deliberately low.
const maxTagSearchLimit = 100

// caseTooLargeMsg is returned when the CreateCase body is over maxCaseBodySize.
const caseTooLargeMsg = "request exceeds the maximum allowed size of 15 MB"

// CaseHandler handles HTTP requests for the case resource.
type CaseHandler struct {
	svc service.CaseService
	// m2mClientIDs is config.Config.M2MClientIDs -- the same trusted-internal-
	// client-id set AccessService.ResolveScope already uses to grant
	// unconditional, unrestricted access. AddCaseTag/CreateCaseComment reuse
	// it to decide whether a caller may claim an arbitrary ActorEmail: a
	// caller whose x-jwt-assertion names a client id in this set is already
	// trusted to bypass RLS entirely, so it needs no second, narrower
	// allowlist just to claim a comment/tag author.
	m2mClientIDs map[string]bool
}

// NewCaseHandler constructs a CaseHandler with the given service and the
// trusted M2M client id set permitted to use AddCaseTagRequest.ActorEmail/
// CreateCaseCommentRequest.ActorEmail (see that field's own doc comment and
// config.Config.M2MClientIDs'). Passing a nil/empty set is safe -- it simply
// means no ActorEmail is ever trusted, matching the config's default-closed
// posture.
func NewCaseHandler(svc service.CaseService, m2mClientIDs map[string]bool) *CaseHandler {
	return &CaseHandler{svc: svc, m2mClientIDs: m2mClientIDs}
}

// GetCase handles GET /cases/{id}.
func (h *CaseHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.svc.GetCaseByID(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

// CreateCase handles POST /cases.
func (h *CaseHandler) CreateCase(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCaseRequest
	if !decodeRequestWithLimit(w, r, &req, maxCaseBodySize, caseTooLargeMsg) {
		return
	}
	c, err := h.svc.CreateCase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

// PatchCase handles PATCH /cases/{id}.
// Accepts stateKey, severityKey, workStateKey (both data sources), or watchList, assigneeEmail (ServiceNow only).
// Exactly one field must be provided per request.
//
// ActorEmail follows the same M2M trust gate as CreateCaseComment/AddCaseTag's
// own ActorEmail (see that doc comment): honored only when the caller's own
// x-jwt-assertion names a client id in h.m2mClientIDs, and rejected outright
// if it arrives alongside a real x-user-id-token. CaseService.UpdateCase
// itself rejects it on every branch except MarkFixIssued. Unlike those two,
// this one's value is only trimmed, never lowercased -- see its own
// normalization block below for why.
func (h *CaseHandler) PatchCase(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateCaseRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")

	if req.ActorEmail != nil {
		if middleware.UserIDTokenFromContext(r.Context()) != "" {
			writeServiceError(w, r, &apierror.ValidationError{
				Msg: "actorEmail must not be supplied alongside an x-user-id-token",
			})
			return
		}
		if clientID := auth.IdentityFromContext(r.Context()).ClientID; clientID == "" || !h.m2mClientIDs[clientID] {
			writeServiceError(w, r, &apierror.ForbiddenError{
				Msg: "caller is not an authorized M2M client",
			})
			return
		}
		// Trimmed, not lowercased -- unlike CreateCaseComment/AddCaseTag's own
		// ActorEmail, this value is stored verbatim as work_item_activity's
		// created_by/user_email (CaseService.updateCaseMarkFixIssued) and is
		// not necessarily even an email address in practice (a configured
		// display label like "WSO2 Updates" is a real, intended case) -- so
		// case must be preserved exactly as the trusted caller sent it.
		actorEmail := strings.TrimSpace(*req.ActorEmail)
		if actorEmail == "" {
			writeServiceError(w, r, &apierror.ValidationError{
				Msg: "actorEmail must not be blank",
			})
			return
		}
		req.ActorEmail = &actorEmail
	}

	resp, err := h.svc.UpdateCase(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateCaseComment handles POST /cases/{id}/comments. It has two callers:
//
//   - A token-resolved caller (the normal case): no ActorEmail in the body,
//     the acting user is resolved from x-user-id-token inside
//     CaseService.CreateCaseComment itself, exactly as before this M2M path
//     existed.
//   - An M2M caller with no x-user-id-token to resolve an actor from (e.g.
//     UMT via csm-integration-service): ActorEmail is set instead, and is
//     honored only when the caller's own x-jwt-assertion names a client id
//     in the trusted M2M set (h.m2mClientIDs, config.Config.M2MClientIDs) --
//     otherwise it is rejected here, before ever reaching the service layer,
//     since an unchecked caller-supplied actorEmail would let any caller
//     claim to be any user.
//
// The two are mutually exclusive: a request carrying both a real
// x-user-id-token and an ActorEmail is rejected as a bad request rather than
// silently preferring one.
func (h *CaseHandler) CreateCaseComment(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req domain.CreateCaseCommentRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = caseID

	hasToken := middleware.UserIDTokenFromContext(r.Context()) != ""

	var (
		resp domain.CreateCaseCommentResponse
		err  error
	)
	switch {
	case req.ActorEmail != nil && hasToken:
		// A token-resolved caller should never also be claiming an explicit
		// actor -- treat as a caller error rather than picking one silently.
		writeServiceError(w, r, &apierror.ValidationError{
			Msg: "actorEmail must not be supplied alongside an x-user-id-token",
		})
		return

	case req.ActorEmail != nil:
		if clientID := auth.IdentityFromContext(r.Context()).ClientID; clientID == "" || !h.m2mClientIDs[clientID] {
			writeServiceError(w, r, &apierror.ForbiddenError{
				Msg: "caller is not an authorized M2M client",
			})
			return
		}
		actorEmail := strings.ToLower(strings.TrimSpace(*req.ActorEmail))
		resp, err = h.svc.CreateCaseCommentAs(r.Context(), req, actorEmail)

	default:
		// Unchanged from before ActorEmail existed: no token means
		// CaseService.CreateCaseComment itself returns 401 via resolveActor.
		resp, err = h.svc.CreateCaseComment(r.Context(), req)
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseComments handles POST /cases/{id}/comments/search.
func (h *CaseHandler) SearchCaseComments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCaseCommentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.SearchCaseComments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseActivities handles POST /cases/{id}/activities/search.
func (h *CaseHandler) SearchCaseActivities(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCaseActivitiesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, err := h.svc.SearchCaseActivities(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCases handles POST /cases/search.
func (h *CaseHandler) SearchCases(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCases(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// AggregateCases handles POST /cases/aggregate.
func (h *CaseHandler) AggregateCases(w http.ResponseWriter, r *http.Request) {
	var req domain.AggregateCasesRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.AggregateCases(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CaseVolumeTrend handles POST /cases/volume-trend.
func (h *CaseHandler) CaseVolumeTrend(w http.ResponseWriter, r *http.Request) {
	var req domain.CaseVolumeTrendRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CaseVolumeTrend(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCaseAttachments handles POST /attachments/search.
func (h *CaseHandler) SearchCaseAttachments(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAttachmentsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchCaseAttachments(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetCaseFeedback handles GET /cases/{id}/feedback.
func (h *CaseHandler) GetCaseFeedback(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetCaseFeedback(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SubmitCaseFeedback handles POST /cases/{id}/feedback.
func (h *CaseHandler) SubmitCaseFeedback(w http.ResponseWriter, r *http.Request) {
	var req domain.SubmitCaseFeedbackRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SubmitCaseFeedback(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// AddCaseTag handles POST /cases/{id}/tags. It has two callers:
//
//   - A token-resolved caller (the normal case): no ActorEmail in the body,
//     the acting user is resolved from x-user-id-token inside
//     CaseService.AddCaseTag itself, exactly as before this M2M path
//     existed.
//   - An M2M caller with no x-user-id-token to resolve an actor from (e.g.
//     UMT via csm-integration-service): ActorEmail is set instead, and is
//     honored only when the caller's own x-jwt-assertion names a client id
//     in the trusted M2M set (h.m2mClientIDs, config.Config.M2MClientIDs) --
//     otherwise it is rejected here, before ever reaching the service layer,
//     since an unchecked caller-supplied actorEmail would let any caller
//     claim to be any user.
//
// The two are mutually exclusive: a request carrying both a real
// x-user-id-token and an ActorEmail is rejected as a bad request rather than
// silently preferring one.
func (h *CaseHandler) AddCaseTag(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")

	var req domain.AddCaseTagRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = caseID

	hasToken := middleware.UserIDTokenFromContext(r.Context()) != ""

	var (
		tag domain.Tag
		err error
	)
	switch {
	case req.ActorEmail != nil && hasToken:
		// A token-resolved caller should never also be claiming an explicit
		// actor -- treat as a caller error rather than picking one silently.
		writeServiceError(w, r, &apierror.ValidationError{
			Msg: "actorEmail must not be supplied alongside an x-user-id-token",
		})
		return

	case req.ActorEmail != nil:
		if clientID := auth.IdentityFromContext(r.Context()).ClientID; clientID == "" || !h.m2mClientIDs[clientID] {
			writeServiceError(w, r, &apierror.ForbiddenError{
				Msg: "caller is not an authorized M2M client",
			})
			return
		}
		actorEmail := strings.ToLower(strings.TrimSpace(*req.ActorEmail))
		tag, err = h.svc.AddCaseTagAs(r.Context(), caseID, req.Label, actorEmail)

	default:
		// Unchanged from before ActorEmail existed: no token means
		// CaseService.AddCaseTag itself returns 401 via resolveActor.
		tag, err = h.svc.AddCaseTag(r.Context(), caseID, req.Label)
	}
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tag)
}

// RemoveCaseTag handles DELETE /cases/{id}/tags/{tagId}.
func (h *CaseHandler) RemoveCaseTag(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	tagID := r.PathValue("tagId")

	if err := h.svc.RemoveCaseTag(r.Context(), caseID, tagID); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SearchTags handles POST /tags/search. Not scoped to a single case; used
// for FE autocomplete when attaching a tag.
func (h *CaseHandler) SearchTags(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchTagsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	h.searchTags(w, r, req)
}

// SearchTagsQuery handles GET /tags/search?q={query}&limit={limit}, the
// query-parameter form tag search used before it moved to a JSON body.
//
// Deprecated: use POST /tags/search instead. This alias exists only so the
// services in front of this one can be rolled out over the following release
// instead of having to deploy in lockstep with it; a caller still on the GET
// would otherwise get a 405. Delete it once that release has shipped.
func (h *CaseHandler) SearchTagsQuery(w http.ResponseWriter, r *http.Request) {
	req := domain.SearchTagsRequest{
		Filters: domain.SearchTagsFilters{SearchQuery: r.URL.Query().Get("q")},
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
			return
		}
		req.Limit = parsed
	}
	h.searchTags(w, r, req)
}

// searchTags is the single path both /tags/search forms run through: validation,
// the service call, and the response envelope all live here so the deprecated
// GET alias cannot drift from the POST.
func (h *CaseHandler) searchTags(w http.ResponseWriter, r *http.Request, req domain.SearchTagsRequest) {
	if req.Limit < 0 {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
		return
	}
	if req.Limit > maxTagSearchLimit {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must not exceed 100"})
		return
	}

	tags, err := h.svc.SearchTags(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]domain.Tag{"tags": tags})
}
