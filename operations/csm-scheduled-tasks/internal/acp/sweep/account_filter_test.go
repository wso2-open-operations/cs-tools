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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

// accountFilterFixture is a Salesforce with one project, a0dP, past its end
// date, whose account has the given status, classification and primary
// partner. Nothing else would stop it from being suspended.
func accountFilterFixture(status, classification, primaryPartner *string) *fakeSalesforce {
	f := partnerFixture()
	f.links = nil
	cu := f.customers["001P"]
	cu.Status, cu.AccountClassification, cu.PrimaryPartnerAccountID = status, classification, primaryPartner
	f.customers["001P"] = cu
	return f
}

func openProject(id, sfID string) project {
	open := "Open"
	p := csmPartnerProject()
	p.ID, p.SfID = id, &sfID
	p.EndDate = timePtr(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	p.EndDateClosureState, p.InvoiceDueDateClosureState, p.ClosureState = &open, &open, &open
	p.SuspensionProcessState = json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"7_days_notice"}}`)
	return p
}

func TestRun_AccountSelectionFilter(t *testing.T) {
	now := time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                                    string
		status, classification, primaryPartner  *string
		wantActed                               bool
		wantStatus, wantNoStatus, wantPartnerSk int
	}{
		{"customer", sp("Customer"), sp("Enterprise"), nil, true, 0, 0, 0},
		{"customer, case and spaces", sp("  customer "), sp("Enterprise"), nil, true, 0, 0, 0},
		{"customer with a primary partner", sp("Customer"), sp("Enterprise"), sp("001X"), true, 0, 0, 0},
		{"prospect", sp("Prospect"), sp("Enterprise"), nil, false, 1, 0, 0},
		{"lost customer", sp("Lost Customer"), sp("Enterprise"), nil, false, 1, 0, 0},
		{"nil status fails closed", nil, sp("Enterprise"), nil, false, 0, 1, 0},
		{"empty status fails closed", sp("  "), sp("Enterprise"), nil, false, 0, 1, 0},
		{"partner account", sp("Customer"), sp("Partner"), nil, false, 0, 0, 1},
		{"partner account with a primary partner", sp("Customer"), sp("Partner"), sp("001X"), false, 0, 0, 1},
	} {
		for _, scoped := range []bool{false, true} {
			name := tc.name
			scope := ""
			if scoped {
				name += " (single project)"
				scope = "csm-1"
			}
			t.Run(name, func(t *testing.T) {
				proj := openProject("csm-1", "a0dP")
				updater, ntf := &mockProjectUpdater{}, &mockNotifier{}
				result, err := Run(context.Background(), csmListing(t, proj), NewSalesforceDetails(accountFilterFixture(tc.status, tc.classification, tc.primaryPartner)), updater, ntf, now, scope, nil, 0)
				if err != nil || len(result.Failures) != 0 {
					t.Fatalf("Run = %+v, %v", result, err)
				}
				if (result.ProjectsActed == 1) != tc.wantActed || (len(ntf.sent) > 0) != tc.wantActed || (len(updater.calls) > 0) != tc.wantActed {
					t.Errorf("acted=%d sent=%d writes=%d, want acted=%v", result.ProjectsActed, len(ntf.sent), len(updater.calls), tc.wantActed)
				}
				if result.ProjectsFilteredStatus != tc.wantStatus || result.ProjectsFilteredNoStatus != tc.wantNoStatus || result.ProjectsFilteredPartner != tc.wantPartnerSk {
					t.Errorf("filtered status/noStatus/partner = %d/%d/%d, want %d/%d/%d", result.ProjectsFilteredStatus, result.ProjectsFilteredNoStatus, result.ProjectsFilteredPartner, tc.wantStatus, tc.wantNoStatus, tc.wantPartnerSk)
				}
			})
		}
	}
}

// Filtered projects do not use ACP_DAILY_LIMIT: with a limit of 1, a
// Prospect and a partner ahead of a Customer do not stop the Customer acting.
func TestRun_AccountFilterDoesNotConsumeTheLimit(t *testing.T) {
	now := time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)
	f := accountFilterFixture(sp("Customer"), sp("Enterprise"), nil)
	for id, st := range map[string][2]string{"001Q": {"Prospect", "Enterprise"}, "001R": {"Customer", "Partner"}} {
		f.customers[id] = salesentity.Customer{ID: id, Status: sp(st[0]), AccountClassification: sp(st[1])}
	}
	f.projects["a0dQ"] = salesentity.Project{ID: "a0dQ", EndDate: sp("2026-10-13"), CustomerID: sp("001Q")}
	f.projects["a0dR"] = salesentity.Project{ID: "a0dR", EndDate: sp("2026-10-13"), CustomerID: sp("001R")}
	list := csmListing(t, openProject("csm-q", "a0dQ"), openProject("csm-r", "a0dR"), openProject("csm-1", "a0dP"), openProject("csm-2", "a0dP"))

	result, err := Run(context.Background(), list, NewSalesforceDetails(f), &mockProjectUpdater{}, &mockNotifier{}, now, "", nil, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ProjectsActed != 1 || result.ProjectsFilteredStatus != 1 || result.ProjectsFilteredPartner != 1 || result.NotEvaluatedByLimit != 1 {
		t.Errorf("acted=%d status=%d partner=%d notEvaluated=%d, want 1/1/1/1", result.ProjectsActed, result.ProjectsFilteredStatus, result.ProjectsFilteredPartner, result.NotEvaluatedByLimit)
	}
}

// The status filter needs a status the details source read: the CSM details
// source reads none, so it applies only the partner test.
func TestSkipByAccount_StatusOnlyWhenRead(t *testing.T) {
	var r Result
	if skipByAccount(project{}, &r) {
		t.Error("a project with no status read was skipped")
	}
	yes := true
	if !skipByAccount(project{Account: &projectAccountRef{IsPartner: &yes}}, &r) || r.ProjectsFilteredPartner != 1 {
		t.Errorf("partner not skipped: %+v", r)
	}
}
