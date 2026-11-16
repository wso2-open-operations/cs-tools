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

package notify_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

func openCasesFixture() []longopenreport.Row {
	created := time.Date(2026, 8, 1, 4, 5, 0, 0, time.UTC)
	return []longopenreport.Row{
		{ID: "00000000-0000-0000-0000-000000000001", Number: "CS0000001", Type: "case", State: "open", CreatedOn: created, UpdatedOn: created},
		{ID: "00000000-0000-0000-0000-000000000002", Number: "CS0000002", Type: "service_request", State: "open", CreatedOn: created, UpdatedOn: created},
		{ID: "00000000-0000-0000-0000-000000000003", Number: "CS0000003", Type: "security_report_analysis", State: "open", CreatedOn: created, UpdatedOn: created},
		{Number: "CS0000004", Type: "case", State: "open", CreatedOn: created, UpdatedOn: created},
		{ID: "00000000-0000-0000-0000-000000000005", Number: "CS0000005", Type: "engagement", State: "open", CreatedOn: created, UpdatedOn: created},
	}
}

func TestRenderOpenCasesReportLinksPerType(t *testing.T) {
	body := notify.RenderOpenCasesReport(notify.OpenCasesReportData{
		Report: longopenreport.Report{Cases: openCasesFixture()}, CaseLinkBase: "https://portal.example.com/",
	})
	for _, want := range []string{
		`href="https://portal.example.com/cases/00000000-0000-0000-0000-000000000001"`,
		`href="https://portal.example.com/operations/service-requests/00000000-0000-0000-0000-000000000002"`,
		`href="https://portal.example.com/security-center/security-reports/00000000-0000-0000-0000-000000000003"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	// No id, or a type with no portal route here: plain text, no broken link.
	if strings.Count(body, "<a href") != 3 || !strings.Contains(body, "CS0000004<br/>") || !strings.Contains(body, "CS0000005<br/>") {
		t.Errorf("rows without an id or a known route must be plain text")
	}
}

func TestRenderOpenCasesReportTypeColumn(t *testing.T) {
	body := notify.RenderOpenCasesReport(notify.OpenCasesReportData{Report: longopenreport.Report{Cases: openCasesFixture()}})
	for _, want := range []string{">Type</th>", ">Case</td>", ">Service request</td>", ">Security report analysis</td>"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(body, "<a href") {
		t.Errorf("no base URL must render plain text")
	}
	if strings.Contains(body, "<!-- [") {
		t.Errorf("body still carries a template placeholder")
	}
}

func TestRenderOpenCasesReportEmpty(t *testing.T) {
	body := notify.RenderOpenCasesReport(notify.OpenCasesReportData{})
	if !strings.Contains(body, `colspan="8"`) || !strings.Contains(body, "No matching cases.") {
		t.Errorf("empty report should span the 8 columns with the no-match row")
	}
}
