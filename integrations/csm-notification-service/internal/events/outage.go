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
// entity-service's outage notice drainer. Kept in sync by hand with
// entity-service's internal/events/outage.go.
const (
	// TypeOutageNotificationDue is the internal-stakeholder notification
	// (Declared / Update / Resolved).
	TypeOutageNotificationDue Type = "outage.notification_due"
	// TypeOutageCommunicationDue is the SRE declaration/resolution pair.
	TypeOutageCommunicationDue Type = "outage.communication_due"
)

// OutageNoticePayload carries one outage email, already decided and worded by
// entity-service. This service wraps Body in its HTML shell, adds the portal
// link and sends it to Recipients.
type OutageNoticePayload struct {
	OutageID   string   `json:"outageId"`
	Number     string   `json:"number"`
	Kind       string   `json:"kind"`
	Subject    string   `json:"subject"`
	Body       string   `json:"body"`
	Recipients []string `json:"recipients"`
}

// TypeOutageStatusPageDue asks this service to post one cloud status webhook
// to the public status dashboard: an outage began or ended on a cloud the
// page shows. Published by entity-service on the operations topic
// (sre-events) the moment the outage is written. Kept in sync by hand with
// entity-service's internal/events copy.
const TypeOutageStatusPageDue Type = "outage.status_page_due"

// OutageStatusPageDuePayload is one webhook, already decided. Event,
// Timestamp and Cloud are posted verbatim as the dashboard's body; the
// outcome is reported back to entity-service under WebhookID.
type OutageStatusPageDuePayload struct {
	WebhookID string `json:"webhookId"`
	// ClaimToken must win POST /internal/cloud-status/{id}/claim before the
	// post, and is sent back with the outcome.
	ClaimToken string `json:"claimToken"`
	OutageID   string `json:"outageId"`
	Number     string `json:"number,omitempty"`
	// Cloud is the dashboard's slug: asgardeo, choreo, bijira, devant,
	// moesif, choreo-eu or agent-manager.
	Cloud string `json:"cloud"`
	// Event is outage_begin or outage_end.
	Event string `json:"event"`
	// Timestamp is ISO-8601 UTC with milliseconds.
	Timestamp string `json:"timestamp"`
}

// validStatusPageEvent are the only wire events the dashboards parse.
var validStatusPageEvent = map[string]bool{"outage_begin": true, "outage_end": true}

// validStatusPageCloud are the dashboards' cloud slugs.
var validStatusPageCloud = map[string]bool{
	"asgardeo": true, "choreo": true, "bijira": true, "devant": true,
	"moesif": true, "choreo-eu": true, "agent-manager": true,
}

// statusPageTimestampLayout is the instant format the dashboards parse:
// ISO-8601 UTC with exactly three decimals.
const statusPageTimestampLayout = "2006-01-02T15:04:05.000Z"
