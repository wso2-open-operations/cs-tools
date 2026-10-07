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

import (
	"strconv"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// The dashboard's outstanding-engagements chart matches engagement type
// buckets on the LABEL, against "Consultancy"/"Onboarding"/"Migration"/
// "Follow Up"/"New Feature Improvement" (features/dashboard/constants/dashboard.ts,
// OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA). The Postgres data source
// returns the raw enum label (UPPER_SNAKE) as both id and label -- without
// normalization every bucket misses that match and silently drops out of
// both the chart and its total, the same class of bug
// TestNormalizeCaseSeverityChoices_PostgresEnumLabels guards for severity.
func TestMapProjectCaseStats_NormalizesEngagementTypeChoices(t *testing.T) {
	resp := entity.ProjectCaseStatsResponse{
		EngagementTypeCount: []entity.ChoiceListItem{
			{ID: "NEW_FEATURE_IMPROVEMENT", Label: "NEW_FEATURE_IMPROVEMENT", Count: intPtr(2)},
		},
		OutstandingEngagementTypeCount: []entity.ChoiceListItem{
			{ID: "FOLLOW_UP", Label: "FOLLOW_UP", Count: intPtr(3)},
		},
	}

	got := MapProjectCaseStats(resp)

	if len(got.EngagementTypeCount) != 1 || got.EngagementTypeCount[0].Label != "New Feature Improvement" {
		t.Fatalf("EngagementTypeCount not normalized: %+v", got.EngagementTypeCount)
	}
	if len(got.OutstandingEngagementTypeCount) != 1 || got.OutstandingEngagementTypeCount[0].Label != "Follow Up" {
		t.Fatalf("OutstandingEngagementTypeCount not normalized: %+v", got.OutstandingEngagementTypeCount)
	}
}

// GET /projects/{id}/filters' changeRequestStates/changeRequestImpacts fed
// ChangeRequestsPage.tsx's State/Impact filter dropdowns directly. On the
// Postgres data source these carried the raw enum label as id
// (e.g. {"id":"ROLLBACK"}), never run through the normalizer every sibling
// field on this same response already uses -- so filters.stateIds?.map(Number)
// converted every selection to NaN, which reached the search request as
// null instead of a real state key. Also verifies New and Assess are excluded
// from the list (no change request a customer can see is ever in either) while
// Authorize is offered: a change request the customer proposed a new time for
// waits there, so the filter has to be able to name it.
func TestMapProjectFilterOptions_NormalizesChangeRequestChoicesAndOffersAuthorize(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ChangeRequestStates: []entity.ChoiceListItem{
			{ID: "NEW", Label: "NEW"},
			{ID: "ASSESS", Label: "ASSESS"},
			{ID: "AUTHORIZE", Label: "AUTHORIZE"},
			{ID: "ROLLBACK", Label: "ROLLBACK"},
			{ID: "CLOSED", Label: "CLOSED"},
		},
		ChangeRequestImpacts: []entity.ChoiceListItem{
			{ID: "HIGH", Label: "HIGH"},
		},
	}

	got := MapProjectFilterOptions(resp)

	want := []struct{ id, label string }{{"-3", "Authorize"}, {"2", "Rollback"}, {"3", "Closed"}}
	if len(got.ChangeRequestStates) != len(want) {
		t.Fatalf("ChangeRequestStates = %+v, want Authorize, Rollback and Closed (New/Assess excluded)", got.ChangeRequestStates)
	}
	for i, w := range want {
		if got.ChangeRequestStates[i].ID != w.id || got.ChangeRequestStates[i].Label != w.label {
			t.Errorf("ChangeRequestStates[%d] = %+v, want {ID: %q, Label: %q}", i, got.ChangeRequestStates[i], w.id, w.label)
		}
	}
	if len(got.ChangeRequestImpacts) != 1 || got.ChangeRequestImpacts[0].ID != "1" || got.ChangeRequestImpacts[0].Label != "High" {
		t.Errorf("ChangeRequestImpacts = %+v, want [{ID: \"1\", Label: \"High\"}]", got.ChangeRequestImpacts)
	}
}

