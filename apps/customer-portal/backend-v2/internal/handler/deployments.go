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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityDeploymentClient abstracts the entity-service deployment operations
// used by DeploymentHandler.
type entityDeploymentClient interface {
	SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error)
	CreateDeployment(ctx context.Context, req entity.CreateDeploymentRequest) (entity.CreateDeploymentResponse, error)
	UpdateDeployment(ctx context.Context, id string, req entity.UpdateDeploymentRequest) (entity.UpdateDeploymentResponse, error)
	UpdateAttachment(ctx context.Context, id string, req entity.UpdateAttachmentRequest) (entity.UpdateAttachmentResponse, error)
	SearchAttachments(ctx context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error)
	CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error)
}

// DeploymentHandler handles HTTP requests for deployment operations.
type DeploymentHandler struct {
	entity entityDeploymentClient
}

// NewDeploymentHandler creates a DeploymentHandler backed by the given entity client.
func NewDeploymentHandler(entity entityDeploymentClient) *DeploymentHandler {
	return &DeploymentHandler{entity: entity}
}

// SearchDeployments handles POST /projects/{id}/deployments/search.
func (h *DeploymentHandler) SearchDeployments(w http.ResponseWriter, r *http.Request) {
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

	var req entity.SearchDeploymentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	// ProjectIDs is always forced to the {id} path parameter, never the
	// client-supplied body: project scoping comes exclusively from the URL,
	// same reasoning as dto.BuildEntitySearchCasesRequest's projectID
	// parameter for POST /projects/{id}/cases/search.
	req.ProjectIDs = []string{projectID}

	result, err := h.entity.SearchDeployments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchDeployments(result))
}

// CreateDeployment handles POST /projects/{id}/deployments.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — a Postgres-mode deployment returns 400 for every call, which
// mapUpstreamError surfaces as ErrMsgBadRequest.
func (h *DeploymentHandler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
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

	var req dto.DeploymentCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateDeployment(r.Context(), dto.BuildEntityCreateDeploymentRequest(projectID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateDeployment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapDeploymentCreate(result))
}

// PatchDeployment handles PATCH /projects/{projectId}/deployments/{id}.
// projectId is read from the URL only to match the frontend's nesting — the
// handler never uses it, since entity-service's UpdateDeployment is keyed on
// the deployment's own id alone (same pattern as
// PATCH /cases/{caseId}/call-requests/{id} — see this backend's CLAUDE.md).
func (h *DeploymentHandler) PatchDeployment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("projectId")
	id := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) || id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// The route is /projects/{projectId}/deployments/{id}, but entity-service's
	// PATCH /deployments/{id} is keyed on the deployment alone and has no
	// project field to scope it with — unlike UpdateDeployedProduct, whose
	// request carries a DeploymentID the portal injects from the path for
	// exactly this purpose. Without a check here the path's project segment is
	// decorative: any deployment id would be updated under any project the
	// caller happens to name.
	//
	// So the pairing is verified first. The search is project-scoped, and
	// entity-service applies the caller's own access to it, which makes this
	// one call serve as both checks: a deployment in a project the caller
	// cannot reach does not come back, and neither does one that simply is not
	// in this project.
	ok, err := h.deploymentBelongsToProject(r.Context(), projectID, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployments failed", "userID", user.UserID, "projectID", projectID, "deploymentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update deployment.")
		return
	}
	if !ok {
		// Deliberately 404, not 403: "exists but is not yours" and "does not
		// exist" must be indistinguishable, or this becomes an oracle for
		// enumerating other customers' deployment ids.
		writeError(w, http.StatusNotFound, "Deployment not found.")
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var portalReq dto.DeploymentUpdateRequest
	if err := json.Unmarshal(body, &portalReq); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req := dto.BuildEntityUpdateDeploymentRequest(id, portalReq)
	// entity-service requires exactly one of the detail-fields group
	// (name/type/description) or active=false — never both, never neither.
	detailFieldsSet := req.Name != nil || req.Type != nil || len(req.Description) > 0
	activeSet := req.Active != nil
	if detailFieldsSet == activeSet {
		writeError(w, http.StatusBadRequest, "Provide either name/type/description or active, but not both.")
		return
	}
	if activeSet && *req.Active {
		writeError(w, http.StatusBadRequest, "active can only be set to false.")
		return
	}

	result, err := h.entity.UpdateDeployment(r.Context(), id, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateDeployment failed", "userID", user.UserID, "deploymentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update deployment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeploymentUpdate(result))
}

