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

package domain

import "time"

// CommentTemplateAppliesTo says which comment box entry a template is offered
// for: a public comment, an internal work note, or either.
type CommentTemplateAppliesTo string

const (
	CommentTemplateAppliesToComment  CommentTemplateAppliesTo = "COMMENT"
	CommentTemplateAppliesToWorkNote CommentTemplateAppliesTo = "WORK_NOTE"
	CommentTemplateAppliesToBoth     CommentTemplateAppliesTo = "BOTH"
)

// Valid reports whether a is one of the three accepted values.
func (a CommentTemplateAppliesTo) Valid() bool {
	switch a {
	case CommentTemplateAppliesToComment, CommentTemplateAppliesToWorkNote, CommentTemplateAppliesToBoth:
		return true
	}
	return false
}

// Limits on a case comment template.
const (
	MaxCaseCommentTemplateNameLength = 100
	// MaxCaseCommentTemplateBodyLength is in bytes: the body is the comment
	// box's HTML, so this bounds the stored size, not the visible text.
	MaxCaseCommentTemplateBodyLength = 20000
	MaxPersonalCaseCommentTemplates  = 200
	MaxSharedCaseCommentTemplates    = 500
)

// CaseCommentTemplate is a saved comment or work note text. Shared is true for
// a template every CSM portal user sees (no owner), false for the caller's
// own personal template.
type CaseCommentTemplate struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Body      string                   `json:"body"`
	AppliesTo CommentTemplateAppliesTo `json:"appliesTo"`
	Shared    bool                     `json:"shared"`
	CreatedOn time.Time                `json:"createdOn"`
	UpdatedOn time.Time                `json:"updatedOn"`
}

// CaseCommentTemplateList is GET /users/me/case-comment-templates: the
// caller's personal templates followed by the shared ones, each ordered by
// name.
type CaseCommentTemplateList struct {
	Templates []CaseCommentTemplate `json:"templates"`
}

// CreateCaseCommentTemplateRequest creates a personal or a shared template;
// which one is decided by the route, never by the body.
type CreateCaseCommentTemplateRequest struct {
	Name      string                   `json:"name"`
	Body      string                   `json:"body"`
	AppliesTo CommentTemplateAppliesTo `json:"appliesTo"`
}

// UpdateCaseCommentTemplateRequest changes any of a template's three fields;
// a nil field is left as it is. A template cannot move between personal and
// shared.
type UpdateCaseCommentTemplateRequest struct {
	Name      *string                   `json:"name,omitempty"`
	Body      *string                   `json:"body,omitempty"`
	AppliesTo *CommentTemplateAppliesTo `json:"appliesTo,omitempty"`
}
