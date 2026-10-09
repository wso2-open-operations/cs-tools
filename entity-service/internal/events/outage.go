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

// The two outage emails, published on the outage-events topic by
// entity-service's outage notice drainer and sent by csm-notification-service.
// Kept in sync by hand with csm-notification-service's internal/events copy.
const (
	// TypeOutageNotificationDue is the internal-stakeholder notification
	// (ServiceNow "Internal Stakeholders Email Notification"): Declared,
	// Update or Resolved, for an outage with notify_internal_stakeholders set.
	TypeOutageNotificationDue Type = "outage.notification_due"
	// TypeOutageCommunicationDue is the SRE declaration/resolution pair
	// (ServiceNow "Outage Communication"), for an outage with
	// outage_communication set.
	TypeOutageCommunicationDue Type = "outage.communication_due"
)

// OutageNoticePayload carries one outage email, already decided and worded.
//
// entity-service owns the decision and the wording (Subject, Body) so both stay
// testable against the ServiceNow originals in one place; the consumer only
// wraps Body in its HTML shell, adds the portal link and sends. The two event
// types share this shape and differ only in which shell wraps it.
type OutageNoticePayload struct {
	OutageID string `json:"outageId"`
	Number   string `json:"number"`
	// Kind is DECLARED, UPDATE or RESOLVED.
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	// Body is plain text; the consumer escapes it.
	Body string `json:"body"`
	// Recipients are the configured audience for this email. Never empty: the
	// drainer does not sweep a flow it has nobody to send to.
	Recipients []string `json:"recipients"`
}

// TypeOutageStatusPageDue asks for one cloud status webhook to the public
// status dashboard: an outage began or ended on a cloud the page shows.
// Published on the operations topic (sre-events) the moment the outage is
// written, and posted by csm-notification-service -- which reports the
// outcome back (POST /internal/cloud-status/{id}/delivery). csm-scheduled-tasks
// remains the retry path for a post that fails.
const TypeOutageStatusPageDue Type = "outage.status_page_due"

// OutageStatusPageDuePayload is one webhook, already decided: the consumer
// posts Event, Timestamp and Cloud verbatim as the dashboard's body
// ({"event","timestamp","cloud"}) and reports the result under WebhookID.
type OutageStatusPageDuePayload struct {
	// WebhookID is the cloud_status_events row; the delivery is recorded
	// against it.
	WebhookID string `json:"webhookId"`
	// ClaimToken is the reservation this event was published under. The
	// consumer must claim the webhook with it before posting
	// (POST /internal/cloud-status/{id}/claim) and report the outcome with it.
	ClaimToken string `json:"claimToken"`
	OutageID   string `json:"outageId"`
	Number     string `json:"number,omitempty"`
	// Cloud is the dashboard's slug: asgardeo, choreo, bijira, devant,
	// moesif, choreo-eu or agent-manager.
	Cloud string `json:"cloud"`
	// Event is the wire value: outage_begin or outage_end.
	Event string `json:"event"`
	// Timestamp is the outage's begin or end instant, ISO-8601 UTC with
	// milliseconds ("2026-10-09T06:54:00.000Z"), as the dashboard parses it.
	Timestamp string `json:"timestamp"`
}
