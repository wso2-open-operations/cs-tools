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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"strings"
	"testing"
)

func TestClosureEnumLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Open":                       "OPEN",
		"Suspended":                  "SUSPENDED",
		"Pending Notified":           "PENDING_NOTIFIED",
		"PENDING_NOTIFIED":           "PENDING_NOTIFIED",
		"Notified & Previously Paid": "NOTIFIED_AND_PREVIOUSLY_PAID",
		" closure notices ":          "CLOSURE_NOTICES",
	} {
		if got := closureEnumLabel(&in); got == nil || *got != want {
			t.Errorf("closureEnumLabel(%q) = %v, want %q", in, got, want)
		}
	}
	if closureEnumLabel(nil) != nil {
		t.Error("closureEnumLabel(nil) must stay nil so COALESCE keeps the column")
	}
}

func TestProjectSearchOrderBy(t *testing.T) {
	cases := map[[2]string]string{
		{"", ""}:                     "p.created_on DESC, p.id",
		{"", "asc"}:                  "p.created_on ASC, p.id",
		{"endDate", ""}:              "p.end_date ASC NULLS LAST, p.id",
		{"endDate", "desc"}:          "p.end_date DESC NULLS LAST, p.id",
		{"goLivePlanDate", "asc"}:    "p.onboarding_go_live_plan_date ASC NULLS LAST, p.id",
		{"goLiveDate", "desc"}:       "p.onboarding_go_live_date DESC NULLS LAST, p.id",
		{"onboardingExpiryDate", ""}: "p.onboarding_expiry_date ASC NULLS LAST, p.id",
		{"productionReadinessCompletionDate", "desc"}: "p.onboarding_production_readiness_completion_date DESC NULLS LAST, p.id",
		{"x; DROP", "desc"}:                           "p.created_on DESC, p.id",
	}
	for in, want := range cases {
		if got := projectSearchOrderBy(in[0], in[1]); got != want {
			t.Errorf("projectSearchOrderBy(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// TestWso2ClosureStateRule pins the rule's precedence: SUSPENDED, then RESTRICTED, then OPEN.
func TestWso2ClosureStateRule(t *testing.T) {
	susp := strings.Index(wso2ClosureStateRule, "'SUSPENDED'")
	rest := strings.Index(wso2ClosureStateRule, "THEN 'RESTRICTED'")
	open := strings.Index(wso2ClosureStateRule, "ELSE 'OPEN'")
	if susp < 0 || rest < susp || open < rest {
		t.Fatalf("unexpected rule order:\n%s", wso2ClosureStateRule)
	}
}

func TestSrAccessForProject(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name     string
		typeName string
		static   bool
		pdp      *bool
		want     bool
	}{
		{"cloud support pdp true", "Cloud Support", false, &yes, true},
		{"cloud support pdp false", "Cloud Support", false, &no, false},
		{"cloud support pdp null", "Cloud Support", false, nil, false},
		{"cloud support pdp false beats static true", "Cloud Support", true, &no, false},
		{"managed cloud pdp true keeps static true", "Managed Cloud Subscription", true, &yes, true},
		{"managed cloud pdp false keeps static true", "Managed Cloud Subscription", true, &no, true},
		{"managed cloud pdp null keeps static true", "Managed Cloud Subscription", true, nil, true},
		{"other type static false ignores pdp true", "Cloud Evaluation Support", false, &yes, false},
	} {
		if got := srAccessForProject(tc.typeName, tc.static, tc.pdp); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
