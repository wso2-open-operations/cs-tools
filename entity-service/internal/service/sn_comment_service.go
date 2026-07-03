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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

type snCommentSearchService struct {
	client *integrationservice.Client
}

// NewServiceNowCommentService constructs a CommentService backed by the Choreo API.
func NewServiceNowCommentService(client *integrationservice.Client) CommentService {
	return &snCommentSearchService{client: client}
}

func (s *snCommentSearchService) SearchComments(ctx context.Context, req domain.SearchCommentsRequest) (domain.SearchCommentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	if req.ReferenceID == "" {
		return domain.SearchCommentsResponse{}, &apierror.ValidationError{Msg: "referenceId is required"}
	}
	if _, ok := validReferenceTypes[req.ReferenceType]; !ok {
		return domain.SearchCommentsResponse{}, &apierror.ValidationError{Msg: "referenceType must be one of: case, conversation, change_request, deployment"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCommentsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchCommentsPayload{
		ReferenceID:   uuidToSysid(req.ReferenceID),
		ReferenceType: string(req.ReferenceType),
		Pagination:    snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	if req.Filters != nil && req.Filters.Type != nil {
		snType, ok := snCommentTypeMap[*req.Filters.Type]
		if !ok {
			return domain.SearchCommentsResponse{}, &apierror.ValidationError{Msg: "filters.type must be one of: comment, work_notes, activity"}
		}
		payload.Filters = &snCommentFilters{Type: snType}
	}

	raw, err := s.client.Post(ctx, "/comments/search", token, payload)
	if err != nil {
		return domain.SearchCommentsResponse{}, err
	}

	var snResp snSearchCommentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCommentsResponse{}, fmt.Errorf("sn search comments: parse response: %w", err)
	}

	comments := make([]domain.Comment, 0, len(snResp.Comments))
	for _, c := range snResp.Comments {
		createdAt, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
		if err != nil {
			slog.WarnContext(ctx, "sn search comments: skipping comment with unparsable createdOn",
				"commentID", c.ID, "createdOn", c.CreatedOn, "error", err)
			continue
		}
		comments = append(comments, domain.Comment{
			ID:                 sysidToUUID(c.ID),
			ReferenceID:        sysidToUUID(c.ReferenceID),
			Content:            c.Content,
			Type:               c.Type,
			CreatedOn:          createdAt,
			CreatedBy:          c.CreatedBy,
			CreatedByFirstName: c.CreatedByFirstName,
			CreatedByLastName:  c.CreatedByLastName,
			CreatedByFullName:  c.CreatedByFullName,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchCommentsResponse{
		Comments: comments,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(comments) < total,
	}, nil
}
