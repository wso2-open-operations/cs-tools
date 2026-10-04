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
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

// ---- fakes ---------------------------------------------------------------

type fakeContactRepo struct {
	upserts          []domain.SalesforceContactUpsert
	states           []domain.UpsertSalesforceIngestStateRequest
	upsertErr        error
	deactivated      []string
	deactivateStates []domain.UpsertSalesforceIngestStateRequest
	deactivateFound  bool
	deactivateErr    error
}

func (f *fakeContactRepo) Upsert(_ context.Context, in domain.SalesforceContactUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceContactUpsertResult, error) {
	f.upserts = append(f.upserts, in)
	f.states = append(f.states, state)
	if f.upsertErr != nil {
		return domain.SalesforceContactUpsertResult{}, f.upsertErr
	}
	return domain.SalesforceContactUpsertResult{UserID: "user-1", AccountContactID: "ac-1", CreatedUser: true, CreatedAccountContact: true}, nil
}

func (f *fakeContactRepo) DeactivateBySfID(_ context.Context, id string, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	f.deactivated = append(f.deactivated, id)
	f.deactivateStates = append(f.deactivateStates, state)
	return f.deactivateFound, f.deactivateErr
}

// ---- fixtures ------------------------------------------------------------

func contactEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{EventType: eventType, Entity: "Contact", ReferenceID: testContactID}
}

// sampleWriterContact is sampleContact with the fields the Contact writer
// reads: a classified account, the flat accountId, the primary flag and a
// record version, and no memberships.
func sampleWriterContact() salesentity.Contact {
	c := sampleContact()
	c.Email = sampleStr(" Jane@Acme.com ")
	c.Account = &salesentity.ContactAccount{ID: sampleStr(testAccountID), Classification: sampleStr("Customer")}
	c.AccountID = sampleStr(testAccountID)
	c.IsPrimaryContact = boolPtr(true)
	c.LastModifiedDate = sampleStr(testLastModified)
	c.Memberships = nil
	return c
}

// ---- CREATED / UPDATED / RESTORED ------------------------------------------

// TestContactWriter_ContactWithoutMembershipIsWritten is the main build item:
// a contact with no project membership (a commercial or billing contact)
// used to write nothing to CSM. Every event type that carries a live record
// now writes it.
func TestContactWriter_ContactWithoutMembershipIsWritten(t *testing.T) {
	for _, et := range []string{"CREATED", "UPDATED", "RESTORED"} {
		t.Run(et, func(t *testing.T) {
			h := newIngestHarness(sampleProjectContact("INVITED"), sampleWriterContact(), false)
			if err := h.svc.HandleEvent(context.Background(), contactEvent(et)); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if len(h.contacts.upserts) != 1 {
				t.Fatalf("contact upserts = %d, want 1", len(h.contacts.upserts))
			}
			in := h.contacts.upserts[0]
			if in.ContactSfID != testContactID || in.Email != "jane@acme.com" || in.Name != "Jane Doe" || in.FirstName != "Jane" || in.LastName != "Doe" ||
				in.AccountID != testAccountCSMID || in.AccountSfID != testAccountID || in.IsPrimaryContact == nil || !*in.IsPrimaryContact ||
				in.IsCsAdmin || in.IsCsIntegrationUser {
				t.Errorf("unexpected contact upsert: %+v", in)
			}
			roles := append([]string{}, in.GlobalRoles...)
			sort.Strings(roles)
			if !reflect.DeepEqual(roles, []string{"customer", "external"}) || in.AdminRoleName != "customer_admin" ||
				!reflect.DeepEqual(in.ManagedGlobalRoles, []string{"customer", "partner"}) ||
				!reflect.DeepEqual(in.ManagedAdminRoles, []string{"customer_admin", "partner_admin"}) {
				t.Errorf("roles = %v managedGlobal = %v managedAdmin = %v adminRole = %q", roles, in.ManagedGlobalRoles, in.ManagedAdminRoles, in.AdminRoleName)
			}
			st := h.contacts.states[0]
			if st.Entity != domain.SalesforceIngestEntityContact || st.SfID != testContactID || st.EventType != et ||
				st.Status != domain.SalesforceIngestSucceeded || !st.EventModifiedOn.Equal(time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC)) {
				t.Errorf("ledger row = %+v", st)
			}
			if len(h.repo.upserts) != 0 || len(h.se.pcCalls) != 0 {
				t.Error("a contact with no membership must not touch memberships")
			}
		})
	}
}

