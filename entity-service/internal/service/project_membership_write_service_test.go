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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	writeProjectID    = "3f1e8d6a-3b4c-4d5e-8f90-123456789abc"
	writeProjectSfID  = "a0p000000000009AAA"
	writeAccountID    = "7c2e8d6a-3b4c-4d5e-8f90-123456789abc"
	writeAccountSfID  = "001000000000009AAA"
	writeContactSfID  = "003000000000009AAA"
	writeMembershipID = "a0e000000000009AAA"
	writeEmail        = "jane@acme.com"
)

// ---- fakes ---------------------------------------------------------------

// fakeWriteSalesEntity records every Salesforce call, so a test can assert
// that a create was NOT made when a search already found the record.
type fakeWriteSalesEntity struct {
	contact *salesentity.Contact
	// contactByID is what GetContact answers with when the caller resolves
	// the linked contact by its Salesforce id instead of by address. Left
	// nil it falls back to contact, so a test that does not care about the
	// distinction behaves as it did before the by-id lookup existed.
	contactByID   *salesentity.Contact
	getContactErr error
	// createdContactOmitsFlags models the contract POST /contacts actually
	// promises: an id, and not necessarily anything else.
	createdContactOmitsFlags bool
	membership               *salesentity.ProjectContact
	contactGets              []string
	contactSearchs           []string
	pcSearches               [][2]string
	createdContact           []salesentity.CreateContactInput
	createdPC                []salesentity.CreateProjectContactInput
	updates                  []writeUpdateCall
	searchErr                error
	createErr                error
	updateErr                error
}

type writeUpdateCall struct {
	id    string
	state *string
	roles *[]string
}

func (f *fakeWriteSalesEntity) GetContact(_ context.Context, id string) (salesentity.Contact, error) {
	f.contactGets = append(f.contactGets, id)
	if f.getContactErr != nil {
		return salesentity.Contact{}, f.getContactErr
	}
	if f.contactByID != nil {
		return *f.contactByID, nil
	}
	if f.contact != nil {
		return *f.contact, nil
	}
	return salesentity.Contact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: contact not found"}
}

func (f *fakeWriteSalesEntity) SearchContactByEmail(_ context.Context, email string) (salesentity.Contact, bool, error) {
	f.contactSearchs = append(f.contactSearchs, email)
	if f.searchErr != nil {
		return salesentity.Contact{}, false, f.searchErr
	}
	if f.contact == nil {
		return salesentity.Contact{}, false, nil
	}
	return *f.contact, true, nil
}

func (f *fakeWriteSalesEntity) CreateContact(_ context.Context, in salesentity.CreateContactInput) (salesentity.Contact, error) {
	f.createdContact = append(f.createdContact, in)
	if f.createErr != nil {
		return salesentity.Contact{}, f.createErr
	}
	// The real service echoes the created record back, including the
	// integration-user flag, which everything downstream then reads off the
	// contact rather than off the request.
	if f.createdContactOmitsFlags {
		c := salesentity.Contact{ID: sampleStr(writeContactSfID), Email: sampleStr(in.Email)}
		f.contact = &c
		return c, nil
	}
	isIntegration := in.IsCsIntegrationUser
	c := salesentity.Contact{
		ID: sampleStr(writeContactSfID), Email: sampleStr(in.Email),
		FirstName: sampleStr(in.FirstName), LastName: sampleStr(in.LastName),
		Account:             &salesentity.ContactAccount{ID: sampleStr(in.AccountID)},
		IsCsIntegrationUser: &isIntegration,
	}
	f.contact = &c
	return c, nil
}

func (f *fakeWriteSalesEntity) SearchProjectContact(_ context.Context, projectSfID, contactSfID string) (salesentity.ProjectContact, bool, error) {
	f.pcSearches = append(f.pcSearches, [2]string{projectSfID, contactSfID})
	if f.searchErr != nil {
		return salesentity.ProjectContact{}, false, f.searchErr
	}
	if f.membership == nil {
		return salesentity.ProjectContact{}, false, nil
	}
	return *f.membership, true, nil
}

func (f *fakeWriteSalesEntity) CreateProjectContact(_ context.Context, in salesentity.CreateProjectContactInput) (salesentity.ProjectContact, error) {
	f.createdPC = append(f.createdPC, in)
	if f.createErr != nil {
		return salesentity.ProjectContact{}, f.createErr
	}
	pc := salesentity.ProjectContact{ID: writeMembershipID, Email: writeEmail, State: sampleStr(in.State), Roles: in.Role}
	f.membership = &pc
	return pc, nil
}

