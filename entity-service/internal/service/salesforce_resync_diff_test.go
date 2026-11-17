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
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

func TestDiffResyncFields(t *testing.T) {
	tests := []struct {
		name string
		spec resyncFieldSpec
		want []domain.SalesforceResyncFieldDiff
	}{
		{"in sync", resyncFieldSpec{name: "f", stored: str("a"), written: str("a")}, nil},
		{"both null is in sync", resyncFieldSpec{name: "f"}, nil},
		{"differs", resyncFieldSpec{name: "f", stored: str("a"), written: str("b")},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("a"), SfValue: str("b")}}},
		{"salesforce cleared the value", resyncFieldSpec{name: "f", stored: str("a")},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("a")}}},
		{"column CSM keeps when Salesforce sends nothing", resyncFieldSpec{name: "f", stored: str("a"), keepStoredWhenNil: true}, nil},
		{"numbers compare as numbers", resyncFieldSpec{name: "f", stored: str("5.00"), written: str("5"), numeric: true}, nil},
		{"different numbers", resyncFieldSpec{name: "f", stored: str("5.00"), written: str("6"), numeric: true},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("5.00"), SfValue: str("6")}}},
		{"stored value already cut is in sync", resyncFieldSpec{name: "f", stored: str("abc"), written: str("abc"), raw: str("abcdef"), limit: 3}, nil},
		{"guard still cuts: wouldTruncate and the full value shown",
			resyncFieldSpec{name: "f", stored: str("old"), written: str("abc"), raw: str("abcdef"), limit: 3},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("old"), SfValue: str("abcdef"), WouldTruncate: true}}},
		{"oversized account value is written as NULL: wouldTruncate",
			resyncFieldSpec{name: "f", stored: str("old"), written: nil, raw: str("abcdef"), limit: 3},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("old"), SfValue: str("abcdef"), WouldTruncate: true}}},
		{"within the limit is not truncation",
			resyncFieldSpec{name: "f", stored: str("old"), written: str("abc"), raw: str("abc"), limit: 3},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("old"), SfValue: str("abc")}}},
		{"limit counts characters, not bytes",
			resyncFieldSpec{name: "f", stored: str("old"), written: str("ééé"), raw: str("ééé"), limit: 3},
			[]domain.SalesforceResyncFieldDiff{{Field: "f", CsmValue: str("old"), SfValue: str("ééé")}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := diffResyncFields([]resyncFieldSpec{tc.spec})
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				g, w := got[i], tc.want[i]
				if g.Field != w.Field || derefString(g.CsmValue) != derefString(w.CsmValue) || derefString(g.SfValue) != derefString(w.SfValue) ||
					(g.CsmValue == nil) != (w.CsmValue == nil) || (g.SfValue == nil) != (w.SfValue == nil) || g.WouldTruncate != w.WouldTruncate {
					t.Errorf("got %+v, want %+v", g, w)
				}
			}
		})
	}
}

// storedAccountCols is the stored row of sampleCustomer's account as it would
// be read back after an ingest of that customer.
func storedAccountCols(cust salesentity.Customer) map[string]*string {
	row := mapSalesEntityCustomer(context.Background(), cust)
	return map[string]*string{
		"name": str(row.Name), "industry": row.Industry, "region": row.Region, "global_pod": row.GlobalPod, "phone": row.Phone,
		"sales_region": row.SalesRegion, "sub_region": row.SubRegion, "life_cycle": row.LifeCycle, "naics_industry": row.NAICSIndustry,
		"sub_industry": row.SubIndustry, "classification": row.Classification, "street": row.Street, "city": row.City,
		"state_province": row.StateProvince, "postal_code": row.PostalCode, "country": row.Country, "lost_reason": row.LostReason,
		"account_vertical": row.AccountVertical, "lost_reason_category": row.LostReasonCategory,
		"activation_date": resyncDateText(row.ActivationDate), "lost_date": resyncDateText(row.LostDate),
		"deactivation_date": resyncDateText(row.DeactivationDate),
	}
}

