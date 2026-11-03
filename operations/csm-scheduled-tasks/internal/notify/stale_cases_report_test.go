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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
)

func TestRenderStaleCasesReport_TruncationNote(t *testing.T) {
	cases := []entitycases.Case{{Number: "CS0000001"}, {Number: "CS0000002"}}

	capped := RenderStaleCasesReport(StaleCasesReportData{ThresholdDays: 30, Cases: cases, TotalCases: 13642})
	if !strings.Contains(capped, "Showing only the oldest 2 of 13642.") {
		t.Errorf("capped report missing truncation note")
	}
	if !strings.Contains(capped, "<strong>13642</strong>") {
		t.Errorf("capped report headline should show the true total")
	}

	full := RenderStaleCasesReport(StaleCasesReportData{ThresholdDays: 30, Cases: cases, TotalCases: 2})
	if strings.Contains(full, "Showing only") || strings.Contains(full, "[TRUNCATION_NOTE]") {
		t.Errorf("complete report must have no truncation note or leftover placeholder")
	}
}
