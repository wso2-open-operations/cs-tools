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
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// ---- fakes ---------------------------------------------------------------

type fakeMembershipSalesEntity struct {
	projectContacts map[string]salesentity.ProjectContact
	contacts        map[string]salesentity.Contact
	pcCalls         []string
	contactCalls    []string
	pcErr           error
	contactErr      error
}

func (f *fakeMembershipSalesEntity) GetProjectContact(_ context.Context, id string) (salesentity.ProjectContact, error) {
	f.pcCalls = append(f.pcCalls, id)
	if f.pcErr != nil {
		return salesentity.ProjectContact{}, f.pcErr
	}
	pc, ok := f.projectContacts[id]
	if !ok {
		return salesentity.ProjectContact{}, salesentity.NotFound("salesentity: project contact not found")
	}
	return pc, nil
}

func (f *fakeMembershipSalesEntity) GetContact(_ context.Context, id string) (salesentity.Contact, error) {
	f.contactCalls = append(f.contactCalls, id)
	if f.contactErr != nil {
		return salesentity.Contact{}, f.contactErr
	}
	c, ok := f.contacts[id]
	if !ok {
		return salesentity.Contact{}, salesentity.NotFound("salesentity: contact not found")
	}
	return c, nil
}

type fakeMembershipRepo struct {
	upserts   []domain.SalesforceMembershipUpsert
	steps     []domain.UpsertOnboardingStepRequest
	upsertErr error
	// rowAlreadyExisted makes the upsert report that it UPDATED an existing
	// project_contact rather than creating one -- which is what a portal
	// write's own Salesforce echo looks like by the time it reaches here.
	rowAlreadyExisted bool
	// previousState is the state that existing row carried BEFORE the
	// upsert. A portal write's echo finds it equal to the incoming state
	// (the portal stored it first); a re-invitation made in Salesforce
	// finds DEACTIVATED there. Only meaningful with rowAlreadyExisted.
	previousState string
	deactivated   []string
	deactivateOK  bool
	deactivateErr error
	// deactivateBasis is the admin-role basis the DELETED path handed over,
	// so a test can run it the way the repository would.
	deactivateBasis repository.AdminRoleBasisFunc
}

func (f *fakeMembershipRepo) Upsert(_ context.Context, in domain.SalesforceMembershipUpsert, step domain.UpsertOnboardingStepRequest) (domain.SalesforceMembershipUpsertResult, error) {
	f.upserts = append(f.upserts, in)
	f.steps = append(f.steps, step)
	if f.upsertErr != nil {
		return domain.SalesforceMembershipUpsertResult{}, f.upsertErr
	}
	return domain.SalesforceMembershipUpsertResult{
		ProjectID:             "proj-1",
		ProjectContactID:      "pc-1",
		CreatedUser:           true,
		CreatedProjectContact: !f.rowAlreadyExisted,
		PreviousState:         f.previousState,
	}, nil
}

func (f *fakeMembershipRepo) DeactivateBySfID(_ context.Context, id string, basis repository.AdminRoleBasisFunc) (bool, error) {
	f.deactivated = append(f.deactivated, id)
	f.deactivateBasis = basis
	return f.deactivateOK, f.deactivateErr
}

func (f *fakeMembershipRepo) UpsertWithin(context.Context, string, string, repository.MembershipWritePlan) (domain.SalesforceMembershipUpsertResult, error) {
	return domain.SalesforceMembershipUpsertResult{}, errors.New("not used by the ingest")
}

func (f *fakeMembershipRepo) ResolveWriteContext(context.Context, string, string) (repository.MembershipWriteContext, error) {
	return repository.MembershipWriteContext{}, errors.New("not used by the ingest")
}

func (f *fakeMembershipRepo) GetMembershipByEmail(context.Context, string, string) (domain.ProjectMembershipRow, error) {
	return domain.ProjectMembershipRow{}, errors.New("not used by the ingest")
}

type fakeStepRepo struct {
	existing      []domain.OnboardingStep
	upserts       []domain.UpsertOnboardingStepRequest
	getErr        error
	listErr       error
	retryAttempts []string
	requeued      []repository.MissingParent
}

func (f *fakeStepRepo) RecordRetryAttempt(_ context.Context, stepID string) (bool, error) {
	f.retryAttempts = append(f.retryAttempts, stepID)
	return true, nil
}

func (f *fakeStepRepo) RequeueMissingParentFailures(_ context.Context, parent repository.MissingParent) (int64, error) {
	f.requeued = append(f.requeued, parent)
	return 0, nil
}

func (f *fakeStepRepo) Upsert(_ context.Context, req domain.UpsertOnboardingStepRequest) (domain.OnboardingStep, error) {
	f.upserts = append(f.upserts, req)
	return domain.OnboardingStep{MembershipSfID: req.MembershipSfID, Step: req.Step, Status: req.Status}, nil
}

