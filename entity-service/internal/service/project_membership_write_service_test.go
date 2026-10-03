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
	// savedState, when set, is the state Salesforce's automation stores (and
	// the create/PATCH re-read returns) instead of the one requested.
	savedState string
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
	state := in.State
	if f.savedState != "" {
		state = f.savedState
	}
	pc := salesentity.ProjectContact{ID: writeMembershipID, Email: writeEmail, State: sampleStr(state), Roles: in.Role}
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
	if f.savedState != "" {
		return salesentity.ProjectContact{ID: id, State: sampleStr(f.savedState)}, nil
	}
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
	resolves  int
}

func (f *fakeWriteMembershipRepo) Upsert(context.Context, domain.SalesforceMembershipUpsert, domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error) {
	return domain.SalesforceMembershipUpsertResult{}, errors.New("the portal writes use UpsertWithin")
}

func (f *fakeWriteMembershipRepo) DeactivateBySfID(context.Context, string, repository.AdminRoleBasisFunc) (bool, error) {
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

func (f *fakeWriteMembershipRepo) ResolveWriteContext(context.Context, string, string) (repository.MembershipWriteContext, error) {
	f.resolves++
	if f.targetErr != nil {
		return repository.MembershipWriteContext{}, f.targetErr
	}
	return repository.MembershipWriteContext{Target: f.target, Existing: f.existing}, nil
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
	// The event carries the version the DATABASE step was stamped with, so
	// csm-notification-service can tell a later re-invitation (newer) from a
	// duplicate of this one (same or older).
	if want := h.repo.steps[0].EventModifiedOn.UTC().Format(time.RFC3339Nano); h.repo.steps[0].EventModifiedOn.IsZero() || payload.EventModifiedOn != want {
		t.Errorf("payload eventModifiedOn = %q, want the DATABASE step's %q", payload.EventModifiedOn, want)
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
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Email: writeEmail, State: sampleStr("INVITED"), Roles: []string{"Portal user"}}

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
		name                string
		req                 domain.CreateProjectMembershipRequest
		wantNoSalesforceHit bool // false for "no roles": see its own comment below
	}{
		{"no email", domain.CreateProjectMembershipRequest{Roles: []string{"Portal user"}}, true},
		{"malformed email", domain.CreateProjectMembershipRequest{Email: "not-an-address", Roles: []string{"Portal user"}}, true},
		{"unknown role", domain.CreateProjectMembershipRequest{Email: writeEmail, Roles: []string{"Billing Contact"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			_, err := h.svc.Invite(context.Background(), writeProjectID, tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if tc.wantNoSalesforceHit && len(h.se.contactSearchs) != 0 {
				t.Error("input is rejected before Salesforce is called")
			}
		})
	}
}

// TestMembershipWrite_InviteRejectsNoRolesAfterResolvingTheContact covers the
// "no roles" case on its own, since its behavior genuinely differs from the
// other bad-input cases above: the zero-roles check can only be applied
// after Salesforce's REAL contact classification is known (see
// salesforceWriteIntent.ValidateContactType's own doc comment) -- a caller
// cannot bypass it by merely claiming IsCsIntegrationUser on an existing,
// real contact. So unlike a malformed email or an unknown role, this
// request DOES reach Salesforce (a search, and a create for a
// not-yet-existing address) before being refused, and still writes no
// project membership.
func TestMembershipWrite_InviteRejectsNoRolesAfterResolvingTheContact(t *testing.T) {
	h := newInternalWriteHarness(t)
	_, err := h.svc.Invite(context.Background(), writeProjectID, domain.CreateProjectMembershipRequest{Email: writeEmail})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if len(h.se.contactSearchs) != 1 {
		t.Errorf("contact searches = %d, want 1 (the contact must be resolved before the roles check runs)", len(h.se.contactSearchs))
	}
	if len(h.se.pcSearches) != 0 {
		t.Error("no project-membership search/write may happen once the roles check refuses the request")
	}
}

// TestMembershipWrite_InviteIgnoresTheClaimedIntegrationFlagForAnExistingContact
// is the regression test for the CodeRabbit finding this fix addresses: a
// caller cannot invite an EXISTING, real (non-integration) Salesforce
// contact with zero roles just by setting IsCsIntegrationUser: true in the
// request -- Salesforce's own stored classification must win, exactly as
// CreateProjectMembershipRequest.IsCsIntegrationUser's own doc comment
// already promises for every other effect of that flag.
func TestMembershipWrite_InviteIgnoresTheClaimedIntegrationFlagForAnExistingContact(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.se.contact = &salesentity.Contact{
		ID: sampleStr(writeContactSfID), Email: sampleStr(writeEmail),
		IsCsIntegrationUser: boolPtr(false), // Salesforce says: a real human contact
	}

	_, err := h.svc.Invite(context.Background(), writeProjectID, domain.CreateProjectMembershipRequest{
		Email:               writeEmail,
		IsCsIntegrationUser: true, // the caller's claim -- must not override Salesforce's own record
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError (an existing non-integration contact still needs at least one role)", err)
	}
	if len(h.se.pcSearches) != 0 || len(h.se.createdPC) != 0 {
		t.Error("no project membership may be created for a refused invitation")
	}
}

// TestMembershipWrite_InviteAllowsNoRolesForAnIntegrationUser is a regression
// test for a real, reported bug: the Customer Portal's Add Contact form
// correctly sends zero human-facing roles for a CS integration user (it has
// no Asgardeo identity and never signs in, so Portal user/Lead/Security
// Contact/Admin are all meaningless for it) -- but Invite's own "roles must
// contain at least one role" guard used to reject every such request
// unconditionally, since it only ever checked the role list, never
// IsCsIntegrationUser. A non-integration contact with zero roles must still
// be rejected -- that's still a mistake, just not for an integration user.
func TestMembershipWrite_InviteAllowsNoRolesForAnIntegrationUser(t *testing.T) {
	h := newInternalWriteHarness(t)
	req := inviteReq() // no roles
	req.IsCsIntegrationUser = true

	if _, err := h.svc.Invite(context.Background(), writeProjectID, req); err != nil {
		t.Fatalf("Invite() error = %v, want nil for a roleless integration user", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(h.repo.upserts))
	}
	if !h.repo.upserts[0].IsCsIntegrationUser {
		t.Error("membership upsert isCsIntegrationUser = false, want true")
	}

	h2 := newInternalWriteHarness(t)
	_, err := h2.svc.Invite(context.Background(), writeProjectID, inviteReq()) // no roles, not an integration user
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError for a roleless non-integration contact", err)
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
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Roles: []string{"Portal user"}}

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

// TestMembershipWrite_InviteOfASignedInContactStoresRegisteredAndPublishes:
// Salesforce saves REGISTERED for an unlocked contact; CSM stores that and the
// existing-account invitation is still published, for a new or reactivated row.
func TestMembershipWrite_InviteOfASignedInContactStoresRegisteredAndPublishes(t *testing.T) {
	for _, reactivate := range []bool{false, true} {
		t.Run(fmt.Sprintf("reactivate=%v", reactivate), func(t *testing.T) {
			h := newInternalWriteHarness(t)
			h.se.contact = existingSalesforceContact()
			h.se.savedState = domain.MembershipStateRegistered
			if reactivate {
				h.repo.existing = &domain.ProjectMembershipRow{
					ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
					Email: writeEmail, State: domain.MembershipStateDeactivated,
				}
				h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Roles: []string{"Portal user"}}
			}
			got, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user"))
			if err != nil {
				t.Fatalf("Invite: %v", err)
			}
			if got.State != domain.MembershipStateRegistered || h.repo.upserts[0].State != domain.MembershipStateRegistered {
				t.Errorf("state = %q / %q, want REGISTERED (what Salesforce stored)", got.State, h.repo.upserts[0].State)
			}
			if len(h.pub.published) != 1 || h.pub.published[0].Type != events.TypeProjectContactInvited {
				t.Fatalf("published = %+v, want one project_contact.invited", h.pub.published)
			}
		})
	}
}

// A re-invitation's PATCH lands but its re-read comes back empty, so the
// record in hand still carries the pre-write LastModifiedDate. The version
// stamped on the write and the event must still be strictly newer, or
// csm-notification-service takes the re-invitation for a duplicate of the
// earlier one and drops its email.
func TestMembershipWrite_ReinviteVersionIsNewerThanThePreWriteOne(t *testing.T) {
	for _, tt := range []struct {
		name string
		pre  string
	}{
		{"pre-write version in the past: now is used", "2026-09-18T06:37:07.000+0000"},
		{"pre-write version ahead of the clock: one microsecond later", "2999-01-01T00:00:00.000+0000"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			h.repo.existing = &domain.ProjectMembershipRow{
				ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
				Email: writeEmail, State: domain.MembershipStateDeactivated,
			}
			h.se.contact = existingSalesforceContact()
			pre := tt.pre
			h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, LastModifiedDate: &pre, Roles: []string{"Portal user"}}

			if _, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err != nil {
				t.Fatalf("Invite: %v", err)
			}
			preTime, ok := parseSalesforceLastModified(&pre)
			if !ok {
				t.Fatal("bad fixture")
			}
			stamped := h.repo.steps[0].EventModifiedOn
			if !stamped.After(preTime) || stamped.Sub(preTime) < time.Microsecond {
				t.Errorf("stamped version %s must be at least 1µs after the pre-write %s", stamped, preTime)
			}
			var payload events.ProjectContactInvitedPayload
			if err := json.Unmarshal(h.pub.published[0].Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.EventModifiedOn != stamped.UTC().Format(time.RFC3339Nano) {
				t.Errorf("payload eventModifiedOn = %q, want the stamped %s", payload.EventModifiedOn, stamped)
			}
		})
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
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Roles: []string{"Portal user", "Admin"}}

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

// A portal role edit owns only the four portal labels. Every other label on
// the Salesforce membership -- Business Contact, the D2 labels, anything
// unknown -- was set in Salesforce and must survive the PATCH, and the stored
// groups follow what Salesforce now holds.
func TestMembershipWrite_UpdateRolesPreservesUnmanagedSalesforceRoles(t *testing.T) {
	for _, tt := range []struct {
		name    string
		current salesentity.ProjectContact
	}{
		{"raw role string", salesentity.ProjectContact{ID: writeMembershipID, Role: sampleStr("Portal user;Business Contact;Technical Champion")}},
		{"split roles list", salesentity.ProjectContact{ID: writeMembershipID, Roles: []string{"Portal user", "Business Contact", "Technical Champion"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newInternalWriteHarness(t)
			h.repo.existing = &domain.ProjectMembershipRow{
				ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
				Email: writeEmail, State: domain.MembershipStateRegistered,
				ProjectGroups: []string{projectGroupGeneralAccess, projectGroupBusinessContact},
			}
			h.se.contact = existingSalesforceContact()
			current := tt.current
			h.se.membership = &current

			got, err := h.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail,
				domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Portal user", "Security Contact"}})
			if err != nil {
				t.Fatalf("UpdateRoles: %v", err)
			}
			want := []string{"Portal user", "Security Contact", "Business Contact", "Technical Champion"}
			if u := h.se.updates[0]; u.roles == nil || !reflect.DeepEqual(*u.roles, want) {
				t.Errorf("PATCHed roles = %v, want %v", u.roles, want)
			}
			if g := h.repo.upserts[0].ProjectGroups; !reflect.DeepEqual(g, []string{projectGroupFullAccess, projectGroupBusinessContact}) {
				t.Errorf("groups = %v, want Full Access + Business Contact  Group", g)
			}
			// The response answers with what the caller manages.
			if !reflect.DeepEqual(got.Roles, []string{"Portal user", "Security Contact"}) {
				t.Errorf("response roles = %v", got.Roles)
			}
		})
	}
}

// A re-invitation PATCHes Role__c too, so it keeps the Salesforce-only
// labels the same way a role edit does.
func TestMembershipWrite_ReinvitePreservesUnmanagedSalesforceRoles(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateDeactivated,
	}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID, Roles: []string{"Admin", "Business Owner", "Some Future Label"}}

	if _, err := h.svc.Invite(context.Background(), writeProjectID, inviteReq("Portal user")); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	want := []string{"Portal user", "Business Owner", "Some Future Label"}
	if u := h.se.updates[0]; u.roles == nil || !reflect.DeepEqual(*u.roles, want) {
		t.Errorf("PATCHed roles = %v, want %v (Admin dropped, the rest kept)", u.roles, want)
	}
}

// When the fetched membership carries neither role nor roles, its current
// labels are unknown: the write fails before any PATCH rather than replacing
// Role__c blind, and no row is written.
func TestMembershipWrite_UpdateRolesRefusesWhenCurrentRolesUnreadable(t *testing.T) {
	h := newInternalWriteHarness(t)
	h.repo.existing = &domain.ProjectMembershipRow{
		ProjectContactID: "pc-1", MembershipSfID: writeMembershipID, ContactSfID: writeContactSfID,
		Email: writeEmail, State: domain.MembershipStateRegistered,
	}
	h.se.contact = existingSalesforceContact()
	h.se.membership = &salesentity.ProjectContact{ID: writeMembershipID}

	_, err := h.svc.UpdateRoles(context.Background(), writeProjectID, writeEmail,
		domain.UpdateProjectMembershipRolesRequest{Roles: []string{"Portal user"}})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if len(h.se.updates) != 0 || len(h.repo.upserts) != 0 {
		t.Errorf("updates = %d, upserts = %d; want nothing written", len(h.se.updates), len(h.repo.upserts))
	}
}

func TestMergePortalManagedRoles(t *testing.T) {
	for _, tt := range []struct {
		name      string
		requested []string
		current   salesentity.ProjectContact
		want      []string
	}{
		{"no current roles (empty list)", []string{"Portal user"}, salesentity.ProjectContact{Roles: []string{}}, []string{"Portal user"}},
		{"empty raw role", []string{"Lead"}, salesentity.ProjectContact{Role: sampleStr("")}, []string{"Lead"}},
		{"managed labels are replaced, case-insensitively", []string{"Security Contact"},
			salesentity.ProjectContact{Roles: []string{"portal user", "ADMIN", "Lead"}}, []string{"Security Contact"}},
		{"every unmanaged label is kept in stored order", []string{"Portal user"},
			salesentity.ProjectContact{Role: sampleStr("Technical Detractor; Business Promoter;Business Detractor;Technical Owner;Business Contact;Business Owner;Technical Champion")},
			[]string{"Portal user", "Technical Detractor", "Business Promoter", "Business Detractor", "Technical Owner", "Business Contact", "Business Owner", "Technical Champion"}},
		{"clearing the portal roles keeps the rest", nil,
			salesentity.ProjectContact{Roles: []string{"Portal user", "Business Contact"}}, []string{"Business Contact"}},
		{"roles wins over role; duplicates collapse", []string{"Portal user"},
			salesentity.ProjectContact{Role: sampleStr("ignored"), Roles: []string{"Business Contact", "business contact", " "}}, []string{"Portal user", "Business Contact"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergePortalManagedRoles(tt.requested, tt.current)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
	if _, err := mergePortalManagedRoles([]string{"Portal user"}, salesentity.ProjectContact{}); err == nil {
		t.Error("unreadable current roles must be an error")
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
