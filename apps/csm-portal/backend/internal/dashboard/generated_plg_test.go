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
		{"pie grouped on the server", org("pie", `,"query":{},"groupBy":{"field":"product","maxGroups":5}`), "cannot use groupBy; group with slices"},
		{"slice with owner ids", org("bar", `,"query":{},"slices":[{"label":"Mine","query":{"ownerIds":["5e636a12-0000-4000-8000-000000000001"]}}]`), `slices[0].query: "ownerIds" is not a PLG organisation filter`},
		{"slice with an unknown stage", reg("pie", `,"query":{},"slices":[{"label":"Won","query":{"lifecycleStages":["WON"]}}]`), `"WON" is not a PLG registration lifecycle stage`},
		{"slice unacknowledged on organisations", org("pie", `,"query":{},"slices":[{"label":"New","query":{"unacknowledgedOnly":true}}]`), `slices[0].query: "unacknowledgedOnly" is not a PLG organisation filter`},
		{"slice with a bad date", reg("bar", `,"query":{},"slices":[{"label":"Sep","query":{"registeredFrom":"September"}}]`), "slices[0].query.registeredFrom must be a date"},
		{"unknown product", org("count", `,"query":{"productCodes":["CHOREO"]}`), `"CHOREO" is not a PLG organisation product`},
		{"lower-case stage", org("count", `,"query":{"lifecycleStages":["activated"]}`), `"activated" is not a PLG organisation lifecycle stage`},
		{"unknown tier", org("count", `,"query":{"subscriptionTiers":["ENTERPRISE"]}`), `"ENTERPRISE" is not a PLG organisation subscription tier`},
		{"owner ids", org("count", `,"query":{"ownerIds":["5e636a12-0000-4000-8000-000000000001"]}`), `"ownerIds" is not a PLG organisation filter`},
		{"free text", org("count", `,"query":{"query":"acme"}`), `"query" is not a PLG organisation filter`},
		{"unowned as a string", org("count", `,"query":{"unowned":"true"}`), "unowned must be true or false"},
		{"bad date", org("count", `,"query":{"registeredFrom":"last month"}`), "registeredFrom must be a date"},
		{"organisation sort", org("list", `,"query":{},"listLimit":5,"sortBy":{"field":"owner","order":"asc"}`), "sorts by name or registeredOn"},
		{"unacknowledged on organisations", org("count", `,"query":{"unacknowledgedOnly":true}`), `"unacknowledgedOnly" is not a PLG organisation filter`},
		{"registration sort", reg("list", `,"query":{},"listLimit":5,"sortBy":{"field":"name","order":"asc"}`), "a PLG registration list takes no sortBy"},
		{"registration groupBy", reg("count", `,"query":{},"groupBy":{"field":"product","maxGroups":5}`), "cannot use groupBy"},
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
