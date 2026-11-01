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
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeSLAEngineService records every call this file's tests care about,
// standing in for the real repository-backed SLAEngineService (see
// sla_engine_service_test.go for that one's own dedicated tests) so these
// tests only verify snCaseService's own hook wiring: which severity/
// projectId/caseId/state actually reach SLAEngineService, not the
// resolver/repository logic behind it.
type fakeSLAEngineService struct {
	registerCalls     []fakeSLARegisterCall
	reviseCalls       []fakeSLARegisterCall
	completeCalls     []string // caseID (response clock)
	fieldEffectsCalls []string // caseID (workaround/fix-ETA field re-evaluation)
	stateCalls        []fakeSLAStateCall

	registerIncidentCalls []fakeSLAIncidentRegisterCall
	reviseIncidentCalls   []fakeSLAIncidentRegisterCall
	incidentStateCalls    []fakeSLAIncidentStateCall
}

type fakeSLARegisterCall struct {
	caseID    string
	severity  *domain.CaseSeverity
	projectID string
}

type fakeSLAStateCall struct {
	caseID string
	state  domain.CaseState
}

type fakeSLAIncidentRegisterCall struct {
	incidentID string
	priority   *domain.IncidentPriority
}

type fakeSLAIncidentStateCall struct {
	incidentID string
	state      domain.IncidentState
}

func (f *fakeSLAEngineService) RegisterCaseClocks(_ context.Context, caseID string, severity *domain.CaseSeverity, projectID string) {
	f.registerCalls = append(f.registerCalls, fakeSLARegisterCall{caseID, severity, projectID})
}

func (f *fakeSLAEngineService) ReviseCaseClocks(_ context.Context, caseID string, severity *domain.CaseSeverity, projectID string) {
	f.reviseCalls = append(f.reviseCalls, fakeSLARegisterCall{caseID, severity, projectID})
}

func (f *fakeSLAEngineService) CompleteResponseClock(_ context.Context, caseID string) {
	f.completeCalls = append(f.completeCalls, caseID)
}

func (f *fakeSLAEngineService) ApplyCaseFieldEffects(_ context.Context, caseID string) {
	f.fieldEffectsCalls = append(f.fieldEffectsCalls, caseID)
}

func (f *fakeSLAEngineService) ApplyCaseStateEffects(_ context.Context, caseID string, state domain.CaseState) {
	f.stateCalls = append(f.stateCalls, fakeSLAStateCall{caseID, state})
}

func (f *fakeSLAEngineService) RegisterIncidentClocks(_ context.Context, incidentID string, priority *domain.IncidentPriority) {
	f.registerIncidentCalls = append(f.registerIncidentCalls, fakeSLAIncidentRegisterCall{incidentID, priority})
}

func (f *fakeSLAEngineService) ReviseIncidentClocks(_ context.Context, incidentID string, priority *domain.IncidentPriority) {
	f.reviseIncidentCalls = append(f.reviseIncidentCalls, fakeSLAIncidentRegisterCall{incidentID, priority})
}

func (f *fakeSLAEngineService) ApplyIncidentStateEffects(_ context.Context, incidentID string, state domain.IncidentState) {
	f.incidentStateCalls = append(f.incidentStateCalls, fakeSLAIncidentStateCall{incidentID, state})
}

