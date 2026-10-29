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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Change requests raised from a standard change template, end to end over the real
// change request and standard change services: the create renders the template,
// the template's fields stay frozen, Request Approval needs the template usable,
// and a rollback or a deviation flags the template for review. Same DSNs as
// TestStandardChangeIntegration.

func crCtx(t *testing.T, email string) context.Context {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, email))
	ctx = auth.WithIdentity(ctx, auth.Identity{Validated: true, UserEmail: email})
	return repository.WithCallerIdentity(ctx, repository.SearchScope{Unrestricted: true, HasInternalAccess: true, ViewerEmail: email})
}

func wantCode(t *testing.T, err error, code, what string) {
	t.Helper()
	var ve *apierror.ValidationError
	var ce *apierror.ConflictError
	got := ""
	switch {
	case errors.As(err, &ve):
		got = ve.Code
	case errors.As(err, &ce):
		got = ce.Code
	default:
		t.Fatalf("%s: err = %v (%T), want code %s", what, err, err, code)
	}
	if got != code {
		t.Fatalf("%s: code %q (%v), want %q", what, got, err, code)
	}
}

// stdApprovedTemplate takes a proposal from a closed change request through CAB
// approval and returns the template.
func stdApprovedTemplate(t *testing.T, env *stdEnv, name string) domain.StandardChangeTemplate {
	t.Helper()
	p, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closed1"])})
	if err != nil {
		t.Fatal(err)
	}
	makeProposalComplete(t, env, p.ID, name)
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
		t.Fatal(err)
	}
	p, err = env.svc.DecideProposal(stdCtx(stdCAB1), p.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := env.svc.GetTemplate(stdCtx(stdCAB1), p.Template.ID)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func TestChangeRequestStandardChangeIntegration(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			env := newStdEnv(t, dsn)
			pool, err := pgxpool.New(context.Background(), dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			crRepo := repository.NewChangeRequestRepository(repository.NewScoped(pool))
			stdRepo := repository.NewStandardChangeRepository(repository.NewScoped(pool))
			crs := WithStandardChangeTemplates(NewChangeRequestService(crRepo, repository.NewUserRepository(pool)), stdRepo)
			stdChangeRequestFlow(t, env, crs)
			t.Run("raised only for the template's customer", func(t *testing.T) { stdChangeRequestCustomer(t, env, crs, crRepo, stdRepo) })
		})
	}
}

