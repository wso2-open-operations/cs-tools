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

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// TestIncidentStateToEnum locks in the one deliberate mismatch between
// domain.IncidentState and incident_state_enum's real labels: "cancelled"
// is stored as 'CANCELED' (one L), not domain's "CANCELLED" (two Ls).
// Every other state matches by identity.
func TestIncidentStateToEnum(t *testing.T) {
	tests := []struct {
		state domain.IncidentState
		want  string
	}{
		{domain.IncidentStateNew, "NEW"},
		{domain.IncidentStateInProgress, "IN_PROGRESS"},
		{domain.IncidentStateOnHold, "ON_HOLD"},
		{domain.IncidentStateResolved, "RESOLVED"},
		{domain.IncidentStateClosed, "CLOSED"},
		{domain.IncidentStateCancelled, "CANCELED"},
	}
	for _, tt := range tests {
		if got := incidentStateToEnum(tt.state); got != tt.want {
			t.Errorf("incidentStateToEnum(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestIncidentPriorityToEnum locks in that incident_priority_enum has no
// 'PLANNING' label at all (only CRITICAL/HIGH/MODERATE/LOW) -- a caller
// filtering by IncidentPriorityPlanning must be rejected, not silently cast
// into an invalid enum value.
func TestIncidentPriorityToEnum(t *testing.T) {
	tests := []struct {
		priority domain.IncidentPriority
		want     string
		wantOk   bool
	}{
		{domain.IncidentPriorityCritical, "CRITICAL", true},
		{domain.IncidentPriorityHigh, "HIGH", true},
		{domain.IncidentPriorityModerate, "MODERATE", true},
		{domain.IncidentPriorityLow, "LOW", true},
		{domain.IncidentPriorityPlanning, "", false},
	}
	for _, tt := range tests {
		got, ok := incidentPriorityToEnum(tt.priority)
		if ok != tt.wantOk || (ok && got != tt.want) {
			t.Errorf("incidentPriorityToEnum(%q) = (%q, %v), want (%q, %v)", tt.priority, got, ok, tt.want, tt.wantOk)
		}
	}
}

// stubIncidentRepo is a minimal repository.IncidentRepository whose
// unconfigured methods panic if called -- same convention as stubCaseRepo
// (case_service_test.go).
type stubIncidentRepo struct {
	createIncidentFromServiceNow func(ctx context.Context, req domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error)
}

func (s *stubIncidentRepo) SearchIncidents(context.Context, domain.SearchIncidentsRequest, []string, []string, []string, []string, *bool, *bool, *time.Time, *time.Time) ([]domain.SearchIncidentView, int, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) AggregateIncidents(context.Context, domain.SearchIncidentsRequest, []string, []string, []string, []string, *bool, *bool, *time.Time, *time.Time, string, int) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) GetIncidentByID(context.Context, string) (domain.IncidentView, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) SearchIncidentActivities(context.Context, domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) CreateIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error) {
	if s.createIncidentFromServiceNow != nil {
		return s.createIncidentFromServiceNow(ctx, req, id, number, createdBy)
	}
	panic("CreateIncidentFromServiceNow called unexpectedly: Postgres must stay untouched when ServiceNow never accepts the incident")
}

// stubMirrorIncidentService embeds IncidentService (nil) and overrides only
// CreateIncident -- same convention as stubMirrorCaseService
// (case_service_test.go). Any other method being called would panic on the
// nil embedded interface, which is the point: this pilot's incident mode
// only ever calls the mirror's CreateIncident.
type stubMirrorIncidentService struct {
	IncidentService
	createIncident func(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)
}

func (s *stubMirrorIncidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	return s.createIncident(ctx, req)
}

// validCreateIncidentRequest (sn_incident_service_test.go) already returns a
// minimally valid CreateIncidentRequest -- reused here as-is.

// TestIncidentService_CreateIncident_SNFailureLeavesPostgresUntouched is the
// pilot's core regression guard for incident CREATE, mirroring
// TestCaseService_CreateCase_SNFailureLeavesPostgresUntouched exactly: a
// single ServiceNow failure must return an error immediately -- no internal
// retry -- and the Postgres repository must never be called at all -- no
// row, no orphan.
func TestIncidentService_CreateIncident_SNFailureLeavesPostgresUntouched(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateIncidentResponse{}, errors.New("sn downstream unreachable")
		},
	}
	// No createIncidentFromServiceNow override -- stubIncidentRepo panics if
	// it's ever called, which is exactly the assertion: Postgres must stay
	// untouched.
	repo := &stubIncidentRepo{}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, nil)

	_, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the incident")
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (no internal retry), got %d", attempts)
	}
}

