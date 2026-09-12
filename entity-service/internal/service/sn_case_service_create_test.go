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
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testProjectUUID = "66666666-6666-6666-6666-666666666666"

// TestSNCaseService_CreateCase_EngagementValidation verifies that an
// engagement CreateCaseRequest requires subject, description, and a valid
// engagementType, matching the ServiceNow-side validation.
func TestSNCaseService_CreateCase_EngagementValidation(t *testing.T) {
	baseReq := domain.CreateCaseRequest{
		Type:              "engagement",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
	}

	tests := []struct {
		name string
		req  domain.CreateCaseRequest
	}{
		{name: "missing subject", req: baseReq},
		{name: "missing description", req: func() domain.CreateCaseRequest { r := baseReq; r.Subject = "Migration planning"; return r }()},
		{name: "invalid engagementType", req: func() domain.CreateCaseRequest {
			r := baseReq
			r.Subject = "Migration planning"
			r.Description = "Plan the migration"
			r.EngagementType = "not_a_real_type"
			r.EngagementPaymentType = domain.EngagementPaymentTypePaid
			return r
		}()},
		{name: "missing engagementPaymentType", req: func() domain.CreateCaseRequest {
			r := baseReq
			r.Subject = "Migration planning"
			r.Description = "Plan the migration"
			r.EngagementType = domain.EngagementTypeMigration
			return r
		}()},
		{name: "invalid engagementPaymentType", req: func() domain.CreateCaseRequest {
			r := baseReq
			r.Subject = "Migration planning"
			r.Description = "Plan the migration"
			r.EngagementType = domain.EngagementTypeMigration
			r.EngagementPaymentType = "not_a_real_payment_type"
			return r
		}()},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNCaseService_CreateCase_Engagement verifies a valid engagement request
// builds the expected snCreateCasePayload (title/description/engagementType)
// and maps a successful ServiceNow response back to domain.CreateCaseResponse.
func TestSNCaseService_CreateCase_Engagement(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000001", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	req := domain.CreateCaseRequest{
		Type:                  "engagement",
		ProjectID:             testProjectUUID,
		DeploymentID:          testDeploymentUUID,
		DeployedProductID:     testDeployedProdID,
		Subject:               "Migration planning",
		Description:           "Plan the migration",
		EngagementType:        domain.EngagementTypeMigration,
		EngagementPaymentType: domain.EngagementPaymentTypeFOC,
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.Number != "CS0000001" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}

	if gotBody["title"] != "Migration planning" {
		t.Fatalf("payload title: got %v, want %q", gotBody["title"], "Migration planning")
	}
	if gotBody["description"] != "Plan the migration" {
		t.Fatalf("payload description: got %v, want %q", gotBody["description"], "Plan the migration")
	}
	if gotBody["engagementType"] != float64(1) {
		t.Fatalf("payload engagementType: got %v, want 1", gotBody["engagementType"])
	}
	if gotBody["engagementPaymentType"] != float64(2) {
		t.Fatalf("payload engagementPaymentType: got %v, want 2", gotBody["engagementPaymentType"])
	}
	if gotBody["type"] != "engagement" {
		t.Fatalf("payload type: got %v, want %q", gotBody["type"], "engagement")
	}
}

// TestSNCaseService_CreateCase_DefaultCaseAliasNormalizesToCase verifies that
// "default_case" -- the production customer-portal frontend's actual value
// for this type (ServiceNow's own raw caseType wire value, which predates
// this service's "case" enum; see caseTypeAliases) -- is accepted, runs the
// same validation/payload-building as "case" (subject/description/severity/
// issueType required), and is sent to ServiceNow as "default_case" on the
// wire, exactly as "case" would be.
func TestSNCaseService_CreateCase_DefaultCaseAliasNormalizesToCase(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000002", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	req := domain.CreateCaseRequest{
		Type:              "default_case",
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
		t.Fatalf("unexpected error for the default_case alias: %v", err)
	}
	if resp.Case.Number != "CS0000002" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}
	if gotBody["type"] != "default_case" {
		t.Fatalf("payload type: got %v, want %q (SN wire value)", gotBody["type"], "default_case")
	}
	if gotBody["title"] != "Cannot log in" {
		t.Fatalf("payload title: got %v, want %q", gotBody["title"], "Cannot log in")
	}
}

