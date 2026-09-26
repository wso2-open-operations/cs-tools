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
)

// oppLinksResponse builds a SearchProjectOpportunityLinks response body
// linking projectID to each of oppIDs.
func oppLinksResponse(projectID string, oppIDs ...string) []byte {
	links := make([]map[string]any, len(oppIDs))
	for i, id := range oppIDs {
		links[i] = map[string]any{
			"id":          "link-" + id,
			"project":     map[string]any{"id": projectID, "name": "proj"},
			"opportunity": map[string]any{"id": id, "name": "opp-" + id},
		}
	}
	body, _ := json.Marshal(map[string]any{"links": links})
	return body
}

func TestResolveDueInvoice_HappyPathPicksTheOnlyEligibleInvoice(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	proj := project{ID: "p1", StartDate: &start}

	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"opp1","name":"Opp One","stage":"50 - Closed Won","eulaVersion":"EULA 3.3","eulaVersionDecimal":"3.3"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"invoices":[{
				"id":"inv1","name":"INV-1","invoiceDate":"2026-01-01",
				"invoicedDueDate":"2026-02-01","classification":"CL",
				"opportunity":{"id":"opp1","name":"Opp One"}
			}]}`), nil
		},
	}

	got, err := resolveDueInvoice(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("resolveDueInvoice() = nil, want a resolved invoice")
	}
	if got.ID != "inv1" {
		t.Errorf("ID = %q, want %q", got.ID, "inv1")
	}
	if got.Opportunity != "Opp One" {
		t.Errorf("Opportunity = %q, want %q", got.Opportunity, "Opp One")
	}
	if got.EULAVersionDecimal != 3.3 {
		t.Errorf("EULAVersionDecimal = %v, want 3.3", got.EULAVersionDecimal)
	}
	wantDue := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if !got.DueDate.Equal(wantDue) {
		t.Errorf("DueDate = %v, want %v", got.DueDate, wantDue)
	}
	wantInvoiceDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.InvoiceDate.Equal(wantInvoiceDate) {
		t.Errorf("InvoiceDate = %v, want %v", got.InvoiceDate, wantInvoiceDate)
	}
}

// TestResolveDueInvoice_PaginatesProjectOpportunityLinks is the regression
// test for a real gap (CodeRabbit, PR #1933): resolveDueInvoice read only
// the first page of SearchProjectOpportunityLinks and never checked
// hasMore, silently missing any link on a later page. Opportunity 1 (page
// 1) is deliberately ineligible (EULA "Customer contract"), so the only way
// this test can resolve an invoice at all is if page 2's link was actually
// fetched.
func TestResolveDueInvoice_PaginatesProjectOpportunityLinks(t *testing.T) {
	proj := project{ID: "p1"}

	linksCalls := 0
	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			linksCalls++
			if linksCalls == 1 {
				return []byte(`{"links":[{"id":"link1","opportunity":{"id":"opp-page1"}}],"hasMore":true}`), nil
			}
			return []byte(`{"links":[{"id":"link2","opportunity":{"id":"opp-page2"}}],"hasMore":false}`), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			if id == "opp-page2" {
				return []byte(`{"id":"opp-page2","name":"Opp Page 2","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
			}
			return []byte(`{"id":"opp-page1","name":"Opp Page 1","stage":"50 - Closed Won","eulaVersion":"Customer contract"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"invoices":[{
				"id":"inv-from-page2","invoiceDate":"2026-01-01","invoicedDueDate":"2026-05-01",
				"opportunity":{"id":"opp-page2","name":"Opp Page 2"}
			}]}`), nil
		},
	}

	got, err := resolveDueInvoice(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
	}
	if linksCalls < 2 {
		t.Fatalf("SearchProjectOpportunityLinks called %d times, want at least 2 — pagination (hasMore) not followed", linksCalls)
	}
	if got == nil || got.ID != "inv-from-page2" {
		t.Errorf("got = %+v, want the invoice reachable only via page 2's link", got)
	}
}

// TestResolveDueInvoice_PaginatesInvoicesForOpportunity is the regression
// test for the SearchInvoices half of the same gap: the earlier-due,
// eligible invoice sits on the second page. If pagination isn't followed,
// resolveDueInvoice would return the later-due invoice from page 1 instead
// (or, before this fix, silently miss the earlier one entirely).
func TestResolveDueInvoice_PaginatesInvoicesForOpportunity(t *testing.T) {
	proj := project{ID: "p1"}

	invoiceCalls := 0
	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"opp1","name":"Opp One","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			invoiceCalls++
			if invoiceCalls == 1 {
				return []byte(`{"invoices":[{"id":"inv-page1-later","invoiceDate":"2026-01-01","invoicedDueDate":"2026-08-01"}],"hasMore":true}`), nil
			}
			return []byte(`{"invoices":[{"id":"inv-page2-earlier","invoiceDate":"2026-01-01","invoicedDueDate":"2026-03-01"}],"hasMore":false}`), nil
		},
	}

	got, err := resolveDueInvoice(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
	}
	if invoiceCalls < 2 {
		t.Fatalf("SearchInvoices called %d times, want at least 2 — pagination (hasMore) not followed", invoiceCalls)
	}
	if got == nil || got.ID != "inv-page2-earlier" {
		t.Errorf("got = %+v, want the earlier-due invoice from page 2 to win, not page 1's later one", got)
	}
}

