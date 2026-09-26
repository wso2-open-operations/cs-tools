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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type conversationService struct {
	repo   repository.ConversationRepository
	access AccessService
}

// NewConversationService constructs a ConversationService backed by Postgres.
func NewConversationService(repo repository.ConversationRepository, access AccessService) ConversationService {
	return &conversationService{repo: repo, access: access}
}

// scopeConversationProjectIDs narrows f.ProjectIDs to the caller's own
// registered projects -- see scopeChangeRequestProjectIDs
// (change_request_service.go) for the full reasoning, identical here:
// intersect rather than union, and an empty result must short-circuit at
// the call site rather than reach the repo, since conversationWhereClause
// treats a len-0 ProjectIDs slice as "no filter", not "no rows".
func scopeConversationProjectIDs(scope AccessScope, requested []string) []string {
	if scope.Unrestricted {
		return requested
	}
	if len(requested) == 0 {
		return scope.ProjectIDs
	}
	out := make([]string, 0, len(requested))
	for _, id := range requested {
		for _, allowed := range scope.ProjectIDs {
			if strings.EqualFold(id, allowed) {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

// authorizeConversationProject refuses a caller whose AccessScope doesn't
// include projectID -- used after a by-id fetch. project is nil for a
// conversation whose project was deleted (work_item.project_id is
// ON DELETE SET NULL) -- fails closed: with no project to check membership
// against, a non-unrestricted caller is refused rather than let through.
// Reported as NotFound, never Forbidden, matching authorizeProject: a 403
// would confirm the conversation exists to someone not entitled to know
// that.
func authorizeConversationProject(scope AccessScope, project *domain.EntityRef) error {
	if scope.Unrestricted {
		return nil
	}
	if project != nil {
		for _, id := range scope.ProjectIDs {
			if strings.EqualFold(id, project.ID) {
				return nil
			}
		}
	}
	return &apierror.NotFoundError{Msg: "conversation not found"}
}

// SearchConversations implements ConversationService.
func (s *conversationService) SearchConversations(ctx context.Context, req domain.SearchConversationsRequest) (domain.SearchConversationsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	if err := validateUUIDs("filters.projectIds", req.Filters.ProjectIDs); err != nil {
		return domain.SearchConversationsResponse{}, err
	}

	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.SearchConversationsResponse{}, err
	}
	req.Filters.ProjectIDs = scopeConversationProjectIDs(scope, req.Filters.ProjectIDs)
	if !scope.Unrestricted && len(req.Filters.ProjectIDs) == 0 {
		// See scopeConversationProjectIDs's own doc comment: an empty
		// ProjectIDs here must short-circuit, not reach the repo.
		return domain.SearchConversationsResponse{
			Conversations: []domain.SearchConversationView{},
			Limit:         req.Pagination.Limit,
			Offset:        req.Pagination.Offset,
		}, nil
	}

	// callerEmail is only needed to resolve filters.createdByMe -- a request
	// that doesn't set it works fine with no caller identity at all, so a
	// missing/unparsable token isn't an error here the way it is for a
	// write (e.g. UpdateConversation).
	var callerEmail string
	if req.Filters.CreatedByMe {
		token := middleware.UserIDTokenFromContext(ctx)
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.SearchConversationsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required to filter by createdByMe"}
		}
		callerEmail = email
	}

	views, total, err := s.repo.SearchConversations(ctx, req, callerEmail)
	if err != nil {
		return domain.SearchConversationsResponse{}, err
	}

	return domain.SearchConversationsResponse{
		Conversations: views,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

// GetConversation implements ConversationService.
func (s *conversationService) GetConversation(ctx context.Context, id string) (domain.ConversationDetails, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ConversationDetails{}, err
	}
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.ConversationDetails{}, err
	}
	d, err := s.repo.GetConversation(ctx, id)
	if err != nil {
		return domain.ConversationDetails{}, err
	}
	if err := authorizeConversationProject(scope, d.Project); err != nil {
		return domain.ConversationDetails{}, err
	}
	return d, nil
}

// CreateConversation is not supported for the PostgreSQL data source: like
// CaseRepository.CreateCase, work_item.number has no DB default and no
// backing sequence anywhere in migrations/.
func (s *conversationService) CreateConversation(_ context.Context, _ domain.CreateConversationRequest) (domain.CreateConversationResponse, error) {
	return domain.CreateConversationResponse{}, &apierror.ServiceUnavailableError{
		Msg: "creating a conversation is not available on this data source: work_item.number has no generation strategy defined here",
	}
}

// UpdateConversation implements ConversationService.
func (s *conversationService) UpdateConversation(ctx context.Context, id string, req domain.UpdateConversationRequest) (domain.UpdateConversationResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.UpdateConversationResponse{}, err
	}
	if !validConversationUpdateState[req.State] {
		return domain.UpdateConversationResponse{}, &apierror.ValidationError{Msg: "state contains invalid value: " + string(req.State)}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	callerEmail, err := emailFromJWT(token)
	if err != nil {
		return domain.UpdateConversationResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	// Scope is checked against the row's CURRENT project, before the update
	// applies -- not the response afterward. A caller outside the
	// conversation's project must never be able to mutate it in the first
	// place, even if the response would then correctly be withheld.
	scope, err := s.access.ResolveScope(ctx)
	if err != nil {
		return domain.UpdateConversationResponse{}, err
	}
	if !scope.Unrestricted {
		existing, err := s.repo.GetConversation(ctx, id)
		if err != nil {
			return domain.UpdateConversationResponse{}, err
		}
		if err := authorizeConversationProject(scope, existing.Project); err != nil {
			return domain.UpdateConversationResponse{}, err
		}
	}

	updated, err := s.repo.UpdateConversation(ctx, id, req.State, callerEmail)
	if err != nil {
		return domain.UpdateConversationResponse{}, err
	}

	return domain.UpdateConversationResponse{
		Message:      "Conversation updated successfully",
		Conversation: updated,
	}, nil
}
