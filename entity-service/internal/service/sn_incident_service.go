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

// snIncidentsResponse mirrors the Choreo POST /incidents/search response.
type snIncidentsResponse struct {
	Incidents    []snIncident `json:"incidents"`
	TotalRecords int          `json:"totalRecords"`
	Offset       int          `json:"offset"`
	Limit        int          `json:"limit"`
}

type snIncident struct {
	ID              *string             `json:"id"`
	Number          *string             `json:"number"`
	OpenedOn        *string             `json:"openedOn"`
	Subject         *string             `json:"subject"`
	Caller          *snIncidentEntityRef `json:"caller"`
	Priority        *snIncidentIntLabel  `json:"priority"`
	State           *snIncidentIntLabel  `json:"state"`
	Category        *snIncidentStrLabel  `json:"category"`
	Parent          *snIncidentEntityRef `json:"parent"`
	AssignmentGroup *snIncidentEntityRef `json:"assignmentGroup"`
	AssignedTo      *snIncidentEntityRef `json:"assignedTo"`
	CreatedOn       string               `json:"createdOn"`
	CreatedBy       string               `json:"createdBy"`
	UpdatedOn       string               `json:"updatedOn"`
	UpdatedBy       string               `json:"updatedBy"`
}

type snIncidentEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snIncidentIntLabel struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type snIncidentStrLabel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snIncidentSearchPayload is the Choreo POST /incidents/search request body.
type snIncidentSearchPayload struct {
	Filters    snIncidentFilters   `json:"filters,omitempty"`
	SortBy     *snIncidentSort     `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snIncidentSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type snIncidentFilters struct {
	SearchQuery  string   `json:"searchQuery,omitempty"`
	PriorityKeys []int    `json:"priorityKeys,omitempty"` // SN expects int keys
	ParentIDs    []string `json:"parentIds,omitempty"`
}

// snIncidentPriorityKeyMap maps domain IncidentPriority enums to SN numeric priority keys.
var snIncidentPriorityKeyMap = map[domain.IncidentPriority]int{
	domain.IncidentPriorityCritical: 1,
	domain.IncidentPriorityHigh:     2,
	domain.IncidentPriorityModerate: 3,
	domain.IncidentPriorityLow:      4,
	domain.IncidentPriorityPlanning: 5,
}

// snIncidentPriorityLabelMap maps SN numeric priority keys to domain enum strings.
var snIncidentPriorityLabelMap = map[int]string{
	1: "CRITICAL",
	2: "HIGH",
	3: "MODERATE",
	4: "LOW",
	5: "PLANNING",
}

var validIncidentPriority = map[domain.IncidentPriority]bool{
	domain.IncidentPriorityCritical: true,
	domain.IncidentPriorityHigh:     true,
	domain.IncidentPriorityModerate: true,
	domain.IncidentPriorityLow:      true,
	domain.IncidentPriorityPlanning: true,
}

var snIncidentStateLabelMap = map[int]string{
	1: "NEW",
	2: "IN_PROGRESS",
	3: "ON_HOLD",
	6: "RESOLVED",
	7: "CLOSED",
	8: "CANCELLED",
}

var snIncidentCategoryLabelMap = map[string]string{
	"inquiry":              "INQUIRY",
	"service_interruption": "SERVICE_INTERRUPTION",
	"security":             "SECURITY",
}

var validIncidentSortField = map[domain.IncidentSortField]bool{
	domain.IncidentSortFieldCreatedOn: true,
	domain.IncidentSortFieldUpdatedOn: true,
	domain.IncidentSortFieldOpenedOn:  true,
}

var validIncidentSortOrder = map[domain.IncidentSortOrder]bool{
	domain.IncidentSortOrderAsc:  true,
	domain.IncidentSortOrderDesc: true,
}

type snIncidentService struct {
	client *integrationservice.Client
}

// NewServiceNowIncidentService constructs an IncidentService backed by the Choreo API.
func NewServiceNowIncidentService(client *integrationservice.Client) IncidentService {
	return &snIncidentService{client: client}
}

func (s *snIncidentService) SearchIncidents(ctx context.Context, req domain.SearchIncidentsRequest) (domain.SearchIncidentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}
	if req.SortBy.Field != "" && !validIncidentSortField[req.SortBy.Field] {
		return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validIncidentSortOrder[req.SortBy.Order] {
		return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}
	for _, p := range req.Filters.Priorities {
		if !validIncidentPriority[p] {
			return domain.SearchIncidentsResponse{}, &apierror.ValidationError{Msg: "priorities contains invalid value: " + string(p)}
		}
	}
	if err := validateUUIDs("parentIds", req.Filters.ParentIDs); err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchIncidentsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	var snSortBy *snIncidentSort
	if req.SortBy.Field != "" {
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snIncidentSort{Field: string(req.SortBy.Field), Order: order}
	}

	priorityKeys := make([]int, 0, len(req.Filters.Priorities))
	for _, p := range req.Filters.Priorities {
		priorityKeys = append(priorityKeys, snIncidentPriorityKeyMap[p])
	}

	payload := snIncidentSearchPayload{
		Filters: snIncidentFilters{
			SearchQuery:  req.Filters.SearchQuery,
			PriorityKeys: priorityKeys,
			ParentIDs:    uuidsToSysids(req.Filters.ParentIDs),
		},
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/incidents/search", token, payload)
	if err != nil {
		return domain.SearchIncidentsResponse{}, err
	}

	var snResp snIncidentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchIncidentsResponse{}, fmt.Errorf("sn incidents: parse response: %w", err)
	}

	views := make([]domain.SearchIncidentView, 0, len(snResp.Incidents))
	for _, inc := range snResp.Incidents {
		view := domain.SearchIncidentView{
			OpenedOn:  inc.OpenedOn,
			Subject:   inc.Subject,
			CreatedOn: inc.CreatedOn,
			CreatedBy: inc.CreatedBy,
			UpdatedOn: inc.UpdatedOn,
			UpdatedBy: inc.UpdatedBy,
		}
		if inc.ID != nil && *inc.ID != "" {
			id := sysidToUUID(*inc.ID)
			view.ID = &id
		}
		if inc.Number != nil {
			view.Number = inc.Number
		}
		if inc.Caller != nil {
			view.Caller = &domain.EntityRef{ID: sysidToUUID(inc.Caller.ID), Name: inc.Caller.Name}
		}
		if inc.Priority != nil {
			if label, ok := snIncidentPriorityLabelMap[inc.Priority.ID]; ok {
				view.Priority = &label
			}
		}
		if inc.State != nil {
			if label, ok := snIncidentStateLabelMap[inc.State.ID]; ok {
				view.State = &label
			}
		}
		if inc.Category != nil {
			if label, ok := snIncidentCategoryLabelMap[inc.Category.ID]; ok {
				view.Category = &label
			}
		}
		if inc.Parent != nil {
			view.Parent = &domain.EntityRef{ID: sysidToUUID(inc.Parent.ID), Name: inc.Parent.Name}
		}
		if inc.AssignmentGroup != nil {
			view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(inc.AssignmentGroup.ID), Name: inc.AssignmentGroup.Name}
		}
		if inc.AssignedTo != nil {
			view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(inc.AssignedTo.ID), Name: inc.AssignedTo.Name}
		}
		views = append(views, view)
	}

	return domain.SearchIncidentsResponse{
		Incidents: views,
		Total:     snResp.TotalRecords,
		Limit:     req.Pagination.Limit,
		Offset:    req.Pagination.Offset,
	}, nil
}
