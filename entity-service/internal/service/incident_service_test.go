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
	createIncidentComment        func(ctx context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error)
	getIncidentByID              func(ctx context.Context, id string) (domain.IncidentView, error)
}

func (s *stubIncidentRepo) SearchIncidents(context.Context, domain.SearchIncidentsRequest, []string, []string, []string, []string, *bool, *bool, *time.Time, *time.Time) ([]domain.SearchIncidentView, int, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) AggregateIncidents(context.Context, domain.SearchIncidentsRequest, []string, []string, []string, []string, *bool, *bool, *time.Time, *time.Time, string, int) (domain.AggregateResponse, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) GetIncidentByID(ctx context.Context, id string) (domain.IncidentView, error) {
	if s.getIncidentByID != nil {
		return s.getIncidentByID(ctx, id)
	}
	panic("not implemented")
}
func (s *stubIncidentRepo) SearchIncidentActivities(context.Context, domain.SearchIncidentActivitiesRequest) ([]domain.CaseActivity, int, error) {
	panic("not implemented")
}
func (s *stubIncidentRepo) CreateIncidentComment(ctx context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
	if s.createIncidentComment != nil {
		return s.createIncidentComment(ctx, incidentID, commentType, content, createdBy)
	}
	panic("not implemented")
}
func (s *stubIncidentRepo) CreateIncidentFromServiceNow(ctx context.Context, req domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error) {
	if s.createIncidentFromServiceNow != nil {
		return s.createIncidentFromServiceNow(ctx, req, id, number, createdBy)
	}
	panic("CreateIncidentFromServiceNow called unexpectedly: Postgres must stay untouched when ServiceNow never accepts the incident")
}

// stubMirrorIncidentService embeds IncidentService (nil) and overrides only
// CreateIncident/UpdateIncident -- same convention as stubMirrorCaseService
// (case_service_test.go). Any other method being called would panic on the
// nil embedded interface, which is the point: this pilot's incident mode
// only ever calls the mirror's CreateIncident (synchronously) and
// UpdateIncident (via the async writeback dispatch).
type stubMirrorIncidentService struct {
	IncidentService
	createIncident func(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error)
	updateIncident func(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error)
}

func (s *stubMirrorIncidentService) CreateIncident(ctx context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	return s.createIncident(ctx, req)
}

func (s *stubMirrorIncidentService) UpdateIncident(ctx context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
	return s.updateIncident(ctx, req)
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
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

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
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

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
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

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

// TestIncidentService_CreateIncident_RejectsConfigurationItemID guards the
// one field with no backing column at all on this data source
// (ConfigurationItemID): rejecting it explicitly is strictly better than
// silently accepting and dropping it, since ServiceNow would already have
// stored it by the time Postgres is ever touched.
func TestIncidentService_CreateIncident_RejectsConfigurationItemID(t *testing.T) {
	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			t.Fatal("ServiceNow must never be called when an unsupported field is rejected up front")
			return domain.CreateIncidentResponse{}, nil
		},
	}
	repo := &stubIncidentRepo{}
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

	configItemID := "77777777-7777-7777-7777-777777777777"
	req := validCreateIncidentRequest()
	req.ConfigurationItemID = &configItemID
	if _, err := svc.CreateIncident(context.Background(), req); !asValidationError(err, new(*apierror.ValidationError)) {
		t.Errorf("expected *apierror.ValidationError for configurationItemId, got %T: %v", err, err)
	}
}

