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

import "encoding/json"

const (
	caseStateOpen             = "open"
	caseStateWorkInProgress   = "work_in_progress"
	caseStateWaitingOnWSO2    = "waiting_on_wso2"
	caseStateAwaitingInfo     = "awaiting_info"
	caseStateReopened         = "reopened" // deprecated alias for waiting_on_wso2
	caseStateSolutionProposed = "solution_proposed"
	caseStateClosed           = "closed"
)

// nextStates returns the valid next states reachable from the given case state.
// Role-gated transitions (team-lead override, auto-close) are excluded until
// role enforcement is implemented.
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
	case caseStateReopened:
		return []string{caseStateWaitingOnWSO2}
	case caseStateSolutionProposed:
		return []string{caseStateClosed, caseStateWaitingOnWSO2}
	default: // caseStateClosed and any unknown values are terminal
		return []string{}
	}
}

// injectNextStates parses raw case JSON returned by the entity service, derives
// the valid next states from the "state" field, and returns the JSON with a
// "nextStates" key appended.
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
	ns, err := json.Marshal(nextStates(state))
	if err != nil {
		return nil, err
	}
	m["nextStates"] = ns
	return json.Marshal(m)
}
