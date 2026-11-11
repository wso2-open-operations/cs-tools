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

// SlaReportProject identifies the project an SLA report is for.
type SlaReportProject struct {
	ID   string  `json:"id"`
	Key  string  `json:"key"`
	Name *string `json:"name"`
}

// SlaReportMetric is one clock target (response, workaround or resolution)
// for one case type and severity.
//
// Elapsed is the 95th percentile of the elapsed time of the clocks that have
// finished (stage ACHIEVED or COMPLETED), as a display string, or "—" when
// none has. Target is the policy duration those clocks run against, or "Not
// applicable" when no clock exists. Met, Missed and Running count the latest
// clock of each case: Met finished within the target, Missed went past it
// (finished late, or still running and already over), Running is still inside
// it (in progress or paused). A cancelled clock is in none of them.
type SlaReportMetric struct {
	Elapsed string `json:"elapsed"`
	Target  string `json:"target"`
	Met     int    `json:"met"`
	Missed  int    `json:"missed"`
	Running int    `json:"running"`
}

// SlaReportSummaryRow is one case type and severity.
type SlaReportSummaryRow struct {
	Label      string          `json:"label"`
	CaseType   string          `json:"caseType"`
	Severity   string          `json:"severity"`
	Cases      int             `json:"cases"`
	Response   SlaReportMetric `json:"response"`
	Workaround SlaReportMetric `json:"workaround"`
	Resolution SlaReportMetric `json:"resolution"`
}

// SlaReportClock is the latest clock of one target on one case. Status is one
// of "Met", "Missed", "Running", "Paused" or "Cancelled", empty when the case
// has no clock for the target, and Elapsed is then "—".
type SlaReportClock struct {
	Elapsed string `json:"elapsed"`
	Status  string `json:"status"`
}

// SlaReportCase is one case opened in the report window.
type SlaReportCase struct {
	CaseID     string         `json:"caseId"`
	CaseNumber string         `json:"caseNumber"`
	CaseType   string         `json:"caseType"`
	Severity   string         `json:"severity"`
	State      string         `json:"state"`
	Opened     string         `json:"opened"`
	Response   SlaReportClock `json:"response"`
	Workaround SlaReportClock `json:"workaround"`
	Resolution SlaReportClock `json:"resolution"`
}

// SlaReportResponse is the response for GET /projects/{id}/slas/report.
type SlaReportResponse struct {
	Project SlaReportProject      `json:"project"`
	From    string                `json:"from"`
	To      string                `json:"to"`
	Summary []SlaReportSummaryRow `json:"summary"`
	Cases   []SlaReportCase       `json:"cases"`
}
