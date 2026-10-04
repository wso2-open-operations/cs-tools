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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

type stubSalesEntityClient struct {
	customer salesentity.Customer
	err      error
	calls    int
	lastID   string
}

func (s *stubSalesEntityClient) GetCustomer(_ context.Context, id string) (salesentity.Customer, error) {
	s.calls++
	s.lastID = id
	return s.customer, s.err
}

type stubSalesforceAccountRepo struct {
	repository.AccountRepository
	upsertCalls       int
	lastUpsert        domain.SalesforceAccountUpsert
	upsertErr         error
	deleteCalls       int
	lastDeleteID      string
	deleteErr         error
	lookup            map[string]string
	lookupErr         error
	accountsBySfID    map[string]string
	lookupBySfIDErr   error
	lookupBySfIDCalls int
	// registerOnUpsert, when set, makes UpsertFromSalesforce record the
	// written account under this id so a later LookupAccountIDBySfID finds it.
	registerOnUpsert string
	// lastState is the ledger row the last successful write carried; states,
	// when set, receives it as the real repository's transaction would.
	lastState     domain.UpsertSalesforceIngestStateRequest
	states        *fakeIngestStateRepo
	deleteMissing bool
}

func (s *stubSalesforceAccountRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceAccountUpsert, state domain.UpsertSalesforceIngestStateRequest) error {
	s.upsertCalls++
	s.lastUpsert = row
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.lastState = state
	s.states.apply(state)
	if s.registerOnUpsert != "" {
		if s.accountsBySfID == nil {
			s.accountsBySfID = map[string]string{}
		}
		s.accountsBySfID[row.SfID] = s.registerOnUpsert
	}
	return nil
}

func (s *stubSalesforceAccountRepo) SoftDeleteBySfID(_ context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	s.deleteCalls++
	s.lastDeleteID = sfID
	if s.deleteErr != nil {
		return false, s.deleteErr
	}
	s.lastState = state
	s.states.apply(state)
	return !s.deleteMissing, nil
}

func (s *stubSalesforceAccountRepo) LookupUserIDByEmail(_ context.Context, email string) (*string, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if id, ok := s.lookup[strings.ToLower(email)]; ok {
		return &id, nil
	}
	return nil, nil
}

func sampleStr(s string) *string { return &s }

func sampleCustomer() salesentity.Customer {
	return salesentity.Customer{
		ID:                    "001xx0000001",
		Name:                  sampleStr("Acme"),
		Industry:              sampleStr("Technology"),
		Region:                sampleStr("EU"),
		GlobalPod:             sampleStr("EU"),
		Phone:                 sampleStr("+494012345678"),
		SalesRegions:          sampleStr("EMEA"),
		SubRegion:             sampleStr("Northern Germany"),
		Status:                sampleStr("Lost Prospect"),
		NAICSIndustry:         sampleStr("Utilities"),
		SubIndustry:           sampleStr("Energy"),
		AccountClassification: sampleStr("Customer"),
		TechnicalOwner:        sampleStr("owner@example.com"),
	}
}

func TestHandleEvent_CreatedUpdatedRestored(t *testing.T) {
	se := &stubSalesEntityClient{customer: sampleCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: map[string]string{
		"owner@example.com": "user-1",
	}}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	for _, eventType := range []string{
		domain.SalesforceEventCreated,
		domain.SalesforceEventUpdated,
		domain.SalesforceEventRestored,
	} {
		t.Run(eventType, func(t *testing.T) {
			se.calls = 0
			repo.upsertCalls = 0
			err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
				EventType:   "  " + eventType + "  ",
				Entity:      "  Account  ",
				ReferenceID: "  001xx0000001  ",
			})
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if se.calls != 1 || se.lastID != "001xx0000001" {
				t.Errorf("GetCustomer calls = %d id = %q, want 1 / 001xx0000001", se.calls, se.lastID)
			}
			if repo.upsertCalls != 1 {
				t.Errorf("upsert calls = %d, want 1", repo.upsertCalls)
			}
			got := repo.lastUpsert
			if got.SfID != "001xx0000001" || got.Name != "Acme" || got.Number != "001xx0000001" {
				t.Errorf("upsert = %+v", got)
			}
			if got.KeepExistingPhone {
				t.Error("KeepExistingPhone = true, want false for a valid phone")
			}
			if got.TechnicalOwnerID == nil || *got.TechnicalOwnerID != "user-1" {
				t.Errorf("technical owner = %v, want user-1", got.TechnicalOwnerID)
			}
			if got.SecondaryTechnicalOwnerID != nil {
				t.Errorf("secondary owner = %v, want nil", got.SecondaryTechnicalOwnerID)
			}
			if got.AccountVertical != nil {
				t.Errorf("account vertical = %v, want nil", got.AccountVertical)
			}
		})
	}
}

