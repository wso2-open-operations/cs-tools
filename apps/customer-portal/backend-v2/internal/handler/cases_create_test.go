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

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type mockCreateCaseClient struct {
	entityCaseClient
	project       entity.ProjectDetailsView
	getProjectErr error
	createdCase   entity.CreateCaseResponse
	createCaseErr error
}

func (m *mockCreateCaseClient) GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error) {
	if m.getProjectErr != nil {
		return entity.ProjectDetailsView{}, m.getProjectErr
	}
	return m.project, nil
}

func (m *mockCreateCaseClient) CreateCase(ctx context.Context, req entity.CreateCaseRequest) (entity.CreateCaseResponse, error) {
	if m.createCaseErr != nil {
		return entity.CreateCaseResponse{}, m.createCaseErr
	}
	return m.createdCase, nil
}

func TestCreateCase_SuspendedProject_Forbidden(t *testing.T) {
	suspended := "Suspended"
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID: "11111111-1111-1111-1111-111111111111",
			ProjectClosureFields: entity.ProjectClosureFields{
				ClosureState: &suspended,
			},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCase_ExpiredProject_Forbidden(t *testing.T) {
	pastDate := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID:      "11111111-1111-1111-1111-111111111111",
			EndDate: pastDate,
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCase_ActiveProject_Success(t *testing.T) {
	futureDate := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	mock := &mockCreateCaseClient{
		project: entity.ProjectDetailsView{
			ID:      "11111111-1111-1111-1111-111111111111",
			EndDate: futureDate,
		},
		createdCase: entity.CreateCaseResponse{
			Case: entity.CreateCaseDetails{
				ID:     "22222222-2222-2222-2222-222222222222",
				Number: "CS12345",
			},
		},
	}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"11111111-1111-1111-1111-111111111111","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateCase_InvalidProjectID_BadRequest(t *testing.T) {
	mock := &mockCreateCaseClient{}
	h := NewCaseHandler(mock)

	reqBody := `{"projectId":"not-a-valid-uuid","title":"Test Case","description":"Details"}`
	req := authedRequest(http.MethodPost, "/cases", reqBody)
	rec := httptest.NewRecorder()

	h.CreateCase(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}