// TestIncidentService_CreateIncident_PersistsAssignmentGroupID guards the
// fix for work_item.assignment_group_id (migration 0075): unlike
// ConfigurationItemID, this field DOES have a backing column, so it must be
// forwarded through to CreateIncidentFromServiceNow rather than rejected.
func TestIncidentService_CreateIncident_PersistsAssignmentGroupID(t *testing.T) {
	assignmentGroupID := "88888888-8888-8888-8888-888888888888"

	mirror := &stubMirrorIncidentService{
		createIncident: func(context.Context, domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = "55555555-5555-5555-5555-555555555555"
			resp.Incident.Number = "INC0023003"
			resp.Incident.CreatedBy = "jane.doe@example.com"
			return resp, nil
		},
	}
	var gotAssignmentGroupID *string
	repo := &stubIncidentRepo{
		createIncidentFromServiceNow: func(_ context.Context, req domain.CreateIncidentRequest, id, number, createdBy string) (domain.CreateIncidentResponse, error) {
			gotAssignmentGroupID = req.AssignmentGroupID
			resp := domain.CreateIncidentResponse{Message: "Incident created successfully."}
			resp.Incident.ID = id
			resp.Incident.Number = number
			resp.Incident.CreatedBy = createdBy
			return resp, nil
		},
	}
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil)

	req := validCreateIncidentRequest()
	req.AssignmentGroupID = &assignmentGroupID
	if _, err := svc.CreateIncident(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAssignmentGroupID == nil || *gotAssignmentGroupID != assignmentGroupID {
		t.Errorf("expected assignmentGroupId %q to be forwarded to CreateIncidentFromServiceNow, got %v", assignmentGroupID, gotAssignmentGroupID)
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
		// Publishing enriches the event with the escalation fields, which
		// means reading the incident back -- the same read the ServiceNow
		// path already makes, now reached from this caller too. An empty view
		// is enough: every escalation field is optional and this test is
		// about WHEN the publish happens, not what it carries.
		getIncidentByID: func(context.Context, string) (domain.IncidentView, error) {
			return domain.IncidentView{}, nil
		},
	}
	publisher := &mockEventPublisher{}
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, publisher, nil)

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
	svc := NewIncidentServiceWithSNMirror(repo, nil, mirror, publisher, nil)

	if _, err := svc.CreateIncident(context.Background(), validCreateIncidentRequest()); err == nil {
		t.Fatal("expected an error when the Postgres insert fails")
	}

	if len(publisher.calls) != 0 {
		t.Errorf("expected 0 publish calls when Postgres fails after a ServiceNow success, got %d: %+v", len(publisher.calls), publisher.calls)
	}
}

// stubUpdateIncidentUserRepo resolves any email to a fixed actor -- only
// GetUserByEmail is ever exercised by UpdateIncident's resolveActor call.
type stubUpdateIncidentUserRepo struct {
	stubUserRepo
	email string
}

func (s stubUpdateIncidentUserRepo) GetUserByEmail(_ context.Context, _ string) (domain.User, error) {
	return domain.User{Email: s.email}, nil
}

// newTestIncidentView is a minimal, valid domain.IncidentView returned by
// stubIncidentRepo.getIncidentByID in the UpdateIncident tests below --
// UpdateIncident re-reads the incident purely to build the response's
// IncidentView, so its exact contents don't matter to these tests beyond
// having a non-nil ID.
func newTestIncidentView(id string) domain.IncidentView {
	return domain.IncidentView{ID: &id}
}

// TestIncidentService_UpdateIncident_UnsupportedOnPlainDataSource guards the
// plain-PostgreSQL-only path: with no snWriteback (as NewIncidentService
// constructs), UpdateIncident must still 503, exactly as the original stub
// always did.
func TestIncidentService_UpdateIncident_UnsupportedOnPlainDataSource(t *testing.T) {
	svc := NewIncidentService(&stubIncidentRepo{})
	workNotes := "investigating"
	_, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &workNotes})
	var se *apierror.ServiceUnavailableError
	if !errors.As(err, &se) {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestIncidentService_UpdateIncident_WorkNotesOnly covers the WorkNotes-only
// path: exactly one WORK_NOTE comment row is written, and the async mirror
// dispatch carries only ID+WorkNotes (AdditionalComments left nil).
func TestIncidentService_UpdateIncident_WorkNotesOnly(t *testing.T) {
	var mu sync.Mutex
	var gotCalls []struct {
		incidentID  string
		commentType domain.CommentType
		content     string
		createdBy   string
	}
	repo := &stubIncidentRepo{
		createIncidentComment: func(_ context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
			mu.Lock()
			gotCalls = append(gotCalls, struct {
				incidentID  string
				commentType domain.CommentType
				content     string
				createdBy   string
			}{incidentID, commentType, content, createdBy})
			mu.Unlock()
			return domain.CaseComment{ID: "comment-1", CaseID: incidentID, Type: commentType, Content: content}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return newTestIncidentView(id), nil
		},
	}

	mirrorCalled := make(chan domain.UpdateIncidentRequest, 1)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrorCalled <- req
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	userRepo := stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, mirror, nil, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	workNotes := "investigating the repeat alert"
	resp, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &workNotes})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Incident.ID == nil || *resp.Incident.ID != testDeploymentUUID {
		t.Errorf("response incident ID = %v, want %q", resp.Incident.ID, testDeploymentUUID)
	}

	mu.Lock()
	if len(gotCalls) != 1 {
		mu.Unlock()
		t.Fatalf("expected exactly 1 CreateIncidentComment call, got %d", len(gotCalls))
	}
	call := gotCalls[0]
	mu.Unlock()
	if call.incidentID != testDeploymentUUID || call.commentType != domain.CommentTypeWorkNote ||
		call.content != workNotes || call.createdBy != "jane.doe@example.com" {
		t.Errorf("unexpected CreateIncidentComment call: %+v", call)
	}

	select {
	case mirrorReq := <-mirrorCalled:
		if mirrorReq.ID != testDeploymentUUID {
			t.Errorf("mirror got ID %q, want %q", mirrorReq.ID, testDeploymentUUID)
		}
		if mirrorReq.WorkNotes == nil || *mirrorReq.WorkNotes != workNotes {
			t.Errorf("mirror got WorkNotes %v, want %q", mirrorReq.WorkNotes, workNotes)
		}
		if mirrorReq.AdditionalComments != nil {
			t.Errorf("mirror got AdditionalComments %v, want nil", mirrorReq.AdditionalComments)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateIncident was never called")
	}
}

