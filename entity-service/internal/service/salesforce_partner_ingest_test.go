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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	testPartnerCustomerSfID = "0012S00002WlTCqQAN"
	testPartnerCustomerRow  = "c0000000-0000-4000-8000-000000000001"
	testPartnerASfID        = "001E000001Oa5xEIAR"
	testPartnerARow         = "a0000000-0000-4000-8000-00000000000a"
	testPartnerBSfID        = "001E000001Oa5xFIAR"
	testPartnerBRow         = "b0000000-0000-4000-8000-00000000000b"
)

type fakePartnerSalesEntity struct {
	id       string
	partners []salesentity.CustomerPartner
	err      error
	calls    []string
}

func (f *fakePartnerSalesEntity) GetCustomerPartners(_ context.Context, id string) (string, []salesentity.CustomerPartner, error) {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return "", nil, f.err
	}
	returned := f.id
	if returned == "" {
		returned = id
	}
	return returned, f.partners, nil
}

type partnerReplaceCall struct {
	customerSfID, customerID string
	partnerIDs               []string
	state                    domain.UpsertSalesforceIngestStateRequest
}

type fakePartnerWriteRepo struct {
	calls []partnerReplaceCall
	// adds records AddPartners calls (the admin resync's insert-only path).
	adds []partnerReplaceCall
	err  error
}

func (f *fakePartnerWriteRepo) AddPartners(_ context.Context, customerSfID, customerID string, partnerIDs []string, state domain.UpsertSalesforceIngestStateRequest) (int, error) {
	f.adds = append(f.adds, partnerReplaceCall{customerSfID, customerID, append([]string(nil), partnerIDs...), state})
	if f.err != nil {
		return 0, f.err
	}
	return 2 * len(partnerIDs), nil
}

func (f *fakePartnerWriteRepo) ReplacePartners(_ context.Context, customerSfID, customerID string, partnerIDs []string, state domain.UpsertSalesforceIngestStateRequest) (int, int, error) {
	f.calls = append(f.calls, partnerReplaceCall{customerSfID, customerID, append([]string(nil), partnerIDs...), state})
	if f.err != nil {
		return 0, 0, f.err
	}
	return 2 * len(partnerIDs), 0, nil
}

// customersByID answers GetCustomer from a map, so EnsureAccount can ingest
// several different accounts in one test.
type customersByID struct {
	customers map[string]salesentity.Customer
	calls     []string
}

func (c *customersByID) GetCustomer(_ context.Context, id string) (salesentity.Customer, error) {
	c.calls = append(c.calls, id)
	cust, ok := c.customers[id]
	if !ok {
		return salesentity.Customer{}, &apierror.ServiceUnavailableError{Msg: "salesentity: customer not found"}
	}
	return cust, nil
}

func partner(id string) salesentity.CustomerPartner { return salesentity.CustomerPartner{ID: id} }

// newPartnerService builds a service with the partner refresh on, the
// Account ingest off (EnsureAccount is a pure lookup) and accounts in CSM.
func newPartnerService(se *fakePartnerSalesEntity, repo *fakePartnerWriteRepo, states *fakeIngestStateRepo, accounts map[string]string) *salesforceEventService {
	lookup := &stubSalesforceAccountRepo{accountsBySfID: accounts}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: states})
	return WithPartnerIngest(svc, PartnerIngest{Partners: repo, SalesEntity: se}).(*salesforceEventService)
}

func allPartnerAccounts() map[string]string {
	return map[string]string{
		testPartnerCustomerSfID: testPartnerCustomerRow,
		testPartnerASfID:        testPartnerARow,
		testPartnerBSfID:        testPartnerBRow,
	}
}

func TestRefreshPartners_DisabledIsNoOp(t *testing.T) {
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}}).(*salesforceEventService)
	res, err := svc.RefreshPartners(context.Background(), testPartnerCustomerSfID)
	if err != nil || !res.Disabled {
		t.Fatalf("res = %+v err = %v, want Disabled and no error", res, err)
	}
	// A nil PartnerIngest (flag off) means no Sales Entity read, no write:
	// the account event and the membership hook both go through the same check.
	if err := svc.refreshPartnersAfterAccountEvent(context.Background(), testPartnerCustomerSfID); err != nil {
		t.Errorf("account hook: %v", err)
	}
	svc.refreshPartnersForMembership(context.Background(), "a0m1", partnerContactMembership())
}