// GET /projects/{id}/filters' conversationStates fed AllConversationsPage.tsx's
// State filter dropdown directly. On the Postgres data source these carried
// the raw enum label as id (e.g. {"id":"ACTIVE"}, {"id":"CLOSE"} -- the real
// Postgres label for the closed state has no D), never run through the
// normalizer every sibling choice list on this same response already uses --
// so filters.stateId ? [Number(filters.stateId)] : undefined converted every
// selection to NaN, which conversationIDsToEnums then refused as an unmapped
// state id. The page never recovered: its own loading flag only clears once
// the search query has a successful response, so every selection left it
// stuck showing its loader (digiops-cs#3273).
func TestMapProjectFilterOptions_NormalizesConversationStateChoices(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ConversationStates: []entity.ChoiceListItem{
			{ID: "OPEN", Label: "OPEN"},
			{ID: "ACTIVE", Label: "ACTIVE"},
			{ID: "RESOLVED", Label: "RESOLVED"},
			{ID: "CONVERTED", Label: "CONVERTED"},
			{ID: "ABANDONED", Label: "ABANDONED"},
			{ID: "CLOSE", Label: "CLOSE"},
		},
	}

	got := MapProjectFilterOptions(resp)

	want := []ReferenceItem{
		{ID: "1", Label: "Open"},
		{ID: "2", Label: "Active"},
		{ID: "3", Label: "Resolved"},
		{ID: "4", Label: "Converted"},
		{ID: "5", Label: "Abandoned"},
		{ID: "6", Label: "Closed"},
	}
	if len(got.ConversationStates) != len(want) {
		t.Fatalf("ConversationStates = %+v, want %+v", got.ConversationStates, want)
	}
	for i, w := range want {
		if got.ConversationStates[i].ID != w.ID || got.ConversationStates[i].Label != w.Label {
			t.Errorf("ConversationStates[%d] = %+v, want %+v", i, got.ConversationStates[i], w)
		}
	}
}

// An id this normalizer does not recognise (ServiceNow's own numeric choice
// key, already what the frontend wants) must pass through unchanged rather
// than being dropped or rewritten.
func TestMapProjectFilterOptions_ConversationStateUnrecognisedIDPassesThrough(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ConversationStates: []entity.ChoiceListItem{
			{ID: "7", Label: "Some Future State"},
		},
	}

	got := MapProjectFilterOptions(resp)

	if len(got.ConversationStates) != 1 || got.ConversationStates[0].ID != "7" || got.ConversationStates[0].Label != "Some Future State" {
		t.Errorf("ConversationStates = %+v, want [{ID: \"7\", Label: \"Some Future State\"}] unchanged", got.ConversationStates)
	}
}

// The ServiceNow data source's own vocabulary carries all eleven states under real
// numeric ids with display-cased labels. A customer there has no designated change
// requests (nothing is asked of them through entity-service) so the legacy rule
// applies: every state except New, Assess and Authorize. Offering "-3" would have
// the webapp ask for Authorize, and ServiceNow would answer with the project's
// internal pre-approval change requests.
func TestMapProjectFilterOptions_ServiceNowOffersNeitherNewNorAssessNorAuthorize(t *testing.T) {
	resp := entity.ProjectMetadataResponse{ChangeRequestStates: serviceNowChangeRequestStates()}

	got := MapProjectFilterOptions(resp)

	var ids []string
	for _, s := range got.ChangeRequestStates {
		ids = append(ids, s.ID+"="+s.Label)
	}
	want := "5=Customer Approval,-2=Scheduled,-1=Implement,0=Review,1=Customer Review,2=Rollback,3=Closed,4=Canceled"
	if strings.Join(ids, ",") != want {
		t.Fatalf("ChangeRequestStates = %v, want exactly the eight states past Authorize: %s", ids, want)
	}
}

// The Postgres data source names the same states by raw enum label (never a
// numeric id), and there Authorize IS offered: a change request designated to the
// customer waits in it after the customer proposed a new time, and stays on their
// list. New and Assess are left out under their raw labels.
func TestMapProjectFilterOptions_PostgresKeepsAuthorizeAndDropsNewAndAssess(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ChangeRequestStates: []entity.ChoiceListItem{
			{ID: "NEW", Label: "NEW"},
			{ID: "ASSESS", Label: "ASSESS"},
			{ID: "AUTHORIZE", Label: "AUTHORIZE"},
			{ID: "CLOSED", Label: "CLOSED"},
		},
	}

	got := MapProjectFilterOptions(resp)

	var labels []string
	for _, s := range got.ChangeRequestStates {
		labels = append(labels, s.ID+"="+s.Label)
	}
	if strings.Join(labels, ",") != "-3=Authorize,3=Closed" {
		t.Fatalf("ChangeRequestStates = %v, want [-3=Authorize 3=Closed]", labels)
	}
}