func TestHandleEvent_Deleted(t *testing.T) {
	se := &stubSalesEntityClient{err: salesentity.NotFound("salesentity: customer not found")}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})
	deleted := domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventDeleted,
		Entity:      "account",
		ReferenceID: "  001xx0000001  ",
	}

	if err := svc.HandleEvent(context.Background(), deleted); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if se.calls != 1 || se.lastID != "001xx0000001" {
		t.Errorf("GetCustomer calls = %d id = %q, want one confirming lookup of the trimmed id", se.calls, se.lastID)
	}
	if repo.deleteCalls != 1 || repo.lastDeleteID != "001xx0000001" {
		t.Errorf("soft-delete calls = %d id = %q", repo.deleteCalls, repo.lastDeleteID)
	}
}

// TestHandleEvent_DeletedButStillPresentUpstream: a DELETED whose record the
// upstream still returns is acknowledged without changing anything, and an
// upstream failure is returned (so the event is retried) without changing
// anything either.
func TestHandleEvent_DeletedButStillPresentUpstream(t *testing.T) {
	deleted := domain.SalesforceEventRequest{EventType: domain.SalesforceEventDeleted, Entity: "account", ReferenceID: "001xx0000001"}

	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, &stubSalesEntityClient{customer: fullCustomer()}, SalesforceIngestSupport{})
	if err := svc.HandleEvent(context.Background(), deleted); err != nil {
		t.Fatalf("still present: %v", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("still present: soft-delete calls = %d, want 0", repo.deleteCalls)
	}

	outage := &apierror.ServiceUnavailableError{Msg: "salesentity: customer-search returned 503"}
	svc = NewSalesforceEventService(repo, &stubSalesEntityClient{err: outage}, SalesforceIngestSupport{})
	if err := svc.HandleEvent(context.Background(), deleted); !errors.Is(err, outage) {
		t.Fatalf("upstream failure: err = %v, want it returned", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("upstream failure: soft-delete calls = %d, want 0", repo.deleteCalls)
	}
}

// A customer record without its Salesforce id is refused before any write:
// the id is also the account number, which must be unique and non-empty.
func TestHandleEvent_CustomerWithoutIDIsRefused(t *testing.T) {
	se := &stubSalesEntityClient{customer: salesentity.Customer{ID: "  ", Name: sampleStr("Acme")}}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType: domain.SalesforceEventUpdated, Entity: "Account", ReferenceID: "001xx0000001",
	})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if repo.upsertCalls != 0 {
		t.Errorf("upsert calls = %d, want 0", repo.upsertCalls)
	}
}

// With the account ingest flag off, routes.go passes a nil repository: every
// Account envelope is acknowledged without a Sales Entity read or a write.
func TestHandleEvent_AccountIngestDisabledIgnoresAccountEvents(t *testing.T) {
	se := &stubSalesEntityClient{}
	svc := NewSalesforceEventService(nil, se, SalesforceIngestSupport{})

	for _, eventType := range []string{
		domain.SalesforceEventCreated,
		domain.SalesforceEventUpdated,
		domain.SalesforceEventDeleted,
		domain.SalesforceEventUndefined,
	} {
		t.Run(eventType, func(t *testing.T) {
			err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
				EventType: eventType, Entity: "Account", ReferenceID: "001xx0000001",
			})
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
		})
	}
	if se.calls != 0 {
		t.Errorf("GetCustomer calls = %d, want 0", se.calls)
	}
}

