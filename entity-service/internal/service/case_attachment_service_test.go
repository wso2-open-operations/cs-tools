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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const testCaseID = "00000000-0000-0000-0000-0000000000c1"
const testAttachmentID = "00000000-0000-0000-0000-0000000000a1"
const testStorageKey = "cases/00000000-0000-0000-0000-0000000000c1/00000000-0000-0000-0000-0000000000a1"

func actorUserRepo(t *testing.T) stubUserRepo {
	t.Helper()
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			if email != "jane.doe@example.com" {
				t.Fatalf("unexpected email looked up: %s", email)
			}
			return domain.User{ID: "user-jane", Email: "jane.doe@example.com", FirstName: "Jane", LastName: "Doe"}, nil
		},
	}
}

// TestCaseService_SearchCaseAttachments_ReturnsStorageKey proves the search
// path surfaces storageKey for every row, not just create.
func TestCaseService_SearchCaseAttachments_ReturnsStorageKey(t *testing.T) {
	key := testStorageKey
	repo := &stubCaseRepo{
		searchCaseAttachments: func(_ context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
			if caseID != testCaseID {
				t.Fatalf("expected caseID %q, got %q", testCaseID, caseID)
			}
			return []domain.Attachment{{
				ID:            testAttachmentID,
				ReferenceID:   caseID,
				ReferenceType: domain.ReferenceTypeCase,
				Name:          "diagnostics.log",
				Type:          "text/plain",
				SizeBytes:     2048,
				CreatedBy:     domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
				CreatedOn:     time.Now(),
				StorageKey:    &key,
			}}, 1, nil
		},
	}

	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
	})
	if err != nil {
		t.Fatalf("SearchCaseAttachments returned error: %v", err)
	}
	if len(resp.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(resp.Attachments))
	}
	if resp.Attachments[0].StorageKey == nil || *resp.Attachments[0].StorageKey != testStorageKey {
		t.Fatalf("expected storageKey %q, got %v", testStorageKey, resp.Attachments[0].StorageKey)
	}
}

func errorsAsUnauthorized(err error, target **apierror.UnauthorizedError) bool {
	if ue, ok := err.(*apierror.UnauthorizedError); ok {
		*target = ue
		return true
	}
	return false
}

func errorsAsNotFound(err error, target **apierror.NotFoundError) bool {
	if nfe, ok := err.(*apierror.NotFoundError); ok {
		*target = nfe
		return true
	}
	return false
}

func errorsAsServiceUnavailable(err error, target **apierror.ServiceUnavailableError) bool {
	if sue, ok := err.(*apierror.ServiceUnavailableError); ok {
		*target = sue
		return true
	}
	return false
}

func errorsAsConflict(err error, target **apierror.ConflictError) bool {
	if ce, ok := err.(*apierror.ConflictError); ok {
		*target = ce
		return true
	}
	return false
}

func errorsAsForbidden(err error, target **apierror.ForbiddenError) bool {
	if fe, ok := err.(*apierror.ForbiddenError); ok {
		*target = fe
		return true
	}
	return false
}

const testWorkItemID = "00000000-0000-0000-0000-0000000000d1"

