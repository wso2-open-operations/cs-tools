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
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
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

// TestParseGenerated_AcceptsTimeCardTotals: grouped on the server by state,
// user, project or day, counting cards or summing hours.
func TestParseGenerated_AcceptsTimeCardTotals(t *testing.T) {
	raw := timeCardDashboard(
		`{"id":"by-state","displayName":"Time Cards by State","resourceType":"time_card","shape":"pie","gridWidth":4,
		  "query":{},"groupBy":{"field":"state"}}`,
		`{"id":"hours-per-user","displayName":"Hours per User","resourceType":"time_card","shape":"bar","gridWidth":4,
		  "query":{"startDate":"__startOfMonth:0__","endDate":"__today__"},"groupBy":{"field":"user","measure":"hours","maxGroups":10}}`,
		`{"id":"cards-per-project","displayName":"Time Cards per Project","resourceType":"time_card","shape":"bar","gridWidth":4,
		  "query":{"states":["approved"]},"groupBy":{"field":"project","measure":"count"}}`,
		`{"id":"my-hours-per-day","displayName":"My Hours per Day","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"userIds":["__current_user__"],"startDate":"__daysAgo:13__","endDate":"__today__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-last-month","displayName":"Hours per Day Last Month","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfMonth:-1__","endDate":"__endOfMonth:-1__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-october","displayName":"Hours per Day in October","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"2026-10-01","endDate":"2026-10-31"},"groupBy":{"field":"day"}}`,
		`{"id":"hours-30-days","displayName":"Hours, Last 31 Days","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__daysAgo:30__","endDate":"__today__"},"groupBy":{"field":"day","measure":"hours"}}`,
		// The catalogue's three day windows: last N days (N <= 31), this month, last month (above).
		`{"id":"hours-this-month","displayName":"Hours per Day This Month","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfMonth:0__","endDate":"__today__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-this-month-full","displayName":"Hours per Day, This Month","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfMonth:0__","endDate":"__endOfMonth:0__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-this-week","displayName":"Hours per Day This Week","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfWeek:0__","endDate":"__endOfWeek:0__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-last-week","displayName":"Hours per Day Last Week","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfWeek:-1__","endDate":"__endOfWeek:-1__"},"groupBy":{"field":"day","measure":"hours"}}`,
		`{"id":"hours-this-year","displayName":"Hours per User This Year","resourceType":"time_card","shape":"bar","gridWidth":6,
		  "query":{"startDate":"__startOfYear:0__","endDate":"__today__"},"groupBy":{"field":"user","measure":"hours"}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if g := d.Widgets[1].GroupBy; g == nil || g.Field != "user" || g.Measure != MeasureHours || g.MaxGroups != 10 {
		t.Errorf("hours-per-user groupBy = %+v, want user / hours / 10", g)
	}
	if g := d.Widgets[0].GroupBy; g == nil || g.Measure != "" {
		t.Errorf("by-state groupBy = %+v, want no measure (count)", g)
	}
}

// TestParseGenerated_TheCatalogueTimeCardTotalsExamplesAreValid: every
// grouped time card widget the catalogue shows the model (section 5a) passes
// this backend's own check, so the catalogue cannot teach a widget the save
// would refuse.
func TestParseGenerated_TheCatalogueTimeCardTotalsExamplesAreValid(t *testing.T) {
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	_, section, ok := strings.Cut(string(raw), "### 5a. Time cards")
	if !ok {
		t.Fatal("catalogue has no section 5a")
	}
	section, _, _ = strings.Cut(section, "### 5b.")
	var widgets []string
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		if strings.Contains(block, `"groupBy"`) {
			widgets = append(widgets, block)
		}
		rest = next
	}
	if len(widgets) < 3 {
		t.Fatalf("found %d grouped time card examples in section 5a, want at least 3 (day, user, project)", len(widgets))
	}
	for i, w := range widgets {
		if _, err := ParseGenerated([]byte(timeCardDashboard(w))); err != nil {
			t.Errorf("section 5a example %d is rejected: %v\n%s", i+1, err, w)
		}
	}
}

// TestParseGenerated_MeasureIsForTimeCardsOnly: a case widget, or a case
// feedback bucket, with a measure is refused, so no other widget's groupBy
// changes meaning.
func TestParseGenerated_MeasureIsForTimeCardsOnly(t *testing.T) {
	for name, w := range map[string]string{
		"case": `{"id":"w","displayName":"W","resourceType":"case","shape":"pie","gridWidth":6,
		          "query":{},"groupBy":{"field":"state","measure":"hours"}}`,
		"incident": `{"id":"w","displayName":"W","resourceType":"incident","shape":"pie","gridWidth":6,
		          "query":{},"groupBy":{"field":"state","measure":"count"}}`,
		"case feedback bucket": `{"id":"w","displayName":"W","resourceType":"case_feedback","shape":"bar","gridWidth":6,
		          "query":{},"groupBy":{"bucket":"week","measure":"count"}}`,
	} {
		_, err := ParseGenerated([]byte(timeCardDashboard(w)))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), `"groupBy.measure" is only supported on a "time_card" widget grouped by a field`) {
			t.Errorf("%s: err = %v, want the measure refusal", name, err)
		}
	}
}

// TestValidateWidgets_MeasureNeedsAField: in a dashboard file (which, unlike
// a generated one, may group a time card widget by bucket), a measure on a
// bucket grouping is refused; a field grouping with one passes.
func TestValidateWidgets_MeasureNeedsAField(t *testing.T) {
	widget := func(g GroupByConfig) Dashboard {
		return Dashboard{ID: "d", Widgets: []WidgetTemplate{{
			ID: "w", DisplayName: "W", ResourceType: ResourceTimeCard, Shape: ShapeBar, GridWidth: 6, GroupBy: &g,
		}}}
	}
	err := validateWidgets(widget(GroupByConfig{Bucket: "week", Measure: MeasureHours}), "test.json")
	if err == nil || !strings.Contains(err.Error(), `"groupBy.measure" is only supported on a "time_card" widget grouped by a field`) {
		t.Errorf("bucket with a measure: err = %v, want the measure refusal", err)
	}
	if err := validateWidgets(widget(GroupByConfig{Field: "user", Measure: MeasureHours}), "test.json"); err != nil {
		t.Errorf("field with a measure: err = %v, want none", err)
	}
}

// TestRelativeDate: relativeDate resolves the tokens as the webapp does, on
// days where month and quarter lengths, weekdays and year ends matter.
func TestRelativeDate(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, c := range []struct{ token, today, want string }{
		{"__today__", "2026-11-02", "2026-11-02"},
		{"__daysAgo:13__", "2026-11-02", "2026-10-20"},
		{"__daysAgo:0__", "2026-03-01", "2026-03-01"},
		{"__startOfMonth:0__", "2026-11-02", "2026-11-01"},
		{"__startOfMonth:-1__", "2026-01-15", "2025-12-01"},
		{"__endOfMonth:0__", "2024-02-10", "2024-02-29"},
		{"__endOfMonth:-1__", "2026-03-31", "2026-02-28"},
		{"__startOfQuarter:0__", "2026-11-02", "2026-10-01"},
		{"__endOfQuarter:0__", "2026-11-02", "2026-12-31"},
		{"__startOfQuarter:-1__", "2026-02-14", "2025-10-01"},
		{"__endOfQuarter:-1__", "2026-02-14", "2025-12-31"},
		// Weeks run Monday to Sunday. 2026-11-06 is a Friday.
		{"__startOfWeek:0__", "2026-11-06", "2026-11-02"},
		{"__endOfWeek:0__", "2026-11-06", "2026-11-08"},
		{"__startOfWeek:-1__", "2026-11-06", "2026-10-26"},
		{"__endOfWeek:-1__", "2026-11-06", "2026-11-01"},
		{"__startOfWeek:0__", "2026-11-08", "2026-11-02"}, // a Sunday ends its week
		{"__startOfWeek:0__", "2026-11-02", "2026-11-02"}, // a Monday starts it
		{"__startOfWeek:0__", "2026-01-01", "2025-12-29"}, // across a year end
		{"__endOfWeek:0__", "2026-01-01", "2026-01-04"},
		{"__startOfYear:0__", "2026-11-06", "2026-01-01"},
		{"__endOfYear:0__", "2026-11-06", "2026-12-31"},
		{"__startOfYear:-1__", "2026-11-06", "2025-01-01"},
		{"__endOfYear:-1__", "2024-02-29", "2023-12-31"},
	} {
		got, ok := relativeDate(c.token, day(c.today))
		if !ok || !got.Equal(day(c.want)) {
			t.Errorf("relativeDate(%s) on %s = %v %v, want %s", c.token, c.today, got.Format("2006-01-02"), ok, c.want)
		}
	}
	for _, token := range []string{"__today:1__", "__daysAgo__", "__daysAgo:-1__", "__startOfMonth__", "__startOfWeek__", "__endOfYear__", "__yesterday__", "2026-11-02"} {
		if _, ok := relativeDate(token, day("2026-11-02")); ok {
			t.Errorf("relativeDate(%s) resolved, want it left unresolved (the webapp does)", token)
		}
	}
}

// TestParseGenerated_RejectsTimeCardWidgetsItCannotRun: a time card widget the
// entity service cannot answer is refused with the reason: another grouping or
// a bucket, an unknown measure, and a per-day window that is missing, too long
// on some day of the year, or mixes a plain and a relative date.
func TestParseGenerated_RejectsTimeCardWidgetsItCannotRun(t *testing.T) {
	tc := func(shape, rest string) string {
		return timeCardDashboard(`{"id":"w","displayName":"W","resourceType":"time_card","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"group by approver", tc("bar", `,"query":{},"groupBy":{"field":"approver","maxGroups":10}`), "groups by state, user, project or day only"},
		{"group by a bucket", tc("bar", `,"query":{},"groupBy":{"bucket":"week"}`), "groups by state, user, project or day only"},
		{"an unknown measure", tc("bar", `,"query":{},"groupBy":{"field":"user","measure":"minutes"}`), `unknown "groupBy.measure" "minutes"`},
		{"day with no window", tc("bar", `,"query":{},"groupBy":{"field":"day"}`), "needs query.startDate and query.endDate"},
		{"day with only an end", tc("bar", `,"query":{"endDate":"__today__"},"groupBy":{"field":"day"}`), "needs query.startDate and query.endDate"},
		{"day with only a start", tc("bar", `,"query":{"startDate":"__daysAgo:6__"},"groupBy":{"field":"day"}`), "needs query.startDate and query.endDate"},
		{"day over 32 plain days", tc("bar", `,"query":{"startDate":"2026-10-01","endDate":"2026-11-01"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day ending before it starts", tc("bar", `,"query":{"startDate":"2026-10-10","endDate":"2026-10-09"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day over 31 relative days", tc("bar", `,"query":{"startDate":"__daysAgo:31__","endDate":"__today__"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day over the last two months", tc("bar", `,"query":{"startDate":"__startOfMonth:-1__","endDate":"__today__"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		// Short at a quarter's start, too long by its end: refused whatever today is.
		{"day over this quarter so far", tc("bar", `,"query":{"startDate":"__startOfQuarter:0__","endDate":"__today__"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day over this year so far", tc("bar", `,"query":{"startDate":"__startOfYear:0__","endDate":"__today__"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day over the last five weeks", tc("bar", `,"query":{"startDate":"__startOfWeek:-4__","endDate":"__endOfWeek:0__"},"groupBy":{"field":"day"}`), "covers at most 31 days"},
		{"day from a plain date to today", tc("bar", `,"query":{"startDate":"2026-10-20","endDate":"__today__"},"groupBy":{"field":"day"}`), "two plain dates or two relative dates"},
		{"day with a timestamp", tc("bar", `,"query":{"startDate":"2026-10-01T00:00:00Z","endDate":"2026-10-07"},"groupBy":{"field":"day"}`), "takes plain dates (YYYY-MM-DD)"},
		{"day with a token the webapp leaves as it is", tc("bar", `,"query":{"startDate":"__startOfMonth__","endDate":"__today__"},"groupBy":{"field":"day"}`), "not a relative date the dashboard can resolve"},
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
