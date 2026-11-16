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

// The worklog report's move spans against a live PostgreSQL, on the schedule
// fixtures. Skipped unless ENTITY_TEST_DATABASE_URL is set; see
// schedule_repo_integration_test.go for how to build the database.

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Leave taken during a move cuts the move for its days and is filed under the
// move's team. The report reads that leave as a day on the move's team, so the
// person is not filed back under their own team while on leave. Leave filed
// under their own team is not a move.
func TestScheduleIntegration_WorklogMoveSpansKeepLeaveOnTheMovesTeam(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	kind := moveKind(t, pool)
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM team_schedule_absence WHERE user_id = ANY($1::uuid[])`,
			[]string{schedMemberID, schedOtherID})
	})
	const wednesday, friday = "2026-09-23", "2026-09-25"

	// Moved Monday to Friday, on leave Wednesday from the move's team row.
	for _, req := range []domain.ApplyScheduleAbsenceRequest{
		{UserID: schedMemberID, TeamKey: schedTeamKey, HomeTeamKey: schedTeamKey, KindCode: kind, From: schedMonday, To: friday},
		{UserID: schedMemberID, TeamKey: schedOtherTeam, KindCode: "ANNUAL_LEAVE", From: wednesday, To: wednesday},
		// Somebody on leave from their own team, with no move.
		{UserID: schedOtherID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: wednesday, To: wednesday},
	} {
		if _, err := repo.ApplyAbsence(ctx, req, schedLeadEmail); err != nil {
			t.Fatalf("ApplyAbsence(%s %s): %v", req.KindCode, req.From, err)
		}
	}

	from, _ := time.Parse("2006-01-02", schedMonday)
	to, _ := time.Parse("2006-01-02", friday)
	spans, err := NewWorklogReportRepository(NewScoped(pool)).MoveSpans(ctx, from, to)
	if err != nil {
		t.Fatalf("MoveSpans: %v", err)
	}
	days := map[string]bool{}
	for _, s := range spans {
		switch s.UserID {
		case schedOtherID:
			t.Fatalf("leave on their own team was read as a move: %+v", s)
		case schedMemberID:
			if s.TeamKey != schedOtherTeam {
				t.Fatalf("span on %q, want every span on %q: %+v", s.TeamKey, schedOtherTeam, s)
			}
			end := s.EndsOn
			if end.IsZero() {
				end = to
			}
			for d := s.StartsOn; !d.After(end); d = d.AddDate(0, 0, 1) {
				days[d.Format("2006-01-02")] = true
			}
		}
	}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if !days[d.Format("2006-01-02")] {
			t.Errorf("%s is not on %q; spans %+v", d.Format("2006-01-02"), schedOtherTeam, spans)
		}
	}
}