// TestCaseService_SearchCaseAttachments_WorkItemTypes proves change_request,
// incident and conversation searches go through SearchWorkItemAttachments
// (never the case path), echo the reference type, and fill pagination the
// same way the case path does.
func TestCaseService_SearchCaseAttachments_WorkItemTypes(t *testing.T) {
	for _, rt := range []domain.ReferenceType{
		domain.ReferenceTypeChangeRequest,
		domain.ReferenceTypeIncident,
		domain.ReferenceTypeConversation,
	} {
		t.Run(string(rt), func(t *testing.T) {
			repo := &stubCaseRepo{
				searchWorkItemAttachments: func(_ context.Context, id string, gotType domain.ReferenceType, p domain.Pagination) ([]domain.Attachment, int, error) {
					if id != testWorkItemID || gotType != rt {
						t.Fatalf("got id=%q type=%q, want %q/%q", id, gotType, testWorkItemID, rt)
					}
					if p.Limit != 1 || p.Offset != 0 {
						t.Fatalf("unexpected pagination %+v", p)
					}
					return []domain.Attachment{{
						ID:            testAttachmentID,
						ReferenceID:   id,
						ReferenceType: gotType,
						Name:          "plan.pdf",
						Type:          "application/pdf",
						SizeBytes:     10,
						CreatedBy:     domain.NewUserReference("", "jane.doe@example.com", "Jane Doe"),
						CreatedOn:     time.Now(),
						Status:        domain.AttachmentStatusComplete,
					}}, 3, nil
				},
			}
			svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
			resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
				ReferenceID:   testWorkItemID,
				ReferenceType: rt,
				Pagination:    domain.Pagination{Limit: 1, Offset: 0},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Attachments) != 1 || resp.Attachments[0].ReferenceType != rt {
				t.Fatalf("unexpected attachments: %+v", resp.Attachments)
			}
			if resp.Total != 3 || resp.Limit != 1 || resp.Offset != 0 || !resp.HasMore {
				t.Fatalf("pagination = total %d limit %d offset %d hasMore %v, want 3/1/0/true", resp.Total, resp.Limit, resp.Offset, resp.HasMore)
			}
		})
	}
}