// TestIncidentService_UpdateIncident_NoForwardedTokenFallsBackToSystemActor
// pins the scenario this whole UpdateIncident extension exists for: an M2M
// caller with no end-user identity to forward at all (a machine client via
// csm-integration-service, both M2M-only by design -- see resolveActor's own
// doc comment). Unlike caseService.resolveActor, this must NOT 401 when
// context.Background() carries no x-user-id-token -- it must succeed, using
// incidentSystemActorEmail as comment.created_by.
func TestIncidentService_UpdateIncident_NoForwardedTokenFallsBackToSystemActor(t *testing.T) {
	var gotCreatedBy string
	repo := &stubIncidentRepo{
		createIncidentComment: func(_ context.Context, _ string, _ domain.CommentType, _ string, createdBy string) (domain.CaseComment, error) {
			gotCreatedBy = createdBy
			return domain.CaseComment{ID: "comment-1"}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return newTestIncidentView(id), nil
		},
	}
	mirrorCalled := make(chan domain.UpdateIncidentRequest, 1)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrorCalled <- req
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	// GetUserByEmail must never be called on this path -- fail the test if it is.
	userRepo := stubUpdateIncidentUserRepo{email: ""}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, mirror, nil, dispatcher)

	workNotes := "auto-attached repeat alert"
	resp, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &workNotes})
	if err != nil {
		t.Fatalf("unexpected error with no forwarded token: %v", err)
	}
	if resp.Incident.ID == nil || *resp.Incident.ID != testDeploymentUUID {
		t.Errorf("response incident ID = %v, want %q", resp.Incident.ID, testDeploymentUUID)
	}
	if gotCreatedBy != incidentSystemActorEmail {
		t.Errorf("createdBy = %q, want the system actor fallback %q", gotCreatedBy, incidentSystemActorEmail)
	}

	select {
	case <-mirrorCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateIncident was never called")
	}
}

// TestIncidentService_UpdateIncident_AdditionalCommentsOnly is
// WorkNotesOnly's mirror image: exactly one COMMENT row, mirror dispatch
// carries only AdditionalComments.
func TestIncidentService_UpdateIncident_AdditionalCommentsOnly(t *testing.T) {
	var mu sync.Mutex
	var gotType domain.CommentType
	var callCount int
	repo := &stubIncidentRepo{
		createIncidentComment: func(_ context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
			mu.Lock()
			gotType = commentType
			callCount++
			mu.Unlock()
			return domain.CaseComment{ID: "comment-1", CaseID: incidentID, Type: commentType, Content: content}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return newTestIncidentView(id), nil
		},
	}
	mirrorCalled := make(chan domain.UpdateIncidentRequest, 1)
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			mirrorCalled <- req
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	userRepo := stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, mirror, nil, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	comments := "customer notified"
	if _, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, AdditionalComments: &comments}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	if callCount != 1 || gotType != domain.CommentTypeComment {
		mu.Unlock()
		t.Fatalf("expected exactly 1 CreateIncidentComment call with type COMMENT, got %d calls, type %v", callCount, gotType)
	}
	mu.Unlock()

	select {
	case mirrorReq := <-mirrorCalled:
		if mirrorReq.AdditionalComments == nil || *mirrorReq.AdditionalComments != comments {
			t.Errorf("mirror got AdditionalComments %v, want %q", mirrorReq.AdditionalComments, comments)
		}
		if mirrorReq.WorkNotes != nil {
			t.Errorf("mirror got WorkNotes %v, want nil", mirrorReq.WorkNotes)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateIncident was never called")
	}
}

