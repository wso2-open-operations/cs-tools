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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// These three tests are the plain-Postgres-path counterparts of
// TestSNCaseService_CreateCase_RegistersSLAClocks/
// _NonCaseTypeSkipsSLARegistration/_NilSLAEngineIsNoOp
// (sn_case_service_sla_test.go) — a real, reported gap this closes: before
// WithSLAEngine existed, caseService.CreateCase (the NewCaseService path,
// used for plain DATA_SOURCE=postgres with no ServiceNow mirror at all)
// never called RegisterCaseClocks at all, so such a case got no SLA
// tracking whatsoever from creation onward, regardless of severity.
// fakeSLAEngineService is shared from sn_case_service_sla_test.go (same
// package).

func validCaseServiceCreateCaseRequest() domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type:              "case",
		CreatedBy:         "system",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Cannot log in",
		Description:       "Login fails with a 500",
		Severity:          domain.CaseSeverityHigh,
		IssueType:         domain.CaseIssueTypeQuestion,
	}
}

func TestCaseService_CreateCase_RegistersSLAClocks(t *testing.T) {
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1"}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := WithSLAEngine(
		NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		slaEngine,
	)

	req := validCaseServiceCreateCaseRequest()
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}

	if len(slaEngine.registerCalls) != 1 {
		t.Fatalf("RegisterCaseClocks calls = %d, want 1", len(slaEngine.registerCalls))
	}
	call := slaEngine.registerCalls[0]
	if call.caseID != "case-1" {
		t.Errorf("caseID = %q, want %q", call.caseID, "case-1")
	}
	if call.severity == nil || *call.severity != domain.CaseSeverityHigh {
		t.Errorf("severity = %v, want %v", call.severity, domain.CaseSeverityHigh)
	}
	if call.projectID != testProjectUUID {
		t.Errorf("projectID = %q, want %q", call.projectID, testProjectUUID)
	}
}

func TestCaseService_CreateCase_NonCaseTypeSkipsSLARegistration(t *testing.T) {
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1"}, nil
		},
	}
	slaEngine := &fakeSLAEngineService{}
	svc := WithSLAEngine(
		NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil),
		slaEngine,
	)

	req := domain.CreateCaseRequest{
		Type:                  "engagement",
		CreatedBy:             "system",
		ProjectID:             testProjectUUID,
		DeploymentID:          testDeploymentUUID,
		DeployedProductID:     testDeployedProdID,
		Subject:               "Migration planning",
		Description:           "Plan the migration",
		EngagementType:        domain.EngagementTypeMigration,
		EngagementPaymentType: domain.EngagementPaymentTypePaid,
	}
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}

	if len(slaEngine.registerCalls) != 0 {
		t.Errorf("RegisterCaseClocks calls = %d, want 0 for a non-case work item type", len(slaEngine.registerCalls))
	}
}

// TestCaseService_CreateCase_NilSLAEngineIsNoOp verifies CreateCase never
// panics/fails when s.slaEngine is nil -- the normal state for a deployment
// with CSM_SLA_ENGINE_ENABLED=false, or where WithSLAEngine was simply never
// called (every pre-existing NewCaseService call site, before this change).
func TestCaseService_CreateCase_NilSLAEngineIsNoOp(t *testing.T) {
	repo := &stubCaseRepo{
		createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
			return domain.Case{ID: "case-1"}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)

	req := validCaseServiceCreateCaseRequest()
	if _, err := svc.CreateCase(context.Background(), req); err != nil {
		t.Fatalf("CreateCase() error = %v", err)
	}
}
