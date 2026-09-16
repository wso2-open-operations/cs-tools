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

package dto

import (
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// CreateCaseAttachmentRequest is the portal's request body for
// POST /cases/{id}/attachments, matching the frontend's own
// PostCaseAttachmentRequest type
// (apps/customer-portal/webapp/src/features/support/types/attachments.ts)
// field-for-field. There's no referenceId field: the case is scoped by the
// {id} path parameter, never the body.
type CreateCaseAttachmentRequest struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Content     string  `json:"content"`
	Description *string `json:"description,omitempty"`
}

// attachmentFileDataURI builds the base64 data URI entity-service requires for an
// attachment upload.
//
// The frontend deliberately strips the "data:<mime>;base64," prefix before
// sending: UploadAttachmentModal.tsx reads the file with readAsDataURL then
// slices off everything up to the first comma, passing the MIME type separately
// as `type`. entity-service validates the opposite — `file` must start with
// "data:" and contain ";base64," — so forwarding the raw content fails with
// "file must be a base64 data URI (e.g. data:image/png;base64,...)".
//
// Reconciling that mismatch is the DTO layer's job: the frontend contract stays
// frozen and entity-service gets the shape it demands. A value that already looks
// like a data URI passes through untouched, so a caller sending the full URI
// keeps working.
func attachmentFileDataURI(mimeType, content string) string {
	if content == "" {
		return ""
	}
	if strings.HasPrefix(content, "data:") && strings.Contains(content, ";base64,") {
		return content
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return "data:" + mimeType + ";base64," + content
}

// BuildEntityCreateCaseAttachmentRequest translates the portal's request
// into entity-service's CreateAttachmentRequest, forcing ReferenceID (from
// the {id} path parameter) and ReferenceType to case, and renaming
// Content->File to match entity-service's own field name.
func BuildEntityCreateCaseAttachmentRequest(caseID string, req CreateCaseAttachmentRequest) entity.CreateAttachmentRequest {
	return entity.CreateAttachmentRequest{
		ReferenceID:   caseID,
		ReferenceType: entity.ReferenceTypeCase,
		Name:          req.Name,
		Type:          req.Type,
		File:          attachmentFileDataURI(req.Type, req.Content),
		Description:   req.Description,
	}
}

// AttachmentCreateResponse is the portal's response for POST /attachments.
type AttachmentCreateResponse struct {
	ID        string    `json:"id"`
	SizeBytes int       `json:"sizeBytes"`
	CreatedOn time.Time `json:"createdOn"`
	// DownloadURL is nil for a CSM-native (Postgres) data source attachment.
	// Always non-nil for ServiceNow-sourced attachments.
	DownloadURL *string `json:"downloadUrl"`
}

// MapAttachmentCreate builds the portal response from entity-service's CreateAttachmentResponse.
func MapAttachmentCreate(r entity.CreateAttachmentResponse) AttachmentCreateResponse {
	return AttachmentCreateResponse{
		ID:          r.Attachment.ID,
		SizeBytes:   r.Attachment.SizeBytes,
		CreatedOn:   r.Attachment.CreatedOn,
		DownloadURL: r.Attachment.DownloadURL,
	}
}

// AttachmentSummary is one item of an attachment list response (used by case and
// deployment attachment listings).
type AttachmentSummary struct {
	ID            string    `json:"id"`
	ReferenceID   string    `json:"referenceId"`
	ReferenceType string    `json:"referenceType"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	SizeBytes     int       `json:"sizeBytes"`
	Description   *string   `json:"description,omitempty"`
	CreatedBy     string    `json:"createdBy"`
	CreatedOn     time.Time `json:"createdOn"`
	DownloadURL   *string   `json:"downloadUrl,omitempty"`
	PreviewURL    *string   `json:"previewUrl,omitempty"`
}

// CaseAttachmentsResponse is the portal's response for
// GET /cases/{id}/attachments. A distinct type from SearchAttachmentsResponse
// (not just a reuse) because the frontend's pagination envelope for this
// specific endpoint uses totalRecords, not total — see
// PaginationResponse in apps/customer-portal/webapp/src/types/common.ts.
type CaseAttachmentsResponse struct {
	Attachments  []AttachmentSummary `json:"attachments"`
	Offset       int                 `json:"offset"`
	Limit        int                 `json:"limit"`
	TotalRecords int                 `json:"totalRecords"`
}

// MapCaseAttachments builds the portal response from entity-service's
// SearchAttachmentsResponse for GET /cases/{id}/attachments.
// attachmentCreatedByName flattens entity-service's createdBy user object to the
// single display string the portal contract exposes (the frontend's
// AuditMetadata types createdBy as `string | null` and renders it directly, e.g.
// "Uploaded by {createdBy}"). Prefers the resolved name and falls back to the
// email, since Name is omitempty upstream and absent when the data source could
// not resolve the uploader to a user record.
func attachmentCreatedByName(u entity.UserRef) string {
	if n := strings.TrimSpace(u.Name); n != "" {
		return n
	}
	return strings.TrimSpace(u.Email)
}

func MapCaseAttachments(r entity.SearchAttachmentsResponse) CaseAttachmentsResponse {
	items := make([]AttachmentSummary, 0, len(r.Attachments))
	for _, a := range r.Attachments {
		items = append(items, AttachmentSummary{
			ID:            a.ID,
			ReferenceID:   a.ReferenceID,
			ReferenceType: string(a.ReferenceType),
			Name:          a.Name,
			Type:          a.Type,
			SizeBytes:     a.SizeBytes,
			Description:   a.Description,
			CreatedBy:     attachmentCreatedByName(a.CreatedBy),
			CreatedOn:     a.CreatedOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
		})
	}
	return CaseAttachmentsResponse{
		Attachments:  items,
		Offset:       r.Offset,
		Limit:        r.Limit,
		TotalRecords: r.Total,
	}
}

// DeleteResponse is the portal's response for DELETE /attachments/{id}.
type DeleteResponse struct {
	Message string `json:"message"`
}

// MapDeleteAttachment builds the portal response from entity-service's DeleteAttachmentResponse.
func MapDeleteAttachment(r entity.DeleteAttachmentResponse) DeleteResponse {
	return DeleteResponse{Message: r.Message}
}

// AttachmentDetails is the portal's response for GET /attachments/{id} —
// metadata plus base64-encoded content. entity-service's response has no
// fields worth restricting here, so this is a direct passthrough shape kept
// as its own portal type purely for this package's "always map through dto"
// convention.
type AttachmentDetails struct {
	ID          string    `json:"id"`
	ReferenceID string    `json:"referenceId"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	SizeBytes   int       `json:"sizeBytes"`
	Description *string   `json:"description"`
	CreatedBy   string    `json:"createdBy"`
	CreatedOn   time.Time `json:"createdOn"`
	DownloadURL *string   `json:"downloadUrl"`
	PreviewURL  *string   `json:"previewUrl"`
	// Content is nil for a CSM-native (Postgres) data source attachment.
	// Always non-nil for ServiceNow-sourced attachments.
	Content *string `json:"content"`
}

// MapAttachmentDetails builds the portal response from entity-service's AttachmentDetails.
func MapAttachmentDetails(r entity.AttachmentDetails) AttachmentDetails {
	return AttachmentDetails{
		ID:          r.ID,
		ReferenceID: r.ReferenceID,
		Name:        r.Name,
		Type:        r.Type,
		SizeBytes:   r.SizeBytes,
		Description: r.Description,
		CreatedBy:   r.CreatedBy,
		CreatedOn:   r.CreatedOn,
		DownloadURL: r.DownloadURL,
		PreviewURL:  r.PreviewURL,
		Content:     r.Content,
	}
}

// AttachmentUpdateRequest is the portal's request body for the two
// reference-scoped attachment-update routes (PATCH
// /deployments/{deploymentId}/attachments/{attachmentId} and PATCH
// /cases/{caseId}/attachments/{attachmentId}). referenceId/referenceType are
// never client-supplied — each handler injects them from its own path
// params and the appropriate ReferenceType. The case-scoped route only ever
// reads Name (never Description) by design.
type AttachmentUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// BuildEntityUpdateAttachmentRequest builds entity-service's
// UpdateAttachmentRequest from the portal request plus the reference id/type
// the calling handler injects.
func BuildEntityUpdateAttachmentRequest(req AttachmentUpdateRequest, referenceID string, referenceType entity.ReferenceType) entity.UpdateAttachmentRequest {
	return entity.UpdateAttachmentRequest{
		ReferenceID:   referenceID,
		ReferenceType: referenceType,
		Name:          req.Name,
		Description:   req.Description,
	}
}

// UpdatedAttachment is the portal's response for the two attachment-update routes.
type UpdatedAttachment struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// MapUpdatedAttachment builds the portal response from entity-service's
// UpdateAttachmentResponse — a raw passthrough of the attachment field
// (both routes return response.attachment directly).
func MapUpdatedAttachment(r entity.UpdateAttachmentResponse) UpdatedAttachment {
	return UpdatedAttachment{
		ID:        r.Attachment.ID,
		UpdatedOn: r.Attachment.UpdatedOn,
		UpdatedBy: r.Attachment.UpdatedBy,
	}
}

// CreateDeploymentAttachmentRequest mirrors the frontend's
// PostDeploymentAttachmentRequest field-for-field. Like its case counterpart
// there is no referenceId field: the deployment is scoped by the
// {deploymentId} path parameter, never the body.
type CreateDeploymentAttachmentRequest struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Content     string  `json:"content"`
	Description *string `json:"description,omitempty"`
}

