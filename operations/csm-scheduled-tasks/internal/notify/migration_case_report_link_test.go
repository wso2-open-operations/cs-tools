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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

func TestRenderMigrationCaseReportLinksByCaseID(t *testing.T) {
	r := migrationreport.Report{
		OlderThanDays: 30,
		CreatedBefore: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		OnHold: []migrationreport.Row{
			{ID: "00000000-0000-0000-0000-000000000001", Number: "CS0000001", IssueKey: "ABC-1",
				CreatedOn: time.Date(2026, 8, 1, 4, 5, 0, 0, time.UTC)},
			{Number: "CS0000002", IssueKey: "ABC-2", CreatedOn: time.Date(2026, 8, 1, 4, 5, 0, 0, time.UTC)},
		},
	}
	body := notify.RenderMigrationCaseReport(r, "https://portal.example.com/cases/")
	if !strings.Contains(body, `href="https://portal.example.com/cases/00000000-0000-0000-0000-000000000001"`) {
		t.Errorf("row is not linked by the case UUID")
	}
	if strings.Contains(body, "/cases/CS0000001") || strings.Contains(body, "/cases/CS0000002") {
		t.Errorf("links must not be built from the case number")
	}
	if strings.Count(body, "<a href") != 1 || !strings.Contains(body, "ABC-2</td>") {
		t.Errorf("a row without an id should render its key as plain text")
	}
	if plain := notify.RenderMigrationCaseReport(r, ""); strings.Contains(plain, "<a href") {
		t.Errorf("no base URL must render plain text")
	}
}
