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

package handler

import "testing"

// TestIsValidIncidentStateTransition pins the exact graph against
// apps/csm-portal/webapp's own csm-operations/utils/incidents.ts
// STATE_TRANSITIONS, so the two can't silently drift apart.
func TestIsValidIncidentStateTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{incidentStateNew, incidentStateInProgress, true},
		{incidentStateNew, incidentStateCancelled, true},
		{incidentStateNew, incidentStateResolved, false},
		{incidentStateNew, incidentStateClosed, false}, // the exact bug this closes
		{incidentStateInProgress, incidentStateOnHold, true},
		{incidentStateInProgress, incidentStateResolved, true},
		{incidentStateInProgress, incidentStateCancelled, true},
		{incidentStateInProgress, incidentStateClosed, false},
		{incidentStateOnHold, incidentStateInProgress, true},
		{incidentStateOnHold, incidentStateCancelled, true},
		{incidentStateOnHold, incidentStateResolved, false},
		{incidentStateResolved, incidentStateClosed, true},
		{incidentStateResolved, incidentStateInProgress, true},
		{incidentStateResolved, incidentStateOnHold, false},
		{incidentStateClosed, incidentStateInProgress, false},
		{incidentStateCancelled, incidentStateInProgress, false},
		// Terminal states and an unknown/unset current state are fail-closed,
		// not wildcard-legal.
		{"", incidentStateInProgress, false},
		{"UNKNOWN", incidentStateInProgress, false},
		// A same-state resend is always legal.
		{incidentStateNew, incidentStateNew, true},
		{incidentStateClosed, incidentStateClosed, true},
	}
	for _, tc := range tests {
		if got := isValidIncidentStateTransition(tc.from, tc.to); got != tc.want {
			t.Errorf("isValidIncidentStateTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
