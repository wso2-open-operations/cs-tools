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
	"os"
	"strings"
	"testing"

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
	stdTag      = "ZZ std-test"
	cabGroupID  = "00000000-0000-4000-8000-00000000ca01" // migration 0188
)

type stdEnv struct {
	svc    StandardChangeService
	scoped *repository.Scoped
	crs    map[string]string // label -> change request id
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
	for _, email := range []string{stdProposer, stdCAB1, stdCAB2, stdOutsider} {
		var id string
		if err := s.QueryRow(sys, `INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, name, is_active)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $1, $2, true) RETURNING id::text`, email, strings.Split(email, "@")[0]).Scan(&id); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if _, err := s.Exec(sys, `INSERT INTO user_role (created_on, updated_on, user_id, role_id) VALUES (NOW(), NOW(), $1::uuid, '00000000-0000-0000-0000-000000000101')`, id); err != nil {
			t.Fatalf("seed role: %v", err)
		}
		if email != stdOutsider {
			if _, err := s.Exec(sys, `INSERT INTO group_member (id, created_on, updated_on, created_by, updated_by, group_id, user_id)
				VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1::uuid, $2::uuid)`, cabGroupID, id); err != nil {
				t.Fatalf("seed CAB member: %v", err)
			}
		}
	}
	env := &stdEnv{svc: NewStandardChangeService(repository.NewStandardChangeRepository(s), ctxIdentityAccess{}), scoped: s, crs: map[string]string{}}
	for label, state := range map[string]string{"closed1": "CLOSED", "closed2": "CLOSED", "open": "IMPLEMENT", "rolledback": "ROLLBACK"} {
		var id string
		if err := s.QueryRow(sys, `WITH wi AS (
				INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, description, type)
				VALUES (gen_random_uuid(), NOW(), NOW(), $1, $1, 'CHG-STD-' || substr(md5(random()::text), 1, 10), $2, '<p>Rotate keys on cluster aks-1</p>', 'CHANGE_REQUEST')
				RETURNING id)
			INSERT INTO change_request (id, state, change_model, implementation_plan, backout_plan, test_plan, risk, impact, category)
			SELECT id, $3::change_request_state_enum, 'NORMAL', '<p>Run the rotation on aks-1</p>', '<p>Restore the keys</p>', '<p>Check aks-1</p>',
			       'LOW', 'LOW', 'SOFTWARE' FROM wi RETURNING id::text`, stdProposer, stdTag+" "+label, state).Scan(&id); err != nil {
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
	if p.State != domain.StandardChangeProposalStateCABReview || p.SubmittedOn == nil || p.CanEdit {
		t.Fatalf("submitted = state %s, canEdit %v", p.State, p.CanEdit)
	}
	states := approverStates(p)
	if states[stdCAB1] != "REQUESTED" || states[stdCAB2] != "REQUESTED" || states[stdProposer] != "CANCELLED" {
		t.Fatalf("CAB stage = %v (the proposer, a CAB member, is listed cancelled)", states)
	}
	if _, ok := states[stdOutsider]; ok {
		t.Fatalf("a non-member was asked: %v", states)
	}

	// Who may decide.
	_, err = env.svc.DecideProposal(stdCtx(stdProposer), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "the proposer decides")
	_, err = env.svc.DecideProposal(stdCtx(stdOutsider), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ForbiddenError](t, err, "a non-member decides")
	viewed, _ := env.svc.GetProposal(stdCtx(stdCAB1), p.ID)
	for _, a := range viewed.Approvers {
		if a.CanDecide != (a.Approver.Email == stdCAB1) {
			t.Fatalf("canDecide for %s = %v", a.Approver.Email, a.CanDecide)
		}
	}

	p, err = env.svc.DecideProposal(stdCtx(stdCAB1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", Comment: stdp("ok")})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if p.State != domain.StandardChangeProposalStateApproved || p.Template == nil || p.ResultingVersion == nil || *p.ResultingVersion != 1 {
		t.Fatalf("approved = %+v", p)
	}
	if s := approverStates(p); s[stdCAB1] != "APPROVED" || s[stdCAB2] != "CANCELLED" {
		t.Fatalf("after approval = %v", s)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdCAB2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("late")})
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
	if tpl, err = env.svc.ResumeTemplate(stdCtx(stdProposer), tpl.ID); err != nil || tpl.State != domain.StandardChangeTemplateStateActive {
		t.Fatalf("resume by the owner: %v %v", tpl.State, err)
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
	if tpl, err = env.svc.ResumeTemplate(stdCtx(stdCAB1), tpl.ID); err != nil || tpl.State != domain.StandardChangeTemplateStateActive || tpl.SuspendedBy != nil {
		t.Fatalf("resume by a CAB member: %+v %v", tpl, err)
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
	if _, err := env.svc.DecideProposal(stdCtx(stdCAB1), m.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatalf("approve the modification: %v", err)
	}
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
	_, err = env.svc.DecideProposal(stdCtx(stdCAB1), dup.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	expectErr[*apierror.ConflictError](t, err, "approve a duplicate name")
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_template SET name = $2 WHERE id = $1::uuid`, tpl.ID, name); err != nil {
		t.Fatal(err)
	}
	if again, _ := env.svc.GetProposal(stdCtx(stdCAB1), dup.ID); again.State != domain.StandardChangeProposalStateCABReview || approverStates(again)[stdCAB1] != "REQUESTED" {
		t.Fatalf("a refused approval wrote something: %s %v", again.State, approverStates(again))
	}
	if _, err := env.svc.WithdrawProposal(stdCtx(stdProposer), dup.ID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if w, _ := env.svc.GetProposal(stdCtx(stdProposer), dup.ID); w.State != domain.StandardChangeProposalStateWithdrawn || approverStates(w)[stdCAB1] != "CANCELLED" {
		t.Fatalf("withdrawn = %s %v", w.State, approverStates(w))
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
	if _, err := env.svc.DecideProposal(stdCtx(stdCAB2), r.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
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
	_, err = env.svc.ResumeTemplate(stdCtx(stdCAB1), tpl.ID)
	expectErr[*apierror.ConflictError](t, err, "resume a retired template")

	mine, err := env.svc.SearchProposals(stdCtx(stdProposer), domain.SearchStandardChangeProposalsRequest{ProposedByMe: true})
	if err != nil || mine.Total != 3 {
		t.Fatalf("proposed by me = %d, %v", mine.Total, err)
	}
}

func stdRejection(t *testing.T, env *stdEnv) {
	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Flush caches")})
	if err != nil {
		t.Fatalf("propose from nothing: %v", err)
	}
	if len(p.Evidence) != 0 || p.SourceChangeRequest != nil {
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
	awaiting, err := env.svc.SearchProposals(stdCtx(stdCAB2), domain.SearchStandardChangeProposalsRequest{AwaitingMyDecision: true})
	if err != nil || awaiting.Total != 1 || awaiting.Proposals[0].ID != p.ID {
		t.Fatalf("awaiting my decision = %+v, %v", awaiting, err)
	}
	_, err = env.svc.DecideProposal(stdCtx(stdCAB2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected"})
	expectErr[*apierror.ValidationError](t, err, "a rejection without a comment")
	p, err = env.svc.DecideProposal(stdCtx(stdCAB2), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "rejected", Comment: stdp("Not low risk enough")})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if p.State != domain.StandardChangeProposalStateRejected || p.Template != nil || p.DecisionNote == nil || *p.DecisionNote != "Not low risk enough" ||
		p.DecidedBy == nil || p.DecidedBy.Email != stdCAB2 || p.ImplementationPlan == nil {
		t.Fatalf("rejected record = %+v", p)
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
}

func stdEvidence(t *testing.T, env *stdEnv) {
	_, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["open"])})
	expectErr[*apierror.ValidationError](t, err, "propose from an open change request")
	_, err = env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["rolledback"])})
	expectErr[*apierror.ValidationError](t, err, "propose from a rolled-back change request")

	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, Name: stdp(stdTag + " Evidence")})
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
