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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
)

// A small registry with group ids (32-hex, as configured): two CRE ABT teams,
// a plain CRE team, an SRE ABT team, a CRE team with no group configured, and
// a team of another family.
const testDashboardTeamRegistry = "rigel|Rigel|CRE-ABT|391737f405882f9f9bddb18d39f83d9a," +
	"sirius|Sirius|CRE-ABT|542b0b7de52cb6e9a210d06e57990c7d," +
	"americas|Americas|CRE|6341878aeb9bac6e73a1d104b2ab001f," +
	"apollo|Apollo|SRE-ABT||f9e9df561215e316c6f2ce9180c0af64," +
	"orion|Orion|CRE-ABT," +
	"experts|Experts|SME"

const (
	rigelGroup  = "391737f4-0588-2f9f-9bdd-b18d39f83d9a"
	siriusGroup = "542b0b7d-e52c-b6e9-a210-d06e57990c7d"
	apolloGroup = "f9e9df56-1215-e316-c6f2-ce9180c0af64"
)

func testDashboardTeams(t *testing.T) dashboardTeams {
	t.Helper()
	teams, err := directory.ParseTeamRegistry(testDashboardTeamRegistry)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := directory.New(teams, nil)
	if err != nil {
		t.Fatal(err)
	}
	return newDashboardTeams(dir.TeamResults())
}

// teamDashboard is a one-widget case dashboard whose widget query is query.
func teamDashboard(query string) []byte {
	return []byte(`{"displayName":"Rigel: Open Cases","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,` +
		`"widgets":[{"id":"open","displayName":"Open","resourceType":"case","shape":"count","gridWidth":3,"query":` + query + `}]}`)
}

