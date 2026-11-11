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

package dashboard

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// stateEnteredDashboard wraps widgets in a generated cs dashboard.
func stateEnteredDashboard(widgets ...string) string {
	return `{"displayName":"Moved to WOW","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

// TestParseGenerated_AcceptsStateEntered: "cases moved to a state in a
// window" as a count, a list (with the state now), a bar grouped on the
// server, and a pie whose slices are windows over the widget's stateEntered.
func TestParseGenerated_AcceptsStateEntered(t *testing.T) {
	raw := stateEnteredDashboard(
		`{"id":"wow-today","displayName":"Moved to WOW Today","resourceType":"case","shape":"count","gridWidth":3,
		  "query":{"filters":[{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]},{"field":"stateEnteredOn","op":"gte","values":["__today__"]}]}}`,
		`{"id":"still-wow","displayName":"Moved to WOW This Week and Still There","resourceType":"case","shape":"list","gridWidth":9,"listLimit":10,
		  "sortBy":{"field":"updatedOn","order":"desc"},
		  "query":{"filters":[{"field":"state","op":"in","values":["waiting_on_wso2"]},{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]},
		    {"field":"stateEnteredOn","op":"gte","values":["__startOfWeek:0__"]},{"field":"stateEnteredOn","op":"lte","values":["2026-11-30"]}]}}`,
		`{"id":"reopened-by-severity","displayName":"Reopened This Month by Severity","resourceType":"case","shape":"bar","gridWidth":6,
		  "query":{"filters":[{"field":"stateEntered","op":"in","values":["reopened","awaiting_info"]},{"field":"stateEnteredOn","op":"gte","values":["__startOfMonth:0__"]}]},
		  "groupBy":{"field":"severity"}}`,
		`{"id":"wow-windows","displayName":"Moved to WOW","resourceType":"case","shape":"pie","gridWidth":6,
		  "query":{"filters":[{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]}]},
		  "slices":[{"label":"Today","query":{"filters":[{"field":"stateEnteredOn","op":"gte","values":["__today__"]}]}},
		    {"label":"Since Monday","query":{"filters":[{"field":"stateEnteredOn","op":"gte","values":["__startOfWeek:0__"]}]}}]}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	got, _ := json.Marshal(d.Widgets[0].Query)
	if !strings.Contains(string(got), `{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]}`) ||
		!strings.Contains(string(got), `{"field":"stateEnteredOn","op":"gte","values":["__today__"]}`) {
		t.Errorf("wow-today query = %s, want both filters kept as written", got)
	}
}

// TestParseGenerated_RefusesStateEnteredItCannotRun: a stateEntered /
// stateEnteredOn filter entity-service would refuse when the tile loads is
// refused at generation instead, with the reason, so the model can retry.
func TestParseGenerated_RefusesStateEnteredItCannotRun(t *testing.T) {
	widget := func(query string) string {
		return stateEnteredDashboard(`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":` + query + `}`)
	}
	pie := func(base, slice string) string {
		return stateEnteredDashboard(`{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":` + base +
			`,"slices":[{"label":"A","query":` + slice + `}]}`)
	}
	const wow = `{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]}`
	for _, tc := range []struct{ name, raw, want string }{
		{"stateEntered notIn", widget(`{"filters":[{"field":"stateEntered","op":"notIn","values":["closed"]}]}`), `stateEntered takes op "in"`},
		{"no states", widget(`{"filters":[{"field":"stateEntered","op":"in","values":[]}]}`), `stateEntered takes op "in" with one or more`},
		{"not a case state", widget(`{"filters":[{"field":"stateEntered","op":"in","values":["waiting_on_wso2","WOW"]}]}`), `"WOW" is not a case state`},
		{"a window alone", widget(`{"filters":[{"field":"stateEnteredOn","op":"gte","values":["__today__"]}]}`), "stateEnteredOn needs a stateEntered filter"},
		{"a window op it does not take", widget(`{"filters":[` + wow + `,{"field":"stateEnteredOn","op":"eq","values":["__today__"]}]}`), `stateEnteredOn takes op "gte" or "lte"`},
		{"two dates", widget(`{"filters":[` + wow + `,{"field":"stateEnteredOn","op":"gte","values":["__today__","2026-11-01"]}]}`), "with one date"},
		{"not a date", widget(`{"filters":[` + wow + `,{"field":"stateEnteredOn","op":"gte","values":["today"]}]}`), "with one date"},
		{"a slice window with no stateEntered anywhere", pie(`{"filters":[]}`, `{"filters":[{"field":"stateEnteredOn","op":"gte","values":["__today__"]}]}`), `slices[0].query.filters[0]: stateEnteredOn needs a stateEntered filter`},
		{"a bad state in a slice", pie(`{"filters":[]}`, `{"filters":[{"field":"stateEntered","op":"in","values":["pending"]}]}`), `"pending" is not a case state`},
		{"in an anyOf branch", widget(`{"anyOf":[{"filters":[` + wow + `]},{"filters":[{"field":"state","op":"in","values":["open"]}]}]}`), "stateEntered cannot be in an anyOf branch"},
	} {
		_, err := ParseGenerated([]byte(tc.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", tc.name, err, tc.want)
		}
	}
}

// TestParseGenerated_TheCatalogueStateEnteredExamplesAreValid: every
// "moved to a state" widget the catalogue shows the model (section 5) passes
// this backend's own check, so the catalogue cannot teach a widget the save
// would refuse.
func TestParseGenerated_TheCatalogueStateEnteredExamplesAreValid(t *testing.T) {
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	_, section, ok := strings.Cut(string(raw), "## 5. Filters")
	if !ok {
		t.Fatal("catalogue has no section 5")
	}
	section, _, _ = strings.Cut(section, "### 5a.")
	var widgets []string
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		if strings.Contains(block, `"stateEntered"`) {
			widgets = append(widgets, block)
		}
		rest = next
	}
	if len(widgets) < 2 {
		t.Fatalf("found %d stateEntered examples in section 5, want at least 2 (a count and slices per period)", len(widgets))
	}
	for i, w := range widgets {
		if _, err := ParseGenerated([]byte(stateEnteredDashboard(w))); err != nil {
			t.Errorf("section 5 stateEntered example %d is rejected: %v\n%s", i+1, err, w)
		}
	}
}

