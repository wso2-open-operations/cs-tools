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

// snIncidentTasksResponse mirrors the Choreo POST /incident-tasks/search response.
type snIncidentTasksResponse struct {
	IncidentTasks []snIncidentTask `json:"incidentTasks"`
	TotalRecords  int              `json:"totalRecords"`
	Offset        int              `json:"offset"`
	Limit         int              `json:"limit"`
}

type snIncidentTask struct {
	ID              *string                `json:"id"`
	Number          *string                `json:"number"`
	Subject         *string                `json:"subject"`
	State           *string                `json:"state"`
	StateLabel      *string                `json:"stateLabel"`
	Incident        *snIncidentTaskRef     `json:"incident"`
	AssignmentGroup *snIncidentTaskUserRef `json:"assignmentGroup"`
	AssignedTo      *snIncidentTaskUserRef `json:"assignedTo"`
}

// snIncidentTaskRef is a compact id+number reference used for the incident
// task's parent incident.
type snIncidentTaskRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// snIncidentTaskUserRef is a compact id+name reference used for
// assignmentGroup/assignedTo.
type snIncidentTaskUserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snIncidentTaskSearchPayload is the Choreo POST /incident-tasks/search request body.
type snIncidentTaskSearchPayload struct {
	Filters    snIncidentTaskFilters `json:"filters,omitempty"`
	Pagination snProjectPagination   `json:"pagination"`
}

type snIncidentTaskFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	// Number: see domain.SearchIncidentTasksFilters.Number doc comment. Exact
	// match against ServiceNow's `number` column -- not part of the
	// free-text SearchQuery scan.
	Number string `json:"number,omitempty"`
	// StateKeys: raw ServiceNow integer state values, NOT translated through
	// a domain enum -- see domain.SearchIncidentTasksFilters.Filters doc
	// comment for why incident_task's state filter skips that layer.
	StateKeys []int `json:"stateKeys,omitempty"`
	// AssignmentGroupIDs: sys_user_group sys_ids (converted from UUIDs).
	AssignmentGroupIDs []string `json:"assignmentGroupIds,omitempty"`
	// IncidentIDs: parent incident sys_ids (converted from UUIDs).
	IncidentIDs []string `json:"incidentIds,omitempty"`
}

type snIncidentTaskService struct {
	client *integrationservice.Client
}

// NewServiceNowIncidentTaskService constructs an IncidentTaskService backed by the Choreo API.
func NewServiceNowIncidentTaskService(client *integrationservice.Client) IncidentTaskService {
	return &snIncidentTaskService{client: client}
}

func (s *snIncidentTaskService) SearchIncidentTasks(ctx context.Context, req domain.SearchIncidentTasksRequest) (domain.SearchIncidentTasksResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}
	parsedFilters, err := ParseIncidentTaskFieldFilters(req.Filters.Filters)
	if err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snIncidentTaskSearchPayload{
		Filters: snIncidentTaskFilters{
			SearchQuery:        req.Filters.SearchQuery,
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			IncidentIDs:        uuidsToSysids(parsedFilters.IncidentIDs),
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/incident-tasks/search", token, payload)
	if err != nil {
		return domain.SearchIncidentTasksResponse{}, err
	}

	var snResp snIncidentTasksResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchIncidentTasksResponse{}, fmt.Errorf("sn incident tasks: parse response: %w", err)
	}

	views := make([]domain.IncidentTask, 0, len(snResp.IncidentTasks))
	for _, it := range snResp.IncidentTasks {
		views = append(views, mapSNIncidentTaskToView(it))
	}

	return domain.SearchIncidentTasksResponse{
		IncidentTasks: views,
		Total:         snResp.TotalRecords,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

// snIncidentTaskAggregatePayload is the Choreo POST /incident-tasks/aggregate
// request body.
type snIncidentTaskAggregatePayload struct {
	Filters   snIncidentTaskFilters `json:"filters,omitempty"`
	GroupBy   string                `json:"groupBy"`
	MaxGroups int                   `json:"maxGroups,omitempty"`
}

// validIncidentTaskAggregateField is the allow-list for
// AggregateIncidentTasksRequest.GroupBy, matching openapi.yaml's
// AggregateIncidentTasksRequest.groupBy enum exactly.
var validIncidentTaskAggregateField = map[string]bool{
	"state":           true,
	"assignmentGroup": true,
}

// AggregateIncidentTasks implements IncidentTaskService by calling the Choreo
// POST /incident-tasks/aggregate endpoint: a single server-side aggregation
// over the requested field, capped to the top MaxGroups buckets with the
// remainder folded into AggregateResponse.OthersCount. Filter parsing and
// validation mirror SearchIncidentTasks.
func (s *snIncidentTaskService) AggregateIncidentTasks(ctx context.Context, req domain.AggregateIncidentTasksRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validIncidentTaskAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.AggregateResponse{}, err
	}
	parsedFilters, err := ParseIncidentTaskFieldFilters(req.Filters.Filters)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snIncidentTaskAggregatePayload{
		Filters: snIncidentTaskFilters{
			SearchQuery:        req.Filters.SearchQuery,
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
			IncidentIDs:        uuidsToSysids(parsedFilters.IncidentIDs),
		},
		GroupBy:   req.GroupBy,
		MaxGroups: req.MaxGroups,
	}

	raw, err := s.client.Post(ctx, "/incident-tasks/aggregate", token, payload)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	var resp domain.AggregateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("sn incident tasks: parse aggregate response: %w", err)
	}
	// "assignmentGroup" is the only ID-valued field in
	// validIncidentTaskAggregateField; SN returns its bucket keys as raw
	// sys_ids, so convert them to this platform's UUIDs before returning.
	// "state" is a plain enum and is left as-is.
	if req.GroupBy == "assignmentGroup" {
		for i := range resp.Groups {
			resp.Groups[i].Key = sysidToUUID(resp.Groups[i].Key)
		}
	}
	return resp, nil
}

