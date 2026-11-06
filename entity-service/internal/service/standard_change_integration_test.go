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

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The standard change catalogue end to end against PostgreSQL: the real service
// over the real repository, run once per DSN given -- STANDARD_CHANGE_TEST_DSN
// (typically a superuser) and STANDARD_CHANGE_TEST_APP_DSN (a role without
// BYPASSRLS, so the internal-only policies of migration 100026 are in force).
//
//	STANDARD_CHANGE_TEST_DSN=postgres://... STANDARD_CHANGE_TEST_APP_DSN=postgres://... \
//	  go test ./internal/service/ -run StandardChangeIntegration -p 1

const (
	stdProposer = "std-proposer@scp.test"
	stdCAB1     = "std-cab-one@scp.test"
	stdCAB2     = "std-cab-two@scp.test"
	stdOutsider = "std-outsider@scp.test"
	// The Standard Change Authority (migration 0277): stdAuth1 and stdAuth2, and
	// stdCAB1, who sits on the CAB too. stdCAB2 is on the CAB only, which gives
	// no say on a proposal (migration 0287 removed the CAB's stage).
	stdAuth1   = "std-auth-one@scp.test"
	stdAuth2   = "std-auth-two@scp.test"
	stdTag     = "ZZ std-test"
	cabGroupID = "00000000-0000-4000-8000-00000000ca01" // migration 0188
)

type stdEnv struct {
	svc      StandardChangeService
	pool     *pgxpool.Pool
	scoped   *repository.Scoped
	crs      map[string]string // label -> change request id
	accounts map[string]string // "A", "B" -> account id (two customers)
	projects map[string]string // "A", "A2", "B" -> project id (A and A2 are customer A's)
}

func stdDSNs(t *testing.T) map[string]string {
	out := map[string]string{}
	if v := os.Getenv("STANDARD_CHANGE_TEST_DSN"); v != "" {
		out["owner"] = v
	}
	if v := os.Getenv("STANDARD_CHANGE_TEST_APP_DSN"); v != "" {
		out["app"] = v
	}
	if len(out) == 0 {
		t.Skip("STANDARD_CHANGE_TEST_DSN / STANDARD_CHANGE_TEST_APP_DSN not set")
	}
	return out
}

func stdCtx(email string) context.Context {
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
	return repository.WithCallerIdentity(ctx, repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: email})
}

func stdCleanup(t *testing.T, s *repository.Scoped) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	stmts := []string{
		// Change requests raised from the tests' templates first (the version is a
		// RESTRICT foreign key).
		`DELETE FROM work_item WHERE id IN (SELECT cr.id FROM change_request cr
			JOIN standard_change_template_version v ON v.id = cr.standard_change_template_version_id
			JOIN standard_change_template t ON t.id = v.template_id WHERE t.name ILIKE '` + stdTag + `%')`,
		`DELETE FROM change_request WHERE id IN (SELECT id FROM work_item WHERE subject ILIKE '` + stdTag + `%' AND type = 'CHANGE_REQUEST')`,
		`DELETE FROM work_item WHERE subject ILIKE '` + stdTag + `%'`,
		`DELETE FROM work_item WHERE type = 'STANDARD_CHANGE_PROPOSAL' AND created_by LIKE 'std-%@scp.test'`,
		`DELETE FROM standard_change_template WHERE name ILIKE '` + stdTag + `%'`,
		`DELETE FROM group_member WHERE user_id IN (SELECT id FROM "user" WHERE email LIKE 'std-%@scp.test')`,
		`DELETE FROM user_role WHERE user_id IN (SELECT id FROM "user" WHERE email LIKE 'std-%@scp.test')`,
		`DELETE FROM "user" WHERE email LIKE 'std-%@scp.test'`,
		`DELETE FROM work_item WHERE project_id IN (SELECT id FROM project WHERE key LIKE 'ZZSTDTEST-%')`,
		`DELETE FROM project WHERE key LIKE 'ZZSTDTEST-%'`,
		`DELETE FROM account WHERE number LIKE 'ZZSTDTEST-%'`,
	}
	for _, q := range stmts {
		if _, err := s.Exec(sys, q); err != nil {
			t.Fatalf("cleanup %q: %v", q, err)
		}
	}
}

func newStdEnv(t *testing.T, dsn string) *stdEnv {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	s := repository.NewScoped(pool)
	stdCleanup(t, s)
	t.Cleanup(func() { stdCleanup(t, s) })

	sys := repository.WithSystemIdentity(context.Background())
	for _, email := range []string{stdProposer, stdCAB1, stdCAB2, stdOutsider, stdAuth1, stdAuth2} {
		var id string
		if err := s.QueryRow(sys, `INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, name, is_active)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $1, $2, true) RETURNING id::text`, email, strings.Split(email, "@")[0]).Scan(&id); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if _, err := s.Exec(sys, `INSERT INTO user_role (created_on, updated_on, user_id, role_id) VALUES (NOW(), NOW(), $1::uuid, '00000000-0000-0000-0000-000000000101')`, id); err != nil {
			t.Fatalf("seed role: %v", err)
		}
		if email == stdProposer || email == stdCAB1 || email == stdCAB2 {
			if _, err := s.Exec(sys, `INSERT INTO group_member (id, created_on, updated_on, created_by, updated_by, group_id, user_id)
				VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1::uuid, $2::uuid)`, cabGroupID, id); err != nil {
				t.Fatalf("seed CAB member: %v", err)
			}
		}
		if email == stdAuth1 || email == stdAuth2 || email == stdCAB1 {
			if _, err := s.Exec(sys, `INSERT INTO group_member (id, created_on, updated_on, created_by, updated_by, group_id, user_id)
				SELECT gen_random_uuid(), NOW(), NOW(), 't', 't', g.id, $1::uuid FROM "group" g WHERE g.name = $2
				ORDER BY g.created_on LIMIT 1`, id, domain.StandardChangeAuthorityGroupName); err != nil {
				t.Fatalf("seed authority member: %v", err)
			}
		}
	}
	env := &stdEnv{svc: NewStandardChangeService(repository.NewStandardChangeRepository(s), ctxIdentityAccess{}), pool: pool, scoped: s,
		crs: map[string]string{}, accounts: map[string]string{}, projects: map[string]string{}}
	// Two customers (accounts), each with a project; customer A has a second one.
	for _, label := range []string{"A", "B"} {
		var id string
		if err := s.QueryRow(sys, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $2) RETURNING id::text`,
			stdTag+" Customer "+label, "ZZSTDTEST-"+label).Scan(&id); err != nil {
			t.Fatalf("seed account: %v", err)
		}
		env.accounts[label] = id
	}
	for label, account := range map[string]string{"A": "A", "A2": "A", "B": "B"} {
		var id string
		if err := s.QueryRow(sys, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $2, $3::uuid) RETURNING id::text`,
			"ZZSTDTEST-"+label, stdTag+" Project "+label, env.accounts[account]).Scan(&id); err != nil {
			t.Fatalf("seed project: %v", err)
		}
		env.projects[label] = id
	}
	// Change requests: customer A's (the evidence of the tests' proposals), one of
	// customer B's and one raised for no project.
	type seed struct{ state, project string }
	for label, cr := range map[string]seed{
		"closed1": {"CLOSED", "A"}, "closed2": {"CLOSED", "A2"}, "open": {"IMPLEMENT", "A"}, "rolledback": {"ROLLBACK", "A"},
		"closedOther": {"CLOSED", "B"}, "closedNone": {"CLOSED", ""}, "closedMoves": {"CLOSED", "A"},
		"closedDriftSource": {"CLOSED", "A"}, "closedDriftEvidence": {"CLOSED", "A"},
	} {
		var project *string
		if cr.project != "" {
			p := env.projects[cr.project]
			project = &p
		}
		var id string
		if err := s.QueryRow(sys, `WITH wi AS (
				INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, description, type, project_id)
				VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, 'CHG-STD-' || substr(md5(random()::text), 1, 10), $2, '<p>Rotate keys on cluster aks-1</p>', 'CHANGE_REQUEST', $4::uuid)
				RETURNING id)
			INSERT INTO change_request (id, state, change_model, implementation_plan, backout_plan, test_plan, risk, impact, category)
			SELECT id, $3::change_request_state_enum, 'NORMAL', '<p>Run the rotation on aks-1</p>', '<p>Restore the keys</p>', '<p>Check aks-1</p>',
			       'LOW', 'LOW', 'SOFTWARE' FROM wi RETURNING id::text`, stdProposer, stdTag+" "+label, cr.state, project).Scan(&id); err != nil {
			t.Fatalf("seed change request: %v", err)
		}
		env.crs[label] = id
	}
	return env
}

func expectErr[T any](t *testing.T, err error, what string) {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("%s: err = %v (%T), want %T", what, err, err, target)
	}
}

func approverStates(p domain.StandardChangeProposal) map[string]string {
	out := map[string]string{}
	for _, a := range p.Approvers {
		if a.Approver != nil {
			out[a.Approver.Email] = a.State
		}
	}
	return out
}

// authorise has the two authority members approve a submitted proposal, and
// returns it (approved).
func authorise(t *testing.T, env *stdEnv, id string) domain.StandardChangeProposal {
	t.Helper()
	p, err := env.svc.DecideProposal(stdCtx(stdAuth1), id, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	if err != nil || p.State != domain.StandardChangeProposalStateAuthorityReview {
		t.Fatalf("first authorisation: %s, %v", p.State, err)
	}
	p, err = env.svc.DecideProposal(stdCtx(stdAuth2), id, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	if err != nil || p.State != domain.StandardChangeProposalStateApproved {
		t.Fatalf("second authorisation: %s, %v", p.State, err)
	}
	return p
}

func makeProposalComplete(t *testing.T, env *stdEnv, id, name string) domain.StandardChangeProposal {
	t.Helper()
	low := domain.ChangeRequestRiskLow
	params := []domain.StandardChangeParameter{
		{Key: "cluster", Label: "Cluster", Type: "text", Required: true},
		{Key: "env", Label: "Environment", Type: "select", Options: []string{"dev", "prod"}, Required: true},
	}
	p, err := env.svc.UpdateProposal(stdCtx(stdProposer), id, domain.UpdateStandardChangeProposalRequest{
		Name:               &name,
		Rationale:          stdp("<p>Done many times, no incident.</p>"),
		Summary:            stdp("Rotates a cluster's keys. Not for certificates."),
		Description:        stdp("<p>Rotate the keys of {{cluster}}</p>"),
		Subject:            stdp("Rotate keys on {{cluster}} ({{env}})"),
		ImplementationPlan: stdp("<p>Run the rotation on {{cluster}}</p>"),
		Risk:               &low,
		Parameters:         &params,
	})
	if err != nil {
		t.Fatalf("complete the proposal: %v", err)
	}
	return p
}

func TestStandardChangeIntegration(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			env := newStdEnv(t, dsn)
			t.Run("lifecycle", func(t *testing.T) { stdLifecycle(t, env) })
			t.Run("rejection is kept as the record", func(t *testing.T) { stdRejection(t, env) })
			t.Run("evidence and drafts", func(t *testing.T) { stdEvidence(t, env) })
			t.Run("a template is one customer's", func(t *testing.T) { stdCustomer(t, env) })
			t.Run("change requests that move to another customer", func(t *testing.T) { stdMovedChangeRequests(t, env) })
			t.Run("the customer stays fixed, and the authority approves for it", func(t *testing.T) { stdCustomerHolds(t, env) })
			t.Run("the authority's stage", func(t *testing.T) { stdAuthority(t, env) })
			t.Run("deleting and cancelling", func(t *testing.T) { stdDeleteCancel(t, env) })
			t.Run("who puts a template back in use, and its history", func(t *testing.T) { stdTemplateGates(t, env) })
			t.Run("one change, one template", func(t *testing.T) { stdDuplicates(t, env) })
			t.Run("a proposal the CAB approved before its stage was removed", func(t *testing.T) { stdOldTwoStageProposal(t, env) })
		})
	}
}

