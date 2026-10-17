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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type mockProjectRepoForConsumption struct {
	repository.ProjectRepository
	statusRes domain.ProjectConsumptionStatus
	statusErr error

	updateReq       domain.ProjectConsumptionUpdateRequest
	updateID        string
	updateUpdatedBy string
	updateErr       error
}

func (m *mockProjectRepoForConsumption) GetProjectConsumptionStatus(_ context.Context, id string) (domain.ProjectConsumptionStatus, error) {
	if m.statusErr != nil {
		return domain.ProjectConsumptionStatus{}, m.statusErr
	}
	return m.statusRes, nil
}

func (m *mockProjectRepoForConsumption) UpdateProjectConsumption(_ context.Context, id string, req domain.ProjectConsumptionUpdateRequest, updatedBy string) error {
	m.updateID = id
	m.updateReq = req
	m.updateUpdatedBy = updatedBy
	return m.updateErr
}

func TestProjectService_GetProjectConsumptionStatus_RejectsInvalidUUID(t *testing.T) {
	repo := &mockProjectRepoForConsumption{}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	_, err := svc.GetProjectConsumptionStatus(context.Background(), "invalid-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestProjectService_GetProjectConsumptionStatus_Success(t *testing.T) {
	appID := "app-123"
	repo := &mockProjectRepoForConsumption{
		statusRes: domain.ProjectConsumptionStatus{
			Status:        5,
			ApplicationID: &appID,
		},
	}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	res, err := svc.GetProjectConsumptionStatus(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != 5 || res.ApplicationID == nil || *res.ApplicationID != "app-123" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestProjectService_UpdateProjectConsumption_RejectsInvalidUUID(t *testing.T) {
	repo := &mockProjectRepoForConsumption{}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	err := svc.UpdateProjectConsumption(context.Background(), "invalid-uuid", domain.ProjectConsumptionUpdateRequest{Status: 5}, "user@example.com")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestProjectService_UpdateProjectConsumption_Success(t *testing.T) {
	repo := &mockProjectRepoForConsumption{}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	req := domain.ProjectConsumptionUpdateRequest{
		Status: 5,
	}
	err := svc.UpdateProjectConsumption(context.Background(), "11111111-1111-1111-1111-111111111111", req, "actor@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updateID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("repo got id %q", repo.updateID)
	}
	if repo.updateUpdatedBy != "actor@example.com" {
		t.Errorf("repo got updatedBy %q, want %q", repo.updateUpdatedBy, "actor@example.com")
	}
}
