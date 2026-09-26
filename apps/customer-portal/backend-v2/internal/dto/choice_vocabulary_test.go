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

package dto

import "testing"

func intPtr(n int) *int { return &n }

// The dashboard's outstanding-cases chart matches severity buckets on the
// LABEL, against "Catastrophic (P0)" .. "Low (P4)"
// (features/dashboard/constants/dashboard.ts, SEVERITY_LEGEND_ORDER). The
// Postgres data source returns the raw enum label as both id and label, so
// without this translation every bucket misses and the chart renders empty
// beside a non-zero outstanding count.
func TestNormalizeCaseSeverityChoices_PostgresEnumLabels(t *testing.T) {
	in := []ReferenceItem{
		{ID: "S0", Label: "S0", Count: intPtr(1)},
		{ID: "S1", Label: "S1", Count: intPtr(7)},
		{ID: "S2", Label: "S2", Count: intPtr(0)},
		{ID: "S3", Label: "S3", Count: intPtr(4)},
		{ID: "S4", Label: "S4", Count: intPtr(11)},
	}
	want := []ReferenceItem{
		{ID: "14", Label: "Catastrophic (P0)", Count: intPtr(1)},
		{ID: "10", Label: "Critical (P1)", Count: intPtr(7)},
		{ID: "11", Label: "High (P2)", Count: intPtr(0)},
		{ID: "12", Label: "Medium (P3)", Count: intPtr(4)},
		{ID: "13", Label: "Low (P4)", Count: intPtr(11)},
	}

	got := normalizeCaseSeverityChoices(in)
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Label != want[i].Label {
			t.Errorf("[%d] = {%s, %s}, want {%s, %s}", i, got[i].ID, got[i].Label, want[i].ID, want[i].Label)
		}
		if got[i].Count == nil || *got[i].Count != *want[i].Count {
			t.Errorf("[%d] count not preserved", i)
		}
	}
}

// entity-service's own documented contract is the lowercase domain enum, so
// that spelling has to resolve too.
func TestNormalizeCaseSeverityChoices_DomainEnums(t *testing.T) {
	got := normalizeCaseSeverityChoices([]ReferenceItem{{ID: "critical", Label: "critical"}})
	if got[0].ID != "10" || got[0].Label != "Critical (P1)" {
		t.Errorf("got {%s, %s}, want {10, Critical (P1)}", got[0].ID, got[0].Label)
	}
}

// ServiceNow already sends what the frontend wants. Translating it again would
// break the data source that works today, so an unrecognised id passes through
// untouched.
func TestNormalizeCaseSeverityChoices_ServiceNowIDsPassThrough(t *testing.T) {
	in := []ReferenceItem{
		{ID: "10", Label: "Critical (P1)", Count: intPtr(3)},
		{ID: "999", Label: "Something Unknown"},
	}
	got := normalizeCaseSeverityChoices(in)
	for i := range in {
		if got[i].ID != in[i].ID || got[i].Label != in[i].Label {
			t.Errorf("[%d] = {%s, %s}, want it unchanged {%s, %s}",
				i, got[i].ID, got[i].Label, in[i].ID, in[i].Label)
		}
	}
}

// The cases table derives its status filter with Number(status.id)
// (features/dashboard/utils/casesTable.ts). A non-numeric id becomes NaN,
// which serialises to null and silently drops the filter, so these ids have
// to come back numeric.
func TestNormalizeCaseStateChoices_PostgresEnumLabels(t *testing.T) {
	in := []ReferenceItem{
		{ID: "OPEN", Label: "OPEN"},
		{ID: "WORK_IN_PROGRESS", Label: "WORK_IN_PROGRESS"},
		{ID: "WAITING_ON_WSO2", Label: "WAITING_ON_WSO2"},
		{ID: "AWAITING_INFO", Label: "AWAITING_INFO"},
		{ID: "REOPENED", Label: "REOPENED"},
		{ID: "SOLUTION_PROPOSED", Label: "SOLUTION_PROPOSED"},
		{ID: "CLOSED", Label: "CLOSED"},
	}
	want := []ReferenceItem{
		{ID: "1", Label: "Open"},
		{ID: "10", Label: "Work In Progress"},
		{ID: "1003", Label: "Waiting On WSO2"},
		{ID: "18", Label: "Awaiting Info"},
		{ID: "1006", Label: "Reopened"},
		{ID: "6", Label: "Solution Proposed"},
		{ID: "3", Label: "Closed"},
	}

	got := normalizeCaseStateChoices(in)
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Label != want[i].Label {
			t.Errorf("[%d] = {%s, %s}, want {%s, %s}", i, got[i].ID, got[i].Label, want[i].ID, want[i].Label)
		}
	}
}

// The closed state must keep the exact label the table filters on when it
// builds its default "outstanding" status set.
func TestNormalizeCaseStateChoices_ClosedLabelIsRecognisable(t *testing.T) {
	got := normalizeCaseStateChoices([]ReferenceItem{{ID: "CLOSED", Label: "CLOSED"}})
	if got[0].Label != "Closed" {
		t.Errorf("closed label = %q, want %q", got[0].Label, "Closed")
	}
}

func TestNormalizeChoices_EmptyInputIsEmptyNotNil(t *testing.T) {
	if got := normalizeCaseSeverityChoices(nil); got == nil {
		t.Error("nil input must produce an empty slice, not nil -- it serialises as [] not null")
	}
}