func (f *fakeStepRepo) GetByMembership(_ context.Context, id string) ([]domain.OnboardingStep, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	var out []domain.OnboardingStep
	for _, s := range f.existing {
		if s.MembershipSfID == id {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeStepRepo) Search(context.Context, domain.SearchOnboardingStepsRequest) ([]domain.OnboardingStep, int, error) {
	return nil, 0, nil
}

type fakeInvitePublisher struct {
	published []events.Envelope
	err       error
}

func (f *fakeInvitePublisher) Publish(_ context.Context, t events.Type, id string, payload json.RawMessage) error {
	f.published = append(f.published, events.Envelope{Type: t, EntityID: id, Payload: payload})
	return f.err
}
func (f *fakeInvitePublisher) Close() {}

// ---- fixtures ------------------------------------------------------------

const (
	testMembershipID = "a0e000000000001AAA"
	testContactID    = "003000000000001AAA"
	testProjectID    = "a0p000000000001AAA"
	testAccountID    = "001000000000001AAA"
	testLastModified = "2026-09-18T06:37:07.000+0000"
)

func sampleProjectContact(state string, roles ...string) salesentity.ProjectContact {
	return salesentity.ProjectContact{
		ID:    testMembershipID,
		Email: "Jane@Acme.com",
		State: sampleStr(state),
		Roles: roles,
		Type:  sampleStr(domain.MembershipTypeOwnContact),
		Contact: &salesentity.ProjectContactContact{
			ID: sampleStr(testContactID), Name: sampleStr("Jane Doe"), Email: sampleStr("jane@acme.com"), CustomerID: sampleStr(testAccountID),
		},
		Subscription: &salesentity.ProjectContactSubscription{
			ID: sampleStr(testProjectID), Name: sampleStr("Acme Prod"), Key: sampleStr("ACMEPROD"), CustomerID: sampleStr(testAccountID),
		},
		LastModifiedDate: sampleStr(testLastModified),
	}
}

func sampleContact() salesentity.Contact {
	return salesentity.Contact{
		ID: sampleStr(testContactID), Email: sampleStr("jane@acme.com"), Name: sampleStr("Jane Doe"),
		FirstName: sampleStr("Jane"), LastName: sampleStr("Doe"),
		IsCsAdmin: boolPtr(false), IsCsIntegrationUser: boolPtr(false),
		Account:     &salesentity.ContactAccount{ID: sampleStr(testAccountID)},
		Memberships: []salesentity.ContactMembership{{ID: sampleStr(testMembershipID), SubscriptionID: sampleStr(testProjectID)}},
	}
}

type ingestHarness struct {
	se       *fakeMembershipSalesEntity
	repo     *fakeMembershipRepo
	steps    *fakeStepRepo
	contacts *fakeContactRepo
	states   *fakeIngestStateRepo
	accounts *stubSalesforceAccountRepo
	pub      *fakeInvitePublisher
	svc      SalesforceEventService
}

// testAccountCSMID is the CSM id the harness's account lookup answers for
// testAccountID, so the Contact writer's EnsureAccount finds the parent.
const testAccountCSMID = "5f0c6f5e-1b1e-4c1e-9d6a-2f6f5f0c6f5e"

func newIngestHarness(pc salesentity.ProjectContact, contact salesentity.Contact, withPublisher bool) *ingestHarness {
	h := &ingestHarness{
		se: &fakeMembershipSalesEntity{
			projectContacts: map[string]salesentity.ProjectContact{pc.ID: pc},
			contacts:        map[string]salesentity.Contact{derefString(contact.ID): contact},
		},
		repo:     &fakeMembershipRepo{deactivateOK: true},
		steps:    &fakeStepRepo{},
		contacts: &fakeContactRepo{deactivateFound: true},
		states:   &fakeIngestStateRepo{},
		accounts: &stubSalesforceAccountRepo{accountsBySfID: map[string]string{testAccountID: testAccountCSMID}},
	}
	ingest := MembershipIngest{Memberships: h.repo, Steps: h.steps, SalesEntity: h.se, Contacts: h.contacts}
	if withPublisher {
		h.pub = &fakeInvitePublisher{}
		ingest.Publisher = h.pub
	}
	h.svc = NewSalesforceEventServiceWithMembershipIngest(h.accounts, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: h.accounts, States: h.states}, ingest)
	return h
}

func membershipEvent(eventType, entity string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{EventType: eventType, Entity: entity, ReferenceID: testMembershipID}
}

// ---- mapping tables ------------------------------------------------------

func TestMapProjectGroups(t *testing.T) {
	cases := []struct {
		name    string
		roles   []string
		groups  []string
		ignored []string
	}{
		{"portal+security → Full Access", []string{"Portal user", "Security Contact"}, []string{projectGroupFullAccess}, nil},
		{"portal only → General Access", []string{"Portal user"}, []string{projectGroupGeneralAccess}, nil},
		{"security only → Security Only", []string{"Security Contact"}, []string{projectGroupSecurityOnly}, nil},
		{"lead adds Lead User Group", []string{"Portal user", "Lead"}, []string{projectGroupGeneralAccess, projectGroupLeadUserGroup}, nil},
		{"lead alone", []string{"Lead"}, []string{projectGroupLeadUserGroup}, nil},
		{"admin joins the Admin group", []string{"Admin"}, []string{projectGroupAdmin}, nil},
		{"case-insensitive", []string{"PORTAL USER", " security contact "}, []string{projectGroupFullAccess}, nil},
		{"unknown roles ignored", []string{"Portal user", "Billing Contact", "Legal"}, []string{projectGroupGeneralAccess}, []string{"Billing Contact", "Legal"}},
		{"no roles", nil, nil, nil},
		{"business contact alone", []string{"Business Contact"}, []string{projectGroupBusinessContact}, nil},
		{"portal user + business contact", []string{"Portal user", "Business Contact"}, []string{projectGroupGeneralAccess, projectGroupBusinessContact}, nil},
		{"security contact + business contact", []string{"Business Contact", "Security Contact"}, []string{projectGroupSecurityOnly, projectGroupBusinessContact}, nil},
		{"every mapped role at once", []string{"Portal user", "Security Contact", "Lead", "Admin", "Business Contact"},
			[]string{projectGroupFullAccess, projectGroupLeadUserGroup, projectGroupAdmin, projectGroupBusinessContact}, nil},
		{"business contact is case-insensitive", []string{" business CONTACT "}, []string{projectGroupBusinessContact}, nil},
		{"the six D2 labels stay ignored", []string{"Portal user", "Business Owner", "Business Promoter", "Business Detractor", "Technical Owner", "Technical Champion", "Technical Detractor", "Business Contact"},
			[]string{projectGroupGeneralAccess, projectGroupBusinessContact},
			[]string{"Business Owner", "Business Promoter", "Business Detractor", "Technical Owner", "Technical Champion", "Technical Detractor"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups, ignored := mapProjectGroups(tc.roles)
			if !reflect.DeepEqual(groups, tc.groups) {
				t.Errorf("groups = %v, want %v", groups, tc.groups)
			}
			if !reflect.DeepEqual(ignored, tc.ignored) {
				t.Errorf("ignored = %v, want %v", ignored, tc.ignored)
			}
		})
	}
}

// TestMapProjectGroups_BusinessContactGroupName pins the exact stored name:
// ServiceNow named the group with two spaces, and syncProjectGroups matches
// project_group."group" exactly.
func TestMapProjectGroups_BusinessContactGroupName(t *testing.T) {
	if projectGroupBusinessContact != "Business Contact  Group" {
		t.Errorf("projectGroupBusinessContact = %q, want the stored name with two spaces", projectGroupBusinessContact)
	}
}

func TestSalesforceRolesForGroups_BusinessContactRoundTrips(t *testing.T) {
	groups, _ := mapProjectGroups([]string{"Portal user", "Business Contact"})
	if got := salesforceRolesForGroups(groups); !reflect.DeepEqual(got, []string{"Portal user", "Business Contact"}) {
		t.Errorf("roles = %v: a portal deactivation writes these back to Salesforce, and must not drop Business Contact", got)
	}
}

func TestSplitIgnoredRoles(t *testing.T) {
	byDecision, unknown := splitIgnoredRoles([]string{"Technical Owner", "Billing Contact", "business promoter"})
	if !reflect.DeepEqual(byDecision, []string{"Technical Owner", "business promoter"}) || !reflect.DeepEqual(unknown, []string{"Billing Contact"}) {
		t.Errorf("byDecision = %v, unknown = %v", byDecision, unknown)
	}
}

// TestMapGlobalRoles pins decision D1: customer or partner follows the
// contact's account classification (it used to follow the membership type),
// and {customer, partner} is a managed pair only when the classification is
// known (non-blank).
func TestMapGlobalRoles(t *testing.T) {
	sorted := func(s []string) []string { c := append([]string{}, s...); sort.Strings(c); return c }
	account := func(classification *string) *salesentity.ContactAccount {
		return &salesentity.ContactAccount{ID: sampleStr(testAccountID), Classification: classification}
	}
	cases := []struct {
		name          string
		account       *salesentity.ContactAccount
		isIntegration bool
		want          []string
		wantAdminRole string
		wantManaged   bool
		wantOrgPair   bool
	}{
		{"customer account", account(sampleStr("Customer")), false, []string{"customer", "external"}, "customer_admin", true, true},
		{"partner account", account(sampleStr("Partner")), false, []string{"external", "partner"}, "partner_admin", true, true},
		{"classification is case-insensitive", account(sampleStr(" partner ")), false, []string{"external", "partner"}, "partner_admin", true, true},
		{"no classification: customer, pair not managed", account(nil), false, []string{"customer", "external"}, "customer_admin", true, false},
		{"blank classification: customer, pair not managed", account(sampleStr("  ")), false, []string{"customer", "external"}, "customer_admin", true, false},
		{"unknown account: customer, pair not managed", nil, false, []string{"customer", "external"}, "customer_admin", true, false},
		{"integration user gets nothing", account(sampleStr("Partner")), true, nil, "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapGlobalRoles(tc.account, tc.isIntegration)
			if !reflect.DeepEqual(sorted(got.Grant), sorted(tc.want)) {
				t.Errorf("grant = %v, want %v", got.Grant, tc.want)
			}
			// The admin role is never GRANTED here -- mapGlobalRoles only says
			// which of the two it would be. Whether the user holds it is
			// derived from every membership they have, in the repository,
			// after the write.
			for _, r := range got.Grant {
				if r == "customer_admin" || r == "partner_admin" {
					t.Errorf("mapGlobalRoles must not grant an admin role, got %v", got.Grant)
				}
			}
			if got.AdminRole != tc.wantAdminRole {
				t.Errorf("adminRole = %q, want %q", got.AdminRole, tc.wantAdminRole)
			}
			if tc.wantManaged && !reflect.DeepEqual(sorted(got.ManagedAdmin), []string{"customer_admin", "partner_admin"}) {
				t.Errorf("managed admin = %v", got.ManagedAdmin)
			}
			if !tc.wantManaged && got.ManagedAdmin != nil {
				t.Errorf("managed admin = %v, want nil", got.ManagedAdmin)
			}
			if tc.wantOrgPair && !reflect.DeepEqual(sorted(got.ManagedGlobal), []string{"customer", "partner"}) {
				t.Errorf("managed org roles = %v, want the {customer, partner} pair", got.ManagedGlobal)
			}
			if !tc.wantOrgPair && got.ManagedGlobal != nil {
				t.Errorf("managed org roles = %v, want nil", got.ManagedGlobal)
			}
		})
	}
}

