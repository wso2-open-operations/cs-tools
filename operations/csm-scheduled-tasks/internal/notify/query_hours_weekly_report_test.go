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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

// timeConvert must match SN_Utils.timeConvert character for character,
// leading space and all. Every expectation here was read off a real
// production report.
func TestTimeConvert_MatchesServiceNowExactly(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, " 0h 0m"},
		{60, " 1h 0m"},
		{95, " 1h 35m"},
		{6000, " 100h 0m"},
		{6275, " 104h 35m"},
		{1500, " 25h 0m"},
		{4255, " 70h 55m"},
		// Negatives carry the sign AND the space: "- 4h 35m", not "-4h 35m".
		{-275, "- 4h 35m"},
		{-30, "- 0h 30m"},
		{-10742, "- 179h 2m"},
	}
	for _, tc := range tests {
		if got := timeConvert(tc.minutes); got != tc.want {
			t.Errorf("timeConvert(%d) = %q, want %q", tc.minutes, got, tc.want)
		}
	}
}

// A project funded by two opportunities in one group renders twice but is
// greyed the second time, and its consumption counts once in the group total
// — the layout ServiceNow uses.
func TestRenderQueryHoursWeeklyReport_GreysTheDuplicateProjectRow(t *testing.T) {
	body := RenderQueryHoursWeeklyReport(QueryHoursWeeklyReportData{
		Report: queryhoursreport.Report{
			GeneratedOn:   "2026-09-20",
			ExceededCount: 1,
			Exceeded: []queryhoursreport.Account{{
				Name: "Vivicta OY", RowCount: 2, Exceeded: true,
				Groups: []queryhoursreport.Group{{
					EntitlementMinutes: 6000, ConsumedMinutes: 6600,
					RemainingMinutes: -600, Exceeded: true, RowCount: 2,
					Opportunities: []queryhoursreport.Opportunity{
						{Name: "Opp A", EntitlementMinutes: 3000, Projects: []queryhoursreport.Project{
							{Name: "Tieto - Subscription", Key: "TIETOOEMSUB", ConsumedMinutes: 6600},
						}},
						{Name: "Opp B", EntitlementMinutes: 3000, Projects: []queryhoursreport.Project{
							{Name: "Tieto - Subscription", Key: "TIETOOEMSUB", ConsumedMinutes: 6600, Duplicate: true},
						}},
					},
				}},
			}},
		},
	})

	if strings.Count(body, "TIETOOEMSUB") != 2 {
		t.Errorf("the shared project should render once per funding opportunity")
	}
	if !strings.Contains(body, "background:#eceff1") {
		t.Error("the duplicate row should be greyed, as ServiceNow greys it")
	}
	// The group totals are rendered once, spanning both rows.
	if strings.Count(body, `rowspan="2"`) < 1 {
		t.Error("group totals should span the group's rows")
	}
	if !strings.Contains(body, "- 10h 0m") {
		t.Error("the group's merged remaining figure should be rendered once")
	}
}

// An unmatched product line understates an entitlement silently in
// ServiceNow. The port says so.
func TestRenderQueryHoursWeeklyReport_SurfacesUnmatchedProductLines(t *testing.T) {
	with := RenderQueryHoursWeeklyReport(QueryHoursWeeklyReportData{
		Report: queryhoursreport.Report{GeneratedOn: "2026-09-20", UnmatchedLineCount: 3},
	})
	if !strings.Contains(with, "matched none of the six known") {
		t.Error("unmatched product lines must be surfaced in the report body")
	}
	if !strings.Contains(with, "<strong>3</strong>") {
		t.Error("the unmatched count should be shown")
	}

	without := RenderQueryHoursWeeklyReport(QueryHoursWeeklyReportData{
		Report: queryhoursreport.Report{GeneratedOn: "2026-09-20"},
	})
	if strings.Contains(without, "matched none of the six known") {
		t.Error("no note should appear when every line matched")
	}
}

// The seven-column format carries the project key; the five-column threshold
// email deliberately does not. This test exists so the two do not converge
// again — a project-key suffix has already leaked from here into that email
// once.
func TestRenderQueryHoursWeeklyReport_KeepsTheSevenColumnShape(t *testing.T) {
	body := RenderQueryHoursWeeklyReport(QueryHoursWeeklyReportData{
		Report: queryhoursreport.Report{GeneratedOn: "2026-09-20"},
	})
	for _, header := range []string{
		"Account", "Opportunity", "Total<br/>Query Hour",
		"Project", "Consumed", "Total<br/>Consumed", "Remains",
	} {
		if !strings.Contains(body, header) {
			t.Errorf("missing column header %q", header)
		}
	}
	if !strings.Contains(body, "Hours Exceeded Accounts") ||
		!strings.Contains(body, "Accounts Going to Exceed") {
		t.Error("both tables must always be present, even when empty")
	}
}

// HTML from account, opportunity and project names must not escape into the
// markup.
func TestRenderQueryHoursWeeklyReport_EscapesNames(t *testing.T) {
	body := RenderQueryHoursWeeklyReport(QueryHoursWeeklyReportData{
		Report: queryhoursreport.Report{
			GeneratedOn: "2026-09-20",
			Exceeded: []queryhoursreport.Account{{
				Name: `<script>alert(1)</script>`, RowCount: 1, Exceeded: true,
				Groups: []queryhoursreport.Group{{
					RowCount: 1, Exceeded: true,
					Opportunities: []queryhoursreport.Opportunity{{
						Name: `"><b>x`, Projects: []queryhoursreport.Project{{
							Name: `<img src=x>`, Key: `A&B`,
						}},
					}},
				}},
			}},
		},
	})
	for _, bad := range []string{"<script>", "<img src=x>", `"><b>x`} {
		if strings.Contains(body, bad) {
			t.Errorf("unescaped content in output: %q", bad)
		}
	}
	if !strings.Contains(body, "A&amp;B") {
		t.Error("expected the project key to be HTML-escaped")
	}
}
