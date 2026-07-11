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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snProblemsResponse mirrors the Choreo POST /problems/search response.
type snProblemsResponse struct {
	Problems     []snProblem `json:"problems"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

type snProblem struct {
	ID      *string `json:"id"`
	Number  *string `json:"number"`
	Subject *string `json:"subject"`
}

// snProblemSearchPayload is the Choreo POST /problems/search request body.
type snProblemSearchPayload struct {
	Filters    snProblemFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProblemFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snProblemService struct {
	client *integrationservice.Client
}

// NewServiceNowProblemService constructs a ProblemService backed by the Choreo API.
func NewServiceNowProblemService(client *integrationservice.Client) ProblemService {
	return &snProblemService{client: client}
}

func (s *snProblemService) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchProblemsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snProblemSearchPayload{
		Filters:    snProblemFilters{SearchQuery: req.Filters.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/problems/search", token, payload)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	var snResp snProblemsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProblemsResponse{}, fmt.Errorf("sn problems: parse response: %w", err)
	}

	views := make([]domain.SearchProblemView, 0, len(snResp.Problems))
	for _, p := range snResp.Problems {
		view := domain.SearchProblemView{
			Subject: p.Subject,
			Number:  p.Number,
		}
		if p.ID != nil && *p.ID != "" {
			id := sysidToUUID(*p.ID)
			view.ID = &id
		}
		views = append(views, view)
	}

	return domain.SearchProblemsResponse{
		Problems: views,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}