func TestNormalizeMembershipState(t *testing.T) {
	for in, want := range map[string]string{
		"INVITED": "INVITED", " registered ": "REGISTERED", "Re-Invited": "RE-INVITED", "RE_INVITED": "RE-INVITED", "deactivated": "DEACTIVATED",
	} {
		got, err := normalizeMembershipState(in)
		if err != nil || got != want {
			t.Errorf("normalizeMembershipState(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "PENDING", "ACTIVE"} {
		_, err := normalizeMembershipState(bad)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("normalizeMembershipState(%q) err = %v, want ValidationError", bad, err)
		}
	}
}

func TestParseSalesforceLastModified(t *testing.T) {
	got, ok := parseSalesforceLastModified(sampleStr(testLastModified))
	if !ok || !got.Equal(time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC)) {
		t.Fatalf("parse = %v, %v", got, ok)
	}
	if _, ok := parseSalesforceLastModified(sampleStr("2026-09-18T06:37:07Z")); !ok {
		t.Error("RFC3339 should parse")
	}
	if _, ok := parseSalesforceLastModified(nil); ok {
		t.Error("nil should not parse")
	}
	if _, ok := parseSalesforceLastModified(sampleStr("yesterday")); ok {
		t.Error("garbage should not parse")
	}
}

// ---- ingest scenarios -----------------------------------------------------

