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

// Characterisation (golden-value) tests for the SLA clock engine's service
// layer, written BEFORE the engine is refactored (calendar abstraction,
// generic subject core, sla_policy.applies_to) so that a later change can
// prove it altered nothing. They pin what the code does today, including
// behaviour that looks odd ("QUIRK"); they are not a statement that the
// behaviour is right.
//
//   - The resolver tests (no database) pin which policies match a fixed set
//     of seeded policies for a representative set of case and incident facts,
//     and how a policy's condition text is classified as a case policy, an
//     incident policy or neither.
//   - The decision-order test (no database) pins which repository call the
//     engine makes for a clock, given which of the stop/cancel/pause/resume
//     conditions are true.
//   - The end-to-end tests run the engine over the real repository against a
//     real PostgreSQL and are skipped unless ENTITY_TEST_DATABASE_URL is set
//     (apply every migration first):
//
//     ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run SLAEngineCharacterisation ./internal/service/
//
// Time: the end-to-end tests that do not depend on "now" assert exactly (fixed
// instants; recalculation replays fixed activity timestamps); those that do
// (the flat paths read the database's NOW(), the business-day paths Go's
// time.Now()) assert with a computed expectation and a tolerance, as the
// repository characterisation tests do.

package service

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/businessdays"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ---------------------------------------------------------------------------
// Resolver: which policies match which facts (no database)
// ---------------------------------------------------------------------------

// slaCharCasePolicies is the fixed policy set the case matrix runs against.
// It mixes the two condition dialects the resolver sees: source SERVICENOW
// rows carry raw ServiceNow-encoded text (translated before evaluation) and
// source CSM rows carry the neutral vocabulary (evaluated as written). The
// last few rows of RESPONSE can never match a case (non-case classification).
// Slice order is the order ListCandidatePolicies would return (by id), and is
// the order of every expected result.
func slaCharCasePolicies() map[string][]repository.SLAPolicyRef {
	const (
		incidentCaseTypeSysID = "8d4b87bd1b18f010cb6898aebd4bcb59" // translates to priority IN S0..S3
		queryCaseTypeSysID    = "0d5b8fbd1b18f010cb6898aebd4bcba5" // translates to priority = S4
		secReportCaseTypeID   = "ab36479047ccf510a0a29cd3846d43ee" // translates to case_type = security_report_analysis
	)
	sn := func(id, target, start string) repository.SLAPolicyRef {
		return repository.SLAPolicyRef{ID: id, Name: id, Target: target, Duration: time.Hour, Source: "SERVICENOW", StartCondition: start}
	}
	csm := func(id, target, schedule, start string) repository.SLAPolicyRef {
		return repository.SLAPolicyRef{ID: id, Name: id, Target: target, Duration: time.Hour, Source: "CSM", Schedule: schedule, StartCondition: start}
	}
	return map[string][]repository.SLAPolicyRef{
		"RESPONSE": {
			sn("r-s1-os", "RESPONSE", "state=1^u_case_type="+incidentCaseTypeSysID+"^priority=10^u_wso2_response_comment=false^EQ"),
			sn("r-query", "RESPONSE", "u_case_type="+queryCaseTypeSysID+"^state=1^u_wso2_response_comment=false^EQ"),
			sn("r-sra", "RESPONSE", "state=1^u_case_type="+secReportCaseTypeID+"^u_wso2_response_comment=false^EQ"),
			csm("r-service-request", "RESPONSE", slaCharBusinessDays, "case_type=service_request^state=OPEN^case_typeISNOTEMPTY"),
			csm("r-cloud", "RESPONSE", "", "product=Asgardeo^state=OPEN^priority=S1^cs_engineer_response_comment=false"),
			csm("r-unsupported-schedule", "RESPONSE", "8-5 Weekdays", "case_type=case^priority=S2^state=OPEN"),
			csm("r-date", "RESPONSE", "", "case_type=case^opened_at>=2024-01-01^priority=S3^state=OPEN"),
			csm("r-account", "RESPONSE", "", "case_type=case^accountISNOTEMPTY^priority=S3^state=OPEN"),
			// Never case policies: classified out before any condition is evaluated.
			csm("r-escalation", "RESPONSE", "", "active_escalation.escalation_severity=abc^active_escalation.state=101"),
			csm("r-incident-shaped", "RESPONSE", "", "business_service=abc^category=x"),
			csm("r-bare-priority", "RESPONSE", "", "priority=S1^state=OPEN"),
		},
		"WORKAROUND": {
			csm("w-subscription", "WORKAROUND", "", "project_type=Subscription^ORproject_type=Managed Cloud Subscription^case_type=case^priority=S1"),
			csm("w-s0", "WORKAROUND", "", "case_type=case^priority=S0"),
			csm("w-no-workaround-yet", "WORKAROUND", "", "case_type=case^u_workaround_providedISEMPTY^priority=S2"),
		},
		"RESOLUTION": {
			csm("z-p0", "RESOLUTION", "", "priority=S0^case_type=case"),
			csm("z-hosting-ms", "RESOLUTION", "", "case_type=hosting^priority=S1"),
			csm("z-hosting-os", "RESOLUTION", "", "case_typeINcase,hosting^priority=S1"),
			csm("z-no-eta-yet", "RESOLUTION", "", "case_type=case^fix_eta_sharedISEMPTY^priority=S2"),
			csm("z-low-or-announcement", "RESOLUTION", "", "priorityINS3,S4^case_type=case^NQcase_type=announcement"),
		},
	}
}

const slaCharBusinessDays = "WSO2 Business days"

