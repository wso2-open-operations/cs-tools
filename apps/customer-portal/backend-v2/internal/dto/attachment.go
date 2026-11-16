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

// AttachmentSummary is one item of the portal's response for POST /attachments/search.
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

// SearchAttachmentsResponse is the portal's response for POST /attachments/search.
type SearchAttachmentsResponse struct {
	Attachments  []AttachmentSummary `json:"attachments"`
	TotalRecords int                 `json:"totalRecords"`
	Limit        int                 `json:"limit"`
	Offset       int                 `json:"offset"`
	HasMore      bool                `json:"hasMore"`
}

// MapSearchAttachments builds the portal response from entity-service's SearchAttachmentsResponse.
func MapSearchAttachments(r entity.SearchAttachmentsResponse) SearchAttachmentsResponse {
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
			CreatedBy:     attachmentCreatedByIdentity(a.CreatedBy),
			CreatedOn:     a.CreatedOn,
			DownloadURL:   a.DownloadURL,
			PreviewURL:    a.PreviewURL,
		})
	}
	return SearchAttachmentsResponse{
		Attachments:  items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
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

// attachmentCreatedByIdentity flattens entity-service's createdBy user object to
// the single string the portal contract exposes as createdBy. It returns the
// uploader's email, because the frontend decides whether the signed-in user may
// edit or delete an attachment by comparing createdBy (trimmed, lowercased) to
// that user's email, and the earlier portal backend passed the email through
// unchanged. It falls back to the resolved name only when the email is empty.
func attachmentCreatedByIdentity(u entity.UserRef) string {
	if e := strings.TrimSpace(u.Email); e != "" {
		return e
	}
	return strings.TrimSpace(u.Name)
}

// MapCaseAttachments builds the portal response from entity-service's
// SearchAttachmentsResponse for GET /cases/{id}/attachments.
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
			CreatedBy:     attachmentCreatedByIdentity(a.CreatedBy),
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
			CreatedBy:     attachmentCreatedByIdentity(a.CreatedBy),
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