// TestContactWriter_PartnerAccount: D1 on the Contact writer.
func TestContactWriter_PartnerAccount(t *testing.T) {
	c := sampleWriterContact()
	c.Account.Classification = sampleStr("Partner")
	c.IsCsAdmin = boolPtr(true)
	h := newIngestHarness(sampleProjectContact("INVITED"), c, false)
	if err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err != nil {
		t.Fatal(err)
	}
	in := h.contacts.upserts[0]
	roles := append([]string{}, in.GlobalRoles...)
	sort.Strings(roles)
	if !reflect.DeepEqual(roles, []string{"external", "partner"}) || in.AdminRoleName != "partner_admin" || !in.IsCsAdmin {
		t.Errorf("roles = %v adminRole = %q isCsAdmin = %v", roles, in.AdminRoleName, in.IsCsAdmin)
	}
}

// TestContactWriter_IntegrationUser keeps the ingest's behaviour: the user
// row is written (is_system_user), with no global roles and no admin
// decision.
func TestContactWriter_IntegrationUser(t *testing.T) {
	c := sampleWriterContact()
	c.IsCsIntegrationUser = boolPtr(true)
	c.IsCsAdmin = boolPtr(true)
	h := newIngestHarness(sampleProjectContact("INVITED"), c, false)
	if err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err != nil {
		t.Fatal(err)
	}
	in := h.contacts.upserts[0]
	if !in.IsCsIntegrationUser || len(in.GlobalRoles) != 0 || len(in.ManagedGlobalRoles) != 0 || len(in.ManagedAdminRoles) != 0 || in.AdminRoleName != "" {
		t.Errorf("integration user must get no roles: %+v", in)
	}
}

// TestContactWriter_FanOutReusesTheContact: the contact is read once and each
// membership's upsert uses it, including its primary flag and classification.
func TestContactWriter_FanOutReusesTheContact(t *testing.T) {
	pc1 := sampleProjectContact("REGISTERED", "Portal user")
	pc2 := sampleProjectContact("REGISTERED", "Business Contact")
	pc2.ID = "a0e000000000002AAA"
	c := sampleWriterContact()
	c.Account.Classification = sampleStr("Partner")
	c.Memberships = []salesentity.ContactMembership{{ID: sampleStr(pc1.ID)}, {ID: sampleStr(pc2.ID)}}
	h := newIngestHarness(pc1, c, false)
	h.se.projectContacts[pc2.ID] = pc2

	if err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err != nil {
		t.Fatal(err)
	}
	if len(h.se.contactCalls) != 1 {
		t.Errorf("contact reads = %d, want 1", len(h.se.contactCalls))
	}
	if len(h.repo.upserts) != 2 {
		t.Fatalf("membership upserts = %d, want 2", len(h.repo.upserts))
	}
	for _, in := range h.repo.upserts {
		if in.IsPrimaryContact == nil || !*in.IsPrimaryContact || in.AdminRoleName != "partner_admin" {
			t.Errorf("membership upsert did not use the fetched contact: %+v", in)
		}
	}
	if !reflect.DeepEqual(h.repo.upserts[1].ProjectGroups, []string{projectGroupBusinessContact}) {
		t.Errorf("groups = %v", h.repo.upserts[1].ProjectGroups)
	}
}