// TestIncidentService_UpdateIncident_BothFieldsWriteTwoComments covers both
// WorkNotes and AdditionalComments set in the same call: two comment rows,
// one of each type.
func TestIncidentService_UpdateIncident_BothFieldsWriteTwoComments(t *testing.T) {
	var mu sync.Mutex
	var gotTypes []domain.CommentType
	repo := &stubIncidentRepo{
		createIncidentComment: func(_ context.Context, incidentID string, commentType domain.CommentType, content, createdBy string) (domain.CaseComment, error) {
			mu.Lock()
			gotTypes = append(gotTypes, commentType)
			mu.Unlock()
			return domain.CaseComment{ID: "comment-" + string(commentType), CaseID: incidentID, Type: commentType, Content: content}, nil
		},
		getIncidentByID: func(_ context.Context, id string) (domain.IncidentView, error) {
			return newTestIncidentView(id), nil
		},
	}
	mirror := &stubMirrorIncidentService{
		updateIncident: func(_ context.Context, req domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			return domain.UpdateIncidentResponse{}, nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	userRepo := stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, mirror, nil, dispatcher)

	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	workNotes := "internal note"
	comments := "customer-visible update"
	if _, err := svc.UpdateIncident(ctx, domain.UpdateIncidentRequest{ID: testDeploymentUUID, WorkNotes: &workNotes, AdditionalComments: &comments}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gotTypes) != 2 {
		t.Fatalf("expected 2 CreateIncidentComment calls, got %d: %v", len(gotTypes), gotTypes)
	}
	hasWorkNote, hasComment := false, false
	for _, ty := range gotTypes {
		if ty == domain.CommentTypeWorkNote {
			hasWorkNote = true
		}
		if ty == domain.CommentTypeComment {
			hasComment = true
		}
	}
	if !hasWorkNote || !hasComment {
		t.Errorf("expected one WORK_NOTE and one COMMENT, got %v", gotTypes)
	}
}

// TestIncidentService_UpdateIncident_RejectsUnsupportedFields is a
// table-driven guard over a representative subset of the fields this data
// source does not support on UPDATE -- each must be rejected with a
// ValidationError before any repository or mirror call happens.
func TestIncidentService_UpdateIncident_RejectsUnsupportedFields(t *testing.T) {
	repo := &stubIncidentRepo{
		createIncidentComment: func(context.Context, string, domain.CommentType, string, string) (domain.CaseComment, error) {
			panic("CreateIncidentComment must not be called when an unsupported field is rejected up front")
		},
	}
	mirror := &stubMirrorIncidentService{
		updateIncident: func(context.Context, domain.UpdateIncidentRequest) (domain.UpdateIncidentResponse, error) {
			panic("mirror.UpdateIncident must not be called when an unsupported field is rejected up front")
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	userRepo := stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, mirror, nil, dispatcher)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	subject := "new subject"
	state := domain.IncidentStateInProgress
	priority := domain.IncidentPriorityHigh
	assignmentGroupID := "77777777-7777-7777-7777-777777777777"
	watchList := []string{testDeploymentUUID}

	tests := []struct {
		name string
		req  domain.UpdateIncidentRequest
	}{
		{name: "subject", req: domain.UpdateIncidentRequest{ID: testDeploymentUUID, Subject: &subject}},
		{name: "state", req: domain.UpdateIncidentRequest{ID: testDeploymentUUID, State: &state}},
		{name: "priority", req: domain.UpdateIncidentRequest{ID: testDeploymentUUID, Priority: &priority}},
		{name: "assignmentGroupId", req: domain.UpdateIncidentRequest{ID: testDeploymentUUID, AssignmentGroupID: &assignmentGroupID}},
		{name: "watchList", req: domain.UpdateIncidentRequest{ID: testDeploymentUUID, WatchList: &watchList}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateIncident(ctx, tc.req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError for %s, got %T: %v", tc.name, err, err)
			}
		})
	}
}

// TestIncidentService_UpdateIncident_RequiresAtLeastOneField guards the
// "neither WorkNotes nor AdditionalComments set" case -- a request with no
// rejected fields either (an otherwise-empty UpdateIncidentRequest) must
// still fail, not silently no-op.
func TestIncidentService_UpdateIncident_RequiresAtLeastOneField(t *testing.T) {
	repo := &stubIncidentRepo{
		createIncidentComment: func(context.Context, string, domain.CommentType, string, string) (domain.CaseComment, error) {
			panic("CreateIncidentComment must not be called when neither field is set")
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	userRepo := stubUpdateIncidentUserRepo{email: "jane.doe@example.com"}
	svc := NewIncidentServiceWithSNMirror(repo, userRepo, &stubMirrorIncidentService{}, nil, dispatcher)

	_, err := svc.UpdateIncident(context.Background(), domain.UpdateIncidentRequest{ID: testDeploymentUUID})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}