// What the webapp sends is exactly the ids the filters offered it (it asks for
// every offered state when no filter is chosen). From a ServiceNow-sourced
// vocabulary that must never name New, Assess or Authorize, which is the
// request the customer portal used to build itself and now builds from this.
func TestChangeRequestSearch_AWebappThatAsksForEveryOfferedStateNeverNamesAHiddenOneOnServiceNow(t *testing.T) {
	offered := MapProjectFilterOptions(entity.ProjectMetadataResponse{ChangeRequestStates: serviceNowChangeRequestStates()}).ChangeRequestStates

	var keys []int
	for _, s := range offered {
		n, err := strconv.Atoi(s.ID)
		if err != nil {
			t.Fatalf("an offered state id %q is not a number", s.ID)
		}
		keys = append(keys, n)
	}
	got := BuildEntitySearchChangeRequestsRequest("proj-9", ChangeRequestSearchRequest{Filters: ChangeRequestSearchFilters{StateKeys: keys}})

	for _, st := range got.Filters.States {
		switch st {
		case "new", "assess", "authorize":
			t.Fatalf("the search names %q, a state a customer on the ServiceNow data source is never shown (states = %v)", st, got.Filters.States)
		}
	}
	if len(got.Filters.States) != 8 {
		t.Fatalf("states = %v, want the eight offered", got.Filters.States)
	}
}

// serviceNowChangeRequestStates is ServiceNow's own change request state
// vocabulary, as GET /projects/{id}/metadata carries it: numeric ids, display labels.
func serviceNowChangeRequestStates() []entity.ChoiceListItem {
	return []entity.ChoiceListItem{
		{ID: "-5", Label: "New"},
		{ID: "-4", Label: "Assess"},
		{ID: "-3", Label: "Authorize"},
		{ID: "5", Label: "Customer Approval"},
		{ID: "-2", Label: "Scheduled"},
		{ID: "-1", Label: "Implement"},
		{ID: "0", Label: "Review"},
		{ID: "1", Label: "Customer Review"},
		{ID: "2", Label: "Rollback"},
		{ID: "3", Label: "Closed"},
		{ID: "4", Label: "Canceled"},
	}
}

// TestMapProjectFilterOptions_ExposesResolutionCodesAndCauses is the
// regression test for a real, reported bug: closing a case requires
// resolutionCode/cause/closeNotes, but the webapp had no choice lists to
// build a close dialog from at all -- GET /projects/{id}/filters simply
// never carried either field. Confirms both now pass through unchanged.
func TestMapProjectFilterOptions_ExposesResolutionCodesAndCauses(t *testing.T) {
	resp := entity.ProjectMetadataResponse{
		ResolutionCodes: []entity.ChoiceListItem{{ID: "SOLVED_WORKAROUND_PROVIDED", Label: "Solved Workaround Provided"}},
		Causes:          []entity.ChoiceListItem{{ID: "PRODUCT_BUG", Label: "Product Bug"}},
	}

	got := MapProjectFilterOptions(resp)

	if len(got.ResolutionCodes) != 1 || got.ResolutionCodes[0].ID != "SOLVED_WORKAROUND_PROVIDED" {
		t.Fatalf("ResolutionCodes = %+v", got.ResolutionCodes)
	}
	if len(got.Causes) != 1 || got.Causes[0].ID != "PRODUCT_BUG" {
		t.Fatalf("Causes = %+v", got.Causes)
	}
}