// TestIngestMembership_OtherContactIsRefetched: a contact handed in that is
// not the membership's own is never used in its place.
func TestIngestMembership_OtherContactIsRefetched(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), sampleContact(), false)
	other := sampleContact()
	other.ID = sampleStr("003000000000009AAA")
	svc := h.svc.(*salesforceEventService)
	if err := svc.ingestMembership(context.Background(), testMembershipID, "UPDATED", &other); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.se.contactCalls, []string{testContactID}) {
		t.Errorf("contact reads = %v, want the membership's own contact", h.se.contactCalls)
	}
}

// TestContactWriter_DuplicateVersionSkipsTheWriteNotTheFanOut: a contact
// version the ledger already holds is not written again, but its memberships
// still run (each with its own guard), so a membership that failed on the
// first delivery is retried by the redelivery.
func TestContactWriter_DuplicateVersionSkipsTheWriteNotTheFanOut(t *testing.T) {
	c := sampleWriterContact()
	c.Memberships = []salesentity.ContactMembership{{ID: sampleStr(testMembershipID)}}
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), c, false)
	recorded := ingestStateRow(domain.SalesforceIngestSucceeded, "UPDATED", testLastModified)
	recorded.Entity, recorded.SfID = domain.SalesforceIngestEntityContact, testContactID
	h.states.rows = map[string]domain.SalesforceIngestState{domain.SalesforceIngestEntityContact + "/" + testContactID: recorded}

	if err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err != nil {
		t.Fatal(err)
	}
	if len(h.contacts.upserts) != 0 {
		t.Errorf("duplicate contact version must not be written again: %d", len(h.contacts.upserts))
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("memberships must still fan out: %d", len(h.repo.upserts))
	}

	// A ledger row stamped DELETED never blocks the RESTORED of that version.
	recorded.EventType = domain.SalesforceEventDeleted
	h.states.rows[domain.SalesforceIngestEntityContact+"/"+testContactID] = recorded
	if err := h.svc.HandleEvent(context.Background(), contactEvent("RESTORED")); err != nil {
		t.Fatal(err)
	}
	if len(h.contacts.upserts) != 1 {
		t.Errorf("RESTORED after DELETED must be written: %d", len(h.contacts.upserts))
	}
}

// TestContactWriter_MissingAccountIngestOff: the parent account is not in CSM
// and the Account ingest is off, so EnsureAccount cannot create it. The event
// fails (Service Bus redelivers it), the ledger says why, and nothing else
// runs.
func TestContactWriter_MissingAccountIngestOff(t *testing.T) {
	c := sampleWriterContact()
	c.Memberships = []salesentity.ContactMembership{{ID: sampleStr(testMembershipID)}}
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), c, false)
	lookup := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{}}
	h.svc = NewSalesforceEventServiceWithMembershipIngest(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: h.states},
		MembershipIngest{Memberships: h.repo, Steps: h.steps, SalesEntity: h.se, Contacts: h.contacts})

	err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED"))
	var nfe *apierror.NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
	if len(h.contacts.upserts) != 0 || len(h.repo.upserts) != 0 {
		t.Error("nothing may be written without the parent account")
	}
	if len(h.states.upserts) != 1 || h.states.upserts[0].Status != domain.SalesforceIngestFailed || h.states.upserts[0].LastError == nil ||
		h.states.upserts[0].Entity != domain.SalesforceIngestEntityContact {
		t.Errorf("FAILED ledger row not recorded: %+v", h.states.upserts)
	}
}

