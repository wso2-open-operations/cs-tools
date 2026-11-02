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

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// The functions below turn one Salesforce record plus the stored columns of
// its row into the fields that differ. They reuse the ingest's own mappers
// (mapSalesEntity*, buildContactUpsert, buildMembershipUpsert) for the value
// the ingest would write, and compare it with the stored columns of the same
// names: the mappers only carry Salesforce-owned columns, so a CSM-only
// column can never appear. They make no call and touch no store.
//
// The person references of an account (owner, technical owner, CSM, ...) are
// not compared: the ingest resolves each by email to a user, and may create
// one, which a preview must not do.

// accountDiffFields compares a Salesforce Customer with its stored account.
func accountDiffFields(ctx context.Context, cust salesentity.Customer, st map[string]*string) []domain.SalesforceResyncFieldDiff {
	row := mapSalesEntityCustomer(ctx, cust)
	addr := cust.Address
	if addr == nil {
		addr = &salesentity.CustomerAddress{}
	}
	bounded := func(name, col string, raw, written *string) resyncFieldSpec {
		return resyncFieldSpec{name: name, stored: st[col], written: written, raw: optionalPtr(raw), limit: accountColumnLimits[col]}
	}
	specs := []resyncFieldSpec{
		// account.name is NOT NULL: a Customer with no name is never written.
		{name: "name", stored: st["name"], written: resyncText(row.Name), keepStoredWhenNil: true},
		bounded("industry", "industry", cust.Industry, row.Industry),
		bounded("region", "region", cust.Region, row.Region),
		bounded("globalPod", "global_pod", cust.GlobalPod, row.GlobalPod),
		bounded("salesRegion", "sales_region", cust.SalesRegions, row.SalesRegion),
		bounded("subRegion", "sub_region", cust.SubRegion, row.SubRegion),
		bounded("lifeCycle", "life_cycle", cust.Status, row.LifeCycle),
		bounded("naicsIndustry", "naics_industry", cust.NAICSIndustry, row.NAICSIndustry),
		bounded("subIndustry", "sub_industry", cust.SubIndustry, row.SubIndustry),
		bounded("classification", "classification", cust.AccountClassification, row.Classification),
		bounded("street", "street", addr.BillingStreet, row.Street),
		bounded("city", "city", addr.BillingCity, row.City),
		bounded("stateProvince", "state_province", addr.BillingState, row.StateProvince),
		bounded("postalCode", "postal_code", addr.BillingPostalCode, row.PostalCode),
		bounded("country", "country", addr.BillingCountry, row.Country),
		{name: "activationDate", stored: st["activation_date"], written: resyncDateText(row.ActivationDate)},
		{name: "lostDate", stored: st["lost_date"], written: resyncDateText(row.LostDate)},
		bounded("lostReason", "lost_reason", cust.LostReason, row.LostReason),
		// The three columns Sales Entity's "customer fields" release added
		// keep the stored value when Salesforce sends nothing.
		withKeep(bounded("accountVertical", "account_vertical", cust.AccountVertical, row.AccountVertical)),
		withKeep(bounded("lostReasonCategory", "lost_reason_category", cust.LostReasonCategory, row.LostReasonCategory)),
		{name: "deactivationDate", stored: st["deactivation_date"], written: resyncDateText(row.DeactivationDate), keepStoredWhenNil: true},
		// An upsert clears deleted_on, so a soft-deleted account is a difference.
		{name: "deletedOn", stored: st["deleted_on"], written: nil},
	}
	if !row.KeepExistingPhone {
		specs = append(specs, resyncFieldSpec{name: "phone", stored: st["phone"], written: row.Phone, raw: optionalPtr(cust.Phone), limit: maxAccountPhoneChars})
	}
	return diffResyncFields(specs)
}

func withKeep(s resyncFieldSpec) resyncFieldSpec {
	s.keepStoredWhenNil = true
	return s
}

