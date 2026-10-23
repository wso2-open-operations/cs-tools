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

// Runs against a live database like schedule_repo_integration_test.go, and is skipped unless ENTITY_TEST_DATABASE_URL is set.

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	importSunday    = "2026-09-27"
	importNextMon   = "2026-09-28"
	importWednesday = "2026-09-23"
	importThursday  = "2026-09-24"
)

// seedImportRows puts the other engineer on the fixture team inside the week and on the Monday after it, and the lead on the other team inside the week.
func seedImportRows(t *testing.T, repo ScheduleRepository, shift string) {
	t.Helper()
	ctx := context.Background()
	for _, r := range []domain.ApplyScheduleRangeRequest{
		{UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday},
		{UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: importNextMon, To: importNextMon},
		{UserID: schedLeadID, TeamKey: schedOtherTeam, ShiftCode: shift, From: schedMonday, To: schedMonday},
	} {
		if _, err := repo.ApplyRange(ctx, r, schedLeadEmail); err != nil {
			t.Fatalf("seed %+v: %v", r, err)
		}
	}
}

func importWeek(shift string) RotaImportWrite {
	return RotaImportWrite{
		RotaCode: "SRE_SAAS",
		TeamKeys: []string{schedTeamKey},
		From:     schedMonday,
		To:       importSunday,
		Replace:  true,
		Assignments: []domain.ApplyScheduleRangeRequest{
			{UserID: schedMemberID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: importSunday},
		},
		Absences: []domain.ApplyScheduleAbsenceRequest{
			{UserID: schedLeadID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: importWednesday, To: importThursday},
		},
	}
}

func assignmentsOn(t *testing.T, pool *pgxpool.Pool, user, team, from, to string) int {
	t.Helper()
	return countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date BETWEEN $3::date AND $4::date`,
		user, team, from, to)
}

func TestScheduleIntegration_ImportRotaReplacesThePeriodAndWritesTheFile(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	res, err := repo.ImportRota(context.Background(), importWeek(shift), schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota: %v", err)
	}
	if res.Cleared != 1 {
		t.Errorf("cleared %d, want the one row of the team inside the period", res.Cleared)
	}
	if len(res.Assignments) != 1 || res.Assignments[0].Applied != 5 || res.Assignments[0].Skipped != 2 {
		t.Errorf("assignment result %+v, want the 5 weekdays applied and the weekend skipped", res.Assignments)
	}
	if len(res.Absences) != 1 || res.Absences[0].Created != 1 {
		t.Errorf("absence result %+v", res.Absences)
	}

	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, importSunday); n != 0 {
		t.Errorf("%d of the replaced rows are still inside the period", n)
	}
	// The cleared row is in the activity feed, as a delete on the roster would be.
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment_activity
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date = $3::date AND shift_code = $4
		   AND action = 'DELETED' AND actor_email = $5 AND note = $6`,
		schedOtherID, schedTeamKey, schedMonday, shift, schedLeadEmail, rotaImportReplaceNote); n != 1 {
		t.Errorf("%d DELETED activity rows for the cleared assignment, want 1", n)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, importNextMon, importNextMon); n != 1 {
		t.Errorf("the row after the period was touched (%d left)", n)
	}
	if n := assignmentsOn(t, pool, schedLeadID, schedOtherTeam, schedMonday, schedMonday); n != 1 {
		t.Errorf("a team outside the import was touched (%d left)", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 5 {
		t.Errorf("%d imported rows, want 5", n)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence
		 WHERE user_id = $1::uuid AND starts_on = $2::date AND ends_on = $3::date`,
		schedLeadID, importWednesday, importThursday); n != 1 {
		t.Errorf("%d imported absences, want 1", n)
	}
}

func TestScheduleIntegration_ImportRotaIsAllOrNothing(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	// An escalation window with no tier is refused by the apply rules, after the first entry has been written.
	w.Assignments = append(w.Assignments, domain.ApplyScheduleRangeRequest{
		UserID: schedLeadID, TeamKey: schedTeamKey, ShiftCode: "SRE_TZ1", From: schedMonday, To: schedMonday,
	})
	_, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	var invalid *apierror.ValidationError
	if !errors.As(err, &invalid) || !strings.HasPrefix(invalid.Msg, "assignments[1]: ") {
		t.Fatalf("want a ValidationError naming assignments[1], got %v", err)
	}

	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("the replace was not rolled back: %d of the seeded rows left", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 0 {
		t.Errorf("the first entry was kept although a later one failed (%d rows)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedLeadID); n != 0 {
		t.Errorf("%d absences written by a failed import", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_assignment_activity WHERE note = $1 AND team_key = $2`, rotaImportReplaceNote, schedTeamKey); n != 0 {
		t.Errorf("a failed import left %d activity rows for a replace it rolled back", n)
	}
}

func TestScheduleIntegration_ImportRotaDryRunWritesNothing(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	w.DryRun = true
	res, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if res.Cleared != 1 || res.Assignments[0].Applied != 5 || res.Absences[0].Created != 1 {
		t.Errorf("dry run reported %+v, want what the real import would do", res)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("a dry run cleared the period (%d seeded rows left)", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 0 {
		t.Errorf("a dry run wrote %d rows", n)
	}
}

func TestScheduleIntegration_ImportRotaReplaceKeepsTheShiftsAMoveDerives(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)
	// The seeded row inside the week stands in for a shift a move absence derives.
	mustExec(t, pool, `
		UPDATE team_schedule_assignment SET source = 'MOVE'
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date = $3::date`,
		schedOtherID, schedTeamKey, schedMonday)

	res, err := repo.ImportRota(context.Background(), importWeek(shift), schedLeadEmail)
	if err != nil {
		t.Fatalf("ImportRota: %v", err)
	}
	if res.Cleared != 0 {
		t.Errorf("cleared %d, want the move's shift left alone", res.Cleared)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment
		 WHERE user_id = $1::uuid AND team_key = $2 AND rota_date = $3::date AND source = 'MOVE'`,
		schedOtherID, schedTeamKey, schedMonday); n != 1 {
		t.Errorf("a replace removed a shift its move still derives (%d left)", n)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_assignment_activity
		 WHERE user_id = $1::uuid AND action = 'DELETED' AND note = $2`, schedOtherID, rotaImportReplaceNote); n != 0 {
		t.Errorf("%d DELETED activity rows for a shift the replace kept", n)
	}
}

func TestScheduleIntegration_ImportRotaMergeKeepsWhatIsThere(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	seedImportRows(t, repo, shift)

	w := importWeek(shift)
	w.Replace = false
	res, err := repo.ImportRota(context.Background(), w, schedLeadEmail)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.Cleared != 0 {
		t.Errorf("a merge cleared %d rows", res.Cleared)
	}
	if n := assignmentsOn(t, pool, schedOtherID, schedTeamKey, schedMonday, schedMonday); n != 1 {
		t.Errorf("a merge removed somebody the file does not mention (%d left)", n)
	}
	if n := assignmentsOn(t, pool, schedMemberID, schedTeamKey, schedMonday, importSunday); n != 5 {
		t.Errorf("%d merged rows, want 5", n)
	}
}

func TestScheduleIntegration_ImportRotaReplaceCutsTheLeaveUnderAnAssignment(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	// Leave from the Monday to the next Monday: the week's weekdays go, the weekend and the next Monday stay.
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: schedMonday, To: importNextMon,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed leave: %v", err)
	}
	// Leave filed on a team outside the rota is left alone.
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedOtherID, TeamKey: schedOtherTeam, KindCode: "ANNUAL_LEAVE", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed leave on the other team: %v", err)
	}

	w := importWeek(shift)
	w.Absences = nil
	w.Assignments = append(w.Assignments, domain.ApplyScheduleRangeRequest{
		UserID: schedOtherID, TeamKey: schedTeamKey, ShiftCode: shift, From: schedMonday, To: schedMonday,
	})
	res, err := repo.ImportRota(ctx, w, schedLeadEmail)
	if err != nil {
		t.Fatalf("replace over leave: %v", err)
	}
	if res.LeaveWrittenOver != 1 {
		t.Errorf("leave written over %d, want the member's leave cut back to the weekend", res.LeaveWrittenOver)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence
		 WHERE user_id = $1::uuid AND starts_on = $2::date AND ends_on = $3::date`,
		schedMemberID, "2026-09-26", importNextMon); n != 1 {
		t.Errorf("the member's leave was not cut to the weekend and the next Monday (%d)", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedOtherID); n != 1 {
		t.Errorf("leave on a team outside the rota was touched (%d left)", n)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence_activity
		 WHERE user_id = $1::uuid AND action = 'TRIMMED' AND note = $2`, schedMemberID, rotaImportReplaceNote); n != 1 {
		t.Errorf("%d TRIMMED activity rows for the cut leave, want 1", n)
	}
}