func stdChangeRequestFlow(t *testing.T, env *stdEnv, crs ChangeRequestService) {
	ctx := crCtx(t, stdCAB2)
	tpl := stdApprovedTemplate(t, env, stdTag+" Key rotation")
	standard := domain.ChangeRequestTypeStandard
	normal := domain.ChangeRequestTypeNormal
	group := cabGroupID
	params := map[string]string{"cluster": " aks-9 ", "env": "prod"}
	projectA := env.projects["A"]
	raise := func(extra func(*domain.CreateChangeRequestRequest)) (domain.CreateChangeRequestResponse, error) {
		req := domain.CreateChangeRequestRequest{Type: &standard, GroupID: &group, ProjectID: &projectA, StandardChangeTemplateID: &tpl.ID, StandardChangeParameters: params}
		if extra != nil {
			extra(&req)
		}
		return crs.CreateChangeRequest(ctx, req)
	}

	// A standard change needs a template; a template needs type standard and
	// sends none of what it specifies.
	_, err := crs.CreateChangeRequest(ctx, domain.CreateChangeRequestRequest{Type: &standard, Subject: stdTag + " free", GroupID: &group})
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "a standard change with no template")
	_, err = raise(func(r *domain.CreateChangeRequestRequest) { r.Type = &normal })
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "a template with type normal")
	_, err = raise(func(r *domain.CreateChangeRequestRequest) { r.ImplementationPlan = stdp("<p>my own plan</p>") })
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "a template-specified field sent")
	_, err = raise(func(r *domain.CreateChangeRequestRequest) {
		r.StandardChangeParameters = map[string]string{"env": "prod"}
	})
	wantStdValidation(t, err, "Cluster (cluster) is required")
	_, err = raise(func(r *domain.CreateChangeRequestRequest) {
		r.StandardChangeParameters = map[string]string{"cluster": "a", "env": "qa"}
	})
	wantStdValidation(t, err, "must be one of: dev, prod")
	other := tpl.CurrentVersion.TemplateID // not a version id
	_, err = raise(func(r *domain.CreateChangeRequestRequest) { r.StandardChangeTemplateVersionID = &other })
	wantCode(t, err, apierror.CodeStandardChangeTemplateUnavailable, "a version that is not the current one")

	created, err := raise(func(r *domain.CreateChangeRequestRequest) { r.StandardChangeTemplateVersionID = &tpl.CurrentVersion.ID })
	if err != nil {
		t.Fatalf("raise from the template: %v", err)
	}
	id := created.ChangeRequest.ID
	cr, err := crs.GetChangeRequest(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if derefString(cr.Subject) != "Rotate keys on aks-9 (prod)" || derefString(cr.ImplementationPlan) != "<p>Run the rotation on aks-9</p>" ||
		derefString(cr.TestPlan) != "<p>Check aks-1</p>" || derefString(cr.Type) != string(standard) {
		t.Fatalf("rendered change request = title %q plan %q test %q type %q", derefString(cr.Subject), derefString(cr.ImplementationPlan), derefString(cr.TestPlan), derefString(cr.Type))
	}
	sc := cr.StandardChange
	if sc == nil || sc.Template.ID != tpl.ID || sc.VersionID != tpl.CurrentVersion.ID || sc.Version != 1 || len(sc.Parameters) != 2 ||
		sc.Parameters[0].Value == nil || *sc.Parameters[0].Value != "aks-9" {
		t.Fatalf("standardChange = %+v", sc)
	}
	if sc.Customer == nil || sc.Customer.ID != env.accounts["A"] || sc.Customer.Name != stdTag+" Customer A" {
		t.Fatalf("standardChange.customer = %+v", sc.Customer)
	}

	// A proposal's CAB rows live in the change request approval tables, but the
	// change request routes never reach them: it is decided on its own route only.
	pending, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
		Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(env.crs["closed2"])})
	if err != nil {
		t.Fatal(err)
	}
	makeProposalComplete(t, env, pending.ID, stdTag+" Pending")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), pending.ID); err != nil {
		t.Fatal(err)
	}
	_, err = crs.DecideChangeRequestApproval(crCtx(t, stdCAB1), pending.ID, "rejected")
	expectErr[*apierror.NotFoundError](t, err, "decide a proposal through the change request route")
	_, err = crs.GetChangeRequestApprovals(crCtx(t, stdCAB1), pending.ID)
	expectErr[*apierror.NotFoundError](t, err, "read a proposal's approvals through the change request route")
	if p, _ := env.svc.GetProposal(stdCtx(stdCAB1), pending.ID); p.State != domain.StandardChangeProposalStateCABReview || approverStates(p)[stdCAB1] != "REQUESTED" {
		t.Fatalf("the change request route touched the proposal: %s %v", p.State, approverStates(p))
	}

	// Frozen in New already; resending a stored value is accepted.
	plan := stdp("<p>something else</p>")
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ImplementationPlan: &plan})
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "change the plan")
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{Title: stdp("other")})
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "change the title")
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{Type: &normal})
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "change the type")
	same := cr.ImplementationPlan
	if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ImplementationPlan: &same, Title: cr.Subject, WorkNote: stdp("note")}); err != nil {
		t.Fatalf("resend the stored values: %v", err)
	}

	// No other change request becomes standard by a PATCH.
	n, err := crs.CreateChangeRequest(ctx, domain.CreateChangeRequestRequest{Type: &normal, Subject: stdTag + " normal", GroupID: &group})
	if err != nil {
		t.Fatal(err)
	}
	_, err = crs.PatchChangeRequest(ctx, n.ChangeRequest.ID, domain.PatchChangeRequestRequest{Type: &standard})
	wantCode(t, err, apierror.CodeChangeRequestStandardTemplateField, "a normal change made standard")

	// Request Approval needs the template usable.
	assess := domain.ChangeRequestStateAssess
	if _, err := env.svc.SuspendTemplate(stdCtx(stdCAB1), tpl.ID, domain.SuspendStandardChangeTemplateRequest{}); err != nil {
		t.Fatal(err)
	}
	_, err = raise(nil)
	wantCode(t, err, apierror.CodeStandardChangeTemplateUnavailable, "raise from a suspended template")
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{State: &assess})
	wantCode(t, err, apierror.CodeStandardChangeTemplateUnavailable, "request approval on a suspended template")
	if _, err := env.svc.ResumeTemplate(stdCtx(stdCAB1), tpl.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{State: &assess}); err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if cr, _ = crs.GetChangeRequest(ctx, id); cr.State == nil || *cr.State != string(domain.ChangeRequestStateScheduled) {
		t.Fatalf("a standard change is scheduled at once, state = %v", cr.State)
	}

	// A second one, raised before the first fails, deviates from its plan.
	second, err := raise(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := crs.PatchChangeRequest(ctx, second.ChangeRequest.ID, domain.PatchChangeRequestRequest{State: &assess}); err != nil {
		t.Fatal(err)
	}

	// The first is rolled back: the template is flagged.
	for _, st := range []domain.ChangeRequestState{domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview, domain.ChangeRequestStateRollback} {
		st := st
		if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{State: &st}); err != nil {
			t.Fatalf("move to %s: %v", st, err)
		}
	}
	got, _ := env.svc.GetTemplate(stdCtx(stdCAB1), tpl.ID)
	if !got.ReviewRequired || got.ReviewRequiredChangeRequest == nil || got.ReviewRequiredChangeRequest.ID != id ||
		got.ReviewRequiredReason == nil || !strings.Contains(*got.ReviewRequiredReason, "rolled back") || got.Usage.RolledBack != 1 {
		t.Fatalf("after a rollback = review %v %+v %v usage %+v", got.ReviewRequired, got.ReviewRequiredChangeRequest, got.ReviewRequiredReason, got.Usage)
	}
	if got.State != domain.StandardChangeTemplateStateActive {
		t.Fatalf("a flag does not suspend the template: %s", got.State)
	}
	if _, err := env.svc.ClearTemplateReview(stdCtx(stdProposer), tpl.ID); err != nil {
		t.Fatalf("the owner clears the flag: %v", err)
	}

	implement := domain.ChangeRequestStateImplement
	if _, err := crs.PatchChangeRequest(ctx, second.ChangeRequest.ID, domain.PatchChangeRequestRequest{State: &implement}); err != nil {
		t.Fatal(err)
	}
	if _, err := crs.PatchChangeRequest(ctx, second.ChangeRequest.ID, domain.PatchChangeRequestRequest{ImplementationDeviation: stdp("<p>skipped the check</p>")}); err != nil {
		t.Fatalf("record a deviation: %v", err)
	}
	got, _ = env.svc.GetTemplate(stdCtx(stdCAB1), tpl.ID)
	if !got.ReviewRequired || got.ReviewRequiredChangeRequest == nil || got.ReviewRequiredChangeRequest.ID != second.ChangeRequest.ID ||
		!strings.Contains(*got.ReviewRequiredReason, "deviated") {
		t.Fatalf("after a deviation = review %v %+v", got.ReviewRequired, got.ReviewRequiredChangeRequest)
	}
	// Cleared, the flag stays cleared when the deviated change moves on: only the
	// write that rolls a change back or records the deviation raises it.
	if _, err := env.svc.ClearTemplateReview(stdCtx(stdCAB1), tpl.ID); err != nil {
		t.Fatal(err)
	}
	review := domain.ChangeRequestStateReview
	if _, err := crs.PatchChangeRequest(ctx, second.ChangeRequest.ID, domain.PatchChangeRequestRequest{State: &review}); err != nil {
		t.Fatal(err)
	}
	if got, _ = env.svc.GetTemplate(stdCtx(stdCAB1), tpl.ID); got.ReviewRequired {
		t.Fatalf("a later state move raised the cleared flag again: %v", got.ReviewRequiredReason)
	}

	// A revised template: a change raised from version 1 and not yet approved
	// cannot be any more.
	third, err := raise(nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindModify, TemplateID: &tpl.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), m.ID, domain.UpdateStandardChangeProposalRequest{Rationale: stdp("<p>Fix the plan</p>")}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdCAB1), m.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	_, err = crs.PatchChangeRequest(ctx, third.ChangeRequest.ID, domain.PatchChangeRequestRequest{State: &assess})
	wantCode(t, err, apierror.CodeStandardChangeTemplateUnavailable, "request approval after the template was revised")
	if c, _ := crs.GetChangeRequest(ctx, third.ChangeRequest.ID); c.StandardChange == nil || c.StandardChange.Version != 1 {
		t.Fatalf("the change keeps the version it was raised from: %+v", c.StandardChange)
	}
}

