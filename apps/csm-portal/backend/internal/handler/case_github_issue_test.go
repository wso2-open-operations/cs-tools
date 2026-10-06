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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

type createIssueCall struct {
	orgName, owner, repo, title, body string
	labels                            []string
}

type mockEngineeringClient struct {
	calls []createIssueCall
	issue entity.GitHubIssue
	err   error
}

func (m *mockEngineeringClient) CreateGitIssue(_ context.Context, orgName, owner, repoName, title, body string, labels []string) (entity.GitHubIssue, error) {
	m.calls = append(m.calls, createIssueCall{orgName, owner, repoName, title, body, labels})
	return m.issue, m.err
}

func TestCreateCaseGithubIssue_ViaEngineering(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	post := func(t *testing.T, h *CaseHandler, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader(body)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)
		return w
	}
	newHandler := func(eng *mockEngineeringClient, entityClient *mockEntityCaseClient) *CaseHandler {
		if entityClient.getCaseFn == nil {
			entityClient.getCaseFn = func(context.Context, string) ([]byte, error) {
				return []byte(`{"deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			}
		}
		if entityClient.getProductRepoMappingFn == nil {
			entityClient.getProductRepoMappingFn = func(context.Context, string) ([]byte, error) {
				return []byte(`{"productName":"Alpha","owner":"example-org","repository":"alpha-repo","githubLabel":"Alpha"}`), nil
			}
		}
		return NewCaseHandler(entityClient).WithEngineeringClient(eng)
	}
	const base = `"title":"Crash on startup","description":"It fails.","repoOverride":{"owner":"example-org","repo":"alpha-repo"}`

	t.Run("creates the issue through the engineering service, not the entity service", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 42}}
		entityCalled := false
		entityClient := &mockEntityCaseClient{
			createCaseGithubIssueFn: func(context.Context, string, []byte) ([]byte, error) {
				entityCalled = true
				return nil, nil
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")

		assertStatus(t, w, http.StatusCreated)
		if entityCalled {
			t.Error("the entity service's CreateCaseGithubIssue was called; it must be bypassed")
		}
		if len(eng.calls) != 1 {
			t.Fatalf("CreateGitIssue calls = %d, want 1", len(eng.calls))
		}
		c := eng.calls[0]
		wantBody := "Product : Alpha\n\nIt fails."
		if c.orgName != "example-org" || c.owner != "example-org" || c.repo != "alpha-repo" || c.title != "Crash on startup" || c.body != wantBody {
			t.Errorf("call = %+v", c)
		}
		if !slices.Equal(c.labels, []string{"Origin/CS", "Alpha"}) {
			t.Errorf("labels = %v, want Origin/CS and the repo option's own label", c.labels)
		}
		type resp struct {
			Message string `json:"message"`
			Issue   struct {
				URL    string `json:"url"`
				Number int    `json:"number"`
				Repo   string `json:"repo"`
			} `json:"issue"`
		}
		got := decodeJSON[resp](t, w)
		if got.Issue.Number != 42 || got.Issue.Repo != "example-org/alpha-repo" || got.Issue.URL != "https://github.com/example-org/alpha-repo/issues/42" {
			t.Errorf("response = %+v", got)
		}
	})

	t.Run("writes the issue URL back onto the case", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 42}}
		var note []byte
		entityClient := &mockEntityCaseClient{
			createCaseCommentFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				if id != caseID {
					t.Errorf("work note case = %s, want %s", id, caseID)
				}
				note = body
				return []byte(`{}`), nil
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusCreated)
		if !strings.Contains(string(note), `"type":"work_note"`) || !strings.Contains(string(note), "https://github.com/example-org/alpha-repo/issues/42") {
			t.Errorf("work note = %s", note)
		}
	})

	t.Run("files in the mapped repository and ignores repoOverride", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		w := post(t, newHandler(eng, &mockEntityCaseClient{}), `{"title":"t","description":"d","repoOverride":{"owner":"someone-else","repo":"not-the-mapped-repo"}}`)
		assertStatus(t, w, http.StatusCreated)
		if len(eng.calls) != 1 || eng.calls[0].owner != "example-org" || eng.calls[0].repo != "alpha-repo" {
			t.Errorf("calls = %+v, want the mapping, not the request body", eng.calls)
		}
	})

	t.Run("builds the body and labels from the optional fields", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		w := post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"updateLevel":"U12","publicIssueUrl":"https://example.com/i/1","hotFixRequired":true,"regression":true,"reason":"migration","onboardingInProgress":true,"issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/High"}`)
		assertStatus(t, w, http.StatusCreated)
		c := eng.calls[0]
		wantBody := "Product : Alpha\nUpdate Level : U12\n\nIt fails.\n\nPublic Issue : https://example.com/i/1\n\nHotfix Required : Yes"
		if c.body != wantBody {
			t.Errorf("body = %q, want %q", c.body, wantBody)
		}
		if want := []string{"Origin/CS", "Alpha", "Priority/High", "Require/Hotfix", "regression", "Affected/Migration", "Onboarding/affected"}; !slices.Equal(c.labels, want) {
			t.Errorf("labels = %v, want %v", c.labels, want)
		}
	})

	t.Run("patch adds its labels and discussion is what carries priority", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Patch","priorityLevel":"Priority/High","regression":true}`)
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/Critical"}`)
		if want := []string{"Origin/CS", "Alpha", "Type/Patch", "patch", "regression"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[0].labels, want)
		}
		if want := []string{"Origin/CS", "Alpha", "Priority/Critical"}; !slices.Equal(eng.calls[1].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[1].labels, want)
		}
	})

	t.Run("the fixed info block carries the case's own fields and omits what's missing", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{
					"number":"CS0000001",
					"internalId":"ACME-1",
					"assignedTeam":{"id":"t1","name":"Falcon"},
					"createdBy":{"email":"reporter@example.com","name":"Test Reporter"},
					"deployedProduct":{"product":{"name":"Alpha"}}
				}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+`,"updateLevel":"86"}`)
		wantBody := "Product : Alpha\nUpdate Level : 86\nWSO2 Case Id : ACME-1\nCase Number : CS0000001\nABT Team : Falcon\nOpened By : reporter@example.com\n\nIt fails."
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
		want := []string{"Origin/CS", "Alpha"}
		if !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v -- the update level must never appear as a label", eng.calls[0].labels, want)
		}
	})

	t.Run("a case missing team/reporter omits those lines instead of rendering them empty", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"number":"CS0000001","deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+"}")
		wantBody := "Product : Alpha\nCase Number : CS0000001\n\nIt fails."
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
	})

	t.Run("an empty mapping label falls back to the case's own product name for the Product line", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"deployedProduct":{"product":{"name":"Beta"}}}`), nil
			},
			getProductRepoMappingFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"productName":"Beta","owner":"example-org","repository":"alpha-repo","githubLabel":""}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+"}")
		wantBody := "Product : Beta\n\nIt fails."
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
		// The empty label must not become a GitHub label either -- buildGitHubIssueLabels
		// is unaffected by this fallback, which is body-only.
		if want := []string{"Origin/CS"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[0].labels, want)
		}
	})

	t.Run("a priority string in the update level is not applied as a label", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Patch","updateLevel":"Priority/Critical"}`)
		if want := []string{"Origin/CS", "Alpha", "Type/Patch", "patch"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[0].labels, want)
		}
	})

	t.Run("rejects a bad request before touching any service", func(t *testing.T) {
		tests := []struct {
			name, body, wantMsg string
		}{
			{"malformed shape", `{"title":42}`, ErrMsgBadRequest},
			{"missing title", `{"description":"d","repoOverride":{"owner":"example-org","repo":"alpha-repo"}}`, errMsgGitHubTitleInvalid},
			{"blank title", `{"title":"   ","repoOverride":{"owner":"example-org","repo":"alpha-repo"}}`, errMsgGitHubTitleInvalid},
			{"title too long", `{"title":"` + strings.Repeat("x", 257) + `","repoOverride":{"owner":"example-org","repo":"alpha-repo"}}`, errMsgGitHubTitleInvalid},
			{"description too long", `{"title":"t","description":"` + strings.Repeat("x", 65537) + `","repoOverride":{"owner":"example-org","repo":"alpha-repo"}}`, errMsgGitHubBodyTooLong},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				eng := &mockEngineeringClient{}
				getCaseCalled := false
				entityClient := &mockEntityCaseClient{getCaseFn: func(context.Context, string) ([]byte, error) {
					getCaseCalled = true
					return []byte(`{}`), nil
				}}
				w := post(t, newHandler(eng, entityClient), tc.body)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, tc.wantMsg)
				if len(eng.calls) != 0 || getCaseCalled {
					t.Errorf("engineering calls = %d, GetCase called = %v, want neither", len(eng.calls), getCaseCalled)
				}
			})
		}
	})

	t.Run("a product with no mapping is not filed", func(t *testing.T) {
		eng := &mockEngineeringClient{}
		entityClient := &mockEntityCaseClient{
			getProductRepoMappingFn: func(context.Context, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, errMsgGitHubRepoNotMapped)
		if len(eng.calls) != 0 {
			t.Errorf("CreateGitIssue calls = %d, want 0", len(eng.calls))
		}
	})

	t.Run("a case with no product is not filed", func(t *testing.T) {
		eng := &mockEngineeringClient{}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{}`), nil
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, errMsgGitHubRepoNotMapped)
		if len(eng.calls) != 0 {
			t.Errorf("CreateGitIssue calls = %d, want 0", len(eng.calls))
		}
	})

	t.Run("a case the caller cannot see is not filed against", func(t *testing.T) {
		eng := &mockEngineeringClient{}
		entityClient := &mockEntityCaseClient{getCaseFn: func(context.Context, string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusNotFound}
		}}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusNotFound)
		if len(eng.calls) != 0 {
			t.Errorf("CreateGitIssue calls = %d, want 0", len(eng.calls))
		}
	})

	t.Run("upstream engineering failures are mapped without leaking detail", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create GitHub issue.") {
			t.Run(tc.name, func(t *testing.T) {
				eng := &mockEngineeringClient{err: tc.err}
				w := post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+"}")
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
		eng := &mockEngineeringClient{err: errors.New("dial tcp: refused")}
		w := post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+"}")
		assertStatus(t, w, http.StatusInternalServerError)
	})

	t.Run("without an engineering client the request goes to the entity service as before", func(t *testing.T) {
		entityCalled := false
		entityClient := &mockEntityCaseClient{createCaseGithubIssueFn: func(_ context.Context, id string, _ []byte) ([]byte, error) {
			entityCalled = true
			return []byte(`{"issue":{}}`), nil
		}}
		w := post(t, NewCaseHandler(entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusCreated)
		if !entityCalled {
			t.Error("the entity service was not called")
		}
	})
}
