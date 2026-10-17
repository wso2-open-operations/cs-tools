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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type mockProjectConsumptionService struct {
	service.ProjectService

	statusRes domain.ProjectConsumptionStatus
	statusErr error

	updateReq       domain.ProjectConsumptionUpdateRequest
	updateID        string
	updateUpdatedBy string
	updateErr       error
}

func (m *mockProjectConsumptionService) GetProjectConsumptionStatus(_ context.Context, id string) (domain.ProjectConsumptionStatus, error) {
	if m.statusErr != nil {
		return domain.ProjectConsumptionStatus{}, m.statusErr
	}
	return m.statusRes, nil
}

func (m *mockProjectConsumptionService) UpdateProjectConsumption(_ context.Context, id string, req domain.ProjectConsumptionUpdateRequest, updatedBy string) error {
	m.updateID = id
	m.updateReq = req
	m.updateUpdatedBy = updatedBy
	return m.updateErr
}

func TestGetProjectConsumptionStatus_Success(t *testing.T) {
	appID := "app-999"
	name := "Sample Project"
	svc := &mockProjectConsumptionService{
		statusRes: domain.ProjectConsumptionStatus{
			Status:        5,
			ApplicationID: &appID,
			Name:          &name,
		},
	}
	h := NewProjectConsumptionHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/projects/11111111-1111-1111-1111-111111111111/consumption/status", strings.NewReader(`{"email":"user@wso2.com","deploymentId":"dep-1"}`))
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.GetProjectConsumptionStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: the result carries the project's secrets", got)
	}

	var body struct {
		ApplicationID *string                         `json:"applicationId"`
		Result        domain.ProjectConsumptionStatus `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Result.Status != 5 {
		t.Errorf("result.status = %d, want 5", body.Result.Status)
	}
	if body.Result.ApplicationID == nil || *body.Result.ApplicationID != "app-999" {
		t.Errorf("result.applicationId = %v, want app-999", body.Result.ApplicationID)
	}
	if body.ApplicationID == nil || *body.ApplicationID != "app-999" {
		t.Errorf("top-level applicationId = %v, want app-999", body.ApplicationID)
	}
}

func TestGetProjectConsumptionStatus_RejectsInvalidUUID(t *testing.T) {
	svc := &mockProjectConsumptionService{
		statusErr: &apierror.ValidationError{Msg: "id is not a valid UUID: not-a-uuid"},
	}
	h := NewProjectConsumptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/projects/not-a-uuid/consumption/status", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()

	h.GetProjectConsumptionStatus(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetProjectConsumptionStatus_NotFound(t *testing.T) {
	svc := &mockProjectConsumptionService{
		statusErr: &apierror.NotFoundError{Msg: "project not found"},
	}
	h := NewProjectConsumptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/projects/11111111-1111-1111-1111-111111111111/consumption/status", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()

	h.GetProjectConsumptionStatus(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUpdateProjectConsumption_Success(t *testing.T) {
	svc := &mockProjectConsumptionService{}
	h := NewProjectConsumptionHandler(svc)

	bodyJSON := `{"status":5,"consumerKey":"ckey","consumerSecret":"csec","primarySecretKey":"pkey","secondarySecretKey":"skey","licenseSecrets":"lsec"}`
	req := httptest.NewRequest(http.MethodPatch, "/projects/11111111-1111-1111-1111-111111111111/consumption", strings.NewReader(bodyJSON))
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.UpdateProjectConsumption(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if svc.updateID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("svc.updateID = %q", svc.updateID)
	}
	if svc.updateReq.Status != 5 {
		t.Errorf("svc.updateReq.Status = %d, want 5", svc.updateReq.Status)
	}
	if svc.updateReq.ConsumerKey == nil || *svc.updateReq.ConsumerKey != "ckey" {
		t.Errorf("svc.updateReq.ConsumerKey = %v, want ckey", svc.updateReq.ConsumerKey)
	}
}

func TestUpdateProjectConsumption_InvalidBody(t *testing.T) {
	svc := &mockProjectConsumptionService{}
	h := NewProjectConsumptionHandler(svc)

	req := httptest.NewRequest(http.MethodPatch, "/projects/11111111-1111-1111-1111-111111111111/consumption", strings.NewReader(`{invalid-json`))
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.UpdateProjectConsumption(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateProjectConsumption_NotFound(t *testing.T) {
	svc := &mockProjectConsumptionService{
		updateErr: &apierror.NotFoundError{Msg: "project not found"},
	}
	h := NewProjectConsumptionHandler(svc)

	bodyJSON := `{"status":5}`
	req := httptest.NewRequest(http.MethodPatch, "/projects/11111111-1111-1111-1111-111111111111/consumption", strings.NewReader(bodyJSON))
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.UpdateProjectConsumption(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
