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

package notify

import (
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/deprecatedusagereport"
)

func deprecatedStatus(s string) *string { return &s }

func deprecatedSampleReport() deprecatedusagereport.Report {
	return deprecatedusagereport.Report{
		AsOf: "2026-11-02", TotalVersions: 3, TotalAccounts: 2,
		Regions: []deprecatedusagereport.Region{
			{Region: "UK", Accounts: []deprecatedusagereport.Account{{
				AccountID: "acc-1", AccountName: "Example <Roads> & Co", CsAbt: "Team Beta", Region: "UK",
				AccountManager: &deprecatedusagereport.Person{Name: "Account Manager Two", Email: "am.two@example.com"},
				TechnicalOwner: &deprecatedusagereport.Person{Name: "Technical Owner Two", Email: "to.two@example.com"},
				Projects: []deprecatedusagereport.Project{{ProjectID: "prj-1", ProjectName: "Example - Subscription", ProjectKey: "EXAMPLESUB",
					Versions: []deprecatedusagereport.Version{
						{ProductName: "WSO2 API Manager", Version: "2.6.0", SupportStatus: deprecatedStatus("DEPRECATED"), EOLDate: "2022-10-22"},
						{ProductName: "WSO2 Identity Server", Version: "5.10.0", SupportStatus: deprecatedStatus("DEPRECATED"), EOLDate: "2026-03-25"},
					}}},
			}}},
			{Region: "", Accounts: []deprecatedusagereport.Account{{
				AccountID: "acc-2", AccountName: "Gamma",
				Projects: []deprecatedusagereport.Project{{ProjectID: "prj-2", ProjectName: "Gamma Sub",
					// A deprecated version with no EOL date yet.
					Versions: []deprecatedusagereport.Version{{ProductName: "WSO2 Enterprise Integrator", Version: "6.6.0", SupportStatus: deprecatedStatus("DEPRECATED")}}}},
			}}},
		},
	}
}

func TestRenderDeprecatedProductUsageReportLayout(t *testing.T) {
	body := RenderDeprecatedProductUsageReport(deprecatedSampleReport(), "")

	for _, want := range []string{
		"Deprecated Product Version Usage Monthly Report",
		"This monthly report highlights customers who use deprecated product versions",
		"The details are as follows:",
		"As of 2026-11-02: 3 product versions across 2 accounts",
		// A region heading per region; the account with no region is under "No region".
		">UK <span", "(1 account, 2 product versions)",
		">No region <span",
		// The account's own cells span its two version rows; the project's too.
		`rowspan="2"`,
		"Example - Subscription (EXAMPLESUB)",
		"WSO2 API Manager 2.6.0",
		">Deprecated<",
		"2022-10-22", ">Not set<",
		`href="mailto:am.two@example.com"`, `href="mailto:to.two@example.com"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("report does not contain %q", want)
		}
	}
	// Escaped, not raw markup.
	if strings.Contains(body, "<Roads>") || !strings.Contains(body, "Example &lt;Roads&gt; &amp; Co") {
		t.Error("account name is not HTML-escaped")
	}
	// An account cell spans all its rows: the UK account's cells appear once.
	if n := strings.Count(body, "Team Beta"); n != 1 {
		t.Errorf("CS ABT appears %d times, want 1 (one spanning cell)", n)
	}
	// The project without a key shows its name alone; no links without a base.
	if !strings.Contains(body, ">Gamma Sub<") || strings.Contains(body, "/customers/") {
		t.Error("want plain project name and no portal links when no base URL is set")
	}
	if strings.Contains(body, "No customers are using") {
		t.Error("a report with versions must not say there are none")
	}
}

func TestRenderDeprecatedProductUsageReportHasNoRegionColumn(t *testing.T) {
	body := RenderDeprecatedProductUsageReport(deprecatedSampleReport(), "")
	if strings.Contains(body, ">Region<") {
		t.Error("the table should not repeat the region as a column: each region has its own heading")
	}
	for _, heading := range []string{">Account<", ">Project<", ">Product Version<", ">Support Status<", ">EOL Date<", ">CS ABT<", ">Account Manager<", ">Technical Owner<"} {
		if !strings.Contains(body, heading) {
			t.Errorf("missing column %s", heading)
		}
	}
}

func TestRenderDeprecatedProductUsageReportLinks(t *testing.T) {
	body := RenderDeprecatedProductUsageReport(deprecatedSampleReport(), "https://csm.example.com/")
	for _, want := range []string{
		`href="https://csm.example.com/customers/accounts/acc-1"`,
		`href="https://csm.example.com/customers/projects/prj-1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("report does not contain %q", want)
		}
	}
	if got := deprecatedUsageLink("https://csm.example.com", "accounts", "", "Name"); got != "Name" {
		t.Errorf("no id = %q, want plain text", got)
	}
	if got := deprecatedUsageLink("https://x.example.com", "projects", "a/b", "N"); !strings.Contains(got, "/customers/projects/a%2Fb") {
		t.Errorf("id not path-escaped: %s", got)
	}
}

func TestRenderDeprecatedProductUsageReportNone(t *testing.T) {
	body := RenderDeprecatedProductUsageReport(deprecatedusagereport.Report{AsOf: "2026-12-07", Regions: []deprecatedusagereport.Region{}}, "")
	if !strings.Contains(body, "No customers are using deprecated product versions") || !strings.Contains(body, "2026-12-07") {
		t.Error("the empty report does not say there are no customers as of the date")
	}
	if strings.Contains(body, "reportTable\" border") || strings.Contains(body, "The details are as follows") {
		t.Error("the empty report must have no table or introduction to a list")
	}
}

func TestDeprecatedSupportStatusLabel(t *testing.T) {
	for in, want := range map[string]string{"EXTENDED": "Extended", "available": "Available", " ": ""} {
		if got := deprecatedSupportStatusLabel(&in); got != want {
			t.Errorf("deprecatedSupportStatusLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if got := deprecatedSupportStatusLabel(nil); got != "" {
		t.Errorf("nil = %q, want blank", got)
	}
}
