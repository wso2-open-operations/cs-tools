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

// TypeSalesforceMembershipWriteRequested is the command entity-service
// publishes to itself when a portal membership write could not reach
// Salesforce (sales/sales-entity-service unavailable) and was committed
// locally instead. Its own consumer (see
// service.ProjectMembershipWriteService's deferred write) performs the
// Salesforce half. Published to SALESFORCE_WRITE_EVENT_HUB_TOPIC, never to
// the case or project topics: it is not a notification and nothing else
// should react to it.
const TypeSalesforceMembershipWriteRequested Type = "salesforce.membership_write.requested"

// SalesforceMembershipWriteRequestedPayload identifies the
// salesforce_write_request row to carry out. The row, not the event, holds
// the intent: the event is at-least-once and may be stale by the time it is
// consumed, while the row is read fresh and says whether the write is still
// owed. Keyed (Envelope.EntityID) by RequestID so redeliveries of one request
// stay on one partition.
type SalesforceMembershipWriteRequestedPayload struct {
	RequestID        string `json:"requestId"`
	Operation        string `json:"operation"`
	ProjectID        string `json:"projectId"`
	ProjectContactID string `json:"projectContactId"`
}