// projectDiffFields compares a Salesforce Project__c with its stored project.
// writtenType is the project type name the ingest would store: the
// Salesforce type when a project_type row has that name, else nil.
func projectDiffFields(ctx context.Context, p salesentity.Project, writtenType *string, st map[string]*string) []domain.SalesforceResyncFieldDiff {
	sfID := strings.TrimSpace(p.ID)
	key := strings.TrimSpace(derefString(p.Key))
	row := mapSalesEntityProject(ctx, p, sfID, key)
	return diffResyncFields([]resyncFieldSpec{
		{name: "key", stored: st["key"], written: resyncText(row.Key), raw: resyncText(key), limit: maxProjectKeyChars},
		{name: "name", stored: st["name"], written: row.Name, raw: optionalPtr(p.Name), limit: maxProjectNameChars},
		{name: "description", stored: st["description"], written: row.Description},
		{name: "startDate", stored: st["start_date"], written: resyncDateText(row.StartDate)},
		{name: "endDate", stored: st["end_date"], written: resyncDateText(row.EndDate)},
		{name: "complianceViolationDate", stored: st["compliance_violation_date"], written: resyncDateText(row.ComplianceViolationDate)},
		{name: "goLiveDate", stored: st["onboarding_go_live_date"], written: resyncDateText(row.GoLiveDate)},
		// The account keeps its stored value when Salesforce names none.
		{name: "account", stored: resyncStoredSfID(st["account_sf_id"]), written: resyncSfIDText(derefString(p.CustomerID)), keepStoredWhenNil: true},
		{name: "projectType", stored: st["project_type"], written: writtenType},
	})
}

// opportunityDiffFields compares a Salesforce Opportunity, line items
// included, with its stored opportunity and the line items stored under it
// (keyed by line item sf id). Stored line items Salesforce no longer lists are
// reported only when the opportunity differs anyway: the apply never deletes
// them, so on their own they would make a permanent, unfixable difference.
func opportunityDiffFields(ctx context.Context, opp salesentity.Opportunity, st map[string]*string, storedItems map[string]map[string]*string) []domain.SalesforceResyncFieldDiff {
	row := mapSalesEntityOpportunity(ctx, opp)
	specs := []resyncFieldSpec{
		{name: "name", stored: st["name"], written: row.Name, raw: optionalPtr(opp.Name), limit: maxSfOpportunityNameChars},
		{name: "stage", stored: st["stage"], written: row.Stage, raw: optionalPtr(opp.StageName), limit: maxSfOpportunityStageChars},
		{name: "isWon", stored: st["is_won"], written: resyncBoolText(row.IsWon)},
		{name: "closeDate", stored: st["close_date"], written: resyncDateText(row.CloseDate)},
		{name: "account", stored: resyncStoredSfID(st["account_sf_id"]), written: resyncSfIDText(opp.CustomerID)},
	}
	if !row.KeepExistingEula {
		specs = append(specs,
			resyncFieldSpec{name: "eulaVersion", stored: st["eula_version"], written: row.EulaVersion, raw: optionalPtr(opp.EulaVersion.Value), limit: maxSfOpportunityEulaVersionChars},
			resyncFieldSpec{name: "eulaVersionDecimal", stored: st["eula_version_decimal"], written: row.EulaVersionDecimal, numeric: true},
		)
	}
	fields := diffResyncFields(specs)

	rawItems := map[string]salesentity.SubscriptionLineItem{}
	for _, li := range opp.SubscriptionLineItems {
		rawItems[strings.TrimSpace(derefString(li.ID))] = li
	}
	seen := map[string]bool{}
	for _, li := range row.LineItems {
		seen[li.LineItemSfID] = true
		prefix := "lineItem[" + li.LineItemSfID + "]"
		stItem, ok := storedItems[li.LineItemSfID]
		if !ok {
			fields = append(fields, domain.SalesforceResyncFieldDiff{Field: prefix, CsmValue: nil, SfValue: li.Name})
			continue
		}
		for _, d := range lineItemDiffFields(li, rawItems[li.LineItemSfID], stItem) {
			d.Field = prefix + "." + d.Field
			fields = append(fields, d)
		}
	}
	if len(fields) > 0 {
		stale := make([]string, 0)
		for id := range storedItems {
			if !seen[id] {
				stale = append(stale, id)
			}
		}
		sort.Strings(stale)
		for _, id := range stale {
			fields = append(fields, domain.SalesforceResyncFieldDiff{Field: "lineItem[" + id + "]", CsmValue: storedItems[id]["name"], SfValue: nil})
		}
	}
	return fields
}

