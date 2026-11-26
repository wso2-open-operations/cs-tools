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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	plgdomain "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// resolvedFiltersFixture is a handler over made-up products, projects,
// accounts, teams and regions, with the fakes it searches.
type resolvedFiltersFixture struct {
	handler  *DashboardResolvedFiltersHandler
	products *fakeProductSearch
	projects *fakeProjectSearch
	accounts *fakeAccountSearch
	cases    *fakeCaseAggregate
}

// testRegions is the cases aggregate by region the fixture's regions come from.
const testRegions = `{"groups":[{"key":"apac","label":"APAC","count":4},{"key":"emea","label":"EMEA","count":2}],"othersCount":0,"totalRecords":6}`

func newResolvedFiltersFixture() resolvedFiltersFixture {
	f := resolvedFiltersFixture{
		products: &fakeProductSearch{names: testProducts, versions: map[string][]string{"p0": {"3.2.0", "4.2.0", "4.3.0"}}},
		projects: &fakeProjectSearch{projects: []searchedProject{
			{ID: "proj-1", Key: "EXCORP-PROJ-1", Name: "Example Corp Production"},
			{ID: "proj-2", Key: "EXCORP-PROJ-2", Name: "Example Corp Staging"},
		}},
		accounts: &fakeAccountSearch{accounts: []searchedAccount{
			{ID: "acc-1", Number: "ACC-0001", Name: "Example Corp"},
			{ID: "acc-2", Number: "ACC-0002", Name: "Twin Labs"},
			{ID: "acc-3", Number: "ACC-0003", Name: "Twin Labs"},
		}},
		cases: &fakeCaseAggregate{reply: testRegions},
	}
	f.handler = NewDashboardResolvedFiltersHandler(f.products, f.projects, f.accounts, []directory.TeamResult{
		{ID: "rigel", Name: "Rigel", Family: string(directory.FamilyCREAbt), CreGroupID: rigelGroup},
		{ID: "apollo", Name: "Apollo", Family: string(directory.FamilySREAbt), SreGroupID: apolloGroup},
	}).WithRegions(f.cases)
	return f
}

// post sends body to the handler as a signed-in user.
func (f resolvedFiltersFixture) post(body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dashboards/resolved-filters", strings.NewReader(body))
	f.handler.ResolveFilters(w, withUser(r))
	return w
}

