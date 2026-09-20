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

package service

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestIncidentStateToEnum locks in the one deliberate mismatch between
// domain.IncidentState and incident_state_enum's real labels: "cancelled"
// is stored as 'CANCELED' (one L), not domain's "CANCELLED" (two Ls).
// Every other state matches by identity.
func TestIncidentStateToEnum(t *testing.T) {
	tests := []struct {
		state domain.IncidentState
		want  string
	}{
		{domain.IncidentStateNew, "NEW"},
		{domain.IncidentStateInProgress, "IN_PROGRESS"},
		{domain.IncidentStateOnHold, "ON_HOLD"},
		{domain.IncidentStateResolved, "RESOLVED"},
		{domain.IncidentStateClosed, "CLOSED"},
		{domain.IncidentStateCancelled, "CANCELED"},
	}
	for _, tt := range tests {
		if got := incidentStateToEnum(tt.state); got != tt.want {
			t.Errorf("incidentStateToEnum(%q) = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestIncidentPriorityToEnum locks in that incident_priority_enum has no
// 'PLANNING' label at all (only CRITICAL/HIGH/MODERATE/LOW) -- a caller
// filtering by IncidentPriorityPlanning must be rejected, not silently cast
// into an invalid enum value.
func TestIncidentPriorityToEnum(t *testing.T) {
	tests := []struct {
		priority domain.IncidentPriority
		want     string
		wantOk   bool
	}{
		{domain.IncidentPriorityCritical, "CRITICAL", true},
		{domain.IncidentPriorityHigh, "HIGH", true},
		{domain.IncidentPriorityModerate, "MODERATE", true},
		{domain.IncidentPriorityLow, "LOW", true},
		{domain.IncidentPriorityPlanning, "", false},
	}
	for _, tt := range tests {
		got, ok := incidentPriorityToEnum(tt.priority)
		if ok != tt.wantOk || (ok && got != tt.want) {
			t.Errorf("incidentPriorityToEnum(%q) = (%q, %v), want (%q, %v)", tt.priority, got, ok, tt.want, tt.wantOk)
		}
	}
}
