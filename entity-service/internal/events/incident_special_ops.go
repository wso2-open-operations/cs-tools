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

package events

// TypeIncidentSpecialOpsAlert: an incident's assignment group changed to a
// Special Ops team's group (any team in SPECIALIST_HANDOFF_CONFIG), however it
// changed -- the specialist handoff today. Published on the operations topic
// (sre-events) for csm-notification-service, which decides who to page and
// where to post: ServiceNow's "Incident Special Ops Escalation Notification
// Flow" (on-call calls, then Chat cards; discovery scripts 61-68). Keyed by
// the incident id. The struct is duplicated there by hand (separate Go
// modules); keep the two in step.
const TypeIncidentSpecialOpsAlert Type = "incident.special_ops_alert"

// IncidentSpecialOpsAlertPayload is TypeIncidentSpecialOpsAlert's payload.
// The incident fields are as they stand when the alert is published; the
// group fields are the change that raised it.
type IncidentSpecialOpsAlertPayload struct {
	IncidentID string `json:"incidentId"`
	Number     string `json:"number"`
	Subject    string `json:"subject"`
	// Description is the incident's description as stored (may be HTML).
	Description string `json:"description,omitempty"`
	// State, Priority, Impact and Urgency are enum labels: IN_PROGRESS,
	// CRITICAL, HIGH, ...
	State    string `json:"state,omitempty"`
	Priority string `json:"priority,omitempty"`
	Impact   string `json:"impact,omitempty"`
	Urgency  string `json:"urgency,omitempty"`

	ServiceID   string `json:"serviceId,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`

	// Product, TeamKey and TeamLabel name the Special Ops team the group
	// belongs to, from SPECIALIST_HANDOFF_CONFIG: e.g. "Choreo",
	// "choreo-runtime-team", "Choreo Runtime Team".
	Product   string `json:"product"`
	TeamKey   string `json:"teamKey"`
	TeamLabel string `json:"teamLabel"`

	// AssignmentGroupID/Name is the Special Ops group the incident moved to;
	// PreviousAssignmentGroupID/Name the group it left (empty if none).
	AssignmentGroupID           string `json:"assignmentGroupId"`
	AssignmentGroupName         string `json:"assignmentGroupName,omitempty"`
	PreviousAssignmentGroupID   string `json:"previousAssignmentGroupId,omitempty"`
	PreviousAssignmentGroupName string `json:"previousAssignmentGroupName,omitempty"`

	// ChangedBy is who changed the group (work_item.updated_by: an email,
	// or "system" for a background writer); ChangedOn when, RFC3339.
	ChangedBy string `json:"changedBy,omitempty"`
	ChangedOn string `json:"changedOn"`
}
