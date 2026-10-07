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

const (
	incidentStateNew        = "NEW"
	incidentStateInProgress = "IN_PROGRESS"
	incidentStateOnHold     = "ON_HOLD"
	incidentStateResolved   = "RESOLVED"
	incidentStateClosed     = "CLOSED"
	incidentStateCancelled  = "CANCELLED"
)

// incidentNextStates returns the valid next states reachable from the given
// incident state. This mirrors apps/csm-portal/webapp's own
// csm-operations/utils/incidents.ts STATE_TRANSITIONS exactly — it is the
// server-side enforcement of that same, already-agreed lifecycle, not a new
// graph invented here. Before this existed, PatchIncident had no
// from-state-aware check at all: a direct PATCH could jump straight from
// NEW to CLOSED, since only the target value's enum membership was ever
// validated (see validIncidentStates), never whether the transition from the
// incident's current state was legal. CLOSED and CANCELLED are terminal.
func incidentNextStates(state string) []string {
	switch state {
	case incidentStateNew:
		return []string{incidentStateInProgress, incidentStateCancelled}
	case incidentStateInProgress:
		return []string{incidentStateOnHold, incidentStateResolved, incidentStateCancelled}
	case incidentStateOnHold:
		return []string{incidentStateInProgress, incidentStateCancelled}
	case incidentStateResolved:
		return []string{incidentStateClosed, incidentStateInProgress}
	case incidentStateClosed, incidentStateCancelled:
		return []string{}
	default: // an unset/unrecognized current state is terminal — fail closed, not open
		return []string{}
	}
}

// isValidIncidentStateTransition reports whether transitioning from the
// current incident state to the requested state is permitted by the
// incident state machine. A same-state resend is always legal — unlike
// isValidStateTransition for cases, this needs that explicit case because
// PatchIncident had no transition check at all before this existed, so
// nothing already relies on a client never resending the current value.
func isValidIncidentStateTransition(from, to string) bool {
	if from == to {
		return true
	}
	for _, s := range incidentNextStates(from) {
		if s == to {
			return true
		}
	}
	return false
}
