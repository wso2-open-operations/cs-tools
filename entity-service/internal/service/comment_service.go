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

package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// commentTypeToEnum maps the CommentType values this Postgres data source can
// represent to comment_type_enum's labels (migration 000037: APPROVAL_HISTORY,
// COMMENT, WORK_NOTE). CommentTypeActivity maps to APPROVAL_HISTORY for
// filtering/read-back, but CreateComment refuses to write one: APPROVAL_HISTORY
// only ever arises from ServiceNow's own audit trail, never from a
// caller-authored comment.
var commentTypeToEnum = map[domain.CommentType]string{
	domain.CommentTypeComment:  "COMMENT",
	domain.CommentTypeWorkNote: "WORK_NOTE",
	domain.CommentTypeActivity: "APPROVAL_HISTORY",
}

// commentCreatedByAgent is CreateCommentRequest.CreatedBy's one legitimate
// override value -- see that field's own doc comment. CreateComment refuses
// any other non-empty value rather than trusting it at face value.
const commentCreatedByAgent = "agent"

// commentEnumToType is commentTypeToEnum's read-back inverse.
var commentEnumToType = map[string]domain.CommentType{
	"COMMENT":          domain.CommentTypeComment,
	"WORK_NOTE":        domain.CommentTypeWorkNote,
	"APPROVAL_HISTORY": domain.CommentTypeActivity,
}

type commentService struct {
	repo     repository.CommentRepository
	userRepo repository.UserRepository
}

// NewCommentService constructs a CommentService backed by Postgres.
func NewCommentService(repo repository.CommentRepository, userRepo repository.UserRepository) CommentService {
	return &commentService{repo: repo, userRepo: userRepo}
}

func commentRowToDomain(row repository.CommentRow) domain.Comment {
	c := domain.Comment{
		ID:          row.ID,
		ReferenceID: row.WorkItemID,
		Content:     row.Content,
		CreatedOn:   row.CreatedOn,
		// comment.created_by (migration 000037) is a free-text VARCHAR, not a
		// foreign key into "user" -- it mirrors ServiceNow's sys_journal_field
		// author string, which can be an integration/automation account with
		// no local user row. This data source writes the resolved caller's
		// email into it (see CreateComment below), so read it back as an
		// email for symmetry; there is no id to attach.
		CreatedBy: domain.NewUserReference("", row.CreatedBy, ""),
	}
	if row.Type != nil {
		if t, ok := commentEnumToType[*row.Type]; ok {
			c.Type = t
		}
	}
	return c
}

// SearchComments implements CommentService.
func (s *commentService) SearchComments(ctx context.Context, req domain.SearchCommentsRequest) (domain.SearchCommentsResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.SearchCommentsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	var typeFilter *string
	if req.Filters != nil && req.Filters.Type != nil {
		enumType, ok := commentTypeToEnum[*req.Filters.Type]
		if !ok {
			return domain.SearchCommentsResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(*req.Filters.Type)}
		}
		typeFilter = &enumType
	}

	rows, total, err := s.repo.SearchComments(ctx, req.ReferenceID, req.ReferenceType, typeFilter, req.Pagination)
	if err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	comments := make([]domain.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, commentRowToDomain(row))
	}

	return domain.SearchCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}

// CreateComment implements CommentService.
func (s *commentService) CreateComment(ctx context.Context, req domain.CreateCommentRequest) (domain.CreateCommentResponse, error) {
	if err := validateUUIDs("referenceId", []string{req.ReferenceID}); err != nil {
		return domain.CreateCommentResponse{}, err
	}
	enumType, ok := commentTypeToEnum[req.Type]
	if !ok || req.Type == domain.CommentTypeActivity {
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "type contains invalid value: " + string(req.Type)}
	}
	if req.Content == "" {
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "content is required"}
	}

	// req.CreatedBy is a caller-supplied override, documented on
	// CreateCommentRequest as existing solely to attribute an AI-assistant
	// reply to the assistant rather than to the customer whose token relayed
	// it -- its only legitimate value is the "agent" sentinel. Anything else
	// is refused rather than trusted at face value: honoring an arbitrary
	// caller-supplied string here would let any caller attribute a comment
	// to anyone (identity spoofing), since comment.created_by is a free-text
	// column with no server-side verification of who it names. Every
	// ordinary call (no override) always has createdBy derived from the
	// caller's own authenticated token, never from the request body.
	createdBy := ""
	switch req.CreatedBy {
	case "":
		// fall through to token resolution below
	case commentCreatedByAgent:
		createdBy = commentCreatedByAgent
	default:
		return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: `createdBy must be "agent" or omitted`}
	}
	if createdBy == "" {
		token := middleware.UserIDTokenFromContext(ctx)
		if token == "" {
			return domain.CreateCommentResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
		}
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.CreateCommentResponse{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		user, err := s.userRepo.GetUserByEmail(ctx, email)
		if err != nil {
			return domain.CreateCommentResponse{}, err
		}
		createdBy = user.Email
	}

	row, err := s.repo.CreateComment(ctx, req.ReferenceID, req.ReferenceType, enumType, req.Content, createdBy)
	if err != nil {
		return domain.CreateCommentResponse{}, err
	}

	return domain.CreateCommentResponse{
		Message: "Comment created successfully",
		Comment: domain.CaseCommentDetail{
			ID:        row.ID,
			CreatedOn: row.CreatedOn,
			CreatedBy: createdBy,
		},
	}, nil
}
