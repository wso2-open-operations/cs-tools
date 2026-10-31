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

package domain

// IncidentAutocloseRequest is POST /incidents/autoclose's body: close the
// incidents that have been RESOLVED for at least ResolvedForDays days, the
// way ServiceNow's hourly auto-close did (14 days after resolved_at, closed_by
// = resolved_by, no comment or work note).
type IncidentAutocloseRequest struct {
	// ResolvedForDays is required and at least 1. An incident is due when its
	// resolved_on (its last update when no resolved_on is recorded) is at
	// least this many days ago.
	ResolvedForDays *int `json:"resolvedForDays"`
	// Limit caps how many incidents one call selects, oldest first. Optional:
	// 0 or absent means 200; at most 1000.
	Limit int `json:"limit,omitempty"`
	// DryRun selects what is due and returns it as "closed" without changing
	// anything.
	DryRun bool `json:"dryRun,omitempty"`
}

// IncidentAutocloseClosed is one incident a call closed (or, in a dry run,
// would close).
type IncidentAutocloseClosed struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	// ResolvedOn is the stored resolved_on (RFC 3339, UTC), null when the
	// incident has none and was due by its last update instead.
	ResolvedOn *string `json:"resolvedOn"`
}

// IncidentAutocloseFailed is one due incident a call could not close. It is
// left RESOLVED and selected again by the next call.
type IncidentAutocloseFailed struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Error  string `json:"error"`
}

// IncidentAutocloseResponse is what POST /incidents/autoclose did. More is
// true when the call selected Limit incidents, so more may be due: the
// caller calls again. In a dry run nothing changes, so calling again returns
// the same page.
type IncidentAutocloseResponse struct {
	DryRun bool                      `json:"dryRun"`
	Closed []IncidentAutocloseClosed `json:"closed"`
	Failed []IncidentAutocloseFailed `json:"failed"`
	More   bool                      `json:"more"`
}
