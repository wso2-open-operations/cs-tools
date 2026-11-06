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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package events

// TypeProjectReactivated is published when the ACP reopen check
// (internal/service/acp_reopen.go) has set a project the Account Closure
// Process suspended back to Open, because the reason it was suspended for is
// gone (renewed, or the overdue invoice paid). It goes to
// PROJECT_EVENT_HUB_TOPIC, keyed by the CSM project id;
// csm-notification-service sends the "Back to Open state" email from it.
const TypeProjectReactivated Type = "project.reactivated"

// Reasons a project was reactivated, in ProjectReactivatedPayload.Reasons.
const (
	// ProjectReactivatedReasonRenewal: the end-date track reopened because
	// the project's end date is in the future again.
	ProjectReactivatedReasonRenewal = "renewal"
	// ProjectReactivatedReasonInvoicePaid: the invoice track reopened because
	// no counted invoice is overdue any more.
	ProjectReactivatedReasonInvoicePaid = "invoice_paid"
)

// ProjectReactivatedPayload is the payload of TypeProjectReactivated.
// csm-notification-service decodes it strictly: keep both copies identical.
// Dates are Salesforce dates ("2006-01-02"); ReactivatedOn is RFC 3339 UTC.
type ProjectReactivatedPayload struct {
	// ProjectID is the CSM project id; ProjectSfID the Salesforce one.
	ProjectID   string `json:"projectId"`
	ProjectSfID string `json:"projectSfId"`
	// ProjectName is the project name as Salesforce had it before the
	// reopen, "[S]" included, as the legacy email showed it.
	ProjectName string `json:"projectName"`
	ProjectKey  string `json:"projectKey"`
	// AccountOwnerName greets the email ("Dear <first name>,"); empty when
	// Salesforce has no owner name.
	AccountOwnerName  string `json:"accountOwnerName,omitempty"`
	AccountOwnerEmail string `json:"accountOwnerEmail,omitempty"`
	StartDate         string `json:"startDate,omitempty"`
	EndDate           string `json:"endDate,omitempty"`
	// Reasons lists why it reopened: ProjectReactivatedReasonRenewal and/or
	// ProjectReactivatedReasonInvoicePaid.
	Reasons []string `json:"reasons"`
	// Recipients are the To addresses: the account owner, the renewal
	// account manager and the technical owner, each once. The CC list is the
	// notification service's own configuration.
	Recipients    []string `json:"recipients"`
	ReactivatedOn string   `json:"reactivatedOn"`
}