// firstFilterValues is widgets[0].query.filters[0].values of a resolved definition.
func firstFilterValues(t *testing.T, def []byte) []any {
	t.Helper()
	var d struct {
		Widgets []struct {
			Query struct {
				Filters []struct {
					Values []any `json:"values"`
				} `json:"filters"`
			} `json:"query"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(def, &d); err != nil {
		t.Fatalf("resolved definition is not JSON: %v\n%s", err, def)
	}
	return d.Widgets[0].Query.Filters[0].Values
}

func TestDashboardTeams_SwapsKeysAndNamesForGroupIDs(t *testing.T) {
	teams := testDashboardTeams(t)
	cases := map[string]struct {
		filter string
		want   []any
	}{
		"key":                 {`{"field":"creTeam","op":"in","values":["rigel"]}`, []any{rigelGroup}},
		"key in another case": {`{"field":"creTeam","op":"in","values":["RIGEL"]}`, []any{rigelGroup}},
		"display name":        {`{"field":"creTeam","op":"in","values":["Rigel"]}`, []any{rigelGroup}},
		"several teams":       {`{"field":"creTeam","op":"in","values":["rigel","sirius"]}`, []any{rigelGroup, siriusGroup}},
		"plain CRE team":      {`{"field":"creTeam","op":"in","values":["americas"]}`, []any{"6341878a-eb9b-ac6e-73a1-d104b2ab001f"}},
		"SRE team in sreTeam": {`{"field":"sreTeam","op":"in","values":["apollo"]}`, []any{apolloGroup}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := teams.resolve(teamDashboard(`{"filters":[` + tc.filter + `,{"field":"state","op":"notIn","values":["closed"]}]}`))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if vals := firstFilterValues(t, got); !equalValues(vals, tc.want) {
				t.Errorf("values = %v, want %v", vals, tc.want)
			}
			// The rest of the definition is untouched, and it still parses.
			if !strings.Contains(string(got), `"field":"state","op":"notIn","values":["closed"]`) {
				t.Errorf("other filters changed: %s", got)
			}
			if _, err := dashboard.ParseGenerated(got); err != nil {
				t.Errorf("resolved definition does not parse: %v", err)
			}
		})
	}
}

func equalValues(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDashboardTeams_ResolvesPieSlices(t *testing.T) {
	teams := testDashboardTeams(t)
	def := []byte(`{"displayName":"Teams","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"split","displayName":"Split","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[` +
		`{"label":"Rigel","query":{"filters":[{"field":"creTeam","op":"in","values":["rigel"]}]}},` +
		`{"label":"Sirius","query":{"filters":[{"field":"creTeam","op":"in","values":["sirius"]}]}}]}]}`)
	got, err := teams.resolve(def)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(string(got), rigelGroup) || !strings.Contains(string(got), siriusGroup) || strings.Contains(string(got), `"rigel"`) {
		t.Errorf("slices not resolved: %s", got)
	}
}

func TestDashboardTeams_LeavesWhatIsAlreadyResolved(t *testing.T) {
	teams := testDashboardTeams(t)
	cases := map[string]string{
		"no team filter":               `{"filters":[{"field":"state","op":"notIn","values":["closed"]}]}`,
		"my team":                      `{"filters":[{"field":"creTeam","op":"in","values":["__current_team__"]}]}`,
		"a group id (draft sent back)": `{"filters":[{"field":"creTeam","op":"in","values":["` + rigelGroup + `"]}]}`,
		"an upper-case group id":       `{"filters":[{"field":"creTeam","op":"in","values":["` + strings.ToUpper(rigelGroup) + `"]}]}`,
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			def := teamDashboard(query)
			got, err := teams.resolve(def)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if name == "an upper-case group id" {
				if vals := firstFilterValues(t, got); !equalValues(vals, []any{rigelGroup}) {
					t.Errorf("values = %v, want the lower-case id", vals)
				}
				return
			}
			if string(got) != string(def) {
				t.Errorf("definition changed:\n got %s\nwant %s", got, def)
			}
		})
	}
}

func TestDashboardTeams_RefusesUnusableTeamFilters(t *testing.T) {
	teams := testDashboardTeams(t)
	cases := map[string]struct {
		query string
		want  string
	}{
		"unknown team": {`{"filters":[{"field":"creTeam","op":"in","values":["vega"]}]}`,
			`unknown team "vega" (known teams: americas, apollo, orion, rigel, sirius)`},
		"CRE team in sreTeam": {`{"filters":[{"field":"sreTeam","op":"in","values":["rigel"]}]}`,
			"Rigel is a cre-abt team: filter it with creTeam, not sreTeam"},
		"SRE team in creTeam": {`{"filters":[{"field":"creTeam","op":"in","values":["apollo"]}]}`,
			"Apollo is a sre-abt team: filter it with sreTeam, not creTeam"},
		"no group configured": {`{"filters":[{"field":"creTeam","op":"in","values":["orion"]}]}`,
			"team Orion has no group configured"},
		"another family": {`{"filters":[{"field":"creTeam","op":"in","values":["experts"]}]}`,
			`unknown team "experts"`},
		"notIn": {`{"filters":[{"field":"creTeam","op":"notIn","values":["rigel"]}]}`,
			`a creTeam filter must use op "in"`},
		"no values": {`{"filters":[{"field":"creTeam","op":"in","values":[]}]}`,
			"a creTeam filter needs at least one team"},
		"in an anyOf branch": {`{"filters":[],"anyOf":[{"filters":[{"field":"creTeam","op":"in","values":["rigel"]}]}]}`,
			"a creTeam filter must be in the top-level filters, not an anyOf branch"},
		"another team's group id in the wrong field": {`{"filters":[{"field":"sreTeam","op":"in","values":["` + rigelGroup + `"]}]}`,
			"Rigel is a cre-abt team"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := teams.resolve(teamDashboard(tc.query))
			if !errors.Is(err, errDashboardTeam) {
				t.Fatalf("err = %v, want errDashboardTeam", err)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "widgets[0]") {
				t.Errorf("err = %q, want it to name widgets[0] and say %q", err.Error(), tc.want)
			}
		})
	}
}

func TestDashboardTeams_LeavesNonDashboardsForParseGenerated(t *testing.T) {
	teams := testDashboardTeams(t)
	for _, raw := range []string{`not json`, `[1,2]`, `{"widgets":"x"}`} {
		got, err := teams.resolve([]byte(raw))
		if err != nil || string(got) != raw {
			t.Errorf("resolve(%s) = %s, %v; want it unchanged", raw, got, err)
		}
	}
}

func TestDashboardTeams_ListsCREAndSRETeamsOnly(t *testing.T) {
	teams := testDashboardTeams(t)
	var got []string
	for _, team := range teams.list {
		got = append(got, team.Key+":"+team.Field)
	}
	want := []string{"americas:creTeam", "apollo:sreTeam", "orion:creTeam", "rigel:creTeam", "sirius:creTeam"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("teams = %v, want %v", got, want)
	}
}

// teamFilterDashboard is a parsed one-widget dashboard (plus a pie slice)
// filtering on team, as a generator returns it.
func teamFilterDashboard(t *testing.T, team string) dashboard.Dashboard {
	t.Helper()
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"Rigel: Open Cases","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"open","displayName":"Open","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
		`{"field":"creTeam","op":"in","values":["` + team + `"]},{"field":"state","op":"notIn","values":["closed"]},` +
		`{"field":"createdOn","op":"gte","values":["__daysAgo:30__"]}]}},` +
		`{"id":"split","displayName":"Split","resourceType":"case","shape":"pie","gridWidth":6,"query":{},"slices":[` +
		`{"label":"Rigel","query":{"filters":[{"field":"creTeam","op":"in","values":["` + team + `"]}]}}]}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

