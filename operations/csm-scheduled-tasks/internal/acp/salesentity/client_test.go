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

package salesentity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/apierror"
)

// tokenServer issues a client-credentials token for every request.
func tokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeSalesEntity records every request body and method by path and answers
// from responses, a function of the path and the decoded body.
type fakeSalesEntity struct {
	mu        sync.Mutex
	requests  map[string][]map[string]any
	methods   map[string][]string
	responses func(path string, body map[string]any) (int, string)
}

func (f *fakeSalesEntity) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			t.Errorf("%s %s, want POST or PATCH", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want the bearer token", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("%s body is not JSON: %v", r.URL.Path, err)
		}
		f.mu.Lock()
		if f.requests == nil {
			f.requests = map[string][]map[string]any{}
			f.methods = map[string][]string{}
		}
		f.requests[r.URL.Path] = append(f.requests[r.URL.Path], body)
		f.methods[r.URL.Path] = append(f.methods[r.URL.Path], r.Method)
		f.mu.Unlock()
		status, resp := f.responses(r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	})
}

func (f *fakeSalesEntity) bodies(path string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[path]
}

func (f *fakeSalesEntity) methodsFor(path string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.methods[path]
}

func newTestClient(t *testing.T, f *fakeSalesEntity) *Client {
	t.Helper()
	api := httptest.NewServer(f.handler(t))
	t.Cleanup(api.Close)
	c, err := NewClient(Config{BaseURL: api.URL, TokenURL: tokenServer(t).URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClient_RejectsInsecureURLs(t *testing.T) {
	for _, cfg := range []Config{
		{BaseURL: "https://sales.example", TokenURL: "http://sales.example/token"},
		{BaseURL: "http://sales.example", TokenURL: "https://sales.example/token"},
	} {
		if _, err := NewClient(cfg); err == nil {
			t.Errorf("NewClient(%+v) error = nil, want an https error", cfg)
		}
	}
}

// TestTokenFetchRejectsRedirects: the token request carries the client
// secret, so a redirected token endpoint must never be followed.
func TestTokenFetchRejectsRedirects(t *testing.T) {
	var targetHit bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHit = true
		_, _ = w.Write([]byte(`{"access_token":"x","token_type":"bearer"}`))
	}))
	defer target.Close()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer tokenSrv.Close()

	c, err := NewClient(Config{BaseURL: "https://unused.invalid", TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.GetProject(context.Background(), "a0dxx"); err == nil {
		t.Fatal("err = nil, want an error for a redirected token request")
	}
	if targetHit {
		t.Error("redirect target was contacted")
	}
}

// TestRequestsRejectRedirects: a redirected API call is an error, and the
// bearer token never reaches the redirect target.
func TestRequestsRejectRedirects(t *testing.T) {
	var gotAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer target.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer api.Close()
	c, err := NewClient(Config{BaseURL: api.URL, TokenURL: tokenServer(t).URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.GetProject(context.Background(), "a0dxx")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("err = %v, want apierror.Error 302", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization leaked to the redirect target: %q", gotAuth)
	}
}

func TestUpstreamErrorAndOversizedBody(t *testing.T) {
	f := &fakeSalesEntity{responses: func(path string, _ map[string]any) (int, string) {
		if path == "/projects/search" {
			return http.StatusInternalServerError, `{"message":"boom"}`
		}
		return http.StatusOK, "[" + strings.Repeat(" ", maxResponseBody) + "]"
	}}
	c := newTestClient(t, f)

	_, err := c.GetProject(context.Background(), "a0dxx")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("GetProject err = %v, want apierror.Error 500", err)
	}
	if _, err := c.GetCustomer(context.Background(), "001xx"); err == nil {
		t.Error("GetCustomer with an oversized body: err = nil, want an error")
	}
}

func TestGetProject(t *testing.T) {
	f := &fakeSalesEntity{responses: func(_ string, body map[string]any) (int, string) {
		if body["id"] == "a0dmissing" {
			return http.StatusOK, `[]`
		}
		return http.StatusOK, `[{"id":"a0dE200000E7CxNIAV","key":"APPSUB","startDate":"2025-07-01","endDate":"2026-10-13","customerId":"001E200000tk3ZyIAI","status":"Open"}]`
	}}
	c := newTestClient(t, f)

	p, err := c.GetProject(context.Background(), "a0dE200000E7CxNIAV")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.ID != "a0dE200000E7CxNIAV" || deref(p.EndDate) != "2026-10-13" || deref(p.StartDate) != "2025-07-01" || deref(p.CustomerID) != "001E200000tk3ZyIAI" {
		t.Errorf("project = %+v", p)
	}
	body := f.bodies("/projects/search")[0]
	if body["id"] != "a0dE200000E7CxNIAV" || body["limit"] != float64(1) {
		t.Errorf("request = %v, want {id, limit: 1}", body)
	}

	if _, err := c.GetProject(context.Background(), "a0dmissing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing project: err = %v, want ErrNotFound", err)
	}
}

// TestGetCustomer: the account is read live from Salesforce (isRealTime);
// the database copy has no renewal manager and can be stale.
func TestGetCustomer(t *testing.T) {
	f := &fakeSalesEntity{responses: func(_ string, body map[string]any) (int, string) {
		if ids, _ := body["ids"].([]any); len(ids) == 1 && ids[0] == "001missing" {
			return http.StatusOK, `[]`
		}
		return http.StatusOK, `[{"id":"001E200000tk3ZyIAI","owner":{"name":"Kavith R","email":"kavithr@wso2.com"},"technicalOwner":"to@wso2.com",
			"renewalManager":{"name":"Abinash A","email":"abinash@wso2.com"},"accountClassification":"Partner","primaryPartnerAccountId":null,"isPartner":false}]`
	}}
	c := newTestClient(t, f)

	cu, err := c.GetCustomer(context.Background(), "001E200000tk3ZyIAI")
	if err != nil {
		t.Fatalf("GetCustomer: %v", err)
	}
	if deref(cu.Owner.Email) != "kavithr@wso2.com" || deref(cu.Owner.Name) != "Kavith R" || deref(cu.TechnicalOwner) != "to@wso2.com" ||
		deref(cu.RenewalManager.Email) != "abinash@wso2.com" || deref(cu.AccountClassification) != "Partner" || cu.PrimaryPartnerAccountID != nil {
		t.Errorf("customer = %+v", cu)
	}
	body := f.bodies("/customer-search")[0]
	if body["isRealTime"] != true || body["limit"] != float64(1) {
		t.Errorf("request = %v, want isRealTime true and limit 1", body)
	}
	if ids, _ := body["ids"].([]any); len(ids) != 1 || ids[0] != "001E200000tk3ZyIAI" {
		t.Errorf("ids = %v", body["ids"])
	}

	if _, err := c.GetCustomer(context.Background(), "001missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing customer: err = %v, want ErrNotFound", err)
	}
}

func TestProjectContacts(t *testing.T) {
	f := &fakeSalesEntity{responses: func(string, map[string]any) (int, string) {
		return http.StatusOK, `[{"id":"a0e1","email":"jane@bankco.com","state":"REGISTERED","roles":["Portal user","Business Contact"],"contact":{"name":"Jane B","email":"jane@bankco.com"}}]`
	}}
	c := newTestClient(t, f)

	got, err := c.ProjectContacts(context.Background(), "a0dxx")
	if err != nil {
		t.Fatalf("ProjectContacts: %v", err)
	}
	if len(got) != 1 || got[0].Email != "jane@bankco.com" || deref(got[0].State) != "REGISTERED" || len(got[0].Roles) != 2 || deref(got[0].Contact.Name) != "Jane B" {
		t.Errorf("contacts = %+v", got)
	}
	body := f.bodies("/project-contacts/search")[0]
	if body["projectId"] != "a0dxx" || body["fetchAll"] != true {
		t.Errorf("request = %v, want {projectId, fetchAll: true}", body)
	}
}

// TestPagedSearches: account contacts, linked opportunities and invoices are
// read page by page until a short page.
func TestPagedSearches(t *testing.T) {
	page := func(n int, item string) string {
		items := make([]string, n)
		for i := range items {
			items[i] = item
		}
		return "[" + strings.Join(items, ",") + "]"
	}
	f := &fakeSalesEntity{responses: func(path string, body map[string]any) (int, string) {
		full := body["offset"] == float64(0)
		n := 3
		if full {
			n = pageSize
		}
		switch path {
		case "/contacts/search":
			return http.StatusOK, page(n, `{"name":"A","email":"a@x.com","isPrimaryContact":true}`)
		case "/linked-opportunities/search":
			return http.StatusOK, page(n, `{"id":"l1","opportunityId":"006xx"}`)
		case "/invoices/search":
			return http.StatusOK, page(n, `{"id":"a0Ixx","name":"INV-1","opportunityId":"006xx","dueDate":"2026-10-15","paidDate":null,"invoiceDate":"2026-10-01","classification":"SUB"}`)
		}
		return http.StatusNotFound, `{}`
	}}
	c := newTestClient(t, f)
	ctx := context.Background()

	contacts, err := c.AccountContacts(ctx, "001xx")
	if err != nil || len(contacts) != pageSize+3 || !contacts[0].IsPrimaryContact || deref(contacts[0].Email) != "a@x.com" {
		t.Errorf("AccountContacts = %d, %v", len(contacts), err)
	}
	links, err := c.LinkedOpportunities(ctx, "a0dxx")
	if err != nil || len(links) != pageSize+3 || deref(links[0].OpportunityID) != "006xx" {
		t.Errorf("LinkedOpportunities = %d, %v", len(links), err)
	}
	invoices, err := c.Invoices(ctx, "006xx")
	if err != nil || len(invoices) != pageSize+3 || deref(invoices[0].DueDate) != "2026-10-15" || invoices[0].PaidDate != nil || deref(invoices[0].Classification) != "SUB" {
		t.Errorf("Invoices = %d, %v", len(invoices), err)
	}

	for path, filter := range map[string]string{"/contacts/search": "accountId", "/linked-opportunities/search": "projectId", "/invoices/search": "opportunityId"} {
		bodies := f.bodies(path)
		if len(bodies) != 2 {
			t.Errorf("%s pages = %d, want 2", path, len(bodies))
			continue
		}
		if bodies[0][filter] == nil || bodies[1]["offset"] != float64(pageSize) || bodies[0]["limit"] != float64(pageSize) {
			t.Errorf("%s requests = %v", path, bodies)
		}
	}
}

// TestPagedSearchStopsAtThePageCap: an upstream that keeps sending full
// pages is an error, not an endless loop.
func TestPagedSearchStopsAtThePageCap(t *testing.T) {
	full := "[" + strings.TrimSuffix(strings.Repeat(`{"id":"l1","opportunityId":"006xx"},`, pageSize), ",") + "]"
	f := &fakeSalesEntity{responses: func(string, map[string]any) (int, string) { return http.StatusOK, full }}
	c := newTestClient(t, f)
	if _, err := c.LinkedOpportunities(context.Background(), "a0dxx"); err == nil {
		t.Fatal("err = nil, want an error after the page cap")
	}
	if n := len(f.bodies("/linked-opportunities/search")); n != maxPages {
		t.Errorf("pages requested = %d, want %d", n, maxPages)
	}
}

func TestOpportunities(t *testing.T) {
	f := &fakeSalesEntity{responses: func(_ string, body map[string]any) (int, string) {
		return http.StatusOK, `[{"id":"006a","name":"BankCo renewal","stageName":"50 - Closed Won","eulaVersion":"EULA 3.4"}]`
	}}
	c := newTestClient(t, f)

	got, err := c.Opportunities(context.Background(), []string{"006a"})
	if err != nil || len(got) != 1 || deref(got[0].StageName) != "50 - Closed Won" || deref(got[0].EulaVersion) != "EULA 3.4" || deref(got[0].Name) != "BankCo renewal" {
		t.Fatalf("Opportunities = %+v, %v", got, err)
	}
	body := f.bodies("/opportunities/search")[0]
	if ids, _ := body["ids"].([]any); len(ids) != 1 || ids[0] != "006a" {
		t.Errorf("ids = %v", body["ids"])
	}

	if got, err := c.Opportunities(context.Background(), nil); err != nil || got != nil {
		t.Errorf("no ids: %v, %v; want no call", got, err)
	}
}

// TestUpdateProjectStatus: the status goes to PATCH /projects/{id}, the
// endpoint that writes the project's State__c; an upstream error is returned.
func TestUpdateProjectStatus(t *testing.T) {
	f := &fakeSalesEntity{responses: func(path string, _ map[string]any) (int, string) {
		if path == "/projects/a0dmissing" {
			return http.StatusNotFound, `{"message":"not found"}`
		}
		return http.StatusOK, `{"id":"a0dE200000E7CxNIAV","status":"Suspended"}`
	}}
	c := newTestClient(t, f)

	if err := c.UpdateProjectStatus(context.Background(), "a0dE200000E7CxNIAV", "Suspended"); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}
	if got := f.methodsFor("/projects/a0dE200000E7CxNIAV"); len(got) != 1 || got[0] != http.MethodPatch {
		t.Errorf("methods = %v, want one PATCH", got)
	}
	body := f.bodies("/projects/a0dE200000E7CxNIAV")[0]
	if len(body) != 1 || body["status"] != "Suspended" {
		t.Errorf("request = %v, want {status: Suspended}", body)
	}

	err := c.UpdateProjectStatus(context.Background(), "a0dmissing", "Suspended")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("missing project: err = %v, want a 404 *apierror.Error", err)
	}
}

// TestUpdateProjectStatus_EscapesTheID: an id is one path segment, never a
// path of its own.
func TestUpdateProjectStatus_EscapesTheID(t *testing.T) {
	var gotPath string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(api.Close)
	c, err := NewClient(Config{BaseURL: api.URL, TokenURL: tokenServer(t).URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := c.UpdateProjectStatus(context.Background(), "../customer-search", "Suspended"); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}
	if gotPath != "/projects/..%2Fcustomer-search" {
		t.Errorf("path = %q, want the id escaped into one segment", gotPath)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