func (f *fakeWriteSalesEntity) UpdateProjectContact(_ context.Context, id string, state *string, roles *[]string) (salesentity.ProjectContact, error) {
	f.updates = append(f.updates, writeUpdateCall{id: id, state: state, roles: roles})
	if f.updateErr != nil {
		return salesentity.ProjectContact{}, f.updateErr
	}
	// The PATCH answers 200 with an EMPTY body when its own re-read failed:
	// a zero record and no error. Modelled here so the caller is exercised
	// against the harder of the two shapes.
	return salesentity.ProjectContact{}, nil
}

// fakeWriteMembershipRepo runs the plan the way the real repository does --
// resolve the target and any existing membership, call the plan, then "write"
// the rows -- so a test can assert that a Salesforce failure wrote nothing.
type fakeWriteMembershipRepo struct {
	existing  *domain.ProjectMembershipRow
	target    domain.MembershipWriteTarget
	targetErr error
	commitErr bool
	upserts   []domain.SalesforceMembershipUpsert
	steps     []domain.UpsertOnboardingStepRequest
	getErr    error
}

func (f *fakeWriteMembershipRepo) Upsert(context.Context, domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error) {
	return domain.SalesforceMembershipUpsertResult{}, errors.New("the portal writes use UpsertWithin")
}

func (f *fakeWriteMembershipRepo) DeactivateBySfID(context.Context, string) (bool, error) {
	return false, errors.New("not used by the portal writes")
}

func (f *fakeWriteMembershipRepo) UpsertWithin(ctx context.Context, _, _ string, plan repository.MembershipWritePlan) (domain.SalesforceMembershipUpsertResult, error) {
	if f.targetErr != nil {
		return domain.SalesforceMembershipUpsertResult{}, f.targetErr
	}
	in, step, err := plan(ctx, repository.MembershipWriteContext{Target: f.target, Existing: f.existing})
	if err != nil {
		// The transaction rolls back: no rows are written.
		return domain.SalesforceMembershipUpsertResult{}, err
	}
	f.upserts = append(f.upserts, in)
	f.steps = append(f.steps, step)
	res := domain.SalesforceMembershipUpsertResult{
		ProjectID:             writeProjectID,
		AccountID:             writeAccountID,
		UserID:                "9a2e8d6a-3b4c-4d5e-8f90-123456789abc",
		ProjectContactID:      "5b3e8d6a-3b4c-4d5e-8f90-123456789abc",
		CreatedProjectContact: f.existing == nil,
	}
	if f.commitErr {
		return res, fmt.Errorf("%w: connection reset", repository.ErrMembershipCommitFailed)
	}
	return res, nil
}

func (f *fakeWriteMembershipRepo) GetMembershipByEmail(context.Context, string, string) (domain.ProjectMembershipRow, error) {
	if f.getErr != nil {
		return domain.ProjectMembershipRow{}, f.getErr
	}
	if f.existing == nil {
		return domain.ProjectMembershipRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
	}
	return *f.existing, nil
}

type fakeFailureRecorder struct {
	recorded []domain.CreateEventPublishFailureRequest
}

func (f *fakeFailureRecorder) CreateEventPublishFailure(_ context.Context, req domain.CreateEventPublishFailureRequest) (domain.EventPublishFailure, error) {
	f.recorded = append(f.recorded, req)
	return domain.EventPublishFailure{}, nil
}

func (f *fakeFailureRecorder) ResolveEventPublishFailure(context.Context, string) (domain.EventPublishFailure, error) {
	return domain.EventPublishFailure{}, nil
}

func (f *fakeFailureRecorder) SearchEventPublishFailures(context.Context, domain.SearchEventPublishFailuresRequest) (domain.SearchEventPublishFailuresResponse, error) {
	return domain.SearchEventPublishFailuresResponse{}, nil
}

// ---- harness -------------------------------------------------------------

type writeHarness struct {
	se       *fakeWriteSalesEntity
	repo     *fakeWriteMembershipRepo
	steps    *fakeStepRepo
	pub      *fakeInvitePublisher
	failures *fakeFailureRecorder
	svc      ProjectMembershipWriteService
}

