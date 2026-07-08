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

import (
	"encoding/json"
	"time"
)

// relatedCaseWindow mirrors the upstream data source's own eligibility rule:
// a new case may only be linked as related to a case that was closed within
// this window.
const relatedCaseWindow = 60 * 24 * time.Hour

// caseTypeCase is the standard support case type — as opposed to
// service_request, security_report_analysis, or engagement. Related-case
// creation is only offered for this type; the other types have their own
// create flows that don't accept a relatedCaseId today.
const caseTypeCase = "case"

const (
	caseStateOpen             = "open"
	caseStateWorkInProgress   = "work_in_progress"
	caseStateWaitingOnWSO2    = "waiting_on_wso2"
	caseStateAwaitingInfo     = "awaiting_info"
	caseStateSolutionProposed = "solution_proposed"
	caseStateClosed           = "closed"
	caseStateReopened         = "reopened"
)

// nextStates returns the valid next states reachable from the given case state.
// Role-gated transitions (team-lead override, auto-close) are excluded until
// role enforcement is implemented. Closed is terminal — the upstream data
// source rejects any outbound transition from a closed case, so no next
// states are advertised for it here. The one exception is surfaced by the
// caller (injectNextStates): `caseStateReopened` is reused as a signal that
// this closed case is still within its related-case window, since the
// frontend has no real reopen to offer, only "create a related case".
func nextStates(state string) []string {
	switch state {
	case caseStateOpen:
		return []string{caseStateWorkInProgress}
	case caseStateWorkInProgress:
		return []string{caseStateWaitingOnWSO2, caseStateAwaitingInfo, caseStateSolutionProposed, caseStateClosed}
	case caseStateWaitingOnWSO2:
		return []string{caseStateWorkInProgress}
	case caseStateAwaitingInfo:
		return []string{caseStateWaitingOnWSO2}
	case caseStateSolutionProposed:
		return []string{caseStateClosed, caseStateWaitingOnWSO2}
	case caseStateClosed:
		return []string{}
	case caseStateReopened:
		return []string{caseStateWorkInProgress}
	default: // unknown values are terminal
		return []string{}
	}
}

// isValidStateTransition reports whether transitioning from the current state to
// the requested state is permitted by the case state machine.
func isValidStateTransition(from, to string) bool {
	for _, s := range nextStates(from) {
		if s == to {
			return true
		}
	}
	return false
}

// canCreateRelatedCase reports whether a new case may be created as related to
// a case of the given type/state, closed at the given timestamp (RFC 3339, as
// returned in the "closedOn" field; empty when the case was never closed).
// The upstream data source rejects the link once a closed case falls outside
// relatedCaseWindow, so this mirrors that rule to avoid advertising an action
// that would fail server-side. Scoped to caseType == "case" — the other case
// types' create flows have no related-case field yet.
func canCreateRelatedCase(caseType, state, closedOn string) bool {
	if caseType != caseTypeCase || state != caseStateClosed || closedOn == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, closedOn)
	if err != nil {
		return false
	}
	return time.Since(t) <= relatedCaseWindow
}

// injectNextStates parses raw case JSON returned by the entity service,
// derives the valid next states from "state", and returns the JSON with a
// "nextStates" key appended. For a closed case still within its related-case
// window (see canCreateRelatedCase), "reopened" is included as the sole
// entry — there is no separate eligibility field; the frontend renders that
// entry as "Create related case" rather than an actual reopen, since a real
// reopen is never valid.
func injectNextStates(data []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	var state string
	if raw, ok := m["state"]; ok {
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
	}
	var caseType string
	if raw, ok := m["type"]; ok {
		// Best-effort: a null/absent type just leaves it empty, which
		// canCreateRelatedCase treats as ineligible.
		_ = json.Unmarshal(raw, &caseType)
	}
	var closedOn string
	if raw, ok := m["closedOn"]; ok {
		// Best-effort: a null or malformed value just leaves closedOn empty,
		// which canCreateRelatedCase treats as "not closed".
		_ = json.Unmarshal(raw, &closedOn)
	}
	ns := nextStates(state)
	if canCreateRelatedCase(caseType, state, closedOn) {
		ns = []string{caseStateReopened}
	}
	nsJSON, err := json.Marshal(ns)
	if err != nil {
		return nil, err
	}
	m["nextStates"] = nsJSON
	return json.Marshal(m)
}
