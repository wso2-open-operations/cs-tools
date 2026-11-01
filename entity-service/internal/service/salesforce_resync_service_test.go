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
	"strings"
	"sync"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	resyncAccountSfID = "001xx0000001"
	resyncAccountRow  = "a1000000-0000-4000-8000-000000000001"
	resyncLastMod     = "2026-10-01T10:00:00.000+0000"
)

// fakeResyncSearch answers the resync's list searches from memory and counts
// the calls. oppFn, when set, answers SearchOpportunities instead of opps.
type fakeResyncSearch struct {
	mu    sync.Mutex
	opps  []salesentity.Opportunity
	oppFn func(f salesentity.SearchFilter) ([]salesentity.Opportunity, error)
	calls int
}

func (f *fakeResyncSearch) count() { f.mu.Lock(); f.calls++; f.mu.Unlock() }

func (f *fakeResyncSearch) SearchOpportunities(_ context.Context, filter salesentity.SearchFilter) ([]salesentity.Opportunity, error) {
	f.count()
	if f.oppFn != nil {
		return f.oppFn(filter)
	}
	return f.opps, nil
}
func (f *fakeResyncSearch) SearchProjects(context.Context, salesentity.SearchFilter) ([]salesentity.Project, error) {
	f.count()
	return nil, nil
}
func (f *fakeResyncSearch) SearchLinkedOpportunities(context.Context, salesentity.SearchFilter) ([]salesentity.LinkedOpportunity, error) {
	f.count()
	return nil, nil
}
func (f *fakeResyncSearch) SearchInvoices(context.Context, salesentity.SearchFilter) ([]salesentity.Invoice, error) {
	f.count()
	return nil, nil
}
func (f *fakeResyncSearch) SearchContacts(context.Context, salesentity.SearchFilter) ([]salesentity.Contact, error) {
	f.count()
	return nil, nil
}
func (f *fakeResyncSearch) SearchProjectContacts(context.Context, salesentity.SearchFilter) ([]salesentity.ProjectContact, error) {
	f.count()
	return nil, nil
}

// fakeResyncReads is the stored side, read-only by construction.
type fakeResyncReads struct {
	target   repository.SalesforceResyncTarget
	accounts []repository.SalesforceStoredRow
	partners []repository.SalesforceStoredRow
	opps     []repository.SalesforceStoredRow
	items    []repository.SalesforceStoredRow
}

func (f *fakeResyncReads) TargetAccount(context.Context, string) (repository.SalesforceResyncTarget, error) {
	return f.target, nil
}
func (f *fakeResyncReads) TargetProject(context.Context, string) (repository.SalesforceResyncTarget, error) {
	return f.target, nil
}
func (f *fakeResyncReads) Accounts(context.Context, []string) ([]repository.SalesforceStoredRow, error) {
	return f.accounts, nil
}
func (f *fakeResyncReads) Partners(context.Context, string) ([]repository.SalesforceStoredRow, error) {
	return f.partners, nil
}
func (f *fakeResyncReads) Projects(context.Context, []string, []string, string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) Opportunities(context.Context, []string, string, string) ([]repository.SalesforceStoredRow, error) {
	return f.opps, nil
}
func (f *fakeResyncReads) LineItems(context.Context, []string) ([]repository.SalesforceStoredRow, error) {
	return f.items, nil
}
func (f *fakeResyncReads) Links(context.Context, []string, []string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) Invoices(context.Context, []string, []string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) ContactUsers(context.Context, []string, []string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) AccountContacts(context.Context, []string, string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) Memberships(context.Context, []string, []string) ([]repository.SalesforceStoredRow, error) {
	return nil, nil
}
func (f *fakeResyncReads) MembershipGroups(context.Context, []string) (map[string][]string, error) {
	return nil, nil
}

type resyncHarness struct {
	svc      *salesforceResyncService
	ev       *salesforceEventService
	search   *fakeResyncSearch
	reads    *fakeResyncReads
	states   *fakeIngestStateRepo
	oppRepo  *fakeOpportunityRepo
	oppSE    *fakeOpportunitySalesEntity
	accounts *stubSalesforceAccountRepo
	partRepo *fakePartnerWriteRepo
	partSE   *fakePartnerSalesEntity
	custSE   *stubSalesEntityClient
}