func newWriteHarness(t *testing.T, access AccessService) *writeHarness {
	t.Helper()
	h := &writeHarness{
		se: &fakeWriteSalesEntity{},
		repo: &fakeWriteMembershipRepo{target: domain.MembershipWriteTarget{
			ProjectID:   writeProjectID,
			ProjectKey:  "ACMEPROD",
			ProjectName: "Acme Prod",
			ProjectSfID: writeProjectSfID,
			AccountID:   writeAccountID,
			AccountSfID: writeAccountSfID,
		}},
		steps:    &fakeStepRepo{},
		pub:      &fakeInvitePublisher{},
		failures: &fakeFailureRecorder{},
	}
	h.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
		Memberships: h.repo,
		Steps:       h.steps,
		SalesEntity: h.se,
		Publisher:   h.pub,
		Failures:    h.failures,
		Access:      access,
	})
	return h
}

func newInternalWriteHarness(t *testing.T) *writeHarness {
	t.Helper()
	return newWriteHarness(t, alwaysUnrestrictedAccess{})
}

func inviteReq(roles ...string) domain.CreateProjectMembershipRequest {
	return domain.CreateProjectMembershipRequest{Email: " Jane@Acme.com ", FirstName: "Jane", LastName: "Doe", Roles: roles}
}

func existingSalesforceContact() *salesentity.Contact {
	return &salesentity.Contact{
		ID: sampleStr(writeContactSfID), Email: sampleStr(writeEmail),
		FirstName: sampleStr("Jane"), LastName: sampleStr("Doe"),
		IsCsAdmin: boolPtr(false), IsCsIntegrationUser: boolPtr(false),
		Account: &salesentity.ContactAccount{ID: sampleStr(writeAccountSfID)},
	}
}

// ---- authorization -------------------------------------------------------

// TestMembershipWrite_RejectsNonInternalCallers pins that every one of the
// four operations is refused for a caller that is not an allow-listed
// internal service, BEFORE anything downstream is touched -- no Salesforce
// call, no transaction, no event.
func TestMembershipWrite_RejectsNonInternalCallers(t *testing.T) {
	h := newWriteHarness(t, stubAccess{scope: AccessScope{ProjectIDs: []string{writeProjectID}}})
	ctx := context.Background()

	_, inviteErr := h.svc.Invite(ctx, writeProjectID, inviteReq("Portal user"))
	_, rolesErr := h.svc.UpdateRoles(ctx, writeProjectID, writeEmail, domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Admin"}})
	deactivateErr := h.svc.Deactivate(ctx, writeProjectID, writeEmail)
	resendErr := h.svc.ResendInvitation(ctx, writeProjectID, writeEmail)

	for name, err := range map[string]error{
		"Invite": inviteErr, "UpdateRoles": rolesErr, "Deactivate": deactivateErr, "ResendInvitation": resendErr,
	} {
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Errorf("%s: err = %v, want ForbiddenError", name, err)
		}
	}
	if len(h.se.contactSearchs)+len(h.se.createdContact)+len(h.se.createdPC)+len(h.se.updates) != 0 {
		t.Error("Salesforce must not be touched for a caller that is refused")
	}
	if len(h.repo.upserts) != 0 || len(h.pub.published) != 0 {
		t.Error("nothing may be written or published for a caller that is refused")
	}
}

// ---- invite --------------------------------------------------------------

