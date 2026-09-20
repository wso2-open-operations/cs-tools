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

func incidentView(state, priority, subject string) domain.IncidentView {
	v := domain.IncidentView{}
	if state != "" {
		v.State = &state
	}
	if priority != "" {
		v.Priority = &priority
	}
	if subject != "" {
		v.Subject = &subject
	}
	return v
}

// Acknowledgement, per the escalation specification, is the incident leaving
// NEW ("update the ticket status to Work In Progress to stop further
// notifications"). Nothing else cancels a running call ladder.
func TestIncidentStateTransition_OnlyFiresLeavingNew(t *testing.T) {
	tests := []struct {
		name          string
		before, after string
		want          bool
	}{
		{"NEW to IN_PROGRESS acknowledges", "NEW", "IN_PROGRESS", true},
		{"NEW to RESOLVED also cancels", "NEW", "RESOLVED", true},
		{"NEW to CANCELLED also cancels", "NEW", "CANCELLED", true},
		{"no-op re-PATCH of NEW must not cancel", "NEW", "NEW", false},
		{"already in progress, later change is not an acknowledgement", "IN_PROGRESS", "ON_HOLD", false},
		{"resolved to closed is not an acknowledgement", "RESOLVED", "CLOSED", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := incidentStateTransition(incidentView(tc.before, "", ""), incidentView(tc.after, "", ""))
			if ok != tc.want {
				t.Fatalf("transition %s -> %s: got %v, want %v", tc.before, tc.after, ok, tc.want)
			}
		})
	}
}

// A missing baseline must never be read as a transition — that would cancel a
// live escalation on the strength of a failed fetch.
func TestIncidentStateTransition_RequiresBothSides(t *testing.T) {
	if _, _, ok := incidentStateTransition(domain.IncidentView{}, incidentView("IN_PROGRESS", "", "")); ok {
		t.Fatal("a zero-valued baseline must not report a transition")
	}
	if _, _, ok := incidentStateTransition(incidentView("NEW", "", ""), domain.IncidentView{}); ok {
		t.Fatal("a missing post-PATCH state must not report a transition")
	}
}

// The ladder reacts to an incident becoming MORE urgent. A downgrade or a
// no-op must not start a second escalation.
func TestIncidentPriorityElevation_OnlyOnIncreasedUrgency(t *testing.T) {
	tests := []struct {
		name          string
		before, after string
		want          bool
	}{
		{"HIGH to CRITICAL is an elevation", "HIGH", "CRITICAL", true},
		{"PLANNING to CRITICAL is an elevation", "PLANNING", "CRITICAL", true},
		{"MODERATE to HIGH is an elevation", "MODERATE", "HIGH", true},
		{"LOW to MODERATE is an elevation", "LOW", "MODERATE", true},
		{"CRITICAL to HIGH is a downgrade", "CRITICAL", "HIGH", false},
		{"unchanged priority is not an elevation", "CRITICAL", "CRITICAL", false},
		{"unknown old priority is not an elevation", "URGENT", "CRITICAL", false},
		{"unknown new priority is not an elevation", "HIGH", "SEVERE", false},
		{"P-notation is not an incident priority", "P2", "P1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := incidentPriorityElevation(incidentView("", tc.before, ""), incidentView("", tc.after, ""))
			if ok != tc.want {
				t.Fatalf("priority %s -> %s: got %v, want %v", tc.before, tc.after, ok, tc.want)
			}
		})
	}
}

func TestIncidentPriorityElevation_ReturnsBothValues(t *testing.T) {
	oldP, newP, ok := incidentPriorityElevation(incidentView("", "MODERATE", ""), incidentView("", "CRITICAL", ""))
	if !ok {
		t.Fatal("expected an elevation")
	}
	if oldP != "MODERATE" || newP != "CRITICAL" {
		t.Fatalf("got %q -> %q, want MODERATE -> CRITICAL", oldP, newP)
	}
}