func TestMembershipIngest_CreatedInvitedPublishesEvent(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user", "Security Contact", "Admin"), sampleContact(), true)

	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(h.repo.upserts))
	}
	in := h.repo.upserts[0]
	if in.MembershipSfID != testMembershipID || in.State != "INVITED" || in.Email != "jane@acme.com" || in.ContactSfID != testContactID ||
		in.ContactAccountSfID != testAccountID || in.ProjectKey != "ACMEPROD" || in.ProjectSfID != testProjectID ||
		in.ContactFirstName != "Jane" || in.ContactLastName != "Doe" || in.IsCsIntegrationUser {
		t.Errorf("unexpected upsert: %+v", in)
	}
	// Admin is a PROJECT role now: it joins the Admin group rather than
	// landing straight on the user as a global role.
	if !reflect.DeepEqual(in.ProjectGroups, []string{projectGroupFullAccess, projectGroupAdmin}) {
		t.Errorf("groups = %v", in.ProjectGroups)
	}
	sort.Strings(in.GlobalRoles)
	if !reflect.DeepEqual(in.GlobalRoles, []string{"customer", "external"}) {
		t.Errorf("global roles = %v", in.GlobalRoles)
	}
	if in.AdminRoleName != "customer_admin" {
		t.Errorf("adminRoleName = %q, want customer_admin (which of the two, not whether)", in.AdminRoleName)
	}
	step := h.repo.steps[0]
	if step.Step != domain.OnboardingStepDatabase || step.Status != domain.OnboardingStepSucceeded || step.EventType != "CREATED" ||
		step.Email != "jane@acme.com" || step.ContactSfID == nil || *step.ContactSfID != testContactID || step.UpdatedBy != domain.SalesforceSyncActor ||
		!step.EventModifiedOn.Equal(time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC)) {
		t.Errorf("unexpected step: %+v", step)
	}
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1", len(h.pub.published))
	}
	env := h.pub.published[0]
	if env.Type != events.TypeProjectContactInvited || env.EntityID != testMembershipID {
		t.Errorf("envelope = %+v", env)
	}
	var payload events.ProjectContactInvitedPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Email != "jane@acme.com" || payload.GivenName != "Jane" || payload.FamilyName != "Doe" || payload.ProjectKey != "ACMEPROD" ||
		payload.ProjectName != "Acme Prod" || payload.ContactSfID != testContactID || payload.Type != domain.MembershipTypeOwnContact ||
		!reflect.DeepEqual(payload.Roles, []string{"Portal user", "Security Contact", "Admin"}) || payload.IsIntegrationUser {
		t.Errorf("payload = %+v", payload)
	}
	// Not merely "a valid, non-zero timestamp": this value is what the
	// consumer's own version check compares against, so a payload carrying
	// the processing time instead of the record's LastModifiedDate would
	// silently make every replay look newer. Pin the exact instant.
	wantEventModifiedOn, ok := parseSalesforceLastModified(sampleStr(testLastModified))
	if !ok {
		t.Fatalf("the testLastModified fixture %q must parse", testLastModified)
	}
	if ts, err := time.Parse(time.RFC3339Nano, payload.EventModifiedOn); err != nil || !ts.Equal(wantEventModifiedOn) {
		t.Errorf("payload.eventModifiedOn = %q, want %s (the membership's Salesforce LastModifiedDate in RFC 3339)",
			payload.EventModifiedOn, wantEventModifiedOn.Format(time.RFC3339Nano))
	}
	if h.se.pcCalls[0] != testMembershipID || h.se.contactCalls[0] != testContactID {
		t.Errorf("calls: pc=%v contact=%v", h.se.pcCalls, h.se.contactCalls)
	}
}

func TestMembershipIngest_EventOnlyForInvitedStates(t *testing.T) {
	for state, want := range map[string]int{"INVITED": 1, "RE-INVITED": 1, "REGISTERED": 0, "DEACTIVATED": 0} {
		t.Run(state, func(t *testing.T) {
			h := newIngestHarness(sampleProjectContact(state, "Portal user"), sampleContact(), true)
			if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if len(h.repo.upserts) != 1 {
				t.Fatalf("upserts = %d", len(h.repo.upserts))
			}
			if len(h.pub.published) != want {
				t.Errorf("published = %d, want %d", len(h.pub.published), want)
			}
		})
	}
}

// TestMembershipIngest_EchoOfAPortalWriteDoesNotPublish is the echo
// suppression. Every portal membership write also writes Salesforce, and that
// Salesforce write comes back here through the Service Bus subscriber as an
// ordinary CREATED/UPDATED envelope. By then the row already exists AND
// already carries the state the echo is announcing — the portal write wrote
// both first — so the event is our own write returning. Without this, one
// invitation sent by a customer admin would produce TWO e-mails: one from
// the portal write, one from its own echo.
func TestMembershipIngest_EchoOfAPortalWriteDoesNotPublish(t *testing.T) {
	for _, state := range []string{"INVITED", "RE-INVITED"} {
		t.Run(state, func(t *testing.T) {
			h := newIngestHarness(sampleProjectContact(state, "Portal user"), sampleContact(), true)
			h.repo.rowAlreadyExisted = true
			h.repo.previousState = state

			if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			// The row is still updated — silently. Only the e-mail is suppressed.
			if len(h.repo.upserts) != 1 {
				t.Fatalf("upserts = %d, want 1: an echo must still update the row", len(h.repo.upserts))
			}
			if len(h.pub.published) != 0 {
				t.Errorf("published = %d, want 0 for a membership we already knew about", len(h.pub.published))
			}
		})
	}
}

// TestMembershipIngest_GenuinelySalesforceOriginatedInvitationStillPublishes
// is the other half: an invitation made in Salesforce itself (or the
// historical backfill) creates the row here, so it is not an echo and the
// e-mail must still be sent.
func TestMembershipIngest_GenuinelySalesforceOriginatedInvitationStillPublishes(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	h.repo.rowAlreadyExisted = false

	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.pub.published) != 1 {
		t.Errorf("published = %d, want 1 for a row this ingest created", len(h.pub.published))
	}
}