// TestSLAEngineCharacterisation_CaseResolverMatrix pins resolve() for a fixed
// policy set against a representative set of case facts.
func TestSLAEngineCharacterisation_CaseResolverMatrix(t *testing.T) {
	d := func(s string) time.Time {
		tm, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	type want struct{ response, workaround, resolution []string }
	cases := []struct {
		name  string
		facts repository.CaseFacts
		want  want
	}{
		{"S1 case, subscription project",
			repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case", ProjectType: "Subscription", CreatedOn: d("2024-06-01")},
			want{[]string{"r-s1-os"}, []string{"w-subscription"}, []string{"z-hosting-os"}}},
		{"S1 case on a cloud product (two response policies, in id order) and another project type",
			repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case", Product: "Asgardeo", ProjectType: "Other", CreatedOn: d("2024-06-01")},
			want{[]string{"r-s1-os", "r-cloud"}, nil, []string{"z-hosting-os"}}},
		{"S1 case already replied to: every response policy needs cs_engineer_response_comment=false",
			repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case", Product: "Asgardeo", CSEngineerResponseComment: true},
			want{nil, nil, []string{"z-hosting-os"}}},
		{"S1 hosting case matches both hosting resolution policies (and r-s1-os, whose translated condition has no case_type clause)",
			repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "hosting"},
			want{[]string{"r-s1-os"}, nil, []string{"z-hosting-ms", "z-hosting-os"}}},
		{"S0 case",
			repository.CaseFacts{State: "OPEN", Severity: "S0", CaseType: "case"},
			want{nil, []string{"w-s0"}, []string{"z-p0"}}},
		{"S4 case (Query)",
			repository.CaseFacts{State: "OPEN", Severity: "S4", CaseType: "case"},
			want{[]string{"r-query"}, nil, []string{"z-low-or-announcement"}}},
		{"S4 announcement matches r-query because that policy has no case_type clause, and the NQ branch of z-low-or-announcement",
			repository.CaseFacts{State: "OPEN", Severity: "S4", CaseType: "announcement"},
			want{[]string{"r-query"}, nil, []string{"z-low-or-announcement"}}},
		{"no state recorded: every state= clause fails",
			repository.CaseFacts{Severity: "S1", CaseType: "case", ProjectType: "Subscription"},
			want{nil, []string{"w-subscription"}, []string{"z-hosting-os"}}},
		{"closed case: response policies need state=OPEN, the others have no state clause",
			repository.CaseFacts{State: "CLOSED", Severity: "S1", CaseType: "case", ProjectType: "Subscription"},
			want{nil, []string{"w-subscription"}, []string{"z-hosting-os"}}},
		{"service request with no severity",
			repository.CaseFacts{State: "OPEN", CaseType: "service_request"},
			want{[]string{"r-service-request"}, nil, nil}},
		{"security report analysis with no severity",
			repository.CaseFacts{State: "OPEN", CaseType: "security_report_analysis"},
			want{[]string{"r-sra"}, nil, nil}},
		{"S2 case: unsupported schedule is evaluated anyway; nothing provided yet",
			repository.CaseFacts{State: "OPEN", Severity: "S2", CaseType: "case", CreatedOn: d("2024-06-01")},
			want{[]string{"r-unsupported-schedule"}, []string{"w-no-workaround-yet"}, []string{"z-no-eta-yet"}}},
		{"S2 case with a workaround and a fix ETA already given",
			repository.CaseFacts{State: "OPEN", Severity: "S2", CaseType: "case", WorkaroundProvided: true, FixEtaShared: true},
			want{[]string{"r-unsupported-schedule"}, nil, nil}},
		{"S3 case with an account, created after the date threshold",
			repository.CaseFacts{State: "OPEN", Severity: "S3", CaseType: "case", HasAccount: true, CreatedOn: d("2024-06-01")},
			want{[]string{"r-date", "r-account"}, nil, []string{"z-low-or-announcement"}}},
		{"S3 case with an account, created before the date threshold",
			repository.CaseFacts{State: "OPEN", Severity: "S3", CaseType: "case", HasAccount: true, CreatedOn: d("2023-06-01")},
			want{[]string{"r-account"}, nil, []string{"z-low-or-announcement"}}},
		{"S3 case with no account, created after the date threshold",
			repository.CaseFacts{State: "OPEN", Severity: "S3", CaseType: "case", CreatedOn: d("2024-06-01")},
			want{[]string{"r-date"}, nil, []string{"z-low-or-announcement"}}},
		{"S3 case with no creation date: the date clause fails",
			repository.CaseFacts{State: "OPEN", Severity: "S3", CaseType: "case", HasAccount: true},
			want{[]string{"r-account"}, nil, []string{"z-low-or-announcement"}}},
		{"values compare case-insensitively",
			repository.CaseFacts{State: "open", Severity: "s1", CaseType: "CASE", ProjectType: "subscription"},
			want{[]string{"r-s1-os"}, []string{"w-subscription"}, []string{"z-hosting-os"}}},
		{"empty facts match nothing",
			repository.CaseFacts{},
			want{nil, nil, nil}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newSLAPolicyResolver(&fakeCandidatePolicyRepo{byTarget: slaCharCasePolicies()})
			for target, wantIDs := range map[string][]string{"RESPONSE": c.want.response, "WORKAROUND": c.want.workaround, "RESOLUTION": c.want.resolution} {
				got, err := r.resolve(context.Background(), target, c.facts)
				if err != nil {
					t.Fatalf("%s: resolve: %v", target, err)
				}
				var ids []string
				for _, p := range got {
					ids = append(ids, p.ID)
				}
				if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
					t.Errorf("%s: matched %v, want %v", target, ids, wantIDs)
				}
			}
		})
	}
}