// TestSNCaseService_CreateCase_RegistersSLAClocks verifies CreateCase, for
// req.Type == "case", re-fetches the created case and calls
// SLAEngineService.RegisterCaseClocks with its resolved severity and
// project id -- reusing publish_test.go's own newTestCreateCaseClient
// fixture (POST /cases then GET /cases/{id}).
func TestSNCaseService_CreateCase_RegistersSLAClocks(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	const projectSysid = "2222222222222222222222222222bbbb"

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
		"severity": {"id": 3, "label": "3 - High"},
		"state": {"id": 1, "label": "Open"}
	}`

	client := newTestCreateCaseClient(t, caseSysid, getCaseBody)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

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
	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}

	if len(slaEngine.registerCalls) != 1 {
		t.Fatalf("RegisterCaseClocks calls = %d, want 1", len(slaEngine.registerCalls))
	}
	call := slaEngine.registerCalls[0]
	if call.caseID != sysidToUUID(caseSysid) {
		t.Errorf("caseID = %q, want %q", call.caseID, sysidToUUID(caseSysid))
	}
	if call.severity == nil || *call.severity != domain.CaseSeverityHigh {
		t.Errorf("severity = %v, want %s", call.severity, domain.CaseSeverityHigh)
	}
	if call.projectID != sysidToUUID(projectSysid) {
		t.Errorf("projectID = %q, want %q", call.projectID, sysidToUUID(projectSysid))
	}
}

// TestSNCaseService_CreateCase_NonCaseTypeSkipsSLARegistration verifies
// non-"case" work item types (engagement, service_request, ...) never
// reach SLAEngineService at all -- this engine's resolver only understands
// P0-P3/Query support-case severities, see sla_policy_resolver.go's own
// doc comment on scope.
func TestSNCaseService_CreateCase_NonCaseTypeSkipsSLARegistration(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	client := newTestCreateCaseClient(t, caseSysid, `{"id": "`+caseSysid+`"}`)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	req := domain.CreateCaseRequest{
		Type:                  "engagement",
		ProjectID:             testProjectUUID,
		DeploymentID:          testDeploymentUUID,
		DeployedProductID:     testDeployedProdID,
		Subject:               "Migration planning",
		Description:           "Plan the migration",
		EngagementType:        domain.EngagementTypeMigration,
		EngagementPaymentType: domain.EngagementPaymentTypePaid,
	}
	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}

	if len(slaEngine.registerCalls) != 0 {
		t.Errorf("RegisterCaseClocks calls = %d, want 0 for a non-case work item type", len(slaEngine.registerCalls))
	}
}

// TestSNCaseService_CreateCase_NilSLAEngineIsNoOp verifies CreateCase never
// panics/fails when s.slaEngine is nil (no database configured — see
// snCaseService.slaEngine's own doc comment).
func TestSNCaseService_CreateCase_NilSLAEngineIsNoOp(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	client := newTestCreateCaseClient(t, caseSysid, `{"id": "`+caseSysid+`", "severity": {"id": 3, "label": "3 - High"}}`)
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)

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
	if _, err := svc.CreateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}
}

func TestSNCaseService_CreateCaseComment_CompletesResponseClockForCSEngineerReply(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	const commentSysid = "4444444444444444444444444444dddd"
	const engineerSysid = "5555555555555555555555555555eeee"

	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Comment created",
			"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 10:05:00", "createdBy": "engineer@example.com"}
		}`))
	})
	mux.HandleFunc("/comments/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"comments": [
				{"id": "` + commentSysid + `", "type": "comment", "createdOn": "2026-01-02 10:05:00", "createdBy": "engineer@example.com", "createdByUser": {"id": "` + engineerSysid + `", "name": "Engineer", "email": "engineer@example.com"}}
			],
			"totalRecords": 1
		}`))
	})
	mux.HandleFunc("/users/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users": [{"id": "` + engineerSysid + `", "email": "engineer@example.com", "roles": ["cs_engineer"]}], "totalRecords": 1}`))
	})
	mux.HandleFunc("/cases/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "` + caseSysid + `", "state": {"id": 1, "label": "Open"}}`))
	})

	client := newTestSNClient(t, mux)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, NewServiceNowUserService(client), nil, "cs_engineer", slaEngine)

	req := domain.CreateCaseCommentRequest{
		CaseID:  sysidToUUID(caseSysid),
		Type:    domain.CommentTypeComment,
		Content: "We've identified the root cause.",
	}
	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateCaseComment() error = %v", err)
	}

	if len(slaEngine.completeCalls) != 1 || slaEngine.completeCalls[0] != sysidToUUID(caseSysid) {
		t.Errorf("CompleteResponseClock calls = %v, want [%s]", slaEngine.completeCalls, sysidToUUID(caseSysid))
	}
}

