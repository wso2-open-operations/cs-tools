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
	"reflect"
	"strings"
	"testing"
)

// plgDashboard wraps widgets in a generated cs dashboard.
func plgDashboard(widgets ...string) string {
	return `{"displayName":"PLG","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

// TestParseGenerated_AcceptsPlgWidgets checks the PLG counts and lists a
// generated dashboard may hold: every flat filter, plain and relative
// registration dates, and the organisation list's sorts.
func TestParseGenerated_AcceptsPlgWidgets(t *testing.T) {
	raw := plgDashboard(
		`{"id":"trials","displayName":"IAM Organisations on a Trial","resourceType":"plg_organization","shape":"count","gridWidth":3,
		  "query":{"productCodes":["IAM"],"subscriptionTiers":["TRIAL","TRIAL_EXTENDED"]}}`,
		`{"id":"unowned","displayName":"Unowned Activated Organisations","resourceType":"plg_organization","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"name","order":"asc"},"query":{"lifecycleStages":["ACTIVATED","COMMERCIAL"],"unowned":true}}`,
		`{"id":"newest","displayName":"Organisations Registered This Month","resourceType":"plg_organization","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"registeredOn","order":"desc"},"query":{"registeredFrom":"__startOfMonth:0__","registeredTo":"__today__"}}`,
		`{"id":"new-regs","displayName":"New Registrations","resourceType":"plg_registration","shape":"count","gridWidth":3,
		  "query":{"unacknowledgedOnly":true}}`,
		`{"id":"regs-2026","displayName":"API Platform Registrations in October 2026","resourceType":"plg_registration","shape":"list","gridWidth":6,"listLimit":5,
		  "query":{"productCodes":["API_PLATFORM"],"registeredFrom":"2026-10-01","registeredTo":"2026-10-31"}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if len(d.Widgets) != 5 {
		t.Fatalf("widgets = %d, want 5", len(d.Widgets))
	}
}

// TestParseGenerated_AcceptsPlgSlices checks a PLG pie or bar grouped with
// slices: each slice's query takes the same filters as the widget's, so a
// registration slice may set unacknowledgedOnly and relative registration
// dates.
func TestParseGenerated_AcceptsPlgSlices(t *testing.T) {
	raw := plgDashboard(
		`{"id":"by-product","displayName":"Organisations by Product","resourceType":"plg_organization","shape":"pie","gridWidth":6,
		  "query":{"subscriptionTiers":["TRIAL","TRIAL_EXTENDED"]},
		  "slices":[{"label":"Identity & Access Management","query":{"productCodes":["IAM"]}},{"label":"API Platform","query":{"productCodes":["API_PLATFORM"]}}]}`,
		`{"id":"by-stage","displayName":"Organisations by Stage","resourceType":"plg_organization","shape":"bar","gridWidth":6,"query":{},
		  "slices":[{"label":"Activated","query":{"lifecycleStages":["ACTIVATED"]}},{"label":"Unowned","query":{"unowned":true}}]}`,
		`{"id":"signups","displayName":"Sign-ups, Last 3 Months","resourceType":"plg_registration","shape":"bar","gridWidth":6,"query":{"unacknowledgedOnly":true},
		  "slices":[{"label":"2 Months Ago","query":{"registeredFrom":"__startOfMonth:-2__","registeredTo":"__endOfMonth:-2__"}},
		            {"label":"Last Month","query":{"registeredFrom":"__startOfMonth:-1__","registeredTo":"__endOfMonth:-1__"}},
		            {"label":"This Month","query":{"registeredFrom":"__startOfMonth:0__","registeredTo":"__today__"}}]}`,
		`{"id":"new-by-tier","displayName":"New Registrations by Tier","resourceType":"plg_registration","shape":"pie","gridWidth":6,"query":{},
		  "slices":[{"label":"Free","query":{"subscriptionTiers":["FREE"],"unacknowledgedOnly":true}},{"label":"Pay as You Go","query":{"subscriptionTiers":["PAYG"],"unacknowledgedOnly":true}}]}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if len(d.Widgets) != 4 || len(d.Widgets[2].Slices) != 3 {
		t.Fatalf("widgets = %d, signups slices = %d, want 4 and 3", len(d.Widgets), len(d.Widgets[2].Slices))
	}
}

// TestParseGenerated_AcceptsPlgRegistrationLines checks the registration trend
// a generated dashboard may hold: the trailing days, a fixed window by day,
// week or month (plain or relative dates), as bars, over the registration
// filters.
func TestParseGenerated_AcceptsPlgRegistrationLines(t *testing.T) {
	raw := plgDashboard(
		`{"id":"last-30","displayName":"PLG Sign-ups, Last 30 Days","resourceType":"plg_registration","shape":"line","gridWidth":12,
		  "query":{"productCodes":["IAM"]},"days":30}`,
		`{"id":"per-month","displayName":"PLG Sign-ups per Month, Last 3 Months","resourceType":"plg_registration","shape":"line","gridWidth":12,
		  "query":{},"startDate":"__startOfMonth:-2__","endDate":"__today__","bucket":"month","chart":"bar"}`,
		`{"id":"per-week","displayName":"New Registrations per Week This Year","resourceType":"plg_registration","shape":"line","gridWidth":12,
		  "query":{"unacknowledgedOnly":true},"startDate":"__startOfYear:0__","endDate":"__endOfYear:0__","bucket":"week"}`,
		`{"id":"oct-2026","displayName":"Sign-ups per Day in October 2026","resourceType":"plg_registration","shape":"line","gridWidth":12,
		  "query":{"lifecycleStages":["ACTIVATED"]},"startDate":"2026-10-01","endDate":"2026-10-31"}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if w := d.Widgets[1]; w.Shape != ShapeLine || w.Bucket != LineBucketMonth || w.Chart != LineChartBar || w.StartDate != "__startOfMonth:-2__" {
		t.Errorf("per-month = %+v, want a month line from __startOfMonth:-2__ as bars", w)
	}
	if w := d.Widgets[0]; w.Days != 30 || w.StartDate != "" {
		t.Errorf("last-30 = days %d window %q, want the trailing 30 days", w.Days, w.StartDate)
	}
}

// TestParseGenerated_AcceptsMyPlgWidgets checks "my PLG organisations" and
// "my registrations": ownerIds exactly ["__current_user__"], on either PLG
// type, in a query, a slice or a registration line.
func TestParseGenerated_AcceptsMyPlgWidgets(t *testing.T) {
	raw := plgDashboard(
		`{"id":"mine","displayName":"My PLG Organisations","resourceType":"plg_organization","shape":"list","gridWidth":6,"listLimit":5,
		  "sortBy":{"field":"name","order":"asc"},"query":{"ownerIds":["__current_user__"]}}`,
		`{"id":"my-new","displayName":"My New Registrations","resourceType":"plg_registration","shape":"count","gridWidth":3,
		  "query":{"ownerIds":["__current_user__"],"unacknowledgedOnly":true}}`,
		`{"id":"mine-vs-unowned","displayName":"Mine and Unowned","resourceType":"plg_organization","shape":"pie","gridWidth":6,"query":{},
		  "slices":[{"label":"Mine","query":{"ownerIds":["__current_user__"]}},{"label":"Unowned","query":{"unowned":true}}]}`,
		`{"id":"my-signups","displayName":"My Sign-ups per Month","resourceType":"plg_registration","shape":"line","gridWidth":12,
		  "query":{"ownerIds":["__current_user__"]},"startDate":"__startOfMonth:-2__","endDate":"__today__","bucket":"month"}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if got := d.Widgets[0].Query["ownerIds"]; !reflect.DeepEqual(got, []any{"__current_user__"}) {
		t.Errorf("mine ownerIds = %#v, want the placeholder kept for the webapp to resolve", got)
	}
}

// TestParseGenerated_AcceptsPlgOrganisationsByOwner checks "PLG
// organisations by owner": a pie or bar grouped by owner on the server, with
// the widget's filters and maxGroups kept.
func TestParseGenerated_AcceptsPlgOrganisationsByOwner(t *testing.T) {
	raw := plgDashboard(
		`{"id":"by-owner","displayName":"PLG Organisations by Owner","resourceType":"plg_organization","shape":"bar","gridWidth":6,
		  "query":{},"groupBy":{"field":"owner","maxGroups":10}}`,
		`{"id":"iam-by-owner","displayName":"Activated IAM Organisations by Owner","resourceType":"plg_organization","shape":"pie","gridWidth":6,
		  "query":{"productCodes":["IAM"],"lifecycleStages":["ACTIVATED"]},"groupBy":{"field":"owner","maxGroups":5}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if g := d.Widgets[1].GroupBy; g == nil || g.Field != "owner" || g.MaxGroups != 5 {
		t.Errorf("iam-by-owner groupBy = %+v, want owner with maxGroups 5", g)
	}
	if got := d.Widgets[1].Query["productCodes"]; !reflect.DeepEqual(got, []any{"IAM"}) {
		t.Errorf("iam-by-owner productCodes = %#v, want the filter kept", got)
	}
}

// TestParseGenerated_RejectsPlgWidgetsItCannotRun checks that a PLG widget
// holds only what the PLG searches take and a prompt can name.
func TestParseGenerated_RejectsPlgWidgetsItCannotRun(t *testing.T) {
	org := func(shape, rest string) string {
		return plgDashboard(`{"id":"w","displayName":"W","resourceType":"plg_organization","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	reg := func(shape, rest string) string {
		return plgDashboard(`{"id":"w","displayName":"W","resourceType":"plg_registration","shape":"` + shape + `","gridWidth":6` + rest + `}`)
	}
	cases := []struct {
		name, raw, want string
	}{
		{"line", org("line", `,"query":{},"days":30`), "shape must be count, list, pie or bar"},
		{"pie without slices", org("pie", `,"query":{}`), "needs either"},
		{"pie grouped by product on the server", org("pie", `,"query":{},"groupBy":{"field":"product","maxGroups":5}`), "groups by owner only"},
		{"bar grouped by an owner bucket", org("bar", `,"query":{},"groupBy":{"bucket":"month"}`), "groups by owner only"},
		{"owner grouping and slices", org("pie", `,"query":{},"groupBy":{"field":"owner"},"slices":[{"label":"Unowned","query":{"unowned":true}}]`), `carries both "slices" and "groupBy"`},
		{"owner grouping in hours", org("bar", `,"query":{},"groupBy":{"field":"owner","measure":"hours"}`), `"groupBy.measure"`},
		{"registrations by owner", reg("pie", `,"query":{},"groupBy":{"field":"owner","maxGroups":5}`), "cannot use groupBy; group with slices"},
		{"a registration line by owner", reg("line", `,"query":{},"days":30,"groupBy":{"field":"owner"}`), "cannot use groupBy"},
		{"slice with owner ids", org("bar", `,"query":{},"slices":[{"label":"Mine","query":{"ownerIds":["5e636a12-0000-4000-8000-000000000001"]}}]`), `slices[0].query.ownerIds can only be ["__current_user__"]`},
		{"slice with an unknown stage", reg("pie", `,"query":{},"slices":[{"label":"Won","query":{"lifecycleStages":["WON"]}}]`), `"WON" is not a PLG registration lifecycle stage`},
		{"slice unacknowledged on organisations", org("pie", `,"query":{},"slices":[{"label":"New","query":{"unacknowledgedOnly":true}}]`), `slices[0].query: "unacknowledgedOnly" is not a PLG organisation filter`},
		{"slice with a bad date", reg("bar", `,"query":{},"slices":[{"label":"Sep","query":{"registeredFrom":"September"}}]`), "slices[0].query.registeredFrom must be a date"},
		{"unknown product", org("count", `,"query":{"productCodes":["CHOREO"]}`), `"CHOREO" is not a PLG organisation product`},
		{"lower-case stage", org("count", `,"query":{"lifecycleStages":["activated"]}`), `"activated" is not a PLG organisation lifecycle stage`},
		{"unknown tier", org("count", `,"query":{"subscriptionTiers":["ENTERPRISE"]}`), `"ENTERPRISE" is not a PLG organisation subscription tier`},
		{"owner ids", org("count", `,"query":{"ownerIds":["5e636a12-0000-4000-8000-000000000001"]}`), `query.ownerIds can only be ["__current_user__"]`},
		{"me and another owner", org("count", `,"query":{"ownerIds":["__current_user__","5e636a12-0000-4000-8000-000000000001"]}`), `query.ownerIds can only be ["__current_user__"]`},
		{"me as a string", reg("count", `,"query":{"ownerIds":"__current_user__"}`), `ownerIds`},
		{"free text", org("count", `,"query":{"query":"acme"}`), `"query" is not a PLG organisation filter`},
		{"unowned as a string", org("count", `,"query":{"unowned":"true"}`), "unowned must be true or false"},
		{"bad date", org("count", `,"query":{"registeredFrom":"last month"}`), "registeredFrom must be a date"},
		{"organisation sort", org("list", `,"query":{},"listLimit":5,"sortBy":{"field":"owner","order":"asc"}`), "sorts by name or registeredOn"},
		{"unacknowledged on organisations", org("count", `,"query":{"unacknowledgedOnly":true}`), `"unacknowledgedOnly" is not a PLG organisation filter`},
		{"registration sort", reg("list", `,"query":{},"listLimit":5,"sortBy":{"field":"name","order":"asc"}`), "a PLG registration list takes no sortBy"},
		{"registration groupBy", reg("count", `,"query":{},"groupBy":{"field":"product","maxGroups":5}`), "cannot use groupBy"},
		{"registration table", reg("table", `,"query":{}`), "shape must be count, list, pie, bar or line"},
		{"registration line with a series", reg("line", `,"query":{},"days":30,"series":["opened"]`), `"series" is only meaningful for resourceType "case"`},
		{"registration line over 90 days", reg("line", `,"query":{},"days":91`), `"days" is 91`},
		{"registration line, days and a window", reg("line", `,"query":{},"days":30,"startDate":"2026-01-01","endDate":"2026-01-31"`), `"days" cannot be combined`},
		{"registration line, this year by day", reg("line", `,"query":{},"startDate":"__startOfYear:0__","endDate":"__today__"`), "cover at most 90 days"},
		{"registration line, a bucket alone", reg("line", `,"query":{},"bucket":"month"`), `"bucket" needs "startDate" and "endDate"`},
		{"registration line, an owner filter", reg("line", `,"query":{"ownerIds":["5e636a12-0000-4000-8000-000000000001"]},"days":30`), `query.ownerIds can only be ["__current_user__"]`},
		{"registration line, unknown chart", reg("line", `,"query":{},"days":30,"chart":"area"`), `unknown "chart" "area"`},
	}
	for _, c := range cases {
		_, err := ParseGenerated([]byte(c.raw))
		if !errors.Is(err, ErrGeneratedInvalid) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want ErrGeneratedInvalid mentioning %q", c.name, err, c.want)
		}
	}
}

// TestParseGenerated_TheCataloguePlgExamplesAreValid: every PLG widget the
// catalogue shows the model (section 5e) passes this backend's own check, so
// the catalogue cannot teach a PLG widget the save would refuse. The
// catalogue writes a widget without the keys every widget has (id,
// displayName, gridWidth); those are filled in, the rest is checked as
// written.
func TestParseGenerated_TheCataloguePlgExamplesAreValid(t *testing.T) {
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	_, section, ok := strings.Cut(string(raw), "### 5e. PLG")
	if !ok {
		t.Fatal("catalogue has no section 5e")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var blocks []string
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		blocks = append(blocks, block)
		rest = next
	}
	if len(blocks) == 0 {
		t.Fatal("found no PLG example in section 5e")
	}
	for i, block := range blocks {
		var w map[string]any
		if err := json.Unmarshal([]byte(block), &w); err != nil {
			t.Fatalf("section 5e example %d is not JSON: %v\n%s", i+1, err, block)
		}
		for key, value := range map[string]any{"id": "example", "displayName": "Example", "gridWidth": 6} {
			if _, set := w[key]; !set {
				w[key] = value
			}
		}
		widget, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParseGenerated([]byte(plgDashboard(string(widget)))); err != nil {
			t.Errorf("section 5e example %d is rejected: %v\n%s", i+1, err, block)
		}
	}
}
