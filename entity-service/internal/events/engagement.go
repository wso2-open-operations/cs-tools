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

// Customer-engagement events.
//
// This type and its payload are the contract between this service, which
// decides WHO should be told about a status update, and
// csm-notification-service, which sends the mail. The struct is duplicated
// there by hand (separate Go modules); keep the two in step.
//
// Its own file rather than events.go, for the same reason change_request.go
// is: the engagement notices are a self-contained family with their own
// audience rules, and events.go is already the case.* contract.
package events

// TypeEngagementStatusUpdateCreated asks csm-notification-service to circulate
// a weekly engagement status update. This service resolves WHO -- it owns the
// rows and the mailing list lives on them -- and that one sends the mail.
//
// The port of ServiceNow's SendEmailsOnEngagementStatusUpdateFlow, which
// triggered on a record insert on u_customer_engagement_status_update. There
// is no ServiceNow equivalent to publish this, so unlike the case.* types it
// is Postgres-data-source-only.
const TypeEngagementStatusUpdateCreated Type = "engagement.status_update_created"

// EngagementStatusUpdateCreatedPayload is TypeEngagementStatusUpdateCreated's
// payload.
//
// Recipients is the AUTHOR, and CcRecipients is the update's mailing list --
// not the other way round. The ServiceNow action bound its "to" input to
// Status Update Record > Author > Email and its "ccList" to the mailing list,
// so the author receives the notice and the audience is copied. Swapping them
// would change who appears as the primary recipient of every status update.
//
// Both lists are already resolved AND already filtered to @wso2.com by the
// time they get here: who may read an internal engagement update is a rule
// about the content, not a delivery concern. (The original tried to enforce
// that in its action's script step, but the check threw inside its own
// try/catch and was swallowed, so the send went ahead regardless.)
//
// Content is the update's HTML body as the author wrote it. The sending
// service wraps it in the WSO2 shell rather than re-rendering it.
type EngagementStatusUpdateCreatedPayload struct {
	EngagementID   string   `json:"engagementId"`
	EngagementName string   `json:"engagementName,omitempty"`
	AuthorName     string   `json:"authorName,omitempty"`
	Subject        string   `json:"subject"`
	Content        string   `json:"content"`
	CycleStartDate string   `json:"cycleStartDate,omitempty"`
	Recipients     []string `json:"recipients"`
	CcRecipients   []string `json:"ccRecipients,omitempty"`
}
