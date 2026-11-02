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
	"errors"
	"strings"
	"testing"
)

// timeCardDashboard wraps widgets in a generated cs dashboard.
func timeCardDashboard(widgets ...string) string {
	return `{"displayName":"Time Cards","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

func TestParseGenerated_AcceptsTimeCardWidgets(t *testing.T) {
	raw := timeCardDashboard(
		`{"id":"my-pending","displayName":"My Pending Time Cards","resourceType":"time_card","shape":"count","gridWidth":3,
		  "query":{"states":["pending","submitted"],"userIds":["__current_user__"]}}`,
		`{"id":"recent","displayName":"Recent Time Cards","resourceType":"time_card","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"workDate","order":"desc"},"query":{"startDate":"__daysAgo:30__","endDate":"__today__"}}`,
		`{"id":"by-state","displayName":"Time Cards by State","resourceType":"time_card","shape":"pie","gridWidth":6,
		  "query":{"startDate":"__startOfMonth:-1__","endDate":"2026-10-31"},
		  "slices":[{"label":"Pending","query":{"states":["pending"]}},{"label":"Approved","color":"success","query":{"states":["approved","processed"]}}]}`,
		// A case widget beside them.
		`{"id":"my-cases","displayName":"My Active Cases","resourceType":"case","shape":"count","gridWidth":3,
		  "query":{"filters":[{"field":"assignedUserId","op":"in","values":["__current_user__"]},{"field":"state","op":"notIn","values":["closed"]}]}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if len(d.Widgets) != 4 || d.Widgets[0].ResourceType != ResourceTimeCard {
		t.Fatalf("widgets = %+v", d.Widgets)
	}
	// A time card query is the search's flat filters: no case "type" filter is injected.
	if _, ok := d.Widgets[0].Query["filters"]; ok {
		t.Errorf("time card query got a filters list: %+v", d.Widgets[0].Query)
	}
}

func TestParseGenerated_RejectsTimeCardWidgetsItCannotRun(t *testing.T) {
	tc := func(shape, rest string) string {
		return timeCardDashboard(`{"id":"w","displayName":"W","resourceType":"time_card","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"group by user", tc("bar", `,"query":{},"groupBy":{"field":"user","maxGroups":10}`), "cannot use groupBy"},
		{"line", tc("line", `,"query":{},"days":30`), "shape must be count, list, pie or bar"},
		{"a filter it does not take", tc("count", `,"query":{"projectIds":["p1"]}`), `"projectIds" is not a time card filter`},
		{"a case-style filter list", tc("count", `,"query":{"filters":[{"field":"state","op":"in","values":["pending"]}]}`), `"filters" is not a time card filter`},
		{"an unknown state", tc("count", `,"query":{"states":["open"]}`), `"open" is not a time card state`},
		{"no states", tc("count", `,"query":{"states":[]}`), "non-empty list of states"},
		{"someone else's cards", tc("count", `,"query":{"userIds":["5e636a12-0000-4000-8000-000000000001"]}`), `userIds can only be ["__current_user__"]`},
		{"a date that is not one", tc("count", `,"query":{"startDate":"last month"}`), "startDate must be a date"},
		{"an unknown relative date", tc("count", `,"query":{"endDate":"__yesterday__"}`), "endDate must be a date"},
		{"a slice with a bad filter", tc("pie", `,"query":{},"slices":[{"label":"A","query":{"state":["pending"]}}]`), `slices[0].query: "state" is not a time card filter`},
		{"a sort it does not have", tc("list", `,"query":{},"listLimit":5,"sortBy":{"field":"createdOn","order":"desc"}`), "sorts by workDate or updatedOn"},
	}
	for _, c := range cases {
		_, err := ParseGenerated([]byte(c.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", c.name, err, c.want)
		}
	}
}
