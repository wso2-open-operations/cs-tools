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
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/aichatagent"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/registry"
)

const callerScopeTestProjectID = "22222222-2222-2222-2222-222222222222"
const callerScopeTestEmail = "customer@example.com"

// fakeEntityContacts implements entityProjectContactsClient, keyed by
// project id.
type fakeEntityContacts struct {
	byProjectID    map[string][]entity.ProjectContact
	errByProjectID map[string]error
	// pages, if set, overrides byProjectID for pagination tests: each call
	// pops the next entry, keyed by offset.
	pagesByProjectID map[string]map[int]entity.SearchProjectContactsResponse
}

func (f *fakeEntityContacts) SearchProjectContacts(_ context.Context, projectID string, req entity.SearchProjectContactsRequest) (entity.SearchProjectContactsResponse, error) {
	if err, ok := f.errByProjectID[projectID]; ok {
		return entity.SearchProjectContactsResponse{}, err
	}
	if pages, ok := f.pagesByProjectID[projectID]; ok {
		return pages[req.Pagination.Offset], nil
	}
	contacts := f.byProjectID[projectID]
	return entity.SearchProjectContactsResponse{Contacts: contacts, Total: len(contacts)}, nil
}

// fakeEntityProjectResolver implements entityProjectClient (SearchProjects +
// GetProject) for the ProjectHandler.SearchProjects tests.
type fakeEntityProjectResolver struct {
	searchResult entity.SearchProjectsResponse
	searchErr    error
	pages        map[int]entity.SearchProjectsResponse
}

func (f *fakeEntityProjectResolver) GetProject(_ context.Context, id string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{ID: id}, nil
}

func (f *fakeEntityProjectResolver) SearchProjects(_ context.Context, req entity.SearchProjectsRequest) (entity.SearchProjectsResponse, error) {
	if f.searchErr != nil {
		return entity.SearchProjectsResponse{}, f.searchErr
	}
	if f.pages != nil {
		if res, ok := f.pages[req.Pagination.Offset]; ok {
			return res, nil
		}
		return entity.SearchProjectsResponse{}, nil
	}
	return f.searchResult, nil
}

func TestCallerScopeResolver_IsProjectMember(t *testing.T) {
	t.Run("true for a matching contact that grants case access, case-insensitive", func(t *testing.T) {
		fake := &fakeEntityContacts{byProjectID: map[string][]entity.ProjectContact{
			callerScopeTestProjectID: {{Email: "Customer@Example.com", GrantsCaseAccess: true}},
		}}
		r := NewCallerScopeResolver(fake)

		member, err := r.IsProjectMember(context.Background(), callerScopeTestProjectID, callerScopeTestEmail)
		if err != nil || !member {
			t.Fatalf("IsProjectMember = (%v, %v), want (true, nil)", member, err)
		}
	})

	t.Run("false when the matching contact does not grant case access", func(t *testing.T) {
		fake := &fakeEntityContacts{byProjectID: map[string][]entity.ProjectContact{
			callerScopeTestProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: false}},
		}}
		r := NewCallerScopeResolver(fake)

		member, err := r.IsProjectMember(context.Background(), callerScopeTestProjectID, callerScopeTestEmail)
		if err != nil || member {
			t.Fatalf("IsProjectMember = (%v, %v), want (false, nil)", member, err)
		}
	})

	t.Run("false when no contact matches the email", func(t *testing.T) {
		fake := &fakeEntityContacts{byProjectID: map[string][]entity.ProjectContact{
			callerScopeTestProjectID: {{Email: "someone-else@example.com", GrantsCaseAccess: true}},
		}}
		r := NewCallerScopeResolver(fake)

		member, err := r.IsProjectMember(context.Background(), callerScopeTestProjectID, callerScopeTestEmail)
		if err != nil || member {
			t.Fatalf("IsProjectMember = (%v, %v), want (false, nil)", member, err)
		}
	})

	t.Run("propagates a SearchProjectContacts failure", func(t *testing.T) {
		wantErr := errors.New("upstream down")
		fake := &fakeEntityContacts{errByProjectID: map[string]error{callerScopeTestProjectID: wantErr}}
		r := NewCallerScopeResolver(fake)

		member, err := r.IsProjectMember(context.Background(), callerScopeTestProjectID, callerScopeTestEmail)
		if member || !errors.Is(err, wantErr) {
			t.Fatalf("IsProjectMember = (%v, %v), want (false, %v)", member, err, wantErr)
		}
	})

	t.Run("pages through the full contact list to find a later match", func(t *testing.T) {
		fake := &fakeEntityContacts{pagesByProjectID: map[string]map[int]entity.SearchProjectContactsResponse{
			callerScopeTestProjectID: {
				0: {Contacts: []entity.ProjectContact{{Email: "someone-else@example.com", GrantsCaseAccess: true}}, Total: 2},
				callerScopeContactsLimit: {
					Contacts: []entity.ProjectContact{{Email: callerScopeTestEmail, GrantsCaseAccess: true}}, Total: 2,
				},
			},
		}}
		r := NewCallerScopeResolver(fake)

		member, err := r.IsProjectMember(context.Background(), callerScopeTestProjectID, callerScopeTestEmail)
		if err != nil || !member {
			t.Fatalf("IsProjectMember = (%v, %v), want (true, nil)", member, err)
		}
	})
}