// TestCaseService_SearchCaseAttachments_DeploymentReadsDeploymentAttachments
// proves a deployment search is listed from Postgres (deployment_attachment),
// never from the work item path and never from the mirrored data source.
func TestCaseService_SearchCaseAttachments_DeploymentReadsDeploymentAttachments(t *testing.T) {
	repo := &stubCaseRepo{
		searchDeploymentAttachments: func(_ context.Context, id string, p domain.Pagination) ([]domain.Attachment, int, error) {
			if id != testWorkItemID || p.Limit != 1 || p.Offset != 0 {
				t.Fatalf("got id=%q pagination=%+v", id, p)
			}
			return []domain.Attachment{{
				ID: testAttachmentID, ReferenceID: id, ReferenceType: domain.ReferenceTypeDeployment,
				Name: "topology.pdf", Type: "application/pdf", SizeBytes: 10, Status: domain.AttachmentStatusComplete,
			}}, 2, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testWorkItemID,
		ReferenceType: domain.ReferenceTypeDeployment,
		Pagination:    domain.Pagination{Limit: 1, Offset: 0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Attachments) != 1 || resp.Attachments[0].ReferenceType != domain.ReferenceTypeDeployment {
		t.Fatalf("unexpected attachments: %+v", resp.Attachments)
	}
	if resp.Total != 2 || !resp.HasMore {
		t.Fatalf("total %d hasMore %v, want 2/true", resp.Total, resp.HasMore)
	}
}

// TestCaseService_SearchCaseAttachments_WorkItemEmptyIsSuccess: no
// attachments is total 0 and no error.
func TestCaseService_SearchCaseAttachments_WorkItemEmptyIsSuccess(t *testing.T) {
	repo := &stubCaseRepo{
		searchWorkItemAttachments: func(context.Context, string, domain.ReferenceType, domain.Pagination) ([]domain.Attachment, int, error) {
			return []domain.Attachment{}, 0, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testWorkItemID,
		ReferenceType: domain.ReferenceTypeIncident,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 0 || len(resp.Attachments) != 0 || resp.HasMore {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// TestCaseService_SearchCaseAttachments_CaseStaysOnCasePath: case must not
// touch the work item method (the stub panics if it does).
func TestCaseService_SearchCaseAttachments_CaseStaysOnCasePath(t *testing.T) {
	called := false
	repo := &stubCaseRepo{
		searchCaseAttachments: func(_ context.Context, id string, _ domain.Pagination) ([]domain.Attachment, int, error) {
			called = true
			return nil, 0, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	if _, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("case search did not use SearchCaseAttachments")
	}
}

// TestCaseService_SearchCaseAttachments_UnsupportedType: unknown types stay a
// validation error, before any repository call.
func TestCaseService_SearchCaseAttachments_UnsupportedType(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	for _, rt := range []domain.ReferenceType{"bogus", ""} {
		_, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
			ReferenceID:   testWorkItemID,
			ReferenceType: rt,
		})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("type %q: want ValidationError, got %v", rt, err)
		}
		if !strings.Contains(ve.Msg, "not supported for this data source") {
			t.Fatalf("type %q: unclear message %q", rt, ve.Msg)
		}
	}
}

// stubAttachmentSearchMirror is a mirror CaseService that only implements
// SearchCaseAttachments; any other method panics via the nil embed.
type stubAttachmentSearchMirror struct {
	CaseService
	search func(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error)
	calls  int
}

func (m *stubAttachmentSearchMirror) SearchCaseAttachments(ctx context.Context, req domain.SearchAttachmentsRequest) (domain.SearchAttachmentsResponse, error) {
	m.calls++
	return m.search(ctx, req)
}

// TestValidateCreateCaseRequest_NeedsNoAttachmentsForAnyType pins that every case
// type is created from its own fields alone. Files are uploaded afterwards,
// through the stored-attachment routes, so a request body has no attachments.
func TestValidateCreateCaseRequest_NeedsNoAttachmentsForAnyType(t *testing.T) {
	const common = `"projectId":"` + testCaseID + `","deploymentId":"` + testCaseID + `","deployedProductId":"` + testCaseID + `"`
	for name, body := range map[string]string{
		"case":                     `{"type":"case",` + common + `,"subject":"s","description":"d","severity":"medium","issueType":"question"}`,
		"service_request":          `{"type":"service_request",` + common + `,"catalogId":"` + testCaseID + `","catalogItemId":"` + testCaseID + `","variables":[{"id":"` + testCaseID + `","value":"v"}]}`,
		"security_report_analysis": `{"type":"security_report_analysis",` + common + `,"subject":"s","description":"d"}`,
		"engagement":               `{"type":"engagement",` + common + `,"subject":"s","description":"d","engagementType":"migration","engagementPaymentType":"paid"}`,
		"announcement":             `{"type":"announcement","projectId":"` + testCaseID + `","subject":"s","description":"d"}`,
	} {
		t.Run(name, func(t *testing.T) {
			var req domain.CreateCaseRequest
			if err := json.Unmarshal([]byte(body), &req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := validateCreateCaseRequest(&req); err != nil {
				t.Fatalf("a %s without attachments must be valid, got %v", name, err)
			}
		})
	}
}

// A request for deleted attachments is honoured only from the CSM portal
// backend; any other caller's flag is ignored, so the repo is never asked.
func TestCaseService_SearchCaseAttachments_DeletedOnlyHonouredOnlyForCSMPortalBackend(t *testing.T) {
	const portalClient = "csm-portal-backend-client"
	tests := []struct {
		name      string
		clientID  string
		configure bool
		include   bool
		want      bool
	}{
		{"portal backend asks", portalClient, true, true, true},
		{"portal backend does not ask", portalClient, true, false, false},
		{"another caller asks", "customer-portal-client", true, true, false},
		{"no client id configured", portalClient, false, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got bool
			repo := &stubCaseRepo{
				searchCaseAttachments: func(ctx context.Context, _ string, _ domain.Pagination) ([]domain.Attachment, int, error) {
					got = repository.DeletedAttachmentsOnlyRequested(ctx)
					return nil, 0, nil
				},
				searchWorkItemAttachments: func(ctx context.Context, _ string, _ domain.ReferenceType, _ domain.Pagination) ([]domain.Attachment, int, error) {
					got = repository.DeletedAttachmentsOnlyRequested(ctx)
					return nil, 0, nil
				},
			}
			var svc CaseService = NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
			if tc.configure {
				svc = WithDeletedAttachmentsForCSMPortal(svc, portalClient)
			}
			ctx := auth.WithIdentity(context.Background(), auth.Identity{ClientID: tc.clientID})
			for _, rt := range []domain.ReferenceType{domain.ReferenceTypeCase, domain.ReferenceTypeIncident} {
				got = false
				if _, err := svc.SearchCaseAttachments(ctx, domain.SearchAttachmentsRequest{
					ReferenceID: testCaseID, ReferenceType: rt, DeletedOnly: tc.include,
				}); err != nil {
					t.Fatalf("%s: %v", rt, err)
				}
				if got != tc.want {
					t.Fatalf("%s: deleted attachments requested = %v, want %v", rt, got, tc.want)
				}
			}
		})
	}
}
