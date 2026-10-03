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

// TeamMemberEntry is one person's membership of one team, and the rank they
// hold in it.
//
// Deliberately not ScheduleEngineer: that type answers "who is rostered", is
// read by the Team Schedule page, and carries IsLead because a badge is all
// the page needs. This one answers "what rank does this person hold", which
// the call-escalation ladder resolves every rung from, and a boolean cannot
// carry five values.
type TeamMemberEntry struct {
	TeamKey string `json:"teamKey"`
	// TeamType is the team's own type -- cre-abt, sre-abt, cre,
	// cre-leadership. It is what groups teams into an ABT: cre-abt holds
	// seven teams and sre-abt two, while Americas is type cre and is not an
	// ABT at all. The escalation ladder reads it so "every team in this ABT"
	// is a question about the data rather than a list in a config file that
	// drifts the first time a team is added.
	TeamType string `json:"teamType,omitempty"`
	// Role is the raw team_member.role: engineer, sub_lead, lead, cre_head or
	// cs_head. Passed through unvalidated, the same way the schedule views
	// render state and priority, so a role added by a migration reaches a
	// caller without this service needing to learn about it first.
	Role string `json:"role"`
	// AlertTier is the standing alert-duty nomination (T1/T2/T3), empty when
	// this member holds none. A different axis from Role -- see migration
	// 0171 -- and the input the ladder's first rung resolves from.
	AlertTier string `json:"alertTier,omitempty"`
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}

// TeamMembersResponse is every membership matching a lookup.
//
// Flat rather than grouped by team: the caller asking for two teams at once is
// the escalation resolver, which wants one pass to bucket by role, and a
// grouped shape would make it walk a map to do the same thing.
type TeamMembersResponse struct {
	Members []TeamMemberEntry `json:"members"`
	Count   int               `json:"count"`
}
