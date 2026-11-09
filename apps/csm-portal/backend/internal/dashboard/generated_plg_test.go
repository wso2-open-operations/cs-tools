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
		{"pie", org("pie", `,"query":{},"slices":[{"label":"IAM","query":{"productCodes":["IAM"]}}]`), "shape must be count or list"},
		{"line", org("line", `,"query":{},"days":30`), "shape must be count or list"},
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