func stdLifecycle(t *testing.T, env *stdEnv) {
	name := stdTag + " Rotate cluster keys"
	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closed1"]),
	})
	if err != nil {
		t.Fatalf("propose from a closed change request: %v", err)
	}
	if !strings.HasPrefix(p.Number, "SCP") || p.State != domain.StandardChangeProposalStateDraft || !p.CanEdit {
		t.Fatalf("new draft = %+v", p)
	}
	if p.ImplementationPlan == nil || *p.ImplementationPlan != "<p>Run the rotation on aks-1</p>" || p.Risk == nil || *p.Risk != domain.ChangeRequestRiskLow {
		t.Fatalf("content not copied from the change request: %+v", p.StandardChangeContent)
	}
	if len(p.Evidence) != 1 || p.Evidence[0].ID != env.crs["closed1"] {
		t.Fatalf("the source change request is the first evidence: %+v", p.Evidence)
	}

	// Only the proposer edits; a draft is not decided.
	_, err = env.svc.UpdateProposal(stdCtx(stdOutsider), p.ID, domain.UpdateStandardChangeProposalRequest{Rationale: stdp("x")})
	expectErr[*apierror.ForbiddenError](t, err, "outsider edits")
	_, err = env.svc.DecideProposal(stdCtx(stdCAB1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "decide a draft")

	// Not complete: the placeholders must be declared and the rationale given.
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{Subject: stdp("Rotate {{cluster}}")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.SubmitProposal(stdCtx(stdProposer), p.ID)
	expectErr[*apierror.ValidationError](t, err, "submit incomplete")
	if !strings.Contains(err.Error(), "rationale") {
		t.Fatalf("incomplete: %v", err)
	}

	p = makeProposalComplete(t, env, p.ID, name)
	if _, err := env.svc.SubmitProposal(stdCtx(stdOutsider), p.ID); err == nil {
		t.Fatal("an outsider submitted the proposal")
	}
	p, err = env.svc.SubmitProposal(stdCtx(stdProposer), p.ID)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Submitted straight to the Standard Change Authority: one stage, its members
	// asked, two of them to approve. The CAB has no stage.
	if p.State != domain.StandardChangeProposalStateAuthorityReview || p.SubmittedOn == nil || p.CanEdit || p.Template != nil {
		t.Fatalf("submitted = state %s, canEdit %v", p.State, p.CanEdit)
	}
	if len(p.Stages) != 1 || p.Stages[0].Label != domain.StandardChangeAuthorityGroupName || p.Stages[0].RequiredApprovals != 2 ||
		p.Stages[0].Approvals != 0 || p.Stages[0].Decision != nil {
		t.Fatalf("stages = %+v", p.Stages)
	}
	states := approverStates(p)
	if states[stdAuth1] != "REQUESTED" || states[stdAuth2] != "REQUESTED" || states[stdCAB1] != "REQUESTED" || len(states) != 3 {
		t.Fatalf("authority stage = %v (its three members, nobody else)", states)
	}

	// Who may decide: not the proposer, not a non-member, and not a CAB member who
	// is not on the authority.
	_, err = env.svc.DecideProposal(stdCtx(stdProposer), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "the proposer decides")
	_, err = env.svc.DecideProposal(stdCtx(stdOutsider), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "a non-member decides")
	_, err = env.svc.DecideProposal(stdCtx(stdCAB2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "a CAB member who is not on the authority decides")
	if got, _ := env.svc.GetProposal(stdCtx(stdCAB2), p.ID); got.Stages[0].Approvals != 0 {
		t.Fatalf("a refused decision was recorded: %+v", got.Stages)
	}
	viewed, _ := env.svc.GetProposal(stdCtx(stdAuth1), p.ID)
	for _, a := range viewed.Approvers {
		if a.CanDecide != (a.Approver.Email == stdAuth1) {
			t.Fatalf("canDecide for %s = %v", a.Approver.Email, a.CanDecide)
		}
	}
	awaiting, err := env.svc.SearchProposals(stdCtx(stdAuth1), domain.SearchStandardChangeProposalsRequest{AwaitingMyDecision: true})
	if err != nil || awaiting.Total != 1 || awaiting.Proposals[0].ID != p.ID {
		t.Fatalf("awaiting the authority = %+v, %v", awaiting, err)
	}
	if awaiting, err = env.svc.SearchProposals(stdCtx(stdCAB2), domain.SearchStandardChangeProposalsRequest{AwaitingMyDecision: true}); err != nil || awaiting.Total != 0 {
		t.Fatalf("awaiting a CAB member = %+v, %v", awaiting, err)
	}
	// One authorisation is not enough, and nobody authorises twice.
	p, err = env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", Comment: stdp("ok")})
	if err != nil || p.State != domain.StandardChangeProposalStateAuthorityReview || p.Stages[0].Approvals != 1 || p.Template != nil {
		t.Fatalf("after one authorisation = %s %+v, %v", p.State, p.Stages, err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "authorise twice")
	// The second approval, by another member, applies the proposal.
	p, err = env.svc.DecideProposal(stdCtx(stdAuth2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", Comment: stdp("agreed")})
	if err != nil {
		t.Fatalf("second authorisation: %v", err)
	}
	if p.State != domain.StandardChangeProposalStateApproved || p.Template == nil || p.ResultingVersion == nil || *p.ResultingVersion != 1 ||
		p.DecidedBy == nil || p.DecidedBy.Email != stdAuth2 || p.Stages[0].Approvals != 2 || p.Stages[0].Decision == nil || *p.Stages[0].Decision != "APPROVED" {
		t.Fatalf("approved = %+v", p)
	}
	if s := approverStates(p); s[stdAuth1] != "APPROVED" || s[stdAuth2] != "APPROVED" || s[stdCAB1] != "CANCELLED" {
		t.Fatalf("after the second authorisation = %v", s)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdCAB1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("late")})
	expectErr[*apierror.ConflictError](t, err, "decide again")

	tpl, err := env.svc.GetTemplate(stdCtx(stdOutsider), p.Template.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if tpl.State != domain.StandardChangeTemplateStateActive || !strings.HasPrefix(tpl.Number, "STD") || tpl.Name != name ||
		tpl.Owner == nil || tpl.Owner.Email != stdProposer || tpl.CurrentVersion == nil || tpl.CurrentVersion.Version != 1 ||
		len(tpl.CurrentVersion.Parameters) != 2 || len(tpl.Versions) != 1 {
		t.Fatalf("template = %+v", tpl)
	}
	list, err := env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{Query: stdp("rotate CLUSTER")})
	if err != nil || list.Total != 1 || list.Templates[0].ID != tpl.ID {
		t.Fatalf("search = %+v, %v", list, err)
	}

	if tpl.SuspendedReason != nil || tpl.SuspendedOn != nil || tpl.SuspendedBy != nil {
		t.Fatalf("an active template carries a suspension: %v %v %v", tpl.SuspendedReason, tpl.SuspendedOn, tpl.SuspendedBy)
	}

	// Suspend / resume: the owner or a CAB member. The reason is optional, at
	// most 1000 characters, and recorded with who suspended it and when.
	suspendWhy := domain.SuspendStandardChangeTemplateRequest{Reason: "  Rolled back twice on prod  "}
	_, err = env.svc.SuspendTemplate(stdCtx(stdOutsider), tpl.ID, suspendWhy)
	expectErr[*apierror.ForbiddenError](t, err, "outsider suspends")
	_, err = env.svc.SuspendTemplate(stdCtx(stdCAB2), tpl.ID, domain.SuspendStandardChangeTemplateRequest{Reason: strings.Repeat("x", 1001)})
	expectErr[*apierror.ValidationError](t, err, "over-long suspension reason")
	if got, _ := env.svc.GetTemplate(stdCtx(stdOutsider), tpl.ID); got.State != domain.StandardChangeTemplateStateActive {
		t.Fatalf("a refused suspension moved the template: %v", got.State)
	}
	if tpl, err = env.svc.SuspendTemplate(stdCtx(stdCAB2), tpl.ID, suspendWhy); err != nil || tpl.State != domain.StandardChangeTemplateStateSuspended {
		t.Fatalf("suspend: %v %v", tpl.State, err)
	}
	if tpl.SuspendedReason == nil || *tpl.SuspendedReason != "Rolled back twice on prod" || tpl.SuspendedOn == nil ||
		tpl.SuspendedBy == nil || *tpl.SuspendedBy != stdCAB2 {
		t.Fatalf("suspension not recorded: %v %v %v", tpl.SuspendedReason, tpl.SuspendedOn, tpl.SuspendedBy)
	}
	read, err := env.svc.GetTemplate(stdCtx(stdOutsider), tpl.ID)
	if err != nil || read.SuspendedReason == nil || *read.SuspendedReason != "Rolled back twice on prod" ||
		read.SuspendedOn == nil || read.SuspendedBy == nil || *read.SuspendedBy != stdCAB2 {
		t.Fatalf("suspension not read back: %+v, %v", read, err)
	}
	listed, err := env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{Query: stdp(tpl.Number)})
	if err != nil || listed.Total != 1 || listed.Templates[0].SuspendedReason == nil {
		t.Fatalf("suspension not in the list: %+v, %v", listed, err)
	}
	_, err = env.svc.SuspendTemplate(stdCtx(stdCAB2), tpl.ID, domain.SuspendStandardChangeTemplateRequest{})
	expectErr[*apierror.ConflictError](t, err, "suspend twice")
	resumeWhy := domain.StandardChangeTemplateNoteRequest{Note: "Root cause fixed"}
	_, err = env.svc.ResumeTemplate(stdCtx(stdProposer), tpl.ID, resumeWhy)
	expectErr[*apierror.ForbiddenError](t, err, "the owner resumes")
	if tpl, err = env.svc.ResumeTemplate(stdCtx(stdAuth1), tpl.ID, resumeWhy); err != nil || tpl.State != domain.StandardChangeTemplateStateActive {
		t.Fatalf("resume by the authority: %v %v", tpl.State, err)
	}
	if tpl.SuspendedReason != nil || tpl.SuspendedOn != nil || tpl.SuspendedBy != nil {
		t.Fatalf("resume left the suspension: %v %v %v", tpl.SuspendedReason, tpl.SuspendedOn, tpl.SuspendedBy)
	}
	var rawReason, rawBy *string
	if err := env.scoped.QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT suspended_reason, suspended_by FROM standard_change_template WHERE id = $1::uuid`, tpl.ID).Scan(&rawReason, &rawBy); err != nil ||
		rawReason != nil || rawBy != nil {
		t.Fatalf("resume did not clear the columns: %v %v %v", rawReason, rawBy, err)
	}
	// A blank reason is no reason, and it is cleared again on resume.
	if tpl, err = env.svc.SuspendTemplate(stdCtx(stdProposer), tpl.ID, domain.SuspendStandardChangeTemplateRequest{Reason: "   "}); err != nil ||
		tpl.SuspendedReason != nil || tpl.SuspendedBy == nil || *tpl.SuspendedBy != stdProposer || tpl.SuspendedOn == nil {
		t.Fatalf("suspend without a reason: %+v, %v", tpl, err)
	}
	if tpl, err = env.svc.ResumeTemplate(stdCtx(stdCAB1), tpl.ID, resumeWhy); err != nil || tpl.State != domain.StandardChangeTemplateStateActive || tpl.SuspendedBy != nil {
		t.Fatalf("resume by a CAB member who sits on the authority: %+v %v", tpl, err)
	}

	// Modify: starts from v1, one open at a time, approval writes v2 and clears the review flag.
	sys := repository.WithSystemIdentity(context.Background())
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_template SET review_required = true, review_required_reason = 'rolled back' WHERE id = $1::uuid`, tpl.ID); err != nil {
		t.Fatal(err)
	}
	m, err := env.svc.CreateProposal(stdCtx(stdCAB2), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatalf("propose a modification: %v", err)
	}
	if m.BaseVersion == nil || *m.BaseVersion != 1 || m.Subject == nil || *m.Subject != "Rotate keys on {{cluster}} ({{env}})" || m.Name != name {
		t.Fatalf("modification draft = %+v", m)
	}
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindRetire, TemplateID: &tpl.ID})
	expectErr[*apierror.ConflictError](t, err, "a second open proposal on the template")
	if _, err := env.svc.UpdateProposal(stdCtx(stdCAB2), m.ID, domain.UpdateStandardChangeProposalRequest{
		Rationale: stdp("<p>Add a verification step</p>"), TestPlan: stdp("<p>Check {{cluster}} twice</p>")}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdCAB2), m.ID); err != nil {
		t.Fatalf("submit the modification: %v", err)
	}
	authorise(t, env, m.ID)
	tpl, _ = env.svc.GetTemplate(stdCtx(stdOutsider), tpl.ID)
	if tpl.CurrentVersion.Version != 2 || *tpl.CurrentVersion.TestPlan != "<p>Check {{cluster}} twice</p>" || tpl.ReviewRequired || len(tpl.Versions) != 2 {
		t.Fatalf("after the modification = v%d review=%v versions=%d", tpl.CurrentVersion.Version, tpl.ReviewRequired, len(tpl.Versions))
	}
	v1, err := env.svc.GetTemplateVersion(stdCtx(stdOutsider), tpl.ID, tpl.Versions[1].ID)
	if err != nil || v1.Version != 1 || *v1.TestPlan != "<p>Check aks-1</p>" {
		t.Fatalf("version 1 is kept unchanged: %+v %v", v1, err)
	}

	// A new template with a live template's name is refused at submit, while the
	// draft can still be renamed ...
	dup, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closed2"])})
	if err != nil {
		t.Fatal(err)
	}
	makeProposalComplete(t, env, dup.ID, strings.ToUpper(name))
	_, err = env.svc.SubmitProposal(stdCtx(stdProposer), dup.ID)
	expectErr[*apierror.ConflictError](t, err, "submit a duplicate name")
	other := name + " (other)"
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), dup.ID, domain.UpdateStandardChangeProposalRequest{Name: &other}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), dup.ID); err != nil {
		t.Fatalf("submit under a free name: %v", err)
	}
	// ... and again at approval, should a template take the name meanwhile;
	// the refused approval changes nothing.
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_template SET name = $2 WHERE id = $1::uuid`, tpl.ID, other); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), dup.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatalf("first authorisation: %v", err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), dup.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "approve a duplicate name")
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_template SET name = $2 WHERE id = $1::uuid`, tpl.ID, name); err != nil {
		t.Fatal(err)
	}
	if again, _ := env.svc.GetProposal(stdCtx(stdCAB1), dup.ID); again.State != domain.StandardChangeProposalStateAuthorityReview || approverStates(again)[stdAuth2] != "REQUESTED" {
		t.Fatalf("a refused approval wrote something: %s %v", again.State, approverStates(again))
	}
	// Withdrawn while the authority decides: the open rows are cancelled, and who
	// withdrew it is recorded.
	if _, err := env.svc.WithdrawProposal(stdCtx(stdProposer), dup.ID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if w, _ := env.svc.GetProposal(stdCtx(stdProposer), dup.ID); w.State != domain.StandardChangeProposalStateWithdrawn || approverStates(w)[stdAuth2] != "CANCELLED" ||
		w.DecidedBy == nil || w.DecidedBy.Email != stdProposer {
		t.Fatalf("withdrawn = %s %v %+v", w.State, approverStates(w), w.DecidedBy)
	}

	// Retire.
	r, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindRetire, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), r.ID, domain.UpdateStandardChangeProposalRequest{TestPlan: stdp("x")})
	expectErr[*apierror.ValidationError](t, err, "content on a retirement")
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), r.ID, domain.UpdateStandardChangeProposalRequest{Rationale: stdp("No longer needed")}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), r.ID); err != nil {
		t.Fatal(err)
	}
	// A suspended template can be retired; retiring clears the suspension.
	if _, err := env.svc.SuspendTemplate(stdCtx(stdCAB1), tpl.ID, domain.SuspendStandardChangeTemplateRequest{Reason: "Going away"}); err != nil {
		t.Fatalf("suspend before retiring: %v", err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), r.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	if tpl, _ = env.svc.GetTemplate(stdCtx(stdOutsider), tpl.ID); tpl.State == domain.StandardChangeTemplateStateRetired {
		t.Fatal("one authorisation alone retired the template")
	}
	if p, err := env.svc.DecideProposal(stdCtx(stdCAB1), r.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil ||
		p.State != domain.StandardChangeProposalStateApproved {
		t.Fatalf("the second authorisation, by a member who also sits on the CAB: %s, %v", p.State, err)
	}
	tpl, _ = env.svc.GetTemplate(stdCtx(stdOutsider), tpl.ID)
	if tpl.State != domain.StandardChangeTemplateStateRetired {
		t.Fatalf("retired = %s", tpl.State)
	}
	var retiredBy *string
	if err := env.scoped.QueryRow(sys, `SELECT suspended_by FROM standard_change_template WHERE id = $1::uuid`, tpl.ID).Scan(&retiredBy); err != nil ||
		retiredBy != nil || tpl.SuspendedReason != nil {
		t.Fatalf("retiring kept the suspension: %v %v %v", retiredBy, tpl.SuspendedReason, err)
	}
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	expectErr[*apierror.ConflictError](t, err, "modify a retired template")
	_, err = env.svc.ResumeTemplate(stdCtx(stdCAB1), tpl.ID, resumeWhy)
	expectErr[*apierror.ConflictError](t, err, "resume a retired template")

	mine, err := env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{ProposedByMe: true})
	if err != nil || mine.Total != 3 {
		t.Fatalf("proposed by me = %d, %v", mine.Total, err)
	}
	// cab_review is no longer a state (0287), but a webapp from before it may
	// still ask for it: accepted, and it matches nothing, alone or beside others.
	cabOnly, err := env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{ProposedByMe: true,
		States: []domain.StandardChangeProposalState{"cab_review"}})
	if err != nil || cabOnly.Total != 0 || len(cabOnly.Proposals) != 0 {
		t.Fatalf("proposed by me in cab_review = %d, %v", cabOnly.Total, err)
	}
	approved, err := env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{ProposedByMe: true,
		States: []domain.StandardChangeProposalState{domain.StandardChangeProposalStateApproved}})
	if err != nil || approved.Total == 0 {
		t.Fatalf("proposed by me and approved = %d, %v", approved.Total, err)
	}
	withCAB, err := env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{ProposedByMe: true,
		States: []domain.StandardChangeProposalState{"cab_review", domain.StandardChangeProposalStateApproved}})
	if err != nil || withCAB.Total != approved.Total {
		t.Fatalf("proposed by me in cab_review or approved = %d, want %d, %v", withCAB.Total, approved.Total, err)
	}
	_, err = env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{
		States: []domain.StandardChangeProposalState{"cab"}})
	expectErr[*apierror.ValidationError](t, err, "an unknown state")
}