func resyncOpp(id, name string) salesentity.Opportunity {
	o := sampleOpportunity()
	o.ID = id
	o.Name = sampleStr(name)
	o.CustomerID = resyncAccountSfID
	o.LastModifiedDate = sampleStr(resyncLastMod)
	for i := range o.SubscriptionLineItems {
		o.SubscriptionLineItems[i].ID = sampleStr("00k" + id)
	}
	return o
}

// storedOpportunityRow is the row the ingest would have written for opp.
func storedOpportunityRow(opp salesentity.Opportunity) repository.SalesforceStoredRow {
	row := mapSalesEntityOpportunity(context.Background(), opp)
	return repository.SalesforceStoredRow{Key: opp.ID, Cols: map[string]*string{
		"name": row.Name, "stage": row.Stage, "is_won": resyncBoolText(row.IsWon), "close_date": resyncDateText(row.CloseDate),
		"eula_version": row.EulaVersion, "eula_version_decimal": row.EulaVersionDecimal, "account_sf_id": sampleStr(opp.CustomerID),
	}}
}

func storedLineItemRows(opp salesentity.Opportunity) []repository.SalesforceStoredRow {
	var out []repository.SalesforceStoredRow
	for _, li := range mapSalesEntityOpportunity(context.Background(), opp).LineItems {
		out = append(out, repository.SalesforceStoredRow{Key: li.LineItemSfID, Cols: map[string]*string{
			"opportunity_sf_id": sampleStr(opp.ID),
			"name":              li.Name, "quantity": resyncNumText(li.Quantity), "service_start_date": resyncDateText(li.ServiceStartDate),
			"service_end_date": resyncDateText(li.ServiceEndDate), "classification": li.Classification, "environment": li.Environment,
			"total_price": resyncNumText(li.TotalPrice), "product_name": li.ProductName, "product_code": li.ProductCode,
			"product_description": li.ProductDescription, "product_family": li.ProductFamily, "product_unit": li.ProductUnit,
			"eng_product_code": li.EngProductCode, "product_sf_id": li.ProductSfID,
		}})
	}
	return out
}

// newResyncHarness wires an ingest service with the Account, Opportunity and
// partner branches on and the project, link, invoice and membership branches
// off, so those components are the "ingest disabled" cases.
func newResyncHarness(t *testing.T, opps ...salesentity.Opportunity) *resyncHarness {
	t.Helper()
	h := &resyncHarness{
		search:   &fakeResyncSearch{opps: opps},
		reads:    &fakeResyncReads{target: repository.SalesforceResyncTarget{SfID: resyncAccountSfID, Name: sampleStr("Acme")}},
		states:   &fakeIngestStateRepo{},
		oppRepo:  &fakeOpportunityRepo{},
		oppSE:    &fakeOpportunitySalesEntity{},
		custSE:   &stubSalesEntityClient{customer: sampleCustomer()},
		partRepo: &fakePartnerWriteRepo{},
		partSE:   &fakePartnerSalesEntity{},
	}
	h.accounts = &stubSalesforceAccountRepo{
		states:         h.states,
		lookup:         map[string]string{"owner@example.com": "user-1"},
		accountsBySfID: map[string]string{resyncAccountSfID: resyncAccountRow, testOppAccountSfID: resyncAccountRow, testPartnerASfID: testPartnerARow},
	}
	var svc SalesforceEventService = NewSalesforceEventService(h.accounts, h.custSE, SalesforceIngestSupport{Accounts: h.accounts, States: h.states})
	svc = WithOpportunityIngest(svc, OpportunityIngest{Opportunities: h.oppRepo, SalesEntity: h.oppSE})
	svc = WithPartnerIngest(svc, PartnerIngest{Partners: h.partRepo, SalesEntity: h.partSE})
	h.ev = svc.(*salesforceEventService)
	h.svc = &salesforceResyncService{ev: h.ev, search: h.search, reads: h.reads, access: internalAccess{}, adminRoles: []string{"csm-admin"}, running: map[string]struct{}{}}
	return h
}

func resyncAdminCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@example.com", Roles: []string{"csm-admin"}})
}

func resyncPreviewReq() domain.SalesforceResyncRequest {
	return domain.SalesforceResyncRequest{Scope: domain.SalesforceResyncScopeAccount, ID: resyncAccountRow}
}

