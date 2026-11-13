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

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// An allocation tag whose excludes_from_timesheet is set takes the person off
// the worklog report; one without it leaves them on. Run against the real
// query (skipped unless ENTITY_TEST_DATABASE_URL is set, see
// schedule_repo_integration_test.go).
func TestWorklogIntegration_LeaveFollowsTheTagsTimesheetFlag(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()

	const excluding, keeping = "WLFIXTURE_EXCLUDING", "WLFIXTURE_KEEPING"
	cleanup := func() {
		mustExec(t, pool, `DELETE FROM team_schedule_absence
		                    WHERE kind_id IN (SELECT id FROM team_schedule_absence_kind WHERE code = ANY($1))`,
			[]string{excluding, keeping})
		mustExec(t, pool, `DELETE FROM team_schedule_absence_kind WHERE code = ANY($1)`, []string{excluding, keeping})
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, k := range []struct {
		code, short string
		excludes    bool
	}{{excluding, "WLX", true}, {keeping, "WLK", false}} {
		if _, err := repo.CreateAbsenceKind(ctx, k.code, domain.CreateScheduleAbsenceKindRequest{
			ShortCode: k.short, Label: k.code, Bucket: "ALLOCATION", ColourToken: "INT",
			ExcludesFromTimesheet: k.excludes,
		}, nil, schedLeadEmail); err != nil {
			t.Fatalf("create kind %s: %v", k.code, err)
		}
	}
	// A date range nothing else in a developer's database is likely to use.
	for user, code := range map[string]string{schedMemberID: excluding, schedOtherID: keeping} {
		mustExec(t, pool, `
			INSERT INTO team_schedule_absence (user_id, team_key, kind_id, starts_on, ends_on)
			SELECT $1::uuid, $2, id, '2031-03-03', '2031-03-07' FROM team_schedule_absence_kind WHERE code = $3`,
			user, schedTeamKey, code)
	}

	from := time.Date(2031, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2031, 3, 31, 0, 0, 0, 0, time.UTC)
	rows, err := NewWorklogReportRepository(NewScoped(pool)).Leave(ctx, from, to)
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}
	var memberOff, otherOff bool
	for _, r := range rows {
		switch r.UserID {
		case schedMemberID:
			memberOff = true
		case schedOtherID:
			otherOff = true
		}
	}
	if !memberOff {
		t.Error("a tag that excludes from the timesheet did not take the person off the report")
	}
	if otherOff {
		t.Error("a tag that does not exclude from the timesheet took the person off the report")
	}
}
