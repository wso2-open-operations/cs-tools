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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// createGroupStubService records the create request and answers the defaults.
type createGroupStubService struct {
	service.IncidentService
	got         *domain.CreateIncidentRequest
	createErr   error
	defaults    domain.IncidentCreateDefaults
	defaultsErr error
}

// CreateIncident records the request and returns the stub's configured response or error.
func (s *createGroupStubService) CreateIncident(_ context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	s.got = &req
	return domain.CreateIncidentResponse{}, s.createErr
}

// GetIncidentCreateDefaults returns the stub's configured defaults or error.
func (s *createGroupStubService) GetIncidentCreateDefaults(context.Context) (domain.IncidentCreateDefaults, error) {
	return s.defaults, s.defaultsErr
}

const createGroupBody = `{"subject":"s","category":"INQUIRY","serviceId":"11111111-1111-4111-8111-111111111111",` +
	`"impact":"LOW","urgency":"LOW","callerId":"22222222-2222-4222-8222-222222222222"`

// assignmentGroupId is part of the create contract again: it decodes and
// reaches the service, which decides whether it is allowed.
func TestCreateIncident_AcceptsAnAssignmentGroup(t *testing.T) {
	svc := &createGroupStubService{}
	rec := httptest.NewRecorder()
	body := createGroupBody + `,"assignmentGroupId":"33333333-3333-4333-8333-333333333333"}`
	NewIncidentHandler(svc).CreateIncident(rec, httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.got == nil || svc.got.AssignmentGroupID == nil || *svc.got.AssignmentGroupID != "33333333-3333-4333-8333-333333333333" {
		t.Errorf("the service got %+v, want the sent group", svc.got)
	}
}

// The service's refusal of a group outside the support-group set is a 400
// whose body carries the message and the machine-readable errorCode.
func TestCreateIncident_ARefusedGroupIs400WithItsErrorCode(t *testing.T) {
	svc := &createGroupStubService{createErr: &apierror.ValidationError{
		Msg: "assignmentGroupId must be the active support group of a service", Code: apierror.CodeIncidentAssignmentGroupNotAllowed}}
	rec := httptest.NewRecorder()
	body := createGroupBody + `,"assignmentGroupId":"33333333-3333-4333-8333-333333333333"}`
	NewIncidentHandler(svc).CreateIncident(rec, httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body)))

	want := `{"code":400,"message":"assignmentGroupId must be the active support group of a service","errorCode":"incident_assignment_group_not_allowed"}`
	if rec.Code != http.StatusBadRequest || strings.TrimSpace(rec.Body.String()) != want {
		t.Errorf("status = %d, body = %s; want 400 %s", rec.Code, rec.Body.String(), want)
	}
}

// Every other 400 is unchanged: no errorCode key at all.
func TestCreateIncident_AnUnrelated400HasNoErrorCode(t *testing.T) {
	svc := &createGroupStubService{createErr: &apierror.ValidationError{Msg: "assignmentGroupId contains invalid UUID: \"x\""}}
	rec := httptest.NewRecorder()
	NewIncidentHandler(svc).CreateIncident(rec, httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(createGroupBody+`}`)))

	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "errorCode") {
		t.Errorf("status = %d, body = %s; want 400 with no errorCode", rec.Code, rec.Body.String())
	}
}

// Any other unknown field is still refused before the service runs.
func TestCreateIncident_StillRejectsUnknownFields(t *testing.T) {
	svc := &createGroupStubService{}
	rec := httptest.NewRecorder()
	body := createGroupBody + `,"assignmentGroupName":"SRE"}`
	NewIncidentHandler(svc).CreateIncident(rec, httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if svc.got != nil {
		t.Error("the service ran for a body with an unknown field")
	}
}

// GET /incidents/create-defaults returns the default service and its group,
// with explicit nulls when there are none.
func TestGetIncidentCreateDefaults(t *testing.T) {
	id := "44444444-4444-4444-8444-444444444444"
	for name, tc := range map[string]struct {
		svc      *createGroupStubService
		wantCode int
		wantBody string
	}{
		"configured": {svc: &createGroupStubService{defaults: domain.IncidentCreateDefaults{DefaultServiceID: &id, DefaultGroup: &domain.EntityRef{ID: "55555555-5555-4555-8555-555555555555", Name: "SRE"}}},
			wantCode: http.StatusOK, wantBody: `{"defaultServiceId":"` + id + `","defaultGroup":{"id":"55555555-5555-4555-8555-555555555555","name":"SRE"}}`},
		"unset":          {svc: &createGroupStubService{}, wantCode: http.StatusOK, wantBody: `{"defaultServiceId":null,"defaultGroup":null}`},
		"lookup failure": {svc: &createGroupStubService{defaultsErr: errors.New("down")}, wantCode: http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewIncidentHandler(tc.svc).GetIncidentCreateDefaults(rec, httptest.NewRequest(http.MethodGet, "/incidents/create-defaults", nil))
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tc.wantBody {
				t.Errorf("body = %s, want %s", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