func TestHandleEvent_UnknownEntityIgnored(t *testing.T) {
	se := &stubSalesEntityClient{}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	for _, eventType := range []string{
		domain.SalesforceEventCreated,
		domain.SalesforceEventUndefined,
	} {
		t.Run(eventType, func(t *testing.T) {
			se.calls = 0
			repo.upsertCalls = 0
			repo.deleteCalls = 0
			err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
				EventType:   eventType,
				Entity:      "Contact",
				ReferenceID: "003xx",
			})
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if se.calls != 0 || repo.upsertCalls != 0 || repo.deleteCalls != 0 {
				t.Error("unknown entity must not fetch or persist")
			}
		})
	}
}

func TestHandleEvent_UndefinedAndMissingFields(t *testing.T) {
	svc := NewSalesforceEventService(&stubSalesforceAccountRepo{}, &stubSalesEntityClient{}, SalesforceIngestSupport{})
	cases := []struct {
		name string
		req  domain.SalesforceEventRequest
	}{
		{name: "undefined", req: domain.SalesforceEventRequest{EventType: domain.SalesforceEventUndefined, Entity: "Account", ReferenceID: "001"}},
		{name: "missing eventType", req: domain.SalesforceEventRequest{Entity: "Account", ReferenceID: "001"}},
		{name: "missing entity", req: domain.SalesforceEventRequest{EventType: domain.SalesforceEventCreated, ReferenceID: "001"}},
		{name: "missing referenceId", req: domain.SalesforceEventRequest{EventType: domain.SalesforceEventCreated, Entity: "Account"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.HandleEvent(context.Background(), tc.req)
			var ve *apierror.ValidationError
			if !asValidation(err, &ve) {
				t.Fatalf("err = %v (%T), want *apierror.ValidationError", err, err)
			}
		})
	}
}

func TestHandleEvent_EmptySearchIs503(t *testing.T) {
	se := &stubSalesEntityClient{err: &apierror.ServiceUnavailableError{Msg: "salesentity: customer not found"}}
	svc := NewSalesforceEventService(&stubSalesforceAccountRepo{}, se, SalesforceIngestSupport{})

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventCreated,
		Entity:      "Account",
		ReferenceID: "001",
	})
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

// A customer without a name cannot be written (account.name is NOT NULL) and
// no redelivery can fix it, so the event is acknowledged — no error, hence no
// retry and no dead letter — and recorded FAILED in the ledger with the
// reason.
func TestHandleEvent_MissingNameIsAcknowledgedAndRecordedFailed(t *testing.T) {
	cust := sampleCustomer()
	cust.Name = sampleStr("  ")
	cust.LastModifiedDate = sampleStr("2026-09-29T10:00:00.000+0000")
	se := &stubSalesEntityClient{customer: cust}
	states := &fakeIngestStateRepo{}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{States: states})

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventCreated,
		Entity:      "Account",
		ReferenceID: "001xx0000001",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v, want nil (acknowledged)", err)
	}
	if repo.upsertCalls != 0 {
		t.Errorf("upsert calls = %d, want 0", repo.upsertCalls)
	}
	if len(states.upserts) != 1 {
		t.Fatalf("ledger writes = %d, want 1", len(states.upserts))
	}
	got := states.upserts[0]
	if got.Status != domain.SalesforceIngestFailed || got.Entity != domain.SalesforceIngestEntityAccount ||
		got.SfID != "001xx0000001" || got.EventType != domain.SalesforceEventCreated ||
		got.LastError == nil || !strings.Contains(*got.LastError, "missing Name") {
		t.Errorf("ledger row = %+v (lastError %v)", got, got.LastError)
	}
	if want, _ := parseSalesforceLastModified(cust.LastModifiedDate); !got.EventModifiedOn.Equal(want) {
		t.Errorf("eventModifiedOn = %v, want the record's LastModifiedDate %v", got.EventModifiedOn, want)
	}
}

func TestHandleEvent_MissingOwnerLeavesFKNull(t *testing.T) {
	se := &stubSalesEntityClient{customer: sampleCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: map[string]string{}}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventUpdated,
		Entity:      "Account",
		ReferenceID: "001xx0000001",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.lastUpsert.TechnicalOwnerID != nil || repo.lastUpsert.SecondaryTechnicalOwnerID != nil {
		t.Errorf("owners = %v / %v, want nil", repo.lastUpsert.TechnicalOwnerID, repo.lastUpsert.SecondaryTechnicalOwnerID)
	}
}

// overlengthPhone is 66 characters, longer than account.phone's VARCHAR(64).
var overlengthPhone = "+49 40 1234567 / +49 40 7654321 / +49 40 1111111 / +49 40 22222222"

