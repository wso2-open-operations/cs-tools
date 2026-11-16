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

package domain

import "time"

// MTTRReportRequest asks for the MTTR (mean time to resolution) report: for
// each severity band, the average time from a case's creation to the LAST
// time it entered Solution Proposed (a rejected proposal bounces a case back
// for more work before a later one sticks, so the last entry, not the
// first, is when the case was actually resolved), counted over cases whose
// last entry falls within the requested trailing window -- not over cases
// created in the window, which would miss an old case resolved recently and
// bias the average toward fast, recently-opened cases.
type MTTRReportRequest struct {
	// WindowMonths is how many trailing months to measure, counted back from
	// Now. One of 1, 3, 6 or 12.
	WindowMonths int
	// CreTeamIDs restricts the report to cases whose account belongs to one
	// of these CRE teams (account.cre_team_id). Empty means every team.
	CreTeamIDs []string
	// Now is the instant the report is built for.
	Now time.Time
}

// MTTRReportBand is one severity band's MTTR over the requested window.
type MTTRReportBand struct {
	// Band names the severity grouping: "S1-S4", "S1-S3", "S1", "S2", "S3"
	// or "S4".
	Band string `json:"band"`
	// CaseCount is how many cases entered Solution Proposed in the window
	// and are counted in this band's average. Zero means no data --
	// AvgResolutionHours is then also zero, never a divide-by-zero NaN.
	CaseCount int `json:"caseCount"`
	// AvgResolutionHours is the mean time, in hours, from a case's creation
	// to the last time it entered Solution Proposed.
	AvgResolutionHours float64 `json:"avgResolutionHours"`
}

// MTTRReport is the MTTR report: one row per severity band.
type MTTRReport struct {
	GeneratedAt  time.Time `json:"generatedAt"`
	WindowMonths int       `json:"windowMonths"`
	// WindowStart/WindowEnd are the instants the "entered Solution Proposed
	// in window" filter ran against (WindowStart inclusive, WindowEnd
	// exclusive).
	WindowStart time.Time `json:"windowStart"`
	WindowEnd   time.Time `json:"windowEnd"`
	// CreTeamIDs echoes the team restriction actually applied; empty means
	// every team.
	CreTeamIDs []string         `json:"creTeamIds"`
	Bands      []MTTRReportBand `json:"bands"`
}

// MTTRSeverityStat is one severity's raw case count and total resolution
// time, as read from the database -- the unit the service combines into
// the requested bands (S1-S4 needs S1+S2+S3+S4 summed first, not averaged).
type MTTRSeverityStat struct {
	// Severity is the raw case_severity_enum label ("S1".."S4").
	Severity string
	// CaseCount is how many cases of this severity last entered Solution
	// Proposed within the window.
	CaseCount int
	// TotalResolutionSec is the sum, in seconds, of each of those cases'
	// own (last Solution Proposed entry - created_on).
	TotalResolutionSec float64
}