func TestMembershipWrite_InviteCreatesBothSides(t *testing.T) {
	h := newInternalWriteHarness(t)
	got, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("portal user", "Admin"))
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}

	if len(h.se.createdContact) != 1 || h.se.createdContact[0].Email != writeEmail || h.se.createdContact[0].AccountID != writeAccountSfID {
		t.Errorf("contact create = %+v", h.se.createdContact)
	}
	if len(h.se.createdPC) != 1 {
		t.Fatalf("membership creates = %d, want 1", len(h.se.createdPC))
	}
	created := h.se.createdPC[0]
	if created.ProjectID != writeProjectSfID || created.ContactID != writeContactSfID || created.State != domain.MembershipStateInvited {
		t.Errorf("membership create = %+v", created)
	}
	// Roles are canonicalised to Salesforce's own picklist spelling.
	if !reflect.DeepEqual(created.Role, []string{"Portal user", "Admin"}) {
		t.Errorf("roles = %v", created.Role)
	}

	if len(h.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(h.repo.upserts))
	}
	in := h.repo.upserts[0]
	if in.ProjectID != writeProjectID || in.Email != writeEmail || in.State != domain.MembershipStateInvited ||
		in.MembershipSfID != writeMembershipID || in.ContactSfID != writeContactSfID ||
		in.Actor != domain.PortalMembershipWriteActor {
		t.Errorf("upsert = %+v", in)
	}
	// Admin is a project role now: it joins the Admin project group.
	if !reflect.DeepEqual(in.ProjectGroups, []string{projectGroupGeneralAccess, projectGroupAdmin}) {
		t.Errorf("project groups = %v", in.ProjectGroups)
	}
	if in.AdminRoleName != globalRoleCustomerAdmin {
		t.Errorf("adminRoleName = %q", in.AdminRoleName)
	}
	if step := h.repo.steps[0]; step.Step != domain.OnboardingStepDatabase || step.Status != domain.OnboardingStepSucceeded ||
		step.EventType != domain.PortalMembershipWriteEventType || step.Email != writeEmail {
		t.Errorf("step = %+v", step)
	}

	if got.State != domain.MembershipStateInvited || got.Email != writeEmail || got.MembershipSfID != writeMembershipID {
		t.Errorf("response = %+v", got)
	}
	// The portal write is the origin of this invitation, so IT publishes;
	// the Salesforce echo that follows is suppressed by the ingest.
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.pub.published))
	}
	var payload events.ProjectContactInvitedPayload
	if err := json.Unmarshal(h.pub.published[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Email != writeEmail || payload.ProjectKey != "ACMEPROD" || payload.ProjectName != "Acme Prod" || payload.Resend {
		t.Errorf("payload = %+v", payload)
	}
}

// TestMembershipWrite_InviteAdoptsExistingSalesforceContact is the
// search-before-create rule: a contact Salesforce already knows -- whether
// from an earlier orphaned write, a hand edit, or another project -- is
// adopted, never duplicated.
func TestMembershipWrite_InviteAdoptsExistingSalesforceContact(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.se.contact = existingSalesforceContact()

	if _, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if len(h.se.createdContact) != 0 {
		t.Errorf("an existing Salesforce contact must be adopted, not duplicated: %+v", h.se.createdContact)
	}
	if len(h.se.contactSearchs) != 1 || h.se.contactSearchs[0] != writeEmail {
		t.Errorf("the contact must be searched for by address first: %v", h.se.contactSearchs)
	}
	if h.repo.upserts[0].ContactSfID != writeContactSfID {
		t.Errorf("the adopted contact's id must be written: %+v", h.repo.upserts[0])
	}
}

// TestMembershipWrite_InviteAdoptsExistingSalesforceMembership is the same
// rule one level down: a Salesforce membership for this (project, contact) is
// PATCHed, never duplicated with a second Project_Contact__c. This is exactly
// the residue a commit that failed after Salesforce succeeded leaves behind.
func TestMembershipWrite_InviteAdoptsExistingSalesforceMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Email: writeEmail, State: sampleStr("INVITED")}

	if _, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user", "Lead")); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if len(h.se.createdPC) != 0 {
		t.Errorf("an existing membership must be updated, not duplicated: %+v", h.se.createdPC)
	}
	if len(h.se.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(h.se.updates))
	}
	u := h.se.updates[0]
	if u.id != writeMembershipID || u.state == nil || *u.state != domain.MembershipStateInvited ||
		u.roles == nil || !reflect.DeepEqual(*u.roles, []string{"Portal user", "Lead"}) {
		t.Errorf("update = %+v state=%v roles=%v", u, u.state, u.roles)
	}
	// The PATCH answered with an empty body; the id we already held is still
	// the right one, and the row is written with it.
	if h.repo.upserts[0].MembershipSfID != writeMembershipID {
		t.Errorf("membership id = %q", h.repo.upserts[0].MembershipSfID)
	}
}

// TestMembershipWrite_SalesforceFailureRollsBack is the core property: if the
// Salesforce half fails, the transaction rolls back, nothing is written to
// either system, nothing is published, and the caller gets the error.
func TestMembershipWrite_SalesforceFailureRollsBack(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*writeHarness)
	}{
		{"the contact search fails", func(h *writeHarness) {
			h.se.searchErr = &apierror.ServiceUnavailableError{Msg: "salesentity: contacts/search returned 502"}
		}},
		{"the contact create fails", func(h *writeHarness) {
			h.se.createErr = &apierror.DownstreamError{Msg: "salesforce rejected the contact"}
		}},
		{"the membership create fails", func(h *writeHarness) {
			h.se.contact = existingSalesforceContact()
			h.se.createErr = &apierror.ServiceUnavailableError{Msg: "salesentity: project-contacts returned 503"}
		}},
		{"the membership update fails", func(h *writeHarness) {
			h.se.contact = existingSalesforceContact()
			h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}
			h.se.updateErr = &apierror.ServiceUnavailableError{Msg: "salesentity: project-contacts PATCH returned 503"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			tc.setup(h)

			_, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
			if err == nil {
				t.Fatal("want an error")
			}
			if len(h.repo.upserts) != 0 {
				t.Errorf("no row may be written when Salesforce failed: %+v", h.repo.upserts)
			}
			if len(h.pub.published) != 0 {
				t.Error("no event may be published when Salesforce failed")
			}
			if len(h.failures.recorded) != 0 {
				t.Error("nothing is orphaned when Salesforce failed before the commit")
			}
		})
	}
}