func stdRejection(t *testing.T, env *stdEnv) {
	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Flush caches"), AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatalf("propose from nothing: %v", err)
	}
	if len(p.Evidence) != 0 || p.SourceChangeRequest != nil || p.Customer == nil || p.Customer.ID != env.accounts["A"] {
		t.Fatalf("blank draft = %+v", p)
	}
	impact := domain.ChangeRequestImpactLow
	p = makeProposalComplete(t, env, p.ID, stdTag+" Flush caches")
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>None</p>"), TestPlan: stdp("<p>Check</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	awaiting, err := env.svc.SearchProposals(stdCtx(stdAuth2), domain.SearchStandardChangeProposalsRequest{AwaitingMyDecision: true})
	if err != nil || awaiting.Total != 1 || awaiting.Proposals[0].ID != p.ID {
		t.Fatalf("awaiting my decision = %+v, %v", awaiting, err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected"})
	expectErr[*apierror.ValidationError](t, err, "a rejection without a comment")
	p, err = env.svc.DecideProposal(stdCtx(stdAuth2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("Not low risk enough")})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if p.State != domain.StandardChangeProposalStateRejected || p.Template != nil || p.DecisionNote == nil || *p.DecisionNote != "Not low risk enough" ||
		p.DecidedBy == nil || p.DecidedBy.Email != stdAuth2 || p.ImplementationPlan == nil {
		t.Fatalf("rejected record = %+v", p)
	}
	// The first rejection ends it: the others are no longer asked.
	if s := approverStates(p); s[stdAuth2] != "REJECTED" || s[stdAuth1] != "CANCELLED" || s[stdCAB1] != "CANCELLED" {
		t.Fatalf("after the rejection = %v", s)
	}
	list, _ := env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{Query: stdp("Flush caches"),
		States: []domain.StandardChangeTemplateState{"active", "suspended", "retired"}})
	if list.Total != 0 {
		t.Fatalf("a rejected proposal made a template: %+v", list)
	}
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{Rationale: stdp("again")})
	expectErr[*apierror.ConflictError](t, err, "edit a rejected proposal")

	cp, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &p.ID})
	if err != nil {
		t.Fatalf("copy the rejected proposal: %v", err)
	}
	if cp.ID == p.ID || cp.State != domain.StandardChangeProposalStateDraft || cp.Subject == nil || *cp.Subject != *p.Subject || len(cp.Parameters) != 2 {
		t.Fatalf("copy = %+v", cp)
	}
	// A copy takes the copied proposal's customer; another is refused.
	if cp.Customer == nil || cp.Customer.ID != env.accounts["A"] {
		t.Fatalf("copy's customer = %+v", cp.Customer)
	}
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &p.ID, AccountID: stdp(env.accounts["B"])})
	expectErr[*apierror.ValidationError](t, err, "copy a proposal for another customer")
	if cp.CopiedFrom == nil || cp.CopiedFrom.ID != p.ID || cp.CopiedFrom.Name != p.Number {
		t.Fatalf("copy's copiedFrom = %+v, want %s %s", cp.CopiedFrom, p.ID, p.Number)
	}
	// ... and keeps it: a PATCH does not move a copy to another customer either.
	b, a := env.accounts["B"], env.accounts["A"]
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), cp.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a copy to another customer")
	if !strings.Contains(err.Error(), "a copy of proposal "+p.Number) {
		t.Fatalf("refusal = %v", err)
	}
	if again, _ := env.svc.GetProposal(stdCtx(stdProposer), cp.ID); again.Customer == nil || again.Customer.ID != a {
		t.Fatalf("a refused move changed the copy's customer: %+v", again.Customer)
	}
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), cp.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a}); err != nil {
		t.Fatalf("resend the copy's own customer: %v", err)
	}
	// A copy of a draft that has no customer yet names one, once.
	none, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " No customer yet")})
	if err != nil {
		t.Fatal(err)
	}
	nc, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &none.ID})
	if err != nil || nc.Customer != nil || nc.CopiedFrom == nil || nc.CopiedFrom.ID != none.ID {
		t.Fatalf("copy of a draft with no customer = %+v %+v, %v", nc.Customer, nc.CopiedFrom, err)
	}
	if nc, err = env.svc.UpdateProposal(stdCtx(stdProposer), nc.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a}); err != nil ||
		nc.Customer == nil || nc.Customer.ID != a {
		t.Fatalf("name the copy's customer: %+v, %v", nc.Customer, err)
	}
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), nc.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a copy that named its customer to another")
	// A draft started from nothing is not a copy.
	if none.CopiedFrom != nil {
		t.Fatalf("a draft from nothing has copiedFrom %+v", none.CopiedFrom)
	}
}