// deploymentScopeCheckPageLimit is the page size used when confirming a
// deployment belongs to a project. entity-service's SearchDeployments caps a
// search limit at 50 (the shared normalizePagination's maxLimit, not the 100
// this constant assumed until a live request 400'd with "limit cannot exceed
// 50") -- asking for more is rejected outright, not clamped.
const deploymentScopeCheckPageLimit = 50

// deploymentScopeCheckMaxPages bounds that walk. A project with more than
// 10,000 deployments does not exist in practice, and an unbounded loop driven
// by an upstream total is a denial-of-service waiting to happen — a wrong or
// hostile Total would otherwise keep this handler paging indefinitely. Kept
// at 200 (not the previous 100) so halving the page size still covers the
// same ~10,000-deployment bound.
const deploymentScopeCheckMaxPages = 200

// deploymentBelongsToProject reports whether deploymentID is one of projectID's
// deployments, as seen by the calling user.
//
// entity-service exposes no deployment get-by-id, so this is the only way to
// establish the pairing: search the project's deployments and look for the id.
// The search is evaluated with the caller's own access upstream, so a false
// result covers both "not in this project" and "not visible to you" — the
// caller must not be able to tell those apart (see the call site).
func (h *DeploymentHandler) deploymentBelongsToProject(ctx context.Context, projectID, deploymentID string) (bool, error) {
	for page := 0; page < deploymentScopeCheckMaxPages; page++ {
		offset := page * deploymentScopeCheckPageLimit
		resp, err := h.entity.SearchDeployments(ctx, entity.SearchDeploymentsRequest{
			ProjectIDs: []string{projectID},
			Pagination: entity.Pagination{Limit: deploymentScopeCheckPageLimit, Offset: offset},
		})
		if err != nil {
			return false, err
		}
		for _, d := range resp.Deployments {
			if strings.EqualFold(d.ID, deploymentID) {
				return true, nil
			}
		}
		// Stop on an empty page as well as on HasMore, so a response that never
		// clears HasMore cannot spin this loop to its page cap.
		if len(resp.Deployments) == 0 || !resp.HasMore {
			return false, nil
		}
	}
	return false, nil
}

// PatchDeploymentAttachment handles
// PATCH /deployments/{deploymentId}/attachments/{attachmentId}. referenceId/
// referenceType are injected server-side (deploymentId path param,
// ReferenceTypeDeployment) — the client only supplies name/description.
func (h *DeploymentHandler) PatchDeploymentAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	attachmentID := r.PathValue("attachmentId")
	if !uuidRe.MatchString(deploymentID) || !uuidRe.MatchString(attachmentID) {
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

	entityReq := dto.BuildEntityUpdateAttachmentRequest(req, deploymentID, entity.ReferenceTypeDeployment)
	result, err := h.entity.UpdateAttachment(r.Context(), attachmentID, entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateAttachment failed", "userID", user.UserID, "attachmentID", attachmentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update the attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapUpdatedAttachment(result))
}

// SearchDeploymentAttachments handles GET /deployments/{deploymentId}/attachments.
//
// Deployment attachments are stored through entity-service's generic attachment
// API, keyed by reference — this is the deployment-scoped read of it, the exact
// counterpart of CaseHandler.SearchCaseAttachments. The reference type is forced
// to deployment and the reference ID comes from the path, never the query.
func (h *DeploymentHandler) SearchDeploymentAttachments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	if deploymentID == "" || !uuidRe.MatchString(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	limit, offset, ok := parseLimitOffset(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchAttachments(r.Context(), entity.SearchAttachmentsRequest{
		ReferenceID:   deploymentID,
		ReferenceType: entity.ReferenceTypeDeployment,
		Pagination:    entity.Pagination{Limit: limit, Offset: offset},
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAttachments failed", "userID", user.UserID, "deploymentID", deploymentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve deployment attachments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeploymentAttachments(result))
}

// CreateDeploymentAttachment handles POST /deployments/{deploymentId}/attachments.
func (h *DeploymentHandler) CreateDeploymentAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	if deploymentID == "" || !uuidRe.MatchString(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.CreateDeploymentAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateAttachment(r.Context(), dto.BuildEntityCreateDeploymentAttachmentRequest(deploymentID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAttachment failed", "userID", user.UserID, "deploymentID", deploymentID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployment attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapAttachmentCreate(result))
}