// TestMembershipWrite_CommitFailureRecordsTheOrphan covers the one residue
// this ordering cannot roll back: Salesforce succeeded and the commit did
// not. It is recorded so it can be retried, and the caller is told.
func TestMembershipWrite_CommitFailureRecordsTheOrphan(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.commitErr = true

	_, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if len(h.failures.recorded) != 1 {
		t.Fatalf("recorded = %d, want 1", len(h.failures.recorded))
	}
	rec := h.failures.recorded[0]
	if rec.EventType != salesforceMembershipWriteFailureEvent || rec.EntityID != writeMembershipID || rec.Error == "" {
		t.Errorf("recorded = %+v", rec)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["operation"] != "invite" || payload["email"] != writeEmail || payload["membershipSfId"] != writeMembershipID {
		t.Errorf("payload = %v", payload)
	}
	if len(h.pub.published) != 0 {
		t.Error("an invitation whose transaction did not commit must not send an e-mail")
	}
}

func TestMembershipWrite_InviteRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		req  domain.CreateProjectMembershipRequest
	}{
		{"no email", domain.CreateProjectMembershipRequest{Roles: []string{"Portal user"}}},
		{"malformed email", domain.CreateProjectMembershipRequest{Email: "not-an-address", Roles: []string{"Portal user"}}},
		{"no roles", domain.CreateProjectMembershipRequest{Email: writeEmail}},
		{"unknown role", domain.CreateProjectMembershipRequest{Email: writeEmail, Roles: []string{"Billing Contact"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			_, err := h.svc.Invite(context.Background(), writeProjectID, tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if len(h.se.contactSearchs) != 0 {
				t.Error("input is rejected before Salesforce is called")
			}
		})
	}
}

// TestMembershipWrite_InviteConflictsWithAnActiveMembership: re-inviting
// somebody who is already on the project is a 409; a DEACTIVATED membership
// is instead brought back as a RE-INVITED one.
func TestMembershipWrite_InviteConflictsWithAnActiveMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{ProjectContactID: "pc-1", Email: writeEmail, State: domain.MembershipStateRegistered}

	_, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
	if len(h.se.contactSearchs) != 0 || len(h.repo.upserts) != 0 {
		t.Error("a conflicting invitation must not reach Salesforce or the rows")
	}
}

func TestMembershipWrite_InviteReactivatesADeactivatedMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateDeactivated,
	}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	got, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
	if err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if got.State != domain.MembershipStateReInvited || h.repo.upserts[0].State != domain.MembershipStateReInvited {
		t.Errorf("state = %q / %q, want RE-INVITED", got.State, h.repo.upserts[0].State)
	}
	if len(h.pub.published) != 1 {
		t.Error("a re-invitation still sends the invitation e-mail")
	}
}

// ---- role change ---------------------------------------------------------

func TestMembershipWrite_UpdateRolesReplacesThem(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateRegistered,
		ProjectGroups: []string{projectGroupGeneralAccess, projectGroupAdmin},
	}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	got, err := h.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail,
		domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Portal user"}})
	if err != nil {
		t.Fatalf("UpdateRoles: %v", err)
	}
	if len(h.se.createdPC) != 0 {
		t.Error("a role change must update the existing membership, not create a second one")
	}
	u := h.se.updates[0]
	if u.state != nil {
		t.Error("a role change must not restate the membership's state")
	}
	if u.roles == nil || !reflect.DeepEqual(*u.roles, []string{"Portal user"}) {
		t.Errorf("roles = %v", u.roles)
	}
	// Admin was dropped, so the Admin project group goes with it -- the
	// account-level role is then re-derived from the user's other memberships.
	if !reflect.DeepEqual(h.repo.upserts[0].ProjectGroups, []string{projectGroupGeneralAccess}) {
		t.Errorf("groups = %v", h.repo.upserts[0].ProjectGroups)
	}
	if h.repo.upserts[0].State != domain.MembershipStateRegistered || got.State != domain.MembershipStateRegistered {
		t.Errorf("state must be untouched: %q", got.State)
	}
	if len(h.pub.published) != 0 {
		t.Error("a role change sends no invitation")
	}
}

