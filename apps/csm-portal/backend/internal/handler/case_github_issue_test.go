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
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

type createIssueCall struct {
	orgName, owner, repo, title, body string
	labels                            []string
	// ctxErr and hasDeadline are the state of the context CreateGitIssue received, read
	// at the time of the call.
	ctxErr      error
	hasDeadline bool
}

type mockEngineeringClient struct {
	calls []createIssueCall
	issue entity.GitHubIssue
	err   error
}

func (m *mockEngineeringClient) CreateGitIssue(ctx context.Context, orgName, owner, repoName, title, body string, labels []string) (entity.GitHubIssue, error) {
	_, hasDeadline := ctx.Deadline()
	m.calls = append(m.calls, createIssueCall{orgName, owner, repoName, title, body, labels, ctx.Err(), hasDeadline})
	return m.issue, m.err
}

// infoBlock is the footer as the earlier Git issues carry it: all seven lines, in this order,
// a value that is not known written as "undefined", the word the earlier issues carry there.
func infoBlock(product, updateLevel, caseID, caseNumber, publicIssue, team, openedBy string) string {
	dash := func(v string) string {
		if v == "" {
			return "undefined"
		}
		return v
	}
	return "Product : " + dash(product) +
		"\nUpdate level : " + dash(updateLevel) +
		"\nWSO2 case id : " + dash(caseID) +
		"\nCase number : " + dash(caseNumber) +
		"\nPublic Git Issue : " + dash(publicIssue) +
		"\nABT Team : " + dash(team) +
		"\nOpened by : " + dash(openedBy)
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
		wantBody := "It fails.\n\n" + infoBlock("Alpha", "", "", "", "", "", testUser.Email)
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
		wantBody := "It fails.\n\nHotfix Required : Yes\n\n" + infoBlock("Alpha", "U12", "", "", "https://example.com/i/1", "", testUser.Email)
		if c.body != wantBody {
			t.Errorf("body = %q, want %q", c.body, wantBody)
		}
		if want := []string{"Origin/CS", "Alpha", "Type/Incident", "Priority/High(P2)", "Require/Hotfix", "regression", "Affected/Migration", "Onboarding/affected"}; !slices.Equal(c.labels, want) {
			t.Errorf("labels = %v, want %v", c.labels, want)
		}
	})

	t.Run("patch adds its labels and discussion is what carries priority", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Patch","priorityLevel":"Priority/High","regression":true}`)
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/Critical"}`)
		post(t, newHandler(eng, &mockEntityCaseClient{}), "{"+base+`,"issueTypeLabel":"Type/Discussion","priorityLevel":"Priority/Medium(P3)"}`)
		if want := []string{"Origin/CS", "Alpha", "Type/Patch", "patch", "regression"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[0].labels, want)
		}
		if want := []string{"Origin/CS", "Alpha", "Type/Incident", "Priority/Critical(P1)"}; !slices.Equal(eng.calls[1].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[1].labels, want)
		}
		if want := []string{"Origin/CS", "Alpha", "Type/Incident", "Priority/Medium(P3)"}; !slices.Equal(eng.calls[2].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[2].labels, want)
		}
	})

	t.Run("the fixed info block carries the case's own fields, undefined for what the form left blank", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{
					"number":"CS0000001",
					"internalId":"ACME-1",
					"assignedTeam":{"id":"t9","name":"Support Group"},
					"account":{"id":"a1","name":"Acme","creTeam":{"id":"t1","name":"Falcon"},"sreTeam":{"id":"t2","name":"Osprey"}},
					"createdBy":{"email":"customer@customer.example","name":"Test Customer"},
					"deployedProduct":{"displayName":"Alpha 1.2.3","product":{"name":"Alpha"}}
				}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+`,"updateLevel":"86"}`)
		// Opened By is the engineer filing the issue (the signed-in user), never the
		// case's createdBy, which is the customer who raised it. ABT Team is the
		// account's CRE team, not the case's assignedTeam or the SRE team, and Product
		// is the deployed product's own label, which carries the version.
		wantBody := "It fails.\n\n" + infoBlock("Alpha 1.2.3", "86", "ACME-1", "CS0000001", "", "Falcon", testUser.Email)
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
		for _, leaked := range []string{"customer@customer.example", "Support Group", "Osprey"} {
			if strings.Contains(eng.calls[0].body, leaked) {
				t.Errorf("body = %q, must not carry %q", eng.calls[0].body, leaked)
			}
		}
		want := []string{"Origin/CS", "Alpha"}
		if !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v -- the update level must never appear as a label", eng.calls[0].labels, want)
		}
	})

	// A case as the portal reads it for the Git issue form: a deployed product whose label
	// carries the version, on a deployment, under an account with a CRE team.
	const isCase = `{
		"number":"CS0378654",
		"internalId":"MPASUB-231",
		"account":{"id":"a1","name":"Acme","creTeam":{"id":"t1","name":"Sirius"}},
		"deployment":{"id":"d1111111-1111-1111-1111-111111111111","name":"Production"},
		"deployedProduct":{"id":"dd111111-1111-1111-1111-111111111111","displayName":"WSO2 Identity Server 6.1.0","product":{"name":"WSO2 Identity Server"}}
	}`
	isMapping := func(context.Context, string) ([]byte, error) {
		return []byte(`{"productName":"WSO2 Identity Server","owner":"example-org","repository":"is-repo","githubLabel":"product/is"}`), nil
	}
	// deployed-product search results: other products on the deployment, then the case's own
	const isProducts = `{"deployedProducts":[
		{"id":"ee111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2am"},"version":{"name":"4.1.0"}},
		{"id":"DD111111111111111111111111111111","product":{"abbreviation":"wso2is"},"version":{"name":"6.1.0"}}
	],"hasMore":false}`
	fileIsCase := func(t *testing.T, search func(context.Context, []byte) ([]byte, error)) (*mockEngineeringClient, *httptest.ResponseRecorder) {
		t.Helper()
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn:                func(context.Context, string) ([]byte, error) { return []byte(isCase), nil },
			getProductRepoMappingFn:  isMapping,
			searchDeployedProductsFn: search,
		}
		return eng, post(t, newHandler(eng, entityClient), "{"+base+`,"updateLevel":"295"}`)
	}
	fallbackBody := "It fails.\n\n" + infoBlock("WSO2 Identity Server 6.1.0", "295", "MPASUB-231", "CS0378654", "", "Sirius", testUser.Email)

	t.Run("a case's product is its abbreviation and version, with the CRE team, while the repository label stays a label", func(t *testing.T) {
		var searched []byte
		eng, w := fileIsCase(t, func(_ context.Context, body []byte) ([]byte, error) {
			searched = body
			return []byte(isProducts), nil
		})
		assertStatus(t, w, http.StatusCreated)
		wantBody := "It fails.\n\n" + infoBlock("wso2is-6.1.0", "295", "MPASUB-231", "CS0378654", "", "Sirius", testUser.Email)
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
		if want := []string{"Origin/CS", "product/is"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v -- the repository's GitHub label still goes on as a label", eng.calls[0].labels, want)
		}
		if !strings.Contains(string(searched), `"deploymentIds":["d1111111-1111-1111-1111-111111111111"]`) {
			t.Errorf("search body = %s, want the case's deployment", searched)
		}
	})

	t.Run("a failed product lookup keeps the deployed product's label and still files the issue", func(t *testing.T) {
		eng, w := fileIsCase(t, func(context.Context, []byte) ([]byte, error) { return nil, errors.New("entity down") })
		assertStatus(t, w, http.StatusCreated)
		if got := eng.calls[0].body; got != fallbackBody {
			t.Errorf("body = %q, want %q", got, fallbackBody)
		}
	})

	t.Run("a product lookup that hangs cannot hold the filing back", func(t *testing.T) {
		old := deployedProductLookupTimeout
		deployedProductLookupTimeout = 50 * time.Millisecond
		t.Cleanup(func() { deployedProductLookupTimeout = old })
		started := time.Now()
		eng, w := fileIsCase(t, func(ctx context.Context, _ []byte) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})
		assertStatus(t, w, http.StatusCreated)
		if took := time.Since(started); took > 2*time.Second {
			t.Errorf("filing took %v, want it bounded by the lookup timeout", took)
		}
		// The filing call must get the request's own context, not the lookup's timed-out one.
		if c := eng.calls[0]; c.ctxErr != nil || c.hasDeadline {
			t.Errorf("CreateGitIssue context: err=%v deadline=%v, want a live context with no deadline", c.ctxErr, c.hasDeadline)
		}
		if got := eng.calls[0].body; got != fallbackBody {
			t.Errorf("body = %q, want %q", got, fallbackBody)
		}
	})

	t.Run("a deployed product missing its abbreviation or version, or not on the deployment, keeps the label", func(t *testing.T) {
		for name, products := range map[string]string{
			"no abbreviation": `{"deployedProducts":[{"id":"dd111111-1111-1111-1111-111111111111","product":{},"version":{"name":"6.1.0"}}],"hasMore":false}`,
			"blank version":   `{"deployedProducts":[{"id":"dd111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2is"},"version":{"name":" "}}],"hasMore":false}`,
			"no version":      `{"deployedProducts":[{"id":"dd111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2is"}}],"hasMore":false}`,
			"not listed":      `{"deployedProducts":[{"id":"ee111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2am"},"version":{"name":"4.1.0"}}],"hasMore":false}`,
			"not json":        `<html>`,
		} {
			eng, w := fileIsCase(t, func(context.Context, []byte) ([]byte, error) { return []byte(products), nil })
			assertStatus(t, w, http.StatusCreated)
			if got := eng.calls[0].body; got != fallbackBody {
				t.Errorf("%s: body = %q, want %q", name, got, fallbackBody)
			}
		}
	})

	t.Run("the case's deployed product is found on a later page of the deployment's products", func(t *testing.T) {
		var offsets []string
		eng, w := fileIsCase(t, func(_ context.Context, body []byte) ([]byte, error) {
			offsets = append(offsets, string(body))
			if strings.Contains(string(body), `"offset":0`) {
				return []byte(`{"deployedProducts":[{"id":"ee111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2am"},"version":{"name":"4.1.0"}}],"hasMore":true}`), nil
			}
			return []byte(isProducts), nil
		})
		assertStatus(t, w, http.StatusCreated)
		if !strings.Contains(eng.calls[0].body, "Product : wso2is-6.1.0\n") {
			t.Errorf("body = %q, want the abbreviation and version from the second page", eng.calls[0].body)
		}
		if len(offsets) != 2 {
			t.Errorf("searches = %d, want 2 (one per page)", len(offsets))
		} else if !strings.Contains(offsets[1], `"offset":50`) || !strings.Contains(offsets[1], `"limit":50`) {
			t.Errorf("second search = %s, want offset 50 and limit 50", offsets[1])
		}
	})

	t.Run("the walk stops after the page limit when the entity service always says there is more", func(t *testing.T) {
		searches := 0
		eng, w := fileIsCase(t, func(context.Context, []byte) ([]byte, error) {
			searches++
			return []byte(`{"deployedProducts":[{"id":"ee111111-1111-1111-1111-111111111111","product":{"abbreviation":"wso2am"},"version":{"name":"4.1.0"}}],"hasMore":true}`), nil
		})
		assertStatus(t, w, http.StatusCreated)
		if searches != deployedProductLookupMaxPages {
			t.Errorf("searches = %d, want %d (the page limit)", searches, deployedProductLookupMaxPages)
		}
		if got := eng.calls[0].body; got != fallbackBody {
			t.Errorf("body = %q, want %q", got, fallbackBody)
		}
	})

	t.Run("an empty page ends the walk even when it says there is more", func(t *testing.T) {
		searches := 0
		eng, w := fileIsCase(t, func(context.Context, []byte) ([]byte, error) {
			searches++
			return []byte(`{"deployedProducts":[],"hasMore":true}`), nil
		})
		assertStatus(t, w, http.StatusCreated)
		if searches != 1 {
			t.Errorf("searches = %d, want 1", searches)
		}
		if got := eng.calls[0].body; got != fallbackBody {
			t.Errorf("body = %q, want %q", got, fallbackBody)
		}
	})

	t.Run("a case naming only one of its deployment and deployed product makes no lookup", func(t *testing.T) {
		for name, caseJSON := range map[string]string{
			"deployment but no deployed product id": `{"deployment":{"id":"d1111111-1111-1111-1111-111111111111"},"deployedProduct":{"id":null,"displayName":"Alpha 1.2.3","product":{"name":"Alpha"}}}`,
			"deployed product id but no deployment": `{"deployedProduct":{"id":"dd111111-1111-1111-1111-111111111111","displayName":"Alpha 1.2.3","product":{"name":"Alpha"}}}`,
			"blank deployment id":                   `{"deployment":{"id":"  "},"deployedProduct":{"id":"dd111111-1111-1111-1111-111111111111","displayName":"Alpha 1.2.3","product":{"name":"Alpha"}}}`,
		} {
			eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
			entityClient := &mockEntityCaseClient{
				getCaseFn: func(context.Context, string) ([]byte, error) { return []byte(caseJSON), nil },
				searchDeployedProductsFn: func(context.Context, []byte) ([]byte, error) {
					t.Errorf("%s: the deployed product lookup ran", name)
					return nil, nil
				},
			}
			assertStatus(t, post(t, newHandler(eng, entityClient), "{"+base+"}"), http.StatusCreated)
			if got := eng.calls[0].body; !strings.Contains(got, "Product : Alpha 1.2.3") {
				t.Errorf("%s: body = %q, want the display name", name, got)
			}
		}
	})

	t.Run("with no display name the Product is the catalogue name, not the repository label", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"deployedProduct":{"product":{"name":"Gamma Server"}}}`), nil
			},
			getProductRepoMappingFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"productName":"Gamma Server","owner":"example-org","repository":"g-repo","githubLabel":"product/g"}`), nil
			},
		}
		assertStatus(t, post(t, newHandler(eng, entityClient), "{"+base+"}"), http.StatusCreated)
		if want := "It fails.\n\n" + infoBlock("Gamma Server", "", "", "", "", "", testUser.Email); eng.calls[0].body != want {
			t.Errorf("body = %q, want %q", eng.calls[0].body, want)
		}
		if want := []string{"Origin/CS", "product/g"}; !slices.Equal(eng.calls[0].labels, want) {
			t.Errorf("labels = %v, want %v", eng.calls[0].labels, want)
		}
	})

	t.Run("a value holding a newline stays on its own footer line", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{
					"internalId":"ACME-1",
					"account":{"creTeam":{"name":"Falcon\nOpened By : evil@example.com"}},
					"deployedProduct":{"displayName":"Alpha\n1.2.3","product":{"name":"Alpha"}}
				}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+`,"updateLevel":"86\r\nABT Team : evil"}`)
		for _, l := range strings.Split(eng.calls[0].body, "\n") {
			if strings.HasPrefix(l, "Opened by : evil") || strings.HasPrefix(l, "ABT Team : evil") {
				t.Errorf("body = %q: a value started a footer line of its own: %q", eng.calls[0].body, l)
			}
		}
		if !strings.HasSuffix(eng.calls[0].body, "Opened by : "+testUser.Email) {
			t.Errorf("body = %q, want it to end with the real Opened by line", eng.calls[0].body)
		}
		// Nothing but the seven footer lines: the description is only "It fails.", then a blank line.
		if got := strings.Count(eng.calls[0].body, "\n"); got != 1+7 {
			t.Errorf("body = %q has %d line breaks, want 8 (description, blank line, seven footer lines)", eng.calls[0].body, got)
		}
	})

	t.Run("a case with no deployment or deployed product makes no lookup", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
			searchDeployedProductsFn: func(context.Context, []byte) ([]byte, error) {
				t.Error("the deployed product lookup ran for a case that names no deployment")
				return nil, nil
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusCreated)
	})

	t.Run("an account with no CRE team keeps the case's assigned team as the ABT Team, never the SRE team", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{
					"assignedTeam":{"id":"t9","name":"Support Group"},
					"account":{"id":"a1","name":"Acme","sreTeam":{"id":"t2","name":"Osprey"}},
					"deployedProduct":{"product":{"name":"Alpha"}}
				}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+"}")
		got := eng.calls[0].body
		if !strings.Contains(got, "ABT Team : Support Group") || strings.Contains(got, "Osprey") {
			t.Errorf("body = %q, want the assigned team as ABT Team (what the line carried before) and no SRE team", got)
		}
	})

	t.Run("a case with neither a CRE team nor an assigned team has undefined for the ABT Team", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"account":{"id":"a1","creTeam":null},"deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+"}")
		if got := eng.calls[0].body; !strings.Contains(got, "\nABT Team : undefined\n") {
			t.Errorf("body = %q, want the ABT Team line kept, with undefined", got)
		}
	})

	t.Run("an account of an unexpected shape costs only the team line, not the case number or id", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"number":"CS0000009","internalId":"ACME-9","account":"not-an-object","deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
		}
		w := post(t, newHandler(eng, entityClient), "{"+base+"}")
		assertStatus(t, w, http.StatusCreated)
		want := "It fails.\n\n" + infoBlock("Alpha", "", "ACME-9", "CS0000009", "", "", testUser.Email)
		if got := eng.calls[0].body; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
	})

	t.Run("the info block is the footer, after the description and the hotfix line, even when the description quotes its own Opened by", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"number":"CS0000001","deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
		}
		const quoted = "Opened by : someone.else@example.com"
		post(t, newHandler(eng, entityClient), `{"title":"t","description":"`+quoted+`","hotFixRequired":true,"publicIssueUrl":"https://example.com/i/1"}`)
		got := eng.calls[0].body
		footer := infoBlock("Alpha", "", "", "CS0000001", "https://example.com/i/1", "", testUser.Email)
		if !strings.HasSuffix(got, "\n\n"+footer) {
			t.Errorf("body = %q, want it to end with the info block %q", got, footer)
		}
		if i, j := strings.Index(got, "Hotfix Required : Yes"), strings.Index(got, footer); i < 0 || i > j {
			t.Errorf("body = %q, want the hotfix line before the info block", got)
		}
		if !strings.HasPrefix(got, quoted) {
			t.Errorf("body = %q, want it to open with the description", got)
		}
		if strings.LastIndex(got, "Opened") < strings.LastIndex(got, quoted) {
			t.Errorf("body = %q: the quoted line comes after the info block's own Opened By", got)
		}
	})

	t.Run("a case missing its team writes undefined for that line instead of dropping it or leaving it empty", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"number":"CS0000001","deployedProduct":{"product":{"name":"Alpha"}}}`), nil
			},
		}
		post(t, newHandler(eng, entityClient), "{"+base+"}")
		wantBody := "It fails.\n\n" + infoBlock("Alpha", "", "", "CS0000001", "", "", testUser.Email)
		if got := eng.calls[0].body; got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
	})

	t.Run("a token with no email writes undefined for Opened by rather than falling back to the case creator", func(t *testing.T) {
		eng := &mockEngineeringClient{issue: entity.GitHubIssue{Number: 1}}
		entityClient := &mockEntityCaseClient{
			getCaseFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{
					"number":"CS0000001",
					"createdBy":{"email":"customer@customer.example"},
					"deployedProduct":{"product":{"name":"Alpha"}}
				}`), nil
			},
		}
		h := newHandler(eng, entityClient)
		noEmail := *testUser
		noEmail.Email = "  "
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/github-issues", strings.NewReader("{"+base+"}"))
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &noEmail))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseGithubIssue(w, r)

		assertStatus(t, w, http.StatusCreated)
		wantBody := "It fails.\n\n" + infoBlock("Alpha", "", "", "CS0000001", "", "", "")
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
		wantBody := "It fails.\n\n" + infoBlock("Beta", "", "", "", "", "", testUser.Email)
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

