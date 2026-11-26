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

package catalogue

import (
	"strings"
	"testing"
)

func TestBuiltInCatalogue(t *testing.T) {
	if text := DashboardEmbedded(); !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") ||
		!strings.Contains(text, "## 1. Output contract") || !strings.Contains(text, "## 2. Dashboard object") {
		first, _, _ := strings.Cut(text, "\n")
		t.Errorf("built-in dashboard catalogue starts with %q, want the catalogue", first)
	}
}

// TestNewDashboardIsTheBuiltInCopy checks that the dashboard catalogue is the
// copy built into the binary.
func TestNewDashboardIsTheBuiltInCopy(t *testing.T) {
	if NewDashboard().Text() != DashboardEmbedded() {
		t.Error("dashboard Text() is not the built-in catalogue")
	}
}

// TestNewCodeIsTheBuiltInCopy checks that the code catalogue is its own copy
// built into the binary, not the dashboard one.
func TestNewCodeIsTheBuiltInCopy(t *testing.T) {
	if !strings.HasPrefix(CodeEmbedded(), "# CSM Portal code dashboard catalogue") {
		first, _, _ := strings.Cut(CodeEmbedded(), "\n")
		t.Errorf("built-in code catalogue starts with %q, want the code catalogue", first)
	}
	if NewCode().Text() != CodeEmbedded() {
		t.Error("code Text() is not the built-in code catalogue")
	}
}

// TestCatalogueListsTheProjectDateFilters checks that the project section names
// every project query key and sort field, and that no rule still says a project
// list sorts by the end date alone.
func TestCatalogueListsTheProjectDateFilters(t *testing.T) {
	text := DashboardEmbedded()
	for _, want := range []string{
		"closureStates",
		"`endDateFrom`, `endDateTo`",
		"`onboardingStartDateFrom`, `onboardingStartDateTo`",
		"`goLivePlanDateFrom`, `goLivePlanDateTo`",
		"`goLiveDateFrom`, `goLiveDateTo`",
		"`onboardingExpiryDateFrom`, `onboardingExpiryDateTo`",
		"`productionReadinessCompletionDateFrom`, `productionReadinessCompletionDateTo`",
		"`goLivePlanDate`", "`goLiveDate`", "`onboardingExpiryDate`", "`productionReadinessCompletionDate`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("catalogue does not mention %s", want)
		}
	}
	if strings.Contains(text, "sorts by `endDate` only") || strings.Contains(text, "sort only by `endDate` (") {
		t.Error("catalogue still says a project list sorts by endDate only")
	}
}

// TestCodeCatalogueListsTheResolvedAndClosedTimes checks that a searchCases
// record lists resolvedOn and closedOn, and that its items are never averaged:
// one page of records is no MTTR.
func TestCodeCatalogueListsTheResolvedAndClosedTimes(t *testing.T) {
	text := CodeEmbedded()
	for _, want := range []string{
		"createdOn, updatedOn, resolvedOn, closedOn, account, project, assignee }",
		"a closed case\nmay have no `resolvedOn`",
		"Never compute a count, an average or a total time from `items`",
		"and `caseResolutionTime` for a mean time to resolve.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue does not say %q", want)
		}
	}
}

// TestCodeCatalogueListsCustomerHealth checks the customer health call: its
// args, that "to be reviewed" is counted rather than filtered, and that "my
// accounts" and a product are no longer declined.
func TestCodeCatalogueListsCustomerHealth(t *testing.T) {
	text := CodeEmbedded()
	for _, want := range []string{
		"### Customer health: `portal.searchCustomerHealth(args)`",
		"| `mine` | `true`: the viewer's own accounts (\"my accounts\")",
		"Never `\"to_be_reviewed\"`",
		"less the `at_risk` and `healthy` totals",
		"| `product` | one product name",
		"| `subRegions` | a list of `AFRICA`",
		"`portal.searchCustomerHealth`, `portal.countFeedback`",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue does not say %q", want)
		}
	}
	for _, gone := range []string{
		"**Customers cannot**: \"my accounts\"",
		"a region, product, ARR",
		"`healthStatus: \"at_risk\"` / `\"healthy\"` / `\"to_be_reviewed\"`",
	} {
		if strings.Contains(text, gone) {
			t.Errorf("code catalogue still says %q", gone)
		}
	}
}

// TestCodeCatalogueListsThePlgSummary checks the PLG page's figures call: its
// default window, the backlog counts that ignore the window, the weeks with no
// sign-ups left out, and its place in the check list.
func TestCodeCatalogueListsThePlgSummary(t *testing.T) {
	text := CodeEmbedded()
	for _, want := range []string{
		"`portal.plgSummary({ startDate, endDate })` returns the PLG page's own figures in **one call**",
		"with neither, the window is **the last 180\ndays**",
		"| `unacknowledged`, `needingAttention`, `trialsEndingSoon`, `ingestFailures` | **now, whatever the window**",
		"**only weeks with sign-ups**",
		"`portal.plgTrend`, `portal.plgSummary`,\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue does not say %q", want)
		}
	}
}

// TestCodeCatalogueListsTheTeamSchedule checks the rota and absence calls:
// their required window and its limit, that their items are the whole answer,
// the leave kinds, and their place in the check list.
func TestCodeCatalogueListsTheTeamSchedule(t *testing.T) {
	text := CodeEmbedded()
	for _, want := range []string{
		"### Team schedule: `portal.searchRota(args)` and `portal.searchAbsences(args)`",
		"| `startDate`, `endDate` | **both required**",
		"at most **70 days** apart",
		"here, and only here, counting `items` is\nright",
		"`SICK_LEAVE`, `MATERNITY_LEAVE`, `PATERNITY_LEAVE`",
		"`portal.searchRota`, `portal.searchAbsences`, ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue does not say %q", want)
		}
	}
}

// TestCodeCatalogueListsTheResolutionTime checks the MTTR call: its window,
// that an average can be null, how to show it, the severity levels, and that
// MTTR is no longer declined.
func TestCodeCatalogueListsTheResolutionTime(t *testing.T) {
	text := CodeEmbedded()
	for _, want := range []string{
		"### `portal.caseResolutionTime({ filters, startDate, endDate, groupBy, maxGroups })`",
		"| `startDate`, `endDate` | Both or neither: when the cases were resolved",
		"Neither is the last 12 months.",
		"(`averageHours`, **`null` when none has one**)",
		"never combine two calls' averages",
		"Show an average under 48 hours in hours",
		"`null` as \"No resolution time recorded\"",
		"(S1-S4: `[\"critical\", \"high\", \"medium\", \"low\"]`; S1-S3: `[\"critical\", \"high\", \"medium\"]`)",
		"| \"MTTR\", \"mean time to resolve\", \"average resolution time\", \"how long cases take to resolve\" | `caseResolutionTime`",
		"`portal.searchAbsences`, `portal.caseResolutionTime`; at most 12 calls.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("code catalogue does not say %q", want)
		}
	}
	if strings.Contains(text, "resolution times can be listed per case, not averaged") {
		t.Error("code catalogue still declines MTTR")
	}
}
