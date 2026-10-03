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

import "time"

// The port of ServiceNow's `Outage Communication` flow — the SRE-facing pair
// of emails that announce an outage and then announce its resolution.
//
// *** THIS IS A DIFFERENT FLOW FROM THE INTERNAL STAKEHOLDER NOTIFIER. ***
// That one (outage_notification.go) mails a configured list three
// near-identical sentences and tracks its own phase column. This one mails a
// group, writes a communication-log row per send, and carries real content.
// They share a table and nothing else.

// OutageCommunicationKind is which of the two emails is owed.
type OutageCommunicationKind string

const (
	// OutageCommunicationNone means nothing is owed right now, which is the
	// steady state for almost every outage.
	OutageCommunicationNone OutageCommunicationKind = "NONE"
	// OutageCommunicationDeclared is the first email: the outage has begun.
	OutageCommunicationDeclared OutageCommunicationKind = "DECLARED"
	// OutageCommunicationResolved is the final email: the outage has ended.
	OutageCommunicationResolved OutageCommunicationKind = "RESOLVED"
)

// OutageForCommunication is one outage as this flow needs to see it.
type OutageForCommunication struct {
	OutageID string `json:"outageId"`
	Number   string `json:"number"`

	// Type is the whole scope rule for the declaration arm, and the single
	// most surprising thing about this flow. ServiceNow's step 4 requires
	// `type=outage` — a DEGRADATION or PLANNED outage clears the wait and
	// then fails the branch, so nothing is ever sent for it. The panel label
	// "Declare Outage" hides that completely.
	Type string `json:"type"`

	ShortDescription string `json:"shortDescription,omitempty"`

	// Impact and State are outage.impact and outage.state, mirrored from
	// cmdb_ci_outage's impact and state -- the "Impact:" and "Current
	// Status:" lines of ServiceNow's email. Blank when ServiceNow left them
	// blank. Do not source them from a nearby column: an earlier revision
	// filled Impact from `message`, a plausible-looking wrong value.
	Impact  string     `json:"impact,omitempty"`
	State   string     `json:"state,omitempty"`
	StartOn *time.Time `json:"startOn,omitempty"`
	EndOn   *time.Time `json:"endOn,omitempty"`
	// DurationSeconds is whole seconds, because the alternative is what
	// Postgres prints for an interval: 00:31:25.634362 -- six decimal
	// places of microseconds. ServiceNow renders a glide_duration as a
	// human string, so the port formats too; see formatOutageDuration.
	DurationSeconds int64 `json:"durationSeconds,omitempty"`

	// OptedIn is ServiceNow's u_outage_communication. It gates both waits and
	// is NEVER written back by the flow — a human sets it once and it stays
	// set. It is therefore an opt-in, not a state marker, and cannot serve as
	// this port's idempotency guard. See AlreadyDeclared.
	OptedIn bool `json:"optedIn"`

	// AlreadyDeclared and AlreadyResolved come from the communication log,
	// matched on the NORMALISED type so that the "Declare"/"Declared" split
	// in the historical data cannot defeat them.
	//
	// *** THIS PAIR IS THE IDEMPOTENCY GUARD. *** ServiceNow's flow is
	// "Run Trigger: Once", so the platform itself guarantees a single
	// delivery and the flow writes no state to the outage at all. A sweep
	// has no such guarantee, and these two booleans reconstruct it from
	// state the flow was already recording.
	AlreadyDeclared bool `json:"alreadyDeclared"`
	AlreadyResolved bool `json:"alreadyResolved"`

	// DeclaredSubject is the subject line of the declaration email, read back
	// from this outage's own log row.
	//
	// ServiceNow ferries it through a field on the INCIDENT: step 8 writes
	// the subject there and step 12 reads it back, purely so the two mails
	// thread. The port keeps the behaviour and drops the mechanism — the
	// subject is already stored on the log row, so no incident column and no
	// incident write are needed.
	DeclaredSubject string `json:"declaredSubject,omitempty"`
}

// OutageCommunicationDecision is one outage's evaluation.
type OutageCommunicationDecision struct {
	OutageID string                  `json:"outageId"`
	Number   string                  `json:"number"`
	Kind     OutageCommunicationKind `json:"kind"`
	// Reason names the branch, in the ServiceNow flow's own words, so an
	// operator reading a sweep result can find the step it came from.
	Reason  string `json:"reason"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// OutageCommunicationSweepResponse is what a sweep returns.
type OutageCommunicationSweepResponse struct {
	Evaluated int                           `json:"evaluated"`
	Decisions []OutageCommunicationDecision `json:"decisions"`
}

// OutageCommunicationLogEntry is one row of the send log — both what this
// service wrote and what ServiceNow wrote before cutover.
type OutageCommunicationLogEntry struct {
	ID            string    `json:"id"`
	OutageNumber  string    `json:"outageNumber"`
	EmailType     string    `json:"emailType,omitempty"`
	EmailTypeNorm string    `json:"emailTypeNorm,omitempty"`
	OutageStatus  string    `json:"outageStatus,omitempty"`
	Subject       string    `json:"subject,omitempty"`
	Recipients    string    `json:"recipients,omitempty"`
	MainContent   string    `json:"mainContent,omitempty"`
	SentOn        time.Time `json:"sentOn"`
}

// RecordOutageCommunicationRequest logs one sent email.
type RecordOutageCommunicationRequest struct {
	OutageNumber string `json:"outageNumber"`
	EmailType    string `json:"emailType"`
	OutageStatus string `json:"outageStatus"`
	Subject      string `json:"subject"`
	Recipients   string `json:"recipients"`
	EmailContent string `json:"emailContent"`
	MainContent  string `json:"mainContent"`
}
