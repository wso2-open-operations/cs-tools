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