func TestMembershipWrite_UpdateRolesRequiresAnExistingMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	_, err := h.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail,
		domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Portal user"}})
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
	if len(h.se.contactSearchs) != 0 {
		t.Error("an unknown contact must not reach Salesforce")
	}
}

// ---- deactivate ----------------------------------------------------------

func TestMembershipWrite_DeactivateSetsBothSides(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateRegistered,
		ProjectGroups: []string{projectGroupFullAccess, projectGroupAdmin},
	}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	if err := h.svc.Deactivate(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if len(h.se.updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(h.se.updates))
	}
	u := h.se.updates[0]
	if u.state == nil || *u.state != domain.MembershipStateDeactivated {
		t.Errorf("Salesforce state = %v, want DEACTIVATED", u.state)
	}
	if u.roles != nil {
		t.Error("deactivating must not rewrite the roles")
	}
	if len(h.repo.upserts) != 1 || h.repo.upserts[0].State != domain.MembershipStateDeactivated {
		t.Errorf("row state = %+v, want DEACTIVATED", h.repo.upserts)
	}
	// The groups are kept as they are; the derived admin role ignores a
	// deactivated membership, so nothing has to be erased to drop it.
	if !reflect.DeepEqual(h.repo.upserts[0].ProjectGroups, []string{projectGroupFullAccess, projectGroupAdmin}) {
		t.Errorf("groups = %v", h.repo.upserts[0].ProjectGroups)
	}
	if len(h.pub.published) != 0 {
		t.Error("deactivating sends no invitation")
	}
}

// TestMembershipWrite_DeactivateUsesTheLinkedContactIdNotTheInvitedAddress
// is the data-integrity rule for a membership we already hold ids for. The
// linked Salesforce Contact's own Email is a different field from the address
// the membership was invited under and does drift apart on real rows, so
// resolving by address can miss the real Contact -- and a miss used to mean
// a brand new Contact plus a brand new Project_Contact__c in state
// DEACTIVATED, while the membership that actually matters stayed active.
func TestMembershipWrite_DeactivateUsesTheLinkedContactIdNotTheInvitedAddress(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateRegistered,
		ProjectGroups: []string{projectGroupFullAccess},
	}
	// The Contact record carries a DIFFERENT address, so the by-address
	// search finds nothing at all.
	linked := existingSalesforceContact()
	linked.Email = sampleStr("jane.doe@acme-corp.example")
	h.se.contactByID = linked
	h.se.contact = nil
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	if err := h.svc.Deactivate(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if !reflect.DeepEqual(h.se.contactGets, []string{writeContactSfID}) {
		t.Errorf("GetContact calls = %v, want the linked contact id", h.se.contactGets)
	}
	if len(h.se.contactSearchs) != 0 {
		t.Errorf("the address search must not run when the id resolved: %v", h.se.contactSearchs)
	}
	if len(h.se.createdContact) != 0 || len(h.se.createdPC) != 0 {
		t.Fatalf("nothing may be created: contacts=%v memberships=%v", h.se.createdContact, h.se.createdPC)
	}
	if len(h.se.updates) != 1 || h.se.updates[0].id != writeMembershipID {
		t.Errorf("updates = %+v, want a PATCH of the real membership", h.se.updates)
	}
}

// TestMembershipWrite_FallsBackToTheAddressSearchWhenTheLinkedIdIsStale keeps
// every self-healing path the by-id lookup was added in front of: an id that
// no longer resolves must not fail the write.
func TestMembershipWrite_FallsBackToTheAddressSearchWhenTheLinkedIdIsStale(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateRegistered,
		ProjectGroups: []string{projectGroupFullAccess},
	}
	h.se.getContactErr = &apierror.ServiceUnavailableError{Msg: "salesentity: contact not found"}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	if err := h.svc.Deactivate(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if len(h.se.contactSearchs) != 1 {
		t.Errorf("contact searches = %v, want the address search to have run", h.se.contactSearchs)
	}
	if len(h.se.updates) != 1 {
		t.Errorf("updates = %+v, want the write to have gone through anyway", h.se.updates)
	}
}

