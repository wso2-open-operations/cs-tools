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
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestOnboardingStatusEnumLabels(t *testing.T) {
	// The dashboard sends ServiceNow's spellings; each must land on the enum
	// label, regardless of case or separator.
	got, err := onboardingStatusEnumLabels("projectOnboardingStatus", []string{"Not-Started", "In-Progress", "Completed", "OnHold", "Not-Applicable", "Expired", "Cancelled", " on_hold ", "IN PROGRESS"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"NOT_STARTED", "IN_PROGRESS", "COMPLETED", "ON_HOLD", "NOT_APPLICABLE", "EXPIRED", "CANCELLED", "ON_HOLD", "IN_PROGRESS"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("labels = %v, want %v", got, want)
	}

	// An unknown value must fail loudly: silently matching nothing would
	// widen a notIn.
	_, err = onboardingStatusEnumLabels("projectOnboardingStatus", []string{"Completed", "Bogus"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *apierror.ValidationError", err)
	}
}

// --- fetchCaseWatchers: NULL "user".email on a joined watcher row ---
//
// A watcher's joined "user" row is not guaranteed to have an email (see the
// user table's own nullable email column), so the scan must tolerate a SQL
// NULL there rather than assume every watcher resolves to a fully-populated
// user. Regression for a live-data crash: GetCaseByID on a real case
// (CS0439344) panicked with "cannot scan NULL into *string" because the scan
// destination for u.email was a plain string, not a *string.

// fakeCaseWatcherRow is one seeded "work_item_watcher JOIN user" row.
// email is a pointer so a nil value reproduces a NULL "user".email column,
// exactly like fetchCaseWatchers' real query would hand pgx.
type fakeCaseWatcherRow struct {
	id, userName, name string
	email              *string
}

// fakeCaseWatcherRows is a minimal in-memory pgx.Rows over a fixed slice of
// fakeCaseWatcherRow, seeded once per test rather than requiring a live
// Postgres. Only Next/Scan/Err/Close are ever called by fetchCaseWatchers;
// the remaining pgx.Rows methods are stubbed to satisfy the interface.
type fakeCaseWatcherRows struct {
	rows []fakeCaseWatcherRow
	idx  int
}

func (f *fakeCaseWatcherRows) Close()                                       {}
func (f *fakeCaseWatcherRows) Err() error                                   { return nil }
func (f *fakeCaseWatcherRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (f *fakeCaseWatcherRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (f *fakeCaseWatcherRows) Values() ([]any, error)                       { return nil, nil }
func (f *fakeCaseWatcherRows) RawValues() [][]byte                          { return nil }
func (f *fakeCaseWatcherRows) Conn() *pgx.Conn                              { return nil }
func (f *fakeCaseWatcherRows) TypeMap() *pgtype.Map                         { return nil }

func (f *fakeCaseWatcherRows) Next() bool {
	if f.idx >= len(f.rows) {
		return false
	}
	f.idx++
	return true
}

// Scan mirrors fetchCaseWatchers' own column order: id, user_name, name,
// email. dest[3] must accept **string (nil-able), matching u.email's real
// scan destination after the fix -- a plain *string dest here would make
// this fake diverge from what the fix actually needs to handle.
func (f *fakeCaseWatcherRows) Scan(dest ...any) error {
	row := f.rows[f.idx-1]
	*dest[0].(*string) = row.id
	*dest[1].(*string) = row.userName
	*dest[2].(*string) = row.name
	*dest[3].(**string) = row.email
	return nil
}

var _ pgx.Rows = (*fakeCaseWatcherRows)(nil)

// fakeCaseWatcherQuerier satisfies rowsQuerier by handing back a pre-seeded
// fakeCaseWatcherRows, ignoring the SQL and args -- this fake never touches a
// real database.
type fakeCaseWatcherQuerier struct {
	rows *fakeCaseWatcherRows
}

func (f *fakeCaseWatcherQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return f.rows, nil
}

var _ rowsQuerier = (*fakeCaseWatcherQuerier)(nil)

func TestFetchCaseWatchers_NullEmailDoesNotPanic(t *testing.T) {
	withEmail := "watcher.one@example.test"
	q := &fakeCaseWatcherQuerier{rows: &fakeCaseWatcherRows{rows: []fakeCaseWatcherRow{
		{id: "user-1", userName: "watcher.one", name: "Watcher One", email: &withEmail},
		{id: "user-2", userName: "watcher.two", name: "Watcher Two", email: nil},
	}}}

	got, err := fetchCaseWatchers(context.Background(), q, "case-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d watchers, want 2", len(got))
	}

	if got[0].Email != withEmail {
		t.Errorf("watcher[0].Email = %q, want %q", got[0].Email, withEmail)
	}
	if got[0].User == nil || got[0].User.Email != withEmail {
		t.Errorf("watcher[0].User.Email = %v, want %q", got[0].User, withEmail)
	}

	// The NULL-email row is the one that used to crash the scan.
	if got[1].Email != "" {
		t.Errorf("watcher[1].Email (NULL in DB) = %q, want \"\"", got[1].Email)
	}
	if got[1].User == nil || got[1].User.Email != "" {
		t.Errorf("watcher[1].User.Email (NULL in DB) = %v, want \"\"", got[1].User)
	}
}

// Every enum label onboardingStatusLabels can produce must exist in the
// migration's onboarding_status_enum, or a valid filter would fail at query time.
func TestOnboardingStatusLabelsMatchMigration(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0014_projects_table.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range onboardingStatusLabels {
		if !strings.Contains(string(raw), "'"+label+"'") {
			t.Errorf("%s is not an onboarding_status_enum label in migration 0014", label)
		}
	}
}

func TestValidateUpdateCaseFieldsForType(t *testing.T) {
	sev := domain.CaseSeverityHigh
	ws := domain.CaseWorkStateOngoing
	resCode := domain.CaseResolutionCodeSolvedFixedBySupportGuidanceProvided

	tests := []struct {
		name          string
		workItemType  string
		req           domain.UpdateCaseRequest
		state         string
		wantErrSubstr string
	}{
		{
			name:         "CASE allows severity, workState, and resolutionCode",
			workItemType: "CASE",
			req: domain.UpdateCaseRequest{
				Severity:       &sev,
				WorkState:      &ws,
				ResolutionCode: &resCode,
			},
			state: "CLOSED",
		},
		{
			name:         "SECURITY_REPORT_ANALYSIS rejects severity",
			workItemType: "SECURITY_REPORT_ANALYSIS",
			req: domain.UpdateCaseRequest{
				Severity: &sev,
			},
			wantErrSubstr: "severity is only supported for cases",
		},
		{
			name:         "SECURITY_REPORT_ANALYSIS allows workState",
			workItemType: "SECURITY_REPORT_ANALYSIS",
			req: domain.UpdateCaseRequest{
				WorkState: &ws,
			},
		},
		{
			name:         "SECURITY_REPORT_ANALYSIS allows resolutionCode",
			workItemType: "SECURITY_REPORT_ANALYSIS",
			req: domain.UpdateCaseRequest{
				ResolutionCode: &resCode,
			},
			state: "CLOSED",
		},
		{
			name:          "SERVICE_REQUEST rejects severity",
			workItemType:  "SERVICE_REQUEST",
			req:           domain.UpdateCaseRequest{Severity: &sev},
			wantErrSubstr: "severity is only supported for cases",
		},
		{
			name:         "SERVICE_REQUEST allows workState",
			workItemType: "SERVICE_REQUEST",
			req:          domain.UpdateCaseRequest{WorkState: &ws},
		},
		{
			name:         "SERVICE_REQUEST allows resolutionCode",
			workItemType: "SERVICE_REQUEST",
			req:          domain.UpdateCaseRequest{ResolutionCode: &resCode},
			state:        "CLOSED",
		},
		{
			name:         "ENGAGEMENT allows workState",
			workItemType: "ENGAGEMENT",
			req:          domain.UpdateCaseRequest{WorkState: &ws},
		},
		{
			name:          "ENGAGEMENT rejects severity",
			workItemType:  "ENGAGEMENT",
			req:           domain.UpdateCaseRequest{Severity: &sev},
			wantErrSubstr: "severity is only supported for cases",
		},
		{
			name:          "ANNOUNCEMENT rejects workState",
			workItemType:  "ANNOUNCEMENT",
			req:           domain.UpdateCaseRequest{WorkState: &ws},
			wantErrSubstr: "workState is not supported for announcements",
		},
		{
			name:          "ANNOUNCEMENT rejects resolutionCode",
			workItemType:  "ANNOUNCEMENT",
			req:           domain.UpdateCaseRequest{ResolutionCode: &resCode},
			wantErrSubstr: "resolutionCode is not supported for announcements",
		},
		{
			name:          "ANNOUNCEMENT rejects severity",
			workItemType:  "ANNOUNCEMENT",
			req:           domain.UpdateCaseRequest{Severity: &sev},
			wantErrSubstr: "severity is only supported for cases",
		},
		{
			name:         "ANNOUNCEMENT allows CLOSED state",
			workItemType: "ANNOUNCEMENT",
			req:          domain.UpdateCaseRequest{},
			state:        "CLOSED",
		},
		{
			name:         "ANNOUNCEMENT allows OPEN state",
			workItemType: "ANNOUNCEMENT",
			req:          domain.UpdateCaseRequest{},
			state:        "OPEN",
		},
		{
			name:          "ANNOUNCEMENT rejects other states",
			workItemType:  "ANNOUNCEMENT",
			req:           domain.UpdateCaseRequest{},
			state:         "WORK_IN_PROGRESS",
			wantErrSubstr: "announcements only support state open or closed",
		},
		{
			name:         "SECURITY_REPORT_ANALYSIS allows state change without case-only fields",
			workItemType: "SECURITY_REPORT_ANALYSIS",
			req:          domain.UpdateCaseRequest{},
			state:        "CLOSED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpdateCaseFieldsForType(tc.workItemType, tc.req, tc.state)
			if tc.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrSubstr)
			}
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
			}
			if !strings.Contains(ve.Error(), tc.wantErrSubstr) {
				t.Errorf("error = %q, want substring %q", ve.Error(), tc.wantErrSubstr)
			}
		})
	}
}