// stdMovedChangeRequests: a change request's customer is its project's account,
// and the Salesforce project sync may move a project (or a change request may be
// moved) to another account after a proposal cited it. The source change request
// a copy carries over is checked against the copy's customer like the evidence,
// and submit checks the source change request and the evidence again.
func stdMovedChangeRequests(t *testing.T, env *stdEnv) {
	ctx := stdCtx(stdProposer)
	sys := repository.WithSystemIdentity(context.Background())
	move := func(label, project string) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, `UPDATE work_item SET project_id = $2::uuid WHERE id = $1::uuid`, env.crs[label], env.projects[project]); err != nil {
			t.Fatalf("move %s to project %s: %v", label, project, err)
		}
	}
	t.Cleanup(func() { move("closedMoves", "A") })
	otherCustomer := "(another customer: " + stdTag + " Customer B)"

	// From customer A's closed change request; the evidence is another of A's (so
	// what is refused below is the source change request alone).
	p, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closedMoves"])})
	if err != nil || p.Customer == nil || p.Customer.ID != env.accounts["A"] {
		t.Fatalf("from customer A's change request: %+v, %v", p.Customer, err)
	}
	ids := []string{env.crs["closed1"]}
	if _, err := env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	makeProposalComplete(t, env, p.ID, stdTag+" Moved source")
	impact := domain.ChangeRequestImpactLow
	if _, err := env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}

	// The source change request moves to customer B: a copy would carry another
	// customer's change request, and the proposal cannot be submitted.
	move("closedMoves", "B")
	_, err = env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &p.ID})
	expectErr[*apierror.ValidationError](t, err, "copy a proposal whose source change request is another customer's")
	if !strings.Contains(err.Error(), "the change request a proposal starts from") || !strings.Contains(err.Error(), otherCustomer) {
		t.Fatalf("copy refusal = %v", err)
	}
	_, err = env.svc.SubmitProposal(ctx, p.ID)
	expectErr[*apierror.ValidationError](t, err, "submit with another customer's source change request")
	if !strings.Contains(err.Error(), otherCustomer) {
		t.Fatalf("submit refusal = %v", err)
	}
	if again, _ := env.svc.GetProposal(ctx, p.ID); again.State != domain.StandardChangeProposalStateDraft {
		t.Fatalf("a refused submit moved the proposal to %s", again.State)
	}

	// Back with customer A, and cited as evidence too: moved away again, the
	// evidence is what submit refuses.
	move("closedMoves", "A")
	ids = []string{env.crs["closed1"], env.crs["closedMoves"]}
	if _, err := env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	cp, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &p.ID})
	if err != nil || cp.SourceChangeRequest == nil || cp.SourceChangeRequest.ID != env.crs["closedMoves"] || len(cp.Evidence) != 2 {
		t.Fatalf("copy with customer A's source change request = %+v %d, %v", cp.SourceChangeRequest, len(cp.Evidence), err)
	}
	move("closedMoves", "B")
	_, err = env.svc.SubmitProposal(ctx, p.ID)
	expectErr[*apierror.ValidationError](t, err, "submit with another customer's evidence")
	if !strings.Contains(err.Error(), "evidence must be") || !strings.Contains(err.Error(), otherCustomer) {
		t.Fatalf("submit refusal = %v", err)
	}
	move("closedMoves", "A")
	if got, err := env.svc.SubmitProposal(ctx, p.ID); err != nil || got.State != domain.StandardChangeProposalStateAuthorityReview {
		t.Fatalf("submit once both are customer A's again: %v", err)
	}
}

func stdEvidence(t *testing.T, env *stdEnv) {
	_, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["open"])})
	expectErr[*apierror.ValidationError](t, err, "propose from an open change request")
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["rolledback"])})
	expectErr[*apierror.ValidationError](t, err, "propose from a rolled-back change request")

	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Evidence"), AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{env.crs["closed1"], env.crs["open"]}
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	expectErr[*apierror.ValidationError](t, err, "an open change request as evidence")
	ids = []string{env.crs["closed1"], strings.ToUpper(env.crs["closed2"]), env.crs["closed1"]}
	p, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	if err != nil || len(p.Evidence) != 2 {
		t.Fatalf("evidence = %+v, %v", p.Evidence, err)
	}
	// An empty category, impact or risk clears it.
	software, low := domain.ChangeRequestCategory("software"), domain.ChangeRequestImpactLow
	if p, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{Category: &software, Impact: &low}); err != nil || p.Category == nil || p.Impact == nil {
		t.Fatalf("set category and impact: %+v %v", p.StandardChangeContent, err)
	}
	none, noImpact := domain.ChangeRequestCategory(""), domain.ChangeRequestImpact("")
	if p, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{Category: &none, Impact: &noImpact}); err != nil || p.Category != nil || p.Impact != nil {
		t.Fatalf("clear category and impact: %v %v %v", p.Category, p.Impact, err)
	}
	bad := []domain.StandardChangeParameter{{Key: "Bad", Label: "x", Type: "text"}}
	_, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{Parameters: &bad})
	expectErr[*apierror.ValidationError](t, err, "a bad parameter key")
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify})
	expectErr[*apierror.ValidationError](t, err, "modify without a template")
	// Started from nothing (the catalogue's "Propose a standard change"): a draft
	// under a placeholder name the proposer renames.
	blank, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew})
	if err != nil || blank.Name != "Untitled standard change" || blank.State != domain.StandardChangeProposalStateDraft {
		t.Fatalf("a proposal started from nothing = %q %s, %v", blank.Name, blank.State, err)
	}
	renamed := stdTag + " Renamed"
	if blank, err = env.svc.UpdateProposal(stdCtx(stdProposer), blank.ID, domain.UpdateStandardChangeProposalRequest{Name: &renamed}); err != nil || blank.Name != renamed {
		t.Fatalf("rename: %q %v", blank.Name, err)
	}
	if _, err := env.svc.CreateProposal(context.Background(), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, Name: stdp("x")}); err == nil {
		t.Fatal("a caller with no identity created a proposal")
	}
}

// approveNew completes, submits and approves a new proposal and returns its template.
func approveNew(t *testing.T, env *stdEnv, proposer, id, name string) domain.StandardChangeTemplate {
	t.Helper()
	impact := domain.ChangeRequestImpactLow
	makeProposalCompleteAs(t, env, proposer, id, name)
	if _, err := env.svc.UpdateProposal(stdCtx(proposer), id, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>Restore</p>"), TestPlan: stdp("<p>Check</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(proposer), id); err != nil {
		t.Fatalf("submit %s: %v", name, err)
	}
	p := authorise(t, env, id)
	tpl, err := env.svc.GetTemplate(stdCtx(stdCAB1), p.Template.ID)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func makeProposalCompleteAs(t *testing.T, env *stdEnv, proposer, id, name string) {
	t.Helper()
	low := domain.ChangeRequestRiskLow
	params := []domain.StandardChangeParameter{
		{Key: "cluster", Label: "Cluster", Type: "text", Required: true},
		{Key: "env", Label: "Environment", Type: "select", Options: []string{"dev", "prod"}, Required: true},
	}
	if _, err := env.svc.UpdateProposal(stdCtx(proposer), id, domain.UpdateStandardChangeProposalRequest{
		Name: &name, Rationale: stdp("<p>Done many times, no incident.</p>"), Subject: stdp("Rotate keys on {{cluster}} ({{env}})"),
		Summary: stdp("Rotates a cluster's keys."), Description: stdp("<p>Rotate the keys of {{cluster}}</p>"),
		ImplementationPlan: stdp("<p>Run the rotation on {{cluster}}</p>"), Risk: &low, Parameters: &params,
	}); err != nil {
		t.Fatalf("complete the proposal: %v", err)
	}
}

// stdCustomer: a template belongs to one customer (account). A new proposal names
// it or takes it from where it starts, its evidence is that customer's closed
// change requests, a modification keeps its template's, and a name is unique per
// customer.
func stdCustomer(t *testing.T, env *stdEnv) {
	a, b := env.accounts["A"], env.accounts["B"]
	ctx := stdCtx(stdProposer)
	newReq := func(mod func(*domain.CreateStandardChangeProposalRequest)) domain.CreateStandardChangeProposalRequest {
		r := domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew}
		mod(&r)
		return r
	}

	// From a closed change request: its project's customer. Naming that same
	// customer is accepted; another, or a change request with no customer, is not.
	p, err := env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.SourceChangeRequestID = stdp(env.crs["closed2"])
	}))
	if err != nil || p.Customer == nil || p.Customer.ID != a || p.Customer.Name != stdTag+" Customer A" {
		t.Fatalf("from customer A's change request: customer %+v, %v", p.Customer, err)
	}
	if again, err := env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.SourceChangeRequestID, r.AccountID = stdp(env.crs["closed1"]), stdp(strings.ToUpper(a))
	})); err != nil || again.Customer == nil || again.Customer.ID != a {
		t.Fatalf("from a change request naming its own customer: %+v, %v", again.Customer, err)
	}
	_, err = env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.SourceChangeRequestID, r.AccountID = stdp(env.crs["closed1"]), &b
	}))
	expectErr[*apierror.ValidationError](t, err, "from customer A's change request for customer B")
	if !strings.Contains(err.Error(), "the change request's customer ("+stdTag+" Customer A)") {
		t.Fatalf("refusal = %v", err)
	}
	_, err = env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.SourceChangeRequestID = stdp(env.crs["closedNone"])
	}))
	expectErr[*apierror.ValidationError](t, err, "from a change request with no customer")
	_, err = env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.AccountID = stdp("00000000-0000-4000-8000-000000000bad")
	}))
	expectErr[*apierror.ValidationError](t, err, "an unknown customer")
	_, err = env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) { r.AccountID = stdp("not-a-uuid") }))
	expectErr[*apierror.ValidationError](t, err, "a malformed customer")
	// A draft that started from a change request keeps its customer.
	_, err = env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a from-change-request draft to another customer")
	if _, err := env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a}); err != nil {
		t.Fatalf("resend the draft's own customer: %v", err)
	}

	// Evidence must be closed change requests of the proposal's customer.
	ids := []string{env.crs["closed1"], env.crs["closedOther"]}
	_, err = env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	expectErr[*apierror.ValidationError](t, err, "another customer's change request as evidence")
	if !strings.Contains(err.Error(), "(another customer: "+stdTag+" Customer B)") {
		t.Fatalf("the refusal names the offender: %v", err)
	}
	ids = []string{env.crs["closedNone"]}
	_, err = env.svc.UpdateProposal(ctx, p.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	expectErr[*apierror.ValidationError](t, err, "a change request with no customer as evidence")

	// A new proposal started from nothing names its customer, or cites no evidence
	// and cannot be submitted until it does.
	blank, err := env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) { r.Name = stdp(stdTag + " Nobody's") }))
	if err != nil || blank.Customer != nil {
		t.Fatalf("a draft with no customer: %+v, %v", blank.Customer, err)
	}
	ids = []string{env.crs["closed1"]}
	_, err = env.svc.UpdateProposal(ctx, blank.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	expectErr[*apierror.ValidationError](t, err, "evidence on a draft with no customer")
	makeProposalCompleteAs(t, env, stdProposer, blank.ID, stdTag+" Nobody's")
	impact := domain.ChangeRequestImpactLow
	if _, err := env.svc.UpdateProposal(ctx, blank.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.SubmitProposal(ctx, blank.ID)
	expectErr[*apierror.ValidationError](t, err, "submit a new proposal with no customer")
	if !strings.Contains(err.Error(), "customer (accountId)") {
		t.Fatalf("submit refusal = %v", err)
	}
	// Naming customer B, its evidence is B's; moving it to A while it cites B's
	// evidence is refused and changes nothing.
	ids = []string{env.crs["closedOther"]}
	if blank, err = env.svc.UpdateProposal(ctx, blank.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b, EvidenceChangeRequestIDs: &ids}); err != nil ||
		blank.Customer == nil || blank.Customer.ID != b || len(blank.Evidence) != 1 {
		t.Fatalf("name customer B with B's evidence: %+v, %v", blank.Customer, err)
	}
	_, err = env.svc.UpdateProposal(ctx, blank.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a})
	expectErr[*apierror.ValidationError](t, err, "move a draft citing B's evidence to A")
	if again, _ := env.svc.GetProposal(ctx, blank.ID); again.Customer == nil || again.Customer.ID != b {
		t.Fatalf("a refused move changed the customer: %+v", again.Customer)
	}

	// The same name for two customers; not twice for one.
	shared := stdTag + " Shared name"
	tplA := approveNew(t, env, stdProposer, p.ID, shared)
	if tplA.Customer == nil || tplA.Customer.ID != a {
		t.Fatalf("customer A's template: %+v", tplA.Customer)
	}
	forB, err := env.svc.CreateProposal(stdCtx(stdCAB2), newReq(func(r *domain.CreateStandardChangeProposalRequest) {
		r.AccountID = &b
	}))
	if err != nil {
		t.Fatal(err)
	}
	ids = []string{env.crs["closedOther"]}
	if _, err := env.svc.UpdateProposal(stdCtx(stdCAB2), forB.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil {
		t.Fatalf("customer B's evidence: %v", err)
	}
	tplB := approveNew(t, env, stdCAB2, forB.ID, strings.ToUpper(shared))
	if tplB.Customer == nil || tplB.Customer.ID != b || tplB.ID == tplA.ID {
		t.Fatalf("customer B's template of the same name: %+v", tplB)
	}
	dupA, err := env.svc.CreateProposal(ctx, newReq(func(r *domain.CreateStandardChangeProposalRequest) { r.AccountID = &a }))
	if err != nil {
		t.Fatal(err)
	}
	makeProposalCompleteAs(t, env, stdProposer, dupA.ID, " "+strings.ToLower(shared)+" ")
	if _, err := env.svc.UpdateProposal(ctx, dupA.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.SubmitProposal(ctx, dupA.ID)
	expectErr[*apierror.ConflictError](t, err, "the same name twice for customer A")
	// ... and the index holds it should a submit be skipped.
	sys := repository.WithSystemIdentity(context.Background())
	if _, err := env.scoped.Exec(sys, `INSERT INTO standard_change_template (name, created_by, updated_by, account_id) VALUES ($1, 't', 't', $2::uuid)`,
		strings.ToLower(shared), a); err == nil {
		t.Fatal("a second live template of the same name for one customer was written")
	}
	if _, err := env.scoped.Exec(sys, `INSERT INTO standard_change_template (name, created_by, updated_by) VALUES ($1, 't', 't')`, stdTag+" No customer"); err == nil {
		t.Fatal("a template with no customer was written")
	}

	// A modification keeps its template's customer.
	_, err = env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tplB.ID, AccountID: &a})
	expectErr[*apierror.ValidationError](t, err, "modify customer B's template for customer A")
	m, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tplB.ID, AccountID: &b})
	if err != nil || m.Customer == nil || m.Customer.ID != b {
		t.Fatalf("modification of customer B's template: %+v, %v", m.Customer, err)
	}
	_, err = env.svc.UpdateProposal(ctx, m.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a})
	expectErr[*apierror.ValidationError](t, err, "move a modification to another customer")
	if _, err := env.svc.WithdrawProposal(ctx, m.ID); err != nil {
		t.Fatal(err)
	}

	// Reads: the catalogue and the proposals by customer.
	for label, want := range map[string]string{"A": tplA.ID, "B": tplB.ID} {
		acc := env.accounts[label]
		list, err := env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{Query: stdp("shared name"), AccountID: &acc})
		if err != nil || list.Total != 1 || list.Templates[0].ID != want || list.Templates[0].Customer == nil || list.Templates[0].Customer.ID != acc {
			t.Fatalf("customer %s's templates = %+v, %v", label, list, err)
		}
	}
	all, err := env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{Query: stdp("shared name")})
	if err != nil || all.Total != 2 {
		t.Fatalf("both customers' templates = %d, %v", all.Total, err)
	}
	_, err = env.svc.SearchTemplates(stdCtx(stdOutsider), domain.SearchStandardChangeTemplatesRequest{AccountID: stdp("x")})
	expectErr[*apierror.ValidationError](t, err, "search templates by a malformed customer")
	props, err := env.svc.SearchProposals(stdCtx(stdOutsider), domain.SearchStandardChangeProposalsRequest{AccountID: &b})
	if err != nil || props.Total != 3 {
		t.Fatalf("customer B's proposals = %+v, %v", props, err)
	}
	for _, sp := range props.Proposals {
		if sp.Customer == nil || sp.Customer.ID != b {
			t.Fatalf("a proposal of another customer in B's list: %+v", sp)
		}
	}
}