// The set replace hands the repository exactly the partner accounts
// Salesforce lists — deduplicated, 15-character ids widened, the customer
// itself and blank ids dropped — and a SUCCEEDED account_partners ledger row.
func TestRefreshPartners_ReplacesTheSet(t *testing.T) {
	se := &fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{
		partner(testPartnerASfID), partner(testPartnerBSfID[:15]), partner(testPartnerASfID),
		partner(testPartnerCustomerSfID), partner("  "),
	}}
	repo := &fakePartnerWriteRepo{}
	states := &fakeIngestStateRepo{}
	svc := newPartnerService(se, repo, states, allPartnerAccounts())

	res, err := svc.RefreshPartners(context.Background(), testPartnerCustomerSfID[:15])
	if err != nil {
		t.Fatalf("RefreshPartners: %v", err)
	}
	if len(se.calls) != 1 || se.calls[0] != testPartnerCustomerSfID {
		t.Errorf("sales entity calls = %v, want the 18-character customer id", se.calls)
	}
	if len(repo.calls) != 1 {
		t.Fatalf("replace calls = %d, want 1", len(repo.calls))
	}
	call := repo.calls[0]
	if call.customerSfID != testPartnerCustomerSfID || call.customerID != testPartnerCustomerRow ||
		!reflect.DeepEqual(call.partnerIDs, []string{testPartnerARow, testPartnerBRow}) {
		t.Errorf("replace call = %+v", call)
	}
	if call.state.Entity != domain.SalesforceIngestEntityAccountPartners || call.state.SfID != testPartnerCustomerSfID ||
		call.state.Status != domain.SalesforceIngestSucceeded {
		t.Errorf("ledger state = %+v", call.state)
	}
	if !reflect.DeepEqual(res.PartnerSfIDs, []string{testPartnerASfID, testPartnerBSfID}) || res.Added != 4 {
		t.Errorf("result = %+v", res)
	}
}

// An explicit empty list from Salesforce removes every stored partner.
func TestRefreshPartners_EmptySetRemovesAll(t *testing.T) {
	repo := &fakePartnerWriteRepo{}
	svc := newPartnerService(&fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{}}, repo, &fakeIngestStateRepo{}, allPartnerAccounts())
	if _, err := svc.RefreshPartners(context.Background(), testPartnerCustomerSfID); err != nil {
		t.Fatalf("RefreshPartners: %v", err)
	}
	if len(repo.calls) != 1 || len(repo.calls[0].partnerIDs) != 0 {
		t.Fatalf("replace calls = %+v, want one with no partners", repo.calls)
	}
}

// A partner that is not in CSM, with the Account ingest off, fails the
// refresh before any write — dropping it would delete its existing link —
// with the "account not found" prefix the retry job matches.
func TestRefreshPartners_MissingPartnerFailsWithoutWriting(t *testing.T) {
	accounts := allPartnerAccounts()
	delete(accounts, testPartnerBSfID)
	repo := &fakePartnerWriteRepo{}
	states := &fakeIngestStateRepo{}
	svc := newPartnerService(&fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{partner(testPartnerASfID), partner(testPartnerBSfID)}}, repo, states, accounts)

	_, err := svc.RefreshPartners(context.Background(), testPartnerCustomerSfID)
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "account not found") || !strings.Contains(nf.Msg, testPartnerBSfID) {
		t.Fatalf("err = %v, want the missing partner's NotFoundError", err)
	}
	if len(repo.calls) != 0 {
		t.Errorf("replace calls = %d, want 0", len(repo.calls))
	}
	if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed ||
		states.upserts[0].Entity != domain.SalesforceIngestEntityAccountPartners {
		t.Errorf("ledger = %+v, want one FAILED account_partners row", states.upserts)
	}
}

func TestRefreshPartners_SalesEntityErrorLeavesTheSet(t *testing.T) {
	repo := &fakePartnerWriteRepo{}
	states := &fakeIngestStateRepo{}
	svc := newPartnerService(&fakePartnerSalesEntity{err: &apierror.ServiceUnavailableError{Msg: "salesentity: customer-search did not return partners"}}, repo, states, allPartnerAccounts())
	if _, err := svc.RefreshPartners(context.Background(), testPartnerCustomerSfID); err == nil {
		t.Fatal("err = nil, want the Sales Entity error")
	}
	if len(repo.calls) != 0 || len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed {
		t.Errorf("replace calls = %d ledger = %+v", len(repo.calls), states.upserts)
	}
}

