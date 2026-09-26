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

type knowledgeBaseService struct {
	repo repository.KnowledgeBaseRepository
}

// NewKnowledgeBaseService constructs a KnowledgeBaseService backed by the given repository.
func NewKnowledgeBaseService(repo repository.KnowledgeBaseRepository) KnowledgeBaseService {
	return &knowledgeBaseService{repo: repo}
}

// ListKnowledgeBases implements KnowledgeBaseService.
func (s *knowledgeBaseService) ListKnowledgeBases(ctx context.Context) (domain.ListKnowledgeBasesResponse, error) {
	kbs, err := s.repo.ListKnowledgeBases(ctx)
	if err != nil {
		return domain.ListKnowledgeBasesResponse{}, err
	}
	return domain.ListKnowledgeBasesResponse{KnowledgeBases: kbs}, nil
}

// CreateKnowledgeBase implements KnowledgeBaseService.
func (s *knowledgeBaseService) CreateKnowledgeBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	if req.Name == "" {
		return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "name is required"}
	}
	return s.repo.CreateKnowledgeBase(ctx, req)
}

// UpdateKnowledgeBaseName implements KnowledgeBaseService.
func (s *knowledgeBaseService) UpdateKnowledgeBaseName(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	if id == "" {
		return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "id is required"}
	}
	if req.Name == "" {
		return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "name is required"}
	}
	return s.repo.UpdateKnowledgeBaseName(ctx, id, req)
}

// SetKnowledgeBaseActive implements KnowledgeBaseService.
func (s *knowledgeBaseService) SetKnowledgeBaseActive(ctx context.Context, id string, req domain.UpdateKnowledgeBaseActiveRequest) (domain.KnowledgeBase, error) {
	if id == "" {
		return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.SetKnowledgeBaseActive(ctx, id, req)
}