// TestIncidentService_CreateIncident_SNSuccessCreatesPostgresRowWithMatchingIdentity
// covers the other half, mirroring
// TestCaseService_CreateCase_SNSuccessCreatesPostgresRowWithMatchingIdentity:
// on ServiceNow success, the Postgres insert must use EXACTLY the
// id/number/createdBy ServiceNow returned -- not anything generated locally
// -- so both systems agree on identity from the moment the Postgres row
// exists.
func TestIncidentService_CreateIncident_SNSuccessCreatesPostgresRowWithMatchingIdentity(t *testing.T) {
	const (
		snID        = "44444444-4444-4444-4444-444444444444"
		snNumber    = "INC0023001"
		snCreatedBy = "jane.doe@example.com"
	)
	mirror := &stubMirrorIncidentService{
		createIncident: func(_ context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = snID
			resp.Incident.Number = snNumber
			resp.Incident.CreatedBy = snCreatedBy
			resp.Incident.CreatedOn = "2026-09-21T12:00:00Z"
			return resp, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotNumber, gotCreatedBy string
	repo := &stubIncidentRepo{
		createIncidentFromServiceNow: func(_ context.Context, req domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error) {
			mu.Lock()
			gotID, gotNumber, gotCreatedBy = id, number, createdBy
			mu.Unlock()
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = id
			resp.Incident.Number = number
			resp.Incident.CreatedBy = createdBy
			resp.Incident.CreatedOn = "2026-09-21T12:00:00Z"
			return resp, nil
		},
	}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, nil)

	resp, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snID || gotNumber != snNumber || gotCreatedBy != snCreatedBy {
		t.Errorf("CreateIncidentFromServiceNow got (%q, %q, %q), want (%q, %q, %q)",
			gotID, gotNumber, gotCreatedBy, snID, snNumber, snCreatedBy)
	}
	if resp.Incident.ID != snID || resp.Incident.Number != snNumber || resp.Incident.CreatedBy != snCreatedBy {
		t.Errorf("CreateIncident response = %+v, want identity matching ServiceNow's (%q, %q, %q)", resp.Incident, snID, snNumber, snCreatedBy)
	}
}

// TestIncidentService_CreateIncident_DoesNotRetryValidationError guards
// against wasted latency on a deterministic client error, mirroring
// TestCaseService_CreateCase_DoesNotRetryValidationError: there is no
// internal retry at all now, so a validation error (like any other SN
// error) must surface after exactly one attempt.
func TestIncidentService_CreateIncident_DoesNotRetryValidationError(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			mu.Lock()
			attempts++
			mu.Unlock()
			return domain.CreateIncidentResponse{}, &apierror.ValidationError{Msg: "category contains invalid value"}
		},
	}
	repo := &stubIncidentRepo{}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, nil)

	_, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest())
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("expected exactly 1 SN attempt (validation errors are not retried), got %d", attempts)
	}
}