func TestMapPhone_LongerThan64OmitsUpdate(t *testing.T) {
	got, omit := mapPhone(overlengthPhone)
	if got != nil || !omit {
		t.Errorf("mapPhone(long) = %v omit = %v, want nil / true", got, omit)
	}
	// 22 characters: dropped by the old 20-character limit, kept now that the
	// column is 64 wide (migration 0096).
	got, omit = mapPhone("+49 40 123456789012345")
	if got == nil || omit {
		t.Errorf("mapPhone(22 chars) = %v omit = %v, want kept / false", got, omit)
	}
	got, omit = mapPhone("")
	if got != nil || omit {
		t.Errorf("mapPhone(blank) = %v omit = %v, want nil / false", got, omit)
	}
	got, omit = mapPhone("+494012345678")
	if got == nil || *got != "+494012345678" || omit {
		t.Errorf("mapPhone(short) = %v omit = %v, want kept / false", got, omit)
	}
}

func TestHandleEvent_OverlengthPhoneLeavesExisting(t *testing.T) {
	cust := sampleCustomer()
	cust.Phone = sampleStr(overlengthPhone)
	se := &stubSalesEntityClient{customer: cust}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventUpdated,
		Entity:      "Account",
		ReferenceID: cust.ID,
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if !repo.lastUpsert.KeepExistingPhone || repo.lastUpsert.Phone != nil {
		t.Errorf("upsert phone = %v keep = %v, want nil / true", repo.lastUpsert.Phone, repo.lastUpsert.KeepExistingPhone)
	}
}

func TestMapSalesEntityCustomer_NumberIsId(t *testing.T) {
	got := mapSalesEntityCustomer(context.Background(), salesentity.Customer{ID: "001xx", Name: sampleStr("Acme")})
	if got.Number != "001xx" {
		t.Errorf("Number = %q, want Salesforce Id", got.Number)
	}
}

func asValidation(err error, target **apierror.ValidationError) bool {
	if ve, ok := err.(*apierror.ValidationError); ok {
		*target = ve
		return true
	}
	return false
}

func asSvcUnavailable(err error, target **apierror.ServiceUnavailableError) bool {
	if sue, ok := err.(*apierror.ServiceUnavailableError); ok {
		*target = sue
		return true
	}
	return false
}

func (s *stubSalesforceAccountRepo) LookupAccountIDBySfID(_ context.Context, sfID string) (*string, error) {
	s.lookupBySfIDCalls++
	if s.lookupBySfIDErr != nil {
		return nil, s.lookupBySfIDErr
	}
	if id, ok := s.accountsBySfID[sfID]; ok {
		return &id, nil
	}
	return nil, nil
}

// fullCustomer is sampleCustomer plus every field the Account ingest maps,
// including the SE-1 ones Sales Entity does not send yet.
func fullCustomer() salesentity.Customer {
	c := sampleCustomer()
	c.Address = &salesentity.CustomerAddress{
		BillingStreet: sampleStr(" 1 Main St "), BillingCity: sampleStr("Hamburg"), BillingState: sampleStr("HH"),
		BillingPostalCode: sampleStr("20095"), BillingCountry: sampleStr("Germany"),
	}
	c.Owner = &salesentity.CustomerUser{Email: sampleStr("Manager@Example.com")}
	c.ActivationDate = sampleStr("2024-01-15")
	c.LostDate = sampleStr("2025-02-01")
	c.LostReason = sampleStr("Budget")
	c.LastModifiedDate = sampleStr("2026-09-29T10:00:00.000+0000")
	c.CsmEmail = sampleStr("csm@example.com")
	c.SecondaryTechnicalOwner = sampleStr("tech2@example.com")
	c.RenewalManager = &salesentity.CustomerUser{Email: sampleStr("renewal@example.com")}
	c.AccountVertical = sampleStr("Banking")
	c.LostReasonCategory = sampleStr("Commercial")
	c.DeactivationDate = sampleStr("2027-01-31")
	return c
}

func fullCustomerLookup() map[string]string {
	return map[string]string{
		"owner@example.com":   "user-tech",
		"manager@example.com": "user-manager",
		"csm@example.com":     "user-csm",
		"tech2@example.com":   "user-tech2",
		"renewal@example.com": "user-renewal",
	}
}

func accountEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{EventType: eventType, Entity: "Account", ReferenceID: "001xx0000001"}
}

func date(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

func eqStr(p *string, want string) bool { return p != nil && *p == want }

func eqDate(p *time.Time, want string) bool { return p != nil && p.Equal(date(want)) }

// TestHandleEvent_MapsFullColumnSet: every Salesforce-owned column the
// ingest writes, the person references resolved by email.
func TestHandleEvent_MapsFullColumnSet(t *testing.T) {
	se := &stubSalesEntityClient{customer: fullCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup()}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	if err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	got := repo.lastUpsert
	checks := []struct {
		name string
		ok   bool
	}{
		{"street", eqStr(got.Street, "1 Main St")},
		{"city", eqStr(got.City, "Hamburg")},
		{"state_province", eqStr(got.StateProvince, "HH")},
		{"postal_code", eqStr(got.PostalCode, "20095")},
		{"country", eqStr(got.Country, "Germany")},
		{"account_manager_id", eqStr(got.AccountManagerID, "user-manager")},
		{"technical_owner_id", eqStr(got.TechnicalOwnerID, "user-tech")},
		{"activation_date", eqDate(got.ActivationDate, "2024-01-15")},
		{"lost_date", eqDate(got.LostDate, "2025-02-01")},
		{"lost_reason", eqStr(got.LostReason, "Budget")},
		{"customer_success_manager_id", eqStr(got.CustomerSuccessManagerID, "user-csm")},
		{"secondary_technical_owner_id", eqStr(got.SecondaryTechnicalOwnerID, "user-tech2")},
		{"renewal_account_manager_id", eqStr(got.RenewalAccountManagerID, "user-renewal")},
		{"account_vertical", eqStr(got.AccountVertical, "Banking")},
		{"lost_reason_category", eqStr(got.LostReasonCategory, "Commercial")},
		{"deactivation_date", eqDate(got.DeactivationDate, "2027-01-31")},
		{"number is the Salesforce Id", got.Number == "001xx0000001"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s not mapped: %+v", c.name, got)
		}
	}
	if repo.lastState.Status != domain.SalesforceIngestSucceeded || repo.lastState.EventType != domain.SalesforceEventUpdated ||
		repo.lastState.Entity != domain.SalesforceIngestEntityAccount {
		t.Errorf("ledger row = %+v, want SUCCEEDED UPDATED account", repo.lastState)
	}
}

// TestHandleEvent_AbsentSE1FieldsStayNil: until Sales Entity sends the SE-1
// fields they reach the repository as nil, which its COALESCE turns into
// "keep the stored value" (pinned in account_repo_salesforce_test.go).
func TestHandleEvent_AbsentSE1FieldsStayNil(t *testing.T) {
	se := &stubSalesEntityClient{customer: sampleCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup()}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	if err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	got := repo.lastUpsert
	if got.CustomerSuccessManagerID != nil || got.SecondaryTechnicalOwnerID != nil || got.RenewalAccountManagerID != nil ||
		got.AccountVertical != nil || got.LostReasonCategory != nil || got.DeactivationDate != nil {
		t.Errorf("SE-1 fields = %+v, want all nil", got)
	}
}

// An owner email with no CSM user writes NULL and logs the email; the event
// still succeeds.
func TestHandleEvent_OwnerEmailNotFoundWarnsAndWritesNull(t *testing.T) {
	logs := captureSlog(t)
	lookup := fullCustomerLookup()
	delete(lookup, "manager@example.com")
	se := &stubSalesEntityClient{customer: fullCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: lookup}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	if err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.lastUpsert.AccountManagerID != nil {
		t.Errorf("account manager = %v, want nil", *repo.lastUpsert.AccountManagerID)
	}
	if !eqStr(repo.lastUpsert.CustomerSuccessManagerID, "user-csm") {
		t.Errorf("csm = %v, want user-csm (only the owner is missing)", repo.lastUpsert.CustomerSuccessManagerID)
	}
	out := logs.String()
	if !strings.Contains(out, "does not match a CSM user") || !strings.Contains(out, "Manager@Example.com") || !strings.Contains(out, "role=owner") {
		t.Errorf("want a warning naming the owner email, got: %s", out)
	}
}

// A value longer than its column is written as NULL with a warning; a date
// that does not parse likewise.
func TestHandleEvent_OversizedValueAndBadDateAreNulled(t *testing.T) {
	logs := captureSlog(t)
	cust := fullCustomer()
	cust.Address.BillingPostalCode = sampleStr("123456789012345678901") // 21 > VARCHAR(20)
	cust.Status = sampleStr(strings.Repeat("x", 51))                    // life_cycle VARCHAR(50)
	cust.ActivationDate = sampleStr("15/01/2024")
	se := &stubSalesEntityClient{customer: cust}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup()}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{})

	if err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	got := repo.lastUpsert
	if got.PostalCode != nil || got.LifeCycle != nil || got.ActivationDate != nil {
		t.Errorf("postal %v life cycle %v activation %v, want all nil", got.PostalCode, got.LifeCycle, got.ActivationDate)
	}
	if !eqStr(got.City, "Hamburg") {
		t.Errorf("city = %v, want the in-range value kept", got.City)
	}
	out := logs.String()
	for _, want := range []string{"column=postal_code", "column=life_cycle", "column=activation_date"} {
		if !strings.Contains(out, want) {
			t.Errorf("no warning with %s in: %s", want, out)
		}
	}
}