// TestMembershipWrite_CreatedContactKeepsTheRequestedIntegrationFlag covers a
// POST /contacts response that carries only the id: the flag we ASKED for is
// then the only evidence there is, and reading the missing field as false
// would give a machine account global roles and an invitation e-mail.
func TestMembershipWrite_CreatedContactKeepsTheRequestedIntegrationFlag(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.se.createdContactOmitsFlags = true

	req := inviteReq("Portal user")
	req.IsCsIntegrationUser = true
	if _, err := h.svc.Invite(context.Background(), writeProjectID, req); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Fatalf("upserts = %d", len(h.repo.upserts))
	}
	in := h.repo.upserts[0]
	if !in.IsCsIntegrationUser {
		t.Error("an integration user must stay one when the create response omits the flag")
	}
	if len(in.GlobalRoles) != 0 || in.AdminRoleName != "" {
		t.Errorf("an integration user gets no global roles: roles=%v adminRole=%q", in.GlobalRoles, in.AdminRoleName)
	}
	// The event is still published -- csm-notification-service is what acts
	// on the flag (no Asgardeo identity, no e-mail) -- but it must carry the
	// flag, not a silent false.
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.pub.published))
	}
	var payload events.ProjectContactInvitedPayload
	if err := json.Unmarshal(h.pub.published[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.IsIntegrationUser {
		t.Error("the invitation event must tell the consumer this is a machine account")
	}
}

func TestMembershipWrite_DeactivateRequiresAnExistingMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	err := h.svc.Deactivate(context.Background(), writeProjectID, writeEmail)
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

// ---- resend --------------------------------------------------------------

func invitedMembershipRow() *domain.ProjectMembershipRow {
	return &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateInvited,
		ProjectGroups: []string{projectGroupGeneralAccess, projectGroupAdmin},
		FirstName:     "Jane", LastName: "Doe",
		ProjectKey: "ACMEPROD", ProjectName: "Acme Prod", Type: domain.MembershipTypeOwnContact,
	}
}

func TestMembershipWrite_ResendPublishesWithTheResendMarker(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = invitedMembershipRow()
	// An EMAIL step older than the cooldown does not block the resend.
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: writeMembershipID, Step: domain.OnboardingStepEmail,
		Status: domain.OnboardingStepSucceeded, UpdatedOn: time.Now().Add(-30 * time.Minute),
	}}

	if err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("ResendInvitation: %v", err)
	}
	if len(h.repo.upserts) != 0 || len(h.se.updates) != 0 || len(h.se.createdPC) != 0 {
		t.Error("a resend writes nothing to either system")
	}
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.pub.published))
	}
	var payload events.ProjectContactInvitedPayload
	if err := json.Unmarshal(h.pub.published[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Resend {
		t.Error("the resend marker is what makes csm-notification-service bypass its already-sent guard")
	}
	if payload.Email != writeEmail || payload.GivenName != "Jane" || payload.ProjectKey != "ACMEPROD" {
		t.Errorf("payload = %+v", payload)
	}
	if !reflect.DeepEqual(payload.Roles, []string{"Portal user", "Admin"}) {
		t.Errorf("roles = %v, want the Salesforce labels the stored groups represent", payload.Roles)
	}
}

func TestMembershipWrite_ResendOutsideAnOutstandingInvitationIsAConflict(t *testing.T) {
	for _, state := range []string{domain.MembershipStateRegistered, domain.MembershipStateDeactivated} {
		t.Run(state, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			row := invitedMembershipRow()
			row.State = state
			h.repo.existing = row

			err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail)
			var ce *apierror.ConflictError
			if !errors.As(err, &ce) {
				t.Fatalf("err = %v, want ConflictError", err)
			}
			if len(h.pub.published) != 0 {
				t.Error("nothing may be published once the invitation is no longer outstanding")
			}
		})
	}
}

// TestMembershipWrite_ResendIsAllowedForAReInvitedMembership pins that
// RE-INVITED is an OUTSTANDING invitation, not an already-re-sent one. It is
// the state Invite writes when it brings a deactivated contact back, so a
// person left in it by a notification service that was down has no other way
// to be sent their invitation.
func TestMembershipWrite_ResendIsAllowedForAReInvitedMembership(t *testing.T) {
	h := newInternalWriteHarness(t)
	row := invitedMembershipRow()
	row.State = domain.MembershipStateReInvited
	h.repo.existing = row

	if err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("ResendInvitation: %v", err)
	}
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.pub.published))
	}
	var payload events.ProjectContactInvitedPayload
	if err := json.Unmarshal(h.pub.published[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Resend {
		t.Error("a resend must carry the resend marker whatever the outstanding state")
	}
}

func TestMembershipWrite_ResendInsideTheCooldownIsRateLimited(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = invitedMembershipRow()
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: writeMembershipID, Step: domain.OnboardingStepEmail,
		Status: domain.OnboardingStepSucceeded, UpdatedOn: time.Now().Add(-1 * time.Minute),
	}}

	err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail)
	var tme *apierror.TooManyRequestsError
	if !errors.As(err, &tme) {
		t.Fatalf("err = %v, want TooManyRequestsError", err)
	}
	if len(h.pub.published) != 0 {
		t.Error("nothing may be published inside the cooldown")
	}
}