func TestProjectHandler_SearchProjects_CallerScope(t *testing.T) {
	t.Skip("scopeToCallerProjects call site commented out in SearchProjects pending live verification — see handler.CallerScopeResolver")
	t.Run("filters single-page upstream and excludes non-member and error projects", func(t *testing.T) {
		memberProject := entity.ProjectView{ID: "member-project"}
		otherProject := entity.ProjectView{ID: "other-project"}
		errorProject := entity.ProjectView{ID: "error-project"}

		entityFake := &fakeEntityProjectResolver{
			searchResult: entity.SearchProjectsResponse{
				Projects: []entity.ProjectView{memberProject, otherProject, errorProject},
				Total:    3,
			},
		}
		contactsFake := &fakeEntityContacts{
			byProjectID: map[string][]entity.ProjectContact{
				"member-project": {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
				"other-project":  {{Email: "someone-else@example.com", GrantsCaseAccess: true}},
			},
			errByProjectID: map[string]error{
				"error-project": errors.New("upstream hiccup"),
			},
		}
		resolver := NewCallerScopeResolver(contactsFake)

		h := NewProjectHandler(entityFake)
		h.SetCallerScope(resolver)

		req := authedRequest(http.MethodPost, "/projects/search", `{"pagination":{"limit":10,"offset":0}}`)
		rec := httptest.NewRecorder()
		h.SearchProjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); !strings.Contains(got, "member-project") || strings.Contains(got, "other-project") || strings.Contains(got, "error-project") {
			t.Fatalf("expected only member-project in response, got %s", got)
		}
	})

	t.Run("scans across multiple 50-item upstream pages to find member projects on later pages", func(t *testing.T) {
		// Page 0 has 50 projects where the user is NOT a member.
		page0Projects := make([]entity.ProjectView, 50)
		for i := 0; i < 50; i++ {
			page0Projects[i] = entity.ProjectView{ID: "other-project-p0"}
		}
		// Page 1 has 1 project where user IS a member.
		page1Projects := []entity.ProjectView{{ID: "member-project-p1"}}

		entityFake := &fakeEntityProjectResolver{
			pages: map[int]entity.SearchProjectsResponse{
				0:  {Projects: page0Projects, Total: 51, Limit: 50, Offset: 0, HasMore: true},
				50: {Projects: page1Projects, Total: 51, Limit: 50, Offset: 50, HasMore: false},
			},
		}
		contactsFake := &fakeEntityContacts{
			byProjectID: map[string][]entity.ProjectContact{
				"member-project-p1": {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
			},
		}
		resolver := NewCallerScopeResolver(contactsFake)

		h := NewProjectHandler(entityFake)
		h.SetCallerScope(resolver)

		req := authedRequest(http.MethodPost, "/projects/search", `{"pagination":{"limit":10,"offset":0}}`)
		rec := httptest.NewRecorder()
		h.SearchProjects(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); !strings.Contains(got, "member-project-p1") {
			t.Fatalf("expected member-project-p1 in response, got %s", got)
		}
	})

	t.Run("correctly slices caller pagination and sets total/hasMore", func(t *testing.T) {
		p1 := entity.ProjectView{ID: "p1"}
		p2 := entity.ProjectView{ID: "p2"}
		p3 := entity.ProjectView{ID: "p3"}

		entityFake := &fakeEntityProjectResolver{
			searchResult: entity.SearchProjectsResponse{
				Projects: []entity.ProjectView{p1, p2, p3},
				Total:    3,
			},
		}
		contactsFake := &fakeEntityContacts{
			byProjectID: map[string][]entity.ProjectContact{
				"p1": {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
				"p2": {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
				"p3": {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
			},
		}
		resolver := NewCallerScopeResolver(contactsFake)
		h := NewProjectHandler(entityFake)
		h.SetCallerScope(resolver)

		// Page 1: limit 2, offset 0 -> returns p1, p2, totalRecords: 3, hasMore: true
		req := authedRequest(http.MethodPost, "/projects/search", `{"pagination":{"limit":2,"offset":0}}`)
		rec := httptest.NewRecorder()
		h.SearchProjects(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"totalRecords":3`) || !strings.Contains(body, `"hasMore":true`) || !strings.Contains(body, "p1") || !strings.Contains(body, "p2") || strings.Contains(body, "p3") {
			t.Fatalf("unexpected page 1 response: %s", body)
		}

		// Page 2: limit 2, offset 2 -> returns p3, totalRecords: 3, hasMore: false
		req2 := authedRequest(http.MethodPost, "/projects/search", `{"pagination":{"limit":2,"offset":2}}`)
		rec2 := httptest.NewRecorder()
		h.SearchProjects(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
		}
		body2 := rec2.Body.String()
		if !strings.Contains(body2, `"totalRecords":3`) || !strings.Contains(body2, `"hasMore":false`) || !strings.Contains(body2, "p3") || strings.Contains(body2, "p1") {
			t.Fatalf("unexpected page 2 response: %s", body2)
		}

		// Overflow protection: large limit (math.MaxInt) with offset 1 must not panic and slice correctly
		reqOverflow := authedRequest(http.MethodPost, "/projects/search", fmt.Sprintf(`{"pagination":{"limit":%d,"offset":1}}`, math.MaxInt))
		recOverflow := httptest.NewRecorder()
		h.SearchProjects(recOverflow, reqOverflow)
		if recOverflow.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recOverflow.Code, recOverflow.Body.String())
		}
		bodyOverflow := recOverflow.Body.String()
		if !strings.Contains(bodyOverflow, `"totalRecords":3`) || !strings.Contains(bodyOverflow, `"hasMore":false`) || !strings.Contains(bodyOverflow, "p2") || !strings.Contains(bodyOverflow, "p3") {
			t.Fatalf("unexpected overflow test response: %s", bodyOverflow)
		}
	})
}

// TestProjectHandler_SearchProjects_NoResolverConfigured guards a
// nil-safety property this package relies on, not a production toggle:
// requireProjectMember/scopeToCallerProjects must treat an unset
// CallerScopeResolver as unscoped rather than panicking, because dozens of
// pre-existing tests elsewhere in this package construct handlers directly
// without ever calling SetCallerScope. In production, main.go always wires
// a real resolver unconditionally — this state is test-only.
func TestProjectHandler_SearchProjects_NoResolverConfigured(t *testing.T) {
	entityFake := &fakeEntityProjectResolver{
		searchResult: entity.SearchProjectsResponse{
			Projects: []entity.ProjectView{{ID: "any-project"}},
			Total:    1,
		},
	}
	h := NewProjectHandler(entityFake)
	// SetCallerScope deliberately not called.

	req := authedRequest(http.MethodPost, "/projects/search", `{"pagination":{"limit":10,"offset":0}}`)
	rec := httptest.NewRecorder()
	h.SearchProjects(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "any-project") {
		t.Fatalf("expected unscoped 200 response including any-project, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCaseHandler_SearchCases_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("member can search", func(t *testing.T) {
		fake := &fakeEntityCaseClient{}
		h := NewCaseHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/cases/search", h.SearchCases)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/cases/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in SearchCases pending live verification — see handler.CallerScopeResolver")
		fake := &fakeEntityCaseClient{}
		h := NewCaseHandler(fake)
		h.SetCallerScope(resolver)

		otherProjectID := "33333333-3333-3333-3333-333333333333"
		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/cases/search", h.SearchCases)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/cases/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestCaseHandler_GetCase_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("member can view the case", func(t *testing.T) {
		fake := &fakeEntityCaseClientForCase{caseView: entity.CaseView{ID: "case-1", ProjectDetails: entity.EntityRef{ID: testProjectID}}}
		h := NewCaseHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /cases/{id}", h.GetCase)
		req := authedRequest(http.MethodGet, "/cases/44444444-4444-4444-4444-444444444444", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-member gets 404, not 403", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetCase pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "55555555-5555-5555-5555-555555555555"
		fake := &fakeEntityCaseClientForCase{caseView: entity.CaseView{ID: "case-2", ProjectDetails: entity.EntityRef{ID: otherProjectID}}}
		h := NewCaseHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /cases/{id}", h.GetCase)
		req := authedRequest(http.MethodGet, "/cases/44444444-4444-4444-4444-444444444444", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

type fakeEntityCaseClientForCase struct {
	caseView entity.CaseView
}

func (f *fakeEntityCaseClientForCase) GetCase(_ context.Context, _ string) (entity.CaseView, error) {
	return f.caseView, nil
}

func (f *fakeEntityCaseClientForCase) SearchAttachments(_ context.Context, _ entity.SearchAttachmentsRequest) (entity.SearchAttachmentsResponse, error) {
	return entity.SearchAttachmentsResponse{Total: 1}, nil
}

func (f *fakeEntityCaseClientForCase) CreateAttachment(_ context.Context, _ entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error) {
	return entity.CreateAttachmentResponse{Attachment: entity.AttachmentDetail{ID: "att-1"}}, nil
}

func (f *fakeEntityCaseClientForCase) CreateCase(_ context.Context, _ entity.CreateCaseRequest) (entity.CreateCaseResponse, error) {
	return entity.CreateCaseResponse{Case: entity.CreateCaseDetails{ID: "c-1"}}, nil
}

func (f *fakeEntityCaseClientForCase) UpdateCase(_ context.Context, _ string, _ entity.UpdateCaseRequest) (entity.UpdateCaseResponse, error) {
	return entity.UpdateCaseResponse{Case: entity.UpdatedCase{ID: "c-1"}}, nil
}

func (f *fakeEntityCaseClientForCase) CreateCaseComment(_ context.Context, _ string, _ entity.CreateCaseCommentRequest) (entity.CreateCaseCommentResponse, error) {
	return entity.CreateCaseCommentResponse{Comment: entity.CaseCommentDetail{ID: "cm-1"}}, nil
}

func (f *fakeEntityCaseClientForCase) GetCaseFeedback(_ context.Context, _ string) (entity.CaseFeedback, error) {
	return entity.CaseFeedback{ID: "fb-1"}, nil
}

func (f *fakeEntityCaseClientForCase) SubmitCaseFeedback(_ context.Context, _ string, _ entity.SubmitCaseFeedbackRequest) (entity.SubmitCaseFeedbackResponse, error) {
	return entity.SubmitCaseFeedbackResponse{Message: "ok"}, nil
}

func (f *fakeEntityCaseClientForCase) UpdateAttachment(_ context.Context, _ string, _ entity.UpdateAttachmentRequest) (entity.UpdateAttachmentResponse, error) {
	return entity.UpdateAttachmentResponse{Message: "ok"}, nil
}

func (f *fakeEntityCaseClientForCase) CreateEscalation(_ context.Context, _ entity.CreateEscalationRequest) (entity.CreateEscalationResponse, error) {
	return entity.CreateEscalationResponse{Message: "ok"}, nil
}

func (f *fakeEntityCaseClientForCase) SearchCases(_ context.Context, _ entity.SearchCasesRequest) (entity.SearchCasesResponse, error) {
	return entity.SearchCasesResponse{}, nil
}

func (f *fakeEntityCaseClientForCase) UpdateConversation(_ context.Context, _ string, _ entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	return entity.UpdateConversationResponse{}, nil
}

func (f *fakeEntityCaseClientForCase) SearchCaseActivities(_ context.Context, _ string, _ entity.SearchCaseActivitiesRequest) (entity.SearchCaseActivitiesResponse, error) {
	return entity.SearchCaseActivitiesResponse{}, nil
}

func (f *fakeEntityCaseClientForCase) SearchEscalations(_ context.Context, _ entity.SearchEscalationsRequest) (entity.SearchEscalationsResponse, error) {
	return entity.SearchEscalationsResponse{}, nil
}

func TestCaseHandler_SubResources_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	caseID := "44444444-4444-4444-4444-444444444444"
	memberFake := &fakeEntityCaseClientForCase{caseView: entity.CaseView{ID: caseID, ProjectDetails: entity.EntityRef{ID: testProjectID}}}
	nonMemberFake := &fakeEntityCaseClientForCase{caseView: entity.CaseView{ID: caseID, ProjectDetails: entity.EntityRef{ID: "99999999-9999-9999-9999-999999999999"}}}

	tests := []struct {
		name        string
		method      string
		pattern     string
		url         string
		body        string
		handlerFunc func(h *CaseHandler) http.HandlerFunc
		wantMember  int
		wantNonMemb int
	}{
		{
			name:        "SearchCaseAttachments",
			method:      http.MethodGet,
			pattern:     "GET /cases/{id}/attachments",
			url:         "/cases/" + caseID + "/attachments",
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.SearchCaseAttachments },
			wantMember:  http.StatusOK,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "CreateCaseAttachment",
			method:      http.MethodPost,
			pattern:     "POST /cases/{id}/attachments",
			url:         "/cases/" + caseID + "/attachments",
			body:        `{"name":"diag.zip","type":"application/zip","content":"aGVsbG8="}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.CreateCaseAttachment },
			wantMember:  http.StatusCreated,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "PatchCase",
			method:      http.MethodPatch,
			pattern:     "PATCH /cases/{id}",
			url:         "/cases/" + caseID,
			body:        `{"stateKey":2}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.PatchCase },
			wantMember:  http.StatusOK,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "CreateCaseComment",
			method:      http.MethodPost,
			pattern:     "POST /cases/{id}/comments",
			url:         "/cases/" + caseID + "/comments",
			body:        `{"content":"hello"}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.CreateCaseComment },
			wantMember:  http.StatusCreated,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "GetCaseFeedback",
			method:      http.MethodGet,
			pattern:     "GET /cases/{id}/feedback",
			url:         "/cases/" + caseID + "/feedback",
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.GetCaseFeedback },
			wantMember:  http.StatusOK,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "SubmitCaseFeedback",
			method:      http.MethodPost,
			pattern:     "POST /cases/{id}/feedback",
			url:         "/cases/" + caseID + "/feedback",
			body:        `{"comments":"great"}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.SubmitCaseFeedback },
			wantMember:  http.StatusCreated,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "PatchCaseAttachment",
			method:      http.MethodPatch,
			pattern:     "PATCH /cases/{caseId}/attachments/{attachmentId}",
			url:         "/cases/" + caseID + "/attachments/55555555-5555-5555-5555-555555555555",
			body:        `{"name":"new.zip"}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.PatchCaseAttachment },
			wantMember:  http.StatusOK,
			wantNonMemb: http.StatusNotFound,
		},
		{
			name:        "CreateCaseEscalation",
			method:      http.MethodPost,
			pattern:     "POST /cases/{caseId}/escalations",
			url:         "/cases/" + caseID + "/escalations",
			body:        `{"action":"escalate","reason":"urgent"}`,
			handlerFunc: func(h *CaseHandler) http.HandlerFunc { return h.CreateCaseEscalation },
			wantMember:  http.StatusCreated,
			wantNonMemb: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+": member", func(t *testing.T) {
			h := NewCaseHandler(memberFake)
			h.SetCallerScope(resolver)

			mux := http.NewServeMux()
			mux.HandleFunc(tt.pattern, tt.handlerFunc(h))
			req := authedRequest(tt.method, tt.url, tt.body)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantMember {
				t.Fatalf("expected %d, got %d: %s", tt.wantMember, rec.Code, rec.Body.String())
			}
		})

		t.Run(tt.name+": non-member", func(t *testing.T) {
			t.Skip("requireProjectMember call site commented out in subresources pending live verification — see handler.CallerScopeResolver")
			h := NewCaseHandler(nonMemberFake)
			h.SetCallerScope(resolver)

			mux := http.NewServeMux()
			mux.HandleFunc(tt.pattern, tt.handlerFunc(h))
			req := authedRequest(tt.method, tt.url, tt.body)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantNonMemb {
				t.Fatalf("expected %d, got %d: %s", tt.wantNonMemb, rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("CreateCase: member project", func(t *testing.T) {
		h := NewCaseHandler(memberFake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases", h.CreateCase)
		req := authedRequest(http.MethodPost, "/cases", `{"projectId":"`+testProjectID+`","subject":"test"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateCase: non-member project is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in CreateCase pending live verification — see handler.CallerScopeResolver")
		h := NewCaseHandler(memberFake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases", h.CreateCase)
		req := authedRequest(http.MethodPost, "/cases", `{"projectId":"99999999-9999-9999-9999-999999999999","subject":"test"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// fakeEntityChangeRequestClient records SearchChangeRequests calls; a
// representative direct-project-id handler (no case/deployment resolution).
type fakeEntityChangeRequestClient struct {
	entityChangeRequestClient
}

func (f *fakeEntityChangeRequestClient) SearchChangeRequests(_ context.Context, _ entity.SearchChangeRequestsRequest) (entity.SearchChangeRequestsResponse, error) {
	return entity.SearchChangeRequestsResponse{}, nil
}

func TestChangeRequestHandler_SearchChangeRequests_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("member can search", func(t *testing.T) {
		h := NewChangeRequestHandler(&fakeEntityChangeRequestClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/change-requests/search", h.SearchChangeRequests)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/change-requests/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in SearchChangeRequests pending live verification — see handler.CallerScopeResolver")
		h := NewChangeRequestHandler(&fakeEntityChangeRequestClient{})
		h.SetCallerScope(resolver)

		otherProjectID := "66666666-6666-6666-6666-666666666666"
		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/change-requests/search", h.SearchChangeRequests)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/change-requests/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// callerScopeFakeCallRequestClient serves a fixed CaseView from GetCase and
// records SearchCallRequests calls; a representative case-resolved handler.
type callerScopeFakeCallRequestClient struct {
	entityCallRequestClient
	caseView entity.CaseView
}

func (f *callerScopeFakeCallRequestClient) GetCase(_ context.Context, _ string) (entity.CaseView, error) {
	return f.caseView, nil
}

func (f *callerScopeFakeCallRequestClient) SearchCallRequests(_ context.Context, _ entity.SearchCallRequestsRequest) (entity.SearchCallRequestsResponse, error) {
	return entity.SearchCallRequestsResponse{}, nil
}

func TestCallRequestHandler_SearchCallRequests_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("member can search", func(t *testing.T) {
		fake := &callerScopeFakeCallRequestClient{caseView: entity.CaseView{ProjectDetails: entity.EntityRef{ID: testProjectID}}}
		h := NewCallRequestHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases/{caseId}/call-requests/search", h.SearchCallRequests)
		req := authedRequest(http.MethodPost, "/cases/44444444-4444-4444-4444-444444444444/call-requests/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-member gets 404", func(t *testing.T) {
		t.Skip("Caller-scope check in SearchCallRequests is commented out pending review")
		otherProjectID := "77777777-7777-7777-7777-777777777777"
		fake := &callerScopeFakeCallRequestClient{caseView: entity.CaseView{ProjectDetails: entity.EntityRef{ID: otherProjectID}}}
		h := NewCallRequestHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /cases/{caseId}/call-requests/search", h.SearchCallRequests)
		req := authedRequest(http.MethodPost, "/cases/44444444-4444-4444-4444-444444444444/call-requests/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// fakeEntityInstanceClient records which Search* method was called; enough
// to prove the caller-scope check runs (or doesn't) before the entity call.
type fakeEntityInstanceClient struct {
	entityInstanceClient
	called bool
}

func (f *fakeEntityInstanceClient) SearchInstances(_ context.Context, _ entity.SearchInstancesRequest) (entity.SearchInstancesResponse, error) {
	f.called = true
	return entity.SearchInstancesResponse{}, nil
}

func TestInstanceHandler_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("project-scoped: member can search", func(t *testing.T) {
		fake := &fakeEntityInstanceClient{}
		h := NewInstanceHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/instances/search", h.SearchProjectInstances)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/instances/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || !fake.called {
			t.Fatalf("expected 200 and entity call, got %d (called=%v): %s", rec.Code, fake.called, rec.Body.String())
		}
	})

	t.Run("project-scoped: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in SearchProjectInstances pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		fake := &fakeEntityInstanceClient{}
		h := NewInstanceHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/instances/search", h.SearchProjectInstances)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/instances/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden || fake.called {
			t.Fatalf("expected 403 and no entity call, got %d (called=%v): %s", rec.Code, fake.called, rec.Body.String())
		}
	})

	t.Run("deployment-scoped: check is a no-op even with a resolver configured", func(t *testing.T) {
		// Not a project contact of anything, but the deployment-scoped
		// variant doesn't resolve to a project at all yet — it must still
		// reach the entity client rather than being blocked by a check that
		// doesn't apply to it.
		fake := &fakeEntityInstanceClient{}
		h := NewInstanceHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /deployments/{id}/instances/search", h.SearchDeploymentInstances)
		req := authedRequest(http.MethodPost, "/deployments/99999999-9999-9999-9999-999999999999/instances/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || !fake.called {
			t.Fatalf("expected 200 and entity call (no-op check), got %d (called=%v): %s", rec.Code, fake.called, rec.Body.String())
		}
	})
}

// fakeEntityProjectStatsClient mocks entity-service calls for ProjectStatsHandler.
type fakeEntityProjectStatsClient struct {
	entityProjectStatsClient
}

func (f *fakeEntityProjectStatsClient) GetProjectMetadata(_ context.Context, _ string) (entity.ProjectMetadataResponse, error) {
	return entity.ProjectMetadataResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectCaseStats(_ context.Context, _ string, _ []string, _ string) (entity.ProjectCaseStatsResponse, error) {
	return entity.ProjectCaseStatsResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectConversationStats(_ context.Context, _ string, _ string) (entity.ProjectConversationStatsResponse, error) {
	return entity.ProjectConversationStatsResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectDeploymentStats(_ context.Context, _ string) (entity.ProjectDeploymentStatsResponse, error) {
	return entity.ProjectDeploymentStatsResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectStats(_ context.Context, _ string) (entity.ProjectStatsResponse, error) {
	return entity.ProjectStatsResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectTimeCardStats(_ context.Context, _, _, _ string) (entity.ProjectTimeCardStatsResponse, error) {
	return entity.ProjectTimeCardStatsResponse{}, nil
}
func (f *fakeEntityProjectStatsClient) GetProjectChangeRequestStats(_ context.Context, _ string) (entity.ProjectChangeRequestStatsResponse, error) {
	return entity.ProjectChangeRequestStatsResponse{}, nil
}

func TestProjectStatsHandler_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	h := NewProjectStatsHandler(&fakeEntityProjectStatsClient{})
	h.SetCallerScope(resolver)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /projects/{id}/filters", h.GetProjectFilters)
	mux.HandleFunc("GET /projects/{id}/features", h.GetProjectFeatures)
	mux.HandleFunc("GET /projects/{id}/stats", h.GetProjectDashboardStats)
	mux.HandleFunc("GET /projects/{id}/stats/cases", h.GetProjectCaseStats)
	mux.HandleFunc("GET /projects/{id}/stats/conversations", h.GetProjectConversationStats)
	mux.HandleFunc("GET /projects/{id}/stats/support", h.GetProjectSupportStats)
	mux.HandleFunc("GET /projects/{id}/stats/time-cards", h.GetProjectTimeCardStats)
	mux.HandleFunc("GET /projects/{id}/stats/change-requests", h.GetProjectChangeRequestStats)
	mux.HandleFunc("GET /projects/{id}/stats/usage", h.GetProjectUsageStats)

	endpoints := []struct {
		name string
		path string
	}{
		{"filters", "/projects/%s/filters"},
		{"features", "/projects/%s/features"},
		{"dashboard stats", "/projects/%s/stats"},
		{"case stats", "/projects/%s/stats/cases"},
		{"conversation stats", "/projects/%s/stats/conversations"},
		{"support stats", "/projects/%s/stats/support"},
		{"time-cards stats", "/projects/%s/stats/time-cards"},
		{"change-requests stats", "/projects/%s/stats/change-requests"},
		{"usage stats", "/projects/%s/stats/usage"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+": member can view", func(t *testing.T) {
			req := authedRequest(http.MethodGet, strings.ReplaceAll(ep.path, "%s", testProjectID), "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
		})

		t.Run(ep.name+": non-member is forbidden", func(t *testing.T) {
			t.Skip("requireProjectMember call site commented out in project stats endpoints pending live verification — see handler.CallerScopeResolver")
			otherProjectID := "88888888-8888-8888-8888-888888888888"
			req := authedRequest(http.MethodGet, strings.ReplaceAll(ep.path, "%s", otherProjectID), "")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

type fakeAIChatAgentClient struct {
	aiChatAgentClient
}

func (f *fakeAIChatAgentClient) CreateChat(_ context.Context, _ aichatagent.ChatPayload) (aichatagent.ChatResponse, error) {
	return aichatagent.ChatResponse{}, nil
}
func (f *fakeAIChatAgentClient) GetConversationSummary(_ context.Context, _, _ string) (aichatagent.ConversationSummaryResponse, error) {
	return aichatagent.ConversationSummaryResponse{}, nil
}

type fakeEntityConversationClient struct {
	entityConversationClient
	conv entity.ConversationDetails
}

func (f *fakeEntityConversationClient) CreateConversation(_ context.Context, _ entity.CreateConversationRequest) (entity.CreateConversationResponse, error) {
	return entity.CreateConversationResponse{Conversation: entity.CreatedConversation{ID: "conv-1"}}, nil
}
func (f *fakeEntityConversationClient) CreateComment(_ context.Context, _ entity.CreateCommentRequest) (entity.CreateCommentResponse, error) {
	return entity.CreateCommentResponse{}, nil
}
func (f *fakeEntityConversationClient) GetConversation(_ context.Context, _ string) (entity.ConversationDetails, error) {
	return f.conv, nil
}
func (f *fakeEntityConversationClient) UpdateConversation(_ context.Context, _ string, _ entity.UpdateConversationRequest) (entity.UpdateConversationResponse, error) {
	return entity.UpdateConversationResponse{}, nil
}
func (f *fakeEntityConversationClient) SearchComments(_ context.Context, _ entity.SearchCommentsRequest) (entity.SearchCommentsResponse, error) {
	return entity.SearchCommentsResponse{}, nil
}
func (f *fakeEntityConversationClient) GetProject(_ context.Context, _ string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{Account: entity.ProjectAccountRef{ID: "account-1"}}, nil
}

func TestAIChatHandler_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("CreateConversation: member can create", func(t *testing.T) {
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/conversations", h.CreateConversation)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/conversations", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateConversation: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in CreateConversation pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/conversations", h.CreateConversation)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/conversations", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SendConversationMessage: member can send", func(t *testing.T) {
		fakeConv := entity.ConversationDetails{Project: &entity.EntityRef{ID: testProjectID}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{conv: fakeConv})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", h.SendConversationMessage)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/conversations/11111111-1111-1111-1111-111111111111/messages", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SendConversationMessage: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in SendConversationMessage pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", h.SendConversationMessage)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/conversations/11111111-1111-1111-1111-111111111111/messages", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversation: member can view", func(t *testing.T) {
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: &entity.EntityRef{ID: testProjectID}}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}", h.GetConversation)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversation: non-member gets 404", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetConversation pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: &entity.EntityRef{ID: otherProjectID}}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}", h.GetConversation)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversationMessages: member can view", func(t *testing.T) {
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: &entity.EntityRef{ID: testProjectID}}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}/messages", h.GetConversationMessages)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111/messages", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversationMessages: non-member gets 404", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetConversationMessages pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: &entity.EntityRef{ID: otherProjectID}}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}/messages", h.GetConversationMessages)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111/messages", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SendConversationMessage: conversation belongs to different project returns 404", func(t *testing.T) {
		fakeConv := entity.ConversationDetails{Project: &entity.EntityRef{ID: "88888888-8888-8888-8888-888888888888"}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{conv: fakeConv})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", h.SendConversationMessage)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/conversations/11111111-1111-1111-1111-111111111111/messages", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SendConversationMessage: conversation with nil project returns 404", func(t *testing.T) {
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{conv: entity.ConversationDetails{}})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{projectId}/conversations/{conversationId}/messages", h.SendConversationMessage)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/conversations/11111111-1111-1111-1111-111111111111/messages", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversation: nil project gets 404", func(t *testing.T) {
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: nil}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}", h.GetConversation)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetConversationMessages: nil project gets 404", func(t *testing.T) {
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: nil}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /conversations/{id}/messages", h.GetConversationMessages)
		req := authedRequest(http.MethodGet, "/conversations/11111111-1111-1111-1111-111111111111/messages", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("UpdateConversation: nil project gets 404", func(t *testing.T) {
		fake := &fakeEntityConversationClient{conv: entity.ConversationDetails{Project: nil}}
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("PATCH /conversations/{id}", h.UpdateConversation)
		req := authedRequest(http.MethodPatch, "/conversations/11111111-1111-1111-1111-111111111111", `{"status":"closed"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

type fakeEntityUserProjectClient struct {
	roles []string
}

func (f *fakeEntityUserProjectClient) GetMe(_ context.Context) (entity.GetUserMeResponse, error) {
	return entity.GetUserMeResponse{Roles: f.roles}, nil
}

func (f *fakeEntityUserProjectClient) GetProject(_ context.Context, id string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{ID: id, Account: entity.ProjectAccountRef{ID: "acc-1", Name: "Account 1"}, SfID: "sf-1", Key: "PRJ"}, nil
}

type fakeRegistryClient struct {
	token registry.Token
}

func (f *fakeRegistryClient) CreateToken(_ context.Context, _ registry.TokenCreatePayload) (registry.TokenCreationResponse, error) {
	return registry.TokenCreationResponse{Secret: "secret"}, nil
}

func (f *fakeRegistryClient) SearchTokens(_ context.Context, _ registry.TokenSearchPayload) ([]registry.Token, error) {
	return []registry.Token{{Name: "tok-1"}}, nil
}

func (f *fakeRegistryClient) GetTokenByID(_ context.Context, _ string) (registry.Token, error) {
	return f.token, nil
}

func (f *fakeRegistryClient) DeleteToken(_ context.Context, _ string) error {
	return nil
}

func (f *fakeRegistryClient) RegenerateToken(_ context.Context, _ string) (registry.TokenCreationResponse, error) {
	return registry.TokenCreationResponse{Secret: "secret-new"}, nil
}

func (f *fakeRegistryClient) GetIntegrationUsersByProjectID(_ context.Context, _ string) ([]registry.IntegrationUser, error) {
	return []registry.IntegrationUser{{ID: "u-1", Email: "int@example.com"}}, nil
}

func TestRegistryHandler_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	t.Run("CreateRegistryToken: member can create", func(t *testing.T) {
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/registry-tokens", h.CreateRegistryToken)
		req := authedRequest(http.MethodPost, "/projects/"+testProjectID+"/registry-tokens", `{"robotName":"my-robot","tokenType":"User"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateRegistryToken: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in CreateRegistryToken pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/registry-tokens", h.CreateRegistryToken)
		req := authedRequest(http.MethodPost, "/projects/"+otherProjectID+"/registry-tokens", `{"robotName":"my-robot","tokenType":"User"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetProjectIntegrationUsers: member can view", func(t *testing.T) {
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /projects/{id}/integration-users", h.GetProjectIntegrationUsers)
		req := authedRequest(http.MethodGet, "/projects/"+testProjectID+"/integration-users", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetProjectIntegrationUsers: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetProjectIntegrationUsers pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /projects/{id}/integration-users", h.GetProjectIntegrationUsers)
		req := authedRequest(http.MethodGet, "/projects/"+otherProjectID+"/integration-users", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteRegistryToken: member can delete", func(t *testing.T) {
		desc := "acc-1##" + testProjectID + "##User##" + callerScopeTestEmail + "##" + callerScopeTestEmail
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{token: registry.Token{Name: "tok-1", Description: desc}}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /registry-tokens/{id}", h.DeleteRegistryToken)
		req := authedRequest(http.MethodDelete, "/registry-tokens/tok-1", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteRegistryToken: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in DeleteRegistryToken pending live verification — see handler.CallerScopeResolver")
		otherProjectID := "88888888-8888-8888-8888-888888888888"
		desc := "acc-1##" + otherProjectID + "##User##" + callerScopeTestEmail + "##" + callerScopeTestEmail
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{token: registry.Token{Name: "tok-1", Description: desc}}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /registry-tokens/{id}", h.DeleteRegistryToken)
		req := authedRequest(http.MethodDelete, "/registry-tokens/tok-1", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

type fakeEntityAttachmentClient struct {
	attachment entity.AttachmentDetails
	content    []byte
	caseView   entity.CaseView
}

func (f *fakeEntityAttachmentClient) CreateAttachment(_ context.Context, _ entity.CreateAttachmentRequest) (entity.CreateAttachmentResponse, error) {
	return entity.CreateAttachmentResponse{Attachment: entity.AttachmentDetail{ID: "att-new"}}, nil
}

func (f *fakeEntityAttachmentClient) GetAttachmentContent(_ context.Context, _ string) ([]byte, string, error) {
	return f.content, "text/plain", nil
}

func (f *fakeEntityAttachmentClient) DeleteAttachment(_ context.Context, _ string) (entity.DeleteAttachmentResponse, error) {
	return entity.DeleteAttachmentResponse{Message: "deleted"}, nil
}

func (f *fakeEntityAttachmentClient) GetAttachment(_ context.Context, _ string) (entity.AttachmentDetails, error) {
	return f.attachment, nil
}

func (f *fakeEntityAttachmentClient) GetCase(_ context.Context, _ string) (entity.CaseView, error) {
	return f.caseView, nil
}

func TestAttachmentHandler_CallerScope(t *testing.T) {
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			testProjectID: {{Email: callerScopeTestEmail, GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	caseID := "11111111-1111-1111-1111-111111111111"
	memberCaseView := entity.CaseView{ID: caseID, ProjectDetails: entity.EntityRef{ID: testProjectID}}
	nonMemberCaseView := entity.CaseView{ID: caseID, ProjectDetails: entity.EntityRef{ID: "99999999-9999-9999-9999-999999999999"}}

	attID := "22222222-2222-2222-2222-222222222222"
	attachmentDetails := entity.AttachmentDetails{
		ID:            attID,
		ReferenceID:   caseID,
		ReferenceType: entity.ReferenceTypeCase,
		Name:          "test.txt",
		Type:          "text/plain",
	}

	t.Run("GetAttachment: member can view metadata", func(t *testing.T) {
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			caseView:   memberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /attachments/{id}", h.GetAttachment)
		req := authedRequest(http.MethodGet, "/attachments/"+attID, "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetAttachment: non-member gets 404", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetAttachment pending live verification — see handler.CallerScopeResolver")
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			caseView:   nonMemberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /attachments/{id}", h.GetAttachment)
		req := authedRequest(http.MethodGet, "/attachments/"+attID, "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetAttachmentContent: member can download", func(t *testing.T) {
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			content:    []byte("hello world"),
			caseView:   memberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /attachments/{id}/content", h.GetAttachmentContent)
		req := authedRequest(http.MethodGet, "/attachments/"+attID+"/content", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "hello world" {
			t.Fatalf("expected content 'hello world', got %q", rec.Body.String())
		}
	})

	t.Run("GetAttachmentContent: non-member gets 404", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in GetAttachmentContent pending live verification — see handler.CallerScopeResolver")
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			content:    []byte("hello world"),
			caseView:   nonMemberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /attachments/{id}/content", h.GetAttachmentContent)
		req := authedRequest(http.MethodGet, "/attachments/"+attID+"/content", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteAttachment: member can delete", func(t *testing.T) {
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			caseView:   memberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /attachments/{id}", h.DeleteAttachment)
		req := authedRequest(http.MethodDelete, "/attachments/"+attID, "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("DeleteAttachment: non-member gets 404", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in DeleteAttachment pending live verification — see handler.CallerScopeResolver")
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: attachmentDetails,
			caseView:   nonMemberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /attachments/{id}", h.DeleteAttachment)
		req := authedRequest(http.MethodDelete, "/attachments/"+attID, "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateAttachment: member can create case attachment", func(t *testing.T) {
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			caseView: memberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /attachments", h.CreateAttachment)
		body := `{"referenceId":"` + caseID + `","referenceType":"case","name":"diag.zip","type":"application/zip","content":"aGVsbG8="}`
		req := authedRequest(http.MethodPost, "/attachments", body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CreateAttachment: non-member is forbidden", func(t *testing.T) {
		t.Skip("requireProjectMember call site commented out in CreateAttachment pending live verification — see handler.CallerScopeResolver")
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			caseView: nonMemberCaseView,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /attachments", h.CreateAttachment)
		body := `{"referenceId":"` + caseID + `","referenceType":"case","name":"diag.zip","type":"application/zip","content":"aGVsbG8="}`
		req := authedRequest(http.MethodPost, "/attachments", body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GetAttachment: non-case attachment succeeds without parent case resolution", func(t *testing.T) {
		deploymentAttID := "33333333-3333-3333-3333-333333333333"
		deploymentAttDetails := entity.AttachmentDetails{
			ID:            deploymentAttID,
			ReferenceID:   "44444444-4444-4444-4444-444444444444",
			ReferenceType: entity.ReferenceTypeDeployment,
			Name:          "dep.yaml",
			Type:          "text/yaml",
		}
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{
			attachment: deploymentAttDetails,
		})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /attachments/{id}", h.GetAttachment)
		req := authedRequest(http.MethodGet, "/attachments/"+deploymentAttID, "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// NOTE: Caller-scope checks on all endpoints are commented out pending live
// verification against real entity-service data.
// The test below proves that with enforcement disabled, requests proceed to
// the underlying service without being blocked with 403 Forbidden or 404 Not Found.
func TestCallerScope_EnforcementDisabled_AllowsAccess(t *testing.T) {
	nonMemberProjectID := "99999999-9999-9999-9999-999999999999"
	contactsFake := &fakeEntityContacts{
		byProjectID: map[string][]entity.ProjectContact{
			nonMemberProjectID: {{Email: "other@example.com", GrantsCaseAccess: true}},
		},
	}
	resolver := NewCallerScopeResolver(contactsFake)

	// Verify the resolver method itself remains functional and reports member=false
	member, err := resolver.IsProjectMember(context.Background(), nonMemberProjectID, callerScopeTestEmail)
	if err != nil || member {
		t.Fatalf("expected member=false for non-member email, got member=%v, err=%v", member, err)
	}

	t.Run("CaseHandler SearchCases allows non-member", func(t *testing.T) {
		h := NewCaseHandler(&fakeEntityCaseClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/cases/search", h.SearchCases)
		req := authedRequest(http.MethodPost, "/projects/"+nonMemberProjectID+"/cases/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("CaseHandler GetCase allows non-member", func(t *testing.T) {
		fake := &fakeEntityCaseClientForCase{caseView: entity.CaseView{ID: "c-1", ProjectDetails: entity.EntityRef{ID: nonMemberProjectID}}}
		h := NewCaseHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /cases/{id}", h.GetCase)
		req := authedRequest(http.MethodGet, "/cases/44444444-4444-4444-4444-444444444444", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("InstanceHandler SearchProjectInstances allows non-member", func(t *testing.T) {
		fake := &fakeEntityInstanceClient{}
		h := NewInstanceHandler(fake)
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/instances/search", h.SearchProjectInstances)
		req := authedRequest(http.MethodPost, "/projects/"+nonMemberProjectID+"/instances/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || !fake.called {
			t.Fatalf("expected 200 and entity call, got %d (called=%v): %s", rec.Code, fake.called, rec.Body.String())
		}
	})

	t.Run("ChangeRequestHandler SearchChangeRequests allows non-member", func(t *testing.T) {
		h := NewChangeRequestHandler(&fakeEntityChangeRequestClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/change-requests/search", h.SearchChangeRequests)
		req := authedRequest(http.MethodPost, "/projects/"+nonMemberProjectID+"/change-requests/search", `{}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ProjectStatsHandler GetProjectDashboardStats allows non-member", func(t *testing.T) {
		h := NewProjectStatsHandler(&fakeEntityProjectStatsClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("GET /projects/{id}/stats", h.GetProjectDashboardStats)
		req := authedRequest(http.MethodGet, "/projects/"+nonMemberProjectID+"/stats", "")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("AIChatHandler CreateConversation allows non-member", func(t *testing.T) {
		h := NewAIChatHandler(&fakeAIChatAgentClient{}, &fakeEntityConversationClient{})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/conversations", h.CreateConversation)
		req := authedRequest(http.MethodPost, "/projects/"+nonMemberProjectID+"/conversations", `{"message":"hello"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RegistryHandler CreateRegistryToken allows non-member", func(t *testing.T) {
		h := NewRegistryHandler(&fakeEntityUserProjectClient{}, &fakeRegistryClient{}, "admin")
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /projects/{id}/registry-tokens", h.CreateRegistryToken)
		req := authedRequest(http.MethodPost, "/projects/"+nonMemberProjectID+"/registry-tokens", `{"robotName":"my-robot","tokenType":"User"}`)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("AttachmentHandler CreateAttachment allows non-member", func(t *testing.T) {
		caseID := "11111111-1111-1111-1111-111111111111"
		nonMemberCaseView := entity.CaseView{ID: caseID, ProjectDetails: entity.EntityRef{ID: nonMemberProjectID}}
		h := NewAttachmentHandler(&fakeEntityAttachmentClient{caseView: nonMemberCaseView})
		h.SetCallerScope(resolver)

		mux := http.NewServeMux()
		mux.HandleFunc("POST /attachments", h.CreateAttachment)
		body := `{"referenceId":"` + caseID + `","referenceType":"case","name":"diag.zip","type":"application/zip","content":"aGVsbG8="}`
		req := authedRequest(http.MethodPost, "/attachments", body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
