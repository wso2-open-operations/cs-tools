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
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// mockEventPublisher is a minimal EventPublisherService test double that
// records every Publish call rather than actually reaching Event Hub.
type mockEventPublisher struct {
	calls []mockPublishCall
	err   error
}

type mockPublishCall struct {
	eventType events.Type
	entityID  string
	payload   json.RawMessage
}

func (m *mockEventPublisher) Publish(_ context.Context, eventType events.Type, entityID string, payload json.RawMessage) error {
	m.calls = append(m.calls, mockPublishCall{eventType, entityID, payload})
	return m.err
}

func (m *mockEventPublisher) Close() {}

// newTestCreateCaseClient stubs both requests publishCaseCreated triggers
// after a successful create: the POST /cases create call itself, then the
// GetCaseByID enrichment (a GET /cases/{id}, plus a GET /cases/{id}/tags
// listCaseTags always issues — stubbed empty here since it's incidental to
// these tests). getCaseBody is served for the GET /cases/{id} call.
func newTestCreateCaseClient(t *testing.T, caseSysid, getCaseBody string) *integrationservice.Client {
	t.Helper()
	return newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"message": "Case created successfully",
				"case": {"id": "` + caseSysid + `", "number": "CS0009001", "createdBy": "jane.doe@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
			}`))
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			_, _ = w.Write([]byte(getCaseBody))
		}
	})
}

// TestSNCaseService_CreateCase_PublishesCaseCreated verifies the happy path:
// after a successful create, publishCaseCreated enriches via GetCaseByID and
// publishes case.created with the reporter's name, project name, case
// details, and the watch list's emails as Recipients.
func TestSNCaseService_CreateCase_PublishesCaseCreated(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	const projectSysid = "2222222222222222222222222222bbbb"
	const watcherSysid = "3333333333333333333333333333cccc"
	const productSysid = "6666666666666666666666666666ffff"
	const teamSysid = "7777777777777777777777777777aaaa"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-009",
		"number": "CS0009001",
		"title": "Cannot log in",
		"description": "Login fails with a 500",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"product": {"id": "` + productSysid + `", "name": "WSO2 API Manager"},
		"severity": {"id": 3, "label": "3 - High"},
		"state": {"id": 1, "label": "Open"},
		"account": {"id": "` + projectSysid + `", "name": "Account Zeta", "type": "customer", "creTeam": {"id": "` + teamSysid + `", "name": "Team Nova"}},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Cannot log in",
		Description:       "Login fails with a 500",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeCaseCreated {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeCaseCreated)
	}
	if call.entityID != resp.Case.ID {
		t.Errorf("entityID = %q, want the new case's id %q", call.entityID, resp.Case.ID)
	}

	var payload events.CaseCreatedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.ReporterName != "Jane Doe" {
		t.Errorf("reporterName = %q, want %q", payload.ReporterName, "Jane Doe")
	}
	if payload.ProjectName != "Project Zeta" {
		t.Errorf("projectName = %q, want %q", payload.ProjectName, "Project Zeta")
	}
	if payload.CaseID != resp.Case.ID {
		t.Errorf("caseId = %q, want %q", payload.CaseID, resp.Case.ID)
	}
	if payload.CaseTitle != "Cannot log in" {
		t.Errorf("caseTitle = %q, want %q", payload.CaseTitle, "Cannot log in")
	}
	if payload.CaseType != "CASE" {
		t.Errorf("caseType = %q, want %q", payload.CaseType, "CASE")
	}
	if payload.Priority != "HIGH" {
		t.Errorf("priority = %q, want %q", payload.Priority, "HIGH")
	}
	if payload.Product != "WSO2 API Manager" {
		t.Errorf("product = %q, want %q", payload.Product, "WSO2 API Manager")
	}
	if payload.Team != "Team Nova" {
		t.Errorf("team = %q, want %q", payload.Team, "Team Nova")
	}
	if payload.Description != "Login fails with a 500" {
		t.Errorf("description = %q, want %q", payload.Description, "Login fails with a 500")
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers verifies that a
// case created with no watchers does not publish case.created at all — sending
// one with an empty Recipients list would only be rejected downstream by
// csm-notification-service's events.Validate, so this service skips it
// proactively instead.
func TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers(t *testing.T) {
	const caseSysid = "4444444444444444444444444444dddd"
	const projectSysid = "5555555555555555555555555555eeee"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-010",
		"number": "CS0010001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "No watchers here",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCase_PublishFailureDoesNotFailCreateCase verifies
// that neither a Publish error nor a GetCaseByID enrichment error is
// returned to CreateCase's own caller — the case already exists in
// ServiceNow by that point, so a notification-side failure must not be
// reported as a failed case creation.
func TestSNCaseService_CreateCase_PublishFailureDoesNotFailCreateCase(t *testing.T) {
	const caseSysid = "6666666666666666666666666666ffff"
	const projectSysid = "7777777777777777777777777777aaaa"
	const watcherSysid = "8888888888888888888888888888bbbb"

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-011",
		"number": "CS0011001",
		"title": "Publish will fail",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	publisher := &mockEventPublisher{err: errors.New("event hub unreachable")}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Publish will fail",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("CreateCase must succeed even when publishing fails, got: %v", err)
	}
	if resp.Case.Number != "CS0009001" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("expected the publish attempt to still happen, got %d calls", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCase_NoPublisherConfigured verifies that a nil
// publisher (Event Hub not configured) is a silent no-op, not a panic —
// every pre-existing test in this package already relies on this (they
// construct NewServiceNowCaseService with a nil publisher), but this test
// pins it explicitly against a case that does have watchers, so it's
// unambiguous that skipping is due to the nil publisher and not the
// no-watchers path exercised above.
func TestSNCaseService_CreateCase_NoPublisherConfigured(t *testing.T) {
	const caseSysid = "9999999999999999999999999999cccc"

	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + caseSysid + `", "number": "CS0012001", "createdBy": "jane.doe@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})
	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseRequest{
		Type:              "case",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "No publisher configured",
		Description:       "d",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}

	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// newTestCommentClient stubs the three requests publishCommentAdded can