func resyncComponent(t *testing.T, comps []domain.SalesforceResyncComponent, key string) domain.SalesforceResyncComponent {
	t.Helper()
	for _, c := range comps {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("no component %q in %+v", key, comps)
	return domain.SalesforceResyncComponent{}
}

func TestResyncPreview_OpportunityStates(t *testing.T) {
	inSync, differs, missing := resyncOpp("006inSync0000000AAA", "In sync"), resyncOpp("006differs000000AAA", "Differs"), resyncOpp("006missing000000AAA", "Missing")
	only := resyncOpp("006onlyCsm0000000AAA", "Only in CSM")
	h := newResyncHarness(t, inSync, differs, missing)
	differsRow := storedOpportunityRow(differs)
	differsRow.Cols["stage"] = sampleStr("Open")
	h.reads.opps = []repository.SalesforceStoredRow{storedOpportunityRow(inSync), differsRow, storedOpportunityRow(only)}
	h.reads.items = append(storedLineItemRows(inSync), storedLineItemRows(differs)...)

	res, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if res.Target.SfID != resyncAccountSfID || res.Target.Scope != "account" || res.Capped {
		t.Errorf("target/capped = %+v %v", res.Target, res.Capped)
	}
	opps := resyncComponent(t, res.Components, domain.SalesforceResyncComponentOpportunities)
	if opps.Status != domain.SalesforceResyncStatusDiffers || opps.Counts != (domain.SalesforceResyncCounts{InSync: 1, Differs: 1, MissingInCSM: 1, OnlyInCSM: 1}) {
		t.Fatalf("opportunities = %+v", opps)
	}
	states := map[string]string{}
	for _, r := range opps.Records {
		states[r.SfID] = r.State
	}
	want := map[string]string{
		differs.ID: domain.SalesforceResyncRecordDiffers, missing.ID: domain.SalesforceResyncRecordMissingInCSM, only.ID: domain.SalesforceResyncRecordOnlyInCSM,
	}
	for id, st := range want {
		if states[id] != st {
			t.Errorf("record %s state = %q, want %q", id, states[id], st)
		}
	}
	if _, ok := states[inSync.ID]; ok {
		t.Errorf("an in-sync record must not be listed: %+v", opps.Records)
	}
	for _, r := range opps.Records {
		if r.SfID == differs.ID && (len(r.Fields) != 1 || r.Fields[0].Field != "stage" || derefString(r.Fields[0].CsmValue) != "Open") {
			t.Errorf("differs fields = %+v", r.Fields)
		}
	}
}

func TestResyncPreview_OnlyInCSMAloneIsInSync(t *testing.T) {
	h := newResyncHarness(t)
	h.reads.opps = []repository.SalesforceStoredRow{storedOpportunityRow(resyncOpp("006onlyCsm0000000AAA", "Only in CSM"))}
	res, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatal(err)
	}
	c := resyncComponent(t, res.Components, domain.SalesforceResyncComponentOpportunities)
	if c.Status != domain.SalesforceResyncStatusInSync || c.Counts.OnlyInCSM != 1 {
		t.Errorf("a row only CSM holds is informational: %+v", c)
	}
}

func TestResyncPreview_WritesNothing(t *testing.T) {
	opp := resyncOpp("006differs000000AAA", "Differs")
	h := newResyncHarness(t, opp)
	h.partSE.partners = []salesentity.CustomerPartner{partner(testPartnerASfID)}
	if _, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq()); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if n := len(h.states.upserts); n != 0 {
		t.Errorf("ledger upserts = %d, want 0 (preview must not even write the ledger)", n)
	}
	if h.states.getCalls != 0 {
		t.Errorf("ledger reads = %d, want 0", h.states.getCalls)
	}
	if len(h.oppRepo.upserts) != 0 || len(h.oppRepo.deletes) != 0 {
		t.Errorf("opportunity writes = %d upserts, %d deletes", len(h.oppRepo.upserts), len(h.oppRepo.deletes))
	}
	if h.accounts.upsertCalls != 0 || h.accounts.deleteCalls != 0 {
		t.Errorf("account writes = %d upserts, %d deletes", h.accounts.upsertCalls, h.accounts.deleteCalls)
	}
	if len(h.partRepo.calls) != 0 || len(h.partRepo.adds) != 0 {
		t.Errorf("partner writes = %d replaces, %d adds", len(h.partRepo.calls), len(h.partRepo.adds))
	}
}