func TestMembershipWrite_ResendWithNoEmailStepYetIsAllowed(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = invitedMembershipRow()
	// Only a DATABASE step: no invitation has been sent, so there is
	// nothing to wait for.
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: writeMembershipID, Step: domain.OnboardingStepDatabase,
		Status: domain.OnboardingStepSucceeded, UpdatedOn: time.Now(),
	}}
	if err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail); err != nil {
		t.Fatalf("ResendInvitation: %v", err)
	}
	if len(h.pub.published) != 1 {
		t.Error("the first send is never rate-limited")
	}
}

func TestMembershipWrite_ResendWithoutAPublisherIs503(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = invitedMembershipRow()
	h.svc = NewProjectMembershipWriteService(MembershipWriteDeps{
		Memberships: h.repo, Steps: h.steps, SalesEntity: h.se, Access: alwaysUnrestrictedAccess{},
	})
	err := h.svc.ResendInvitation(context.Background(), writeProjectID, writeEmail)
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError -- a resend IS the event", err)
	}
}

// ---- helpers -------------------------------------------------------------

func TestCanonicalSalesforceRoles(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
		bad  bool
	}{
		{"canonicalises spelling", []string{"portal user", " ADMIN "}, []string{"Portal user", "Admin"}, false},
		{"drops blanks and duplicates", []string{"Admin", "", "admin"}, []string{"Admin"}, false},
		{"empty stays empty", nil, []string{}, false},
		{"rejects an unknown role", []string{"Billing Contact"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalSalesforceRoles(tc.in)
			if tc.bad {
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSalesforceRolesForGroups(t *testing.T) {
	cases := []struct {
		groups []string
		want   []string
	}{
		{[]string{projectGroupFullAccess}, []string{"Portal user", "Security Contact"}},
		{[]string{projectGroupGeneralAccess, projectGroupAdmin}, []string{"Portal user", "Admin"}},
		{[]string{projectGroupSecurityOnly, projectGroupLeadUserGroup}, []string{"Security Contact", "Lead"}},
		{nil, []string{}},
	}
	for _, tc := range cases {
		if got := salesforceRolesForGroups(tc.groups); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("salesforceRolesForGroups(%v) = %v, want %v", tc.groups, got, tc.want)
		}
	}
}

func TestNormalizeMembershipEmail(t *testing.T) {
	got, err := normalizeMembershipEmail("  Jane@Acme.COM ")
	if err != nil || got != writeEmail {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", "no-at-sign", "@acme.com", "jane@", "jane doe@acme.com"} {
		if _, err := normalizeMembershipEmail(bad); err == nil {
			t.Errorf("normalizeMembershipEmail(%q) should be rejected", bad)
		}
	}
}

// TestInvite_IntegrationUserReachesSalesforceAndTheEvent pins the machine-account
// path end to end through this service: the flag on the request reaches the
// Salesforce contact create, and comes back out on the published invitation so
// csm-notification-service knows to skip the identity and the e-mail. Without
// it a machine account would be sent an invitation nobody reads and given an
// Asgardeo login nobody uses.
func TestInvite_IntegrationUserReachesSalesforceAndTheEvent(t *testing.T) {
	for _, integration := range []bool{true, false} {
		name := "regular contact"
		if integration {
			name = "integration user"
		}
		t.Run(name, func(t *testing.T) {
			h := newWriteHarness(t, alwaysUnrestrictedAccess{})
			_, err := h.svc.Invite(context.Background(), writeProjectID, domain.CreateProjectMembershipRequest{
				Email:               "svc-account@acme.com",
				LastName:            "Service Account",
				Roles:               []string{"Portal user"},
				IsCsIntegrationUser: integration,
			})
			if err != nil {
				t.Fatalf("Invite() error = %v", err)
			}
			if len(h.se.createdContact) != 1 {
				t.Fatalf("created %d contacts, want 1", len(h.se.createdContact))
			}
			if got := h.se.createdContact[0].IsCsIntegrationUser; got != integration {
				t.Errorf("contact create isCsIntegrationUser = %v, want %v", got, integration)
			}
			if len(h.repo.upserts) != 1 {
				t.Fatalf("upserts = %d, want 1", len(h.repo.upserts))
			}
			if got := h.repo.upserts[0].IsCsIntegrationUser; got != integration {
				t.Errorf("membership upsert isCsIntegrationUser = %v, want %v", got, integration)
			}
		})
	}
}