// TestContactWriter_MissingAccountIngestOn: with the Account ingest on,
// EnsureAccount writes the parent first and the contact follows.
func TestContactWriter_MissingAccountIngestOn(t *testing.T) {
	c := sampleWriterContact()
	c.AccountID = sampleStr(sampleCustomer().ID)
	c.Account.ID = sampleStr(sampleCustomer().ID)
	h := newIngestHarness(sampleProjectContact("REGISTERED"), c, false)
	accounts := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{}, registerOnUpsert: testAccountCSMID}
	se := &stubSalesEntityClient{customer: sampleCustomer()}
	h.svc = NewSalesforceEventServiceWithMembershipIngest(accounts, se, SalesforceIngestSupport{Accounts: accounts, States: h.states},
		MembershipIngest{Memberships: h.repo, Steps: h.steps, SalesEntity: h.se, Contacts: h.contacts})

	if err := h.svc.HandleEvent(context.Background(), contactEvent("CREATED")); err != nil {
		t.Fatal(err)
	}
	if accounts.upsertCalls != 1 || len(h.contacts.upserts) != 1 || h.contacts.upserts[0].AccountID != testAccountCSMID {
		t.Errorf("accountUpserts = %d, contact upserts = %+v", accounts.upsertCalls, h.contacts.upserts)
	}
}

// TestContactRetrier_MissingAccountIsRetriedOnceTheAccountLands: the FAILED
// ledger row the Contact writer records for a missing account carries the
// missing-parent prefix, so the delayed-retry job hands it to the contact
// retrier, which re-runs the writer as UPDATED once the account is in CSM.
func TestContactRetrier_MissingAccountIsRetriedOnceTheAccountLands(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED"), sampleWriterContact(), false)
	lookup := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{}}
	svc := NewSalesforceEventServiceWithMembershipIngest(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: h.states},
		MembershipIngest{Memberships: h.repo, Steps: h.steps, SalesEntity: h.se, Contacts: h.contacts})

	if err := svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err == nil {
		t.Fatal("want the missing account to fail the event")
	}
	if len(h.states.upserts) != 1 || h.states.upserts[0].LastError == nil {
		t.Fatalf("FAILED ledger row not recorded: %+v", h.states.upserts)
	}
	failed := h.states.upserts[0]
	if !repository.IsMissingParentError(*failed.LastError) {
		t.Fatalf("last_error %q is not a missing-parent error, so the retry job would skip it", *failed.LastError)
	}

	// The account arrives through the ServiceNow sync; the job's next tick.
	lookup.accountsBySfID[testAccountID] = testAccountCSMID
	jobStates := &fakeIngestStateRepo{failed: []domain.SalesforceIngestState{{
		Entity: failed.Entity, SfID: failed.SfID, Status: domain.SalesforceIngestFailed, LastError: failed.LastError, AttemptCount: 1,
	}}}
	w := NewSalesforceIngestRetryWorker(nil, nil, jobStates, time.Minute)
	re, ok := svc.(ContactReingester)
	if !ok {
		t.Fatal("the membership-ingest service must implement ContactReingester")
	}
	w.EntityRetriers[domain.SalesforceIngestEntityContact] = re.RetryContactIngest
	w.RunOnce(context.Background())

	if len(h.contacts.upserts) != 1 || h.contacts.upserts[0].AccountID != testAccountCSMID {
		t.Fatalf("contact upserts = %+v, want the retrier to write the contact", h.contacts.upserts)
	}
	if st := h.contacts.states[0]; st.EventType != domain.SalesforceEventUpdated || st.Status != domain.SalesforceIngestSucceeded {
		t.Errorf("ledger row = %+v, want UPDATED SUCCEEDED", st)
	}
}

// TestRetryContactIngest_MembershipIngestOff: the retrier is only registered
// with the membership ingest on; built without it, it says so.
func TestRetryContactIngest_MembershipIngestOff(t *testing.T) {
	off := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{}).(*salesforceEventService)
	if err := off.RetryContactIngest(context.Background(), testContactID); !errors.Is(err, errMembershipIngestDisabled) {
		t.Errorf("err = %v, want errMembershipIngestDisabled", err)
	}
}