func TestSNCaseService_CreateCaseComment_NoCSEngineerRoleConfiguredSkips(t *testing.T) {
	const caseSysid = "1111111111111111111111111111aaaa"
	const commentSysid = "4444444444444444444444444444dddd"

	mux := http.NewServeMux()
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "Comment created",
			"comment": {"id": "` + commentSysid + `", "createdOn": "2026-01-02 10:05:00", "createdBy": "engineer@example.com"}
		}`))
	})

	client := newTestSNClient(t, mux)
	slaEngine := &fakeSLAEngineService{}
	// csEngineerRole left "" (unconfigured).
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	req := domain.CreateCaseCommentRequest{
		CaseID:  sysidToUUID(caseSysid),
		Type:    domain.CommentTypeComment,
		Content: "We've identified the root cause.",
	}
	if _, err := svc.CreateCaseComment(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("CreateCaseComment() error = %v", err)
	}

	if len(slaEngine.completeCalls) != 0 {
		t.Errorf("CompleteResponseClock calls = %v, want none when CSEngineerRole is unconfigured", slaEngine.completeCalls)
	}
}

// TestSNCaseService_UpdateCase_AppliesSLAStateEffects verifies a State-only
// PATCH calls SLAEngineService.ApplyCaseStateEffects with the case id and
// the NEW (post-PATCH) state -- reusing sn_case_service_publish_test.go's
// own newTestUpdateCaseClient fixture.
func TestSNCaseService_UpdateCase_AppliesSLAStateEffects(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-023",
		"number": "CS0023001",
		"title": "State change test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
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
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.stateCalls) != 1 {
		t.Fatalf("ApplyCaseStateEffects calls = %d, want 1", len(slaEngine.stateCalls))
	}
	call := slaEngine.stateCalls[0]
	if call.caseID != caseID {
		t.Errorf("caseID = %q, want %q", call.caseID, caseID)
	}
	if call.state != domain.CaseStateWorkInProgress {
		t.Errorf("state = %q, want %q", call.state, domain.CaseStateWorkInProgress)
	}
}

// TestSNCaseService_UpdateCase_FixEtaFieldTriggersFieldEffects verifies
// UpdateCase's own wiring: setting any of the fix-ETA fields calls
// SLAEngineService.ApplyCaseFieldEffects, which re-checks every active
// clock's own condition text (the real policies' stop_condition reads
// "...ORfix_eta_sharedISNOTEMPTY", resolved from the fix-ETA values
// themselves -- see repository.CaseFacts.FixEtaShared's own doc comment) --
// not a hardcoded completion keyed on work_item.eta_shared_on, which this
// replaced.
func TestSNCaseService_UpdateCase_FixEtaFieldTriggersFieldEffects(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-023",
		"number": "CS0023001",
		"title": "Fix ETA share test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe"}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	addPublicComment := true
	mostLikely := "2026-02-01"
	product := "WSO2 API Manager"
	publicTicket := "CS0023001"
	req := domain.UpdateCaseRequest{
		ID: caseID, AddPublicComment: &addPublicComment,
		MostLikelyFixEta: &mostLikely, Product: &product, PublicTicket: &publicTicket,
	}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.fieldEffectsCalls) != 1 || slaEngine.fieldEffectsCalls[0] != caseID {
		t.Errorf("fieldEffectsCalls = %v, want [%q]", slaEngine.fieldEffectsCalls, caseID)
	}
}

// TestSNCaseService_UpdateCase_PlainFieldUpdateSkipsFieldEffects verifies
// ApplyCaseFieldEffects is only called when this PATCH actually touches
// WorkaroundProvided or a fix-ETA field -- a plain field update touching
// neither must not pay for a clock re-evaluation that can't change anything.
func TestSNCaseService_UpdateCase_PlainFieldUpdateSkipsFieldEffects(t *testing.T) {
	caseSysid := sysid32('a')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{"id": "` + caseSysid + `", "state": {"id": 1, "label": "Open"}}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe"}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	subject := "Updated subject"
	req := domain.UpdateCaseRequest{ID: caseID, Subject: &subject}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.fieldEffectsCalls) != 0 {
		t.Errorf("fieldEffectsCalls = %v, want none", slaEngine.fieldEffectsCalls)
	}
}

// TestSNCaseService_UpdateCase_NilSLAEngineSkipsStateEffects verifies the
// nil-slaEngine guard lives at the UpdateCase call site (no panic, no call)
// when no database is configured.
func TestSNCaseService_UpdateCase_NilSLAEngineSkipsStateEffects(t *testing.T) {
	caseSysid := sysid32('a')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{"id": "` + caseSysid + `", "state": {"id": 1, "label": "Open"}}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "state": {"id": 10, "label": "Work In Progress"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}
}

// TestSNCaseService_UpdateCase_RevisesSLAClocksOnSeverityChange verifies a
// Severity-only PATCH that genuinely changes the case's severity calls
// SLAEngineService.ReviseCaseClocks with the case id, the NEW (post-PATCH)
// severity, and the project id from the pre-PATCH GetCaseByID enrichment --
// mirroring TestSNCaseService_UpdateCase_PublishesSeverityChanged's own
// fixture shape (sn_case_severity_changed_test.go), but with s.publisher
// deliberately nil: reviseCaseSLAClocks must fire independent of whether
// Event Hub publishing is configured (see severityChanged's own doc comment
// in UpdateCase for why).
func TestSNCaseService_UpdateCase_RevisesSLAClocksOnSeverityChange(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-040",
		"number": "CS0040001",
		"title": "Severity change revises SLA clocks",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 9, "label": "0 - Catastrophic"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	slaEngine := &fakeSLAEngineService{}
	// s.publisher is deliberately nil -- this is the whole point of the test.
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	newSeverity := domain.CaseSeverityCatastrophic
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.reviseCalls) != 1 {
		t.Fatalf("ReviseCaseClocks calls = %d, want 1", len(slaEngine.reviseCalls))
	}
	call := slaEngine.reviseCalls[0]
	if call.caseID != caseID {
		t.Errorf("caseID = %q, want %q", call.caseID, caseID)
	}
	if call.severity == nil || *call.severity != domain.CaseSeverityCatastrophic {
		t.Errorf("severity = %v, want %s", call.severity, domain.CaseSeverityCatastrophic)
	}
	if call.projectID != sysidToUUID(projectSysid) {
		t.Errorf("projectID = %q, want %q", call.projectID, sysidToUUID(projectSysid))
	}
	// RegisterCaseClocks must never be called from UpdateCase -- only from
	// CreateCase (see RegisterCaseClocks' own doc comment).
	if len(slaEngine.registerCalls) != 0 {
		t.Errorf("RegisterCaseClocks calls = %d, want 0 from UpdateCase", len(slaEngine.registerCalls))
	}
}

// TestSNCaseService_UpdateCase_SeverityEnrichmentRetriesOnceAndRecovers
// confirms the bounded retry around the pre-PATCH severity-change
// enrichment fetch: the first GetCaseByID call fails (a transient blip),
// the second succeeds, and ReviseCaseClocks still fires -- a case's SLA
// clocks must not silently stay on the old severity just because one
// attempt at detecting the change failed.
func TestSNCaseService_UpdateCase_SeverityEnrichmentRetriesOnceAndRecovers(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-041",
		"number": "CS0041001",
		"title": "Severity enrichment retry test",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 9, "label": "0 - Catastrophic"}}
	}`

	var getCalls int
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPatch:
			_, _ = w.Write([]byte(updateCaseBody))
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			getCalls++
			if getCalls == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(getCaseBody))
		}
	})
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	newSeverity := domain.CaseSeverityCatastrophic
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if getCalls < 2 {
		t.Fatalf("GET calls = %d, want at least 2 (first fails, retry succeeds)", getCalls)
	}
	if len(slaEngine.reviseCalls) != 1 {
		t.Fatalf("ReviseCaseClocks calls = %d, want 1 -- the retry should have let severity-change detection succeed", len(slaEngine.reviseCalls))
	}
	if slaEngine.reviseCalls[0].caseID != caseID {
		t.Errorf("caseID = %q, want %q", slaEngine.reviseCalls[0].caseID, caseID)
	}
}

