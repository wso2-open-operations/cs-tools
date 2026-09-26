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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type kbManagerUserService struct {
	repo repository.KBManagerUserRepository
}

// NewKBManagerUserService constructs a KBManagerUserService backed by the given repository.
func NewKBManagerUserService(repo repository.KBManagerUserRepository) KBManagerUserService {
	return &kbManagerUserService{repo: repo}
}

// SearchKBManagerUsers implements KBManagerUserService.
func (s *kbManagerUserService) SearchKBManagerUsers(ctx context.Context, req domain.SearchKBManagerUsersRequest) (domain.SearchKBManagerUsersResponse, error) {
	managers, err := s.repo.SearchKBManagerUsers(ctx, req)
	if err != nil {
		return domain.SearchKBManagerUsersResponse{}, err
	}
	return domain.SearchKBManagerUsersResponse{Managers: managers}, nil
}

// CreateKBManagerUser implements KBManagerUserService. createdBy is the
// authenticated caller's identity -- NOT YET wired through from the
// handler layer (same open identity question as KB article authorship);
// callers must supply it explicitly until that's resolved.
func (s *kbManagerUserService) CreateKBManagerUser(ctx context.Context, req domain.CreateKBManagerUserRequest, createdBy string) (domain.KBManagerUser, error) {
	if req.KnowledgeBaseID == "" {
		return domain.KBManagerUser{}, &apierror.ValidationError{Msg: "knowledgeBaseId is required"}
	}
	if req.UserID == "" {
		return domain.KBManagerUser{}, &apierror.ValidationError{Msg: "userId is required"}
	}
	return s.repo.CreateKBManagerUser(ctx, req, createdBy)
}

// DeleteKBManagerUser implements KBManagerUserService.
func (s *kbManagerUserService) DeleteKBManagerUser(ctx context.Context, knowledgeBaseID, userID string) error {
	if knowledgeBaseID == "" || userID == "" {
		return &apierror.ValidationError{Msg: "knowledgeBaseId and userId are both required"}
	}
	return s.repo.DeleteKBManagerUser(ctx, knowledgeBaseID, userID)
}

type kbManagerGroupService struct {
	repo repository.KBManagerGroupRepository
}

// NewKBManagerGroupService constructs a KBManagerGroupService backed by the given repository.
func NewKBManagerGroupService(repo repository.KBManagerGroupRepository) KBManagerGroupService {
	return &kbManagerGroupService{repo: repo}
}

// SearchKBManagerGroups implements KBManagerGroupService.
func (s *kbManagerGroupService) SearchKBManagerGroups(ctx context.Context, req domain.SearchKBManagerGroupsRequest) (domain.SearchKBManagerGroupsResponse, error) {
	managers, err := s.repo.SearchKBManagerGroups(ctx, req)
	if err != nil {
		return domain.SearchKBManagerGroupsResponse{}, err
	}
	return domain.SearchKBManagerGroupsResponse{Managers: managers}, nil
}

// CreateKBManagerGroup implements KBManagerGroupService. createdBy -- see
// CreateKBManagerUser's doc comment above; same open question.
func (s *kbManagerGroupService) CreateKBManagerGroup(ctx context.Context, req domain.CreateKBManagerGroupRequest, createdBy string) (domain.KBManagerGroup, error) {
	if req.KnowledgeBaseID == "" {
		return domain.KBManagerGroup{}, &apierror.ValidationError{Msg: "knowledgeBaseId is required"}
	}
	if req.GroupID == "" {
		return domain.KBManagerGroup{}, &apierror.ValidationError{Msg: "groupId is required"}
	}
	return s.repo.CreateKBManagerGroup(ctx, req, createdBy)
}

// DeleteKBManagerGroup implements KBManagerGroupService.
func (s *kbManagerGroupService) DeleteKBManagerGroup(ctx context.Context, knowledgeBaseID, groupID string) error {
	if knowledgeBaseID == "" || groupID == "" {
		return &apierror.ValidationError{Msg: "knowledgeBaseId and groupId are both required"}
	}
	return s.repo.DeleteKBManagerGroup(ctx, knowledgeBaseID, groupID)
}