func TestAccountDiffFields(t *testing.T) {
	ctx := context.Background()
	cust := sampleCustomer()

	if got := accountDiffFields(ctx, cust, storedAccountCols(cust)); len(got) != 0 {
		t.Fatalf("an account stored as the ingest writes it must be in sync, got %+v", got)
	}

	st := storedAccountCols(cust)
	st["industry"] = str("Other")
	st["deleted_on"] = str("2026-01-01")
	// CSM-only columns, and ones the account ingest never writes, must not appear.
	st["cre_team_id"] = str("x")
	st["sync_time_stamp"] = str("x")
	got := accountDiffFields(ctx, cust, st)
	fields := map[string]domain.SalesforceResyncFieldDiff{}
	for _, d := range got {
		fields[d.Field] = d
	}
	if len(fields) != 2 || fields["industry"].SfValue == nil || fields["deletedOn"].CsmValue == nil {
		t.Fatalf("want industry and deletedOn only, got %+v", got)
	}

	// An oversized picklist value is written as NULL: a difference that the
	// guard still cuts.
	long := strings.Repeat("x", 300)
	cust.Industry = &long
	got = accountDiffFields(ctx, cust, storedAccountCols(sampleCustomer()))
	var industry *domain.SalesforceResyncFieldDiff
	for i := range got {
		if got[i].Field == "industry" {
			industry = &got[i]
		}
	}
	if industry == nil || !industry.WouldTruncate || derefString(industry.SfValue) != long {
		t.Fatalf("industry = %+v, want wouldTruncate with the full Salesforce value", industry)
	}
}

func TestOpportunityDiffFields(t *testing.T) {
	ctx := context.Background()
	opp := sampleOpportunity()
	row := mapSalesEntityOpportunity(ctx, opp)
	st := map[string]*string{
		"name": row.Name, "stage": row.Stage, "is_won": resyncBoolText(row.IsWon), "close_date": resyncDateText(row.CloseDate),
		"eula_version": row.EulaVersion, "eula_version_decimal": row.EulaVersionDecimal, "account_sf_id": str(opp.CustomerID),
		// Columns the mapper never carries.
		"type": str("Renewal"), "engagement_code": str("ENG-9"),
	}
	li := row.LineItems[0]
	items := map[string]map[string]*string{li.LineItemSfID: {
		"name": li.Name, "quantity": resyncNumText(li.Quantity), "service_start_date": resyncDateText(li.ServiceStartDate),
		"service_end_date": resyncDateText(li.ServiceEndDate), "classification": li.Classification, "environment": li.Environment,
		"total_price": resyncNumText(li.TotalPrice), "product_name": li.ProductName, "product_code": li.ProductCode,
		"product_description": li.ProductDescription, "product_family": li.ProductFamily, "product_unit": li.ProductUnit,
		"eng_product_code": li.EngProductCode, "product_sf_id": li.ProductSfID,
	}}
	if got := opportunityDiffFields(ctx, opp, st, items); len(got) != 0 {
		t.Fatalf("want in sync, got %+v", got)
	}

	// A stale stored line item alone is informational: the apply never deletes it.
	stale := map[string]map[string]*string{"00kSTALE": {"name": str("Old item")}}
	for k, v := range items {
		stale[k] = v
	}
	if got := opportunityDiffFields(ctx, opp, st, stale); len(got) != 0 {
		t.Fatalf("a stale line item alone must not be a difference, got %+v", got)
	}

	// With another difference, the stale item and a changed item are listed.
	st["stage"] = str("Open")
	items[li.LineItemSfID]["quantity"] = str("9")
	got := opportunityDiffFields(ctx, opp, st, stale)
	names := map[string]bool{}
	for _, d := range got {
		names[d.Field] = true
	}
	for _, want := range []string{"stage", "lineItem[" + li.LineItemSfID + "].quantity", "lineItem[00kSTALE]"} {
		if !names[want] {
			t.Errorf("missing field %q in %+v", want, got)
		}
	}

	// A line item CSM does not have.
	got = opportunityDiffFields(ctx, opp, st, map[string]map[string]*string{})
	if !hasField(got, "lineItem["+li.LineItemSfID+"]") {
		t.Errorf("a line item missing in CSM must be listed, got %+v", got)
	}
}