// TestSNCaseService_UpdateCase_SkipsReviseSLAClocksWhenSeverityUnchanged
// mirrors TestSNCaseService_UpdateCase_SkipsPublishSeverityChangedWhenUnchanged:
// a caller re-PATCHing the case's own current severity (a no-op as far as
// ServiceNow is concerned) must not revise SLA clocks that never actually
// changed policy.
func TestSNCaseService_UpdateCase_SkipsReviseSLAClocksWhenSeverityUnchanged(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"internalId": "WSO2-041",
		"number": "CS0041001",
		"title": "Same severity re-PATCH",
		"description": "d",
		"createdOn": "2026-01-02 10:00:00",
		"createdBy": "jane.doe@example.com",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"deployment": {"id": "", "name": ""},
		"deployedProduct": {"id": "", "name": "", "version": ""},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 11, "label": "2 - High"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	sameSeverity := domain.CaseSeverityHigh
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &sameSeverity}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.reviseCalls) != 0 {
		t.Errorf("ReviseCaseClocks calls = %d, want 0 when severity didn't actually change", len(slaEngine.reviseCalls))
	}
}

// TestSNCaseService_UpdateCase_SkipsReviseSLAClocksWhenSeverityAbsent
// verifies a PATCH that doesn't touch severity at all (e.g. a State-only
// PATCH) never calls ReviseCaseClocks.
func TestSNCaseService_UpdateCase_SkipsReviseSLAClocksWhenSeverityAbsent(t *testing.T) {
	caseSysid := sysid32('a')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{"id": "` + caseSysid + `", "state": {"id": 1, "label": "Open"}}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "state": {"id": 10, "label": "Work In Progress"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	slaEngine := &fakeSLAEngineService{}
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", slaEngine)

	newState := domain.CaseStateWorkInProgress
	req := domain.UpdateCaseRequest{ID: caseID, State: &newState}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}

	if len(slaEngine.reviseCalls) != 0 {
		t.Errorf("ReviseCaseClocks calls = %d, want 0 for a PATCH that never touches severity", len(slaEngine.reviseCalls))
	}
}

