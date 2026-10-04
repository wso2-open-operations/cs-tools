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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// taskUnavailableMsg is the reason returned by every unavailableTaskService
// method. It matches the 503 description the OpenAPI spec documents for the
// task endpoints.
const taskUnavailableMsg = "tasks are not supported by this data source"

// unavailableTaskService is the Postgres-data-source stand-in for TaskService.
// Tasks live only in the ServiceNow backing store, so every operation reports
// a 503 rather than the route being left unregistered: an unregistered route
// answers 404, which the OpenAPI spec does not document for these paths and
// which callers cannot distinguish from a genuinely missing resource.
//
// This mirrors caseService's handling of the ServiceNow-only tag operations
// (AddCaseTag/RemoveCaseTag/SearchTags), which return the same error type.
type unavailableTaskService struct{}

// NewUnavailableTaskService returns a TaskService that reports every task
// operation as unavailable for the current data source.
func NewUnavailableTaskService() TaskService { return &unavailableTaskService{} }

// SearchCaseTasks implements TaskService.
func (s *unavailableTaskService) SearchCaseTasks(_ context.Context, _ string, _ domain.SearchCaseTasksRequest) (domain.SearchCaseTasksResponse, error) {
	return domain.SearchCaseTasksResponse{}, &apierror.ServiceUnavailableError{Msg: taskUnavailableMsg}
}

// SearchTasks implements TaskService.
func (s *unavailableTaskService) SearchTasks(_ context.Context, _ domain.SearchTasksRequest) (domain.SearchTasksResponse, error) {
	return domain.SearchTasksResponse{}, &apierror.ServiceUnavailableError{Msg: taskUnavailableMsg}
}

// GetTask implements TaskService.
func (s *unavailableTaskService) GetTask(_ context.Context, _ string) (domain.TaskDetail, error) {
	return domain.TaskDetail{}, &apierror.ServiceUnavailableError{Msg: taskUnavailableMsg}
}

// CreateCaseTask implements TaskService.
func (s *unavailableTaskService) CreateCaseTask(_ context.Context, _ string, _ domain.CreateCaseTaskRequest) (domain.TaskDetail, error) {
	return domain.TaskDetail{}, &apierror.ServiceUnavailableError{Msg: taskUnavailableMsg}
}

// UpdateTask implements TaskService.
func (s *unavailableTaskService) UpdateTask(_ context.Context, _ string, _ domain.UpdateTaskRequest) (domain.TaskDetail, error) {
	return domain.TaskDetail{}, &apierror.ServiceUnavailableError{Msg: taskUnavailableMsg}
}
