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

type alertIncidentMappingService struct {
	repo repository.AlertIncidentMappingRepository
}

// NewAlertIncidentMappingService constructs an AlertIncidentMappingService
// backed by the given repository.
func NewAlertIncidentMappingService(repo repository.AlertIncidentMappingRepository) AlertIncidentMappingService {
	return &alertIncidentMappingService{repo: repo}
}

// CreateAlertIncidentMapping implements AlertIncidentMappingService.
func (s *alertIncidentMappingService) CreateAlertIncidentMapping(ctx context.Context, req domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error) {
	if req.AlertNumber == "" {
		return domain.AlertIncidentMappingView{}, &apierror.ValidationError{Msg: "alertNumber is required"}
	}
	if req.Source == "" {
		return domain.AlertIncidentMappingView{}, &apierror.ValidationError{Msg: "source is required"}
	}
	if req.AlertStatus == "" {
		return domain.AlertIncidentMappingView{}, &apierror.ValidationError{Msg: "alertStatus is required"}
	}
	if req.IncidentID == "" {
		return domain.AlertIncidentMappingView{}, &apierror.ValidationError{Msg: "incidentId is required"}
	}
	return s.repo.Create(ctx, req)
}

// LookupAlertIncidentMappings implements AlertIncidentMappingService.
func (s *alertIncidentMappingService) LookupAlertIncidentMappings(ctx context.Context, req domain.LookupAlertIncidentMappingsRequest) (domain.LookupAlertIncidentMappingsResponse, error) {
	if req.Source == "" {
		return domain.LookupAlertIncidentMappingsResponse{}, &apierror.ValidationError{Msg: "source is required"}
	}
	if req.UniqueIdentifier == "" {
		return domain.LookupAlertIncidentMappingsResponse{}, &apierror.ValidationError{Msg: "uniqueIdentifier is required"}
	}
	mappings, err := s.repo.Lookup(ctx, req.Source, req.UniqueIdentifier)
	if err != nil {
		return domain.LookupAlertIncidentMappingsResponse{}, err
	}
	return domain.LookupAlertIncidentMappingsResponse{Mappings: mappings}, nil
}