// lineItemDiffFields compares one mapped line item with its stored row.
func lineItemDiffFields(li domain.SalesforceOpportunityLineItemUpsert, raw salesentity.SubscriptionLineItem, st map[string]*string) []domain.SalesforceResyncFieldDiff {
	var p salesentity.SubscriptionProduct
	if raw.Product != nil {
		p = *raw.Product
	}
	return diffResyncFields([]resyncFieldSpec{
		{name: "name", stored: st["name"], written: li.Name, raw: optionalPtr(raw.Name), limit: maxSfLineItemNameChars},
		{name: "quantity", stored: st["quantity"], written: resyncNumText(li.Quantity), numeric: true},
		{name: "serviceStartDate", stored: st["service_start_date"], written: resyncDateText(li.ServiceStartDate)},
		{name: "serviceEndDate", stored: st["service_end_date"], written: resyncDateText(li.ServiceEndDate)},
		{name: "classification", stored: st["classification"], written: li.Classification, raw: optionalPtr(raw.Classification), limit: maxSfLineItemShortChars},
		{name: "environment", stored: st["environment"], written: li.Environment, raw: optionalPtr(raw.Environment), limit: maxSfLineItemShortChars},
		{name: "totalPrice", stored: st["total_price"], written: resyncNumText(li.TotalPrice), numeric: true},
		{name: "productName", stored: st["product_name"], written: li.ProductName, raw: optionalPtr(p.Name), limit: maxSfLineItemProductNameChars},
		{name: "productCode", stored: st["product_code"], written: li.ProductCode, raw: optionalPtr(p.ProductCode), limit: maxSfLineItemProductCodeChars},
		{name: "productDescription", stored: st["product_description"], written: li.ProductDescription, raw: optionalPtr(p.Description), limit: maxSfLineItemProductDescChars},
		{name: "productFamily", stored: st["product_family"], written: li.ProductFamily, raw: optionalPtr(p.Family), limit: maxSfLineItemProductFamilyChars},
		{name: "productUnit", stored: st["product_unit"], written: li.ProductUnit, raw: optionalPtr(p.ProductUnit), limit: maxSfLineItemShortChars},
		{name: "engProductCode", stored: st["eng_product_code"], written: li.EngProductCode, raw: optionalPtr(p.EngProductCode), limit: maxSfLineItemShortChars},
		{name: "productSfId", stored: st["product_sf_id"], written: li.ProductSfID, raw: optionalPtr(p.ID), limit: maxSfLineItemProductSfIDChars},
	})
}

// linkDiffFields compares a Salesforce Linked_Opportunity__c with its stored link.
func linkDiffFields(ctx context.Context, link salesentity.LinkedOpportunity, st map[string]*string) []domain.SalesforceResyncFieldDiff {
	sfID := strings.TrimSpace(link.ID)
	return diffResyncFields([]resyncFieldSpec{
		{name: "number", stored: st["number"], written: mapSalesEntityLinkNumber(ctx, sfID, link), raw: optionalPtr(link.Name), limit: maxSfOpportunityLinkNumberChars},
		{name: "opportunity", stored: resyncStoredSfID(st["opportunity_sf_id"]), written: resyncSfIDText(derefString(link.OpportunityID))},
		{name: "project", stored: resyncStoredSfID(st["project_sf_id"]), written: resyncSfIDText(derefString(link.ProjectID))},
	})
}