// TestHandleEvent_DuplicateGuard: a version the ledger already holds is
// skipped without a write; a newer version, or a replay over a FAILED row,
// goes through.
func TestHandleEvent_DuplicateGuard(t *testing.T) {
	cases := []struct {
		name       string
		recorded   *domain.SalesforceIngestState
		modified   string
		wantUpsert bool
	}{
		{name: "never ingested", modified: "2026-09-29T10:00:00.000+0000", wantUpsert: true},
		{name: "same version replayed", recorded: ptrState(ingestStateRow(domain.SalesforceIngestSucceeded, domain.SalesforceEventUpdated, "2026-09-29T10:00:00.000+0000")),
			modified: "2026-09-29T10:00:00.000+0000"},
		{name: "older version redelivered", recorded: ptrState(ingestStateRow(domain.SalesforceIngestSucceeded, domain.SalesforceEventUpdated, "2026-09-29T11:00:00.000+0000")),
			modified: "2026-09-29T10:00:00.000+0000"},
		{name: "newer version", recorded: ptrState(ingestStateRow(domain.SalesforceIngestSucceeded, domain.SalesforceEventUpdated, "2026-09-29T10:00:00.000+0000")),
			modified: "2026-09-29T10:00:01.000+0000", wantUpsert: true},
		{name: "retry over FAILED", recorded: ptrState(ingestStateRow(domain.SalesforceIngestFailed, domain.SalesforceEventUpdated, "2026-09-29T10:00:00.000+0000")),
			modified: "2026-09-29T10:00:00.000+0000", wantUpsert: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cust := fullCustomer()
			cust.ID = testAccountID
			cust.LastModifiedDate = sampleStr(tc.modified)
			states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{}}
			if tc.recorded != nil {
				states.rows[domain.SalesforceIngestEntityAccount+"/"+testAccountID] = *tc.recorded
			}
			repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup(), states: states}
			svc := NewSalesforceEventService(repo, &stubSalesEntityClient{customer: cust}, SalesforceIngestSupport{States: states})

			err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
				EventType: domain.SalesforceEventUpdated, Entity: "Account", ReferenceID: testAccountID,
			})
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if (repo.upsertCalls == 1) != tc.wantUpsert {
				t.Errorf("upsert calls = %d, want upsert %v", repo.upsertCalls, tc.wantUpsert)
			}
			if tc.wantUpsert {
				want, _ := parseSalesforceLastModified(&tc.modified)
				if !repo.lastState.EventModifiedOn.Equal(want) {
					t.Errorf("ledger eventModifiedOn = %v, want %v", repo.lastState.EventModifiedOn, want)
				}
			}
		})
	}
}