// TestContactWriter_WriteFailureRecordsLedgerAndStops: a failed contact write
// is recorded FAILED and returned before the fan-out.
func TestContactWriter_WriteFailureRecordsLedgerAndStops(t *testing.T) {
	c := sampleWriterContact()
	c.Memberships = []salesentity.ContactMembership{{ID: sampleStr(testMembershipID)}}
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), c, false)
	h.contacts.upsertErr = &apierror.ConflictError{Msg: "more than one user matches the contact email; cannot resolve the membership"}

	err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED"))
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
	if len(h.repo.upserts) != 0 {
		t.Error("the fan-out must not run after a failed contact write")
	}
	if len(h.states.upserts) != 1 || h.states.upserts[0].Status != domain.SalesforceIngestFailed ||
		*h.states.upserts[0].LastError != "more than one user matches the contact email; cannot resolve the membership" {
		t.Errorf("FAILED ledger row = %+v", h.states.upserts)
	}
}

func TestContactWriter_NoEmailIsValidationError(t *testing.T) {
	c := sampleWriterContact()
	c.Email = nil
	h := newIngestHarness(sampleProjectContact("REGISTERED"), c, false)
	err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED"))
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if len(h.contacts.upserts) != 0 || len(h.states.upserts) != 1 || h.states.upserts[0].Status != domain.SalesforceIngestFailed {
		t.Errorf("upserts = %d, ledger = %+v", len(h.contacts.upserts), h.states.upserts)
	}
}

// TestContactWriter_NoAccountSkipsTheWrite: account_contact cannot exist
// without an account, so a private contact is acknowledged, not written, and
// its memberships still run.
func TestContactWriter_NoAccountSkipsTheWrite(t *testing.T) {
	c := sampleWriterContact()
	c.AccountID, c.Account = nil, nil
	c.Memberships = []salesentity.ContactMembership{{ID: sampleStr(testMembershipID)}}
	h := newIngestHarness(sampleProjectContact("REGISTERED", "Portal user"), c, false)
	if err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED")); err != nil {
		t.Fatal(err)
	}
	if len(h.contacts.upserts) != 0 || len(h.states.upserts) != 0 {
		t.Errorf("a contact without an account must not be written: upserts=%d ledger=%d", len(h.contacts.upserts), len(h.states.upserts))
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("memberships must still fan out: %d", len(h.repo.upserts))
	}
}

func TestContactWriter_FetchFailureWritesNothing(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED"), sampleWriterContact(), false)
	h.se.contactErr = &apierror.ServiceUnavailableError{Msg: "salesentity: contact not found"}
	err := h.svc.HandleEvent(context.Background(), contactEvent("UPDATED"))
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if len(h.contacts.upserts) != 0 || len(h.states.upserts) != 0 {
		t.Error("nothing may be written when the contact cannot be read")
	}
}

// ---- DELETED ---------------------------------------------------------------

// TestContactWriter_Deleted: one lookup to confirm the contact is gone
// upstream (a deleted record is hidden from reads), soft delete by sf_id,
// and a DELETED ledger row. A contact still present upstream is left alone.
func TestContactWriter_Deleted(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED"), sampleWriterContact(), false)

	// Still present upstream: acknowledged, nothing deactivated.
	if err := h.svc.HandleEvent(context.Background(), contactEvent("DELETED")); err != nil {
		t.Fatal(err)
	}
	if len(h.contacts.deactivated) != 0 {
		t.Fatalf("a contact still present upstream was deactivated: %v", h.contacts.deactivated)
	}

	delete(h.se.contacts, testContactID)
	h.se.contactCalls = nil
	if err := h.svc.HandleEvent(context.Background(), contactEvent("DELETED")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h.contacts.deactivated, []string{testContactID}) || len(h.se.contactCalls) != 1 || len(h.contacts.upserts) != 0 {
		t.Errorf("deactivated = %v contactCalls = %d upserts = %d", h.contacts.deactivated, len(h.se.contactCalls), len(h.contacts.upserts))
	}
	st := h.contacts.deactivateStates[0]
	if st.Entity != domain.SalesforceIngestEntityContact || st.SfID != testContactID || st.EventType != domain.SalesforceEventDeleted ||
		st.Status != domain.SalesforceIngestSucceeded || st.EventModifiedOn.IsZero() {
		t.Errorf("ledger row = %+v", st)
	}

	// Never ingested: still a success, so the envelope is acknowledged.
	h.contacts.deactivateFound = false
	if err := h.svc.HandleEvent(context.Background(), contactEvent("DELETED")); err != nil {
		t.Errorf("unknown DELETED contact should be a no-op, got %v", err)
	}
}