// With the Account ingest on, a partner missing from CSM is ingested first
// (the script's syncCustomer before the link) and then linked.
func TestRefreshPartners_EnsuresMissingPartnerAccounts(t *testing.T) {
	states := &fakeIngestStateRepo{}
	accounts := &stubSalesforceAccountRepo{
		accountsBySfID:   map[string]string{testPartnerCustomerSfID: testPartnerCustomerRow},
		registerOnUpsert: testPartnerARow,
		states:           states,
	}
	customers := &customersByID{customers: map[string]salesentity.Customer{
		testPartnerASfID: {ID: testPartnerASfID, Name: sampleStr("Smart Software")},
	}}
	repo := &fakePartnerWriteRepo{}
	svc := NewSalesforceEventService(accounts, customers, SalesforceIngestSupport{Accounts: accounts, States: states})
	s := WithPartnerIngest(svc, PartnerIngest{Partners: repo, SalesEntity: &fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{partner(testPartnerASfID)}}}).(*salesforceEventService)

	if _, err := s.RefreshPartners(context.Background(), testPartnerCustomerSfID); err != nil {
		t.Fatalf("RefreshPartners: %v", err)
	}
	if accounts.upsertCalls != 1 || accounts.lastUpsert.SfID != testPartnerASfID || accounts.lastUpsert.Name != "Smart Software" {
		t.Errorf("account upserts = %d last = %+v, want the partner ingested", accounts.upsertCalls, accounts.lastUpsert)
	}
	if len(repo.calls) != 1 || !reflect.DeepEqual(repo.calls[0].partnerIDs, []string{testPartnerARow}) {
		t.Errorf("replace calls = %+v", repo.calls)
	}
}

// Every Account CREATED/UPDATED/RESTORED refreshes the account's partners,
// including a replay the duplicate guard skipped.
func TestAccountEvent_RefreshesPartnersEvenWhenGuardSkips(t *testing.T) {
	states := &fakeIngestStateRepo{}
	cust := salesentity.Customer{ID: testPartnerCustomerSfID, Name: sampleStr("Beta App"), LastModifiedDate: sampleStr("2026-09-18T06:37:07.000+0000")}
	accounts := &stubSalesforceAccountRepo{accountsBySfID: allPartnerAccounts(), states: states}
	se := &fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{partner(testPartnerASfID)}}
	repo := &fakePartnerWriteRepo{}
	svc := WithPartnerIngest(NewSalesforceEventService(accounts, &stubSalesEntityClient{customer: cust}, SalesforceIngestSupport{Accounts: accounts, States: states}),
		PartnerIngest{Partners: repo, SalesEntity: se})

	ev := domain.SalesforceEventRequest{Entity: "Account", EventType: "UPDATED", ReferenceID: testPartnerCustomerSfID}
	for i := 0; i < 2; i++ {
		if err := svc.HandleEvent(context.Background(), ev); err != nil {
			t.Fatalf("HandleEvent #%d: %v", i+1, err)
		}
	}
	if accounts.upsertCalls != 1 {
		t.Errorf("account upserts = %d, want 1 (the replay is skipped by the guard)", accounts.upsertCalls)
	}
	if len(se.calls) != 2 || len(repo.calls) != 2 {
		t.Errorf("partner reads = %d writes = %d, want 2 each", len(se.calls), len(repo.calls))
	}

	// A refresh failure fails the event so Service Bus redelivers it.
	se.err = &apierror.ServiceUnavailableError{Msg: "down"}
	if err := svc.HandleEvent(context.Background(), ev); err == nil {
		t.Error("err = nil, want the partner refresh error")
	}
}

// A customer with no name is acknowledged without an account row, so there is
// nothing to hang partner links on: no refresh.
func TestAccountEvent_NamelessCustomerSkipsPartnerRefresh(t *testing.T) {
	states := &fakeIngestStateRepo{}
	accounts := &stubSalesforceAccountRepo{states: states}
	se := &fakePartnerSalesEntity{}
	svc := WithPartnerIngest(NewSalesforceEventService(accounts, &stubSalesEntityClient{customer: salesentity.Customer{ID: testPartnerCustomerSfID}}, SalesforceIngestSupport{Accounts: accounts, States: states}),
		PartnerIngest{Partners: &fakePartnerWriteRepo{}, SalesEntity: se})
	if err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{Entity: "Account", EventType: "CREATED", ReferenceID: testPartnerCustomerSfID}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(se.calls) != 0 {
		t.Errorf("partner reads = %d, want 0", len(se.calls))
	}
}

// The partner flag off leaves the Account event exactly as before.
func TestAccountEvent_PartnerFlagOffDoesNotRead(t *testing.T) {
	states := &fakeIngestStateRepo{}
	accounts := &stubSalesforceAccountRepo{accountsBySfID: allPartnerAccounts(), states: states}
	svc := NewSalesforceEventService(accounts, &stubSalesEntityClient{customer: salesentity.Customer{ID: testPartnerCustomerSfID, Name: sampleStr("Beta App")}}, SalesforceIngestSupport{Accounts: accounts, States: states})
	if err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{Entity: "Account", EventType: "UPDATED", ReferenceID: testPartnerCustomerSfID}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	for _, st := range states.upserts {
		if st.Entity == domain.SalesforceIngestEntityAccountPartners {
			t.Errorf("partner ledger row written with the flag off: %+v", st)
		}
	}
}