// stdCustomerHolds: a proposal's customer, once fixed, cannot be freed by deleting
// what it came from (ON DELETE SET NULL clears the links, not the lock), and the
// approval that applies it re-checks the source change request and the evidence
// against it.
func stdCustomerHolds(t *testing.T, env *stdEnv) {
	a, b := env.accounts["A"], env.accounts["B"]
	ctx := stdCtx(stdProposer)
	sys := repository.WithSystemIdentity(context.Background())
	exec := func(what, sql string, args ...any) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	move := func(label, project string) {
		t.Helper()
		exec("move "+label, `UPDATE work_item SET project_id = $2::uuid WHERE id = $1::uuid`, env.crs[label], env.projects[project])
	}
	t.Cleanup(func() { move("closedDriftSource", "A"); move("closedDriftEvidence", "A") })
	impact := domain.ChangeRequestImpactLow
	ready := func(id, name string) {
		t.Helper()
		makeProposalComplete(t, env, id, name)
		if _, err := env.svc.UpdateProposal(ctx, id, domain.UpdateStandardChangeProposalRequest{
			BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
			t.Fatal(err)
		}
	}

	// From a change request, then the links cleared as a deletion would clear them.
	fromCR, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closed1"])})
	if err != nil {
		t.Fatal(err)
	}
	exec("clear the source", `UPDATE standard_change_proposal SET source_change_request_id = NULL WHERE id = $1::uuid`, fromCR.ID)
	_, err = env.svc.UpdateProposal(ctx, fromCR.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a draft whose source change request was deleted")
	if !strings.Contains(err.Error(), "customer is fixed") {
		t.Fatalf("refusal = %v", err)
	}
	// A copy of it, likewise once the copied proposal is gone.
	cp, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &fromCR.ID})
	if err != nil || cp.Customer == nil || cp.Customer.ID != a {
		t.Fatalf("copy: %+v, %v", cp.Customer, err)
	}
	exec("clear the copy link", `UPDATE standard_change_proposal SET copied_from_proposal_id = NULL WHERE id = $1::uuid`, cp.ID)
	_, err = env.svc.UpdateProposal(ctx, cp.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a copy whose copied proposal was deleted")
	// A copy of a proposal with no customer names one once, then it is fixed.
	nobody, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Nobody's copy source")})
	if err != nil {
		t.Fatal(err)
	}
	cp2, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, CopyFromProposalID: &nobody.ID})
	if err != nil || cp2.Customer != nil {
		t.Fatalf("copy of a customerless proposal: %+v, %v", cp2.Customer, err)
	}
	if _, err := env.svc.UpdateProposal(ctx, cp2.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &a}); err != nil {
		t.Fatalf("the copy names its customer: %v", err)
	}
	exec("clear the copy link", `UPDATE standard_change_proposal SET copied_from_proposal_id = NULL WHERE id = $1::uuid`, cp2.ID)
	_, err = env.svc.UpdateProposal(ctx, cp2.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	expectErr[*apierror.ValidationError](t, err, "move a copy after it named its customer")
	// A draft from nothing still moves freely until it is submitted.
	free, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, AccountID: &a})
	if err != nil {
		t.Fatal(err)
	}
	if free, err = env.svc.UpdateProposal(ctx, free.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b}); err != nil || free.Customer == nil || free.Customer.ID != b {
		t.Fatalf("a draft from nothing moves: %+v, %v", free.Customer, err)
	}

	// The source change request moved to customer B after submit: the first
	// authorisation is recorded, the second, which would apply it, is refused.
	src, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closedDriftSource"])})
	if err != nil {
		t.Fatal(err)
	}
	ready(src.ID, stdTag+" Drift source")
	if _, err := env.svc.SubmitProposal(ctx, src.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.svc.GetProposal(ctx, src.ID); len(got.Evidence) != 1 || got.Evidence[0].Customer == nil || got.Evidence[0].Customer.ID != a {
		t.Fatalf("the evidence shows its customer: %+v", got.Evidence)
	}
	move("closedDriftSource", "B")
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), src.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), src.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "approve with the source change request now another customer's")
	if !strings.Contains(err.Error(), "this proposal cannot be approved now") {
		t.Fatalf("refusal = %v", err)
	}
	if got, _ := env.svc.GetProposal(ctx, src.ID); got.State != domain.StandardChangeProposalStateAuthorityReview || got.Template != nil ||
		approverStates(got)[stdAuth2] != "REQUESTED" || got.Stages[0].Approvals != 1 || got.Evidence[0].Customer == nil || got.Evidence[0].Customer.ID != b {
		t.Fatalf("a refused approval changed the proposal: %s %+v %v", got.State, got.Template, approverStates(got))
	}
	// Rejecting is still possible; nothing is written to the catalogue.
	if got, err := env.svc.DecideProposal(stdCtx(stdAuth2), src.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("Moved customer")}); err != nil ||
		got.State != domain.StandardChangeProposalStateRejected {
		t.Fatalf("reject: %v", err)
	}

	// Evidence that moved: refused, then approved once it is back.
	ev, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, AccountID: &a})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{env.crs["closedDriftEvidence"]}
	if _, err := env.svc.UpdateProposal(ctx, ev.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	ready(ev.ID, stdTag+" Drift evidence")
	if _, err := env.svc.SubmitProposal(ctx, ev.ID); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.UpdateProposal(ctx, ev.ID, domain.UpdateStandardChangeProposalRequest{AccountID: &b})
	if err == nil {
		t.Fatal("a submitted proposal's customer changed")
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), ev.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	move("closedDriftEvidence", "B")
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), ev.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "authorise with evidence now another customer's")
	move("closedDriftEvidence", "A")
	got, err := env.svc.DecideProposal(stdCtx(stdAuth2), ev.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	if err != nil || got.State != domain.StandardChangeProposalStateApproved || got.Template == nil {
		t.Fatalf("approve once the evidence is back: %+v, %v", got.Template, err)
	}
	if tpl, err := env.svc.GetTemplate(ctx, got.Template.ID); err != nil || tpl.Customer == nil || tpl.Customer.ID != a {
		t.Fatalf("the template's customer: %+v, %v", tpl.Customer, err)
	}

	// A proposal in review that names no customer (written before 0254, which
	// could not give it one) and cites evidence: the refusal says why.
	orphan, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, AccountID: &a})
	if err != nil {
		t.Fatal(err)
	}
	ids = []string{env.crs["closed2"]}
	if _, err := env.svc.UpdateProposal(ctx, orphan.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	ready(orphan.ID, stdTag+" No customer at approval")
	if _, err := env.svc.SubmitProposal(ctx, orphan.ID); err != nil {
		t.Fatal(err)
	}
	exec("drop the customer", `UPDATE standard_change_proposal SET account_id = NULL WHERE id = $1::uuid`, orphan.ID)
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), orphan.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), orphan.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "approve a proposal that names no customer")
	if !strings.Contains(err.Error(), "it names no customer") {
		t.Fatalf("refusal = %v", err)
	}
}

// readyDraft is a complete new draft for customer A, by stdProposer.
func readyDraft(t *testing.T, env *stdEnv, name string) domain.StandardChangeProposal {
	t.Helper()
	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: &name, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatalf("propose %s: %v", name, err)
	}
	makeProposalComplete(t, env, p.ID, name)
	impact := domain.ChangeRequestImpactLow
	p, err = env.svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>Restore</p>"), TestPlan: stdp("<p>Check</p>"), Impact: &impact})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// setAuthorityMember adds email to, or removes it from, the Standard Change Authority.
