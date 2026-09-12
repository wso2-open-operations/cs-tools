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

type kbManagerService struct {
	repo repository.KBManagerRepository
}

// NewKBManagerService constructs a KBManagerService backed by the given repository.
func NewKBManagerService(repo repository.KBManagerRepository) KBManagerService {
	return &kbManagerService{repo: repo}
}

// SearchKBManagers implements KBManagerService.
func (s *kbManagerService) SearchKBManagers(ctx context.Context, req domain.SearchKBManagersRequest) (domain.SearchKBManagersResponse, error) {
	managers, err := s.repo.SearchKBManagers(ctx, req)
	if err != nil {
		return domain.SearchKBManagersResponse{}, err
	}
	return domain.SearchKBManagersResponse{Managers: managers}, nil
}

// CreateKBManager implements KBManagerService.
func (s *kbManagerService) CreateKBManager(ctx context.Context, req domain.CreateKBManagerRequest) (domain.KBManager, error) {
	if req.KnowledgeBaseID == "" {
		return domain.KBManager{}, &apierror.ValidationError{Msg: "knowledgeBaseId is required"}
	}
	if req.UserID == "" {
		return domain.KBManager{}, &apierror.ValidationError{Msg: "userId is required"}
	}
	return s.repo.CreateKBManager(ctx, req)
}

// DeleteKBManager implements KBManagerService.
func (s *kbManagerService) DeleteKBManager(ctx context.Context, knowledgeBaseID, userID string) error {
	if knowledgeBaseID == "" || userID == "" {
		return &apierror.ValidationError{Msg: "knowledgeBaseId and userId are both required"}
	}
	return s.repo.DeleteKBManager(ctx, knowledgeBaseID, userID)
}