// stdChangeRequestCustomer: a change request is raised from a template only for a
// project of the template's customer -- refused in the service (before the
// previous system is called on dual-write) and again in the create transaction
// -- and a PATCH never moves it to another customer's project, in any state.
func stdChangeRequestCustomer(t *testing.T, env *stdEnv, crs ChangeRequestService, crRepo repository.ChangeRequestRepository, stdRepo repository.StandardChangeRepository) {
	ctx := crCtx(t, stdCAB2)
	tpl := stdApprovedTemplate(t, env, stdTag+" Customer key rotation")
	if tpl.Customer == nil || tpl.Customer.ID != env.accounts["A"] {
		t.Fatalf("the template is customer A's: %+v", tpl.Customer)
	}
	standard := domain.ChangeRequestTypeStandard
	group := cabGroupID
	params := map[string]string{"cluster": "aks-3", "env": "dev"}
	req := func(project *string) domain.CreateChangeRequestRequest {
		return domain.CreateChangeRequestRequest{Type: &standard, GroupID: &group, ProjectID: project,
			StandardChangeTemplateID: &tpl.ID, StandardChangeParameters: params}
	}
	pa, pa2, pb := env.projects["A"], env.projects["A2"], env.projects["B"]

	_, err := crs.CreateChangeRequest(ctx, req(&pb))
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "raise customer A's template for customer B's project")
	_, err = crs.CreateChangeRequest(ctx, req(nil))
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "raise a template for no project")

	// Dual-write: refused before the previous system is called.
	mirror := &stubMirrorChangeRequestService{
		createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			t.Error("the previous system was called for another customer's project")
			return domain.CreateChangeRequestResponse{}, nil
		},
	}
	dual := WithStandardChangeTemplates(NewChangeRequestServiceWithSNMirror(crRepo, stubUserRepo{}, mirror), stdRepo)
	_, err = dual.CreateChangeRequest(ctx, req(&pb))
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "dual-write: raise for customer B's project")

	// The create transaction checks it again, whoever calls the repository.
	direct := req(&pb)
	direct.Subject = stdTag + " direct"
	direct.StandardChangeTemplateVersionID = &tpl.CurrentVersion.ID
	direct.StandardChangeTemplateID, direct.StandardChangeParameters = nil, map[string]string{"cluster": "aks-3", "env": "dev"}
	_, err = crRepo.CreateChangeRequest(ctx, direct, stdCAB2)
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "the repository's create for customer B's project")
	var left int
	if err := env.scoped.QueryRow(repository.WithSystemIdentity(context.Background()),
		`SELECT COUNT(*) FROM work_item WHERE subject = $1`, stdTag+" direct").Scan(&left); err != nil || left != 0 {
		t.Fatalf("a refused create left %d work item(s), %v", left, err)
	}

	// The template's customer's projects: either of them.
	created, err := crs.CreateChangeRequest(ctx, req(&pa))
	if err != nil {
		t.Fatalf("raise for customer A's project: %v", err)
	}
	id := created.ChangeRequest.ID
	if _, err := crs.CreateChangeRequest(ctx, req(&pa2)); err != nil {
		t.Fatalf("raise for customer A's other project: %v", err)
	}

	// PATCH: another customer's project is refused in New; another project of the
	// same customer is not; resending the stored one is accepted.
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ProjectID: &pb})
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "move to customer B's project in New")
	if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ProjectID: &pa2}); err != nil {
		t.Fatalf("move to customer A's other project in New: %v", err)
	}
	if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ProjectID: &pa2, WorkNote: stdp("same project")}); err != nil {
		t.Fatalf("resend the stored project: %v", err)
	}
	if cr, _ := crs.GetChangeRequest(ctx, id); !strings.EqualFold(cr.Project.ID, pa2) {
		t.Fatalf("project after the moves = %+v", cr.Project)
	}

	// After Request Approval (a standard change is scheduled at once): still
	// refused with the same code, and the project stays.
	assess := domain.ChangeRequestStateAssess
	if _, err := crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{State: &assess}); err != nil {
		t.Fatalf("request approval: %v", err)
	}
	_, err = crs.PatchChangeRequest(ctx, id, domain.PatchChangeRequestRequest{ProjectID: &pb})
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "move to customer B's project when scheduled")
	if cr, _ := crs.GetChangeRequest(ctx, id); !strings.EqualFold(cr.Project.ID, pa2) {
		t.Fatalf("a refused move changed the project: %+v", cr.Project)
	}

	// The Salesforce project sync (the source of truth for projects) is not
	// blocked: it may move a project to another account after a change request
	// was raised for it. Request Approval checks the project's account again, and
	// the detail says when it no longer matches the template's.
	fresh, err := crs.CreateChangeRequest(ctx, req(&pa2))
	if err != nil {
		t.Fatal(err)
	}
	fid := fresh.ChangeRequest.ID
	if cr, _ := crs.GetChangeRequest(ctx, fid); cr.StandardChange == nil || cr.StandardChange.CustomerMismatch {
		t.Fatalf("a change request of the template's customer is a mismatch: %+v", cr.StandardChange)
	}
	sys := repository.WithSystemIdentity(context.Background())
	reparent := func(account string) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, `UPDATE project SET account_id = $2::uuid WHERE id = $1::uuid`, pa2, account); err != nil {
			t.Fatalf("re-parent project A2: %v", err)
		}
	}
	reparent(env.accounts["B"])
	t.Cleanup(func() { reparent(env.accounts["A"]) })
	for _, which := range []string{fid, id} {
		cr, err := crs.GetChangeRequest(ctx, which)
		if err != nil || cr.StandardChange == nil || !cr.StandardChange.CustomerMismatch {
			t.Fatalf("after the project moved to customer B, standardChange = %+v, %v", cr.StandardChange, err)
		}
	}
	_, err = crs.PatchChangeRequest(ctx, fid, domain.PatchChangeRequestRequest{State: &assess})
	wantCode(t, err, apierror.CodeStandardChangeTemplateOtherCustomer, "request approval after the project moved to customer B")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "approval cannot be requested") || !strings.Contains(ve.Msg, "now belongs to another customer") {
		t.Fatalf("refusal = %v", err)
	}
	if cr, _ := crs.GetChangeRequest(ctx, fid); cr.State == nil || *cr.State != string(domain.ChangeRequestStateNew) {
		t.Fatalf("a refused Request Approval moved the change to %v", cr.State)
	}
	// Moved back to the template's customer, it goes ahead.
	reparent(env.accounts["A"])
	if cr, _ := crs.GetChangeRequest(ctx, fid); cr.StandardChange == nil || cr.StandardChange.CustomerMismatch {
		t.Fatalf("back with customer A, standardChange = %+v", cr.StandardChange)
	}
	if _, err := crs.PatchChangeRequest(ctx, fid, domain.PatchChangeRequestRequest{State: &assess}); err != nil {
		t.Fatalf("request approval once the project is customer A's again: %v", err)
	}
}
