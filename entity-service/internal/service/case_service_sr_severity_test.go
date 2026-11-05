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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

var (
	srsInternalScope = AccessScope{Unrestricted: true, HasInternalAccess: true}
	srsExternalScope = AccessScope{ProjectIDs: []string{testProjectUUID}, ViewerEmail: "jane.doe@example.com"}
	srsPortalScope   = AccessScope{Unrestricted: true, HasInternalAccess: true, ViaCustomerPortal: true}
)

func srUnitCreateRequest(sev domain.CaseSeverity) domain.CreateCaseRequest {
	return domain.CreateCaseRequest{
		Type: "service_request", CreatedBy: "00000000-0000-0000-0000-000000000001", ProjectID: testProjectUUID,
		DeploymentID: testDeploymentUUID, DeployedProductID: testDeployedProdID,
		Subject: "s", Description: "d", Severity: sev,
		CatalogID: testDeploymentUUID, CatalogItemID: testDeploymentUUID,
		Variables: []domain.Variable{{ID: testDeploymentUUID, Value: "v"}},
	}
}

// severityReachingRepo creates a service request through caseService with a
// stub repository and returns the severity the repository was asked to store.
func severityReachingRepo(t *testing.T, scope AccessScope, defaultLabel string, in domain.CaseSeverity) (domain.CaseSeverity, error) {
	t.Helper()
	var got domain.CaseSeverity
	repo := &stubCaseRepo{createCase: func(_ context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
		got = req.Severity
		return domain.Case{ID: "id"}, nil
	}}
	svc := WithServiceRequestDefaultSeverity(NewCaseService(repo, stubUserRepo{}, nil, stubAccessServiceWithScope{scope}, nil), defaultLabel)
	_, err := svc.CreateCase(context.Background(), srUnitCreateRequest(in))
	return got, err
}

func TestCaseService_CreateServiceRequest_Severity(t *testing.T) {
	tests := []struct {
		name  string
		scope AccessScope
		label string
		in    domain.CaseSeverity
		want  domain.CaseSeverity
	}{
		{"internal explicit", srsInternalScope, "", domain.CaseSeverityHigh, domain.CaseSeverityHigh},
		{"internal none, no config: S4", srsInternalScope, "", "", domain.CaseSeverityLow},
		{"internal none, configured S1", srsInternalScope, "S1", "", domain.CaseSeverityCritical},
		{"internal explicit beats the config", srsInternalScope, "S1", domain.CaseSeverityMedium, domain.CaseSeverityMedium},
		{"external explicit is dropped for the default", srsExternalScope, "S2", domain.CaseSeverityCritical, domain.CaseSeverityHigh},
		{"external catastrophic is dropped, not refused", srsExternalScope, "", domain.CaseSeverityCatastrophic, domain.CaseSeverityLow},
		{"staff through the customer portal is external for this", srsPortalScope, "", domain.CaseSeverityCritical, domain.CaseSeverityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := severityReachingRepo(t, tt.scope, tt.label, tt.in)
			if err != nil {
				t.Fatalf("CreateCase: %v", err)
			}
			if got != tt.want {
				t.Errorf("severity stored = %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("internal S0 is refused before the repository is reached", func(t *testing.T) {
		_, err := severityReachingRepo(t, srsInternalScope, "", domain.CaseSeverityCatastrophic)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v, want a ValidationError", err)
		}
	})
	t.Run("a case keeps its own required severity rules", func(t *testing.T) {
		req := srUnitCreateRequest(domain.CaseSeverityCatastrophic)
		req.Type, req.IssueType = "case", domain.CaseIssueTypeError
		var got domain.CaseSeverity
		repo := &stubCaseRepo{createCase: func(_ context.Context, r domain.CreateCaseRequest) (domain.Case, error) {
			got = r.Severity
			return domain.Case{ID: "id"}, nil
		}}
		svc := NewCaseService(repo, stubUserRepo{}, nil, stubAccessServiceWithScope{srsInternalScope}, nil)
		if _, err := svc.CreateCase(context.Background(), req); err != nil || got != domain.CaseSeverityCatastrophic {
			t.Fatalf("case create: severity = %q, err = %v; a case may be catastrophic", got, err)
		}
	})
}

func TestWithServiceRequestDefaultSeverity_UnknownLabelFallsBackToS4(t *testing.T) {
	got, err := severityReachingRepo(t, srsInternalScope, "S9", "")
	if err != nil || got != domain.CaseSeverityLow {
		t.Fatalf("severity = %q, err = %v, want low", got, err)
	}
}

// srUpdateFixture is a caseService over a stub repository holding one service
// request, with every side effect an update of a case's severity would have
// (SLA revision, notification, mirror write) wired to fail the test.
type srUpdateFixture struct {
	svc       CaseService
	repoCalls int
	sla       *fakeSLAEngineService
	publisher *mockEventPublisher
	mirrored  chan struct{}
}

func newSRUpdateFixture(t *testing.T, scope AccessScope) *srUpdateFixture {
	t.Helper()
	f := &srUpdateFixture{sla: &fakeSLAEngineService{}, publisher: &mockEventPublisher{}, mirrored: make(chan struct{}, 4)}
	srType := "service_request"
	prev := domain.CaseSeverityHigh
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ID: testDeploymentUUID, Type: &srType, ProjectDetails: &domain.EntityRef{ID: testProjectUUID}}, nil
		},
		updateCasePrev: func(_ context.Context, req domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, *domain.CaseState, error) {
			f.repoCalls++
			return domain.Case{ID: req.ID, Severity: req.Severity}, &prev, nil, nil
		},
	}
	mirror := &stubMirrorCaseService{patchCaseFieldsFn: func(context.Context, string, *domain.CaseState, *domain.CaseSeverity, *domain.CaseWorkState, *bool, *caseResolutionFields, bool) (domain.UpdatedCase, error) {
		f.mirrored <- struct{}{}
		return domain.UpdatedCase{}, nil
	}}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	f.svc = NewCaseServiceWithSNWriteback(repo, stubUserRepo{}, f.publisher, stubAccessServiceWithScope{scope}, nil, dispatcher, mirror, f.sla, "")
	return f
}