func TestResolveFilters_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/dashboards/resolved-filters", strings.NewReader(`{"filters":[]}`))
	newResolvedFiltersFixture().handler.ResolveFilters(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// Every named kind becomes what the case search takes, with its op, and every
// other filter comes back as it was sent.
func TestResolveFilters_ResolvesEveryNamedKind(t *testing.T) {
	f := newResolvedFiltersFixture()
	w := f.post(`{"filters":[
		{"field":"state","op":"notIn","values":["closed"]},
		{"field":"product","op":"in","values":["API Manager"]},
		{"field":"productVersion","op":"in","values":["4.x"]},
		{"field":"project","op":"notIn","values":["example corp production"]},
		{"field":"account","op":"in","values":["ACC-0001"]},
		{"field":"creTeam","op":"in","values":["Rigel"]},
		{"field":"sreTeam","op":"in","values":["apollo"]},
		{"field":"assignedUserId","op":"isEmpty"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	var got resolvedFiltersBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var want resolvedFiltersBody
	_ = json.Unmarshal([]byte(`{"filters":[
		{"field":"state","op":"notIn","values":["closed"]},
		{"field":"product","op":"in","values":["WSO2 API Manager"]},
		{"field":"productVersion","op":"in","values":["4.2.0","4.3.0"]},
		{"field":"projectId","op":"notIn","values":["proj-1"]},
		{"field":"accountId","op":"in","values":["acc-1"]},
		{"field":"creTeam","op":"in","values":["`+rigelGroup+`"]},
		{"field":"sreTeam","op":"in","values":["`+apolloGroup+`"]},
		{"field":"assignedUserId","op":"isEmpty"}]}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filters =\n%v\nwant\n%v", got.Filters, want.Filters)
	}
}

// A PLG call's playbook task and answers are checked against the playbooks
// and come back as sent; an unknown task or answer is the generate path's 422
// with its reason, and so is any task when the playbooks cannot be read.
func TestResolveFilters_ChecksThePlaybookCodes(t *testing.T) {
	f := newResolvedFiltersFixture()
	f.handler.WithPlaybooks(func(context.Context) ([]plgdomain.Playbook, error) { return testPlaybooks, nil })
	for _, body := range []string{
		`{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"}`,
		`{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["MONITOR","EXCLUDED"]}`,
	} {
		w := f.post(body)
		if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != body {
			t.Errorf("%s: status %d, body %s, want 200 with the same body", body, w.Code, w.Body)
		}
	}
	for _, c := range []struct{ body, want string }{
		{`{"playbookTask":"OUTCOME"}`, `Unknown playbook task "OUTCOME" (known tasks: `},
		{`{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":["Monitor"]}`, `"Monitor" is not an answer to ELIGIBILITY_ASSESSMENT_OUTCOME`},
	} {
		w := f.post(c.body)
		var body errorBody
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != http.StatusUnprocessableEntity || body.ErrorCode != errCodeGeneratePlaybook || !strings.Contains(body.Message, c.want) {
			t.Errorf("%s: status %d body %+v, want 422 %s containing %q", c.body, w.Code, body, errCodeGeneratePlaybook, c.want)
		}
	}

	f.handler.WithPlaybooks(func(context.Context) ([]plgdomain.Playbook, error) { return nil, errors.New("entity-service down") })
	w := f.post(`{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME"}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "no playbook task can be filtered by now") {
		t.Errorf("playbooks unavailable: status %d body %s, want 422 saying no task can be filtered", w.Code, w.Body)
	}
}

// A time card team list becomes the teams' group ids (a CRE team's CRE group,
// an SRE team's SRE group), by key or name, beside filters or alone; only
// what was sent comes back.
func TestResolveFilters_ResolvesATimeCardTeamList(t *testing.T) {
	f := newResolvedFiltersFixture()
	w := f.post(`{"teamIds":["Rigel","apollo"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body)
	}
	if want := `{"teamIds":["` + rigelGroup + `","` + apolloGroup + `"]}`; strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("body = %s, want %s", w.Body, want)
	}
	w = f.post(`{"teamIds":["rigel"],"filters":[{"field":"creTeam","op":"in","values":["rigel"]}]}`)
	want := `{"filters":[{"field":"creTeam","op":"in","values":["` + rigelGroup + `"]}],"teamIds":["` + rigelGroup + `"]}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("status %d, body = %s, want %s", w.Code, w.Body, want)
	}
	if len(f.products.queries)+len(f.projects.queries)+len(f.accounts.queries) != 0 {
		t.Errorf("a team list was searched for products, projects or accounts")
	}
}

// A team that is not in the registry is the generate route's 422.
func TestResolveFilters_UnknownTimeCardTeam(t *testing.T) {
	w := newResolvedFiltersFixture().post(`{"teamIds":["orion"]}`)
	var body errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusUnprocessableEntity || body.ErrorCode != errCodeGenerateTeam || !strings.Contains(body.Message, "orion") {
		t.Errorf("status %d body %+v, want 422 %s naming orion", w.Code, body, errCodeGenerateTeam)
	}
}

// A list without a named field is returned as it is, and nothing is searched.
func TestResolveFilters_NoNamesNoSearch(t *testing.T) {
	f := newResolvedFiltersFixture()
	body := `{"filters":[{"field":"state","op":"in","values":["open"]}]}`
	w := f.post(body)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != body {
		t.Fatalf("status %d, body %s, want 200 with %s", w.Code, w.Body, body)
	}
	if len(f.products.queries)+len(f.projects.queries)+len(f.accounts.queries)+len(f.cases.bodies) != 0 {
		t.Errorf("searched: products %v, projects %v, accounts %v, regions %v", f.products.queries, f.projects.queries, f.accounts.queries, f.cases.bodies)
	}
}

// A region is written as the account record writes it, each once, in an
// in or a notIn, and the regions are read once however many filters name one.
func TestResolveFilters_ResolvesRegions(t *testing.T) {
	f := newResolvedFiltersFixture()
	w := f.post(`{"filters":[{"field":"region","op":"in","values":["apac"," APAC ","Emea"]},{"field":"region","op":"notIn","values":["emea"]},{"field":"state","op":"in","values":["open"]}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body)
	}
	var got resolvedFiltersBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"field": "region", "op": "in", "values": []any{"APAC", "EMEA"}},
		map[string]any{"field": "region", "op": "notIn", "values": []any{"EMEA"}},
		map[string]any{"field": "state", "op": "in", "values": []any{"open"}},
	}
	if !reflect.DeepEqual(got.Filters, want) {
		t.Errorf("filters = %v, want %v", got.Filters, want)
	}
	if len(f.cases.bodies) != 1 {
		t.Errorf("regions read %d times, want once", len(f.cases.bodies))
	}
}