// BuildEntityCreateDeploymentAttachmentRequest translates the portal's request
// into entity-service's CreateAttachmentRequest, forcing ReferenceID (from the
// path) and ReferenceType to deployment, and renaming Content->File to match
// entity-service's own field name.
func BuildEntityCreateDeploymentAttachmentRequest(deploymentID string, req CreateDeploymentAttachmentRequest) entity.CreateAttachmentRequest {
	return entity.CreateAttachmentRequest{
		ReferenceID:   deploymentID,
		ReferenceType: entity.ReferenceTypeDeployment,
		Name:          req.Name,
		Type:          req.Type,
		File:          attachmentFileDataURI(req.Type, req.Content),
		Description:   req.Description,
	}
}

// DeploymentAttachmentsResponse is the portal's response for
// GET /deployments/{deploymentId}/attachments. TotalRecords (not Total) to
// match the frontend's shared pagination envelope, same as the case variant.
type DeploymentAttachmentsResponse struct {
	Attachments  []AttachmentSummary `json:"attachments"`
	Offset       int                 `json:"offset"`
	Limit        int                 `json:"limit"`
	TotalRecords int                 `json:"totalRecords"`
}

// MapDeploymentAttachments builds the portal response from entity-service's
// attachment search.
func MapDeploymentAttachments(r entity.SearchAttachmentsResponse) DeploymentAttachmentsResponse {
	items := make([]AttachmentSummary, 0, len(r.Attachments))
	for _, a := range r.Attachments {
		items = append(items, AttachmentSummary{
			ID:            a.ID,
			ReferenceID:   a.ReferenceID,
			ReferenceType: string(a.ReferenceType),
			Name:          a.Name,
			Type:          a.Type,
			SizeBytes:     a.SizeBytes,
			Description:   a.Description,
			CreatedBy:     attachmentCreatedByName(a.CreatedBy),
			CreatedOn:     a.CreatedOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
		})
	}
	return DeploymentAttachmentsResponse{
		Attachments:  items,
		Offset:       r.Offset,
		Limit:        r.Limit,
		TotalRecords: r.Total,
	}
}