func TestScheduleIntegration_ImportRotaReplaceNeverCutsAMove(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	kind := moveKind(t, pool)
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, HomeTeamKey: schedTeamKey, KindCode: kind, From: schedMonday, To: importSunday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed the move: %v", err)
	}

	w := importWeek(shift)
	w.Absences = nil
	w.TeamKeys = []string{schedTeamKey, schedOtherTeam}
	w.Assignments = []domain.ApplyScheduleRangeRequest{
		{UserID: schedMemberID, TeamKey: schedOtherTeam, ShiftCode: shift, From: schedMonday, To: schedMonday},
	}
	res, err := repo.ImportRota(ctx, w, schedLeadEmail)
	if err != nil {
		t.Fatalf("replace inside a move: %v", err)
	}
	if res.LeaveWrittenOver != 0 {
		t.Errorf("leave written over %d, want the move kept", res.LeaveWrittenOver)
	}
	if n := countRows(t, pool, `
		SELECT count(*) FROM team_schedule_absence a JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		 WHERE a.user_id = $1::uuid AND k.code = $2 AND a.starts_on = $3::date AND a.ends_on = $4::date`,
		schedMemberID, kind, schedMonday, importSunday); n != 1 {
		t.Errorf("a replace cut a move (%d left whole)", n)
	}
}

func TestScheduleIntegration_ImportRotaMergeKeepsTheLeave(t *testing.T) {
	repo, pool := newScheduleIntegrationRepo(t)
	ctx := context.Background()
	shift := shiftWithScope(t, pool, "CRE", "WEEKDAY")
	if _, err := repo.ApplyAbsence(ctx, domain.ApplyScheduleAbsenceRequest{
		UserID: schedMemberID, TeamKey: schedTeamKey, KindCode: "ANNUAL_LEAVE", From: schedMonday, To: schedMonday,
	}, schedLeadEmail); err != nil {
		t.Fatalf("seed leave: %v", err)
	}

	w := importWeek(shift)
	w.Replace = false
	w.Absences = nil
	res, err := repo.ImportRota(ctx, w, schedLeadEmail)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.LeaveWrittenOver != 0 {
		t.Errorf("a merge wrote over %d leave spans", res.LeaveWrittenOver)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM team_schedule_absence WHERE user_id = $1::uuid`, schedMemberID); n != 1 {
		t.Errorf("a merge removed leave (%d left)", n)
	}
}