// TestHandleEvent_DeletedThenRestored: DELETED soft-deletes without a Sales
// Entity read and stamps a DELETED ledger row; the RESTORED that follows
// carries the record's unchanged LastModifiedDate and must still be written
// (the guard's DELETED rule); a replay of that RESTORED is then a duplicate.
func TestHandleEvent_DeletedThenRestored(t *testing.T) {
	cust := fullCustomer()
	cust.ID = testAccountID
	cust.LastModifiedDate = sampleStr("2026-09-29T10:00:00.000+0000")
	states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{
		domain.SalesforceIngestEntityAccount + "/" + testAccountID: ingestStateRow(domain.SalesforceIngestSucceeded, domain.SalesforceEventUpdated, "2026-09-29T10:00:00.000+0000"),
	}}
	se := &stubSalesEntityClient{customer: cust}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup(), states: states}
	svc := NewSalesforceEventService(repo, se, SalesforceIngestSupport{States: states})
	event := func(eventType string) domain.SalesforceEventRequest {
		return domain.SalesforceEventRequest{EventType: eventType, Entity: "Account", ReferenceID: testAccountID}
	}

	se.err = salesentity.NotFound("salesentity: customer not found")
	if err := svc.HandleEvent(context.Background(), event(domain.SalesforceEventDeleted)); err != nil {
		t.Fatalf("DELETED: %v", err)
	}
	if se.calls != 1 || repo.deleteCalls != 1 {
		t.Fatalf("DELETED: GetCustomer calls = %d, soft-delete calls = %d, want 1 / 1", se.calls, repo.deleteCalls)
	}
	se.err = nil
	if repo.lastState.EventType != domain.SalesforceEventDeleted || repo.lastState.Status != domain.SalesforceIngestSucceeded {
		t.Errorf("DELETED ledger row = %+v", repo.lastState)
	}
	recorded := states.rows[domain.SalesforceIngestEntityAccount+"/"+testAccountID]
	if recorded.EventType != domain.SalesforceEventDeleted {
		t.Fatalf("ledger after DELETED = %+v, want event type DELETED", recorded)
	}

	if err := svc.HandleEvent(context.Background(), event(domain.SalesforceEventRestored)); err != nil {
		t.Fatalf("RESTORED: %v", err)
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("RESTORED: upsert calls = %d, want 1 (a DELETED row must not block it)", repo.upsertCalls)
	}
	if repo.lastState.EventType != domain.SalesforceEventRestored {
		t.Errorf("RESTORED ledger row = %+v", repo.lastState)
	}

	if err := svc.HandleEvent(context.Background(), event(domain.SalesforceEventUpdated)); err != nil {
		t.Fatalf("replayed UPDATED: %v", err)
	}
	if repo.upsertCalls != 1 {
		t.Errorf("replayed UPDATED: upsert calls = %d, want still 1 (duplicate)", repo.upsertCalls)
	}
}

// A DELETED for an account CSM never had is acknowledged and still recorded,
// so a later RESTORED is not blocked by anything.
func TestHandleEvent_DeletedUnknownAccountIsAcknowledged(t *testing.T) {
	states := &fakeIngestStateRepo{}
	repo := &stubSalesforceAccountRepo{states: states, deleteMissing: true}
	svc := NewSalesforceEventService(repo, &stubSalesEntityClient{err: salesentity.NotFound("salesentity: customer not found")}, SalesforceIngestSupport{States: states})

	if err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventDeleted)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if st := states.rows[domain.SalesforceIngestEntityAccount+"/001xx0000001"]; st.EventType != domain.SalesforceEventDeleted {
		t.Errorf("ledger = %+v, want a DELETED row", st)
	}
}

// A failed account write is returned (Service Bus redelivers) and recorded
// FAILED in the ledger with the error text and the record's version.
func TestHandleEvent_WriteFailureRecordsFailed(t *testing.T) {
	states := &fakeIngestStateRepo{}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup(), upsertErr: errors.New("db down")}
	svc := NewSalesforceEventService(repo, &stubSalesEntityClient{customer: fullCustomer()}, SalesforceIngestSupport{States: states})

	err := svc.HandleEvent(context.Background(), accountEvent(domain.SalesforceEventUpdated))
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("err = %v, want the write error", err)
	}
	if len(states.upserts) != 1 {
		t.Fatalf("ledger writes = %d, want 1", len(states.upserts))
	}
	got := states.upserts[0]
	want, _ := parseSalesforceLastModified(fullCustomer().LastModifiedDate)
	if got.Status != domain.SalesforceIngestFailed || got.LastError == nil || *got.LastError != "db down" || !got.EventModifiedOn.Equal(want) {
		t.Errorf("ledger row = %+v", got)
	}
}