func TestResyncPreview_DisabledComponentsAreSkipped(t *testing.T) {
	h := newResyncHarness(t)
	res, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		domain.SalesforceResyncComponentProjects, domain.SalesforceResyncComponentLinkedOpportunities,
		domain.SalesforceResyncComponentInvoices, domain.SalesforceResyncComponentContacts, domain.SalesforceResyncComponentMemberships,
	} {
		c := resyncComponent(t, res.Components, key)
		if c.Status != domain.SalesforceResyncStatusSkipped || c.SkippedReason == nil || !strings.Contains(*c.SkippedReason, "disabled") {
			t.Errorf("%s = %+v, want skipped with a disabled reason", key, c)
		}
	}
	if c := resyncComponent(t, res.Components, domain.SalesforceResyncComponentOpportunities); c.Status == domain.SalesforceResyncStatusSkipped {
		t.Errorf("opportunities is enabled but skipped: %+v", c)
	}

	// A nil ingest struct is the same: opportunities off.
	h.ev.opportunity = nil
	res, err = h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatal(err)
	}
	if c := resyncComponent(t, res.Components, domain.SalesforceResyncComponentOpportunities); c.Status != domain.SalesforceResyncStatusSkipped {
		t.Errorf("opportunities with a nil ingest = %+v, want skipped", c)
	}

	// And apply reports it too.
	ares, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities", "invoices"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ares.Components) != 2 {
		t.Fatalf("apply components = %+v", ares.Components)
	}
	for _, c := range ares.Components {
		if c.Status != domain.SalesforceResyncApplySkipped || c.SkippedReason == nil {
			t.Errorf("apply %s = %+v, want skipped", c.Key, c)
		}
	}
}

func TestResyncPreview_CallCap(t *testing.T) {
	h := newResyncHarness(t)
	page := 0
	h.search.oppFn = func(f salesentity.SearchFilter) ([]salesentity.Opportunity, error) {
		if f.Limit != resyncOpportunityPageSize {
			t.Errorf("opportunities asked %d per call, want %d", f.Limit, resyncOpportunityPageSize)
		}
		rows := make([]salesentity.Opportunity, f.Limit)
		for i := range rows {
			page++
			rows[i] = resyncOpp(fmt.Sprintf("006%015d", page), "Opp")
		}
		return rows, nil // always a full page: the list never ends
	}
	res, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !res.Capped || res.CappedReason == nil || !strings.Contains(*res.CappedReason, "200") {
		t.Fatalf("capped = %v reason = %v", res.Capped, res.CappedReason)
	}
	c := resyncComponent(t, res.Components, domain.SalesforceResyncComponentOpportunities)
	if c.Status != domain.SalesforceResyncStatusSkipped || c.SkippedReason == nil || len(c.Records) != 0 {
		t.Errorf("a capped component is skipped, never a partial diff: %+v", c)
	}
	// 200 calls in all: the account read and the partner read took two.
	if h.search.calls != resyncMaxSalesforceCalls-2 {
		t.Errorf("list search calls = %d, want %d", h.search.calls, resyncMaxSalesforceCalls-2)
	}
}

func TestResyncApply_OnlyRequestedComponentsBypassGuardNeverDelete(t *testing.T) {
	a, b := resyncOpp("006aaaaaaaaaaaaAAA", "A"), resyncOpp("006bbbbbbbbbbbbAAA", "B")
	h := newResyncHarness(t, a, b)
	h.oppSE.opp = a
	// The ledger already holds A's current version: the event path skips it.
	mod, _ := parseSalesforceLastModified(sampleStr(resyncLastMod))
	h.states.rows = map[string]domain.SalesforceIngestState{
		domain.SalesforceIngestEntityOpportunity + "/" + a.ID: {Entity: domain.SalesforceIngestEntityOpportunity, SfID: a.ID, Status: domain.SalesforceIngestSucceeded, EventType: domain.SalesforceEventUpdated, EventModifiedOn: mod},
	}

	res, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities"}})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Components) != 1 || res.Components[0].Key != "opportunities" || res.Components[0].Status != domain.SalesforceResyncApplyApplied ||
		res.Components[0].Succeeded != 2 || res.Components[0].Failed != 0 {
		t.Fatalf("apply = %+v", res.Components)
	}
	if len(h.oppRepo.upserts) != 2 {
		t.Fatalf("upserts = %d, want 2 (the guard is bypassed on apply)", len(h.oppRepo.upserts))
	}
	for _, row := range h.oppRepo.upserts {
		if !row.KeepStaleLineItems {
			t.Errorf("apply must not delete stale line items: %+v", row.SfID)
		}
	}
	if len(h.oppRepo.deletes) != 0 {
		t.Errorf("deletes = %v", h.oppRepo.deletes)
	}
	// Components that were not requested are untouched.
	if h.accounts.upsertCalls != 0 || len(h.partRepo.calls)+len(h.partRepo.adds) != 0 || h.custSE.calls != 0 || len(h.partSE.calls) != 0 {
		t.Errorf("unrequested components ran: accounts=%d partners=%d/%d customerReads=%d partnerReads=%d",
			h.accounts.upsertCalls, len(h.partRepo.calls), len(h.partRepo.adds), h.custSE.calls, len(h.partSE.calls))
	}
	if h.oppSE.calls != 0 {
		t.Errorf("apply re-read %d opportunities one by one; it must use the list it already read", h.oppSE.calls)
	}

	// The normal event path is unchanged: the same ledger row still skips.
	before := len(h.oppRepo.upserts)
	if err := h.ev.HandleEvent(context.Background(), domain.SalesforceEventRequest{Entity: "Opportunity", EventType: "UPDATED", ReferenceID: a.ID}); err != nil {
		t.Fatal(err)
	}
	if len(h.oppRepo.upserts) != before {
		t.Errorf("the event path wrote a version the ledger already holds")
	}
}