func setAuthorityMember(t *testing.T, env *stdEnv, email string, member bool) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	q := `DELETE FROM group_member WHERE user_id = (SELECT id FROM "user" WHERE email = $1)
		AND group_id IN (SELECT id FROM "group" WHERE name = $2)`
	if member {
		q = `INSERT INTO group_member (id, created_on, updated_on, created_by, updated_by, group_id, user_id)
			SELECT gen_random_uuid(), NOW(), NOW(), 't', 't', g.id, (SELECT id FROM "user" WHERE email = $1)
			  FROM "group" g WHERE g.name = $2 ORDER BY g.created_on LIMIT 1`
	}
	if _, err := env.scoped.Exec(sys, q, email, domain.StandardChangeAuthorityGroupName); err != nil {
		t.Fatalf("authority membership of %s: %v", email, err)
	}
}

// raisedFrom writes a closed change request of customer A raised from the
// template's current version, and returns its id.
func raisedFrom(t *testing.T, env *stdEnv, tpl domain.StandardChangeTemplate, label string) string {
	t.Helper()
	var id string
	if err := env.scoped.QueryRow(repository.WithSystemIdentity(context.Background()), `WITH wi AS (
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, 'CHG-STD-' || substr(md5(random()::text), 1, 10), $2, 'CHANGE_REQUEST', $3::uuid)
			RETURNING id)
		INSERT INTO change_request (id, state, change_model, implementation_plan, backout_plan, test_plan, risk, impact, standard_change_template_version_id)
		SELECT id, 'CLOSED', 'STANDARD', '<p>x</p>', '<p>x</p>', '<p>x</p>', 'LOW', 'LOW', $4::uuid FROM wi RETURNING id::text`,
		stdProposer, stdTag+" "+label, env.projects["A"], tpl.CurrentVersion.ID).Scan(&id); err != nil {
		t.Fatalf("raise a change request from %s: %v", tpl.Number, err)
	}
	return id
}

// stdAuthority: the one stage needs two different authority members; one
// rejection ends it; the proposer is left out even when they sit on the
// authority; a decision sent for another stage is refused; and a group too small
// to authorise is reported at submit.
func stdAuthority(t *testing.T, env *stdEnv) {
	ctx := stdCtx(stdProposer)
	t.Cleanup(func() {
		setAuthorityMember(t, env, stdAuth2, false)
		setAuthorityMember(t, env, stdAuth2, true)
		setAuthorityMember(t, env, stdCAB1, false)
		setAuthorityMember(t, env, stdCAB1, true)
	})

	p := readyDraft(t, env, stdTag+" Authority rejects")
	if _, err := env.svc.SubmitProposal(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	_, err := env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected"})
	expectErr[*apierror.ValidationError](t, err, "the authority rejects without a comment")
	p, err = env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("Too broad")})
	if err != nil || p.State != domain.StandardChangeProposalStateRejected || p.Template != nil || p.DecidedBy == nil || p.DecidedBy.Email != stdAuth1 {
		t.Fatalf("rejected by the authority = %s %+v, %v", p.State, p.DecidedBy, err)
	}
	if s := approverStates(p); s[stdAuth2] != "CANCELLED" || s[stdCAB1] != "CANCELLED" {
		t.Fatalf("after the rejection = %v", s)
	}
	if p.Stages[0].Decision == nil || *p.Stages[0].Decision != "REJECTED" {
		t.Fatalf("the rejected stage = %+v", p.Stages[0])
	}

	// A decision sent for a stage the proposal does not wait on (here another
	// proposal's) is refused and writes nothing.
	stale := readyDraft(t, env, stdTag+" Stale page")
	if stale, err = env.svc.SubmitProposal(ctx, stale.ID); err != nil {
		t.Fatal(err)
	}
	other := p.Stages[0].ID
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), stale.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &other})
	expectErr[*apierror.ConflictError](t, err, "a decision for another stage")
	if got, _ := env.svc.GetProposal(ctx, stale.ID); got.Stages[0].Approvals != 0 || approverStates(got)[stdAuth1] != "REQUESTED" {
		t.Fatalf("a decision for another stage was recorded: %+v %v", got.Stages, approverStates(got))
	}
	own := stale.Stages[0].ID
	if got, err := env.svc.DecideProposal(stdCtx(stdAuth1), stale.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &own}); err != nil ||
		got.Stages[0].Approvals != 1 {
		t.Fatalf("a decision for its own stage: %+v, %v", got.Stages, err)
	}
	if _, err := env.svc.WithdrawProposal(ctx, stale.ID); err != nil {
		t.Fatal(err)
	}

	// After one authorisation, the second member's rejection still ends it.
	p = readyDraft(t, env, stdTag+" Authority split")
	if _, err := env.svc.SubmitProposal(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	if p, err = env.svc.DecideProposal(stdCtx(stdAuth2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("No")}); err != nil ||
		p.State != domain.StandardChangeProposalStateRejected || p.Template != nil {
		t.Fatalf("split decision = %s, %v", p.State, err)
	}

	// A proposer who sits on the authority is listed cancelled and cannot decide
	// their own proposal; the two other members can.
	mine, err := env.svc.CreateProposal(stdCtx(stdAuth1), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	makeProposalCompleteAs(t, env, stdAuth1, mine.ID, stdTag+" An authority member's own")
	impact := domain.ChangeRequestImpactLow
	if _, err := env.svc.UpdateProposal(stdCtx(stdAuth1), mine.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	if mine, err = env.svc.SubmitProposal(stdCtx(stdAuth1), mine.ID); err != nil {
		t.Fatalf("submit an authority member's proposal: %v", err)
	}
	if s := approverStates(mine); s[stdAuth1] != "CANCELLED" || s[stdAuth2] != "REQUESTED" || s[stdCAB1] != "REQUESTED" {
		t.Fatalf("an authority member's own proposal asks = %v", s)
	}
	for _, a := range mine.Approvers {
		if a.CanDecide {
			t.Fatalf("the proposer is offered a decision: %+v", a)
		}
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), mine.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "the proposer, on the authority, decides")
	if _, err := env.svc.WithdrawProposal(stdCtx(stdAuth1), mine.ID); err != nil {
		t.Fatal(err)
	}

	// Two eligible members for this proposer (stdAuth1, stdCAB1): it is
	// submitted, and both are asked.
	setAuthorityMember(t, env, stdAuth2, false)
	p = readyDraft(t, env, stdTag+" Just enough")
	if p, err = env.svc.SubmitProposal(ctx, p.ID); err != nil {
		t.Fatalf("submit with two eligible members: %v", err)
	}
	if s := approverStates(p); s[stdAuth1] != "REQUESTED" || s[stdCAB1] != "REQUESTED" || len(s) != 2 {
		t.Fatalf("with two eligible members = %v", s)
	}
	// One eligible member: refused at submit, nothing written.
	setAuthorityMember(t, env, stdCAB1, false)
	q := readyDraft(t, env, stdTag+" Too few")
	_, err = env.svc.SubmitProposal(ctx, q.ID)
	expectErr[*apierror.ValidationError](t, err, "submit with one eligible member")
	if !strings.Contains(err.Error(), domain.StandardChangeAuthorityGroupName) {
		t.Fatalf("refusal = %v", err)
	}
	if got, _ := env.svc.GetProposal(ctx, q.ID); got.State != domain.StandardChangeProposalStateDraft || len(got.Stages) != 0 || got.SubmittedOn != nil {
		t.Fatalf("a refused submit moved the proposal: %s %+v", got.State, got.Stages)
	}
	setAuthorityMember(t, env, stdAuth2, true)
	setAuthorityMember(t, env, stdCAB1, true)
	if _, err := env.svc.WithdrawProposal(ctx, p.ID); err != nil {
		t.Fatal(err)
	}

	// People, not "user" rows: the email is not unique. A second row of stdAuth1
	// and one of stdCAB2, both on the authority. stdAuth1 authorises once.
	sys := repository.WithSystemIdentity(context.Background())
	for _, email := range []string{stdAuth1, stdCAB2} {
		var id string
		if err := env.scoped.QueryRow(sys, `INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, name, is_active)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1 || '-second', $1, 'second row', true) RETURNING id::text`, email).Scan(&id); err != nil {
			t.Fatalf("a second user row for %s: %v", email, err)
		}
		if _, err := env.scoped.Exec(sys, `INSERT INTO user_role (created_on, updated_on, user_id, role_id) VALUES (NOW(), NOW(), $1::uuid, '00000000-0000-0000-0000-000000000101')`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := env.scoped.Exec(sys, `INSERT INTO group_member (id, created_on, updated_on, created_by, updated_by, group_id, user_id)
			SELECT gen_random_uuid(), NOW(), NOW(), 't', 't', g.id, $1::uuid FROM "group" g WHERE g.name = $2 ORDER BY g.created_on LIMIT 1`,
			id, domain.StandardChangeAuthorityGroupName); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = env.scoped.Exec(sys, `DELETE FROM approval_stage_approver WHERE approver_user_id = $1::uuid`, id)
			_, _ = env.scoped.Exec(sys, `DELETE FROM group_member WHERE user_id = $1::uuid`, id)
			_, _ = env.scoped.Exec(sys, `DELETE FROM user_role WHERE user_id = $1::uuid`, id)
			_, _ = env.scoped.Exec(sys, `DELETE FROM "user" WHERE id = $1::uuid`, id)
		})
	}
	twice := readyDraft(t, env, stdTag+" One person, one authorisation")
	if _, err := env.svc.SubmitProposal(ctx, twice.ID); err != nil {
		t.Fatal(err)
	}
	if twice, err = env.svc.DecideProposal(stdCtx(stdAuth1), twice.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatalf("first authorisation: %v", err)
	}
	if twice.State != domain.StandardChangeProposalStateAuthorityReview || twice.Stages[0].Approvals != 1 {
		t.Fatalf("after one person's authorisation = %s %+v", twice.State, twice.Stages[0])
	}
	for _, a := range twice.Approvers {
		if a.CanDecide {
			t.Fatalf("stdAuth1 is offered a second authorisation: %+v", a)
		}
	}
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), twice.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "the same person authorises through a second user row")
	if got, _ := env.svc.GetProposal(ctx, twice.ID); got.State != domain.StandardChangeProposalStateAuthorityReview {
		t.Fatalf("one person applied the proposal alone: %s", got.State)
	}
	if _, err := env.svc.WithdrawProposal(ctx, twice.ID); err != nil {
		t.Fatal(err)
	}
	// Two rows of one person are one eligible member: with stdAuth2 and stdCAB1
	// out, only stdAuth1 (two rows) and stdCAB2's second row are left, and
	// stdCAB2 is the proposer, so a proposal stdCAB2 makes has one eligible
	// authoriser.
	setAuthorityMember(t, env, stdAuth2, false)
	setAuthorityMember(t, env, stdCAB1, false)
	one, err := env.svc.CreateProposal(stdCtx(stdCAB2), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	makeProposalCompleteAs(t, env, stdCAB2, one.ID, stdTag+" Two rows, one person")
	if _, err := env.svc.UpdateProposal(stdCtx(stdCAB2), one.ID, domain.UpdateStandardChangeProposalRequest{
		BackoutPlan: stdp("<p>x</p>"), TestPlan: stdp("<p>x</p>"), Impact: &impact}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.SubmitProposal(stdCtx(stdCAB2), one.ID)
	expectErr[*apierror.ValidationError](t, err, "submit with one eligible person who has two user rows")
	setAuthorityMember(t, env, stdAuth2, true)
	setAuthorityMember(t, env, stdCAB1, true)
	if err := env.svc.DeleteProposal(stdCtx(stdCAB2), one.ID); err != nil {
		t.Fatal(err)
	}
}

// stdDeleteCancel: a draft never submitted can be deleted by its proposer; an
// open proposal its proposer abandoned can be cancelled by an authority member,
// which frees the template it held.
func stdDeleteCancel(t *testing.T, env *stdEnv) {
	ctx := stdCtx(stdProposer)
	d, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " To delete"), AccountID: stdp(env.accounts["A"])})
	if err != nil || !d.CanDelete || d.CanCancel {
		t.Fatalf("draft = canDelete %v canCancel %v, %v", d.CanDelete, d.CanCancel, err)
	}
	if v, _ := env.svc.GetProposal(stdCtx(stdAuth1), d.ID); v.CanDelete || !v.CanCancel {
		t.Fatalf("as an authority member: canDelete %v canCancel %v", v.CanDelete, v.CanCancel)
	}
	expectErr[*apierror.ForbiddenError](t, env.svc.DeleteProposal(stdCtx(stdCAB2), d.ID), "someone else deletes a draft")
	if err := env.svc.DeleteProposal(ctx, d.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = env.svc.GetProposal(ctx, d.ID)
	expectErr[*apierror.NotFoundError](t, err, "read a deleted draft")

	s := readyDraft(t, env, stdTag+" Submitted, not deleted")
	if _, err := env.svc.SubmitProposal(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	expectErr[*apierror.ConflictError](t, env.svc.DeleteProposal(ctx, s.ID), "delete a submitted proposal")

	_, err = env.svc.CancelProposal(stdCtx(stdCAB2), s.ID, domain.CancelStandardChangeProposalRequest{Reason: "x"})
	expectErr[*apierror.ForbiddenError](t, err, "a CAB member cancels")
	_, err = env.svc.CancelProposal(stdCtx(stdAuth1), s.ID, domain.CancelStandardChangeProposalRequest{Reason: "  "})
	expectErr[*apierror.ValidationError](t, err, "cancel without a reason")
	c, err := env.svc.CancelProposal(stdCtx(stdAuth1), s.ID, domain.CancelStandardChangeProposalRequest{Reason: "The proposer left"})
	if err != nil || c.State != domain.StandardChangeProposalStateCancelled || c.DecidedBy == nil || c.DecidedBy.Email != stdAuth1 ||
		c.DecisionNote == nil || *c.DecisionNote != "The proposer left" || approverStates(c)[stdAuth2] != "CANCELLED" {
		t.Fatalf("cancelled = %s %+v %v, %v", c.State, c.DecidedBy, approverStates(c), err)
	}
	_, err = env.svc.CancelProposal(stdCtx(stdAuth2), s.ID, domain.CancelStandardChangeProposalRequest{Reason: "again"})
	expectErr[*apierror.ConflictError](t, err, "cancel twice")

	// An abandoned revision holds its template until it is cancelled.
	base, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	tpl := approveNew(t, env, stdProposer, base.ID, stdTag+" Held by a draft")
	m, err := env.svc.CreateProposal(stdCtx(stdCAB2), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	expectErr[*apierror.ConflictError](t, err, "a second revision while one is open")
	if _, err := env.svc.CancelProposal(stdCtx(stdAuth1), m.ID, domain.CancelStandardChangeProposalRequest{Reason: "Abandoned"}); err != nil {
		t.Fatalf("cancel the abandoned revision: %v", err)
	}
	next, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatalf("a revision once the abandoned one is cancelled: %v", err)
	}
	// One's own proposal is withdrawn or deleted, not cancelled.
	own, err := env.svc.CreateProposal(stdCtx(stdAuth1), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.CancelProposal(stdCtx(stdAuth1), own.ID, domain.CancelStandardChangeProposalRequest{Reason: "mine"})
	expectErr[*apierror.ConflictError](t, err, "cancel one's own proposal")
	if err := env.svc.DeleteProposal(stdCtx(stdAuth1), own.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeleteProposal(ctx, next.ID); err != nil {
		t.Fatal(err)
	}
}

// stdTemplateGates: anyone responsible may suspend; only the authority puts a
// template back in use, with a note; and the template keeps a history.
func stdTemplateGates(t *testing.T, env *stdEnv) {
	ctx := stdCtx(stdProposer)
	base, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	tpl := approveNew(t, env, stdProposer, base.ID, stdTag+" Gates")
	if tpl, _ = env.svc.GetTemplate(ctx, tpl.ID); !tpl.CanSuspend || tpl.CanResume || tpl.CanClearReview {
		t.Fatalf("the owner of an active template: %v %v %v", tpl.CanSuspend, tpl.CanResume, tpl.CanClearReview)
	}
	if tpl, err = env.svc.SuspendTemplate(ctx, tpl.ID, domain.SuspendStandardChangeTemplateRequest{Reason: "Investigating"}); err != nil {
		t.Fatalf("the owner suspends: %v", err)
	}
	if v, _ := env.svc.GetTemplate(ctx, tpl.ID); v.CanResume {
		t.Fatal("the owner may resume")
	}
	if v, _ := env.svc.GetTemplate(stdCtx(stdAuth1), tpl.ID); !v.CanResume || v.CanSuspend {
		t.Fatalf("an authority member: canResume %v canSuspend %v", v.CanResume, v.CanSuspend)
	}
	_, err = env.svc.ResumeTemplate(stdCtx(stdCAB2), tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: "ok"})
	expectErr[*apierror.ForbiddenError](t, err, "a CAB member resumes")
	_, err = env.svc.ResumeTemplate(stdCtx(stdAuth1), tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: " "})
	expectErr[*apierror.ValidationError](t, err, "resume without a note")
	if tpl, err = env.svc.ResumeTemplate(stdCtx(stdAuth1), tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: "Fixed in the runbook"}); err != nil ||
		tpl.State != domain.StandardChangeTemplateStateActive {
		t.Fatalf("the authority resumes: %v", err)
	}

	_, err = env.svc.ClearTemplateReview(stdCtx(stdAuth1), tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: "x"})
	expectErr[*apierror.ConflictError](t, err, "clear a flag that is not raised")
	cr := raisedFrom(t, env, tpl, "rolled back")
	repo := repository.NewStandardChangeRepository(env.scoped)
	if err := env.scoped.InTx(repository.WithSystemIdentity(context.Background()), func(tx pgx.Tx) error {
		return repo.FlagReviewRequired(context.Background(), tx, cr, "Rolled back", "std-system@scp.test")
	}); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if v, _ := env.svc.GetTemplate(stdCtx(stdCAB2), tpl.ID); !v.ReviewRequired || v.CanClearReview {
		t.Fatalf("a CAB member sees the flag: %v, canClearReview %v", v.ReviewRequired, v.CanClearReview)
	}
	_, err = env.svc.ClearTemplateReview(ctx, tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: "fine"})
	expectErr[*apierror.ForbiddenError](t, err, "the owner clears the flag")
	if tpl, err = env.svc.ClearTemplateReview(stdCtx(stdAuth2), tpl.ID, domain.StandardChangeTemplateNoteRequest{Note: "One-off network fault"}); err != nil || tpl.ReviewRequired {
		t.Fatalf("the authority clears the flag: %v", err)
	}

	type entry struct {
		action domain.StandardChangeTemplateEventAction
		note   string
		actor  string
	}
	want := []entry{
		{domain.StandardChangeTemplateEventReviewCleared, "One-off network fault", stdAuth2},
		{domain.StandardChangeTemplateEventReviewFlagged, "Rolled back", "std-system@scp.test"},
		{domain.StandardChangeTemplateEventResumed, "Fixed in the runbook", stdAuth1},
		{domain.StandardChangeTemplateEventSuspended, "Investigating", stdProposer},
	}
	if len(tpl.History) != len(want) {
		t.Fatalf("history = %+v", tpl.History)
	}
	for i, w := range want {
		h := tpl.History[i]
		if h.Action != w.action || h.Note == nil || *h.Note != w.note || h.Actor != w.actor {
			t.Fatalf("history[%d] = %+v %v, want %+v", i, h, stringOr(h.Note), w)
		}
	}
	if tpl.History[1].ChangeRequest == nil || tpl.History[1].ChangeRequest.ID != cr {
		t.Fatalf("the flag's change request = %+v", tpl.History[1].ChangeRequest)
	}
}

