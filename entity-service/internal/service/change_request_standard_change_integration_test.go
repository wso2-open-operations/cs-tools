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
			crs := WithStandardChangeTemplates(
				NewChangeRequestService(repository.NewChangeRequestRepository(repository.NewScoped(pool)), repository.NewUserRepository(pool)),
				repository.NewStandardChangeRepository(repository.NewScoped(pool)))
			stdChangeRequestFlow(t, env, crs)
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
	raise := func(extra func(*domain.CreateChangeRequestRequest)) (domain.CreateChangeRequestResponse, error) {
		req := domain.CreateChangeRequestRequest{Type: &standard, GroupID: &group, StandardChangeTemplateID: &tpl.ID, StandardChangeParameters: params}
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
	_, err = raise(func(r *domain.CreateChangeRequestRequest) { r.StandardChangeParameters = map[string]string{"env": "prod"} })
	wantStdValidation(t, err, "Cluster (cluster) is required")
	_, err = raise(func(r *domain.CreateChangeRequestRequest) { r.StandardChangeParameters = map[string]string{"cluster": "a", "env": "qa"} })
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
	if _, err := env.svc.SuspendTemplate(stdCtx(stdCAB1), tpl.ID); err != nil {
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