// TestMembershipIngest_SalesforceReInvitationOfADeactivatedRowPublishes is the
// case the old "did we insert a row" gate silently dropped. Re-inviting in
// Salesforce moves an EXISTING DEACTIVATED project_contact to RE-INVITED, so
// nothing is created and the person would never have been told. The previous
// state is what distinguishes it from a portal echo, which arrives with the
// row already in the state it announces.
func TestMembershipIngest_SalesforceReInvitationOfADeactivatedRowPublishes(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("RE-INVITED", "Portal user"), sampleContact(), true)
	h.repo.rowAlreadyExisted = true
	h.repo.previousState = "DEACTIVATED"

	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Fatalf("upserts = %d, want 1", len(h.repo.upserts))
	}
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1: a Salesforce re-invitation must still send an e-mail", len(h.pub.published))
	}
}

func TestMembershipIngest_NilPublisherSkipsEvent(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), false)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("upserts = %d, want 1 (alt entity spelling accepted)", len(h.repo.upserts))
	}
}

func TestMembershipIngest_NoEventWhenDatabaseFails(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	h.repo.upsertErr = &apierror.NotFoundError{Msg: "project not found"}
	err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c"))
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
	if len(h.pub.published) != 0 {
		t.Error("event must not be published when the database write failed")
	}
	if len(h.steps.upserts) != 1 || h.steps.upserts[0].Status != domain.OnboardingStepFailed || h.steps.upserts[0].LastError == nil ||
		*h.steps.upserts[0].LastError != "project not found" || h.steps.upserts[0].Step != domain.OnboardingStepDatabase {
		t.Errorf("FAILED step not recorded: %+v", h.steps.upserts)
	}
}

func TestMembershipIngest_IdempotentReplay(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	// No step recorded yet (the fake repo does not persist) → every replay
	// runs the upsert with identical input.
	for i := 0; i < 3; i++ {
		if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
	}
	if len(h.repo.upserts) != 3 {
		t.Fatalf("upserts = %d", len(h.repo.upserts))
	}
	if !reflect.DeepEqual(h.repo.upserts[0], h.repo.upserts[1]) || !reflect.DeepEqual(h.repo.upserts[1], h.repo.upserts[2]) {
		t.Error("replays must produce identical upsert requests")
	}
}

func TestMembershipIngest_DuplicateEventSkipped(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	recorded := time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC) // == lastModifiedDate
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepSucceeded, EventModifiedOn: recorded,
	}}
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.repo.upserts) != 0 || len(h.pub.published) != 0 || len(h.se.contactCalls) != 0 {
		t.Errorf("duplicate must be skipped before the contact fetch: upserts=%d published=%d contactCalls=%d", len(h.repo.upserts), len(h.pub.published), len(h.se.contactCalls))
	}

	// A newer Salesforce version is ingested.
	pc := sampleProjectContact("REGISTERED", "Portal user")
	pc.LastModifiedDate = sampleStr("2026-09-18T07:00:00.000+0000")
	h.se.projectContacts[testMembershipID] = pc
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent newer: %v", err)
	}
	if len(h.repo.upserts) != 1 || h.repo.upserts[0].State != "REGISTERED" {
		t.Errorf("newer version not ingested: %+v", h.repo.upserts)
	}

	// A DATABASE row last written by a DELETED event never blocks the same
	// version: an undelete (RESTORED) keeps the Salesforce LastModifiedDate
	// and the membership must leave DEACTIVATED.
	h3 := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), false)
	h3.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepSucceeded,
		EventType: string(domain.SalesforceEventDeleted), EventModifiedOn: recorded,
	}}
	if err := h3.svc.HandleEvent(context.Background(), membershipEvent("RESTORED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h3.repo.upserts) != 1 || h3.repo.upserts[0].State != "INVITED" {
		t.Errorf("a DELETED-typed step must not suppress the restore: %+v", h3.repo.upserts)
	}

	// A FAILED DATABASE row never blocks a retry of the same version.
	h2 := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), false)
	h2.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepFailed, EventModifiedOn: recorded,
	}}
	if err := h2.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h2.repo.upserts) != 1 {
		t.Error("a FAILED step must not suppress the retry")
	}
}

func TestMembershipIngest_UnparseableLastModifiedStillIngests(t *testing.T) {
	pc := sampleProjectContact("INVITED", "Portal user")
	pc.LastModifiedDate = nil
	h := newIngestHarness(pc, sampleContact(), false)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts) != 1 || h.repo.steps[0].EventModifiedOn.IsZero() {
		t.Errorf("upsert should run with a non-zero eventModifiedOn: %+v", h.repo.steps)
	}
}

func TestMembershipIngest_UnknownStateIsValidationError(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("PENDING", "Portal user"), sampleContact(), true)
	err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c"))
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if len(h.repo.upserts) != 0 || len(h.pub.published) != 0 {
		t.Error("nothing must be written for an unknown state")
	}
}

