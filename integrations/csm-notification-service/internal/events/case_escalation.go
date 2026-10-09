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

// TypeCaseEscalated: a case was escalated (to EL1..EL5). Published on the
// case-events topic by entity-service once the escalation commits, and
// emailed here to the escalation's notification list -- the people
// ServiceNow's EscalationNotificationUtils resolves for the level, already
// resolved and carried as Recipients. Ports ServiceNow's "Internal
// Escalation notification" flow; like it, a de-escalation sends nothing and
// is never published. Kept in sync by hand with
// entity-service's internal/events/case_escalation.go (separate Go modules);
// that file is the source of truth for the wire shape. EntityID is the case
// id.
const TypeCaseEscalated Type = "case.escalated"

// CaseEscalatedPayload is TypeCaseEscalated's payload: what ServiceNow's
// email shows (discovery script 82), plus the recipients.
type CaseEscalatedPayload struct {
	CaseID     string `json:"caseId"`
	CaseNumber string `json:"caseNumber"`
	CaseTitle  string `json:"caseTitle"`
	// Severity is the raw uppercase severity (e.g. "MEDIUM"), shown as SN's
	// priority label ("Medium (P3)").
	Severity    string `json:"severity,omitempty"`
	AccountName string `json:"accountName,omitempty"`
	// Product is the deployed product's display name, e.g. "WSO2 Identity
	// Server 7.1.0".
	Product string `json:"product,omitempty"`
	// Environment is the case's deployment name (SN's u_enviroment), e.g.
	// "Production".
	Environment           string `json:"environment,omitempty"`
	AssignedEngineerEmail string `json:"assignedEngineerEmail,omitempty"`

	EscalationID string `json:"escalationId"`
	// PreviousLevel/CurrentLevel are the level the case left (0..4) and the
	// level it is now at (1..5).
	PreviousLevel int    `json:"previousLevel"`
	CurrentLevel  int    `json:"currentLevel"`
	Reason        string `json:"reason,omitempty"`
	// ActorEmail is who escalated (SN shows the record's created_by).
	ActorEmail string `json:"actorEmail"`
	// EscalatedOn is when the escalation was recorded, RFC3339.
	EscalatedOn string `json:"escalatedOn"`
	// Recipients are the escalation's notification list as emails,
	// de-duplicated. Never empty -- an escalation that resolved nobody is not
	// published.
	Recipients []string `json:"recipients"`
}