func TestResyncApply_PartnersOnlyAdd(t *testing.T) {
	h := newResyncHarness(t)
	h.partSE.partners = []salesentity.CustomerPartner{partner(testPartnerASfID)}
	res, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"partners"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Components) != 1 || res.Components[0].Status != domain.SalesforceResyncApplyApplied || res.Components[0].Succeeded != 1 {
		t.Fatalf("apply = %+v", res.Components)
	}
	if len(h.partRepo.calls) != 0 || len(h.partRepo.adds) != 1 {
		t.Errorf("replace calls = %d (must be 0), add calls = %d (want 1)", len(h.partRepo.calls), len(h.partRepo.adds))
	}
}

func TestResyncApply_AccountAndFailureReporting(t *testing.T) {
	h := newResyncHarness(t, resyncOpp("006aaaaaaaaaaaaAAA", "A"))
	h.oppRepo.upsertErr = errors.New("connection reset: password=hunter2")
	res, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities", "account"}})
	if err != nil {
		t.Fatal(err)
	}
	// Applied in the fixed order, parents first, whatever the request order.
	if len(res.Components) != 2 || res.Components[0].Key != "account" || res.Components[1].Key != "opportunities" {
		t.Fatalf("order = %+v", res.Components)
	}
	if res.Components[0].Status != domain.SalesforceResyncApplyApplied || h.accounts.upsertCalls != 1 {
		t.Errorf("account = %+v upserts=%d", res.Components[0], h.accounts.upsertCalls)
	}
	opp := res.Components[1]
	if opp.Status != domain.SalesforceResyncApplyFailed || opp.Failed != 1 || len(opp.Errors) != 1 || opp.Errors[0].SfID != "006aaaaaaaaaaaaAAA" {
		t.Fatalf("opportunities = %+v", opp)
	}
	if strings.Contains(opp.Errors[0].Message, "hunter2") {
		t.Errorf("an unexpected error's detail must stay in the log: %q", opp.Errors[0].Message)
	}
}

func TestResyncRejectsConcurrentRunOnSameTarget(t *testing.T) {
	h := newResyncHarness(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	h.search.oppFn = func(salesentity.SearchFilter) ([]salesentity.Opportunity, error) {
		once.Do(func() { close(entered) })
		<-release
		return nil, nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
		done <- err
	}()
	<-entered

	var conflict *apierror.ConflictError
	if _, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq()); !errors.As(err, &conflict) {
		t.Errorf("concurrent preview err = %v, want a ConflictError", err)
	}
	if _, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities"}}); !errors.As(err, &conflict) {
		t.Errorf("concurrent apply err = %v, want a ConflictError", err)
	}
	// A different target is not blocked by it (the lock is per target).
	if unlock, err := h.svc.lock("account", "a2000000-0000-4000-8000-000000000002"); err != nil {
		t.Errorf("another target was blocked: %v", err)
	} else {
		unlock()
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first preview: %v", err)
	}
	if _, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq()); err != nil {
		t.Errorf("the lock was not released: %v", err)
	}
}

