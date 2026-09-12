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
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// TestSNCaseService_UpdateCase_PublishesSeverityChanged verifies the happy
// path: a Severity-only PATCH that genuinely changes the case's severity
// publishes case.severity_changed with both the old (pre-PATCH, from
// newTestUpdateCaseClient's GetCaseByID enrichment) and new (from the PATCH
// response) severities, the enriched case's product, and the watch list's
// emails as Recipients — mirroring
// TestSNCaseService_UpdateCase_PublishesStatusChanged's own shape.
func TestSNCaseService_UpdateCase_PublishesSeverityChanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	productSysid := sysid32('d')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-030",
		"number": "CS0030001",
		"title": "Severity change test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"product": {"id": "` + productSysid + `", "name": "WSO2 API Manager"},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"},
		"account": {"id": "` + projectSysid + `", "name": "Account Zeta", "type": "customer", "creTeam": {"id": "` + sysid32('e') + `", "name": "Team Nova"}},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 13, "label": "4 - Low"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newSeverity := domain.CaseSeverityLow
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(publisher.calls))
	}
	call := publisher.calls[0]
	if call.eventType != events.TypeSeverityChanged {
		t.Errorf("eventType = %q, want %q", call.eventType, events.TypeSeverityChanged)
	}
	if call.entityID != caseID {
		t.Errorf("entityID = %q, want %q", call.entityID, caseID)
	}

	var payload events.SeverityChangedPayload
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("decode published payload: %v", err)
	}
	if payload.OldSeverity != "HIGH" {
		t.Errorf("oldSeverity = %q, want %q", payload.OldSeverity, "HIGH")
	}
	if payload.NewSeverity != "LOW" {
		t.Errorf("newSeverity = %q, want %q", payload.NewSeverity, "LOW")
	}
	if payload.Product != "WSO2 API Manager" {
		t.Errorf("product = %q, want %q", payload.Product, "WSO2 API Manager")
	}
	if payload.Team != "Team Nova" {
		t.Errorf("team = %q, want %q", payload.Team, "Team Nova")
	}
	wantProjectID := sysidToUUID(projectSysid)
	if payload.ProjectID != wantProjectID {
		t.Errorf("projectId = %q, want %q", payload.ProjectID, wantProjectID)
	}
	if payload.CaseNumber != "CS0030001" {
		t.Errorf("caseNumber = %q, want %q", payload.CaseNumber, "CS0030001")
	}
	if payload.WSO2CaseID != "WSO2-030" {
		t.Errorf("wso2CaseId = %q, want %q", payload.WSO2CaseID, "WSO2-030")
	}
	if len(payload.Recipients) != 1 || payload.Recipients[0] != "john.roe@example.com" {
		t.Errorf("recipients = %v, want [john.roe@example.com]", payload.Recipients)
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenNoWatchers
// mirrors TestSNCaseService_UpdateCase_SkipsPublishWhenNoWatchers for
// case.severity_changed — a case with no watchers has nobody to email, and
// this event has no Chat-only path the way case.acknowledged does (see
// publishSeverityChanged's own doc comment), so the whole publish is
// skipped.
func TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenNoWatchers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-031",
		"number": "CS0031001",
		"title": "No watchers here",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 13, "label": "4 - Low"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newSeverity := domain.CaseSeverityLow
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call for a case with no watchers, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenUnchanged is
// the severity-change equivalent of
// TestSNCaseService_UpdateCase_SkipsPublishWhenStateUnchanged: a caller
// re-PATCHing the case's own current severity must not send every watcher a
// false "severity changed" email/Chat alert.
func TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenUnchanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-032",
		"number": "CS0032001",
		"title": "Same severity re-PATCH",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 11, "label": "2 - High"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	sameSeverity := domain.CaseSeverityHigh
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &sameSeverity}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when the severity didn't actually change, got %d", len(publisher.calls))
	}
}

// TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenPATCHResponseEchoesStaleSeverity
// is a regression test for a CodeRabbit-flagged edge case: the pre-PATCH
// GetCaseByID enrichment confirms the request asks for a genuinely
// different severity (so publishSeverityChange is true), but if
// ServiceNow's PATCH response itself echoes back the pre-update severity
// (e.g. a stale read), publishing anyway would send a false
// case.severity_changed event with identical oldSeverity/newSeverity. The
// PATCH response here deliberately echoes the case's OLD severity ("2 -
// High") even though the request asked for Low, simulating that stale
// response.
func TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenPATCHResponseEchoesStaleSeverity(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	watcherSysid := sysid32('c')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-033",
		"number": "CS0033001",
		"title": "Stale PATCH response",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"createdByFullName": "Jane Doe",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"},
		"watchList": [
			{"id": "` + watcherSysid + `", "userName": "jroe", "name": "John Roe", "email": "john.roe@example.com"}
		]
	}`
	// The request asks for Low, but the PATCH response echoes back the
	// case's pre-update severity (High) unchanged.
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 11, "label": "2 - High"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	publisher := &mockEventPublisher{}
	svc := NewServiceNowCaseService(client, nil, publisher, noopSLAClockService{}, nil, "", nil)

	newSeverity := domain.CaseSeverityLow
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}

	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(publisher.calls) != 0 {
		t.Fatalf("expected no publish call when the PATCH response echoes the pre-update severity unchanged, got %d", len(publisher.calls))
	}
}