func TestResolveFilters_BadRequests(t *testing.T) {
	many := strings.TrimSuffix(strings.Repeat(`{"field":"state","op":"in","values":["open"]},`, maxResolvedFilters+1), ",")
	for _, c := range []struct{ name, body, want string }{
		{"no filters, teams or task", `{}`, "filters, teamIds or playbookTask is required"},
		{"answers without a task", `{"playbookAnswers":["MONITOR"]}`, "playbookAnswers needs a playbookTask"},
		{"no answers", `{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":[]}`, "playbookAnswers needs 1 to 100 answer codes"},
		{"an answer not a string", `{"playbookTask":"ELIGIBILITY_ASSESSMENT_OUTCOME","playbookAnswers":[1]}`, "playbookAnswers[0] is not an answer code"},
		{"no teams", `{"teamIds":[]}`, "teamIds needs 1 to 100 teams"},
		{"too many teams", `{"teamIds":[` + strings.TrimSuffix(strings.Repeat(`"rigel",`, maxResolvedTeamIDs+1), ",") + `]}`, "teamIds needs 1 to 100 teams"},
		{"a team not a string", `{"teamIds":[7]}`, "teamIds[0] is not a team key or name"},
		{"an empty team", `{"teamIds":["rigel",""]}`, "teamIds[1] is not a team key or name"},
		{"my team", `{"teamIds":["__current_team__"]}`, "__current_team__ is not resolved here"},
		{"bad filters beside teams", `{"teamIds":["rigel"],"filters":[{"field":"project"}]}`, "filters[0] needs a field and an op"},
		{"not JSON", `{"filters":`, ""},
		{"too many", `{"filters":[` + many + `]}`, "at most 50 filters"},
		{"not an object", `{"filters":["project"]}`, "filters[0] is not an object"},
		{"no op", `{"filters":[{"field":"project","values":["X"]}]}`, "filters[0] needs a field and an op"},
		{"an op the field does not take", `{"filters":[{"field":"project","op":"eq","values":["X"]}]}`, `a project filter does not take op "eq"`},
		{"a version notIn", `{"filters":[{"field":"product","op":"in","values":["API Manager"]},{"field":"productVersion","op":"notIn","values":["4.x"]}]}`, `does not take op "notIn"`},
		{"no values", `{"filters":[{"field":"account","op":"in","values":[]}]}`, "needs at least one value"},
		{"a value not a string", `{"filters":[{"field":"creTeam","op":"in","values":[7]}]}`, "values are strings"},
		{"a region eq", `{"filters":[{"field":"region","op":"eq","values":["APAC"]}]}`, `a region filter does not take op "eq"`},
		{"no regions", `{"filters":[{"field":"region","op":"in","values":[]}]}`, "needs at least one value"},
		{"a version alone", `{"filters":[{"field":"productVersion","op":"in","values":["4.x"]}]}`, "exactly one product"},
		{"a version beside two products", `{"filters":[{"field":"product","op":"in","values":["API Manager","Identity Hub"]},{"field":"productVersion","op":"in","values":["4.x"]}]}`, "exactly one product"},
		{"a version beside a product left out", `{"filters":[{"field":"product","op":"notIn","values":["API Manager"]},{"field":"productVersion","op":"in","values":["4.x"]}]}`, "exactly one product"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newResolvedFiltersFixture()
			w := f.post(c.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body)
			}
			var body errorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.Message, c.want) {
				t.Errorf("message = %q, want it to contain %q", body.Message, c.want)
			}
			if len(f.products.queries)+len(f.projects.queries)+len(f.accounts.queries)+len(f.cases.bodies) != 0 {
				t.Errorf("a refused request was searched")
			}
		})
	}
}