func TestResyncAdminOnly(t *testing.T) {
	h := newResyncHarness(t, resyncOpp("006aaaaaaaaaaaaAAA", "A"))
	apply := domain.SalesforceResyncApplyRequest{Scope: "account", ID: resyncAccountRow, Components: []string{"opportunities"}}
	cases := []struct {
		name string
		ctx  context.Context
		svc  *salesforceResyncService
	}{
		{"internal user without an admin role", auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com", Roles: []string{"csm-user"}}), h.svc},
		{"service credential (no user token)", auth.WithIdentity(context.Background(), auth.Identity{Validated: true}), h.svc},
		{"no identity", context.Background(), h.svc},
		{"external caller even with the role", resyncAdminCtx(), &salesforceResyncService{ev: h.ev, search: h.search, reads: h.reads, access: customerAccess{}, adminRoles: []string{"csm-admin"}, running: map[string]struct{}{}}},
		{"no admin roles configured", resyncAdminCtx(), &salesforceResyncService{ev: h.ev, search: h.search, reads: h.reads, access: internalAccess{}, running: map[string]struct{}{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var forbidden *apierror.ForbiddenError
			if _, err := tc.svc.Preview(tc.ctx, resyncPreviewReq()); !errors.As(err, &forbidden) {
				t.Errorf("Preview err = %v, want Forbidden", err)
			}
			if _, err := tc.svc.Apply(tc.ctx, apply); !errors.As(err, &forbidden) {
				t.Errorf("Apply err = %v, want Forbidden", err)
			}
		})
	}
	if h.search.calls != 0 || len(h.oppRepo.upserts) != 0 {
		t.Errorf("a rejected caller reached Salesforce (%d calls) or the database (%d upserts)", h.search.calls, len(h.oppRepo.upserts))
	}
	if _, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq()); err != nil {
		t.Errorf("an admin was rejected: %v", err)
	}
}

func TestResyncValidation(t *testing.T) {
	h := newResyncHarness(t)
	var invalid *apierror.ValidationError
	for name, call := range map[string]func() error{
		"unknown scope": func() error {
			_, err := h.svc.Preview(resyncAdminCtx(), domain.SalesforceResyncRequest{Scope: "case", ID: "x"})
			return err
		},
		"empty id": func() error {
			_, err := h.svc.Preview(resyncAdminCtx(), domain.SalesforceResyncRequest{Scope: "account"})
			return err
		},
		"component of another scope": func() error {
			_, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "project", ID: "x", Components: []string{"contacts"}})
			return err
		},
		"no components": func() error {
			_, err := h.svc.Apply(resyncAdminCtx(), domain.SalesforceResyncApplyRequest{Scope: "account", ID: "x"})
			return err
		},
	} {
		if err := call(); !errors.As(err, &invalid) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
	}
	if h.search.calls != 0 || len(h.oppRepo.upserts) != 0 {
		t.Errorf("invalid requests reached Salesforce or the database")
	}
}

func TestResyncPreviewJSONContract(t *testing.T) {
	h := newResyncHarness(t, resyncOpp("006aaaaaaaaaaaaAAA", "A"))
	res, err := h.svc.Preview(resyncAdminCtx(), resyncPreviewReq())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"target", "capped", "cappedReason", "components"} {
		if _, ok := m[k]; !ok {
			t.Errorf("response has no %q: %s", k, raw)
		}
	}
	for _, k := range []string{"scope", "id", "sfId", "name"} {
		if _, ok := m["target"].(map[string]any)[k]; !ok {
			t.Errorf("target has no %q", k)
		}
	}
	comp := m["components"].([]any)[0].(map[string]any)
	for _, k := range []string{"key", "label", "status", "skippedReason", "counts", "records"} {
		if _, ok := comp[k]; !ok {
			t.Errorf("component has no %q", k)
		}
	}
	for _, k := range []string{"inSync", "differs", "missingInCsm", "onlyInCsm"} {
		if _, ok := comp["counts"].(map[string]any)[k]; !ok {
			t.Errorf("counts has no %q", k)
		}
	}
	if recs, ok := comp["records"].([]any); !ok || recs == nil {
		t.Errorf("records must be an array, got %T", comp["records"])
	}
}

func TestResyncBudgetStopsNearTheRequestDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(resyncAdminCtx(), resyncTimeMargin/2)
	defer cancel()
	b := &callBudget{max: 10}
	if err := b.spend(ctx); !errors.Is(err, errResyncCapped) || b.reason == "" || b.used != 0 {
		t.Errorf("spend near the deadline: err=%v used=%d reason=%q", err, b.used, b.reason)
	}
}