func TestContactWriter_BadEventTypesAndWiring(t *testing.T) {
	h := newIngestHarness(sampleProjectContact("REGISTERED"), sampleWriterContact(), false)
	var ve *apierror.ValidationError
	for _, et := range []string{"UNDEFINED", "MOVED"} {
		if err := h.svc.HandleEvent(context.Background(), contactEvent(et)); !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want ValidationError", et, err)
		}
	}

	// Built without the Contact writer's store: a wiring mistake, reported.
	unwired := NewSalesforceEventServiceWithMembershipIngest(h.accounts, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: h.accounts},
		MembershipIngest{Memberships: h.repo, Steps: h.steps, SalesEntity: h.se})
	for _, et := range []string{"UPDATED", "DELETED"} {
		if err := unwired.HandleEvent(context.Background(), contactEvent(et)); !errors.Is(err, errContactWriterNotConfigured) {
			t.Errorf("%s: err = %v, want errContactWriterNotConfigured", et, err)
		}
	}
}

// ---- admin re-derivation on membership DELETED -----------------------------

// TestAdminRoleBasis is the basis the membership DELETED path hands the
// repository: the admin role from the contact's classification plus its
// isCsAdmin, or !ok when the contact cannot be read (so no role is revoked on
// a guess) or is an integration user.
func TestAdminRoleBasis(t *testing.T) {
	c := sampleContact()
	c.Account.Classification = sampleStr("Partner")
	c.IsCsAdmin = boolPtr(true)
	h := newIngestHarness(sampleProjectContact("INVITED", "Portal user"), c, false)
	delete(h.se.projectContacts, testMembershipID)
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("DELETED", "Project_Contact__c")); err != nil {
		t.Fatal(err)
	}
	basis := h.repo.deactivateBasis
	got, ok := basis(context.Background(), testContactID)
	if !ok || got.AdminRole != "partner_admin" || !got.IsCsAdmin || !reflect.DeepEqual(got.Managed, []string{"customer_admin", "partner_admin"}) {
		t.Errorf("basis = %+v ok = %v", got, ok)
	}

	if _, ok := basis(context.Background(), "003000000000009AAA"); ok {
		t.Error("an unreadable contact must report !ok")
	}

	integration := sampleContact()
	integration.IsCsIntegrationUser = boolPtr(true)
	h.se.contacts[testContactID] = integration
	if _, ok := basis(context.Background(), testContactID); ok {
		t.Error("an integration user has no admin role to re-derive")
	}
}

func TestBuildContactUpsert_NameFallback(t *testing.T) {
	c := sampleWriterContact()
	c.FirstName, c.LastName = nil, nil
	c.Name = sampleStr("Mary Ann Smith")
	in, err := buildContactUpsert(c, testContactID, testAccountCSMID, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if in.FirstName != "Mary Ann" || in.LastName != "Smith" {
		t.Errorf("name split: %q %q", in.FirstName, in.LastName)
	}
}

func TestContactAccountSfID(t *testing.T) {
	c := sampleWriterContact()
	c.AccountID = nil
	if got := contactAccountSfID(c); got != testAccountID {
		t.Errorf("fallback to account.id: %q", got)
	}
	c.AccountID = sampleStr("001000000000002AAA")
	if got := contactAccountSfID(c); got != "001000000000002AAA" {
		t.Errorf("flat accountId first: %q", got)
	}
}