// With no product label on the case itself, the Product line falls back to the repository
// mapping's label rather than going missing.
func TestBuildGitHubIssueBody_ProductFallsBackToTheMappingLabel(t *testing.T) {
	got := buildGitHubIssueBody(caseGitHubIssueRequest{Description: "d"}, "product/is", caseGitHubIssueContext{})
	if want := "d\n\n" + infoBlock("product/is", "", "", "", "", "", ""); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	got = buildGitHubIssueBody(caseGitHubIssueRequest{Description: "d"}, "product/is", caseGitHubIssueContext{Product: "IS 6.1.0"})
	if want := "d\n\n" + infoBlock("IS 6.1.0", "", "", "", "", "", ""); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

// The footer is read by things outside this repo: the ABT Team Notifications workflow of the
// internal repositories finds its lines with a case-sensitive fixed-string match on
// "WSO2 case id :", "Case number :", "ABT Team :" and "Opened by :", and the Git Internals
// dashboard reads "ABT Team" and "Opened by". So the block is pinned here as literal text, not
// built from the same constants as the code: its spelling, order and seven lines must not
// change, and no line may ever be dropped, whatever is known about the case.
func TestBuildGitHubIssueBody_FooterIsTheFixedSevenLines(t *testing.T) {
	const blank = "Product : undefined\nUpdate level : undefined\nWSO2 case id : undefined\nCase number : undefined\nPublic Git Issue : undefined\nABT Team : undefined\nOpened by : undefined"
	if got := buildGitHubIssueBody(caseGitHubIssueRequest{}, "", caseGitHubIssueContext{}); got != blank {
		t.Errorf("an empty request and case: body = %q, want %q", got, blank)
	}
	const full = "Product : wso2am-4.4.0\nUpdate level : 70\nWSO2 case id : ACME-1\nCase number : CS0000001\nPublic Git Issue : https://example.com/i/1\nABT Team : Sirius\nOpened by : engineer@wso2.com"
	got := buildGitHubIssueBody(
		caseGitHubIssueRequest{Description: "d", UpdateLevel: "70", PublicIssueURL: "https://example.com/i/1"},
		"product/apim",
		caseGitHubIssueContext{Product: "wso2am-4.4.0", InternalID: "ACME-1", Number: "CS0000001", Team: "Sirius", OpenedBy: "engineer@wso2.com"},
	)
	if want := "d\n\n" + full; got != want {
		t.Errorf("a known case: body = %q, want %q", got, want)
	}
	// Blank or whitespace-only values are blanks too.
	got = buildGitHubIssueBody(caseGitHubIssueRequest{UpdateLevel: "  ", PublicIssueURL: "\t"}, "", caseGitHubIssueContext{Team: " \n "})
	if got != blank {
		t.Errorf("whitespace values: body = %q, want %q", got, blank)
	}
}
