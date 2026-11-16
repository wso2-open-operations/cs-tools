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

package repository

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A state bucket's key is what the search's "state" filter takes, so a
// dashboard slice opened with it finds its records; the label stays lower case.
func TestIncidentAggregateBucket_StateKeyIsTheSearchValue(t *testing.T) {
	cases := []struct {
		stored string
		want   domain.IncidentState
		label  string
	}{
		{"NEW", domain.IncidentStateNew, "new"},
		{"IN_PROGRESS", domain.IncidentStateInProgress, "in_progress"},
		{"ON_HOLD", domain.IncidentStateOnHold, "on_hold"},
		// The enum spells it CANCELED; the domain and the filter, CANCELLED.
		{"CANCELED", domain.IncidentStateCancelled, "cancelled"},
	}
	for _, c := range cases {
		got := incidentAggregateBucket("state", c.stored, 3)
		if got.Key != string(c.want) || got.Label != c.label || got.Count != 3 {
			t.Errorf("state %q: got %+v, want key %q label %q count 3", c.stored, got, c.want, c.label)
		}
	}
}

func TestIncidentAggregateBucket_OtherFieldsStayLowerCase(t *testing.T) {
	got := incidentAggregateBucket("businessService", "B9C999F8-1B86-A010-00AE-86ACDD4BCB61", 1)
	want := "b9c999f8-1b86-a010-00ae-86acdd4bcb61"
	if got.Key != want || got.Label != want {
		t.Errorf("got %+v, want key and label %q", got, want)
	}
}

func TestProblemAggregateBucket_StateKeyIsTheSearchValue(t *testing.T) {
	cases := []struct {
		want  domain.ProblemState
		label string
	}{
		{domain.ProblemStateNew, "new"},
		{domain.ProblemStateAssess, "assess"},
		{domain.ProblemStateRootCauseAnalysis, "root_cause_analysis"},
		{domain.ProblemStateFixInProgress, "fix_in_progress"},
	}
	for _, c := range cases {
		got := problemAggregateBucket("state", string(c.want), 2)
		if got.Key != string(c.want) || got.Label != c.label || got.Count != 2 {
			t.Errorf("state %q: got %+v, want key %q label %q count 2", c.want, got, c.want, c.label)
		}
	}
}