func stringOr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// stdDuplicates: a change already done from a template belongs to that template,
// a name waiting for approval is not proposed twice, and the placeholder name is
// no name.
func stdDuplicates(t *testing.T, env *stdEnv) {
	ctx := stdCtx(stdProposer)
	base, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	tpl := approveNew(t, env, stdProposer, base.ID, stdTag+" Dup base")
	cr := raisedFrom(t, env, tpl, "done from Dup base")

	_, err = env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: &cr})
	expectErr[*apierror.ValidationError](t, err, "a new template from a change raised from a template")
	if !strings.Contains(err.Error(), tpl.Number) || !strings.Contains(err.Error(), "propose a revision") {
		t.Fatalf("refusal = %v", err)
	}
	n, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Dup evidence"), AccountID: stdp(env.accounts["A"])})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{cr}
	_, err = env.svc.UpdateProposal(ctx, n.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids})
	expectErr[*apierror.ValidationError](t, err, "cite it for a new template")
	// ... but it is evidence for a revision of its own template.
	m, err := env.svc.CreateProposal(ctx, domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatal(err)
	}
	if m, err = env.svc.UpdateProposal(ctx, m.ID, domain.UpdateStandardChangeProposalRequest{EvidenceChangeRequestIDs: &ids}); err != nil || len(m.Evidence) != 1 {
		t.Fatalf("cite it for its own template's revision: %v", err)
	}
	if err := env.svc.DeleteProposal(ctx, m.ID); err != nil {
		t.Fatal(err)
	}

	// Two proposals of one customer under one name.
	first := readyDraft(t, env, stdTag+" Same name")
	if _, err := env.svc.SubmitProposal(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	second := readyDraft(t, env, strings.ToUpper(stdTag+" Same name"))
	_, err = env.svc.SubmitProposal(ctx, second.ID)
	expectErr[*apierror.ConflictError](t, err, "the same name twice")
	if !strings.Contains(err.Error(), first.Number) {
		t.Fatalf("refusal = %v", err)
	}
	if _, err := env.svc.WithdrawProposal(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(ctx, second.ID); err != nil {
		t.Fatalf("submit once the other is withdrawn: %v", err)
	}
	if _, err := env.svc.WithdrawProposal(ctx, second.ID); err != nil {
		t.Fatal(err)
	}

	// The placeholder name, and a summary with a placeholder.
	u := readyDraft(t, env, "Untitled standard change")
	_, err = env.svc.SubmitProposal(ctx, u.ID)
	expectErr[*apierror.ValidationError](t, err, "submit under the placeholder name")
	_, err = env.svc.UpdateProposal(ctx, u.ID, domain.UpdateStandardChangeProposalRequest{Summary: stdp("Rotates {{cluster}}")})
	expectErr[*apierror.ValidationError](t, err, "a placeholder in the summary")
	blank := ""
	if _, err := env.svc.UpdateProposal(ctx, u.ID, domain.UpdateStandardChangeProposalRequest{Name: stdp(stdTag + " Named"), Summary: &blank}); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.SubmitProposal(ctx, u.ID)
	expectErr[*apierror.ValidationError](t, err, "submit without a summary")
	if !strings.Contains(err.Error(), "summary") {
		t.Fatalf("refusal = %v", err)
	}
	if err := env.svc.DeleteProposal(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
}

// stdOldTwoStageProposal: a proposal the CAB approved under the old flow is in
// AUTHORITY_REVIEW with two stages, the settled "CAB Approval" (one approval
// needed, APPROVED) and then the authority's. 0287 leaves it as it is. It is
// decided on its latest stage only: a decision sent for the CAB stage is refused,
// a CAB member with no row on the authority stage cannot decide, and the second
// authorisation applies it.
func stdOldTwoStageProposal(t *testing.T, env *stdEnv) {
	sys := repository.WithSystemIdentity(context.Background())
	p := readyDraft(t, env, stdTag+" Approved by the CAB")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
		t.Fatal(err)
	}
	var cabStage string
	if err := env.scoped.QueryRow(sys, `INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label, required_approvals)
		VALUES (gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, 'APPROVED', 'CAB Approval', 1)
		RETURNING id::text`, p.ID, cabGroupID).Scan(&cabStage); err != nil {
		t.Fatalf("seed the CAB stage: %v", err)
	}
	if _, err := env.scoped.Exec(sys, `INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, state)
		SELECT gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, u.id, 'APPROVED' FROM "user" u WHERE u.email = $3`,
		p.ID, cabStage, stdCAB2); err != nil {
		t.Fatalf("seed the CAB approval: %v", err)
	}
	got, err := env.svc.GetProposal(stdCtx(stdProposer), p.ID)
	if err != nil || got.State != domain.StandardChangeProposalStateAuthorityReview || len(got.Stages) != 2 ||
		got.Stages[0].ID != cabStage || got.Stages[0].Label != "CAB Approval" || got.Stages[0].RequiredApprovals != 1 ||
		got.Stages[0].Approvals != 1 || got.Stages[0].Decision == nil ||
		got.Stages[1].Label != domain.StandardChangeAuthorityGroupName || got.Stages[1].RequiredApprovals != 2 || got.Stages[1].Approvals != 0 {
		t.Fatalf("old two-stage proposal = %s %+v, %v", got.State, got.Stages, err)
	}
	authorityStage := got.Stages[1].ID

	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &cabStage})
	expectErr[*apierror.ConflictError](t, err, "a decision sent for the settled CAB stage")
	if !strings.Contains(err.Error(), "nothing was recorded") {
		t.Fatalf("stale-stage refusal = %v", err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdCAB2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "the CAB approver decides again")

	p1, err := env.svc.DecideProposal(stdCtx(stdAuth1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &authorityStage})
	if err != nil || p1.State != domain.StandardChangeProposalStateAuthorityReview || p1.Template != nil ||
		p1.Stages[0].Approvals != 1 || p1.Stages[1].Approvals != 1 {
		t.Fatalf("first authorisation = %s %+v, %v", p1.State, p1.Stages, err)
	}
	p2, err := env.svc.DecideProposal(stdCtx(stdAuth2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &authorityStage})
	if err != nil || p2.State != domain.StandardChangeProposalStateApproved || p2.Template == nil ||
		p2.DecidedBy == nil || p2.DecidedBy.Email != stdAuth2 || p2.Stages[1].Approvals != 2 || p2.Stages[0].Approvals != 1 {
		t.Fatalf("second authorisation = %s %+v, %v", p2.State, p2.Stages, err)
	}
}

