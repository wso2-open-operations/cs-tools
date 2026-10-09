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

// The CSM portal's "Change case type" action (PATCH /cases/{id} {type, ...})
// failed on the Postgres and dual-write data sources because the service
// refused any type. TransferCaseType now replaces the extension row of the old
// type with one of the new type in a single transaction. These tests run it
// against a real Postgres, as a role that does not bypass row-level security,
// for both flavours of "case": an Incident (S0-S3) and a Query (S4), moved out
// of and into "case". Skipped without CASE_STATS_TEST_DSN, which needs
// migrations 0184 (work_state/resolution_code on the other types) and 0210
// (case_attachment references work_item).
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run CaseTypeTransferIntegration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	ctIncidentID     = "92000000-0000-0000-0000-000000000001" // a case at S2: an Incident
	ctQueryID        = "92000000-0000-0000-0000-000000000002" // a case at S4: a Query
	ctEngagementID   = "92000000-0000-0000-0000-000000000003"
	ctServiceReqID   = "92000000-0000-0000-0000-000000000004"
	ctSRAID          = "92000000-0000-0000-0000-000000000005"
	ctAnnouncementID = "92000000-0000-0000-0000-000000000006"
	ctChangeReqID    = "92000000-0000-0000-0000-000000000007"
	ctIncidentWIID   = "92000000-0000-0000-0000-000000000008"
	ctMissingID      = "92000000-0000-0000-0000-0000000000ff"
	ctUserID         = "92000000-0000-0000-0000-0000000000a1"
	ctActor          = "transfer.tester@example.com"
)

var ctAllIDs = []string{ctIncidentID, ctQueryID, ctEngagementID, ctServiceReqID, ctSRAID, ctAnnouncementID, ctChangeReqID, ctIncidentWIID}

type ctFixture struct {
	pool   *pgxpool.Pool
	scoped *repository.Scoped
	repo   repository.CaseTypeTransferRepository
	ctx    context.Context
	userID string
}

func (f *ctFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.scoped.Exec(f.ctx, sql, args...); err != nil {
		t.Fatalf("exec (%.90s): %v", sql, err)
	}
}

func (f *ctFixture) clean() {
	for _, id := range ctAllIDs {
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM time_card WHERE case_id = $1`, id)
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM case_attachment WHERE case_id = $1`, id)
		_, _ = f.scoped.Exec(f.ctx, `DELETE FROM work_item WHERE id = $1`, id)
	}
	_, _ = f.scoped.Exec(f.ctx, `DELETE FROM "user" WHERE id = $1`, ctUserID)
}

