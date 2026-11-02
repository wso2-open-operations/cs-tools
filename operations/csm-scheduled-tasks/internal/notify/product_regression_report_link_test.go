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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
)

func TestRegressionCaseLink(t *testing.T) {
	row := regressionreport.Row{
		ID: "11111111-2222-4333-8444-555555555555", Number: "CS0000001", IssueKey: "EXAMPLESUB-1",
	}

	got := regressionCaseLink("https://portal.example.com/cases/", row)
	if !strings.Contains(got, `href="https://portal.example.com/cases/11111111-2222-4333-8444-555555555555"`) ||
		!strings.Contains(got, ">EXAMPLESUB-1</a>") {
		t.Errorf("linked cell = %s, want a link to the case id labelled with the issue key", got)
	}

	if got := regressionCaseLink("", row); got != "EXAMPLESUB-1" {
		t.Errorf("no base = %q, want the plain issue key", got)
	}
	if got := regressionCaseLink("https://portal.example.com/cases", regressionreport.Row{Number: "CS0000001"}); got != "CS0000001" {
		t.Errorf("no id = %q, want the plain number and no link", got)
	}
}