// With no account lookup wired (no Accounts support and no repo) the
// refresh fails with a configuration error instead of panicking.
func TestAccountEvent_PartnerRefreshWithoutLookupErrors(t *testing.T) {
	svc := WithPartnerIngest(NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}}),
		PartnerIngest{Partners: &fakePartnerWriteRepo{}, SalesEntity: &fakePartnerSalesEntity{}}).(*salesforceEventService)
	if err := svc.refreshPartnersAfterAccountEvent(context.Background(), testPartnerCustomerSfID); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("err = %v, want the account lookup configuration error", err)
	}
}

func partnerContactMembership() salesentity.ProjectContact {
	pc := sampleProjectContact(domain.MembershipStateInvited, "Portal user")
	pc.Contact.CustomerID = sampleStr(testPartnerASfID)
	pc.Subscription.CustomerID = sampleStr(testPartnerCustomerSfID)
	return pc
}

// A partner contact's membership (contact account != project account)
// refreshes the project account's partners; an own contact's does not.
func TestMembershipIngest_PartnerContactRefreshesPartners(t *testing.T) {
	for name, tc := range map[string]struct {
		pc        salesentity.ProjectContact
		wantCalls []string
	}{
		"partner contact": {partnerContactMembership(), []string{testPartnerCustomerSfID}},
		"own contact":     {sampleProjectContact(domain.MembershipStateInvited, "Portal user"), nil},
	} {
		t.Run(name, func(t *testing.T) {
			h := newIngestHarness(tc.pc, sampleContact(), false)
			for k, v := range allPartnerAccounts() {
				h.accounts.accountsBySfID[k] = v
			}
			se := &fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{partner(testPartnerASfID)}}
			h.svc = WithPartnerIngest(h.svc, PartnerIngest{Partners: &fakePartnerWriteRepo{}, SalesEntity: se})
			if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if !reflect.DeepEqual(se.calls, tc.wantCalls) {
				t.Errorf("partner reads = %v, want %v", se.calls, tc.wantCalls)
			}
		})
	}
}

// The membership is the event's subject: a failed partner refresh is logged
// and recorded, never returned.
func TestMembershipIngest_PartnerRefreshFailureDoesNotFailTheMembership(t *testing.T) {
	h := newIngestHarness(partnerContactMembership(), sampleContact(), false)
	se := &fakePartnerSalesEntity{err: &apierror.ServiceUnavailableError{Msg: "down"}}
	h.svc = WithPartnerIngest(h.svc, PartnerIngest{Partners: &fakePartnerWriteRepo{}, SalesEntity: se})
	if err := h.svc.HandleEvent(context.Background(), membershipEvent("UPDATED", "Project_Contact__c")); err != nil {
		t.Fatalf("HandleEvent: %v, want nil", err)
	}
	if len(h.repo.upserts) != 1 || len(se.calls) != 1 {
		t.Errorf("membership upserts = %d partner reads = %d, want 1 and 1", len(h.repo.upserts), len(se.calls))
	}
	found := false
	for _, st := range h.states.upserts {
		if st.Entity == domain.SalesforceIngestEntityAccountPartners && st.Status == domain.SalesforceIngestFailed {
			found = true
		}
	}
	if !found {
		t.Errorf("ledger = %+v, want a FAILED account_partners row", h.states.upserts)
	}
}

func TestPartnerRefreshService_InternalCallersOnly(t *testing.T) {
	repo := &fakePartnerWriteRepo{}
	refresher := newPartnerService(&fakePartnerSalesEntity{partners: []salesentity.CustomerPartner{}}, repo, &fakeIngestStateRepo{}, allPartnerAccounts())

	_, err := NewPartnerRefreshService(refresher, stubAccess{scope: AccessScope{}}).Refresh(context.Background(), testPartnerCustomerSfID)
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || len(repo.calls) != 0 {
		t.Fatalf("err = %v replace calls = %d, want Forbidden and no write", err, len(repo.calls))
	}
	res, err := NewPartnerRefreshService(refresher, stubAccess{scope: AccessScope{Unrestricted: true}}).Refresh(context.Background(), testPartnerCustomerSfID)
	if err != nil || res.CustomerSfID != testPartnerCustomerSfID || len(repo.calls) != 1 {
		t.Fatalf("res = %+v err = %v calls = %d", res, err, len(repo.calls))
	}
}
