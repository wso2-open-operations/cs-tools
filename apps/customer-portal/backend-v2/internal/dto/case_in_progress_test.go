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
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// Both states customers cannot tell apart come back as the same status, under
// Work In Progress's own id.
func TestCaseStatusRef_InProgressFamilyIsMerged(t *testing.T) {
	for _, state := range []string{"work_in_progress", "waiting_on_wso2", "Work In Progress", "Waiting on WSO2"} {
		got := caseStatusRef(state)
		if got == nil || got.ID != "10" || got.Label != "In Progress" {
			t.Errorf("caseStatusRef(%q) = %+v, want {10, In Progress}", state, got)
		}
	}
	if got := caseStatusRef("awaiting_info"); got == nil || got.Label != "Awaiting Info" || got.ID != "18" {
		t.Errorf("awaiting_info must be untouched, got %+v", got)
	}
}

// A filter on In Progress must also find cases that are waiting on WSO2.
func TestExpandInProgressStateIDs(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want []int
	}{
		{"in progress adds waiting", []int{10, 18}, []int{10, 18, 1003}},
		{"already both", []int{10, 1003}, []int{10, 1003}},
		{"unrelated", []int{18, 3}, []int{18, 3}},
		{"empty", nil, nil},
	}
	for _, c := range cases {
		if got := expandInProgressStateIDs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	in := []int{10}
	expandInProgressStateIDs(in)
	if len(in) != 1 {
		t.Errorf("input slice was modified: %v", in)
	}
}

func stateChange(prev, next string) entity.CaseActivity {
	return entity.CaseActivity{
		ID:        "a1",
		Type:      entity.ActivityTypeFieldChange,
		CreatedOn: time.Unix(0, 0),
		Changes:   []entity.FieldChange{{Field: "state", FieldLabel: "State", PreviousValue: prev, NewValue: next}},
	}
}

// A move between the two merged states is invisible to customers, so it must
// not appear in the feed; a move that crosses the boundary shows the merged
// label.
func TestMapSearchCaseActivities_StateChangesUseMergedLabel(t *testing.T) {
	got := MapSearchCaseActivities(entity.SearchCaseActivitiesResponse{
		Activity: []entity.CaseActivity{
			stateChange("Waiting on WSO2", "Work In Progress"),
			stateChange("Awaiting Info", "Waiting on WSO2"),
			stateChange("Work In Progress", "Closed"),
		},
	})
	if len(got.Activities) != 2 {
		t.Fatalf("got %d activities, want 2 (the in-progress to in-progress move is dropped): %+v", len(got.Activities), got.Activities)
	}
	if c := got.Activities[0].Changes[0]; c.PreviousValue != "Awaiting Info" || c.NewValue != "In Progress" {
		t.Errorf("first change = %+v, want Awaiting Info -> In Progress", c)
	}
	if c := got.Activities[1].Changes[0]; c.PreviousValue != "In Progress" || c.NewValue != "Closed" {
		t.Errorf("second change = %+v, want In Progress -> Closed", c)
	}
}

// Global search (the Project Hub's cases, search and export) carries a case's
// state as entity-service sent it: the raw enum as id and label on Postgres,
// ServiceNow's id and display text there. Both in-progress states come back as
// the merged In Progress; every other state, and a missing one, is untouched.
func TestMapGlobalSearchResponse_InProgressFamilyIsMerged(t *testing.T) {
	inProgress := &ReferenceItem{ID: "10", Label: "In Progress"}
	cases := []struct {
		name  string
		state *entity.ChoiceListItem
		want  *ReferenceItem
	}{
		{"postgres work in progress", &entity.ChoiceListItem{ID: "WORK_IN_PROGRESS", Label: "WORK_IN_PROGRESS"}, inProgress},
		{"postgres waiting on wso2", &entity.ChoiceListItem{ID: "WAITING_ON_WSO2", Label: "WAITING_ON_WSO2"}, inProgress},
		{"servicenow work in progress", &entity.ChoiceListItem{ID: "10", Label: "Work In Progress"}, inProgress},
		{"servicenow waiting on wso2", &entity.ChoiceListItem{ID: "1003", Label: "Waiting on WSO2"}, inProgress},
		{"another state is untouched", &entity.ChoiceListItem{ID: "AWAITING_INFO", Label: "AWAITING_INFO"}, &ReferenceItem{ID: "AWAITING_INFO", Label: "AWAITING_INFO"}},
		{"no state stays none", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapGlobalSearchResponse(entity.GlobalSearchResponse{Cases: []entity.GlobalSearchCase{{ID: "c1", State: tc.state}}})
			if len(got.Cases) != 1 {
				t.Fatalf("cases = %d, want 1", len(got.Cases))
			}
			if !reflect.DeepEqual(got.Cases[0].State, tc.want) {
				t.Errorf("state = %+v, want %+v", got.Cases[0].State, tc.want)
			}
		})
	}
}