func hasField(fields []domain.SalesforceResyncFieldDiff, name string) bool {
	for _, f := range fields {
		if f.Field == name {
			return true
		}
	}
	return false
}

func TestMembershipDiffFieldsGroupsAreAddOnly(t *testing.T) {
	in := domain.SalesforceMembershipUpsert{Email: "jane.doe@example.com", State: "REGISTERED", ProjectSfID: "a0X000000000001AAA", ContactSfID: "003000000000001AAA", ProjectGroups: []string{"Admin"}}
	st := map[string]*string{"email": str("jane.doe@example.com"), "state": str("REGISTERED"), "project_sf_id": str("a0X000000000001AAA"), "contact_sf_id": str("003000000000001AAA")}
	// A stored superset (an extra group Salesforce no longer implies) is not a difference.
	if got := membershipDiffFields(in, st, []string{"Admin", "Viewer"}); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := membershipDiffFields(in, st, []string{"Viewer"}); !hasField(got, "projectGroups") {
		t.Fatalf("a missing group must be a difference, got %+v", got)
	}
	st["state"] = str("INVITED")
	if got := membershipDiffFields(in, st, []string{"Admin"}); !hasField(got, "state") || len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestContactDiffFieldsKeepsNamesWhenSalesforceSendsNone(t *testing.T) {
	in := domain.SalesforceContactUpsert{Email: "jane.doe@example.com", AccountSfID: "001000000000001AAA"}
	user := map[string]*string{"email": str("jane.doe@example.com"), "name": str("Jane Doe"), "is_system_user": str("false")}
	ac := map[string]*string{"account_sf_id": str("001000000000001AAA")}
	if got := contactDiffFields(in, user, ac); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := contactDiffFields(in, user, nil); !hasField(got, "account") {
		t.Fatalf("a contact with no account_contact must differ on account, got %+v", got)
	}
}

func TestResyncPaged_KeysetCursor(t *testing.T) {
	ctx := context.Background()
	type rec struct{ id, lm string }
	var seen []salesentity.SearchFilter
	pages := [][]rec{{{"a", "2026-01-01T00:00:00.000+0000"}, {"b", "2026-01-02T00:00:00.000+0000"}}, {{"c", "2026-01-03T00:00:00.000+0000"}}}
	fetch := func(_ context.Context, f salesentity.SearchFilter) ([]rec, error) {
		seen = append(seen, f)
		return pages[len(seen)-1], nil
	}
	cursor := func(r rec) (*string, string) { return &r.lm, r.id }
	b := &callBudget{max: 10}
	got, err := resyncPaged(ctx, b, 2, salesentity.SearchFilter{CustomerID: "001"}, fetch, cursor)
	if err != nil || len(got) != 3 || b.used != 2 {
		t.Fatalf("got %v err %v calls %d", got, err, b.used)
	}
	if seen[0].MinLastModifiedDate != resyncKeysetEpoch || seen[0].MinID != "" || seen[0].Limit != 2 || seen[0].CustomerID != "001" {
		t.Errorf("first page filter = %+v", seen[0])
	}
	if seen[1].MinLastModifiedDate != "2026-01-02T00:00:00.000+0000" || seen[1].MinID != "b" {
		t.Errorf("second page must continue from the last record: %+v", seen[1])
	}

	// A full page whose last record has no lastModifiedDate cannot be continued.
	noCursor := func(r rec) (*string, string) { return nil, r.id }
	seen = nil
	if _, err := resyncPaged(ctx, &callBudget{max: 10}, 2, salesentity.SearchFilter{}, fetch, noCursor); !errors.Is(err, errResyncNoCursor) {
		t.Errorf("err = %v, want errResyncNoCursor", err)
	}

	// The budget ends the list.
	seen = nil
	if _, err := resyncPaged(ctx, &callBudget{max: 1}, 2, salesentity.SearchFilter{}, fetch, cursor); !errors.Is(err, errResyncCapped) {
		t.Errorf("err = %v, want errResyncCapped", err)
	}
}

func TestProjectLinkInvoiceDiffFields(t *testing.T) {
	ctx := context.Background()
	p := salesentity.Project{ID: "a0X000000000001AAA", Name: sampleStr("Proj"), Key: sampleStr("PROJ-1"), CustomerID: sampleStr("001000000000001AAA"),
		StartDate: sampleStr("2026-01-01"), Type: sampleStr("Subscription")}
	st := map[string]*string{"key": str("PROJ-1"), "name": str("Proj"), "start_date": str("2026-01-01"), "account_sf_id": str("001000000000001AAA"), "project_type": str("Subscription"),
		// CSM-owned columns are never read.
		"wso2_closure_state": str("x")}
	if got := projectDiffFields(ctx, p, str("Subscription"), st); len(got) != 0 {
		t.Fatalf("project: %+v", got)
	}
	// No project_type row has that name: the ingest would write NULL.
	if got := projectDiffFields(ctx, p, nil, st); !hasField(got, "projectType") {
		t.Errorf("project type: %+v", got)
	}
	// The account keeps its stored value when Salesforce names none.
	p.CustomerID = nil
	if got := projectDiffFields(ctx, p, str("Subscription"), st); len(got) != 0 {
		t.Errorf("project without account: %+v", got)
	}
	p.Name = sampleStr(strings.Repeat("n", 300))
	if got := projectDiffFields(ctx, p, str("Subscription"), st); len(got) != 1 || !got[0].WouldTruncate || got[0].Field != "name" {
		t.Errorf("long project name: %+v", got)
	}

	link := salesentity.LinkedOpportunity{ID: "a3U000000000001AAA", Name: sampleStr("LO-1"), OpportunityID: sampleStr("006000000000001AAA"), ProjectID: sampleStr("a0X000000000001AAA")}
	lst := map[string]*string{"number": str("LO-1"), "opportunity_sf_id": str("006000000000001AAA"), "project_sf_id": str("a0X000000000001AAA")}
	if got := linkDiffFields(ctx, link, lst); len(got) != 0 {
		t.Fatalf("link: %+v", got)
	}
	lst["project_sf_id"] = str("a0X000000000009AAA")
	if got := linkDiffFields(ctx, link, lst); !hasField(got, "project") {
		t.Errorf("link project: %+v", got)
	}

	amount := 12.5
	inv := salesentity.Invoice{ID: sampleStr("a0I000000000001AAA"), Name: sampleStr("2508"), OpportunityID: sampleStr("006000000000001AAA"), Amount: &amount, DueDate: sampleStr("2026-10-31")}
	ist := map[string]*string{"name": str("2508"), "opportunity_sf_id": str("006000000000001AAA"), "invoiced_amount": str("12.50"), "invoiced_due_date": str("2026-10-31"),
		// original due date falls back to the due date, as the ingest does.
		"original_invoice_due_date": str("2026-10-31")}
	if got := invoiceDiffFields(ctx, inv, ist); len(got) != 0 {
		t.Fatalf("invoice: %+v", got)
	}
	ist["invoiced_due_date"] = str("2026-11-30")
	if got := invoiceDiffFields(ctx, inv, ist); !hasField(got, "dueDate") {
		t.Errorf("invoice due date: %+v", got)
	}
}