// The create-case form's resolveIssueTypeKey (webapp) does
// parseInt(item.id, 10) || 0 to build issueTypeKey, then treats 0 as "no
// issue type selected". Postgres's case_issue_type_enum labels
// (ERROR/PARTIAL_OUTAGE/PERFORMANCE_DEGRADATION/QUESTION/
// SECURITY_OR_COMPLIANCE/TOTAL_OUTAGE) are not numeric, so without this
// translation every issue type parses to NaN -> 0 and the form rejects
// every selection with "Please select an issue type," even though one was
// picked.
func TestNormalizeCaseIssueTypeChoices_PostgresEnumLabels(t *testing.T) {
	in := []ReferenceItem{
		{ID: "TOTAL_OUTAGE", Label: "TOTAL_OUTAGE"},
		{ID: "PARTIAL_OUTAGE", Label: "PARTIAL_OUTAGE"},
		{ID: "PERFORMANCE_DEGRADATION", Label: "PERFORMANCE_DEGRADATION"},
		{ID: "QUESTION", Label: "QUESTION"},
		{ID: "SECURITY_OR_COMPLIANCE", Label: "SECURITY_OR_COMPLIANCE"},
		{ID: "ERROR", Label: "ERROR"},
	}
	want := []ReferenceItem{
		{ID: "1", Label: "Total Outage"},
		{ID: "2", Label: "Partial Outage"},
		{ID: "3", Label: "Performance Degradation"},
		{ID: "4", Label: "Question"},
		{ID: "5", Label: "Security Or Compliance"},
		{ID: "6", Label: "Error"},
	}

	got := normalizeCaseIssueTypeChoices(in)
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Label != want[i].Label {
			t.Errorf("[%d] = {%s, %s}, want {%s, %s}", i, got[i].ID, got[i].Label, want[i].ID, want[i].Label)
		}
	}
}

// ServiceNow-mode already returns numeric ids -- an id this table doesn't
// recognise passes through untouched, same as severity/state.
func TestNormalizeCaseIssueTypeChoices_ServiceNowIDsPassThrough(t *testing.T) {
	in := []ReferenceItem{{ID: "3", Label: "Performance Degradation"}}
	got := normalizeCaseIssueTypeChoices(in)
	if got[0].ID != in[0].ID || got[0].Label != in[0].Label {
		t.Errorf("got {%s, %s}, want it unchanged {%s, %s}", got[0].ID, got[0].Label, in[0].ID, in[0].Label)
	}
}

// engagement_type_enum's Postgres labels have the same non-numeric-id problem
// as issue type, for the same resolveIssueTypeKey-style numeric-key
// assumption on engagementTypeKeys filters.
func TestNormalizeCaseEngagementTypeChoices_PostgresEnumLabels(t *testing.T) {
	in := []ReferenceItem{
		{ID: "MIGRATION", Label: "MIGRATION"},
		{ID: "CONSULTANCY", Label: "CONSULTANCY"},
		{ID: "NEW_FEATURE_IMPROVEMENT", Label: "NEW_FEATURE_IMPROVEMENT"},
		{ID: "FOLLOW_UP", Label: "FOLLOW_UP"},
		{ID: "ONBOARDING", Label: "ONBOARDING"},
	}
	want := []ReferenceItem{
		{ID: "1", Label: "Migration"},
		{ID: "2", Label: "Consultancy"},
		{ID: "3", Label: "New Feature Improvement"},
		{ID: "4", Label: "Follow Up"},
		{ID: "5", Label: "Onboarding"},
	}

	got := normalizeCaseEngagementTypeChoices(in)
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Label != want[i].Label {
			t.Errorf("[%d] = {%s, %s}, want {%s, %s}", i, got[i].ID, got[i].Label, want[i].ID, want[i].Label)
		}
	}
}

// deployment_type_enum's Postgres labels have the same non-numeric-id problem
// as issue type/engagement type. This one had a real, live downstream break:
// EditDeploymentModal.tsx's Number(form.typeKey) on a raw label like
// "DEVELOPMENT" is NaN, and NaN is never equal to itself in JS, so every save
// (even ones that didn't touch type) sent typeKey: NaN -- JSON.stringify'd to
// null -- tripping the handler's "provide detail fields or active, not both"
// guard on completely unrelated edits (e.g. deactivating a deployment).
func TestNormalizeDeploymentTypeChoices_PostgresEnumLabels(t *testing.T) {
	in := []ReferenceItem{
		{ID: "DEVELOPMENT", Label: "DEVELOPMENT"},
		{ID: "QA", Label: "QA"},
		{ID: "STAGING", Label: "STAGING"},
		{ID: "STRESS", Label: "STRESS"},
		{ID: "UAT", Label: "UAT"},
		{ID: "PRIMARY_PRODUCTION", Label: "PRIMARY_PRODUCTION"},
	}
	want := []ReferenceItem{
		{ID: "1", Label: "Development"},
		{ID: "2", Label: "QA"},
		{ID: "3", Label: "Staging"},
		{ID: "4", Label: "Stress"},
		{ID: "5", Label: "UAT"},
		{ID: "6", Label: "Primary Production"},
	}

	got := normalizeDeploymentTypeChoices(in)
	if len(got) != len(want) {
		t.Fatalf("got %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Label != want[i].Label {
			t.Errorf("[%d] = {%s, %s}, want {%s, %s}", i, got[i].ID, got[i].Label, want[i].ID, want[i].Label)
		}
	}
}

// ServiceNow-mode already returns numeric ids -- an id this table doesn't
// recognise passes through untouched, same as severity/state/issue type.
func TestNormalizeDeploymentTypeChoices_ServiceNowIDsPassThrough(t *testing.T) {
	in := []ReferenceItem{{ID: "3", Label: "Staging"}}
	got := normalizeDeploymentTypeChoices(in)
	if got[0].ID != in[0].ID || got[0].Label != in[0].Label {
		t.Errorf("got {%s, %s}, want it unchanged {%s, %s}", got[0].ID, got[0].Label, in[0].ID, in[0].Label)
	}
}