func TestExtensionUpdateQueriesMatchMigrations(t *testing.T) {
	raw0024, err := os.ReadFile("../../migrations/0024_work_item_extensions.sql")
	if err != nil {
		t.Fatal(err)
	}
	m0024 := string(raw0024)

	queries := []struct {
		name      string
		query     string
		tableName string
		stateEnum string
		causeEnum string
	}{
		{
			name:      "security_report_analysis",
			query:     updateSecurityReportAnalysisQuery,
			tableName: "security_report_analysis",
			stateEnum: "security_report_analysis_state_enum",
			causeEnum: "security_report_analysis_cause_enum",
		},
		{
			name:      "service_request",
			query:     updateServiceRequestQuery,
			tableName: "service_request",
			stateEnum: "service_request_state_enum",
			causeEnum: "service_request_cause_enum",
		},
		{
			name:      "engagement",
			query:     updateEngagementQuery,
			tableName: "engagement",
			stateEnum: "engagement_state_enum",
			causeEnum: "engagement_cause_enum",
		},
		{
			name:      "announcement",
			query:     updateAnnouncementQuery,
			tableName: "announcement",
			stateEnum: "announcement_state_enum",
			causeEnum: "announcement_cause_enum",
		},
	}

	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			if !strings.Contains(m0024, q.tableName) {
				t.Errorf("table %s not found in migration 0024", q.tableName)
			}
			if !strings.Contains(m0024, q.stateEnum) {
				t.Errorf("state enum %s not found in migration 0024", q.stateEnum)
			}
			if !strings.Contains(m0024, q.causeEnum) {
				t.Errorf("cause enum %s not found in migration 0024", q.causeEnum)
			}
			if !strings.Contains(q.query, "UPDATE "+q.tableName) {
				t.Errorf("query does not update %s", q.tableName)
			}
			if !strings.Contains(q.query, q.stateEnum) {
				t.Errorf("query does not cast to %s", q.stateEnum)
			}
			if !strings.Contains(q.query, q.causeEnum) {
				t.Errorf("query does not cast to %s", q.causeEnum)
			}
		})
	}
}