// TestSNCaseService_UpdateCase_NilSLAEngineSkipsReviseSLAClocks verifies the
// nil-slaEngine guard also covers the severity-revision call site (no
// panic, no call) when no database is configured.
func TestSNCaseService_UpdateCase_NilSLAEngineSkipsReviseSLAClocks(t *testing.T) {
	caseSysid := sysid32('a')
	projectSysid := sysid32('b')
	caseID := sysidToUUID(caseSysid)

	getCaseBody := `{
		"id": "` + caseSysid + `",
		"project": {"id": "` + projectSysid + `", "name": "Project Zeta"},
		"severity": {"id": 11, "label": "2 - High"},
		"state": {"id": 1, "label": "Open"}
	}`
	updateCaseBody := `{
		"message": "Case updated successfully",
		"case": {"id": "` + caseSysid + `", "updatedOn": "2026-01-02 12:00:00", "updatedBy": "jane.doe", "severity": {"id": 13, "label": "4 - Low"}}
	}`

	client := newTestUpdateCaseClient(t, getCaseBody, updateCaseBody)
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)

	newSeverity := domain.CaseSeverityLow
	req := domain.UpdateCaseRequest{ID: caseID, Severity: &newSeverity}
	if _, err := svc.UpdateCase(contextWithUserIDToken("token"), req); err != nil {
		t.Fatalf("UpdateCase() error = %v", err)
	}
}
