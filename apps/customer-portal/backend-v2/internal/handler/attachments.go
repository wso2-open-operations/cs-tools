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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityAttachmentClient abstracts the entity-service attachment operations
// used by AttachmentHandler.
type entityAttachmentClient interface {
	CreateAttachment(ctx context.Context, req entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error)
	SearchAttachments(ctx context.Context, req entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error)
	GetAttachmentContent(ctx context.Context, id string) (body []byte, contentType string, err error)
	DeleteAttachment(ctx context.Context, id string) (entity.DeleteAttachmentResponse, error)
	GetAttachment(ctx context.Context, id string) (entity.AttachmentDetails, error)
	// GetCase backs both authorizeAttachmentAccess (every route in this file)
	// and DeleteAttachment's closed-case guard — this handler serves no case
	// route of its own (see caseIsClosed in cases.go).
	GetCase(ctx context.Context, id string) (entity.CaseView, error)
	// SearchDeployments backs authorizeAttachmentAccess's deployment branch
	// (see deploymentAttachmentIsVisible below).
	SearchDeployments(ctx context.Context, req entity.SearchDeploymentsRequest) (entity.SearchDeploymentsResponse, error)
}

// deploymentAttachmentIsVisible reports whether deploymentID is visible to
// the calling user, the same way DeploymentHandler.deploymentBelongsToProject
// does: entity-service's SearchDeployments is evaluated under the caller's
// own row-level-security scope (deployment has RLS, migration 0176), so a
// non-empty result already proves access — no second project lookup needed.
// Unlike deploymentBelongsToProject, this doesn't need to know the project
// up front: SearchDeployments' ids filter (entity-service's SearchDeploymentsRequest.IDs)
// resolves the single deployment directly, which is all an attachment's own
// ReferenceID ever carries.
func deploymentAttachmentIsVisible(ctx context.Context, client entityAttachmentClient, deploymentID string) (bool, error) {
	resp, err := client.SearchDeployments(ctx, entity.SearchDeploymentsRequest{
		IDs:        []string{deploymentID},
		Pagination: entity.Pagination{Limit: 1},
	})
	if err != nil {
		return false, err
	}
	return len(resp.Deployments) > 0, nil
}

// authorizeAttachmentAccess verifies the caller may see an attachment's
// underlying entity before GetAttachmentContent/GetAttachment/DeleteAttachment
// act on it, and returns the case view (zero-valued for a non-case reference)
// so DeleteAttachment's own closed-case guard can reuse it instead of a
// second lookup. None of those three routes is nested under a project/case
// path, so unlike every other authorization check in this backend there is
// no path segment to trust — the attachment's own opaque UUID is the only
// thing identifying the resource, and without this check any authenticated
// caller could read or delete any other customer's attachment just by
// guessing or observing its id.
//
// Branches on attachment.ReferenceType where it's actually populated:
// entity-service's SN-backed GetAttachmentByID doc comment says it's left
// nil unconditionally ("the upstream attachment-details response carries no
// reference type... callers must fail closed on it"), but that only holds
// for the plain DATA_SOURCE=servicenow path. Under
// DATA_SOURCE=postgres-servicenow-dual-write (what's actually live today),
// GetAttachmentByID resolves to the Postgres-native implementation
// (caseAttachmentDualWriteService embeds *caseService with no override),
// which does populate ReferenceType from the real column.
//
//   - ReferenceType == deployment: verified via deploymentAttachmentIsVisible
//     (RLS-scoped SearchDeployments by id) — the only reference type besides
//     case that's actually reachable from the Deployed tab today.
//   - Anything else (case, nil, or any other type this codebase has no
//     scoped ownership check for yet — conversation/change_request/incident):
//     falls back to the original GetCase-based check, which both verifies a
//     real case and fails closed on every type it can't tell apart from one
//     (see entity-service's own CLAUDE.md, "Where this is actually enforced").
func authorizeAttachmentAccess(ctx context.Context, client entityAttachmentClient, attachment entity.AttachmentDetails) (entity.CaseView, error) {
	if attachment.ReferenceID == "" {
		return entity.CaseView{}, &apierror.Error{StatusCode: http.StatusNotFound}
	}
	if attachment.ReferenceType != nil && *attachment.ReferenceType == entity.ReferenceTypeDeployment {
		visible, err := deploymentAttachmentIsVisible(ctx, client, attachment.ReferenceID)
		if err != nil {
			return entity.CaseView{}, err
		}
		if !visible {
			return entity.CaseView{}, &apierror.Error{StatusCode: http.StatusNotFound}
		}
		return entity.CaseView{}, nil
	}
	return client.GetCase(ctx, attachment.ReferenceID)
}