func TestCaseService_UpdateServiceRequestSeverity(t *testing.T) {
	low := domain.CaseSeverityLow
	t.Run("internal: stored, and no SLA revision, no event, no mirror write", func(t *testing.T) {
		f := newSRUpdateFixture(t, srsInternalScope)
		resp, err := f.svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &low})
		if err != nil {
			t.Fatalf("UpdateCase: %v", err)
		}
		if f.repoCalls != 1 || resp.Case.Severity == nil || *resp.Case.Severity != low {
			t.Errorf("repo calls = %d, response severity = %v, want 1 and low", f.repoCalls, resp.Case.Severity)
		}
		if len(f.sla.reviseCalls) != 0 || len(f.sla.stateCalls) != 0 {
			t.Errorf("SLA engine was touched: revise %v state %v", f.sla.reviseCalls, f.sla.stateCalls)
		}
		if len(f.publisher.calls) != 0 {
			t.Errorf("an event was published: %d", len(f.publisher.calls))
		}
		select {
		case <-f.mirrored:
			t.Error("the severity was mirrored to the backing data source")
		case <-time.After(300 * time.Millisecond):
		}
	})
	for name, scope := range map[string]AccessScope{"external": srsExternalScope, "staff via the customer portal": srsPortalScope} {
		t.Run(name+": 403, nothing written", func(t *testing.T) {
			f := newSRUpdateFixture(t, scope)
			_, err := f.svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &low})
			var fe *apierror.ForbiddenError
			if !errors.As(err, &fe) || f.repoCalls != 0 {
				t.Fatalf("err = %v, repo calls = %d, want a ForbiddenError and 0", err, f.repoCalls)
			}
		})
	}
	t.Run("S0 is a validation error", func(t *testing.T) {
		f := newSRUpdateFixture(t, srsInternalScope)
		cata := domain.CaseSeverityCatastrophic
		_, err := f.svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &cata})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || f.repoCalls != 0 {
			t.Fatalf("err = %v, repo calls = %d, want a ValidationError and 0", err, f.repoCalls)
		}
	})
}

func TestSNCaseService_CreateCase_ServiceRequestPayloadCarriesNoSeverity(t *testing.T) {
	var gotBody map[string]any
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message": "ok", "case": {"id": "` + testWLCaseSysid + `", "number": "SR0000001", "createdBy": "engineer@example.com", "createdOn": "2026-01-02 10:00:00", "state": {"id": 1, "label": "Open"}}}`))
	})
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)
	// A severity set by caseService before the create (explicit or default).
	if _, err := svc.CreateCase(contextWithUserIDToken("token"), srUnitCreateRequest(domain.CaseSeverityCritical)); err != nil {
		t.Fatalf("CreateCase: %v", err)
	}
	for k := range gotBody {
		if k == "severity" || k == "severityKey" {
			t.Errorf("the create payload carries %q: a service request's severity must not be sent to the backing data source (payload %v)", k, gotBody)
		}
	}
	if gotBody["type"] == nil {
		t.Errorf("payload %v does not look like a create payload", gotBody)
	}
}

// A failed read of the record's type fails the severity update: the general
// path must never run for a record whose type could not be established.
func TestCaseService_UpdateSeverity_FailsClosedWhenTheTypeReadFails(t *testing.T) {
	readErr := errors.New("connection reset")
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, readErr
		},
		updateCasePrev: func(context.Context, domain.UpdateCaseRequest) (domain.Case, *domain.CaseSeverity, *domain.CaseState, error) {
			t.Error("the repository was asked to write after the type read failed")
			return domain.Case{}, nil, nil, nil
		},
	}
	low := domain.CaseSeverityLow
	for name, scope := range map[string]AccessScope{"internal": srsInternalScope, "external": srsExternalScope, "portal staff": srsPortalScope} {
		svc := NewCaseService(repo, stubUserRepo{}, nil, stubAccessServiceWithScope{scope}, nil)
		_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, Severity: &low})
		if !errors.Is(err, readErr) {
			t.Errorf("%s: err = %v, want the type read error returned", name, err)
		}
	}
}

func TestSNCaseService_UpdateCase_ServiceRequestSeverityIsRefused(t *testing.T) {
	var patched bool
	client := newTestCaseClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			patched = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "` + testWLCaseSysid + `", "internalId": "WSO2-001", "number": "SR0001001",
			"title": "t", "description": "d", "createdOn": "2026-01-01 10:00:00", "updatedOn": "2026-01-02 10:00:00",
			"createdBy": "reporter@example.com",
			"project": {"id": "` + testProjectSysid + `", "name": "Project A"},
			"deployment": {"id": "", "name": ""}, "deployedProduct": {"id": "", "name": "", "version": ""},
			"state": {"id": 1, "label": "Open"},
			"caseType": {"id": "5aeff1201b74c210264c997a234bcb54", "name": "Service Request"}
		}`))
	})
	svc := NewServiceNowCaseService(client, nil, nil, nil, nil, "", nil)
	sev := domain.CaseSeverityHigh
	_, err := svc.UpdateCase(contextWithUserIDToken("token"), domain.UpdateCaseRequest{ID: sysidToUUID(testWLCaseSysid), Severity: &sev})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a ValidationError (400)", err)
	}
	if patched {
		t.Error("a write was sent to the backing data source")
	}
}