// GET /projects/{id}/stats/change-requests feeds the Operations page's
// Upcoming Changes / Action Required Changes cards, which find their counts by
// display label ("Scheduled", "Customer Approval", "Customer Review"). The
// Postgres data source returns the raw enum as both id and label, so without
// normalization "SCHEDULED" never matched and the card showed "--" while the
// change-request list beside it showed four Scheduled changes.
func TestMapProjectChangeRequestStats_NormalizesStateCountLabels(t *testing.T) {
	four, zero := 4, 0
	resp := entity.ProjectChangeRequestStatsResponse{
		TotalCount: 134,
		StateCount: []entity.ChoiceListItem{
			{ID: "CUSTOMER_APPROVAL", Label: "CUSTOMER_APPROVAL", Count: &zero},
			{ID: "SCHEDULED", Label: "SCHEDULED", Count: &four},
			{ID: "CUSTOMER_REVIEW", Label: "CUSTOMER_REVIEW", Count: &zero},
		},
	}

	got := MapProjectChangeRequestStats(resp)

	want := []struct{ id, label string }{
		{"5", "Customer Approval"},
		{"-2", "Scheduled"},
		{"1", "Customer Review"},
	}
	if len(got.StateCount) != len(want) {
		t.Fatalf("StateCount = %+v, want %d entries", got.StateCount, len(want))
	}
	for i, w := range want {
		if got.StateCount[i].ID != w.id || got.StateCount[i].Label != w.label {
			t.Errorf("StateCount[%d] = {%q, %q}, want {%q, %q}",
				i, got.StateCount[i].ID, got.StateCount[i].Label, w.id, w.label)
		}
	}
	if got.StateCount[1].Count == nil || *got.StateCount[1].Count != 4 {
		t.Errorf("Scheduled count = %v, want 4 (counts must survive normalization)", got.StateCount[1].Count)
	}
}

// On the Postgres data source a change request the customer proposed a new time for
// waits in Authorize, so the stats must be able to count it under a state the webapp
// recognises: {id: "-3", label: "Authorize"} (the webapp files that id under
// "Ongoing"). New and Assess are two rows of 0 under raw ids no screen names -- no
// change request a customer can see is ever in either -- so they are not sent. The
// totals pass through as entity-service computed them.
func TestMapProjectChangeRequestStats_CountsAuthorizeAndDropsNewAndAssess(t *testing.T) {
	one, zero := 1, 0
	resp := entity.ProjectChangeRequestStatsResponse{
		TotalCount:       3,
		OutstandingCount: 2,
		StateCount: []entity.ChoiceListItem{
			{ID: "NEW", Label: "NEW", Count: &zero},
			{ID: "ASSESS", Label: "ASSESS", Count: &zero},
			{ID: "AUTHORIZE", Label: "AUTHORIZE", Count: &one},
			{ID: "CUSTOMER_APPROVAL", Label: "CUSTOMER_APPROVAL", Count: &one},
		},
	}

	got := MapProjectChangeRequestStats(resp)

	var rows []string
	for _, s := range got.StateCount {
		c := -1
		if s.Count != nil {
			c = *s.Count
		}
		rows = append(rows, s.ID+"|"+s.Label+"|"+strings.Repeat("#", c))
	}
	if strings.Join(rows, ",") != "-3|Authorize|#,5|Customer Approval|#" {
		t.Errorf("StateCount rows = %v, want Authorize and Customer Approval only", rows)
	}
	if got.TotalCount != 3 || got.OutstandingCount != 2 {
		t.Errorf("totals = %d / %d, want them passed through (3 / 2)", got.TotalCount, got.OutstandingCount)
	}
}

// On the ServiceNow data source the three counts of states a customer is never shown
// (New, Assess, Authorize) are not sent either: a state row of "Authorize" would be
// filed under the webapp's "Ongoing" card and tell the customer how many internal
// pre-approval change requests their project has. The totals are ServiceNow's own and
// pass through.
func TestMapProjectChangeRequestStats_ServiceNowDropsNewAssessAndAuthorizeCounts(t *testing.T) {
	two := 2
	var counts []entity.ChoiceListItem
	for _, s := range serviceNowChangeRequestStates() {
		counts = append(counts, entity.ChoiceListItem{ID: s.ID, Label: s.Label, Count: &two})
	}
	got := MapProjectChangeRequestStats(entity.ProjectChangeRequestStatsResponse{TotalCount: 22, StateCount: counts})

	var rows []string
	for _, s := range got.StateCount {
		rows = append(rows, s.ID+"="+s.Label)
	}
	want := "5=Customer Approval,-2=Scheduled,-1=Implement,0=Review,1=Customer Review,2=Rollback,3=Closed,4=Canceled"
	if strings.Join(rows, ",") != want {
		t.Fatalf("StateCount rows = %v, want the eight states past Authorize: %s", rows, want)
	}
	if got.TotalCount != 22 {
		t.Errorf("TotalCount = %d, want ServiceNow's own figure passed through (22)", got.TotalCount)
	}
}