// Migration 0287 removes the CAB stage. A proposal still in CAB review goes back
// to draft, submitted_on cleared, and loses its CAB stage: a clean draft again,
// which can be deleted; submitted again it goes to the authority, and a decision
// sent for its old stage is refused. A proposal the CAB approved under the old
// flow, already in authority review, loses its CAB stage too and carries on with
// the authority's. A decided one keeps every stage, the record of who approved
// the template in force. The state check no longer admits CAB_REVIEW. Running it
// twice changes nothing more.
//
// The tables belong to STANDARD_CHANGE_TEST_DSN's role, the only one here that
// may alter them, so the whole file runs there. Its UPDATEs also run as
// STANDARD_CHANGE_TEST_APP_DSN's role (no BYPASSRLS, so FORCE row-level security
// applies, as it does to the role make migrate uses on staging and production),
// on a plain session nobody flagged internal: they must still find the rows.
func TestStandardChangeIntegration_NoCABStageMigration(t *testing.T) {
	dsns := stdDSNs(t)
	ownerDSN, ok := dsns["owner"]
	if !ok {
		t.Skip("STANDARD_CHANGE_TEST_DSN not set: only the tables' owner can run 0287")
	}
	migration, err := os.ReadFile("../../migrations/0287_standard_change_no_cab_stage.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	env := newStdEnv(t, ownerDSN)
	sys := repository.WithSystemIdentity(context.Background())
	exec := func(what, sql string, args ...any) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	run := func(pool *pgxpool.Pool, what, sql string) {
		t.Helper()
		conn, err := pool.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		if _, err := conn.Exec(context.Background(), sql); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		var flag string
		if err := conn.QueryRow(context.Background(), `SELECT COALESCE(current_setting('app.is_internal', true), '')`).Scan(&flag); err != nil || flag != "" {
			t.Fatalf("%s: app.is_internal = %q afterwards (%v), want it cleared", what, flag, err)
		}
	}
	// The fixtures need the check the old code wrote under; the migration puts
	// the new one back, and so does the cleanup should the test stop first.
	exec("relax the state check", `ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check`)
	t.Cleanup(func() {
		if _, err := env.pool.Exec(context.Background(), string(migration)); err != nil {
			t.Errorf("restore the state check: %v", err)
		}
	})
	// A proposal as the old code left it in CAB review: one "CAB Approval" stage,
	// a single approval needed, its approvers asked.
	inCAB := func(name string) domain.StandardChangeProposal {
		t.Helper()
		p := readyDraft(t, env, name)
		if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
			t.Fatal(err)
		}
		exec("move to CAB review", `UPDATE standard_change_proposal SET state = 'CAB_REVIEW' WHERE id = $1::uuid`, p.ID)
		exec("make the stage the CAB's", `UPDATE approval_stage SET checkpoint_label = 'CAB Approval', required_approvals = 1 WHERE work_item_id = $1::uuid`, p.ID)
		got, err := env.svc.GetProposal(stdCtx(stdProposer), p.ID)
		if err != nil || len(got.Stages) != 1 || got.Stages[0].Label != "CAB Approval" || approverStates(got)[stdAuth1] != "REQUESTED" {
			t.Fatalf("CAB review fixture = %+v, %v", got.Stages, err)
		}
		return got
	}
	// A settled "CAB Approval" stage, an hour older than the authority's, as the
	// old flow left a proposal the CAB had approved.
	addCABStage := func(id string) {
		t.Helper()
		var stage string
		if err := env.scoped.QueryRow(sys, `INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label, required_approvals)
			VALUES (gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, 'APPROVED', 'CAB Approval', 1)
			RETURNING id::text`, id, cabGroupID).Scan(&stage); err != nil {
			t.Fatalf("seed the CAB stage: %v", err)
		}
		exec("seed the CAB approval", `INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, state)
			SELECT gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, u.id, 'APPROVED' FROM "user" u WHERE u.email = $3`,
			id, stage, stdCAB2)
	}
	cab := inCAB(stdTag + " Waiting on the CAB")
	waiting := readyDraft(t, env, stdTag+" Waiting on the authority")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), waiting.ID); err != nil {
		t.Fatal(err)
	}
	addCABStage(waiting.ID)
	if got, err := env.svc.GetProposal(stdCtx(stdProposer), waiting.ID); err != nil || len(got.Stages) != 2 {
		t.Fatalf("authority review fixture = %+v, %v", got.Stages, err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), waiting.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	decided := readyDraft(t, env, stdTag+" Decided")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), decided.ID); err != nil {
		t.Fatal(err)
	}
	addCABStage(decided.ID)
	authorise(t, env, decided.ID)

	checkReturned := func(what string, before domain.StandardChangeProposal) domain.StandardChangeProposal {
		t.Helper()
		got, err := env.svc.GetProposal(stdCtx(stdProposer), before.ID)
		if err != nil || got.State != domain.StandardChangeProposalStateDraft || got.SubmittedOn != nil || !got.CanEdit || !got.CanSubmit || !got.CanDelete {
			t.Fatalf("%s: returned = %s submittedOn %v canEdit %v canSubmit %v canDelete %v, %v",
				what, got.State, got.SubmittedOn, got.CanEdit, got.CanSubmit, got.CanDelete, err)
		}
		if len(got.Stages) != 0 || len(got.Approvers) != 0 {
			t.Fatalf("%s: the CAB stage is still there: %+v %+v", what, got.Stages, got.Approvers)
		}
		return got
	}
	for run1 := 1; run1 <= 2; run1++ {
		run(env.pool, fmt.Sprintf("run %d of 0287", run1), string(migration))
		checkReturned(fmt.Sprintf("run %d", run1), cab)
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), waiting.ID); got.State != domain.StandardChangeProposalStateAuthorityReview ||
			len(got.Stages) != 1 || got.Stages[0].Label != domain.StandardChangeAuthorityGroupName ||
			got.Stages[0].Approvals != 1 || approverStates(got)[stdAuth2] != "REQUESTED" || approverStates(got)[stdCAB2] != "" {
			t.Fatalf("run %d: a proposal in authority review = %s %+v %v, want only the authority's stage", run1, got.State, got.Stages, approverStates(got))
		}
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), decided.ID); got.State != domain.StandardChangeProposalStateApproved ||
			len(got.Stages) != 2 || got.Stages[0].Label != "CAB Approval" || got.Stages[1].Approvals != 2 {
			t.Fatalf("run %d: a decided proposal was touched: %s %+v", run1, got.State, got.Stages)
		}
	}
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_proposal SET state = 'CAB_REVIEW' WHERE id = $1::uuid`, cab.ID); err == nil {
		t.Fatal("the state check still admits CAB_REVIEW")
	}

	// Submitted again, the returned draft goes to the authority, with its one
	// stage; a decision sent for the old CAB stage (a page loaded before) is refused.
	resubmitted, err := env.svc.SubmitProposal(stdCtx(stdProposer), cab.ID)
	if err != nil || resubmitted.State != domain.StandardChangeProposalStateAuthorityReview || len(resubmitted.Stages) != 1 ||
		resubmitted.Stages[0].Label != domain.StandardChangeAuthorityGroupName || resubmitted.Stages[0].RequiredApprovals != 2 {
		t.Fatalf("submitted again = %s %+v, %v", resubmitted.State, resubmitted.Stages, err)
	}
	oldStage := cab.Stages[0].ID
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), cab.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &oldStage})
	expectErr[*apierror.ConflictError](t, err, "a decision for the CAB stage, sent after the proposal was submitted again")
	if !strings.Contains(err.Error(), "the stage you decided on is not the one this proposal now waits on") {
		t.Fatalf("stale-stage refusal = %v", err)
	}
	if got, _ := env.svc.GetProposal(stdCtx(stdProposer), cab.ID); got.Stages[0].Approvals != 0 {
		t.Fatalf("a stale decision was recorded: %+v", got.Stages)
	}
	if got := authorise(t, env, cab.ID); got.Template == nil {
		t.Fatalf("approved without a template: %+v", got)
	}

	// The UPDATEs as a role row-level security applies to.
	appDSN, ok := dsns["app"]
	if !ok {
		return
	}
	// Without FORCE row-level security on the proposal (migration 100026) the
	// app role's run would find the rows with or without 0287's app.is_internal
	// bracket, and prove nothing about it.
	var forced bool
	if err := env.scoped.QueryRow(sys, `SELECT relforcerowsecurity FROM pg_class WHERE oid = 'standard_change_proposal'::regclass`).Scan(&forced); err != nil {
		t.Fatalf("read standard_change_proposal's row-level security: %v", err)
	}
	if !forced {
		t.Skip("standard_change_proposal is not FORCE row-level secured on this database (apply migrations/100026_standard_change_rls.sql): " +
			"the app role's run of 0287 would not exercise its app.is_internal bracket")
	}
	appPool, err := pgxpool.New(context.Background(), appDSN)
	if err != nil {
		t.Fatalf("connect as the app role: %v", err)
	}
	t.Cleanup(appPool.Close)
	exec("relax the state check", `ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check`)
	again := inCAB(stdTag + " Waiting on the CAB, row-level security")
	// The app role, on a session nobody flagged internal, sees no proposal: what
	// 0287's UPDATEs find below, they find through its bracket.
	var seen int
	if err := appPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM standard_change_proposal WHERE id = $1::uuid`, again.ID).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("the app role sees %d proposal(s) unflagged (%v): row-level security is not in force for it", seen, err)
	}
	updatesOnly := regexp.MustCompile(`(?s)ALTER TABLE standard_change_proposal [^;]*;`).ReplaceAllString(string(migration), "")
	if strings.Contains(updatesOnly, "ALTER TABLE") || !strings.Contains(updatesOnly, "UPDATE standard_change_proposal") {
		t.Fatalf("could not take the ALTERs out of 0287:\n%s", updatesOnly)
	}
	run(appPool, "0287's updates as the app role", updatesOnly)
	checkReturned("as the app role", again)
	run(env.pool, "0287 again", string(migration))
}
