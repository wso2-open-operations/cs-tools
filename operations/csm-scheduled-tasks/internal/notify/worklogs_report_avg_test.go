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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

func TestWorklogTeamSectionsWeeklyAvgNA(t *testing.T) {
	r := worklogreport.Report{Teams: []worklogreport.TeamReport{{
		TeamName: "team-a",
		Developers: []worklogreport.Developer{
			{Email: "jane.doe@example.com", WorkedDays: 0, PendingHours: 3, AvgTimePerDay: 0},
			{Email: "john.roe@example.com", WorkedDays: 4, ApprovedHours: 12, AvgTimePerDay: 3},
		},
	}}}

	weekly := renderWorklogTeamSections(r, false, "")
	if !strings.Contains(weekly, ">n/a</td>") {
		t.Errorf("weekly: expected n/a for zero worked days")
	}
	if strings.Contains(weekly, ">0.00</td></tr>") {
		t.Errorf("weekly: zero-worked-days row must not end in 0.00")
	}
	if !strings.Contains(weekly, ">3.00</td></tr>") {
		t.Errorf("weekly: expected 3.00 for the person with worked days")
	}

	daily := renderWorklogTeamSections(r, true, "")
	if strings.Contains(daily, "n/a") {
		t.Errorf("daily: must not render n/a")
	}
}