func testTeamResults(t *testing.T) []directory.TeamResult {
	t.Helper()
	teams, err := directory.ParseTeamRegistry(testDashboardTeamRegistry)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := directory.New(teams, nil)
	if err != nil {
		t.Fatal(err)
	}
	return dir.TeamResults()
}

func TestWithDashboardTeams_ResolvesTheGeneratedDashboard(t *testing.T) {
	inner := &fakeGenerator{result: GeneratedDashboard{Dashboard: teamFilterDashboard(t, "rigel"), Reply: "Here it is."}}
	gen := WithDashboardTeams(inner, testTeamResults(t))

	got, err := gen.Generate(context.Background(), "open rigel cases in the past month", json.RawMessage(`{"id":"x"}`))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inner.gotPrompt != "open rigel cases in the past month" || string(inner.gotCurrent) != `{"id":"x"}` {
		t.Errorf("inner got (%q, %s)", inner.gotPrompt, inner.gotCurrent)
	}
	widget, _ := json.Marshal(got.Dashboard.Widgets[0].Query)
	slice, _ := json.Marshal(got.Dashboard.Widgets[1].Slices[0].Query)
	for _, q := range []string{string(widget), string(slice)} {
		if !strings.Contains(q, rigelGroup) || strings.Contains(q, `"rigel"`) {
			t.Errorf("query not resolved: %s", q)
		}
	}
	// Everything else is as generated: dates, state, the implied type filter, the reply.
	if !strings.Contains(string(widget), "__daysAgo:30__") || !strings.Contains(string(widget), `"field":"type"`) || got.Reply != "Here it is." {
		t.Errorf("other parts changed: %s / %q", widget, got.Reply)
	}
}

func TestWithDashboardTeams_RefusesAnUnknownTeam(t *testing.T) {
	gen := WithDashboardTeams(&fakeGenerator{result: GeneratedDashboard{Dashboard: teamFilterDashboard(t, "vega")}}, testTeamResults(t))
	_, err := gen.Generate(context.Background(), "x", nil)
	if !errors.Is(err, errDashboardTeam) || !strings.Contains(err.Error(), `unknown team "vega"`) {
		t.Fatalf("err = %v, want an unknown-team errDashboardTeam", err)
	}
	if msg := dashboardTeamMessage(err); msg != `Unknown team "vega" (known teams: americas, apollo, orion, rigel, sirius).` {
		t.Errorf("message = %q", msg)
	}
}

func TestWithDashboardTeams_PassesGeneratorErrorsThrough(t *testing.T) {
	gen := WithDashboardTeams(&fakeGenerator{err: errGenerateRefused}, testTeamResults(t))
	if _, err := gen.Generate(context.Background(), "x", nil); !errors.Is(err, errGenerateRefused) {
		t.Fatalf("err = %v, want errGenerateRefused", err)
	}
}