func TestResolveDueInvoice_PicksTheEarliestDueDateAcrossOpportunities(t *testing.T) {
	proj := project{ID: "p1"}

	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return oppLinksResponse("p1", "opp1", "opp2"), nil
		},
		getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","name":"Opp","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			var req struct {
				OpportunityID string `json:"opportunityId"`
			}
			json.Unmarshal(body, &req)
			switch req.OpportunityID {
			case "opp1":
				return []byte(`{"invoices":[{"id":"inv-later","invoiceDate":"2026-01-01","invoicedDueDate":"2026-06-01","opportunity":{"id":"opp1"}}]}`), nil
			case "opp2":
				return []byte(`{"invoices":[{"id":"inv-earlier","invoiceDate":"2026-01-01","invoicedDueDate":"2026-03-01","opportunity":{"id":"opp2"}}]}`), nil
			}
			return []byte(`{"invoices":[]}`), nil
		},
	}

	got, err := resolveDueInvoice(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("resolveDueInvoice() = nil, want a resolved invoice")
	}
	if got.ID != "inv-earlier" {
		t.Errorf("ID = %q, want %q (the earlier due date should win)", got.ID, "inv-earlier")
	}
}

func TestResolveDueInvoice_ExcludesOpportunitiesFailingTheEligibilityCheck(t *testing.T) {
	tests := []struct {
		name string
		opp  string // raw opportunity JSON
	}{
		{
			name: "eulaVersion is Customer contract",
			opp:  `{"id":"opp1","stage":"50 - Closed Won","eulaVersion":"Customer contract","eulaVersionDecimal":"3.4"}`,
		},
		{
			name: "eulaVersion is null",
			opp:  `{"id":"opp1","stage":"50 - Closed Won","eulaVersion":null,"eulaVersionDecimal":"3.4"}`,
		},
		{
			name: "stage is not Closed Won",
			opp:  `{"id":"opp1","stage":"45 - Proposal","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`,
		},
		{
			name: "stage is null",
			opp:  `{"id":"opp1","stage":null,"eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := project{ID: "p1"}
			invoiceSearchCalled := false
			reader := &mockEntityReader{
				searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
					return oppLinksResponse("p1", "opp1"), nil
				},
				getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
					return []byte(tt.opp), nil
				},
				searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
					invoiceSearchCalled = true
					return []byte(`{"invoices":[{"id":"inv1","invoicedDueDate":"2026-06-01","opportunity":{"id":"opp1"}}]}`), nil
				},
			}

			got, err := resolveDueInvoice(context.Background(), reader, proj)
			if err != nil {
				t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
			}
			if got != nil {
				t.Errorf("resolveDueInvoice() = %+v, want nil — ineligible opportunity's invoices must not count", got)
			}
			if invoiceSearchCalled {
				t.Error("SearchInvoices should not be called for an ineligible opportunity")
			}
		})
	}
}

func TestResolveDueInvoice_ExcludesIneligibleInvoices(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		invoice string
	}{
		{
			name:    "already paid",
			invoice: `{"id":"inv1","invoicedDueDate":"2026-06-01","invoicedPaidDate":"2026-05-01","opportunity":{"id":"opp1"}}`,
		},
		{
			name:    "excluded classification PP",
			invoice: `{"id":"inv1","invoicedDueDate":"2026-06-01","classification":"PP","opportunity":{"id":"opp1"}}`,
		},
		{
			name:    "excluded classification CO",
			invoice: `{"id":"inv1","invoicedDueDate":"2026-06-01","classification":"CO","opportunity":{"id":"opp1"}}`,
		},
		{
			name:    "excluded classification TAM",
			invoice: `{"id":"inv1","invoicedDueDate":"2026-06-01","classification":"TAM","opportunity":{"id":"opp1"}}`,
		},
		{
			name:    "excluded name Auto Created PS",
			invoice: `{"id":"inv1","name":"Auto Created PS","invoicedDueDate":"2026-06-01","opportunity":{"id":"opp1"}}`,
		},
		{
			name:    "due date before project start",
			invoice: `{"id":"inv1","invoicedDueDate":"2025-06-01","opportunity":{"id":"opp1"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := project{ID: "p1", StartDate: &start}
			reader := &mockEntityReader{
				searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
					return oppLinksResponse("p1", "opp1"), nil
				},
				getOpportunityFn: func(ctx context.Context, id string) ([]byte, error) {
					return []byte(`{"id":"opp1","stage":"50 - Closed Won","eulaVersion":"EULA 3.4","eulaVersionDecimal":"3.4"}`), nil
				},
				searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
					return []byte(`{"invoices":[` + tt.invoice + `]}`), nil
				},
			}

			got, err := resolveDueInvoice(context.Background(), reader, proj)
			if err != nil {
				t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
			}
			if got != nil {
				t.Errorf("resolveDueInvoice() = %+v, want nil", got)
			}
		})
	}
}

func TestResolveDueInvoice_NoLinksReturnsNilWithoutCallingInvoiceSearch(t *testing.T) {
	proj := project{ID: "p1"}
	invoiceSearchCalled := false
	reader := &mockEntityReader{
		searchProjectOpportunityLinksFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"links":[]}`), nil
		},
		searchInvoicesFn: func(ctx context.Context, body []byte) ([]byte, error) {
			invoiceSearchCalled = true
			return []byte(`{"invoices":[]}`), nil
		},
	}

	got, err := resolveDueInvoice(context.Background(), reader, proj)
	if err != nil {
		t.Fatalf("resolveDueInvoice() error = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("resolveDueInvoice() = %+v, want nil", got)
	}
	if invoiceSearchCalled {
		t.Error("SearchInvoices should not be called when a project has no linked opportunities")
	}
}