// TestParseGenerated_StateEnteredWindowBounds: a window of two plain dates (or
// instants) that ends before it starts is refused at generation, read as
// entity-service reads it (the last bound wins, a date-only lte covers its
// day, a slice's window replaces the widget's); a window with a relative date
// is not compared.
func TestParseGenerated_StateEnteredWindowBounds(t *testing.T) {
	const wow = `{"field":"stateEntered","op":"in","values":["waiting_on_wso2"]}`
	bound := func(op, v string) string {
		return `{"field":"stateEnteredOn","op":"` + op + `","values":["` + v + `"]}`
	}
	count := func(filters ...string) string {
		return stateEnteredDashboard(`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[` +
			strings.Join(append([]string{wow}, filters...), ",") + `]}}`)
	}
	pie := func(base []string, slice ...string) string {
		return stateEnteredDashboard(`{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,"query":{"filters":[` +
			strings.Join(append([]string{wow}, base...), ",") + `]},"slices":[{"label":"A","query":{"filters":[` + strings.Join(slice, ",") + `]}}]}`)
	}
	for name, raw := range map[string]string{
		"one day":                                count(bound("gte", "2026-03-01"), bound("lte", "2026-03-01")),
		"an instant then the rest of its day":    count(bound("gte", "2026-03-01T10:00:00Z"), bound("lte", "2026-03-01")),
		"the last gte wins":                      count(bound("gte", "2026-03-20"), bound("gte", "2026-03-01"), bound("lte", "2026-03-10")),
		"a relative date is not compared":        count(bound("gte", "2026-03-20"), bound("lte", "__today__")),
		"a slice's window replaces the widget's": pie([]string{bound("gte", "2026-03-01"), bound("lte", "2026-03-10")}, bound("gte", "2026-03-20")),
		"a slice without a window keeps the widget's": pie([]string{bound("gte", "2026-03-01"), bound("lte", "2026-03-10")},
			`{"field":"severity","op":"in","values":["high"]}`),
	} {
		if _, err := ParseGenerated([]byte(raw)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, tc := range []struct{ name, raw, want string }{
		{"two dates the wrong way round", count(bound("gte", "2026-03-15"), bound("lte", "2026-03-01")),
			`widgets[0] ("w").query: stateEnteredOn ends (lte 2026-03-01) before it starts (gte 2026-03-15)`},
		{"the last gte is after the end", count(bound("gte", "2026-03-01"), bound("gte", "2026-03-20"), bound("lte", "2026-03-10")),
			"stateEnteredOn ends (lte 2026-03-10) before it starts (gte 2026-03-20)"},
		{"two instants the wrong way round", count(bound("gte", "2026-03-01T10:00:00Z"), bound("lte", "2026-03-01T09:00:00Z")),
			"stateEnteredOn ends"},
		{"a slice's own window", pie(nil, bound("gte", "2026-03-15"), bound("lte", "2026-03-01")),
			`widgets[0] ("w").slices[0].query: stateEnteredOn ends`},
		{"a slice under the widget's reversed window", pie([]string{bound("gte", "2026-03-15"), bound("lte", "2026-03-01")},
			`{"field":"severity","op":"in","values":["high"]}`), "stateEnteredOn ends"},
	} {
		_, err := ParseGenerated([]byte(tc.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", tc.name, err, tc.want)
		}
	}
}