// invoiceDiffFields compares a Salesforce Invoice__c with its stored invoice.
func invoiceDiffFields(ctx context.Context, inv salesentity.Invoice, st map[string]*string) []domain.SalesforceResyncFieldDiff {
	sfID := strings.TrimSpace(derefString(inv.ID))
	row := mapSalesEntityInvoice(ctx, sfID, inv)
	return diffResyncFields([]resyncFieldSpec{
		{name: "name", stored: st["name"], written: row.Name, raw: optionalPtr(inv.Name), limit: maxSfInvoiceTextChars},
		{name: "description", stored: st["description"], written: row.Description, raw: optionalPtr(inv.Description), limit: maxSfInvoiceTextChars},
		{name: "classification", stored: st["classification"], written: row.Classification, raw: optionalPtr(inv.Classification), limit: maxSfInvoiceTextChars},
		{name: "opportunity", stored: resyncStoredSfID(st["opportunity_sf_id"]), written: resyncSfIDText(derefString(inv.OpportunityID))},
		{name: "amount", stored: st["invoiced_amount"], written: resyncNumText(row.InvoicedAmount), numeric: true},
		{name: "invoiceDate", stored: st["invoice_date"], written: resyncDateText(row.InvoiceDate)},
		{name: "dueDate", stored: st["invoiced_due_date"], written: resyncDateText(row.InvoicedDueDate)},
		{name: "originalDueDate", stored: st["original_invoice_due_date"], written: resyncDateText(row.OriginalInvoiceDueDate)},
		{name: "paidDate", stored: st["invoiced_paid_date"], written: resyncDateText(row.InvoicedPaidDate)},
		{name: "serviceStartDate", stored: st["service_start_date"], written: resyncDateText(row.ServiceStartDate)},
		{name: "serviceEndDate", stored: st["service_end_date"], written: resyncDateText(row.ServiceEndDate)},
	})
}

// contactDiffFields compares the Contact writer's request (built from a
// Salesforce Contact) with the stored "user" and account_contact rows; either
// may be nil when CSM has none.
func contactDiffFields(in domain.SalesforceContactUpsert, user, ac map[string]*string) []domain.SalesforceResyncFieldDiff {
	get := func(m map[string]*string, col string) *string {
		if m == nil {
			return nil
		}
		return m[col]
	}
	return diffResyncFields([]resyncFieldSpec{
		{name: "email", stored: get(user, "email"), written: resyncText(in.Email)},
		// The name columns keep the stored value when Salesforce sends none.
		{name: "name", stored: get(user, "name"), written: resyncText(in.Name), keepStoredWhenNil: true},
		{name: "firstName", stored: get(user, "first_name"), written: resyncText(in.FirstName), keepStoredWhenNil: true},
		{name: "lastName", stored: get(user, "last_name"), written: resyncText(in.LastName), keepStoredWhenNil: true},
		{name: "isIntegrationUser", stored: get(user, "is_system_user"), written: resyncBoolText(&in.IsCsIntegrationUser)},
		{name: "account", stored: resyncStoredSfID(get(ac, "account_sf_id")), written: resyncSfIDText(in.AccountSfID)},
		{name: "isPrimaryContact", stored: get(ac, "is_primary_contact"), written: resyncBoolText(in.IsPrimaryContact), keepStoredWhenNil: true},
	})
}

// membershipDiffFields compares the membership upsert request with the stored
// project_contact, whose project groups are storedGroups. Groups are compared
// as "every group Salesforce implies is stored": a stored group it does not
// imply is left (the apply never deletes), so it is not a difference.
func membershipDiffFields(in domain.SalesforceMembershipUpsert, st map[string]*string, storedGroups []string) []domain.SalesforceResyncFieldDiff {
	fields := diffResyncFields([]resyncFieldSpec{
		{name: "email", stored: st["email"], written: resyncText(in.Email)},
		{name: "state", stored: st["state"], written: resyncText(in.State)},
		{name: "project", stored: resyncStoredSfID(st["project_sf_id"]), written: resyncSfIDText(in.ProjectSfID), keepStoredWhenNil: true},
		{name: "contact", stored: resyncStoredSfID(st["contact_sf_id"]), written: resyncSfIDText(in.ContactSfID)},
	})
	have := map[string]bool{}
	for _, g := range storedGroups {
		have[g] = true
	}
	missing := false
	for _, g := range in.ProjectGroups {
		if !have[g] {
			missing = true
			break
		}
	}
	if missing {
		fields = append(fields, domain.SalesforceResyncFieldDiff{
			Field:    "projectGroups",
			CsmValue: joinSorted(storedGroups),
			SfValue:  joinSorted(in.ProjectGroups),
		})
	}
	return fields
}

func joinSorted(items []string) *string {
	if len(items) == 0 {
		return nil
	}
	c := append([]string(nil), items...)
	sort.Strings(c)
	s := strings.Join(c, ", ")
	return &s
}
