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
)

type stubOnboardingStepService struct {
	upsertReq domain.UpsertOnboardingStepRequest
	getID     string
	searchReq domain.SearchOnboardingStepsRequest
	err       error
}

func (s *stubOnboardingStepService) Upsert(_ context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error) {
	s.upsertReq = req
	return domain.OnboardingStep{MembershipSfID: req.MembershipSfID, Step: req.Step, Status: req.Status, AttemptCount: 1}, s.err
}

func (s *stubOnboardingStepService) GetByMembership(_ context.Context, id string) (domain.GetOnboardingStepsResponse, error) {
	s.getID = id
	return domain.GetOnboardingStepsResponse{Steps: []domain.OnboardingStep{}}, s.err
}

func (s *stubOnboardingStepService) Search(_ context.Context, req domain.SearchOnboardingStepsRequest) (domain.SearchOnboardingStepsResponse, error) {
	s.searchReq = req
	return domain.SearchOnboardingStepsResponse{Steps: []domain.OnboardingStep{}, Limit: 20}, s.err
}

func newOnboardingStepMux(svc *stubOnboardingStepService) *http.ServeMux {
	h := NewOnboardingStepHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /onboarding-steps/{membershipSfId}/{step}", h.UpsertOnboardingStep)
	mux.HandleFunc("GET /onboarding-steps/{membershipSfId}", h.GetOnboardingSteps)
	mux.HandleFunc("POST /onboarding-steps/search", h.SearchOnboardingSteps)
	return mux
}

func TestUpsertOnboardingStep_PathValuesAndBody(t *testing.T) {
	svc := &stubOnboardingStepService{}
	req := httptest.NewRequest(http.MethodPut, "/onboarding-steps/a0e1/IDENTITY", strings.NewReader(
		`{"status":"SUCCEEDED","eventType":"CREATED","eventModifiedOn":"2026-09-18T06:37:07Z","email":"jane@acme.com","contactSfId":"003xx"}`))
	rec := httptest.NewRecorder()
	newOnboardingStepMux(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if svc.upsertReq.MembershipSfID != "a0e1" || svc.upsertReq.Step != "IDENTITY" || svc.upsertReq.Status != "SUCCEEDED" ||
		svc.upsertReq.Email != "jane@acme.com" || svc.upsertReq.ContactSfID == nil || *svc.upsertReq.ContactSfID != "003xx" ||
		svc.upsertReq.EventModifiedOn.IsZero() {
		t.Errorf("service req = %+v", svc.upsertReq)
	}
	var out domain.OnboardingStep
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.AttemptCount != 1 {
		t.Errorf("body = %s (%v)", rec.Body.String(), err)
	}
}

func TestUpsertOnboardingStep_Errors(t *testing.T) {
	// Unknown field is rejected by decodeRequest before the service runs.
	svc := &stubOnboardingStepService{}
	req := httptest.NewRequest(http.MethodPut, "/onboarding-steps/a0e1/IDENTITY", strings.NewReader(`{"status":"SUCCEEDED","bogus":1}`))
	rec := httptest.NewRecorder()
	newOnboardingStepMux(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || svc.upsertReq.MembershipSfID != "" {
		t.Errorf("unknown field: status = %d, called = %v", rec.Code, svc.upsertReq.MembershipSfID != "")
	}

	// Service ValidationError maps to 400.
	svc = &stubOnboardingStepService{err: &apierror.ValidationError{Msg: "step must be one of IDENTITY, DATABASE, EMAIL, REGISTRATION"}}
	req = httptest.NewRequest(http.MethodPut, "/onboarding-steps/a0e1/PAYMENT", strings.NewReader(`{"status":"SUCCEEDED","eventType":"x","eventModifiedOn":"2026-09-18T06:37:07Z","email":"e"}`))
	rec = httptest.NewRecorder()
	newOnboardingStepMux(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("validation: status = %d", rec.Code)
	}
}

func TestGetAndSearchOnboardingSteps(t *testing.T) {
	svc := &stubOnboardingStepService{}
	mux := newOnboardingStepMux(svc)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/onboarding-steps/a0e1", nil))
	if rec.Code != http.StatusOK || svc.getID != "a0e1" || !strings.Contains(rec.Body.String(), `"steps":[]`) {
		t.Errorf("get: status = %d id = %q body = %s", rec.Code, svc.getID, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/onboarding-steps/search", strings.NewReader(
		`{"filters":{"statuses":["FAILED"],"membershipSfIds":["a0e1"]},"pagination":{"limit":5}}`)))
	if rec.Code != http.StatusOK || len(svc.searchReq.Filters.Statuses) != 1 || svc.searchReq.Pagination.Limit != 5 {
		t.Errorf("search: status = %d req = %+v", rec.Code, svc.searchReq)
	}
}