// TestMembershipIngest_RolesFollowAccountClassification is decision D1 on the
// membership path. It used to read the membership TYPE: a PARTNER CONTACT
// got partner/partner_admin whatever their own account was. Now the
// contact's account classification decides, as in the Contact writer, so
// the two writers agree.
func TestMembershipIngest_RolesFollowAccountClassification(t *testing.T) {
	t.Run("a PARTNER CONTACT from a customer account is a customer", func(t *testing.T) {
		pc := sampleProjectContact("INVITED", "Portal user", "Admin")
		pc.Type = sampleStr(domain.MembershipTypePartnerContact)
		c := sampleContact()
		c.Account.Classification = sampleStr("Customer")
		h := newIngestHarness(pc, c, false)
		if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
			t.Fatal(err)
		}
		in := h.repo.upserts[0]
		got := append([]string{}, in.GlobalRoles...)
		sort.Strings(got)
		// Changed expectation: [external partner] / partner_admin before D1.
		if !reflect.DeepEqual(got, []string{"customer", "external"}) || in.AdminRoleName != "customer_admin" {
			t.Errorf("roles = %v, adminRole = %q, want customer roles", got, in.AdminRoleName)
		}
		if !reflect.DeepEqual(in.ManagedGlobalRoles, []string{"customer", "partner"}) {
			t.Errorf("managed org roles = %v", in.ManagedGlobalRoles)
		}
		// The membership type itself is untouched: it is still stored and sent.
		if in.Type != domain.MembershipTypePartnerContact {
			t.Errorf("type = %q", in.Type)
		}
		if !reflect.DeepEqual(in.ProjectGroups, []string{projectGroupGeneralAccess, projectGroupAdmin}) {
			t.Errorf("groups = %v", in.ProjectGroups)
		}
	})

	t.Run("an OWN CONTACT of a Partner account is a partner", func(t *testing.T) {
		c := sampleContact()
		c.Account.Classification = sampleStr("Partner")
		h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), c, false)
		if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
			t.Fatal(err)
		}
		in := h.repo.upserts[0]
		got := append([]string{}, in.GlobalRoles...)
		sort.Strings(got)
		if !reflect.DeepEqual(got, []string{"external", "partner"}) || in.AdminRoleName != "partner_admin" {
			t.Errorf("roles = %v, adminRole = %q, want partner roles", got, in.AdminRoleName)
		}
	})
}

func TestMembershipIngest_IntegrationUser(t *testing.T) {
	c := sampleContact()
	c.IsCsIntegrationUser = boolPtr(true)
	c.IsCsAdmin = boolPtr(true)
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user", "Admin"), c, true)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	in := h.repo.upserts[0]
	if !in.IsCsIntegrationUser || len(in.GlobalRoles) != 0 || len(in.ManagedAdminRoles) != 0 {
		t.Errorf("integration user must get no global roles: %+v", in)
	}
	if !reflect.DeepEqual(in.ProjectGroups, []string{projectGroupGeneralAccess, projectGroupAdmin}) {
		t.Errorf("project groups still apply: %v", in.ProjectGroups)
	}
	if in.AdminRoleName != "" {
		t.Errorf("adminRoleName = %q, want empty for an integration user", in.AdminRoleName)
	}
	if len(h.pub.published) != 1 {
		t.Fatal("event still published so the consumer can record IDENTITY/EMAIL as SKIPPED")
	}
	var payload events.ProjectContactInvitedPayload
	_ = json.Unmarshal(h.pub.published[0].Payload, &payload)
	if !payload.IsIntegrationUser {
		t.Error("payload.isIntegrationUser must be true")
	}
}

func TestMembershipIngest_IgnoredRolesDoNotFail(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Billing Contact"), sampleContact(), false)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts[0].ProjectGroups) != 0 {
		t.Errorf("groups = %v, want none", h.repo.upserts[0].ProjectGroups)
	}
}

func TestMembershipIngest_RoleStringFallback(t *testing.T) {
	pc := sampleProjectContact("INVITED")
	pc.Roles = nil
	pc.Role = sampleStr("Portal user;Lead")
	h := newIngestHarness(pc, sampleContact(), true)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.repo.upserts[0].ProjectGroups, []string{projectGroupGeneralAccess, projectGroupLeadUserGroup}) {
		t.Errorf("groups = %v", h.repo.upserts[0].ProjectGroups)
	}
	var payload events.ProjectContactInvitedPayload
	_ = json.Unmarshal(h.pub.published[0].Payload, &payload)
	if !reflect.DeepEqual(payload.Roles, []string{"Portal user", "Lead"}) {
		t.Errorf("payload roles = %v", payload.Roles)
	}
}

func TestMembershipIngest_DeletedDeactivates(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	delete(h.se.projectContacts, testMembershipID)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("DELETED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.repo.deactivated, []string{testMembershipID}) || len(h.repo.upserts) != 0 || len(h.se.pcCalls) != 1 {
		t.Errorf("DELETED must only deactivate: deactivated=%v upserts=%d pcCalls=%d", h.repo.deactivated, len(h.repo.upserts), len(h.se.pcCalls))
	}
	if h.repo.deactivateBasis == nil {
		t.Error("DELETED must hand the repository an admin-role basis so the admin role is re-derived")
	}
	// Unknown membership: still a success (204).
	h.repo.deactivateOK = false
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("DELETED", "Project_Contact__c")); err != nil {
		t.Errorf("unknown DELETED should be a no-op, got %v", err)
	}
}

func TestMembershipIngest_SalesEntityMissingIsServiceUnavailable(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), false)
	delete(h.se.projectContacts, testMembershipID)
	err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c"))
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}

	pc := sampleProjectContact("INVITED", "Portal user")
	pc.Contact = nil
	h2 := newIngestHarness(pc, sampleContact(), false)
	if err := h2.svc.HandleEvent(context.Background(), membershipEvent("CREATED", "Project_Contact__c")); !errors.As(err, &sue) {
		t.Errorf("missing contact: err = %v, want ServiceUnavailableError", err)
	}
}

func TestMembershipIngest_UnknownEntityAndDisabledAreNoOps(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), true)
	for _, entity := range []string{"Opportunity", "Case", "Project__c"} {
		if err := h.svc.HandleEvent(context.Background(), membershipEvent("CREATED", entity)); err != nil {
			t.Errorf("%s: %v", entity, err)
		}
	}
	if len(h.repo.upserts) != 0 || len(h.se.pcCalls) != 0 {
		t.Error("unknown entities must not touch anything")
	}

	// Constructed without the ingest (flag off): membership envelopes are acknowledged and ignored.
	off := NewSalesforceEventService(&stubSalesforceAccountRepo{}, &stubSalesEntityClient{}, SalesforceIngestSupport{})
	for _, entity := range []string{"Project_Contact__c", "Contact"} {
		if err := off.HandleEvent(context.Background(), membershipEvent("UPDATED", entity)); err != nil {
			t.Errorf("disabled %s: %v", entity, err)
		}
	}
}

