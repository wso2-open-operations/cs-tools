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

// snTaskAssignedTo is the assignee reference embedded in a task record.
type snTaskAssignedTo struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

// snTask is a single task record as returned by GET /cases/{id}/tasks.
type snTask struct {
	ID         string            `json:"id"`
	Subject    *string           `json:"subject"`
	State      *string           `json:"state"`
	DueDate    *string           `json:"dueDate"`
	AssignedTo *snTaskAssignedTo `json:"assignedTo"`
	UpdatedOn  *string           `json:"updatedOn"`
}

// snCaseTasksResponse mirrors the Choreo POST /cases/{id}/tasks/search response.
type snCaseTasksResponse struct {
	Tasks  []snTask `json:"tasks"`
	Total  int      `json:"total"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

// snCaseTasksSearchPayload is the Choreo POST /cases/{id}/tasks/search request body.
type snCaseTasksSearchPayload struct {
	Pagination snProjectPagination `json:"pagination"`
}

// snProductRef is a named product reference embedded in a task detail record.
type snProductRef struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

// snTaskParentCase is the parent case reference embedded in a task detail record.
type snTaskParentCase struct {
	ID     *string `json:"id"`
	Number *string `json:"number"`
}

// snTaskDetail mirrors the Choreo GET /tasks/{id} response.
type snTaskDetail struct {
	ID                string            `json:"id"`
	Subject           *string           `json:"subject"`
	State             *string           `json:"state"`
	DueDate           *string           `json:"dueDate"`
	VisibleToCustomer bool              `json:"visibleToCustomer"`
	AssignedTo        *snTaskAssignedTo `json:"assignedTo"`
	RequestType       *string           `json:"requestType"`
	RequestTypeLabel  *string           `json:"requestTypeLabel"`
	Environment       *string           `json:"environment"`
	EnvironmentLabel  *string           `json:"environmentLabel"`
	Product           *snProductRef     `json:"product"`
	ParentCase        *snTaskParentCase `json:"parentCase"`
	CreatedOn         *string           `json:"createdOn"`
	UpdatedOn         *string           `json:"updatedOn"`
}

// snAssignedToToEntityRef converts an snTaskAssignedTo reference to a domain
// EntityRef, converting the sysid to a UUID. Returns nil if the reference or
// its id is absent.
func snAssignedToToEntityRef(a *snTaskAssignedTo) *domain.EntityRef {
	if a == nil || a.ID == nil || *a.ID == "" {
		return nil
	}
	ref := &domain.EntityRef{ID: sysidToUUID(*a.ID)}
	if a.Name != nil {
		ref.Name = *a.Name
	}
	return ref
}

type snTaskService struct {
	client *integrationservice.Client
}

// NewServiceNowTaskService constructs a TaskService backed by the Choreo API.
func NewServiceNowTaskService(client *integrationservice.Client) TaskService {
	return &snTaskService{client: client}
}

func (s *snTaskService) SearchCaseTasks(ctx context.Context, caseID string, req domain.SearchCaseTasksRequest) (domain.SearchCaseTasksResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.SearchCaseTasksResponse{}, err
	}

	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCaseTasksResponse{}, err
	}

	payload := snCaseTasksSearchPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	path := "/cases/" + uuidToSysid(caseID) + "/tasks/search"
	raw, err := s.client.Post(ctx, path, token, payload)
	if err != nil {
		return domain.SearchCaseTasksResponse{}, err
	}

	var snResp snCaseTasksResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseTasksResponse{}, fmt.Errorf("sn case tasks: parse response: %w", err)
	}

	tasks := make([]domain.TaskSummary, 0, len(snResp.Tasks))
	for _, t := range snResp.Tasks {
		task := domain.TaskSummary{
			ID:         sysidToUUID(t.ID),
			State:      t.State,
			DueDate:    t.DueDate,
			AssignedTo: snAssignedToToEntityRef(t.AssignedTo),
		}
		if t.Subject != nil {
			task.Subject = *t.Subject
		}
		if t.UpdatedOn != nil {
			task.UpdatedOn = *t.UpdatedOn
		}
		tasks = append(tasks, task)
	}

	return domain.SearchCaseTasksResponse{
		Tasks:  tasks,
		Total:  snResp.Total,
		Offset: snResp.Offset,
		Limit:  snResp.Limit,
	}, nil
}

func (s *snTaskService) GetTask(ctx context.Context, id string) (domain.TaskDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.TaskDetail{}, err
	}

	raw, err := s.client.Get(ctx, "/tasks/"+uuidToSysid(id), token)
	if err != nil {
		return domain.TaskDetail{}, err
	}

	var t snTaskDetail
	if err := json.Unmarshal(raw, &t); err != nil {
		return domain.TaskDetail{}, fmt.Errorf("sn get task: parse response: %w", err)
	}

	return snTaskDetailToDomain(t), nil
}

// snTaskDetailToDomain converts an snTaskDetail (the Choreo GET /tasks/{id}
// response shape, reused here for the create/update responses) to the domain
// TaskDetail view. Shared by GetTask, CreateCaseTask, and UpdateTask so all
// three map the wire shape identically.
func snTaskDetailToDomain(t snTaskDetail) domain.TaskDetail {
	detail := domain.TaskDetail{
		ID:                sysidToUUID(t.ID),
		State:             t.State,
		DueDate:           t.DueDate,
		VisibleToCustomer: t.VisibleToCustomer,
		AssignedTo:        snAssignedToToEntityRef(t.AssignedTo),
		RequestType:       t.RequestType,
		RequestTypeLabel:  t.RequestTypeLabel,
		Environment:       t.Environment,
		EnvironmentLabel:  t.EnvironmentLabel,
	}
	if t.Subject != nil {
		detail.Subject = *t.Subject
	}
	if t.CreatedOn != nil {
		detail.CreatedOn = *t.CreatedOn
	}
	if t.UpdatedOn != nil {
		detail.UpdatedOn = *t.UpdatedOn
	}

	if t.Product != nil && t.Product.ID != nil && *t.Product.ID != "" {
		ref := &domain.EntityRef{ID: sysidToUUID(*t.Product.ID)}
		if t.Product.Name != nil {
			ref.Name = *t.Product.Name
		}
		detail.Product = ref
	}

	if t.ParentCase != nil && t.ParentCase.ID != nil && *t.ParentCase.ID != "" {
		ref := &domain.CaseNumberRef{ID: sysidToUUID(*t.ParentCase.ID)}
		if t.ParentCase.Number != nil {
			ref.Number = *t.ParentCase.Number
		}
		detail.ParentCase = ref
	}

	return detail
}

// snCreateTaskPayload is the request body for the (not yet existing) Choreo
// POST /cases/{id}/tasks endpoint.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): ServiceNow/Ballerina task support is read-only today
// (TaskSearchPayload/TaskResponse only -- see modules/servicenow/types.bal).
// No Choreo endpoint exists yet to create a sn_customerservice_task record.
// Ask: add POST /cases/{id}/tasks (this payload shape) to servicenow.bal,
// returning a TaskResponse-shaped detail.
type snCreateTaskPayload struct {
	Subject           string  `json:"subject"`
	DueDate           *string `json:"dueDate,omitempty"`
	AssignedToEmail   *string `json:"assignedToEmail,omitempty"`
	VisibleToCustomer *bool   `json:"visibleToCustomer,omitempty"`
}

type snCreateTaskResponse struct {
	Message string       `json:"message"`
	Task    snTaskDetail `json:"task"`
}

// CreateCaseTask creates a new task on the case identified by caseID.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): see snCreateTaskPayload doc comment. Implemented so the
// entity-service side is ready the moment Ballerina adds the endpoint; until
// then, calling it returns a downstream error (no such Choreo route today).
func (s *snTaskService) CreateCaseTask(ctx context.Context, caseID string, req domain.CreateCaseTaskRequest) (domain.TaskDetail, error) {
	if err := validateUUIDs("id", []string{caseID}); err != nil {
		return domain.TaskDetail{}, err
	}
	if req.Subject == "" {
		return domain.TaskDetail{}, &apierror.ValidationError{Msg: "subject is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateTaskPayload{
		Subject:           req.Subject,
		AssignedToEmail:   req.AssignedToEmail,
		VisibleToCustomer: req.VisibleToCustomer,
	}
	if req.DueDate != nil {
		dueDate := formatSNDate(req.DueDate)
		payload.DueDate = &dueDate
	}

	raw, err := s.client.Post(ctx, "/cases/"+uuidToSysid(caseID)+"/tasks", token, payload)
	if err != nil {
		return domain.TaskDetail{}, err
	}

	var snResp snCreateTaskResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.TaskDetail{}, fmt.Errorf("sn create case task: parse response: %w", err)
	}

	return snTaskDetailToDomain(snResp.Task), nil
}

// snUpdateTaskPayload is the request body for the (not yet existing) Choreo
// PATCH /tasks/{id} endpoint. Exactly one of State, AssignedToEmail, or DueDate
// must be provided per request, following the same convention as
// snUpdateCasePayload.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): same gap as snCreateTaskPayload above. Ask: add
// PATCH /tasks/{id} (this payload shape) to servicenow.bal, returning a
// TaskResponse-shaped detail.
type snUpdateTaskPayload struct {
	State           *string `json:"state,omitempty"`
	AssignedToEmail *string `json:"assignedToEmail,omitempty"`
	DueDate         *string `json:"dueDate,omitempty"`
}

type snUpdateTaskResponse struct {
	Message string       `json:"message"`
	Task    snTaskDetail `json:"task"`
}

// UpdateTask updates exactly one of state, assignedToEmail, or dueDate on the
// task identified by taskID.
//
// Ballerina support added on ballerina-tasks-fixeta-tags (not yet merged to digiops-cs main): see snUpdateTaskPayload doc comment. Implemented so the
// entity-service side is ready the moment Ballerina adds the endpoint; until
// then, calling it returns a downstream error (no such Choreo route today).
func (s *snTaskService) UpdateTask(ctx context.Context, taskID string, req domain.UpdateTaskRequest) (domain.TaskDetail, error) {
	if err := validateUUIDs("id", []string{taskID}); err != nil {
		return domain.TaskDetail{}, err
	}

	fieldCount := 0
	if req.State != nil {
		fieldCount++
	}
	if req.AssignedToEmail != nil {
		fieldCount++
	}
	if req.DueDate != nil {
		fieldCount++
	}
	if fieldCount == 0 {
		return domain.TaskDetail{}, &apierror.ValidationError{Msg: "exactly one of state, assignedToEmail, or dueDate must be provided"}
	}
	if fieldCount > 1 {
		return domain.TaskDetail{}, &apierror.ValidationError{Msg: "only one of state, assignedToEmail, or dueDate may be provided per request"}
	}
	if req.State != nil && *req.State == "" {
		return domain.TaskDetail{}, &apierror.ValidationError{Msg: "state cannot be empty"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateTaskPayload{
		State:           req.State,
		AssignedToEmail: req.AssignedToEmail,
	}
	if req.DueDate != nil {
		dueDate := formatSNDate(req.DueDate)
		payload.DueDate = &dueDate
	}

	raw, err := s.client.Patch(ctx, "/tasks/"+uuidToSysid(taskID), token, payload)
	if err != nil {
		return domain.TaskDetail{}, err
	}

	var snResp snUpdateTaskResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.TaskDetail{}, fmt.Errorf("sn update task: parse response: %w", err)
	}

	return snTaskDetailToDomain(snResp.Task), nil
}
