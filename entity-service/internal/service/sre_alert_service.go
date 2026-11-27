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
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// SRE alert search paging: 20 by default, at most 100 a page.
const (
	sreAlertDefaultLimit = 20
	sreAlertMaxLimit     = 100
)

// Facets come from the last 30 days of alerts, at most 200 values each.
const (
	sreAlertFacetWindow = 30 * 24 * time.Hour
	sreAlertFacetLimit  = 200
)

// SreAlertService serves the portal's Alerts pages from the alert table.
type SreAlertService interface {
	Search(ctx context.Context, req domain.SearchSreAlertsRequest) (domain.SearchSreAlertsResponse, error)
	Get(ctx context.Context, idOrNumber string) (domain.SreAlertDetail, error)
	Facets(ctx context.Context) (domain.SreAlertFacets, error)
}

type sreAlertService struct {
	repo repository.SreAlertRepository
	now  func() time.Time
}

// NewSreAlertService returns the service behind /sre-alerts.
func NewSreAlertService(repo repository.SreAlertRepository) SreAlertService {
	return &sreAlertService{repo: repo, now: time.Now}
}

func (s *sreAlertService) Search(ctx context.Context, req domain.SearchSreAlertsRequest) (domain.SearchSreAlertsResponse, error) {
	f := req.Filters
	switch f.Incident {
	case "", domain.SreAlertIncidentAll, domain.SreAlertIncidentLinked, domain.SreAlertIncidentUnlinked:
	default:
		return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "filters.incident must be all, linked or unlinked"}
	}
	if f.IncidentID != "" && !validate.IsUUID(f.IncidentID) {
		return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "filters.incidentId must be a UUID"}
	}
	for _, t := range f.Tiers {
		if t != domain.AlertTierPriority && t != domain.AlertTierStandard {
			return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "filters.tiers must be priority or standard"}
		}
	}
	if f.ReceivedFrom != nil && f.ReceivedTo != nil && !f.ReceivedFrom.Before(*f.ReceivedTo) {
		return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "filters.receivedFrom must be before filters.receivedTo"}
	}
	sort := domain.SreAlertSort{Field: "receivedOn", Order: "desc"}
	if req.SortBy != nil {
		if req.SortBy.Field != "" {
			if !slices.Contains(domain.SreAlertSortFields, req.SortBy.Field) {
				return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "sortBy.field must be receivedOn, number, severity, environment or service"}
			}
			sort.Field = req.SortBy.Field
		}
		switch strings.ToLower(req.SortBy.Order) {
		case "":
		case "asc", "desc":
			sort.Order = strings.ToLower(req.SortBy.Order)
		default:
			return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "sortBy.order must be asc or desc"}
		}
	}
	offset, limit := 0, sreAlertDefaultLimit
	if p := req.Pagination; p != nil {
		if p.Offset < 0 || p.Limit < 0 {
			return domain.SearchSreAlertsResponse{}, &apierror.ValidationError{Msg: "pagination.offset and pagination.limit must not be negative"}
		}
		offset = p.Offset
		if p.Limit > 0 {
			limit = min(p.Limit, sreAlertMaxLimit)
		}
	}
	alerts, total, err := s.repo.Search(ctx, f, sort, offset, limit)
	if err != nil {
		return domain.SearchSreAlertsResponse{}, err
	}
	if alerts == nil {
		alerts = []domain.SreAlertListItem{}
	}
	return domain.SearchSreAlertsResponse{Alerts: alerts, Total: total, Offset: offset, Limit: limit}, nil
}

func (s *sreAlertService) Get(ctx context.Context, idOrNumber string) (domain.SreAlertDetail, error) {
	idOrNumber = strings.TrimSpace(idOrNumber)
	if idOrNumber == "" {
		return domain.SreAlertDetail{}, &apierror.ValidationError{Msg: "alert id is required"}
	}
	d, err := s.repo.Get(ctx, idOrNumber)
	if err != nil {
		return domain.SreAlertDetail{}, err
	}
	if d == nil {
		return domain.SreAlertDetail{}, &apierror.NotFoundError{Msg: "alert not found"}
	}
	return *d, nil
}

func (s *sreAlertService) Facets(ctx context.Context) (domain.SreAlertFacets, error) {
	return s.repo.Facets(ctx, s.now().Add(-sreAlertFacetWindow), sreAlertFacetLimit)
}
