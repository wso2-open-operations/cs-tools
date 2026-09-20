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

package sweep

import (
	"encoding/json"
	"testing"
	"time"
)

// TestProject_ParsesNestedAccountFromRealGetProjectResponse is a regression
// test for a real bug found via direct Postman testing against staging:
// GetProject's response (GET /projects/e3e87599-1bc7-6650-182c-0dc5604bcb68)
// nests the account reference as "account": {"id": ..., "name": ...}, not a
// flat top-level "accountId" string. The project struct originally assumed
// the latter, so accountID() silently returned "" — which produced a real
// 404 on SearchAccountContacts, initially misdiagnosed as bad test data.
// This uses the actual response shape returned for that project (trimmed to
// the fields this component reads, plus one sibling field on the nested
// account object to prove extra fields don't break parsing).
func TestProject_ParsesNestedAccountFromRealGetProjectResponse(t *testing.T) {
	const realGetProjectResponse = `{
		"id": "e3e87599-1bc7-6650-182c-0dc5604bcb68",
		"account": {
			"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
			"name": "ACP Test Partner Account"
		},
		"endDate": null
	}`

	var proj project
	if err := json.Unmarshal([]byte(realGetProjectResponse), &proj); err != nil {
		t.Fatalf("unmarshal real GetProject response: %v", err)
	}

	if proj.Account == nil {
		t.Fatal("Account = nil, want populated from the nested \"account\" object")
	}

	const wantAccountID = "f213fdd1-1b4b-a650-a002-c9d3604bcbac"
	if got := proj.accountID(); got != wantAccountID {
		t.Errorf("accountID() = %q, want %q", got, wantAccountID)
	}
}

// TestProject_AccountIDIsEmptyWhenAccountIsAbsent covers the other real
// shape: SearchProjects's response items (entity-service's ProjectView) carry
// no account reference at all, nested or flat — confirmed against
// entity-service's own domain type. accountID() must degrade to "" rather
// than panic on a nil Account.
func TestProject_AccountIDIsEmptyWhenAccountIsAbsent(t *testing.T) {
	const realSearchProjectsItem = `{"id": "p1", "endDate": null}`

	var proj project
	if err := json.Unmarshal([]byte(realSearchProjectsItem), &proj); err != nil {
		t.Fatalf("unmarshal real SearchProjects item: %v", err)
	}

	if got := proj.accountID(); got != "" {
		t.Errorf("accountID() = %q, want \"\"", got)
	}
}

// TestProject_ParsesNameProjectKeyStartDateAndAccountName covers the fields
// added for the notice-content redesign: name, key, and startDate are
// documented on csm-integration-service's Project schema (openapi.yaml) but
// were previously unread by this component; the same is true of the nested
// account's own name, alongside its id. Uses the literal real GetProject
// response for the dedicated test project (Postman, confirmed directly by
// the user) rather than a synthetic fixture — this is what caught a real
// discrepancy: openapi.yaml documents this field as "projectKey", but the
// live response actually names it "key". Trust the wire, not the spec.
func TestProject_ParsesNameProjectKeyStartDateAndAccountName(t *testing.T) {
	const realGetProjectResponse = `{
		"id": "e3e87599-1bc7-6650-182c-0dc5604bcb68",
		"account": {
			"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
			"name": "ACP Test Partner Account",
			"activationDate": null,
			"tier": "",
			"region": null,
			"agentEnabled": false,
			"kbReferencesEnabled": false
		},
		"sfId": "a0dE200000CZ9ZNIA1",
		"name": "ACP Partner Project - Subscription",
		"key": "APPSUB",
		"subscriptionType": "subscription",
		"startDate": "2025-07-01T00:00:00Z",
		"endDate": "2026-07-29T00:00:00Z",
		"createdOn": "2025-07-29T05:59:07Z",
		"updatedOn": "2025-07-29T05:59:07Z",
		"closureState": "Suspended",
		"endDateClosureState": "Suspended",
		"invoiceDueDateClosureState": "Open",
		"complianceViolationClosureState": "Open",
		"complianceViolationDate": null,
		"suspensionProcessState": {
			"based_on_compliance": {"event_type": "open"},
			"based_on_due_invoices": {"event_type": "7_days_notice", "actionSendEmailNotification": "SUCCESSFUL", "actionServicePortalAnnouncement": "SUCCESSFUL"},
			"based_on_subscription_end_date": {"actionSendEmailNotification": "IGNORED", "event_type": "suspend"}
		}
	}`

	var proj project
	if err := json.Unmarshal([]byte(realGetProjectResponse), &proj); err != nil {
		t.Fatalf("unmarshal real GetProject response: %v", err)
	}

	if proj.Name != "ACP Partner Project - Subscription" {
		t.Errorf("Name = %q, want %q", proj.Name, "ACP Partner Project - Subscription")
	}
	if proj.ProjectKey != "APPSUB" {
		t.Errorf("ProjectKey = %q, want %q", proj.ProjectKey, "APPSUB")
	}
	if proj.StartDate == nil || !proj.StartDate.Equal(time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("StartDate = %v, want 2025-07-01T00:00:00Z", proj.StartDate)
	}
	if proj.Account == nil || proj.Account.Name != "ACP Test Partner Account" {
		t.Errorf("Account.Name = %v, want %q", proj.Account, "ACP Test Partner Account")
	}
}