// TestIncidentService_CreateIncident_RejectsUnsupportedFields guards the
// two fields with no backing column at all on this data source
// (ConfigurationItemID, AssignmentGroupID): rejecting them explicitly is
// strictly better than silently accepting and dropping them, since
// ServiceNow would already have stored them by the time Postgres is ever
// touched.
func TestIncidentService_CreateIncident_RejectsUnsupportedFields(t *testing.T) {
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			t.Fatal("ServiceNow must never be called when an unsupported field is rejected up front")
			return domain.CreateIncidentResponse{}, nil
		},
	}
	repo := &stubIncidentRepo{}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, nil)

	configItemID := "77777777-7777-7777-7777-777777777777"
	req := validCreateIncidentRequest()
	req.ConfigurationItemID = &configItemID
	if _, err := svc.CreateIncident(context.Background(), req); !asValidationError(err, new(*apierror.ValidationError)) {
		t.Errorf("expected *apierror.ValidationError for configurationItemId, got %T: %v", err, err)
	}

	assignmentGroupID := "88888888-8888-8888-8888-888888888888"
	req2 := validCreateIncidentRequest()
	req2.AssignmentGroupID = &assignmentGroupID
	if _, err := svc.CreateIncident(context.Background(), req2); !asValidationError(err, new(*apierror.ValidationError)) {
		t.Errorf("expected *apierror.ValidationError for assignmentGroupId, got %T: %v", err, err)
	}
}

// TestIncidentService_CreateIncident_PublishesOnlyAfterPostgresSucceeds is
// the regression guard for CodeRabbit's finding on PR #1922: the mirror's
// own automatic publish must be suppressed (constructed with publisher=nil
// in routes.go) so incident.created only ever fires from here, after
// CreateIncidentFromServiceNow has actually confirmed the Postgres row --
// never right after the ServiceNow POST, which a consumer could observe
// before the Postgres-backed read API (the only one live in this mode) can
// return anything for it.
func TestIncidentService_CreateIncident_PublishesOnlyAfterPostgresSucceeds(t *testing.T) {
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = "55555555-5555-5555-5555-555555555555"
			resp.Incident.Number = "INC0023002"
			resp.Incident.CreatedBy = "jane.doe@example.com"
			return resp, nil
		},
	}
	repo := &stubIncidentRepo{
		createIncidentFromServiceNow: func(_ context.Context, _ domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error) {
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = id
			resp.Incident.Number = number
			resp.Incident.CreatedBy = createdBy
			return resp, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, publisher)

	if _, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected exactly 1 publish call after Postgres success, got %d", len(publisher.calls))
	}
	if publisher.calls[0].eventType != events.TypeIncidentCreated || publisher.calls[0].entityID != "55555555-5555-5555-5555-555555555555" {
		t.Errorf("unexpected publish call: %+v", publisher.calls[0])
	}
}

// TestIncidentService_CreateIncident_DoesNotPublishWhenPostgresFails proves
// the other half: if ServiceNow already has the incident but the Postgres
// insert fails (real drift, logged separately), no event fires -- a
// consumer must never see incident.created for an incident the
// Postgres-backed read API cannot return.
func TestIncidentService_CreateIncident_DoesNotPublishWhenPostgresFails(t *testing.T) {
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = "66666666-6666-6666-6666-666666666666"
			resp.Incident.Number = "INC0023003"
			resp.Incident.CreatedBy = "jane.doe@example.com"
			return resp, nil
		},
	}
	repo := &stubIncidentRepo{
		createIncidentFromServiceNow: func(context.Context, domain.CreateIncidentRequest, string, string, string) (domain.CreateIncidentResponse, error) {
			return domain.CreateIncidentResponse{}, errors.New("postgres insert failed")
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithSNMirror(repo, mirror, publisher)

	if _, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest()); err == nil {
		t.Fatal("expected an error when the Postgres insert fails")
	}

	if len(publisher.calls) != 0 {
		t.Errorf("expected 0 publish calls when Postgres fails after a ServiceNow success, got %d: %+v", len(publisher.calls), publisher.calls)
	}
}