// AttachmentHandler handles HTTP requests for attachment operations.
type AttachmentHandler struct {
	entity entityAttachmentClient
}

// NewAttachmentHandler creates an AttachmentHandler backed by the given entity client.
func NewAttachmentHandler(entity entityAttachmentClient) *AttachmentHandler {
	return &AttachmentHandler{entity: entity}
}

// CreateAttachment handles POST /attachments.
func (h *AttachmentHandler) CreateAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBodyWithLimit(w, r, maxAttachmentBodyBytes)
	if !ok {
		return
	}

	var req entity.CreateAttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.CreateAttachment(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateAttachment failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create attachment.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapAttachmentCreate(result))
}

// SearchAttachments handles POST /attachments/search.
func (h *AttachmentHandler) SearchAttachments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req entity.SearchAttachmentsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchAttachments(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchAttachments failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search attachments.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchAttachments(result))
}

// GetAttachmentContent handles GET /attachments/{id}/content. The response is
// the raw file content, not JSON. Content-Disposition: attachment is always
// set (mirroring entity-service's own XSS mitigation for this endpoint) so
// browsers never render an attachment inline. Rejected with 404 unless the
// caller can see the attachment's own case (see authorizeAttachmentAccess).
func (h *AttachmentHandler) GetAttachmentContent(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	attachment, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}
	if _, err := authorizeAttachmentAccess(r.Context(), h.entity, attachment); err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}

	content, contentType, err := h.entity.GetAttachmentContent(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachmentContent failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to download attachment.")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content) // #nosec G705 -- Content-Type set from entity-service's own sanitized value; Content-Disposition forces download, never inline rendering
}

// DeleteAttachment handles DELETE /attachments/{id}. Rejected with 404 unless
// the caller can see the attachment's own case (see authorizeAttachmentAccess),
// and with 400 when that case is closed.
//
// The closed-case check here deliberately does NOT reuse caseIsClosed
// (used by CreateCaseAttachment/PatchCaseAttachment, both nested under an
// already-trusted /cases/{caseId}/... path) — caseIsClosed fails OPEN on an
// entity-service error, which is the right call there since it's a pure
// business-rule check layered on top of an already-authorized request. Here
// the GetCase call IS the authorization check (see authorizeAttachmentAccess)
// and must fail closed, so this reuses its already-fetched CaseView directly
// instead of a second, separately-failing-open lookup. A previous version of
// this handler only ran its closed-case check when a fetch of the attachment
// succeeded AND its referenceId was non-empty — silently skipping the check
// entirely otherwise, which is the same class of bug this rewrite closes:
// every path now either resolves a real case and checks its state, or denies
// outright.
func (h *AttachmentHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	attachment, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}
	caseView, err := authorizeAttachmentAccess(r.Context(), h.entity, attachment)
	if err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}
	if dto.IsCaseStateClosed(caseView.State) {
		slog.WarnContext(r.Context(), "rejected attachment delete on a closed case", "userID", user.UserID, "attachmentID", id, "caseID", attachment.ReferenceID)
		writeError(w, http.StatusBadRequest, ErrMsgCaseClosedForAttachmentDelete)
		return
	}

	result, err := h.entity.DeleteAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to delete attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeleteAttachment(result))
}

// GetAttachment handles GET /attachments/{id} — metadata plus base64-encoded
// content, distinct from GetAttachmentContent's raw binary stream. Rejected
// with 404 unless the caller can see the attachment's own case (see
// authorizeAttachmentAccess).
func (h *AttachmentHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !isAttachmentID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetAttachment(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetAttachment failed", "userID", user.UserID, "attachmentID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve attachment.")
		return
	}
	if _, err := authorizeAttachmentAccess(r.Context(), h.entity, result); err != nil {
		slog.WarnContext(r.Context(), "rejected attachment access outside caller's scope", "userID", user.UserID, "attachmentID", id)
		mapUpstreamError(w, err, "Failed to retrieve attachment.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapAttachmentDetails(result))
}