// newCTFixture seeds, for every test, an Incident (S2) and a Query (S4) "case",
// an engagement, a service request, a security report analysis and an
// announcement, each with a time card, and the two cases with an attachment.
func newCTFixture(t *testing.T) *ctFixture {
	t.Helper()
	pool := caseStatsPool(t)
	f := &ctFixture{
		pool:   pool,
		scoped: repository.NewScoped(pool),
		ctx:    repository.WithSystemIdentity(context.Background()),
	}
	repo, ok := repository.NewCaseRepository(f.scoped).(repository.CaseTypeTransferRepository)
	if !ok {
		t.Fatal("the case repository does not implement CaseTypeTransferRepository")
	}
	f.repo = repo
	f.clean()
	t.Cleanup(f.clean)

	// The pool's role must not bypass row-level security, or this proves nothing.
	var bypass bool
	if err := pool.QueryRow(f.ctx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass); err != nil {
		t.Fatalf("role check: %v", err)
	}
	if bypass {
		t.Fatal("CASE_STATS_TEST_DSN logs in as a role that bypasses row-level security; use a plain login")
	}
	// Time cards and attachments name a user, and a freshly migrated database has
	// none, so the fixture brings its own.
	now := time.Now().UTC()
	f.userID = ctUserID
	f.exec(t, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, is_active)
		VALUES ($1, $2, $2, 'type.transfer.fixture', $3, TRUE)`, ctUserID, now, ctActor)
	workItem := func(id, number, kind string) {
		f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES ($1, $2, $2, 'test', 'test', $3, $3, 'type transfer fixture', $4::work_item_type_enum)`, id, now, number, kind)
	}
	timeCard := func(id string, billable bool) {
		f.exec(t, `INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state)
			VALUES (gen_random_uuid(), $2, $2, 'test', 'test', $1, $3, CURRENT_DATE, $4, 'SUBMITTED')`, id, now, f.userID, billable)
	}
	attachment := func(id string) {
		f.exec(t, `INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status)
			VALUES (gen_random_uuid(), $1, 'type-transfer-test', 'a.txt', 'text/plain', 1, $2, 'complete')`, id, f.userID)
	}
	closedOn := now.Add(-time.Hour)

	workItem(ctIncidentID, "CT-TEST-0001", "CASE")
	f.exec(t, `INSERT INTO "case" (id, severity, issue_type, state, work_state, close_notes, cause, resolution_code, closed_on, autoclosure_step)
		VALUES ($1, 'S2', 'ERROR', 'WORK_IN_PROGRESS', 'ONGOING', 'incident notes', 'PRODUCT_BUG', 'SOLVED_WORKAROUND_PROVIDED', $2, 'DEFAULT')`, ctIncidentID, closedOn)
	timeCard(ctIncidentID, false) // an Incident's time cards are not billable
	attachment(ctIncidentID)

	workItem(ctQueryID, "CT-TEST-0002", "CASE")
	f.exec(t, `INSERT INTO "case" (id, severity, issue_type, state, work_state, close_notes)
		VALUES ($1, 'S4', 'QUESTION', 'AWAITING_INFO', 'PAUSED', 'query notes')`, ctQueryID)
	timeCard(ctQueryID, true) // a Query's are
	attachment(ctQueryID)

	workItem(ctEngagementID, "CT-TEST-0003", "ENGAGEMENT")
	f.exec(t, `INSERT INTO engagement (id, state, close_notes, cause, work_state, type, payment_type)
		VALUES ($1, 'WAITING_ON_WSO2', 'engagement notes', 'UNKNOWN', 'ONGOING', 'CONSULTANCY', 'PAID')`, ctEngagementID)
	timeCard(ctEngagementID, false)

	workItem(ctServiceReqID, "CT-TEST-0004", "SERVICE_REQUEST")
	f.exec(t, `INSERT INTO service_request (id, state, close_notes, category) VALUES ($1, 'OPEN', 'sr notes', 'Some catalog')`, ctServiceReqID)
	timeCard(ctServiceReqID, false)

	workItem(ctSRAID, "CT-TEST-0005", "SECURITY_REPORT_ANALYSIS")
	f.exec(t, `INSERT INTO security_report_analysis (id, state, close_notes) VALUES ($1, 'SOLUTION_PROPOSED', 'sra notes')`, ctSRAID)
	timeCard(ctSRAID, false)

	workItem(ctAnnouncementID, "CT-TEST-0006", "ANNOUNCEMENT")
	f.exec(t, `INSERT INTO announcement (id, announcement_type) VALUES ($1, 'GENERAL')`, ctAnnouncementID)
	return f
}

// ctExtensionTables is every extension table a case-like work item can have a row in.
var ctExtensionTables = map[string]string{
	"case":                     `"case"`,
	"engagement":               "engagement",
	"service_request":          "service_request",
	"security_report_analysis": "security_report_analysis",
}