// TestExtensionUpdateQueriesWriteWorkStateAndResolutionCode pins the columns
// migration 0184 adds to the three non-announcement extension tables to the
// update queries that write them, so neither side changes alone.
func TestExtensionUpdateQueriesWriteWorkStateAndResolutionCode(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0184_case_like_work_state_resolution_code.sql")
	if err != nil {
		t.Fatal(err)
	}
	m0184 := string(raw)

	queries := map[string]string{
		"security_report_analysis": updateSecurityReportAnalysisQuery,
		"service_request":          updateServiceRequestQuery,
		"engagement":               updateEngagementQuery,
	}
	for table, query := range queries {
		t.Run(table, func(t *testing.T) {
			for _, col := range []string{"work_state case_work_state_enum", "resolution_code case_resolution_code_enum"} {
				if !strings.Contains(m0184, "ALTER TABLE "+table+" ADD COLUMN IF NOT EXISTS "+col) {
					t.Errorf("migration 0184 does not add %s to %s", col, table)
				}
			}
			for _, want := range []string{"$5::case_work_state_enum", "$6::case_resolution_code_enum", "RETURNING id, state, work_state, closed_on"} {
				if !strings.Contains(query, want) {
					t.Errorf("query does not contain %q", want)
				}
			}
		})
	}
	if strings.Contains(m0184, "ALTER TABLE announcement") {
		t.Error("migration 0184 must not add these columns to announcement")
	}
	if strings.Contains(updateAnnouncementQuery, "work_state") || strings.Contains(updateAnnouncementQuery, "resolution_code") {
		t.Error("updateAnnouncementQuery must not write work_state or resolution_code")
	}
}

