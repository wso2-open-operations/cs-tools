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

// Package service is declared in interfaces.go.
package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type kbArticleService struct {
	repo repository.KBArticleRepository
}

// NewKBArticleService constructs a KBArticleService backed by the given repository.
func NewKBArticleService(repo repository.KBArticleRepository) KBArticleService {
	return &kbArticleService{repo: repo}
}

// CreateKBArticle implements KBArticleService.
func (s *kbArticleService) CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.CreateKBArticleResponse, error) {
	if req.KnowledgeBaseID == "" {
		return domain.CreateKBArticleResponse{}, &apierror.ValidationError{Msg: "knowledgeBaseId is required"}
	}
	if req.Title == "" {
		return domain.CreateKBArticleResponse{}, &apierror.ValidationError{Msg: "title is required"}
	}
	if req.Body == "" {
		return domain.CreateKBArticleResponse{}, &apierror.ValidationError{Msg: "body is required"}
	}
	if req.AuthorID == "" {
		return domain.CreateKBArticleResponse{}, &apierror.ValidationError{Msg: "authorId is required"}
	}

	article, err := s.repo.CreateKBArticle(ctx, req)
	if err != nil {
		return domain.CreateKBArticleResponse{}, err
	}
	return domain.CreateKBArticleResponse{Article: article}, nil
}

// GetKBArticle implements KBArticleService.
func (s *kbArticleService) GetKBArticle(ctx context.Context, id string) (domain.KBArticle, error) {
	if id == "" {
		return domain.KBArticle{}, &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.GetKBArticleByID(ctx, id)
}

// SearchKBArticles implements KBArticleService.
func (s *kbArticleService) SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) (domain.SearchKBArticlesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchKBArticlesResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchKBArticlesResponse{}, err
	}

	articles, total, err := s.repo.SearchKBArticles(ctx, req)
	if err != nil {
		return domain.SearchKBArticlesResponse{}, err
	}

	return domain.SearchKBArticlesResponse{
		Articles: articles,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(articles) < total,
	}, nil
}

// UpdateKBArticleState implements KBArticleService.
func (s *kbArticleService) UpdateKBArticleState(ctx context.Context, id string, req domain.UpdateKBArticleStateRequest) (domain.UpdateKBArticleStateResponse, error) {
	if id == "" {
		return domain.UpdateKBArticleStateResponse{}, &apierror.ValidationError{Msg: "id is required"}
	}
	switch req.State {
	case domain.KBArticleStateDraft, domain.KBArticleStatePendingReview,
		domain.KBArticleStatePublished, domain.KBArticleStateRetired:
		// valid
	default:
		return domain.UpdateKBArticleStateResponse{}, &apierror.ValidationError{Msg: "invalid state: " + string(req.State)}
	}

	article, err := s.repo.UpdateKBArticleState(ctx, id, req)
	if err != nil {
		return domain.UpdateKBArticleStateResponse{}, err
	}
	return domain.UpdateKBArticleStateResponse{Article: article}, nil
}
// UpdateKBArticleContent implements KBArticleService.
func (s *kbArticleService) UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error) {
	if id == "" {
		return domain.KBArticle{}, &apierror.ValidationError{Msg: "id is required"}
	}
	if req.Title == "" {
		return domain.KBArticle{}, &apierror.ValidationError{Msg: "title is required"}
	}
	return s.repo.UpdateKBArticleContent(ctx, id, req)
}

// DeleteKBArticle implements KBArticleService.
func (s *kbArticleService) DeleteKBArticle(ctx context.Context, id string) error {
	if id == "" {
		return &apierror.ValidationError{Msg: "id is required"}
	}
	return s.repo.DeleteKBArticle(ctx, id)
}

// ListKBArticleHistory implements KBArticleService.
func (s *kbArticleService) ListKBArticleHistory(ctx context.Context, kbArticleID string) (domain.ListKBArticleHistoryResponse, error) {
	if kbArticleID == "" {
		return domain.ListKBArticleHistoryResponse{}, &apierror.ValidationError{Msg: "id is required"}
	}
	history, err := s.repo.ListKBArticleHistory(ctx, kbArticleID)
	if err != nil {
		return domain.ListKBArticleHistoryResponse{}, err
	}
	return domain.ListKBArticleHistoryResponse{History: history}, nil
}
