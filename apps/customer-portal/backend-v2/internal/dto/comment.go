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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// CommentCreateRequest is the portal's request shape for POST /comments.
// entity-service also supports "work_note" and "activity" comment types, but
// those are internal WSO2 support annotations — the customer portal only
// ever lets a customer post a plain "comment"; see BuildEntityCreateCommentRequest.
type CommentCreateRequest struct {
	ReferenceID   string `json:"referenceId"`
	ReferenceType string `json:"referenceType"`
	Content       string `json:"content"`
}

// BuildEntityCreateCommentRequest converts the portal's comment request into
// entity-service's request shape, forcing Type to "comment" regardless of
// anything the caller might otherwise try to set.
func BuildEntityCreateCommentRequest(req CommentCreateRequest) entity.CreateCommentRequest {
	return entity.CreateCommentRequest{
		ReferenceID:   req.ReferenceID,
		ReferenceType: entity.ReferenceType(req.ReferenceType),
		Type:          entity.CommentTypeComment,
		Content:       req.Content,
	}
}

// CommentCreateResponse is the portal's response for POST /comments.
type CommentCreateResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// MapCommentCreate builds the portal response from entity-service's CreateCommentResponse.
func MapCommentCreate(r entity.CreateCommentResponse) CommentCreateResponse {
	return CommentCreateResponse{
		ID:        r.Comment.ID,
		CreatedOn: r.Comment.CreatedOn,
		CreatedBy: userRefDisplayName(r.Comment.CreatedBy),
	}
}

// CommentSearchRequest is the portal's request shape for POST /comments/search.
// entity-service's Filters.Type is deliberately not exposed here — it would
// let a caller fetch "work_note" entries (internal WSO2 support annotations
// on the same reference), which must never reach the customer portal
// frontend; see BuildEntitySearchCommentsRequest.
type CommentSearchRequest struct {
	ReferenceID   string            `json:"referenceId"`
	ReferenceType string            `json:"referenceType"`
	Pagination    entity.Pagination `json:"pagination"`
}

// BuildEntitySearchCommentsRequest converts the portal's search request into
// entity-service's request shape, always filtering to Type "comment".
func BuildEntitySearchCommentsRequest(req CommentSearchRequest) entity.SearchCommentsRequest {
	commentType := entity.CommentTypeComment
	return entity.SearchCommentsRequest{
		ReferenceID:   req.ReferenceID,
		ReferenceType: entity.ReferenceType(req.ReferenceType),
		Pagination:    req.Pagination,
		Filters:       &entity.CommentFilters{Type: &commentType},
	}
}

// CommentView is one item of the portal's response for POST /comments/search
// (also shared by GET /conversations/{id}/messages, which decodes the same
// shape as its ConversationMessage type). Type and CreatedByFirstName/
// CreatedByLastName are read by the chat-history view to distinguish the
// AI's replies from the customer's own messages and to build a display
// name — see apps/customer-portal/webapp/src/features/support/pages/
// ConversationDetailsPage.tsx.
type CommentView struct {
	ID                 string    `json:"id"`
	Content            string    `json:"content"`
	Type               string    `json:"type,omitempty"`
	CreatedOn          time.Time `json:"createdOn"`
	CreatedBy          string    `json:"createdBy"`
	CreatedByFirstName string    `json:"createdByFirstName,omitempty"`
	CreatedByLastName  string    `json:"createdByLastName,omitempty"`
	// Inline images in the comment body. The frontend declares both on its
	// CaseCommentInlineAttachment type (features/support/types/attachments.ts)
	// and renders them in case and conversation threads.
	HasInlineAttachments bool                      `json:"hasInlineAttachments,omitempty"`
	InlineAttachments    []CommentInlineAttachment `json:"inlineAttachments,omitempty"`
}

// SearchCommentsResponse is the portal's response for POST /comments/search
// and GET /conversations/{id}/messages. TotalRecords (not Total) to match
// the frontend's shared pagination envelope — useGetConversationMessages.ts's
// getNextPageParam reads totalRecords to decide whether more pages remain.
type SearchCommentsResponse struct {
	Comments     []CommentView `json:"comments"`
	TotalRecords int           `json:"totalRecords"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	HasMore      bool          `json:"hasMore"`
}

// CommentInlineAttachment is an image embedded in a comment body. Field names
// match the frontend's CaseCommentInlineAttachment exactly.
type CommentInlineAttachment struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	DownloadURL string `json:"downloadUrl"`
	// Nil when the upstream had no parseable timestamp; omitted rather than
	// serialised as a year-one date.
	CreatedOn *time.Time `json:"createdOn,omitempty"`
	CreatedBy string     `json:"createdBy"`
}

// mapCommentInlineAttachments converts entity-service's inline attachments to
// the portal shape. Returns nil (omitted) rather than an empty slice when the
// comment has none, so a plain comment does not grow an empty array.
func mapCommentInlineAttachments(in []entity.InlineAttachment) []CommentInlineAttachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]CommentInlineAttachment, 0, len(in))
	for _, a := range in {
		out = append(out, CommentInlineAttachment{
			ID: a.ID, FileName: a.FileName, ContentType: a.ContentType,
			DownloadURL: a.DownloadURL, CreatedOn: a.CreatedOn, CreatedBy: a.CreatedBy,
		})
	}
	return out
}

// MapSearchComments builds the portal response from entity-service's SearchCommentsResponse.
func MapSearchComments(r entity.SearchCommentsResponse) SearchCommentsResponse {
	comments := make([]CommentView, 0, len(r.Comments))
	for _, c := range r.Comments {
		// Work notes (internal WSO2 support annotations) must never reach the customer portal.
		if c.Type == entity.CommentTypeWorkNote || string(c.Type) == "work_note" || string(c.Type) == "work_notes" {
			continue
		}
		comments = append(comments, CommentView{
			ID:        c.ID,
			Content:   c.Content,
			Type:      string(c.Type),
			CreatedOn: c.CreatedOn,
			// The display name, not the email. entity-service replaced its
			// CommentUserRef{firstName,lastName,fullName} with a single
			// UserReference{id,email,name}, and Name is the direct successor of
			// FullName — which is what this mapped before and what the portal
			// depends on: ConversationDetailsPage decides a message came from the
			// assistant with createdBy.toLowerCase() === "novera", and the
			// assistant's name is "Novera". Mapping the email here instead would
			// break that attribution for good.
			CreatedBy: userRefDisplayName(c.CreatedBy),
			// createdByFirstName/createdByLastName are no longer sent upstream —
			// the same change removed them. Left empty rather than split out of
			// Name: the frontend already falls back to createdBy for the display
			// label, which now carries the full name, so nothing has to be guessed.
			HasInlineAttachments: c.HasInlineAttachments,
			InlineAttachments:    mapCommentInlineAttachments(c.InlineAttachments),
		})
	}
	return SearchCommentsResponse{
		Comments:     comments,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}