// TestSLAEngineCharacterisation_IncidentResolverMatrix pins resolveIncident()
// for the real "Cloud Incident - High Priority" policy plus a few
// look-alikes that must never match an incident.
func TestSLAEngineCharacterisation_IncidentResolverMatrix(t *testing.T) {
	choreo := uuidToSysid(snChoreoServiceID)
	candidates := cloudIncidentPolicyCandidates()
	candidates["RESOLUTION"] = append(candidates["RESOLUTION"],
		repository.SLAPolicyRef{ID: "inc-choreo-moderate", Name: "inc-choreo-moderate", Target: "RESOLUTION", Duration: time.Hour, Source: "CSM",
			StartCondition: "business_service=" + choreo + "^priority=MODERATE"},
		repository.SLAPolicyRef{ID: "case-shaped", Name: "case-shaped", Target: "RESOLUTION", Duration: time.Hour, Source: "CSM",
			StartCondition: "case_type=case^priority=S1"},
		repository.SLAPolicyRef{ID: "escalation-shaped", Name: "escalation-shaped", Target: "RESOLUTION", Duration: time.Hour, Source: "CSM",
			StartCondition: "business_service=" + choreo + "^escalation_severity=1"},
		repository.SLAPolicyRef{ID: "bare-priority", Name: "bare-priority", Target: "RESOLUTION", Duration: time.Hour, Source: "CSM",
			StartCondition: "priority=CRITICAL"},
	)
	cases := []struct {
		name  string
		facts repository.IncidentFacts
		want  []string
	}{
		{"critical, Choreo, service interruption", repository.IncidentFacts{State: "NEW", Priority: "CRITICAL", Category: "SERVICE_INTERRUPTION", ServiceID: snChoreoServiceID}, []string{"cloud-incident-high-priority"}},
		{"high, Asgardeo, service interruption", repository.IncidentFacts{State: "NEW", Priority: "HIGH", Category: "SERVICE_INTERRUPTION", ServiceID: snAsgardeoServiceID}, []string{"cloud-incident-high-priority"}},
		{"moderate, Choreo", repository.IncidentFacts{State: "NEW", Priority: "MODERATE", Category: "SERVICE_INTERRUPTION", ServiceID: snChoreoServiceID}, []string{"inc-choreo-moderate"}},
		{"critical, Choreo, another category", repository.IncidentFacts{State: "NEW", Priority: "CRITICAL", Category: "OTHER", ServiceID: snChoreoServiceID}, nil},
		{"critical, no service", repository.IncidentFacts{State: "NEW", Priority: "CRITICAL", Category: "SERVICE_INTERRUPTION"}, nil},
		{"critical, another service", repository.IncidentFacts{State: "NEW", Priority: "CRITICAL", Category: "SERVICE_INTERRUPTION", ServiceID: "11111111-2222-3333-4444-555555555555"}, nil},
		{"low, Choreo", repository.IncidentFacts{State: "NEW", Priority: "LOW", Category: "SERVICE_INTERRUPTION", ServiceID: snChoreoServiceID}, nil},
		{"empty facts", repository.IncidentFacts{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newSLAPolicyResolver(&fakeCandidatePolicyRepo{byTarget: candidates})
			got, err := r.resolveIncident(context.Background(), "RESOLUTION", c.facts)
			if err != nil {
				t.Fatalf("resolveIncident: %v", err)
			}
			var ids []string
			for _, p := range got {
				ids = append(ids, p.ID)
			}
			if strings.Join(ids, ",") != strings.Join(c.want, ",") {
				t.Errorf("matched %v, want %v", ids, c.want)
			}
			// RESPONSE and WORKAROUND have no incident policies at all.
			for _, target := range []string{"RESPONSE", "WORKAROUND"} {
				if other, _ := r.resolveIncident(context.Background(), target, c.facts); len(other) != 0 {
					t.Errorf("%s matched %v, want none", target, other)
				}
			}
		})
	}
}