func TestMembershipIngest_BadEventTypes(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), sampleContact(), false)
	var ve *apierror.ValidationError
	for _, et := range []string{"UNDEFINED", "MOVED"} {
		if err := h.svc.HandleEvent(context.Background(), membershipEvent(et, "Project_Contact__c")); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want ValidationError", et, err)
		}
	}
}

func TestContactEvent_UpdatedReplaysMemberships(t *testing.T) {
	pc1 := sampleProjectContact("REGISTERED", "Portal user")
	pc2 := sampleProjectContact("INVITED", "Security Contact")
	pc2.ID = "a0e000000000002AAA"
	pc2.Subscription.Key = sampleStr("ACMEDEV")
	contact := sampleContact()
	contact.Name = sampleStr("Jane Roe")
	contact.LastName = sampleStr("Roe")
	contact.Memberships = []salesentity.ContactMembership{{ID: sampleStr(pc1.ID)}, {ID: sampleStr(pc2.ID)}, {ID: nil}}
	h := newIngestHarness(pc1, contact, true)
	h.se.projectContacts[pc2.ID] = pc2

	req := domain.SalesforceEventRequest{EventType: "UPDATED", Entity: "Contact", ReferenceID: testContactID}
	if err := h.svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts) != 2 {
		t.Fatalf("upserts = %d, want 2", len(h.repo.upserts))
	}
	for _, in := range h.repo.upserts {
		if in.ContactLastName != "Roe" || in.ContactName != "Jane Roe" {
			t.Errorf("contact profile not propagated: %+v", in)
		}
	}
	if h.repo.upserts[0].ProjectKey != "ACMEPROD" || h.repo.upserts[1].ProjectKey != "ACMEDEV" {
		t.Errorf("wrong memberships: %q %q", h.repo.upserts[0].ProjectKey, h.repo.upserts[1].ProjectKey)
	}
	if len(h.pub.published) != 1 || h.pub.published[0].EntityID != pc2.ID {
		t.Errorf("only the INVITED membership publishes: %+v", h.pub.published)
	}
	// The contact is read once and handed to every membership.
	if len(h.se.contactCalls) != 1 {
		t.Errorf("contact reads = %d, want 1 (the fan-out reuses the fetched contact)", len(h.se.contactCalls))
	}
	// And the Contact writer ran first.
	if len(h.contacts.upserts) != 1 || h.contacts.upserts[0].LastName != "Roe" {
		t.Errorf("contact writer upserts = %+v", h.contacts.upserts)
	}
}

// A contact edit leaves the membership's LastModifiedDate unchanged, so the
// guard must compare the contact's own timestamp or the rename is dropped.
func TestContactEvent_RenameAfterEarlierIngestIsApplied(t *testing.T) {
	contact := sampleContact()
	contact.Name = sampleStr("Jane Roe")
	contact.LastName = sampleStr("Roe")
	contact.LastModifiedDate = sampleStr("2026-09-18T08:15:00.000+0000")
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), contact, false)
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepSucceeded,
		EventModifiedOn: time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC), // == membership lastModifiedDate
	}}

	req := domain.SalesforceEventRequest{EventType: "UPDATED", Entity: "Contact", ReferenceID: testContactID}
	if err := h.svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts) != 1 || h.repo.upserts[0].ContactLastName != "Roe" {
		t.Fatalf("contact rename not applied: %+v", h.repo.upserts)
	}
	want := time.Date(2026, 9, 18, 8, 15, 0, 0, time.UTC)
	if got := h.repo.steps[0].EventModifiedOn; !got.Equal(want) {
		t.Errorf("step eventModifiedOn = %s, want the contact's %s", got, want)
	}

	// Replaying the same contact version is now a duplicate.
	h.steps.existing[0].EventModifiedOn = want
	if err := h.svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("replay of the same contact version must be skipped: upserts=%d", len(h.repo.upserts))
	}

	// With no parseable membership timestamp, the contact's alone drives the guard.
	pc := sampleProjectContact("REGISTERED", "Portal user")
	pc.LastModifiedDate = nil
	h.se.projectContacts[testMembershipID] = pc
	if err := h.svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("contact timestamp must drive the guard when the membership's is missing: upserts=%d", len(h.repo.upserts))
	}
}

func TestContactEvent_FirstErrorReturnedAfterProcessingAll(t *testing.T) {
	pc1 := sampleProjectContact("REGISTERED", "Portal user")
	pc2 := sampleProjectContact("REGISTERED", "Portal user")
	pc2.ID = "a0e000000000002AAA"
	contact := sampleContact()
	contact.Memberships = []salesentity.ContactMembership{{ID: sampleStr("a0e000000000009AAA")}, {ID: sampleStr(pc2.ID)}}
	h := newIngestHarness(pc1, contact, false)
	h.se.projectContacts[pc2.ID] = pc2

	err := h.svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{EventType: "UPDATED", Entity: "Contact", ReferenceID: testContactID})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want the missing membership's ServiceUnavailableError", err)
	}
	if len(h.repo.upserts) != 1 || h.repo.upserts[0].MembershipSfID != pc2.ID {
		t.Errorf("the other membership must still be processed: %+v", h.repo.upserts)
	}
}

