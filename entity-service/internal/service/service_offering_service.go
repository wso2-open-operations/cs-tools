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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type serviceOfferingService struct {
	repo repository.ServiceOfferingRepository
}

// NewServiceOfferingService constructs a ServiceOfferingService backed by Postgres.
func NewServiceOfferingService(repo repository.ServiceOfferingRepository) ServiceOfferingService {
	return &serviceOfferingService{repo: repo}
}

// SearchServiceOfferings implements ServiceOfferingService.
func (s *serviceOfferingService) SearchServiceOfferings(ctx context.Context, req domain.SearchServiceOfferingsRequest) (domain.SearchServiceOfferingsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchServiceOfferingsResponse{}, err
	}
	var serviceIDs []string
	searchQuery := ""
	if req.Filters != nil {
		if err := validateUUIDs("filters.serviceIds", req.Filters.ServiceIDs); err != nil {
			return domain.SearchServiceOfferingsResponse{}, err
		}
		if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
			return domain.SearchServiceOfferingsResponse{}, err
		}
		serviceIDs = req.Filters.ServiceIDs
		searchQuery = req.Filters.SearchQuery
	}

	offerings, total, err := s.repo.SearchServiceOfferings(ctx, serviceIDs, searchQuery, req.Pagination.Limit, req.Pagination.Offset)
	if err != nil {
		return domain.SearchServiceOfferingsResponse{}, err
	}

	return domain.SearchServiceOfferingsResponse{
		ServiceOfferings: offerings,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
	}, nil
}
