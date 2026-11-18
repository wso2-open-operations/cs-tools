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

import "github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"

// caseStateTransitionsExternal is the legacy ServiceNow scoped-app's own
// ALLOWED_STATE_TRANSITIONS table -- what a customer may do to their own
// case. Deliberately narrower than caseStateTransitionsInternal below: a
// customer's own case-update action is almost always "close it," plus a few
// states that route back through WSO2.
var caseStateTransitionsExternal = map[domain.CaseState][]domain.CaseState{
	domain.CaseStateOpen:             {domain.CaseStateClosed},
	domain.CaseStateWorkInProgress:   {domain.CaseStateClosed},
	domain.CaseStateWaitingOnWSO2:    {domain.CaseStateClosed},
	domain.CaseStateSolutionProposed: {domain.CaseStateClosed, domain.CaseStateReopened, domain.CaseStateWaitingOnWSO2},
	domain.CaseStateAwaitingInfo:     {domain.CaseStateClosed, domain.CaseStateWaitingOnWSO2},
	domain.CaseStateReopened:         {domain.CaseStateClosed},
}

// caseStateTransitionsInternal is the legacy ServiceNow scoped-app's own
// SUPERUSER_STATE_TRANSITIONS table -- what WSO2 staff (an agent/CSM caller)
// may do. reopened -> work_in_progress is this codebase's own addition
// (apps/csm-portal/backend's state.go ports this same table and already
// carries it; the legacy table itself has no entry for reopened at all) --
// confirmed as intended rather than a porting gap: an agent treats a
// reopened case like a fresh open one and picks it back up into active
// work.
var caseStateTransitionsInternal = map[domain.CaseState][]domain.CaseState{
	domain.CaseStateOpen:             {domain.CaseStateWorkInProgress},
	domain.CaseStateWorkInProgress:   {domain.CaseStateWaitingOnWSO2, domain.CaseStateAwaitingInfo, domain.CaseStateSolutionProposed, domain.CaseStateClosed},
	domain.CaseStateAwaitingInfo:     {domain.CaseStateWaitingOnWSO2},
	domain.CaseStateWaitingOnWSO2:    {domain.CaseStateWorkInProgress},
	domain.CaseStateSolutionProposed: {domain.CaseStateClosed, domain.CaseStateWaitingOnWSO2},
	domain.CaseStateReopened:         {domain.CaseStateWorkInProgress},
}

// isValidCaseStateTransition reports whether moving a case-like work item
// from "from" to "to" is legal for the given caller tier. A state with no
// entry in the chosen table (closed, or an unrecognized value) has no legal
// next states -- closed is terminal for every caller, matching how it
// already behaves everywhere else in this codebase. The one caller that may
// still move a case out of closed is a portal admin, via
// UpdateCaseRequest.AdminReopen -- handled as a separate bypass in
// CaseService.UpdateCase, not as an addition to either table below, since it
// is not a generally-available transition.
//
// internal selects which table applies: true for WSO2 staff (Unrestricted
// or HasInternalAccess, the same two-flag test UpdateCase's own
// resolutionCode/cause/closeNotes requirement already uses), false for an
// external/customer caller.
func isValidCaseStateTransition(internal bool, from, to domain.CaseState) bool {
	table := caseStateTransitionsExternal
	if internal {
		table = caseStateTransitionsInternal
	}
	for _, s := range table[from] {
		if s == to {
			return true
		}
	}
	return false
}