// extensionRows counts the rows of id in each extension table, so a test can
// assert exactly one of them holds it.
func (f *ctFixture) extensionRows(t *testing.T, id string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for name, table := range ctExtensionTables {
		var n int
		if err := f.scoped.QueryRow(f.ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		out[name] = n
	}
	return out
}

func (f *ctFixture) assertOnlyIn(t *testing.T, id, wantType string) {
	t.Helper()
	for name, n := range f.extensionRows(t, id) {
		want := 0
		if name == wantType {
			want = 1
		}
		if n != want {
			t.Errorf("extension table %s holds %d row(s) of %s, want %d", name, n, id, want)
		}
	}
}

func (f *ctFixture) workItemType(t *testing.T, id string) string {
	t.Helper()
	var got string
	if err := f.scoped.QueryRow(f.ctx, `SELECT type::TEXT FROM work_item WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read work_item.type: %v", err)
	}
	return got
}

func (f *ctFixture) billable(t *testing.T, id string) bool {
	t.Helper()
	var got bool
	if err := f.scoped.QueryRow(f.ctx, `SELECT is_billable FROM time_card WHERE case_id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read time card: %v", err)
	}
	return got
}

func (f *ctFixture) attachments(t *testing.T, id string) int {
	t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.ctx, `SELECT COUNT(*) FROM case_attachment WHERE case_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	return n
}

func ctSev(s domain.CaseSeverity) *domain.CaseSeverity { return &s }

func ctEngagementPlan(id string) repository.CaseTypeTransfer {
	et, pt := domain.EngagementTypeMigration, domain.EngagementPaymentTypeFOC
	return repository.CaseTypeTransfer{CaseID: id, TargetType: "engagement", EngagementType: &et, EngagementPaymentType: &pt, ActorEmail: ctActor}
}

func ctCasePlan(id string, sev domain.CaseSeverity) repository.CaseTypeTransfer {
	it := domain.CaseIssueTypeError
	return repository.CaseTypeTransfer{CaseID: id, TargetType: "case", Severity: &sev, IssueType: &it, ActorEmail: ctActor}
}

// An Incident (S2) and a Query (S4) are each moved to every other type. What
// the case carries -- state, work state, close notes, cause, resolution code,
// closed time -- follows it; its number, attachments and time cards stay; the
// old extension row is gone and exactly one row of the new type exists.
func TestCaseTypeTransferIntegration_OutOfCase_IncidentAndQuery(t *testing.T) {
	sources := []struct {
		name         string
		id           string
		wantPrevSev  domain.CaseSeverity
		wantState    domain.CaseState
		wantWork     domain.CaseWorkState
		wantNotes    string
		billableWas  bool
		billableNow  bool // S4 leaves the Query boundary; an Incident is untouched
		wantCause    string
		wantResCode  string
		hasClosedOn  bool
		hasAutoclose bool
	}{
		{name: "Incident (S2)", id: ctIncidentID, wantPrevSev: domain.CaseSeverityHigh, wantState: domain.CaseStateWorkInProgress, wantWork: domain.CaseWorkStateOngoing,
			wantNotes: "incident notes", billableWas: false, billableNow: false, wantCause: "PRODUCT_BUG", wantResCode: "SOLVED_WORKAROUND_PROVIDED", hasClosedOn: true, hasAutoclose: true},
		{name: "Query (S4)", id: ctQueryID, wantPrevSev: domain.CaseSeverityLow, wantState: domain.CaseStateAwaitingInfo, wantWork: domain.CaseWorkStatePaused,
			wantNotes: "query notes", billableWas: true, billableNow: false},
	}
	targets := map[string]func(id string) repository.CaseTypeTransfer{
		"engagement": ctEngagementPlan,
		"service_request": func(id string) repository.CaseTypeTransfer {
			return repository.CaseTypeTransfer{CaseID: id, TargetType: "service_request", ActorEmail: ctActor}
		},
		"security_report_analysis": func(id string) repository.CaseTypeTransfer {
			return repository.CaseTypeTransfer{CaseID: id, TargetType: "security_report_analysis", ActorEmail: ctActor}
		},
	}
	targetTables := map[string]string{"engagement": "engagement", "service_request": "service_request", "security_report_analysis": "security_report_analysis"}

	for _, src := range sources {
		for target, planFor := range targets {
			t.Run(src.name+" to "+target, func(t *testing.T) {
				f := newCTFixture(t)
				if got := f.billable(t, src.id); got != src.billableWas {
					t.Fatalf("fixture: billable = %v, want %v", got, src.billableWas)
				}

				res, err := f.repo.TransferCaseType(f.ctx, planFor(src.id), nil)
				if err != nil {
					t.Fatalf("TransferCaseType: %v", err)
				}

				if res.PreviousType != "case" || res.Type != target {
					t.Errorf("result types = %s -> %s", res.PreviousType, res.Type)
				}
				if res.PreviousSeverity == nil || *res.PreviousSeverity != src.wantPrevSev {
					t.Errorf("PreviousSeverity = %v, want %v", res.PreviousSeverity, src.wantPrevSev)
				}
				if res.Severity != nil {
					t.Errorf("a transfer out of case has no severity, got %v", *res.Severity)
				}
				if res.State == nil || *res.State != src.wantState {
					t.Errorf("State = %v, want %v", res.State, src.wantState)
				}
				if res.WorkState == nil || *res.WorkState != src.wantWork {
					t.Errorf("WorkState = %v, want %v", res.WorkState, src.wantWork)
				}

				f.assertOnlyIn(t, src.id, target)
				if got := f.workItemType(t, src.id); got != map[string]string{"engagement": "ENGAGEMENT", "service_request": "SERVICE_REQUEST", "security_report_analysis": "SECURITY_REPORT_ANALYSIS"}[target] {
					t.Errorf("work_item.type = %s", got)
				}

				table := targetTables[target]
				var state, notes, workState string
				if err := f.scoped.QueryRow(f.ctx,
					`SELECT state::TEXT, COALESCE(close_notes, ''), COALESCE(work_state::TEXT, '') FROM `+table+` WHERE id = $1`, src.id).
					Scan(&state, &notes, &workState); err != nil {
					t.Fatalf("read new row: %v", err)
				}
				if state != upper(string(src.wantState)) || notes != src.wantNotes || workState != upper(string(src.wantWork)) {
					t.Errorf("new row carries state=%s notes=%q work_state=%s", state, notes, workState)
				}
				var cause, resCode string
				var closedOnSet, autocloseSet bool
				if err := f.scoped.QueryRow(f.ctx,
					`SELECT COALESCE(cause::TEXT, ''), COALESCE(resolution_code::TEXT, ''), closed_on IS NOT NULL, autoclosure_step IS NOT NULL FROM `+table+` WHERE id = $1`, src.id).
					Scan(&cause, &resCode, &closedOnSet, &autocloseSet); err != nil {
					t.Fatalf("read new row: %v", err)
				}
				if cause != src.wantCause || resCode != src.wantResCode || closedOnSet != src.hasClosedOn || autocloseSet != src.hasAutoclose {
					t.Errorf("new row carries cause=%q resolution_code=%q closedOn=%v autoclosure=%v", cause, resCode, closedOnSet, autocloseSet)
				}

				if target == "engagement" {
					var et, pt string
					if err := f.scoped.QueryRow(f.ctx, `SELECT type::TEXT, payment_type::TEXT FROM engagement WHERE id = $1`, src.id).Scan(&et, &pt); err != nil {
						t.Fatalf("read engagement: %v", err)
					}
					if et != "MIGRATION" || pt != "FOC" {
						t.Errorf("engagement type/payment type = %s/%s, want MIGRATION/FOC", et, pt)
					}
				}

				var number, updatedBy string
				if err := f.scoped.QueryRow(f.ctx, `SELECT number, updated_by FROM work_item WHERE id = $1`, src.id).Scan(&number, &updatedBy); err != nil {
					t.Fatalf("read work item: %v", err)
				}
				if number == "" || updatedBy != ctActor {
					t.Errorf("number=%q updated_by=%q, want the number kept and the actor stamped", number, updatedBy)
				}
				if got := f.attachments(t, src.id); got != 1 {
					t.Errorf("attachments after the transfer = %d, want 1", got)
				}
				if got := f.billable(t, src.id); got != src.billableNow {
					t.Errorf("time card billable = %v, want %v", got, src.billableNow)
				}
			})
		}
	}
}

func upper(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'a' && c <= 'z' {
			out[i] = c - 32
		}
	}
	return string(out)
}

// Every other type becomes a case. The severity decides what it is: S4 is a
// Query, whose time cards turn billable, and S0-S3 an Incident, whose do not.
func TestCaseTypeTransferIntegration_IntoCase_QueryAndIncident(t *testing.T) {
	sources := map[string]string{"engagement": ctEngagementID, "service_request": ctServiceReqID, "security_report_analysis": ctSRAID}
	severities := []struct {
		name         string
		sev          domain.CaseSeverity
		wantEnum     string
		wantBillable bool
	}{
		{"Query (S4)", domain.CaseSeverityLow, "S4", true},
		{"Incident (S3)", domain.CaseSeverityMedium, "S3", false},
		{"Incident (S0)", domain.CaseSeverityCatastrophic, "S0", false},
	}
	for from, id := range sources {
		for _, sv := range severities {
			t.Run(from+" to "+sv.name, func(t *testing.T) {
				f := newCTFixture(t)
				res, err := f.repo.TransferCaseType(f.ctx, ctCasePlan(id, sv.sev), nil)
				if err != nil {
					t.Fatalf("TransferCaseType: %v", err)
				}
				if res.PreviousType != from || res.Type != "case" || res.PreviousSeverity != nil {
					t.Errorf("result = %+v", res)
				}
				if res.Severity == nil || *res.Severity != sv.sev {
					t.Errorf("Severity = %v, want %v", res.Severity, sv.sev)
				}

				f.assertOnlyIn(t, id, "case")
				if got := f.workItemType(t, id); got != "CASE" {
					t.Errorf("work_item.type = %s", got)
				}
				var severity, issueType, state string
				if err := f.scoped.QueryRow(f.ctx, `SELECT severity::TEXT, issue_type::TEXT, state::TEXT FROM "case" WHERE id = $1`, id).Scan(&severity, &issueType, &state); err != nil {
					t.Fatalf("read case row: %v", err)
				}
				if severity != sv.wantEnum || issueType != "ERROR" {
					t.Errorf("case row severity/issue_type = %s/%s, want %s/ERROR", severity, issueType, sv.wantEnum)
				}
				if res.State == nil || upper(string(*res.State)) != state {
					t.Errorf("state %s was not carried (result %v)", state, res.State)
				}
				if got := f.billable(t, id); got != sv.wantBillable {
					t.Errorf("time card billable = %v, want %v", got, sv.wantBillable)
				}
			})
		}
	}
}

// What ServiceNow reports after the transfer is what Postgres ends up showing.
func TestCaseTypeTransferIntegration_RemoteStateWins(t *testing.T) {
	f := newCTFixture(t)
	var sawBefore repository.CaseTypeTransferBefore
	res, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctIncidentID), func(_ context.Context, before repository.CaseTypeTransferBefore) (*repository.CaseTypeTransferRemoteResult, error) {
		sawBefore = before
		st := domain.CaseStateOpen
		return &repository.CaseTypeTransferRemoteResult{State: &st}, nil
	})
	if err != nil {
		t.Fatalf("TransferCaseType: %v", err)
	}
	if sawBefore.PreviousType != "case" || sawBefore.PreviousSeverity == nil || *sawBefore.PreviousSeverity != domain.CaseSeverityHigh {
		t.Errorf("remote step was told %+v", sawBefore)
	}
	var state string
	if err := f.scoped.QueryRow(f.ctx, `SELECT state::TEXT FROM engagement WHERE id = $1`, ctIncidentID).Scan(&state); err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	if state != "OPEN" || res.State == nil || *res.State != domain.CaseStateOpen {
		t.Errorf("state = %s (result %v), want ServiceNow's OPEN", state, res.State)
	}
}

// ServiceNow refusing the transfer undoes all of it: the "case" row is still
// there, the type unchanged, no engagement row, the time card and attachment
// untouched.
func TestCaseTypeTransferIntegration_RemoteRefusalRollsEverythingBack(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		want bool // the Query's billable flag must still be true: nothing was re-marked
	}{{"Query (S4)", ctQueryID, true}, {"Incident (S2)", ctIncidentID, false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCTFixture(t)
			refusal := &apierror.ValidationError{Msg: "servicenow says no"}
			_, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(tc.id), func(context.Context, repository.CaseTypeTransferBefore) (*repository.CaseTypeTransferRemoteResult, error) {
				return nil, refusal
			})
			if !errors.Is(err, refusal) {
				t.Fatalf("want the refusal back, got %v", err)
			}
			f.assertOnlyIn(t, tc.id, "case")
			if got := f.workItemType(t, tc.id); got != "CASE" {
				t.Errorf("work_item.type = %s, want CASE", got)
			}
			if got := f.billable(t, tc.id); got != tc.want {
				t.Errorf("time card billable = %v, want it left at %v", got, tc.want)
			}
			if got := f.attachments(t, tc.id); got != 1 {
				t.Errorf("attachments = %d, want 1", got)
			}
		})
	}
}

func TestCaseTypeTransferIntegration_Refusals(t *testing.T) {
	f := newCTFixture(t)
	et := domain.EngagementTypeMigration
	sev := domain.CaseSeverityLow

	tests := []struct {
		name    string
		plan    repository.CaseTypeTransfer
		wantErr any
	}{
		{"same type", repository.CaseTypeTransfer{CaseID: ctEngagementID, TargetType: "engagement", EngagementType: &et, EngagementPaymentType: func() *domain.EngagementPaymentType { p := domain.EngagementPaymentTypePaid; return &p }()}, &apierror.ValidationError{}},
		{"an announcement cannot be transferred", ctEngagementPlan(ctAnnouncementID), &apierror.ValidationError{}},
		{"unknown id", ctEngagementPlan(ctMissingID), &apierror.NotFoundError{}},
		{"announcement is not a target", repository.CaseTypeTransfer{CaseID: ctQueryID, TargetType: "announcement"}, &apierror.ValidationError{}},
		{"engagement without a payment type", repository.CaseTypeTransfer{CaseID: ctQueryID, TargetType: "engagement", EngagementType: &et}, &apierror.ValidationError{}},
		{"case without an issue type", repository.CaseTypeTransfer{CaseID: ctEngagementID, TargetType: "case", Severity: &sev}, &apierror.ValidationError{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.repo.TransferCaseType(f.ctx, tc.plan, nil)
			switch tc.wantErr.(type) {
			case *apierror.ValidationError:
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want a ValidationError, got %v", err)
				}
			case *apierror.NotFoundError:
				var ne *apierror.NotFoundError
				if !errors.As(err, &ne) {
					t.Fatalf("want a NotFoundError, got %v", err)
				}
			}
		})
	}
	// Nothing above changed anything.
	f.assertOnlyIn(t, ctQueryID, "case")
	f.assertOnlyIn(t, ctEngagementID, "engagement")
}

// The table's old foreign key to "case"(id) was also what kept an attachment off
// every work item that is not a case. It now points at work_item (migration 0210,
// so a case that changes type keeps its attachments), and the insert itself keeps
// the narrower rule: the four case-like types own attachments, an announcement, a
// change request or an incident does not. This guards both insert paths.
func TestCaseTypeTransferIntegration_OnlyCaseLikeWorkItemsOwnAttachments(t *testing.T) {
	f := newCTFixture(t)
	cases := repository.NewCaseRepository(f.scoped)
	now := time.Now().UTC()
	for _, w := range []struct{ id, number, kind string }{
		{ctChangeReqID, "CT-TEST-0007", "CHANGE_REQUEST"},
		{ctIncidentWIID, "CT-TEST-0008", "INCIDENT"},
	} {
		f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
			VALUES ($1, $2, $2, 'test', 'test', $3, 'type transfer fixture', $4::work_item_type_enum)`, w.id, now, w.number, w.kind)
	}

	key := "type-transfer-test"
	create := func(id string) error {
		_, err := cases.CreateCaseAttachment(f.ctx, domain.CreateAttachmentRequest{
			ReferenceID: id, ReferenceType: domain.ReferenceTypeCase, Name: "b.txt", Type: "text/plain",
			StorageKey: &key, SizeBytes: 1, CreatedBy: f.userID, Status: domain.AttachmentStatusComplete,
		})
		return err
	}
	fromServiceNow := func(id string) error {
		_, err := cases.CreateCaseAttachmentFromServiceNow(f.ctx, domain.CreateAttachmentRequest{
			ReferenceID: id, ReferenceType: domain.ReferenceTypeCase, Name: "c.txt", Type: "text/plain",
		}, "92000000-0000-0000-0000-0000000000b0", 1, f.userID)
		return err
	}

	owners := map[string]string{
		"case": ctQueryID, "engagement": ctEngagementID, "service request": ctServiceReqID, "security report analysis": ctSRAID,
	}
	for name, id := range owners {
		if err := create(id); err != nil {
			t.Errorf("a %s must be able to own an attachment: %v", name, err)
		}
	}
	if err := fromServiceNow(ctQueryID); err != nil {
		t.Errorf("a ServiceNow-sourced attachment on a case: %v", err)
	}
	_, _ = f.scoped.Exec(f.ctx, `DELETE FROM case_attachment WHERE id = '92000000-0000-0000-0000-0000000000b0'`)

	for name, id := range map[string]string{"announcement": ctAnnouncementID, "change request": ctChangeReqID, "incident": ctIncidentWIID, "unknown id": ctMissingID} {
		for path, do := range map[string]func(string) error{"create": create, "from ServiceNow": fromServiceNow} {
			var ve *apierror.ValidationError
			if err := do(id); !errors.As(err, &ve) {
				t.Errorf("%s attachment on a %s: want a ValidationError, got %v", path, name, err)
			}
		}
		if got := f.attachments(t, id); got != 0 {
			t.Errorf("%s ended up owning %d attachment(s)", name, got)
		}
	}
}