func TestBuildMembershipUpsert_Fallbacks(t *testing.T) {
	pc := sampleProjectContact("INVITED", "Portal user")
	pc.Email = ""
	pc.Contact.CustomerID = nil
	c := sampleContact()
	c.Email = nil
	c.FirstName, c.LastName = nil, nil
	c.Name = sampleStr("Mary Ann Smith")
	in, _, err := buildMembershipUpsert(pc, c)
	if err != nil {
		t.Fatal(err)
	}
	if in.Email != "jane@acme.com" || in.ContactEmail != "jane@acme.com" {
		t.Errorf("email fallback: %+v", in)
	}
	if in.ContactFirstName != "Mary Ann" || in.ContactLastName != "Smith" {
		t.Errorf("name split: %q %q", in.ContactFirstName, in.ContactLastName)
	}
	if in.ContactAccountSfID != testAccountID {
		t.Errorf("account fallback to contact.account.id: %q", in.ContactAccountSfID)
	}

	pc.Contact.Email = nil
	if _, _, err := buildMembershipUpsert(pc, c); err == nil {
		t.Error("no email anywhere must be a ValidationError")
	}
}

func (f *fakeStepRepo) ListMissingParentFailures(_ context.Context, _ time.Duration, maxAttempts, limit int) ([]domain.OnboardingStep, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []domain.OnboardingStep{}
	for _, s := range f.existing {
		if s.Step == domain.OnboardingStepDatabase && s.Status == domain.OnboardingStepFailed &&
			repository.IsMissingParentError(derefString(s.LastError)) && s.RetryCount < maxAttempts && len(out) < limit {
			out = append(out, s)
		}
	}
	return out, nil
}

// TestMembershipIngest_RegisteredTransitionPublishesWelcome: only a move from
// INVITED/RE-INVITED into REGISTERED publishes project_contact.registered.
func TestMembershipIngest_RegisteredTransitionPublishesWelcome(t *testing.T) {
	cases := map[string]struct {
		existed  bool
		previous string
		want     int
	}{
		"invited to registered":     {existed: true, previous: "INVITED", want: 1},
		"re-invited to registered":  {existed: true, previous: "RE-INVITED", want: 1},
		"echo of registered":        {existed: true, previous: "REGISTERED", want: 0},
		"deactivated to registered": {existed: true, previous: "DEACTIVATED", want: 0},
		"backfilled as registered":  {existed: false, want: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), sampleContact(), true)
			h.repo.rowAlreadyExisted = tc.existed
			h.repo.previousState = tc.previous
			if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			welcomes := publishedOfType(h.pub, events.TypeProjectContactRegistered)
			if len(welcomes) != tc.want {
				t.Fatalf("welcomes = %d, want %d", len(welcomes), tc.want)
			}
			if tc.want == 0 {
				return
			}
			env := welcomes[0]
			if env.Type != events.TypeProjectContactRegistered || env.EntityID != testMembershipID {
				t.Errorf("envelope = %s %s", env.Type, env.EntityID)
			}
			var p events.ProjectContactRegisteredPayload
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				t.Fatal(err)
			}
			if p.MembershipSfID != testMembershipID || p.ContactSfID != testContactID || p.Email != "jane@acme.com" ||
				p.GivenName != "Jane" || p.FamilyName != "Doe" || p.ProjectName != "Acme Prod" || p.ProjectKey != "ACMEPROD" || p.EventModifiedOn == "" {
				t.Errorf("payload = %+v", p)
			}
		})
	}
}

// TestMembershipIngest_ReplayOfRegisteredVersionDoesNotPublish: the duplicate
// guard drops a replay before any publish.
func TestMembershipIngest_ReplayOfRegisteredVersionDoesNotPublish(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), sampleContact(), true)
	h.repo.rowAlreadyExisted = true
	h.repo.previousState = "INVITED"
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h.pub.published) != 1 {
		t.Fatalf("published = %d, want 1 for the transition", len(h.pub.published))
	}
	h.steps.existing = []domain.OnboardingStep{{
		MembershipSfID: testMembershipID, Step: domain.OnboardingStepDatabase, Status: domain.OnboardingStepSucceeded, EventModifiedOn: h.repo.steps[0].EventModifiedOn,
	}}
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	if len(h.pub.published) != 1 {
		t.Errorf("published = %d after a replay, want still 1", len(h.pub.published))
	}
}

func publishedOfType(p *fakeInvitePublisher, typ events.Type) []events.Envelope {
	var out []events.Envelope
	for _, e := range p.published {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// TestMembershipIngest_NewMembershipOfASignedInContactPublishesInvited: an
// unlocked contact's new membership is saved REGISTERED; only a new one is told.
func TestMembershipIngest_NewMembershipOfASignedInContactPublishesInvited(t *testing.T) {
	fresh := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	cases := map[string]struct {
		eventType   string
		existed     bool
		previous    string
		createdDate *string
		want        int
	}{
		"created now":                   {eventType: "CREATED", createdDate: &fresh, want: 1},
		"created by an UPDATED first":   {eventType: "UPDATED", createdDate: &fresh, want: 1},
		"backfill of an old record":     {eventType: "UPDATED", createdDate: &old, want: 0},
		"no createdDate":                {eventType: "CREATED", want: 0},
		"reactivated from deactivated":  {eventType: "UPDATED", existed: true, previous: "DEACTIVATED", want: 1},
		"undelete of a deactivated row": {eventType: "RESTORED", existed: true, previous: "DEACTIVATED", want: 0},
		"echo of a portal write":        {eventType: "UPDATED", existed: true, previous: "REGISTERED", createdDate: &fresh, want: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pc := sampleProjectContact("REGISTERED", "Portal user")
			pc.CreatedDate = tc.createdDate
			h := newIngestHarness(pc, sampleContact(), true)
			h.repo.rowAlreadyExisted = tc.existed
			h.repo.previousState = tc.previous
			if err := h.svc.HandleEvent(context.Background(), membershipEvent(tc.eventType, "Project_Contact__c")); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if got := len(publishedOfType(h.pub, events.TypeProjectContactInvited)); got != tc.want {
				t.Errorf("invited = %d, want %d", got, tc.want)
			}
			if got := len(publishedOfType(h.pub, events.TypeProjectContactRegistered)); got != 0 {
				t.Errorf("welcomes = %d, want 0", got)
			}
		})
	}
}