// TestProject_ParsesNameKeyAndStartDateFromRealSearchProjectsResponse is a
// regression test addressing a real review concern (Sajith Ekanayake):
// Name/ProjectKey/StartDate were only regression-tested against
// a GetProject-shaped fixture, but the unattended production sweep
// (TEST_PROJECT_ID unset) reads projects from /projects/search instead — a
// different, historically leaner response shape (CLAUDE.md documents
// account itself being absent from this endpoint for a period). Confirmed
// via a real /projects/search response (Postman) that name/key/startDate
// ARE present on this shape too, not just GetProject's — this is the
// literal response used as the fixture below.
func TestProject_ParsesNameKeyAndStartDateFromRealSearchProjectsResponse(t *testing.T) {
	const realSearchProjectsResponse = `{
		"projects": [
			{
				"id": "266f6292-1b46-f510-264c-997a234bcba9",
				"name": "DemoCloud - Cloud Support",
				"key": "DEMOCLOUDCLOUDSUB",
				"subscriptionType": "cloud_support",
				"startDate": "2025-02-18T00:00:00Z",
				"endDate": null,
				"createdOn": "2023-10-25T06:54:50Z",
				"account": {
					"id": "cf3fee52-1b46-f510-264c-997a234bcbe5",
					"name": "DemoCloud"
				},
				"closureState": "Open",
				"endDateClosureState": "Open",
				"invoiceDueDateClosureState": "Open",
				"complianceViolationClosureState": null,
				"complianceViolationDate": null,
				"suspensionProcessState": {
					"based_on_subscription_end_date": {"event_type": "open"},
					"based_on_due_invoices": {"event_type": "open"},
					"based_on_compliance": {"event_type": "open"}
				}
			}
		]
	}`

	var resp searchProjectsResponse
	if err := json.Unmarshal([]byte(realSearchProjectsResponse), &resp); err != nil {
		t.Fatalf("unmarshal real SearchProjects response: %v", err)
	}
	if len(resp.Projects) != 1 {
		t.Fatalf("Projects = %d, want 1", len(resp.Projects))
	}

	proj := resp.Projects[0]
	if proj.Name != "DemoCloud - Cloud Support" {
		t.Errorf("Name = %q, want %q", proj.Name, "DemoCloud - Cloud Support")
	}
	if proj.ProjectKey != "DEMOCLOUDCLOUDSUB" {
		t.Errorf("ProjectKey = %q, want %q", proj.ProjectKey, "DEMOCLOUDCLOUDSUB")
	}
	if proj.StartDate == nil || !proj.StartDate.Equal(time.Date(2025, 2, 18, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("StartDate = %v, want 2025-02-18T00:00:00Z", proj.StartDate)
	}
}

// TestAccountDTO_ParsesTechnicalOwnerAndRenewalAccountManager covers the two
// account fields that were previously fetched over the wire and silently
// dropped (accountDTO had no field for them): confirmed present on the real
// entity-service response for the dedicated test account, alongside
// accountManager.
func TestAccountDTO_ParsesTechnicalOwnerAndRenewalAccountManager(t *testing.T) {
	const realGetAccountResponse = `{
		"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac",
		"name": "ACP Test Partner Account",
		"technicalOwner": {"id": "tech-1", "name": "Alex Fernando", "email": "alex.fernando@wso2.example"},
		"accountManager": {"id": "am-1", "name": "Jordan Perera", "email": "jordan.perera@wso2.example"},
		"renewalAccountManager": {"id": "ram-1", "name": "Sam Jayasuriya", "email": "sam.jayasuriya@wso2.example"}
	}`

	var acc accountDTO
	if err := json.Unmarshal([]byte(realGetAccountResponse), &acc); err != nil {
		t.Fatalf("unmarshal real GetAccount response: %v", err)
	}

	if acc.TechnicalOwner == nil || acc.TechnicalOwner.Name != "Alex Fernando" {
		t.Errorf("TechnicalOwner = %v, want Name %q", acc.TechnicalOwner, "Alex Fernando")
	}
	if acc.RenewalAccountManager == nil || acc.RenewalAccountManager.Name != "Sam Jayasuriya" {
		t.Errorf("RenewalAccountManager = %v, want Name %q", acc.RenewalAccountManager, "Sam Jayasuriya")
	}
	if acc.AccountManager == nil || acc.AccountManager.Name != "Jordan Perera" {
		t.Errorf("AccountManager = %v, want Name %q", acc.AccountManager, "Jordan Perera")
	}
}
