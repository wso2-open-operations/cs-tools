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
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// fakeProjectSearch answers POST /projects/search as the entity service
// does: every project whose name or key contains the search query, ignoring
// case, one page of at most limit. It records each query.
type fakeProjectSearch struct {
	projects []searchedProject
	limit    int
	err      error
	reply    string
	queries  []string
}

// SearchProjects implements projectSearcher.
func (f *fakeProjectSearch) SearchProjects(_ context.Context, body []byte) ([]byte, error) {
	var req struct {
		SearchQuery string `json:"searchQuery"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	f.queries = append(f.queries, req.SearchQuery)
	if f.err != nil {
		return nil, f.err
	}
	if f.reply != "" {
		return []byte(f.reply), nil
	}
	q := strings.ToLower(req.SearchQuery)
	var out []searchedProject
	for _, p := range f.projects {
		if strings.Contains(strings.ToLower(p.Name), q) || strings.Contains(strings.ToLower(p.Key), q) {
			out = append(out, p)
		}
	}
	hasMore := false
	if f.limit > 0 && len(out) > f.limit {
		out, hasMore = out[:f.limit], true
	}
	return json.Marshal(map[string]any{"projects": out, "hasMore": hasMore})
}

// testProjects is the fixture's projects (made up): a key contained in
// another key, a name contained in another name, two projects of one name
// (and a third whose longer name contains it), and a project without a name.
var testProjects = []searchedProject{
	{ID: "j0", Key: "EXCORP-PROJ-1", Name: "Example Corp Production"},
	{ID: "j1", Key: "EXCORP-PROJ-10", Name: "Example Corp Production EU"},
	{ID: "j2", Key: "GEN01-PROJ1", Name: "Summit Retail Production"},
	{ID: "j3", Key: "QRTZ-PROJ1", Name: "Quartz Labs"},
	{ID: "j4", Key: "QRTZ-PROJ2", Name: "Quartz Labs"},
	{ID: "j6", Key: "QRTZ-STG", Name: "Quartz Labs Staging"},
	{ID: "j5", Key: "NONAME-PROJ1"},
}

// projectDashboard is a generated dashboard whose first widget filters on the
// given projects and whose second is a pie with one slice per sliceProjects
// entry, as the model would write them.
func projectDashboard(t *testing.T, projects []string, sliceProjects ...string) dashboard.Dashboard {
	t.Helper()
	q := func(names ...string) string {
		b, _ := json.Marshal(names)
		return `{"filters":[{"field":"state","op":"in","values":["open"]},{"field":"project","op":"in","values":` + string(b) + `}]}`
	}
	widgets := []string{`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + q(projects...) + `}`}
	if len(sliceProjects) > 0 {
		var slices []string
		for _, p := range sliceProjects {
			slices = append(slices, `{"label":"`+p+`","query":`+q(p)+`}`)
		}
		widgets = append(widgets, `{"id":"p","displayName":"P","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[`+strings.Join(slices, ",")+`]}`)
	}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// projectFilter is a query's filter on field ("project" or "projectId"),
// nil if it has none.
func projectFilter(query map[string]any, field string) map[string]any {
	for _, f := range query["filters"].([]any) {
		if e := f.(map[string]any); e["field"] == field {
			return e
		}
	}
	return nil
}

// TestWithDashboardProjects_ResolvesTheGeneratedDashboard: a project's key
// or name as the user wrote it becomes a projectId filter on its id, in a
// widget and in its slices; an exact key wins over a longer key containing
// it, an exact name over a longer name, and a value contained in one project
// only names that one; a project named twice, or by key and by name, is one
// id; each distinct value is searched once; everything else is as generated.
func TestWithDashboardProjects_ResolvesTheGeneratedDashboard(t *testing.T) {
	search := &fakeProjectSearch{projects: testProjects}
	inner := &fakeGenerator{result: GeneratedDashboard{
		Dashboard: projectDashboard(t, []string{"excorp-proj-1", "Example Corp Production", "EXCORP-PROJ-1", "Summit"}, "Example Corp Production EU", "noname-proj1"),
		Reply:     "Here it is.",
	}}
	got, err := WithDashboardProjects(inner, search).Generate(context.Background(), "example corp cases", json.RawMessage(`{"id":"x"}`))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inner.gotPrompt != "example corp cases" || string(inner.gotCurrent) != `{"id":"x"}` || got.Reply != "Here it is." {
		t.Errorf("inner got (%q, %s), reply %q", inner.gotPrompt, inner.gotCurrent, got.Reply)
	}
	w := got.Dashboard.Widgets[0].Query
	if projectFilter(w, "project") != nil {
		t.Errorf("a project filter is left: %v", w)
	}
	if f := projectFilter(w, "projectId"); f == nil || f["op"] != "in" || fmt.Sprint(f["values"]) != "[j0 j2]" {
		t.Errorf("widget projectId filter = %v, want in [j0 j2]", f)
	}
	for j, want := range []string{"[j1]", "[j5]"} {
		if f := projectFilter(got.Dashboard.Widgets[1].Slices[j].Query, "projectId"); f == nil || fmt.Sprint(f["values"]) != want {
			t.Errorf("slice %d projectId filter = %v, want %s", j, f, want)
		}
	}
	if q, _ := json.Marshal(w); !strings.Contains(string(q), `{"field":"state","op":"in","values":["open"]}`) || !strings.Contains(string(q), `"field":"type"`) {
		t.Errorf("other filters changed: %s", q)
	}
	if fmt.Sprint(search.queries) != "[excorp-proj-1 Example Corp Production Summit Example Corp Production EU noname-proj1]" {
		t.Errorf("searched %q, want each distinct value once", search.queries)
	}
}

// TestWithDashboardProjects_LeavesProjectsOut: a notIn project filter becomes
// a notIn projectId filter on the projects' ids, beside the dashboard's other
// filters; a value it cannot place is refused as for "in", never dropped (a
// filter leaving out nothing would count every project).
func TestWithDashboardProjects_LeavesProjectsOut(t *testing.T) {
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
		`{"field":"createdOn","op":"gte","values":["__startOfMonth:0__"]},{"field":"project","op":"notIn","values":["EXCORP-PROJ-1","Summit"]}]}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	got, err := WithDashboardProjects(&fakeGenerator{result: GeneratedDashboard{Dashboard: d}}, &fakeProjectSearch{projects: testProjects}).Generate(context.Background(), "x", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	q := got.Dashboard.Widgets[0].Query
	if f := projectFilter(q, "projectId"); f == nil || f["op"] != "notIn" || fmt.Sprint(f["values"]) != "[j0 j2]" {
		t.Errorf("projectId filter = %v, want notIn [j0 j2]", f)
	}
	if projectFilter(q, "project") != nil || projectFilter(q, "createdOn") == nil {
		t.Errorf("filters = %v, want the createdOn filter and no project filter", q["filters"])
	}

	bad, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"project","op":"notIn","values":["Orion"]}]}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	_, err = WithDashboardProjects(&fakeGenerator{result: GeneratedDashboard{Dashboard: bad}}, &fakeProjectSearch{projects: testProjects}).Generate(context.Background(), "x", nil)
	if !errors.Is(err, errDashboardProject) || dashboardTeamMessage(err) != `No project is named like "Orion".` {
		t.Errorf("err = %v, want the no-project reason", err)
	}
}

// TestWithDashboardProjects_RefusesAProjectItCannotPlace: a value no project
// contains, one that several contain, or a name two projects share, is an
// errDashboardProject whose message (as the chat shows it) says which,
// listing at most five candidates by name and key (key alone when a project
// has no name); a failed or unreadable search is an errDashboardProjectLookup.
func TestWithDashboardProjects_RefusesAProjectItCannotPlace(t *testing.T) {
	var many []searchedProject
	for i := range 7 {
		many = append(many, searchedProject{ID: fmt.Sprint("m", i), Key: fmt.Sprint("CLOUD-", i), Name: fmt.Sprint("Cloud ", i)})
	}
	for _, tc := range []struct {
		name, project string
		search        *fakeProjectSearch
		want          string
	}{
		{"no project", "Orion", &fakeProjectSearch{projects: testProjects}, `No project is named like "Orion".`},
		{"several projects", "PROJ1", &fakeProjectSearch{projects: testProjects},
			`"PROJ1" matches several projects (Summit Retail Production (GEN01-PROJ1), Quartz Labs (QRTZ-PROJ1), NONAME-PROJ1); name the one you mean.`},
		{"a name two projects share", "quartz labs", &fakeProjectSearch{projects: testProjects},
			`"quartz labs" matches several projects (Quartz Labs (QRTZ-PROJ1), Quartz Labs (QRTZ-PROJ2)); name the one you mean.`},
		{"a name that is its key", "LEGACY", &fakeProjectSearch{projects: []searchedProject{{ID: "l1", Key: "LEGACY-01", Name: "legacy-01"}, {ID: "l2", Key: "LEGACY-02", Name: "Legacy Two"}}},
			`"LEGACY" matches several projects (LEGACY-01, Legacy Two (LEGACY-02)); name the one you mean.`},
		{"more than five", "Cloud", &fakeProjectSearch{projects: many},
			`"Cloud" matches several projects (Cloud 0 (CLOUD-0), Cloud 1 (CLOUD-1), Cloud 2 (CLOUD-2), Cloud 3 (CLOUD-3), Cloud 4 (CLOUD-4) and 2 more); name the one you mean.`},
		{"more than one page", "Cloud", &fakeProjectSearch{projects: many, limit: 3},
			`"Cloud" matches several projects (Cloud 0 (CLOUD-0), Cloud 1 (CLOUD-1), Cloud 2 (CLOUD-2) and more); name the one you mean.`},
		{"one project on a page with more", "Cloud", &fakeProjectSearch{projects: many, limit: 1},
			`"Cloud" matches several projects (Cloud 0 (CLOUD-0) and more); name the one you mean.`},
		{"an empty name", " ", &fakeProjectSearch{projects: testProjects}, "A project filter has an empty name."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gen := WithDashboardProjects(&fakeGenerator{result: GeneratedDashboard{Dashboard: projectDashboard(t, []string{tc.project})}}, tc.search)
			_, err := gen.Generate(context.Background(), "x", nil)
			if !errors.Is(err, errDashboardProject) {
				t.Fatalf("err = %v, want errDashboardProject", err)
			}
			if msg := dashboardTeamMessage(err); msg != tc.want {
				t.Errorf("message = %q, want %q", msg, tc.want)
			}
		})
	}

	for name, search := range map[string]*fakeProjectSearch{
		"a failed search":     {err: errors.New("status 500")},
		"an unreadable reply": {reply: "<html>"},
	} {
		gen := WithDashboardProjects(&fakeGenerator{result: GeneratedDashboard{Dashboard: projectDashboard(t, []string{"Summit"})}}, search)
		if _, err := gen.Generate(context.Background(), "x", nil); !errors.Is(err, errDashboardProjectLookup) {
			t.Errorf("%s: err = %v, want errDashboardProjectLookup", name, err)
		}
	}
}

// TestWithDashboardProjects_LeavesOtherDashboardsAlone: a dashboard with no
// project filter (here grouped by project) is not searched for, and a
// generator error passes through.
func TestWithDashboardProjects_LeavesOtherDashboardsAlone(t *testing.T) {
	search := &fakeProjectSearch{projects: testProjects}
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"D","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"bar","gridWidth":6,"query":{},"groupBy":{"field":"project","maxGroups":10}}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	got, err := WithDashboardProjects(&fakeGenerator{result: GeneratedDashboard{Dashboard: d}}, search).Generate(context.Background(), "x", nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(search.queries) != 0 {
		t.Errorf("searched %q for a dashboard without a project filter", search.queries)
	}
	if g := got.Dashboard.Widgets[0].GroupBy; g == nil || g.Field != "project" {
		t.Errorf("groupBy = %+v, want project", g)
	}
	if _, err := WithDashboardProjects(&fakeGenerator{err: errGenerateRefused}, search).Generate(context.Background(), "x", nil); !errors.Is(err, errGenerateRefused) {
		t.Errorf("err = %v, want errGenerateRefused", err)
	}
}