// trigger after a successful comment create: the POST /comments create
// call itself, the GetCaseByID enrichment (GET /cases/{id} + its always-
// issued GET /cases/{id}/tags, stubbed empty here), and the
// SearchCaseComments lookup (POST /comments/search) resolveCommentAuthorName
// uses to find the new comment's author display name.
func newTestCommentClient(t *testing.T, getCaseBody, createCommentBody, searchCommentsBody string) *integrationservice.Client {
	t.Helper()
	return newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/comments":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(createCommentBody))
		case r.Method == http.MethodPost && r.URL.Path == "/comments/search":
			_, _ = w.Write([]byte(searchCommentsBody))
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			_, _ = w.Write([]byte(getCaseBody))
		}
	})
}

// TestSNCaseService_CreateCaseComment_PublishesCommentAdded verifies the
// happy path: after a successful comment create, publishCommentAdded
// enriches the case via GetCaseByID, resolves the author's display name via
// resolveCommentAuthorName's SearchCaseComments lookup (ServiceNow's create-
// comment response has no resolved name of its own — see
// publishCommentAdded's own doc comment), and publishes case.comment_added
// with every field csm-notification-service's events.Validate requires.
func TestSNCaseService_CreateCaseComment_PublishesCommentAdded(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	commentSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)
	commentID := sysidToUUID(commentSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-020",
		"number": "CS0020001",
		"title": "Login is broken",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	createCommentBody := `{
		"message": "Comment created successfully",
		"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith"}
	}`
	searchCommentsBody := `{
		"comments": [
			{"id": "` + commentSysid + `", "referenceId": "` + caseSysid + `", "content": "Working on it", "type": "comments", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith", "createdByFullName": "Agent Smith"}
		],
		"offset": 0, "limit": 20, "totalRecords": 1
	}`

	client := newTestCommentClient(t, getCaseBody, createCommentBody, searchCommentsBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    domain.CommentTypeComment,
		Content: "Working on it",
	}

	resp, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Comment.ID != commentID {
		t.Fatalf("unexpected comment id: %s", resp.Comment.ID)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeCommentAdded {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeCommentAdded)
	}
	if call.entityID != caseID {
		t.Errorf("entityID = %q, want %q", call.entityID, caseID)
	}

	var payload events.CommentAddedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.Name != "Agent Smith" {
		t.Errorf("name = %q, want %q", payload.Name, "Agent Smith")
	}
	wantProjectID := sysidToUUID(projectSysid)
	if payload.ProjectID != wantProjectID {
		t.Errorf("projectId = %q, want %q", payload.ProjectID, wantProjectID)
	}
	if payload.CaseID != caseID {
		t.Errorf("caseId = %q, want %q", payload.CaseID, caseID)
	}
	if payload.CaseTitle != "Login is broken" {
		t.Errorf("caseTitle = %q, want %q", payload.CaseTitle, "Login is broken")
	}
	if payload.CaseComment != "Working on it" {
		t.Errorf("caseComment = %q, want %q", payload.CaseComment, "Working on it")
	}
	if payload.CommentID != commentID {
		t.Errorf("commentId = %q, want %q", payload.CommentID, commentID)
	}
	if payload.IsInternalNote {
		t.Error("isInternalNote = true, want false for a customer-visible comment")
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_CreateCaseComment_WorkNote_FiltersRecipientsToWso2Domain
// verifies that an internal note (CommentTypeWorkNote — never meant for a
// customer's eyes) only notifies watchers on WSO2's own domain, even when
// the case's watch list also has non-wso2.com watchers.
func TestSNCaseService_CreateCaseComment_WorkNote_FiltersRecipientsToWso2Domain(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	internalWatcherSysid := sysid32('c')
	customerWatcherSysid := sysid32('f')
	commentSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)
	commentID := sysidToUUID(commentSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-030",
		"number": "CS0030001",
		"title": "Login is broken",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + internalWatcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@wso2.com"},
			{"id": "` + customerWatcherSysid + `", "userName": "csmith", "name": "Cara Smith", "email": "cara.smith@acme.com"}
		]
	}`
	createCommentBody := `{
		"message": "Comment created successfully",
		"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith"}
	}`
	searchCommentsBody := `{
		"comments": [
			{"id": "` + commentSysid + `", "referenceId": "` + caseSysid + `", "content": "Internal only", "type": "work_notes", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith", "createdByFullName": "Agent Smith"}
		],
		"offset": 0, "limit": 20, "totalRecords": 1
	}`

	client := newTestCommentClient(t, getCaseBody, createCommentBody, searchCommentsBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    domain.CommentTypeWorkNote,
		Content: "Internal only",
	}

	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	var payload events.CommentAddedPayload
	if err := json.Unmarshal(publisher.calls[0].payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.CommentID != commentID {
		t.Errorf("commentId = %q, want %q", payload.CommentID, commentID)
	}
	if !payload.IsInternalNote {
		t.Error("isInternalNote = false, want true for a work_note comment")
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@wso2.com" {
		t.Errorf("recipients = %v, want only [john.roe@wso2.com] — the customer watcher must not be notified of an internal note", payload.Recipients)
	}
}

// TestSNCaseService_CreateCaseComment_WorkNote_SkipsPublishWhenNoWso2Watchers
// verifies that an internal note with no wso2.com watchers on the case
// skips publishing entirely, rather than notifying customer watchers as a
// fallback.
func TestSNCaseService_CreateCaseComment_WorkNote_SkipsPublishWhenNoWso2Watchers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	customerWatcherSysid := sysid32('f')
	commentSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-031",
		"number": "CS0031001",
		"title": "Login is broken",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + customerWatcherSysid + `", "userName": "csmith", "name": "Cara Smith", "email": "cara.smith@acme.com"}
		]
	}`
	createCommentBody := `{
		"message": "Comment created successfully",
		"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith"}
	}`
	searchCommentsBody := `{
		"comments": [
			{"id": "` + commentSysid + `", "referenceId": "` + caseSysid + `", "content": "Internal only", "type": "work_notes", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith", "createdByFullName": "Agent Smith"}
		],
		"offset": 0, "limit": 20, "totalRecords": 1
	}`

	client := newTestCommentClient(t, getCaseBody, createCommentBody, searchCommentsBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    domain.CommentTypeWorkNote,
		Content: "Internal only",
	}

	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when no wso2.com watcher is on the case, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCaseComment_SkipsPublishWhenNoWatchers mirrors
// TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers for
// case.comment_added.
func TestSNCaseService_CreateCaseComment_SkipsPublishWhenNoWatchers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	commentSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-021",
		"number": "CS0021001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
	createCommentBody := `{
		"message": "Comment created successfully",
		"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith"}
	}`

	client := newTestCommentClient(t, getCaseBody, createCommentBody, `{"comments":[],"offset":0,"limit":20,"totalRecords":0}`)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseCommentRequest{CaseID: caseID, Type: domain.CommentTypeComment, Content: "hi"}
	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_CreateCaseComment_SkipsPublishWhenAuthorNameUnresolved
// verifies that publishCommentAdded skips publishing (rather than sending an
// event with an empty Name, which events.Validate would reject anyway) when
// the new comment isn't found in resolveCommentAuthorName's bounded search —
// e.g. because the search backend hasn't indexed it yet.
func TestSNCaseService_CreateCaseComment_SkipsPublishWhenAuthorNameUnresolved(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	commentSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-022",
		"number": "CS0022001",
		"title": "Author unresolved",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	createCommentBody := `{
		"message": "Comment created successfully",
		"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 11:00:00", "createdBy": "agent.smith"}
	}`
	// The search results simply don't include the just-created comment.
	searchCommentsBody := `{"comments":[],"offset":0,"limit":20,"totalRecords":0}`

	client := newTestCommentClient(t, getCaseBody, createCommentBody, searchCommentsBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	req := domain.CreateCaseCommentRequest{CaseID: caseID, Type: domain.CommentTypeComment, Content: "hi"}
	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when the author's name can't be resolved, got %d", len(publisher.calls))
	}
}

// newTestUpdateCaseClient stubs the two requests publishStatusChanged can
// trigger after a successful update: the PATCH /cases/{id} call itself, and
// the GetCaseByID enrichment (GET /cases/{id} + its always-issued
// GET /cases/{id}/tags, stubbed empty here) publishStatusChanged uses for
// Recipients/ProjectID (snUpdateCaseResponse itself has no project
// reference — see publishStatusChanged's own doc comment).
func newTestUpdateCaseClient(t *testing.T, getCaseBody, updateCaseBody string) *integrationservice.Client {
	t.Helper()
	return newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(updateCaseBody))
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			_, _ = w.Write([]byte(getCaseBody))
		}
	})
}

// TestSNCaseService_UpdateCase_PublishesStatusChanged verifies the happy
// path: a State-only PATCH publishes case.status_changed with the updated
// state's raw ServiceNow label, the enriched case's project id, and the
// watch list's emails as Recipients.
func TestSNCaseService_UpdateCase_PublishesStatusChanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-023",
		"number": "CS0023001",
		"title": "State change test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "state": {"id": 10, "label": "Work In Progress"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeStatusChanged {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeStatusChanged)
	}
	if call.entityID != caseID {
		t.Errorf("entityID = %q, want %q", call.entityID, caseID)
	}

	var payload events.StatusChangedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.NewStatus != "Work In Progress" {
		t.Errorf("newStatus = %q, want %q", payload.NewStatus, "Work In Progress")
	}
	wantProjectID := sysidToUUID(projectSysid)
	if payload.ProjectID != wantProjectID {
		t.Errorf("projectId = %q, want %q", payload.ProjectID, wantProjectID)
	}
	if payload.CaseID != caseID {
		t.Errorf("caseId = %q, want %q", payload.CaseID, caseID)
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishWhenNoWatchers mirrors
// TestSNCaseService_CreateCase_SkipsPublishWhenNoWatchers for
// case.status_changed.
func TestSNCaseService_UpdateCase_SkipsPublishWhenNoWatchers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-024",
		"number": "CS0024001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "state": {"id": 10, "label": "Work In Progress"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishWhenStateUnchanged is a
// regression test for a real false-positive-notification bug CodeRabbit
// flagged: a PATCH that re-sends the case's own current state (e.g. a
// caller resubmitting a resolution code without actually changing state)
// must not send every watcher a false "status changed" email. UpdateCase
// itself must still succeed — only the publish is skipped.
func TestSNCaseService_UpdateCase_SkipsPublishWhenStateUnchanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-026",
		"number": "CS0026001",
		"title": "Same state re-PATCH",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 10, "label": "Work In Progress"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "state": {"id": 10, "label": "Work In Progress"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when the state didn't actually change, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_DoesNotPublishStatusChangedForOtherFields
// verifies that a PATCH not touching State (e.g. assigneeEmail) never
// triggers publishStatusChanged at all — status_changed is specifically a
// State-change event, not a catch-all "case updated" one. This fixture's
// case has no watchList, so its own case.assigned publish (see the
// dedicated tests below) is skipped too, but for an unrelated reason —
// not proof status_changed and case.assigned are mutually exclusive in
// general, just that this particular assigneeEmail PATCH triggers neither.
func TestSNCaseService_UpdateCase_DoesNotPublishStatusChangedForOtherFields(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	assigneeSysid := sysid32('e')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-025",
		"number": "CS0025001",
		"title": "Assignee change test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "assignedTo": {"id": "` + assigneeSysid + `", "name": "Alex Assignee"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	assigneeEmail := "alex@example.com"
	req := domain.UpdateCaseRequest{ID: caseID, AssigneeEmail: &assigneeEmail}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a non-State update, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_PublishesCaseAssigned verifies the happy
// path: an AssigneeEmail-only PATCH publishes case.assigned with the new
// assignee's own name (resolved by ServiceNow in the PATCH response) and
// email (the exact value the caller requested — see publishCaseAssigned's
// own doc comment for why this service has no way to resolve who
// performed the assignment instead), the enriched case's project id, and
// the watch list's emails as Recipients.
func TestSNCaseService_UpdateCase_PublishesCaseAssigned(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	assigneeSysid := sysid32('e')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-027",
		"number": "CS0027001",
		"title": "Assignee change test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "assignedTo": {"id": "` + assigneeSysid + `", "name": "Alex Assignee"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	assigneeEmail := "alex@example.com"
	req := domain.UpdateCaseRequest{ID: caseID, AssigneeEmail: &assigneeEmail}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeCaseAssigned {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeCaseAssigned)
	}
	if call.entityID != caseID {
		t.Errorf("entityID = %q, want %q", call.entityID, caseID)
	}

	var payload events.CaseAssignedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.AssigneeName != "Alex Assignee" {
		t.Errorf("assigneeName = %q, want %q", payload.AssigneeName, "Alex Assignee")
	}
	if payload.AssigneeEmail != assigneeEmail {
		t.Errorf("assigneeEmail = %q, want %q", payload.AssigneeEmail, assigneeEmail)
	}
	wantProjectID := sysidToUUID(projectSysid)
	if payload.ProjectID != wantProjectID {
		t.Errorf("projectId = %q, want %q", payload.ProjectID, wantProjectID)
	}
	if payload.CaseID != caseID {
		t.Errorf("caseId = %q, want %q", payload.CaseID, caseID)
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishCaseAssignedWhenNoWatchers
// mirrors TestSNCaseService_UpdateCase_SkipsPublishWhenNoWatchers for
// case.assigned.
func TestSNCaseService_UpdateCase_SkipsPublishCaseAssignedWhenNoWatchers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	assigneeSysid := sysid32('e')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-028",
		"number": "CS0028001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "assignedTo": {"id": "` + assigneeSysid + `", "name": "Alex Assignee"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	assigneeEmail := "alex@example.com"
	req := domain.UpdateCaseRequest{ID: caseID, AssigneeEmail: &assigneeEmail}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishCaseAssignedWhenAssigneeUnchanged
// is the assigneeEmail-path mirror of
// TestSNCaseService_UpdateCase_SkipsPublishWhenStateUnchanged: a PATCH
// re-sending the case's own current assignee must not send every watcher a
// false "case assigned" email. UpdateCase itself must still succeed — only
// the publish is skipped.
func TestSNCaseService_UpdateCase_SkipsPublishCaseAssignedWhenAssigneeUnchanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	assigneeSysid := sysid32('e')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-029",
		"number": "CS0029001",
		"title": "Same assignee re-PATCH",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"},
		"assignedEngineer": {"id": "` + assigneeSysid + `", "name": "Alex Assignee", "email": "alex@example.com"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "assignedTo": {"id": "` + assigneeSysid + `", "name": "Alex Assignee"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	assigneeEmail := "alex@example.com"
	req := domain.UpdateCaseRequest{ID: caseID, AssigneeEmail: &assigneeEmail}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when the assignee didn't actually change, got %d", len(publisher.calls))
	}
}