func TestDashboardTeams_TurnsAnAccountTeamKeyIntoItsName(t *testing.T) {
	teams := testDashboardTeams(t)
	for in, want := range map[string]string{"rigel": "Rigel", "RIGEL": "Rigel", "Rigel": "Rigel", "apollo": "Apollo", "americas": "Americas"} {
		got, err := teams.resolve(teamDashboard(`{"abtTeam":"` + in + `","lifeCycle":"Customer"}`))
		if err != nil {
			t.Fatalf("%s: resolve: %v", in, err)
		}
		var d struct {
			Widgets []struct {
				Query map[string]any `json:"query"`
			} `json:"widgets"`
		}
		if err := json.Unmarshal(got, &d); err != nil {
			t.Fatal(err)
		}
		if q := d.Widgets[0].Query; q["abtTeam"] != want || q["lifeCycle"] != "Customer" {
			t.Errorf("%s: query = %v, want abtTeam %q", in, q, want)
		}
	}
	_, err := teams.resolve(teamDashboard(`{"abtTeam":"vega"}`))
	if !errors.Is(err, errDashboardTeam) || !strings.Contains(err.Error(), `unknown team "vega"`) {
		t.Fatalf("an unknown team: err = %v, want errDashboardTeam naming it", err)
	}
}

// timeCardTeamsDashboard is a generated time card bar per team over teams,
// and a pie whose one slice holds slice.
func timeCardTeamsDashboard(t *testing.T, teams, slice string) dashboard.Dashboard {
	t.Helper()
	d, err := dashboard.ParseGenerated([]byte(`{"displayName":"Hours by ABT","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` +
		`{"id":"by-abt","displayName":"Hours by ABT","resourceType":"time_card","shape":"bar","gridWidth":6,` +
		`"query":{"teamIds":` + teams + `,"startDate":"__startOfMonth:0__","endDate":"__today__"},"groupBy":{"field":"team","measure":"hours"}},` +
		`{"id":"split","displayName":"Split","resourceType":"time_card","shape":"pie","gridWidth":6,"query":{},"slices":[` +
		`{"label":"A","query":{"teamIds":` + slice + `}}]}]}`))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	return d
}

// TestDashboardTeams_ResolvesTimeCardTeams: a time card query's teamIds
// (keys, names, group ids already resolved, any case; CRE and SRE teams)
// become the group ids entity-service's teamIds matches (a CRE team's CRE
// group, an SRE team's SRE group), in the widget and its slices; the
// viewer's team placeholder and the rest of the query are left as they are.
func TestDashboardTeams_ResolvesTimeCardTeams(t *testing.T) {
	teams := testDashboardTeams(t)
	d := timeCardTeamsDashboard(t, `["rigel","Sirius","AMERICAS","`+strings.ToUpper(rigelGroup)+`","__current_team__","apollo"]`, `["Apollo"]`)
	if err := teams.resolveDashboard(&d); err != nil {
		t.Fatalf("resolveDashboard: %v", err)
	}
	want := []any{rigelGroup, siriusGroup, "6341878a-eb9b-ac6e-73a1-d104b2ab001f", rigelGroup, "__current_team__", apolloGroup}
	if got, _ := d.Widgets[0].Query["teamIds"].([]any); !equalValues(got, want) {
		t.Errorf("widget teamIds = %v, want %v", got, want)
	}
	if got, _ := d.Widgets[1].Slices[0].Query["teamIds"].([]any); !equalValues(got, []any{apolloGroup}) {
		t.Errorf("slice teamIds = %v, want [%s]", got, apolloGroup)
	}
	if d.Widgets[0].Query["startDate"] != "__startOfMonth:0__" || d.Widgets[0].GroupBy.Field != "team" {
		t.Errorf("other parts changed: %+v / %+v", d.Widgets[0].Query, d.Widgets[0].GroupBy)
	}

	// The same through the generator wrapper, from a definition's bytes too.
	def, err := teams.resolve([]byte(`{"widgets":[{"resourceType":"time_card","query":{"teamIds":["rigel"]}}]}`))
	if err != nil || !strings.Contains(string(def), rigelGroup) {
		t.Errorf("resolve = %s, %v; want rigel's group id", def, err)
	}
}

