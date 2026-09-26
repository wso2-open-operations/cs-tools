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

package service

import (
	"net/http"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var (
	testOpportunitySysid = sysid32('f')
	testInvoiceSysid     = sysid32('1')
	testLinkSysid        = sysid32('2')
)

// searchLinksRequest builds a minimal, valid SearchProjectOpportunityLinksRequest (no filters)
// for tests that only care about the response mapping.
func searchLinksRequest() domain.SearchProjectOpportunityLinksRequest {
	return domain.SearchProjectOpportunityLinksRequest{
		Pagination: domain.Pagination{Limit: 50, Offset: 0},
	}
}

// TestSNOpportunityService_GetOpportunityByID_MissingKeysDoNotPanic verifies that a
// sparsely-populated opportunity row (every optional key entirely omitted, matching what a
// real ServiceNow Table API row can look like per the digiops-cs regression test for this
// same shape) maps to nil fields rather than panicking or fabricating zero values.
func TestSNOpportunityService_GetOpportunityByID_MissingKeysDoNotPanic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opportunities/"+testOpportunitySysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "` + testOpportunitySysid + `"}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOpportunityService(client)

	got, err := svc.GetOpportunityByID(contextWithUserIDToken("token"), sysidToUUID(testOpportunitySysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != nil {
		t.Errorf("Name = %v, want nil", got.Name)
	}
	if got.Account != nil {
		t.Errorf("Account = %v, want nil", got.Account)
	}
	if got.EulaVersion != nil || got.EulaVersionDecimal != nil {
		t.Errorf("EulaVersion/EulaVersionDecimal = %v/%v, want nil/nil", got.EulaVersion, got.EulaVersionDecimal)
	}
	if got.Stage != nil {
		t.Errorf("Stage = %v, want nil", got.Stage)
	}
}

// TestSNOpportunityService_GetOpportunityByID_FullRow verifies a fully-populated row maps
// every field correctly, including the sys_id -> UUID conversion for both the opportunity and
// its linked account.
func TestSNOpportunityService_GetOpportunityByID_FullRow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/opportunities/"+testOpportunitySysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "` + testOpportunitySysid + `",
			"name": "Acme Renewal",
			"account": {"id": "` + testAccountSysid + `", "name": "Acme"},
			"eulaVersion": "v2",
			"eulaVersionDecimal": "2.0",
			"stage": "50 - Closed Won"
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowOpportunityService(client)

	got, err := svc.GetOpportunityByID(contextWithUserIDToken("token"), sysidToUUID(testOpportunitySysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != sysidToUUID(testOpportunitySysid) {
		t.Errorf("ID = %q, want %q", got.ID, sysidToUUID(testOpportunitySysid))
	}
	if got.Name == nil || *got.Name != "Acme Renewal" {
		t.Errorf("Name = %v, want \"Acme Renewal\"", got.Name)
	}
	if got.Account == nil || got.Account.ID != sysidToUUID(testAccountSysid) {
		t.Errorf("Account = %v, want id %q", got.Account, sysidToUUID(testAccountSysid))
	}
	if got.Stage == nil || *got.Stage != "50 - Closed Won" {
		t.Errorf("Stage = %v, want \"50 - Closed Won\"", got.Stage)
	}
}

// TestSNInvoiceService_GetInvoiceByID_MissingKeysDoNotPanic mirrors the opportunity test above
// for invoices: an entirely sparse row (only sys_id present) must not panic and every optional
// field, including the four date fields, must come back nil.
func TestSNInvoiceService_GetInvoiceByID_MissingKeysDoNotPanic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/invoices/"+testInvoiceSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "` + testInvoiceSysid + `"}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowInvoiceService(client)

	got, err := svc.GetInvoiceByID(contextWithUserIDToken("token"), sysidToUUID(testInvoiceSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != nil || got.InvoicedAmount != nil || got.Classification != nil {
		t.Errorf("Name/InvoicedAmount/Classification = %v/%v/%v, want nil/nil/nil", got.Name, got.InvoicedAmount, got.Classification)
	}
	if got.InvoiceDate != nil || got.InvoicedPaidDate != nil || got.InvoicedDueDate != nil || got.InvoiceOriginalDueDate != nil {
		t.Errorf("date fields not all nil: invoiceDate=%v paidDate=%v dueDate=%v originalDueDate=%v",
			got.InvoiceDate, got.InvoicedPaidDate, got.InvoicedDueDate, got.InvoiceOriginalDueDate)
	}
	if got.Opportunity != nil {
		t.Errorf("Opportunity = %v, want nil", got.Opportunity)
	}
	if got.SfID != nil {
		t.Errorf("SfID = %v, want nil", got.SfID)
	}
}

// TestSNInvoiceService_GetInvoiceByID_DatesParse verifies the four date-only fields
// (invoiceDate/invoicedPaidDate/invoicedDueDate/invoiceOriginalDueDate) parse correctly, and
// specifically that invoiceOriginalDueDate is read from the "invoiceOriginalDueDate" wire key
// (the Go/Ballerina field name), not the "invoice_original_due_date" name used in the original
// tracking issue.
func TestSNInvoiceService_GetInvoiceByID_DatesParse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/invoices/"+testInvoiceSysid, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "` + testInvoiceSysid + `",
			"invoiceDate": "2026-01-15",
			"invoicedPaidDate": "2026-02-01",
			"invoicedDueDate": "2026-02-15",
			"invoiceOriginalDueDate": "2026-02-01",
			"classification": "CL",
			"sfId": "a0IE200000ArqsLMAR"
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowInvoiceService(client)

	got, err := svc.GetInvoiceByID(contextWithUserIDToken("token"), sysidToUUID(testInvoiceSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InvoiceDate == nil || *got.InvoiceDate != "2026-01-15" {
		t.Errorf("InvoiceDate = %v, want 2026-01-15", got.InvoiceDate)
	}
	if got.InvoiceOriginalDueDate == nil || *got.InvoiceOriginalDueDate != "2026-02-01" {
		t.Errorf("InvoiceOriginalDueDate = %v, want 2026-02-01", got.InvoiceOriginalDueDate)
	}
	if got.Classification == nil || *got.Classification != "CL" {
		t.Errorf("Classification = %v, want \"CL\"", got.Classification)
	}
	if got.SfID == nil || *got.SfID != "a0IE200000ArqsLMAR" {
		t.Errorf("SfID = %v, want \"a0IE200000ArqsLMAR\"", got.SfID)
	}
}

// TestSNProjectOpportunityLinkService_SearchProjectOpportunityLinks_MissingKeysDoNotPanic
// verifies a sparsely-populated link row (both references entirely omitted) maps to nil
// Project/Opportunity fields rather than panicking.
func TestSNProjectOpportunityLinkService_SearchProjectOpportunityLinks_MissingKeysDoNotPanic(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/project-opportunity-links/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"links": [{"id": "` + testLinkSysid + `"}],
			"totalRecords": 1,
			"offset": 0,
			"limit": 50
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProjectOpportunityLinkService(client)

	got, err := svc.SearchProjectOpportunityLinks(contextWithUserIDToken("token"), searchLinksRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Links) != 1 {
		t.Fatalf("len(Links) = %d, want 1", len(got.Links))
	}
	if got.Links[0].Project != nil || got.Links[0].Opportunity != nil {
		t.Errorf("Project/Opportunity = %v/%v, want nil/nil", got.Links[0].Project, got.Links[0].Opportunity)
	}
}

// TestSNProjectOpportunityLinkService_SearchProjectOpportunityLinks_FullRow verifies a
// fully-populated link row maps both references correctly.
func TestSNProjectOpportunityLinkService_SearchProjectOpportunityLinks_FullRow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/project-opportunity-links/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"links": [{
				"id": "` + testLinkSysid + `",
				"project": {"id": "` + testProjectSysid + `", "name": "TESTQUERYSUB"},
				"opportunity": {"id": "` + testOpportunitySysid + `", "name": "Acme Renewal"}
			}],
			"totalRecords": 1,
			"offset": 0,
			"limit": 50
		}`))
	})

	client := newTestSNClient(t, mux)
	svc := NewServiceNowProjectOpportunityLinkService(client)

	got, err := svc.SearchProjectOpportunityLinks(contextWithUserIDToken("token"), searchLinksRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Links) != 1 {
		t.Fatalf("len(Links) = %d, want 1", len(got.Links))
	}
	link := got.Links[0]
	if link.Project == nil || link.Project.ID != sysidToUUID(testProjectSysid) {
		t.Errorf("Project = %v, want id %q", link.Project, sysidToUUID(testProjectSysid))
	}
	if link.Opportunity == nil || link.Opportunity.ID != sysidToUUID(testOpportunitySysid) {
		t.Errorf("Opportunity = %v, want id %q", link.Opportunity, sysidToUUID(testOpportunitySysid))
	}
}