// A name that matches nothing, or several records, is a 422 with the reason
// and the generate route's code; a failed lookup is a 502.
func TestResolveFilters_UnusableNamesAndFailedLookups(t *testing.T) {
	for _, c := range []struct {
		name, filter string
		break_       func(f resolvedFiltersFixture)
		status       int
		code, want   string
	}{
		{"no such project", `{"field":"project","op":"in","values":["No Such Project"]}`, nil, 422, errCodeGenerateProject, "No Such Project"},
		{"two accounts of one name", `{"field":"account","op":"in","values":["Twin Labs"]}`, nil, 422, errCodeGenerateAccount, "Twin Labs"},
		{"no such product", `{"field":"product","op":"in","values":["Nothing Like It"]}`, nil, 422, errCodeGenerateProduct, "Nothing Like It"},
		{"no such team", `{"field":"creTeam","op":"in","values":["orion"]}`, nil, 422, errCodeGenerateTeam, "orion"},
		{"a CRE team as an SRE team", `{"field":"sreTeam","op":"in","values":["rigel"]}`, nil, 422, errCodeGenerateTeam, "Rigel is a cre-abt team"},
		{"the project search fails", `{"field":"project","op":"in","values":["EXCORP-PROJ-1"]}`, func(f resolvedFiltersFixture) { f.projects.err = errors.New("down") }, 502, errCodeGenerateProjectLookup, errMsgGenerateProjectLookup},
		{"the account search fails", `{"field":"account","op":"in","values":["Example Corp"]}`, func(f resolvedFiltersFixture) { f.accounts.err = errors.New("down") }, 502, errCodeGenerateAccountLookup, errMsgGenerateAccountLookup},
		{"the product search fails", `{"field":"product","op":"in","values":["API Manager"]}`, func(f resolvedFiltersFixture) { f.products.err = errors.New("down") }, 502, errCodeGenerateProductLookup, errMsgGenerateProductLookup},
		{"no such region", `{"field":"region","op":"notIn","values":["Mars"]}`, nil, 422, errCodeGenerateRegion, `the region "Mars"; the regions are APAC, EMEA`},
		{"the region lookup fails", `{"field":"region","op":"in","values":["APAC"]}`, func(f resolvedFiltersFixture) { f.cases.err = errors.New("down") }, 502, errCodeGenerateRegionLookup, errMsgGenerateRegionLookup},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newResolvedFiltersFixture()
			if c.break_ != nil {
				c.break_(f)
			}
			w := f.post(`{"filters":[` + c.filter + `]}`)
			if w.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, c.status, w.Body)
			}
			var body errorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.ErrorCode != c.code || !strings.Contains(body.Message, c.want) {
				t.Errorf("body = %+v, want code %q and a message containing %q", body, c.code, c.want)
			}
			if strings.Contains(body.Message, "filters") {
				t.Errorf("message %q leaks the request path", body.Message)
			}
		})
	}
}
