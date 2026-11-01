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

package repository

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

var resyncWriteKeyword = regexp.MustCompile(`(?i)\b(insert|update|delete|truncate|alter|drop|create|grant|nextval|pg_advisory\w*)\b`)

// TestSalesforceResyncSQL_ReadOnly pins the preview's promise that it writes
// nothing: every statement the resync read repository runs is a plain SELECT.
func TestSalesforceResyncSQL_ReadOnly(t *testing.T) {
	queries := map[string]string{
		"accounts": resyncAccountsQuery, "projects": resyncProjectsQuery, "opportunities": resyncOpportunitiesQuery,
		"line items": resyncLineItemsQuery, "links": resyncLinksQuery, "invoices": resyncInvoicesQuery,
		"users": resyncUsersQuery, "account contacts": resyncAccountContactsQuery, "memberships": resyncMembershipsQuery,
		"membership groups": resyncMembershipGroupsQuery, "partners": resyncPartnersQuery,
		"account target": resyncTargetAccountQuery, "project target": resyncTargetProjectQuery,
	}
	for name, q := range queries {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(q)), "select") {
			t.Errorf("%s: not a SELECT: %.40s", name, q)
		}
		if m := resyncWriteKeyword.FindString(q); m != "" {
			t.Errorf("%s: contains %q", name, m)
		}
	}
}

// TestSalesforceResyncSQL_ColumnNames pins the column names the service's
// comparison reads (salesforce_resync_compare.go) and keeps CSM-only columns
// out: only Salesforce-owned columns are selected.
func TestSalesforceResyncSQL_ColumnNames(t *testing.T) {
	names := func(c resyncColumns) []string {
		out := make([]string, 0, len(c.cols))
		for _, col := range c.cols {
			out = append(out, col[0])
		}
		sort.Strings(out)
		return out
	}
	want := map[string][]string{
		"account": {"account_vertical", "activation_date", "city", "classification", "country", "deactivation_date", "deleted_on", "global_pod",
			"industry", "life_cycle", "lost_date", "lost_reason", "lost_reason_category", "naics_industry", "name", "phone", "postal_code",
			"region", "sales_region", "state_province", "street", "sub_industry", "sub_region"},
		"project":     {"account_sf_id", "compliance_violation_date", "description", "end_date", "key", "name", "onboarding_go_live_date", "project_type", "start_date"},
		"opportunity": {"account_sf_id", "close_date", "eula_version", "eula_version_decimal", "is_won", "name", "stage"},
		"line item": {"classification", "eng_product_code", "environment", "name", "opportunity_sf_id", "product_code", "product_description", "product_family",
			"product_name", "product_sf_id", "product_unit", "quantity", "service_end_date", "service_start_date", "total_price"},
		"link":            {"number", "opportunity_sf_id", "project_sf_id"},
		"invoice":         {"classification", "description", "invoice_date", "invoiced_amount", "invoiced_due_date", "invoiced_paid_date", "name", "opportunity_sf_id", "original_invoice_due_date", "service_end_date", "service_start_date"},
		"user":            {"email", "first_name", "is_system_user", "last_name", "name"},
		"account contact": {"account_sf_id", "is_active", "is_primary_contact", "user_name"},
		"membership":      {"contact_sf_id", "email", "project_sf_id", "state"},
	}
	got := map[string]resyncColumns{
		"account": resyncAccountCols, "project": resyncProjectCols, "opportunity": resyncOpportunityCols, "line item": resyncLineItemCols,
		"link": resyncLinkCols, "invoice": resyncInvoiceCols, "user": resyncUserCols, "account contact": resyncAccountContactCols,
		"membership": resyncMembershipCols,
	}
	for k, w := range want {
		if g := names(got[k]); strings.Join(g, ",") != strings.Join(w, ",") {
			t.Errorf("%s columns = %v, want %v", k, g, w)
		}
	}
	// Columns the CSM platform or the sync owns must never be selected.
	for _, q := range []string{resyncAccountsQuery, resyncProjectsQuery, resyncOpportunitiesQuery, resyncLineItemsQuery, resyncLinksQuery, resyncInvoicesQuery} {
		for _, col := range []string{"cre_team_id", "sre_team_id", "wso2_closure_state", "assignment_group_id", "sync_time_stamp", "engagement_code", "query_hour_state", "development_support_hours", "owner"} {
			if regexp.MustCompile(`\b[a-z]+\.` + col + `\b`).MatchString(q) {
				t.Errorf("a query selects the non-Salesforce column %s", col)
			}
		}
	}
}
