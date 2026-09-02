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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// stubAlertIncidentMappingService embeds the service.AlertIncidentMappingService
// interface so tests only need to implement the method(s) under test; any
// call to an unimplemented method panics on the nil embedded interface, which
// is fine since these tests never reach them.
type stubAlertIncidentMappingService struct {
	service.AlertIncidentMappingService

	createResp domain.AlertIncidentMappingView
	createErr  error

	lookupResp domain.LookupAlertIncidentMappingsResponse
	lookupErr  error
}

func (s *stubAlertIncidentMappingService) CreateAlertIncidentMapping(_ context.Context, _ domain.CreateAlertIncidentMappingRequest) (domain.AlertIncidentMappingView, error) {
	return s.createResp, s.createErr
}

func (s *stubAlertIncidentMappingService) LookupAlertIncidentMappings(_ context.Context, _ domain.LookupAlertIncidentMappingsRequest) (domain.LookupAlertIncidentMappingsResponse, error) {
	return s.lookupResp, s.lookupErr
}

func TestCreateAlertIncidentMapping_Success(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		createResp: domain.AlertIncidentMappingView{ID: "id-1", AlertNumber: "ALT-1", Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1"},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.CreateAlertIncidentMappingRequest{
		AlertNumber: "ALT-1", Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CreateAlertIncidentMapping(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.AlertIncidentMappingView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "id-1" || got.AlertNumber != "ALT-1" {
		t.Fatalf("unexpected response body: %+v", got)
	}
}

func TestCreateAlertIncidentMapping_ValidationError(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		createErr: &apierror.ValidationError{Msg: "alertNumber is required"},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.CreateAlertIncidentMappingRequest{
		Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CreateAlertIncidentMapping(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAlertIncidentMapping_ConflictOnDuplicateAlertNumber(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		createErr: &apierror.ConflictError{Msg: "alertNumber is already mapped to an incident: ALT-1"},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.CreateAlertIncidentMappingRequest{
		AlertNumber: "ALT-1", Source: "datadog", AlertStatus: "firing", IncidentID: "inc-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CreateAlertIncidentMapping(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLookupAlertIncidentMappings_Success(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		lookupResp: domain.LookupAlertIncidentMappingsResponse{Mappings: []domain.AlertIncidentMappingView{
			{ID: "id-1", AlertNumber: "ALT-1"},
			{ID: "id-2", AlertNumber: "ALT-2"},
		}},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.LookupAlertIncidentMappingsRequest{Source: "datadog", UniqueIdentifier: "grp-1"})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.LookupAlertIncidentMappings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.LookupAlertIncidentMappingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(got.Mappings))
	}
}

// TestLookupAlertIncidentMappings_EmptyResultReturns200NotNotFound verifies
// that no matches is reported as a 200 with an empty mappings array, not a
// 404 — absence is a valid result for a lookup, not an error.
func TestLookupAlertIncidentMappings_EmptyResultReturns200NotNotFound(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		lookupResp: domain.LookupAlertIncidentMappingsResponse{Mappings: []domain.AlertIncidentMappingView{}},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.LookupAlertIncidentMappingsRequest{Source: "datadog", UniqueIdentifier: "grp-unknown"})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.LookupAlertIncidentMappings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for an empty lookup result, got %d: %s", rec.Code, rec.Body.String())
	}
	var got domain.LookupAlertIncidentMappingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Mappings == nil {
		t.Fatal("expected mappings to serialize as an empty array, not null")
	}
	if len(got.Mappings) != 0 {
		t.Fatalf("expected 0 mappings, got %d", len(got.Mappings))
	}
}

func TestLookupAlertIncidentMappings_ValidationError(t *testing.T) {
	stub := &stubAlertIncidentMappingService{
		lookupErr: &apierror.ValidationError{Msg: "uniqueIdentifier is required"},
	}
	h := NewAlertIncidentMappingHandler(stub)

	body, _ := json.Marshal(domain.LookupAlertIncidentMappingsRequest{Source: "datadog"})
	req := httptest.NewRequest(http.MethodPost, "/alert-incident-mappings/lookup", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.LookupAlertIncidentMappings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}