// TestSLAEngineCharacterisation_PolicyClassification pins isCasePolicy and
// isIncidentPolicy for representative condition text. Classification looks at
// the RAW (untranslated) text of all five condition columns together.
func TestSLAEngineCharacterisation_PolicyClassification(t *testing.T) {
	cases := []struct {
		name               string
		conditions         []string
		wantCase, wantInci bool
	}{
		{"case_type marker", []string{"state=OPEN^case_type=case"}, true, false},
		{"raw ServiceNow u_case_type contains the case_type marker", []string{"u_case_type=8d4b87bd1b18f010cb6898aebd4bcb59^state=1^EQ"}, true, false},
		{"product marker alone", []string{"product=Asgardeo"}, true, false},
		{"project_type marker alone", []string{"project_type=Subscription"}, true, false},
		{"account marker alone", []string{"accountISNOTEMPTY"}, true, false},
		{"opened_at marker alone", []string{"opened_at>=2024-01-01"}, true, false},
		{"marker only in a later column still counts", []string{"priority=S1", "case_type=case"}, true, false},
		{"bare priority/state is neither", []string{"priority=S1^state=OPEN"}, false, false},
		{"incident marker business_service", []string{"business_service=abc^priority=HIGH"}, false, true},
		{"business_service with category=", []string{"business_service=abc^category=service_interruption"}, false, true},
		{"business_service plus a case marker is neither", []string{"business_service=abc^case_type=case"}, false, false},
		{"escalation marker", []string{"escalation_severity=1"}, false, false},
		{"business_service plus an escalation marker is neither", []string{"business_service=abc^source_table=x"}, false, false},
		{"active_escalation alone", []string{"active_escalation.state=101"}, false, false},
		{"category= alone is neither", []string{"category=x"}, false, false},
		{"empty", []string{""}, false, false},
		{"no columns", nil, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isCasePolicy(c.conditions...); got != c.wantCase {
				t.Errorf("isCasePolicy = %v, want %v", got, c.wantCase)
			}
			if got := isIncidentPolicy(c.conditions...); got != c.wantInci {
				t.Errorf("isIncidentPolicy = %v, want %v", got, c.wantInci)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Decision order (no database)
// ---------------------------------------------------------------------------

// TestSLAEngineCharacterisation_DecisionOrder pins which repository call
// ApplyCaseStateEffects makes for one open clock: stop (complete) beats
// cancel beats pause beats resume; the CLOSED safety net completes a clock
// that no stop/cancel settled, even one that was just paused.
func TestSLAEngineCharacterisation_DecisionOrder(t *testing.T) {
	const (
		always = "case_typeISNOTEMPTY" // true for any case with a type
		never  = "case_type=nothing-like-this"
		id     = "case-1"
	)
	complete := id + "|RESOLUTION"
	cancel := id + "|RESOLUTION"
	pause := id + "|RESOLUTION|true"
	resume := id + "|RESOLUTION|false"
	type calls struct{ completed, cancelled, paused []string }
	one := func(s string) []string { return []string{s} }

	cases := []struct {
		name   string
		policy repository.SLAPolicyRef
		state  domain.CaseState
		want   calls
	}{
		{"stop, cancel, pause and resume all true: only complete",
			repository.SLAPolicyRef{StopCondition: always, CancelCondition: always, PauseCondition: always, ResumeCondition: always},
			domain.CaseStateOpen, calls{completed: one(complete)}},
		{"cancel, pause and resume true: only cancel",
			repository.SLAPolicyRef{StopCondition: never, CancelCondition: always, PauseCondition: always, ResumeCondition: always},
			domain.CaseStateOpen, calls{cancelled: one(cancel)}},
		{"pause and resume true: only pause",
			repository.SLAPolicyRef{StopCondition: never, CancelCondition: never, PauseCondition: always, ResumeCondition: always},
			domain.CaseStateOpen, calls{paused: one(pause)}},
		{"resume only",
			repository.SLAPolicyRef{StopCondition: never, CancelCondition: never, PauseCondition: never, ResumeCondition: always},
			domain.CaseStateOpen, calls{paused: one(resume)}},
		{"nothing true: nothing happens",
			repository.SLAPolicyRef{StopCondition: never, CancelCondition: never, PauseCondition: never, ResumeCondition: never},
			domain.CaseStateOpen, calls{}},
		{"no conditions at all: nothing happens",
			repository.SLAPolicyRef{},
			domain.CaseStateOpen, calls{}},
		{"when_to_cancel NEVER ignores a true cancel condition",
			repository.SLAPolicyRef{CancelCondition: always, PauseCondition: always, WhenToCancel: "NEVER"},
			domain.CaseStateOpen, calls{paused: one(pause)}},
		{"when_to_cancel START_CONDITIONS_ARE_NOT_MET cancels when the start condition is false",
			repository.SLAPolicyRef{StartCondition: never, WhenToCancel: "START_CONDITIONS_ARE_NOT_MET"},
			domain.CaseStateOpen, calls{cancelled: one(cancel)}},
		{"when_to_cancel START_CONDITIONS_ARE_NOT_MET with no start condition never cancels",
			repository.SLAPolicyRef{WhenToCancel: "START_CONDITIONS_ARE_NOT_MET"},
			domain.CaseStateOpen, calls{}},
		{"when_to_cancel START_CONDITIONS_ARE_NOT_MET ignores the cancel condition",
			repository.SLAPolicyRef{StartCondition: always, CancelCondition: always, WhenToCancel: "START_CONDITIONS_ARE_NOT_MET"},
			domain.CaseStateOpen, calls{}},
		{"when_to_resume PAUSE_CONDITIONS_ARE_NOT_MET resumes when the pause condition is false",
			repository.SLAPolicyRef{PauseCondition: never, WhenToResume: "PAUSE_CONDITIONS_ARE_NOT_MET"},
			domain.CaseStateOpen, calls{paused: one(resume)}},
		{"when_to_resume PAUSE_CONDITIONS_ARE_NOT_MET with no pause condition never resumes",
			repository.SLAPolicyRef{ResumeCondition: always, WhenToResume: "PAUSE_CONDITIONS_ARE_NOT_MET"},
			domain.CaseStateOpen, calls{}},
		{"CLOSED with no stop condition: the safety net completes",
			repository.SLAPolicyRef{},
			domain.CaseStateClosed, calls{completed: one(complete)}},
		{"CLOSED with a matching stop condition completes once, not twice",
			repository.SLAPolicyRef{StopCondition: always},
			domain.CaseStateClosed, calls{completed: one(complete)}},
		{"CLOSED with a matching cancel condition cancels and the safety net does not fire",
			repository.SLAPolicyRef{CancelCondition: always},
			domain.CaseStateClosed, calls{cancelled: one(cancel)}},
		{"CLOSED with a matching pause condition pauses and then the safety net completes",
			repository.SLAPolicyRef{PauseCondition: always},
			domain.CaseStateClosed, calls{paused: one(pause), completed: one(complete)}},
		{"state is matched against the policy: stop on state=CLOSED",
			repository.SLAPolicyRef{StopCondition: "state=CLOSED"},
			domain.CaseStateClosed, calls{completed: one(complete)}},
		{"state is matched against the policy: pause on state=AWAITING_INFO",
			repository.SLAPolicyRef{PauseCondition: "state=AWAITING_INFO", ResumeCondition: "state=WORK_IN_PROGRESS"},
			domain.CaseStateAwaitingInfo, calls{paused: one(pause)}},
		{"state is matched against the policy: resume on state=WORK_IN_PROGRESS",
			repository.SLAPolicyRef{PauseCondition: "state=AWAITING_INFO", ResumeCondition: "state=WORK_IN_PROGRESS"},
			domain.CaseStateWorkInProgress, calls{paused: one(resume)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newFakeFullSLAEngineRepo()
			repo.factsByCase[id] = repository.CaseFacts{CaseType: "case"}
			policy := c.policy
			policy.ID, policy.Name, policy.Target = "pol", "pol", "RESOLUTION"
			repo.activeByCaseTarget[id+"|RESOLUTION"] = policy
			NewSLAEngineService(repo).ApplyCaseStateEffects(context.Background(), id, c.state)

			if fmt.Sprint(repo.completed) != fmt.Sprint(c.want.completed) {
				t.Errorf("completed = %v, want %v", repo.completed, c.want.completed)
			}
			if fmt.Sprint(repo.clockCancelled) != fmt.Sprint(c.want.cancelled) {
				t.Errorf("cancelled = %v, want %v", repo.clockCancelled, c.want.cancelled)
			}
			if fmt.Sprint(repo.paused) != fmt.Sprint(c.want.paused) {
				t.Errorf("paused = %v, want %v", repo.paused, c.want.paused)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End to end over a real database
// ---------------------------------------------------------------------------

const (
	slaSvcCharActor  = "sla-svc-char-test"
	slaSvcCharPrefix = "SLA svc characterisation "
	slaCharTolerance = 5 * time.Second
)

type slaSvcCharEnv struct {
	t          *testing.T
	ctx        context.Context
	pool       *pgxpool.Pool
	scoped     *repository.Scoped
	repo       repository.SLAEngineRepository
	workItems  []string
	policyIDs  []string
	seq        int
	policyName map[string]bool
}

func newSLASvcCharEnv(t *testing.T) *slaSvcCharEnv {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database SLA engine characterisation tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	e := &slaSvcCharEnv{t: t, ctx: ctx, pool: pool, scoped: scoped, repo: repository.NewSLAEngineRepository(scoped, ""), policyName: map[string]bool{}}
	if _, err := scoped.Exec(ctx, `DELETE FROM sla_policy WHERE name LIKE $1 AND created_by = $2`, slaSvcCharPrefix+"%", slaSvcCharActor); err != nil {
		t.Fatalf("pre-clean policies: %v", err)
	}
	t.Cleanup(func() {
		for _, id := range e.workItems {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla WHERE work_item_id = $1::uuid`, id)
			_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id = $1::uuid`, id)
		}
		for _, id := range e.policyIDs {
			_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, id)
		}
	})
	return e
}

func (e *slaSvcCharEnv) workItem() string {
	e.t.Helper()
	var id string
	if err := e.scoped.QueryRow(e.ctx, `
		INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1,
		        'SLASVC' || substr(md5(random()::text), 1, 10), 'SLASVC-' || substr(md5(random()::text), 1, 10),
		        'sla svc characterisation test case', 'CASE')
		RETURNING id::text`, slaSvcCharActor).Scan(&id); err != nil {
		e.t.Fatalf("seed work_item: %v", err)
	}
	e.workItems = append(e.workItems, id)
	return id
}

// slaSvcPolicySpec is a source='SERVICENOW' (or CSM) sla_policy to insert.
type slaSvcPolicySpec struct {
	target, schedule, source           string
	duration                           time.Duration
	start, stop, pause, resume, cancel string
	retroactive                        bool
	whenToCancel, whenToResume         string
}

func (e *slaSvcCharEnv) policy(s slaSvcPolicySpec) repository.SLAPolicyRef {
	e.t.Helper()
	e.seq++
	name := fmt.Sprintf("%s%s #%d", slaSvcCharPrefix, strings.ToLower(e.t.Name()), e.seq)
	if s.source == "" {
		s.source = "SERVICENOW"
	}
	var id string
	err := e.scoped.QueryRow(e.ctx, `
		INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, schedule, source,
		                        start_condition, stop_condition, pause_condition, resume_condition, cancel_condition, retroactive,
		                        when_to_cancel, when_to_resume)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2, TRUE, $3::sla_policy_target_enum, $4::interval, NULLIF($5, ''), $6::sla_source_enum,
		        NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), $12,
		        NULLIF($13, '')::sla_policy_when_to_cancel_enum, NULLIF($14, '')::sla_policy_when_to_resume_enum)
		RETURNING id::text`,
		slaSvcCharActor, name, s.target, fmt.Sprintf("%d seconds", int64(s.duration.Seconds())), s.schedule, s.source,
		s.start, s.stop, s.pause, s.resume, s.cancel, s.retroactive, s.whenToCancel, s.whenToResume).Scan(&id)
	if err != nil {
		e.t.Fatalf("seed sla_policy: %v", err)
	}
	e.policyIDs = append(e.policyIDs, id)
	e.policyName[name] = true
	return repository.SLAPolicyRef{ID: id, Name: name, Target: s.target, Duration: s.duration, Schedule: s.schedule}
}

// engineRepo is the real repository with the two reads that the end-to-end
// tests control: the case's facts (a bare work_item has none of the rows the
// real GetCaseFacts joins) and the candidate list (limited to this test's own
// policies, so other SERVICENOW rows in a shared database cannot interfere).
type slaSvcCharEngineRepo struct {
	repository.SLAEngineRepository
	facts repository.CaseFacts
	names map[string]bool
}

func (r *slaSvcCharEngineRepo) GetCaseFacts(context.Context, string) (repository.CaseFacts, error) {
	return r.facts, nil
}

func (r *slaSvcCharEngineRepo) ListCandidatePolicies(ctx context.Context, target string) ([]repository.SLAPolicyRef, error) {
	all, err := r.SLAEngineRepository.ListCandidatePolicies(ctx, target)
	if err != nil {
		return nil, err
	}
	var mine []repository.SLAPolicyRef
	for _, p := range all {
		if r.names[p.Name] {
			mine = append(mine, p)
		}
	}
	return mine, nil
}

type slaSvcRow struct {
	Stage       string
	IsActive    *bool
	HasBreached *bool
	StartOn     time.Time
	EndOn       *time.Time
	PausedOn    *time.Time
	PauseDur    *float64
	BPauseDur   *float64
	BusinessDur *float64
	Remaining   *float64
	Pct         *float64
	Schedule    *string
	UpdatedBy   string
}

func (e *slaSvcCharEnv) rows(workItemID string, p repository.SLAPolicyRef) map[string]slaSvcRow {
	e.t.Helper()
	rs, err := e.scoped.Query(e.ctx, `
		SELECT stage::text, is_active, has_breached, start_on, end_on, paused_on,
		       EXTRACT(EPOCH FROM pause_duration)::float8, EXTRACT(EPOCH FROM business_pause_duration)::float8,
		       EXTRACT(EPOCH FROM business_duration)::float8, EXTRACT(EPOCH FROM remaining_business_duration)::float8,
		       business_elapsed_percentage::float8, schedule, updated_by
		FROM sla WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid`, workItemID, p.ID)
	if err != nil {
		e.t.Fatalf("read rows: %v", err)
	}
	defer rs.Close()
	out := map[string]slaSvcRow{}
	for rs.Next() {
		var r slaSvcRow
		if err := rs.Scan(&r.Stage, &r.IsActive, &r.HasBreached, &r.StartOn, &r.EndOn, &r.PausedOn, &r.PauseDur, &r.BPauseDur,
			&r.BusinessDur, &r.Remaining, &r.Pct, &r.Schedule, &r.UpdatedBy); err != nil {
			e.t.Fatalf("scan: %v", err)
		}
		if _, dup := out[r.Stage]; dup {
			e.t.Fatalf("two %s rows for %s", r.Stage, p.Name)
		}
		out[r.Stage] = r
	}
	return out
}

func (e *slaSvcCharEnv) one(workItemID string, p repository.SLAPolicyRef) slaSvcRow {
	e.t.Helper()
	rs := e.rows(workItemID, p)
	if len(rs) != 1 {
		e.t.Fatalf("policy %s: %d rows, want exactly 1", p.Name, len(rs))
	}
	for _, r := range rs {
		return r
	}
	panic("unreachable")
}

// insertClock inserts an sla row directly in a chosen stored state.
func (e *slaSvcCharEnv) insertClock(workItemID string, p repository.SLAPolicyRef, stage string, start time.Time, pausedOn *time.Time, pause time.Duration) {
	e.t.Helper()
	if _, err := e.scoped.Exec(e.ctx, `
		INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, schedule, is_active, stage,
		                 has_breached, start_on, paused_on, duration, pause_duration, business_elapsed_percentage, business_duration,
		                 remaining_business_duration)
		VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, $2::uuid, $3::uuid, NULLIF($4, ''), TRUE, $5::sla_stage_enum,
		        FALSE, $6, $7, $8::interval, $9::interval, 0, INTERVAL '0', $8::interval)`,
		slaSvcCharActor, workItemID, p.ID, p.Schedule, stage, start, pausedOn,
		fmt.Sprintf("%d seconds", int64(p.Duration.Seconds())), fmt.Sprintf("%d seconds", int64(pause.Seconds()))); err != nil {
		e.t.Fatalf("insert clock: %v", err)
	}
}

func (e *slaSvcCharEnv) activity(workItemID string, at time.Time, field, oldV, newV string) {
	e.t.Helper()
	if _, err := e.scoped.Exec(e.ctx, `
		INSERT INTO work_item_activity (id, created_on, created_by, work_item_id, field_name, old_value, new_value)
		VALUES (gen_random_uuid(), $1, $2, $3::uuid, $4, $5, $6)`,
		at, slaSvcCharActor, workItemID, field, oldV, newV); err != nil {
		e.t.Fatalf("insert activity: %v", err)
	}
}

func slaSvcNY(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tm, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm.UTC()
}

func svcSecs(t *testing.T, label string, got *float64, want, tol time.Duration) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want %v", label, want)
		return
	}
	if math.Abs(*got-want.Seconds()) > tol.Seconds()+1e-6 {
		t.Errorf("%s = %vs, want %v (+/- %v)", label, *got, want, tol)
	}
}

func svcNull(t *testing.T, label string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %vs, want NULL", label, *got)
	}
}

func svcStage(t *testing.T, label string, r slaSvcRow, stage string, active bool) {
	t.Helper()
	if r.Stage != stage {
		t.Errorf("%s: stage = %s, want %s", label, r.Stage, stage)
	}
	if r.IsActive == nil || *r.IsActive != active {
		t.Errorf("%s: is_active = %v, want %v", label, r.IsActive, active)
	}
}

func svcTimeNear(t *testing.T, label string, got *time.Time, want time.Time, tol time.Duration) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = NULL, want ~%v", label, want)
		return
	}
	if d := got.Sub(want); d < -tol || d > tol {
		t.Errorf("%s = %v, want %v (+/- %v)", label, got.UTC(), want.UTC(), tol)
	}
}

// slaSvcCharPolicies seeds the response (flat) and resolution (business-day,
// retroactive, pausable) policies the lifecycle tests share.
func slaSvcCharPolicies(e *slaSvcCharEnv) (response, resolution repository.SLAPolicyRef) {
	response = e.policy(slaSvcPolicySpec{target: "RESPONSE", schedule: "24 x 7", duration: time.Hour,
		start: "state=OPEN^case_type=case^priority=S1^cs_engineer_response_comment=false", stop: "state=CLOSED"})
	resolution = e.policy(slaSvcPolicySpec{target: "RESOLUTION", schedule: slaCharBusinessDays, duration: 40 * time.Hour, retroactive: true,
		start: "case_type=case^priority=S1", stop: "state=CLOSED", pause: "state=AWAITING_INFO", resume: "state=WORK_IN_PROGRESS"})
	return response, resolution
}

// TestSLAEngineCharacterisation_CaseLifecycleEndToEnd drives the engine over
// the real repository for one case: register (a retroactive business-day
// clock starts at the case's creation instant, the flat one at now), register
// again (idempotent), pause on AWAITING_INFO, resume on WORK_IN_PROGRESS,
// complete both on CLOSED.
func TestSLAEngineCharacterisation_CaseLifecycleEndToEnd(t *testing.T) {
	e := newSLASvcCharEnv(t)
	wi := e.workItem()
	response, resolution := slaSvcCharPolicies(e)
	created := slaSvcNY(t, "2024-03-08T12:00:00") // a Friday
	repo := &slaSvcCharEngineRepo{SLAEngineRepository: e.repo, names: e.policyName,
		facts: repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case", CreatedOn: created}}
	svc := NewSLAEngineService(repo)
	critical := domain.CaseSeverityCritical

	// --- register
	before := time.Now()
	svc.RegisterCaseClocks(e.ctx, wi, &critical, "")
	rr, rz := e.one(wi, response), e.one(wi, resolution)
	svcStage(t, "response", rr, "IN_PROGRESS", true)
	svcTimeNear(t, "response start_on (flat, non-retroactive: now)", &rr.StartOn, before, slaCharTolerance)
	svcSecs(t, "response remaining", rr.Remaining, time.Hour, 0)
	svcStage(t, "resolution", rz, "IN_PROGRESS", true)
	if !rz.StartOn.Equal(created) {
		t.Errorf("resolution start_on = %v, want the case's creation instant %v (retroactive)", rz.StartOn.UTC(), created)
	}
	svcSecs(t, "resolution remaining", rz.Remaining, 40*time.Hour, 0)
	if rz.Schedule == nil || *rz.Schedule != slaCharBusinessDays {
		t.Errorf("resolution schedule column = %v, want %q", rz.Schedule, slaCharBusinessDays)
	}

	// --- register again: nothing new.
	svc.RegisterCaseClocks(e.ctx, wi, &critical, "")
	e.one(wi, response)
	e.one(wi, resolution)

	// --- AWAITING_INFO pauses the resolution clock (business-day freeze) and leaves the response clock running.
	svc.ApplyCaseStateEffects(e.ctx, wi, domain.CaseStateAwaitingInfo)
	rz = e.one(wi, resolution)
	svcStage(t, "resolution after AWAITING_INFO", rz, "PAUSED", true)
	svcTimeNear(t, "resolution paused_on", rz.PausedOn, time.Now(), slaCharTolerance)
	wantFrozen := businessdays.Elapsed(created, *rz.PausedOn)
	svcSecs(t, "resolution frozen business_duration", rz.BusinessDur, wantFrozen, time.Second)
	svcStage(t, "response after AWAITING_INFO (no pause condition)", e.one(wi, response), "IN_PROGRESS", true)

	// --- WORK_IN_PROGRESS resumes it; the paused moment is booked into both totals.
	svc.ApplyCaseStateEffects(e.ctx, wi, domain.CaseStateWorkInProgress)
	rz = e.one(wi, resolution)
	svcStage(t, "resolution after WORK_IN_PROGRESS", rz, "IN_PROGRESS", true)
	if rz.PausedOn != nil {
		t.Errorf("resolution paused_on = %v, want NULL after resume", rz.PausedOn)
	}
	svcSecs(t, "resolution pause_duration", rz.PauseDur, 0, slaCharTolerance)
	svcSecs(t, "resolution business_pause_duration", rz.BPauseDur, 0, slaCharTolerance)
	e.one(wi, response) // the re-registration pass added no second row for either policy
	e.one(wi, resolution)

	// --- CLOSED completes both (each policy's own stop condition is state=CLOSED).
	svc.ApplyCaseStateEffects(e.ctx, wi, domain.CaseStateClosed)
	rr, rz = e.one(wi, response), e.one(wi, resolution)
	svcStage(t, "response after CLOSED", rr, "ACHIEVED", false)
	svcStage(t, "resolution after CLOSED", rz, "ACHIEVED", false)
	if rr.HasBreached == nil || *rr.HasBreached {
		t.Errorf("response has_breached = %v, want false (completed within the hour)", rr.HasBreached)
	}
	if rz.HasBreached == nil || !*rz.HasBreached {
		t.Errorf("resolution has_breached = %v, want true (the creation instant is long past the 40h policy)", rz.HasBreached)
	}
	svcTimeNear(t, "resolution end_on", rz.EndOn, time.Now(), slaCharTolerance)
}

// TestSLAEngineCharacterisation_SeverityChangeEndToEnd pins ReviseCaseClocks
// over the real repository: a revision that still matches both policies
// cancels and replaces each clock, the replacement starting NOW even for a
// retroactive policy; a revision matching neither cancels both and registers
// nothing.
func TestSLAEngineCharacterisation_SeverityChangeEndToEnd(t *testing.T) {
	for _, c := range []struct {
		name         string
		severity     domain.CaseSeverity
		wantReplaced bool
	}{
		{"new severity still matches the policies (S1)", domain.CaseSeverityCritical, true},
		{"new severity matches nothing (S2)", domain.CaseSeverityHigh, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newSLASvcCharEnv(t)
			wi := e.workItem()
			response, resolution := slaSvcCharPolicies(e)
			created := slaSvcNY(t, "2024-03-08T12:00:00")
			repo := &slaSvcCharEngineRepo{SLAEngineRepository: e.repo, names: e.policyName,
				facts: repository.CaseFacts{State: "OPEN", Severity: "S1", CaseType: "case", CreatedOn: created}}
			svc := NewSLAEngineService(repo)
			critical := domain.CaseSeverityCritical
			svc.RegisterCaseClocks(e.ctx, wi, &critical, "")

			before := time.Now()
			sev := c.severity
			svc.ReviseCaseClocks(e.ctx, wi, &sev, "")

			for _, p := range []repository.SLAPolicyRef{response, resolution} {
				rs := e.rows(wi, p)
				old, ok := rs["CANCELLED"]
				if !ok {
					t.Fatalf("%s: rows %v, want the old clock CANCELLED", p.Target, rs)
				}
				svcStage(t, p.Target+" old", old, "CANCELLED", false)
				if !c.wantReplaced {
					if len(rs) != 1 {
						t.Errorf("%s: %d rows, want only the cancelled one (nothing matches the new severity)", p.Target, len(rs))
					}
					continue
				}
				fresh, ok := rs["IN_PROGRESS"]
				if !ok || len(rs) != 2 {
					t.Fatalf("%s: rows %v, want CANCELLED + fresh IN_PROGRESS", p.Target, rs)
				}
				svcStage(t, p.Target+" new", fresh, "IN_PROGRESS", true)
				// QUIRK: a revision always restarts at now, even for the retroactive resolution policy.
				svcTimeNear(t, p.Target+" new start_on", &fresh.StartOn, before, slaCharTolerance)
				svcSecs(t, p.Target+" new remaining", fresh.Remaining, p.Duration, 0)
			}
		})
	}
}

// TestSLAEngineCharacterisation_RecalculateCasePauseTotals runs
// RecalculateCasePauseTotals end to end against Postgres: real
// work_item_activity rows with fixed timestamps, real clocks, then the
// stored pause totals read back. All instants are fixed, so every value is
// exact (nothing here depends on now: the replay of a clock that is running at
// the end of the history adds nothing after its last resume, and a paused
// clock's open segment is not counted).
//
// History (America/New_York): clock starts Fri 2024-03-08 09:00 EST; Awaiting
// Info Fri 15:00 EST; back to Work In Progress Mon 2024-03-11 10:00 EDT (one
// pause: 66h of wall clock, 19h of business time); in the "paused" case
// Awaiting Info again Tue 2024-03-12 12:00 EDT (an open pause).
func TestSLAEngineCharacterisation_RecalculateCasePauseTotals(t *testing.T) {
	e := newSLASvcCharEnv(t)
	svc := NewSLAEngineService(e.repo)

	start := slaSvcNY(t, "2024-03-08T09:00:00")
	pauseAt := slaSvcNY(t, "2024-03-08T15:00:00")
	resumeAt := slaSvcNY(t, "2024-03-11T10:00:00")
	repauseAt := slaSvcNY(t, "2024-03-12T12:00:00")
	conds := func(target, schedule string, duration time.Duration) repository.SLAPolicyRef {
		// source CSM: conditions are read as written (no ServiceNow translation).
		return e.policy(slaSvcPolicySpec{target: target, schedule: schedule, source: "CSM", duration: duration,
			stop: "state=CLOSED", pause: "state=AWAITING_INFO", resume: "state=WORK_IN_PROGRESS"})
	}
	history := func(wi string, withSecondPause bool) {
		e.activity(wi, start.Add(-time.Minute), "state", "Open", "Work In Progress")
		e.activity(wi, pauseAt, "state", "Work In Progress", "Awaiting Info")
		e.activity(wi, resumeAt, "state", "Awaiting Info", "Work In Progress")
		if withSecondPause {
			e.activity(wi, repauseAt, "state", "Work In Progress", "Awaiting Info")
		}
	}
	find := func(rep CaseSLAPauseRecalcReport, target string) CaseClockPauseRecalcResult {
		t.Helper()
		for _, c := range rep.Clocks {
			if c.Target == target {
				return c
			}
		}
		t.Fatalf("no result for target %s in %+v", target, rep.Clocks)
		return CaseClockPauseRecalcResult{}
	}
	const wantFlat, wantBusiness = 66 * time.Hour, 19 * time.Hour

	t.Run("running clocks get their pause totals rebuilt", func(t *testing.T) {
		wi := e.workItem()
		flat := conds("RESOLUTION", "24 x 7", 12*time.Hour)
		bd := conds("WORKAROUND", slaCharBusinessDays, 100*time.Hour)
		history(wi, false)
		e.insertClock(wi, flat, "IN_PROGRESS", start, nil, 0)
		e.insertClock(wi, bd, "IN_PROGRESS", start, nil, 0)

		rep, err := svc.RecalculateCasePauseTotals(e.ctx, wi)
		if err != nil {
			t.Fatalf("RecalculateCasePauseTotals: %v", err)
		}
		if !rep.HasHistory || len(rep.Clocks) != 2 {
			t.Fatalf("report = %+v, want history and two clocks", rep)
		}
		f, b := find(rep, "RESOLUTION"), find(rep, "WORKAROUND")
		if f.Skipped || !f.Updated || f.OldPauseSeconds != 0 || f.NewPauseSeconds != int64(wantFlat.Seconds()) {
			t.Errorf("flat result = %+v, want updated 0 -> %d", f, int64(wantFlat.Seconds()))
		}
		if b.Skipped || !b.Updated || b.NewPauseSeconds != int64(wantFlat.Seconds()) || b.NewBusinessPauseSeconds != int64(wantBusiness.Seconds()) {
			t.Errorf("business-day result = %+v, want updated to %d / %d business", b, int64(wantFlat.Seconds()), int64(wantBusiness.Seconds()))
		}
		fr, br := e.one(wi, flat), e.one(wi, bd)
		svcSecs(t, "flat pause_duration", fr.PauseDur, wantFlat, 0)
		svcNull(t, "flat business_pause_duration (never written for a flat clock)", fr.BPauseDur)
		svcSecs(t, "business-day pause_duration", br.PauseDur, wantFlat, 0)
		svcSecs(t, "business-day business_pause_duration", br.BPauseDur, wantBusiness, 0)
		svcStage(t, "flat stage unchanged", fr, "IN_PROGRESS", true)
		if fr.UpdatedBy != "sla-engine" {
			t.Errorf("updated_by = %q, want sla-engine", fr.UpdatedBy)
		}

		// A second run finds nothing to correct.
		rep2, err := svc.RecalculateCasePauseTotals(e.ctx, wi)
		if err != nil {
			t.Fatalf("second run: %v", err)
		}
		for _, c := range rep2.Clocks {
			if c.Skipped || c.Updated {
				t.Errorf("second run result = %+v, want no skip and no update", c)
			}
		}
	})

	t.Run("a paused clock also gets its frozen snapshot rebuilt", func(t *testing.T) {
		wi := e.workItem()
		flat := conds("RESOLUTION", "24 x 7", 40*time.Hour)
		bd := conds("WORKAROUND", slaCharBusinessDays, 100*time.Hour)
		history(wi, true)
		e.insertClock(wi, flat, "PAUSED", start, &repauseAt, 0)
		e.insertClock(wi, bd, "PAUSED", start, &repauseAt, 0)

		rep, err := svc.RecalculateCasePauseTotals(e.ctx, wi)
		if err != nil {
			t.Fatalf("RecalculateCasePauseTotals: %v", err)
		}
		if f := find(rep, "RESOLUTION"); f.Skipped || !f.Updated {
			t.Errorf("flat result = %+v, want updated", f)
		}
		if b := find(rep, "WORKAROUND"); b.Skipped || !b.Updated {
			t.Errorf("business-day result = %+v, want updated", b)
		}
		// Flat: elapsed = (Fri 09:00 EST .. Tue 12:00 EDT = 98h) - 66h pause = 32h of a 40h clock.
		fr := e.one(wi, flat)
		svcStage(t, "flat", fr, "PAUSED", true)
		svcSecs(t, "flat pause_duration", fr.PauseDur, wantFlat, 0)
		svcSecs(t, "flat business_duration", fr.BusinessDur, 32*time.Hour, 0)
		svcSecs(t, "flat remaining", fr.Remaining, 8*time.Hour, 0)
		if fr.Pct == nil || *fr.Pct != 80 {
			t.Errorf("flat percentage = %v, want 80", fr.Pct)
		}
		// Business days: Elapsed(Fri 09:00, Tue 12:00) = 15h + 24h + 12h = 51h, minus 19h business pause = 32h of 100h.
		br := e.one(wi, bd)
		svcStage(t, "business-day", br, "PAUSED", true)
		svcSecs(t, "business-day pause_duration", br.PauseDur, wantFlat, 0)
		svcSecs(t, "business-day business_pause_duration", br.BPauseDur, wantBusiness, 0)
		svcSecs(t, "business-day business_duration", br.BusinessDur, 32*time.Hour, 0)
		svcSecs(t, "business-day remaining", br.Remaining, 68*time.Hour, 0)
		if br.Pct == nil || *br.Pct != 32 {
			t.Errorf("business-day percentage = %v, want 32", br.Pct)
		}
		if br.PausedOn == nil || !br.PausedOn.Equal(repauseAt) {
			t.Errorf("business-day paused_on = %v, want unchanged %v", br.PausedOn, repauseAt)
		}
	})

	t.Run("a case with no state history reports no history and writes nothing", func(t *testing.T) {
		wi := e.workItem()
		flat := conds("RESOLUTION", "24 x 7", 12*time.Hour)
		e.insertClock(wi, flat, "IN_PROGRESS", start, nil, 7*time.Hour)
		rep, err := svc.RecalculateCasePauseTotals(e.ctx, wi)
		if err != nil {
			t.Fatalf("RecalculateCasePauseTotals: %v", err)
		}
		if rep.HasHistory || len(rep.Clocks) != 0 {
			t.Errorf("report = %+v, want no history", rep)
		}
		svcSecs(t, "pause_duration untouched", e.one(wi, flat).PauseDur, 7*time.Hour, 0)
	})

	t.Run("a severity change after the clock started skips it with a reason", func(t *testing.T) {
		wi := e.workItem()
		flat := conds("RESOLUTION", "24 x 7", 12*time.Hour)
		history(wi, false)
		e.activity(wi, resumeAt.Add(time.Hour), "severity", "S2", "S1")
		e.insertClock(wi, flat, "IN_PROGRESS", start, nil, 7*time.Hour)
		rep, err := svc.RecalculateCasePauseTotals(e.ctx, wi)
		if err != nil {
			t.Fatalf("RecalculateCasePauseTotals: %v", err)
		}
		c := find(rep, "RESOLUTION")
		const wantReason = "a severity change happened after this clock started -- this clock's own row may be stale"
		if !c.Skipped || c.SkipReason != wantReason || c.Updated {
			t.Errorf("result = %+v, want skipped with %q", c, wantReason)
		}
		svcSecs(t, "pause_duration untouched", e.one(wi, flat).PauseDur, 7*time.Hour, 0)
	})
}