// mapSNIncidentTaskToView maps a Choreo incident-task search-result payload
// to the domain search view.
func mapSNIncidentTaskToView(it snIncidentTask) domain.IncidentTask {
	view := domain.IncidentTask{
		Number:     it.Number,
		Subject:    it.Subject,
		State:      it.State,
		StateLabel: it.StateLabel,
	}
	if it.ID != nil && *it.ID != "" {
		id := sysidToUUID(*it.ID)
		view.ID = &id
	}
	if it.Incident != nil {
		view.Incident = &domain.CaseNumberRef{ID: sysidToUUID(it.Incident.ID), Number: it.Incident.Number}
	}
	if it.AssignmentGroup != nil {
		view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(it.AssignmentGroup.ID), Name: it.AssignmentGroup.Name}
	}
	if it.AssignedTo != nil {
		view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(it.AssignedTo.ID), Name: it.AssignedTo.Name}
	}
	return view
}

// snIncidentTaskDetailResponse mirrors the Choreo GET /incident-tasks/{id} response.
type snIncidentTaskDetailResponse struct {
	ID              string                 `json:"id"`
	Number          *string                `json:"number"`
	Subject         *string                `json:"subject"`
	State           *string                `json:"state"`
	StateLabel      *string                `json:"stateLabel"`
	Incident        *snIncidentTaskRef     `json:"incident"`
	AssignmentGroup *snIncidentTaskUserRef `json:"assignmentGroup"`
	AssignedTo      *snIncidentTaskUserRef `json:"assignedTo"`
	Description     *string                `json:"description"`
	Priority        *string                `json:"priority"`
	OpenedAt        *string                `json:"openedAt"`
	ClosedAt        *string                `json:"closedAt"`
}

// GetIncidentTask implements IncidentTaskService for the ServiceNow data source.
func (s *snIncidentTaskService) GetIncidentTask(ctx context.Context, id string) (domain.IncidentTaskDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.IncidentTaskDetail{}, err
	}

	raw, err := s.client.Get(ctx, "/incident-tasks/"+uuidToSysid(id), token)
	if err != nil {
		return domain.IncidentTaskDetail{}, err
	}

	var it snIncidentTaskDetailResponse
	if err := json.Unmarshal(raw, &it); err != nil {
		return domain.IncidentTaskDetail{}, fmt.Errorf("sn get incident task: parse response: %w", err)
	}

	return mapSNIncidentTaskDetailToView(it), nil
}

// mapSNIncidentTaskDetailToView maps a Choreo incident-task detail payload to
// the domain detail view.
func mapSNIncidentTaskDetailToView(it snIncidentTaskDetailResponse) domain.IncidentTaskDetail {
	id := sysidToUUID(it.ID)

	// Number/Subject stay nil (-> JSON null) when the upstream payload omits
	// them, matching every other response field on this view -- unlike ID,
	// which a detail response always carries (it's how the record was
	// fetched), so it's fine as a plain, always-present string.
	view := domain.IncidentTaskDetail{
		ID:          &id,
		Number:      it.Number,
		Subject:     it.Subject,
		State:       it.State,
		StateLabel:  it.StateLabel,
		Description: it.Description,
		Priority:    it.Priority,
		OpenedOn:    it.OpenedAt,
		ClosedOn:    it.ClosedAt,
	}
	if it.Incident != nil {
		view.Incident = &domain.CaseNumberRef{ID: sysidToUUID(it.Incident.ID), Number: it.Incident.Number}
	}
	if it.AssignmentGroup != nil {
		view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(it.AssignmentGroup.ID), Name: it.AssignmentGroup.Name}
	}
	if it.AssignedTo != nil {
		view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(it.AssignedTo.ID), Name: it.AssignedTo.Name}
	}
	return view
}