// TestSNCaseService_CreateCase_SecurityReportAnalysis_AttachmentsOptional verifies
// that a security_report_analysis request with zero attachments is accepted --
// attachments are uploaded via a separate request after the case is created, not
// bundled into this one, so they must not be required here.
func TestSNCaseService_CreateCase_SecurityReportAnalysis_AttachmentsOptional(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000002", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	req := domain.CreateCaseRequest{
		Type:              "security_report_analysis",
		ProjectID:         testProjectUUID,
		DeploymentID:      testDeploymentUUID,
		DeployedProductID: testDeployedProdID,
		Subject:           "Suspicious log entries",
		Description:       "Found several suspicious entries in the access log",
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error with zero attachments: %v", err)
	}
	if resp.Case.Number != "CS0000002" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}
	if _, present := gotBody["attachments"]; present {
		t.Fatalf("expected no attachments field in payload when none provided, got %v", gotBody["attachments"])
	}
}

// TestSNCaseService_CreateCase_AnnouncementValidation verifies that an
// announcement CreateCaseRequest requires only subject and description --
// announcements have no deployment/deployed-product concept, so those fields
// must not be required for this type.
func TestSNCaseService_CreateCase_AnnouncementValidation(t *testing.T) {
	baseReq := domain.CreateCaseRequest{
		Type:      "announcement",
		ProjectID: testProjectUUID,
	}

	tests := []struct {
		name string
		req  domain.CreateCaseRequest
	}{
		{name: "missing subject", req: baseReq},
		{name: "missing description", req: func() domain.CreateCaseRequest { r := baseReq; r.Subject = "Planned maintenance"; return r }()},
	}

	// client is intentionally nil: every case must fail validation before touching it.
	svc := NewServiceNowCaseService(nil, nil, nil, noopSLAClockService{}, nil, "", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCase(contextWithUserIDToken("token"), tt.req)
			if _, ok := err.(*apierror.ValidationError); !ok {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
		})
	}
}

// TestSNCaseService_CreateCase_Announcement verifies a valid announcement
// request builds the expected snCreateCasePayload (title/description only,
// no deploymentId/deployedProductId) and maps a successful ServiceNow
// response back to domain.CreateCaseResponse.
func TestSNCaseService_CreateCase_Announcement(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"message": "Case created successfully",
			"case": {"id": "` + testWLCaseSysid + `", "number": "CS0000003", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}
		}`))
	})

	svc := NewServiceNowCaseService(client, nil, nil, noopSLAClockService{}, nil, "", nil)
	req := domain.CreateCaseRequest{
		Type:        "announcement",
		ProjectID:   testProjectUUID,
		Subject:     "Planned maintenance",
		Description: "Maintenance window this weekend",
	}

	resp, err := svc.CreateCase(contextWithUserIDToken("token"), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Case.Number != "CS0000003" {
		t.Fatalf("unexpected case number: %s", resp.Case.Number)
	}

	if gotBody["title"] != "Planned maintenance" {
		t.Fatalf("payload title: got %v, want %q", gotBody["title"], "Planned maintenance")
	}
	if gotBody["description"] != "Maintenance window this weekend" {
		t.Fatalf("payload description: got %v, want %q", gotBody["description"], "Maintenance window this weekend")
	}
	if gotBody["type"] != "announcement" {
		t.Fatalf("payload type: got %v, want %q", gotBody["type"], "announcement")
	}
	// deploymentId/deployedProductId are not omitempty on the payload struct,
	// so they still serialize -- but as empty strings, since announcements
	// have no deployment/deployed-product concept and req.DeploymentID/
	// req.DeployedProductID are never populated for this type.
	if v, present := gotBody["deploymentId"]; present && v != "" {
		t.Fatalf("expected empty deploymentId in payload for announcement, got %v", v)
	}
	if v, present := gotBody["deployedProductId"]; present && v != "" {
		t.Fatalf("expected empty deployedProductId in payload for announcement, got %v", v)
	}
}