// TestDashboardTeams_RefusesTimeCardTeamsItCannotFind: an unknown team, a
// team of a family with no team filter, and a team with no group are
// refused with the reason, in a slice as in the widget.
func TestDashboardTeams_RefusesTimeCardTeamsItCannotFind(t *testing.T) {
	for _, c := range []struct{ name, teams, slice, want string }{
		{"unknown", `["vega"]`, `["rigel"]`, `widgets[0]: unknown team "vega" (known teams: americas, apollo, orion, rigel, sirius)`},
		{"no group", `["rigel","orion"]`, `["rigel"]`, "team Orion has no group configured, so its members' time cards cannot be found"},
		{"another family", `["experts"]`, `["rigel"]`, `unknown team "experts"`},
		{"in a slice", `["rigel"]`, `["vega"]`, `widgets[1].slices[0]: unknown team "vega"`},
	} {
		d := timeCardTeamsDashboard(t, c.teams, c.slice)
		err := testDashboardTeams(t).resolveDashboard(&d)
		if !errors.Is(err, errDashboardTeam) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want errDashboardTeam mentioning %q", c.name, err, c.want)
		}
	}
}

// TestDashboardTeams_RefusesANameTwoTeamsAnswerTo: two teams of one name (a
// CRE and an SRE "Rigel"), or one team's name another team's key, are
// refused with the teams it could be, in every place a team is named,
// rather than resolved to whichever the registry listed last. Each team's
// own key still resolves.
func TestDashboardTeams_RefusesANameTwoTeamsAnswerTo(t *testing.T) {
	teams := newDashboardTeams([]directory.TeamResult{
		{ID: "rigel", Name: "Rigel", Family: string(directory.FamilyCREAbt), CreGroupID: rigelGroup},
		{ID: "rigel-sre", Name: "Rigel", Family: string(directory.FamilySREAbt), SreGroupID: apolloGroup},
		{ID: "sirius", Name: "Vega", Family: string(directory.FamilyCREAbt), CreGroupID: siriusGroup},
		{ID: "vega", Name: "Vega Team", Family: string(directory.FamilyCREAbt), CreGroupID: "6341878a-eb9b-ac6e-73a1-d104b2ab001f"},
	})
	for _, c := range []struct{ name, def, want string }{
		{"a case team filter", string(teamDashboard(`{"filters":[{"field":"creTeam","op":"in","values":["RIGEL"]}]}`)), `"RIGEL" could be any of the teams rigel, rigel-sre`},
		{"an account's abtTeam", string(teamDashboard(`{"abtTeam":"Rigel"}`)), `"Rigel" could be any of the teams rigel, rigel-sre`},
		{"a time card teamIds", `{"widgets":[{"resourceType":"time_card","query":{"teamIds":["rigel"]}}]}`, `"rigel" could be any of the teams rigel, rigel-sre`},
		{"a name that is another team's key", `{"widgets":[{"resourceType":"time_card","query":{"teamIds":["vega"]}}]}`, `"vega" could be any of the teams sirius, vega`},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := teams.resolve([]byte(c.def))
			if !errors.Is(err, errDashboardTeam) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want errDashboardTeam containing %q", err, c.want)
			}
		})
	}

	def, err := teams.resolve([]byte(`{"widgets":[{"resourceType":"time_card","query":{"teamIds":["rigel-sre","Vega Team"]}}]}`))
	if err != nil || !strings.Contains(string(def), apolloGroup) || !strings.Contains(string(def), "6341878a-eb9b-ac6e-73a1-d104b2ab001f") {
		t.Errorf("unambiguous keys and names: resolve = %s, %v; want their group ids", def, err)
	}
}

// TestDashboardTeams_LeavesTeamIDsOfOtherResources: teamIds is a list of
// teams only on a time card widget; on any other resourceType it is left as
// it is (ParseGenerated decides whether that resource takes it).
func TestDashboardTeams_LeavesTeamIDsOfOtherResources(t *testing.T) {
	teams := testDashboardTeams(t)
	in := `{"widgets":[{"resourceType":"case","query":{"teamIds":["not-a-team"]},"slices":[{"query":{"teamIds":["rigel"]}}]}]}`
	def, err := teams.resolve([]byte(in))
	if err != nil || string(def) != in {
		t.Errorf("resolve = %s, %v; want the definition unchanged", def, err)
	}
}
