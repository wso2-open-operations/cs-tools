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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	testInvoiceSfID    = "a0IE200000CrbvNMAR"
	testInvoiceOppRow  = "0e000000-0000-4000-8000-000000000001"
	testInvoiceLastMod = "2026-09-18T06:37:07.000+0000"
)

type fakeInvoiceSalesEntity struct {
	inv    salesentity.Invoice
	err    error
	calls  int
	lastID string
}

func (f *fakeInvoiceSalesEntity) GetInvoice(_ context.Context, id string) (salesentity.Invoice, error) {
	f.calls++
	f.lastID = id
	return f.inv, f.err
}

type fakeInvoiceRepo struct {
	upserts     []domain.SalesforceInvoiceUpsert
	upsertState []domain.UpsertSalesforceIngestStateRequest
	upsertErr   error
	deletes     []string
	deleteState []domain.UpsertSalesforceIngestStateRequest
	deleteRows  int64
}

func (f *fakeInvoiceRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceInvoiceUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	f.upserts = append(f.upserts, row)
	f.upsertState = append(f.upsertState, state)
	return true, f.upsertErr
}

func (f *fakeInvoiceRepo) DeleteBySfID(_ context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	f.deletes = append(f.deletes, sfID)
	f.deleteState = append(f.deleteState, state)
	return f.deleteRows, nil
}

// fakeOpportunityLookup answers from byID; when ingested is set, an
// opportunity upsert through fakeOpportunityRepo makes it findable, the way
// the real ingest makes the row readable.
type fakeOpportunityLookup struct {
	byID  map[string]string
	calls []string
}

func (f *fakeOpportunityLookup) LookupOpportunityIDBySfID(_ context.Context, sfID string) (*string, error) {
	f.calls = append(f.calls, sfID)
	if id, ok := f.byID[sfID]; ok {
		return &id, nil
	}
	return nil, nil
}

// registeringOpportunityRepo is fakeOpportunityRepo that also records each
// upserted opportunity in a lookup, as the real table would.
type registeringOpportunityRepo struct {
	fakeOpportunityRepo
	lookup *fakeOpportunityLookup
}

func (r *registeringOpportunityRepo) UpsertFromSalesforce(ctx context.Context, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error) {
	res, err := r.fakeOpportunityRepo.UpsertFromSalesforce(ctx, row, state)
	if err == nil {
		r.lookup.byID[row.SfID] = testInvoiceOppRow
	}
	return res, err
}

func sampleInvoice() salesentity.Invoice {
	amount := 1500.25
	return salesentity.Invoice{
		ID: sampleStr(testInvoiceSfID), Name: sampleStr("2508"), OpportunityID: sampleStr(testOpportunitySfID),
		Amount: &amount, DueDate: sampleStr("2026-10-31"), InvoiceDate: sampleStr("2026-10-01"),
		OriginalInvoiceDueDate: sampleStr("2026-10-15"), Classification: sampleStr("Subscription"),
		Description: sampleStr("CSM Sync Test"), ServiceStartDate: sampleStr("2026-04-01"), ServiceEndDate: sampleStr("2027-03-31"),
		CurrencyCode: sampleStr("USD"), LastModifiedDate: sampleStr(testInvoiceLastMod),
	}
}

type invoiceHarness struct {
	se      *fakeInvoiceSalesEntity
	repo    *fakeInvoiceRepo
	lookup  *fakeOpportunityLookup
	oppSE   *fakeOpportunitySalesEntity
	oppRepo *registeringOpportunityRepo
	states  *fakeIngestStateRepo
	svc     *salesforceEventService
}

// newInvoiceHarness builds a service with the Opportunity and invoice
// branches on and the Account ingest off; accounts and opportunities are
// what CSM already holds.
func newInvoiceHarness(inv salesentity.Invoice, opportunities, accounts map[string]string) *invoiceHarness {
	h := &invoiceHarness{
		se:     &fakeInvoiceSalesEntity{inv: inv},
		repo:   &fakeInvoiceRepo{deleteRows: 1},
		lookup: &fakeOpportunityLookup{byID: opportunities},
		oppSE:  &fakeOpportunitySalesEntity{opp: sampleOpportunity()},
		states: &fakeIngestStateRepo{},
	}
	h.oppRepo = &registeringOpportunityRepo{lookup: h.lookup}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: &stubSalesforceAccountRepo{accountsBySfID: accounts}, States: h.states})
	svc = WithOpportunityIngest(svc, OpportunityIngest{Opportunities: h.oppRepo, SalesEntity: h.oppSE})
	h.svc = WithInvoiceIngest(svc, InvoiceIngest{Invoices: h.repo, Opportunities: h.lookup, SalesEntity: h.se}).(*salesforceEventService)
	return h
}

func invoiceEvent(entity, eventType, ref string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{Entity: entity, EventType: eventType, ReferenceID: ref}
}

func TestInvoiceIngest_DisabledIgnoresEvents(t *testing.T) {
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}})
	for _, et := range []string{"CREATED", "UPDATED", "DELETED", "RESTORED"} {
		if err := svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", et, testInvoiceSfID)); err != nil {
			t.Errorf("%s: err = %v, want nil (acknowledged)", et, err)
		}
	}
	if err := svc.(InvoiceReingester).RetryInvoiceIngest(context.Background(), testInvoiceSfID); !errors.Is(err, errInvoiceIngestDisabled) {
		t.Errorf("retry err = %v, want errInvoiceIngestDisabled", err)
	}
}

// CREATED, UPDATED and RESTORED (under either entity spelling) fetch the
// invoice, resolve its opportunity and write every sf_invoice column.
func TestInvoiceIngest_UpsertsWithOpportunity(t *testing.T) {
	for _, tc := range []struct{ entity, eventType string }{
		{"Invoice__c", "CREATED"}, {"Invoice__c", "UPDATED"}, {"Invoice", "RESTORED"},
	} {
		t.Run(tc.entity+"/"+tc.eventType, func(t *testing.T) {
			h := newInvoiceHarness(sampleInvoice(), map[string]string{testOpportunitySfID: testInvoiceOppRow}, nil)
			if err := h.svc.HandleEvent(context.Background(), invoiceEvent(tc.entity, tc.eventType, testInvoiceSfID[:15])); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if h.se.lastID != testInvoiceSfID[:15] || len(h.repo.upserts) != 1 || h.oppSE.calls != 0 {
				t.Fatalf("fetched %q upserts %d opportunity fetches %d", h.se.lastID, len(h.repo.upserts), h.oppSE.calls)
			}
			row := h.repo.upserts[0]
			day := func(p *time.Time) string {
				if p == nil {
					return ""
				}
				return p.Format(time.DateOnly)
			}
			if row.SfID != testInvoiceSfID || derefString(row.OpportunityID) != testInvoiceOppRow || derefString(row.Name) != "2508" ||
				derefString(row.Description) != "CSM Sync Test" || derefString(row.Classification) != "Subscription" ||
				row.InvoicedAmount == nil || *row.InvoicedAmount != 1500.25 || day(row.InvoiceDate) != "2026-10-01" ||
				day(row.InvoicedDueDate) != "2026-10-31" || day(row.OriginalInvoiceDueDate) != "2026-10-15" || row.InvoicedPaidDate != nil ||
				day(row.ServiceStartDate) != "2026-04-01" || day(row.ServiceEndDate) != "2027-03-31" {
				t.Errorf("row = %+v", row)
			}
			st := h.repo.upsertState[0]
			if st.Entity != domain.SalesforceIngestEntityInvoice || st.SfID != testInvoiceSfID || st.EventType != tc.eventType ||
				st.Status != domain.SalesforceIngestSucceeded || st.EventModifiedOn.Format(time.RFC3339) != "2026-09-18T06:37:07Z" {
				t.Errorf("ledger state = %+v", st)
			}
		})
	}
}

// No original due date in Salesforce: the current due date is stored as the
// original, as the ServiceNow script did.
func TestInvoiceIngest_OriginalDueDateFallsBackToDueDate(t *testing.T) {
	inv := sampleInvoice()
	inv.OriginalInvoiceDueDate = nil
	row := mapSalesEntityInvoice(context.Background(), testInvoiceSfID, inv)
	if row.OriginalInvoiceDueDate == nil || row.OriginalInvoiceDueDate.Format(time.DateOnly) != "2026-10-31" {
		t.Errorf("original due = %v, want the due date", row.OriginalInvoiceDueDate)
	}
	inv.DueDate = nil
	if row := mapSalesEntityInvoice(context.Background(), testInvoiceSfID, inv); row.OriginalInvoiceDueDate != nil {
		t.Errorf("original due = %v, want NULL with neither date", row.OriginalInvoiceDueDate)
	}
}

func TestInvoiceIngest_TruncatesVarchar40Columns(t *testing.T) {
	inv := sampleInvoice()
	long := strings.Repeat("é", 45)
	inv.Name, inv.Description, inv.Classification = &long, &long, &long
	row := mapSalesEntityInvoice(context.Background(), testInvoiceSfID, inv)
	for name, v := range map[string]*string{"name": row.Name, "description": row.Description, "classification": row.Classification} {
		if v == nil || len([]rune(*v)) != 40 {
			t.Errorf("%s = %v, want 40 characters", name, v)
		}
	}
}

// A missing opportunity is ingested inline (the _initOpportunity step), then
// the invoice is written under it.
func TestInvoiceIngest_EnsuresMissingOpportunity(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), map[string]string{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "UPDATED", testInvoiceSfID)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.oppSE.calls != 1 || len(h.oppRepo.upserts) != 1 || len(h.repo.upserts) != 1 || derefString(h.repo.upserts[0].OpportunityID) != testInvoiceOppRow {
		t.Errorf("opportunity fetches %d upserts %d invoice upserts %+v", h.oppSE.calls, len(h.oppRepo.upserts), h.repo.upserts)
	}
}

// The ledger says opportunity version V was written, but the sf_opportunity
// row is gone (a cascade or an out-of-band delete). A child event for V
// re-ingests the opportunity past the duplicate guard and writes the child;
// a plain Opportunity event for V is still skipped.
func TestInvoiceIngest_StaleOpportunityLedgerReingestsMissingRow(t *testing.T) {
	const version = "2026-09-18T06:37:07.000+0000"
	recorded, _ := time.Parse(time.RFC3339, "2026-09-18T06:37:07Z")
	succeeded := domain.UpsertSalesforceIngestStateRequest{Entity: domain.SalesforceIngestEntityOpportunity, SfID: testOpportunitySfID,
		EventModifiedOn: recorded, EventType: "UPDATED", Status: domain.SalesforceIngestSucceeded}

	h := newInvoiceHarness(sampleInvoice(), map[string]string{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	h.oppSE.opp.LastModifiedDate = sampleStr(version)
	h.states.apply(succeeded)
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "UPDATED", testInvoiceSfID)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.oppSE.calls != 1 || len(h.oppRepo.upserts) != 1 {
		t.Fatalf("opportunity fetches %d upserts %d, want the opportunity re-ingested", h.oppSE.calls, len(h.oppRepo.upserts))
	}
	if st := h.oppRepo.upsertState[0]; st.EventModifiedOn.Format(time.RFC3339) != "2026-09-18T06:37:07Z" || st.Status != domain.SalesforceIngestSucceeded {
		t.Errorf("opportunity ledger state = %+v, want version V SUCCEEDED", st)
	}
	if len(h.repo.upserts) != 1 || derefString(h.repo.upserts[0].OpportunityID) != testInvoiceOppRow {
		t.Errorf("invoice upserts %+v, want one under the re-ingested opportunity", h.repo.upserts)
	}

	// The guard still applies to the Opportunity's own events.
	g := newInvoiceHarness(sampleInvoice(), map[string]string{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	g.oppSE.opp.LastModifiedDate = sampleStr(version)
	g.states.apply(succeeded)
	if err := g.svc.HandleEvent(context.Background(), invoiceEvent("Opportunity", "UPDATED", testOpportunitySfID)); err != nil {
		t.Fatalf("Opportunity HandleEvent: %v", err)
	}
	if len(g.oppRepo.upserts) != 0 {
		t.Errorf("opportunity upserts = %d, want 0 (same version, guarded)", len(g.oppRepo.upserts))
	}
}

// The opportunity's account missing with the Account ingest off: the invoice
// fails with the "account not found" prefix the retry job re-runs, recorded
// FAILED under entity invoice, and nothing is written.
func TestInvoiceIngest_MissingAccountFailsForRetry(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), map[string]string{}, map[string]string{})
	err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "CREATED", testInvoiceSfID))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "account not found") {
		t.Fatalf("err = %v, want the account NotFoundError", err)
	}
	if len(h.repo.upserts) != 0 {
		t.Errorf("invoice upserts = %d, want 0", len(h.repo.upserts))
	}
	var failed bool
	for _, st := range h.states.upserts {
		if st.Entity == domain.SalesforceIngestEntityInvoice && st.Status == domain.SalesforceIngestFailed && strings.HasPrefix(derefString(st.LastError), "account not found") {
			failed = true
		}
	}
	if !failed {
		t.Errorf("ledger = %+v, want a FAILED invoice row", h.states.upserts)
	}
}

func TestInvoiceIngest_GuardSkipsReplay(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), map[string]string{testOpportunitySfID: testInvoiceOppRow}, nil)
	recorded, _ := time.Parse(time.RFC3339, "2026-09-18T06:37:07Z")
	h.states.apply(domain.UpsertSalesforceIngestStateRequest{Entity: domain.SalesforceIngestEntityInvoice, SfID: testInvoiceSfID,
		EventModifiedOn: recorded, EventType: "UPDATED", Status: domain.SalesforceIngestSucceeded})
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "UPDATED", testInvoiceSfID)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if len(h.repo.upserts) != 0 {
		t.Errorf("upserts = %d, want 0 (already ingested)", len(h.repo.upserts))
	}
}

func TestInvoiceIngest_DeletedHardDeletesBy18CharID(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), nil, nil)
	h.se.err = salesentity.NotFound("salesentity: invoice not found")
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "DELETED", testInvoiceSfID[:15])); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.se.calls != 1 || len(h.repo.deletes) != 1 || h.repo.deletes[0] != testInvoiceSfID {
		t.Fatalf("fetches %d deletes %v, want one confirming fetch and the 18-character id", h.se.calls, h.repo.deletes)
	}
	st := h.repo.deleteState[0]
	if st.Entity != domain.SalesforceIngestEntityInvoice || st.EventType != domain.SalesforceEventDeleted || st.Status != domain.SalesforceIngestSucceeded {
		t.Errorf("ledger = %+v", st)
	}
	// A never-ingested invoice is acknowledged.
	h.repo.deleteRows = 0
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "DELETED", testInvoiceSfID)); err != nil {
		t.Errorf("missing invoice delete: %v, want nil", err)
	}
}

func TestInvoiceIngest_EmptyFetchIsRetryable(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), nil, nil)
	h.se.err = &apierror.ServiceUnavailableError{Msg: "salesentity: invoice not found"}
	var su *apierror.ServiceUnavailableError
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "CREATED", testInvoiceSfID)); !errors.As(err, &su) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
}

// Salesforce's clock can run ahead of ours: a DELETED stamped with "now"
// alone would be older than the recorded UPDATED version, the ledger would
// keep UPDATED, and the RESTORED that follows (carrying that same version)
// would be skipped. The DELETED row takes the later of the two instead.
func TestInvoiceIngest_DeleteThenRestoreWhenSalesforceClockIsAhead(t *testing.T) {
	h := newInvoiceHarness(sampleInvoice(), map[string]string{testOpportunitySfID: testInvoiceOppRow}, nil)
	ahead := time.Now().UTC().Add(time.Hour)
	inv := sampleInvoice()
	inv.LastModifiedDate = sampleStr(ahead.Format("2006-01-02T15:04:05.000+0000"))
	h.se.inv = inv
	h.states.apply(domain.UpsertSalesforceIngestStateRequest{Entity: domain.SalesforceIngestEntityInvoice, SfID: testInvoiceSfID,
		EventModifiedOn: ahead.Truncate(time.Second), EventType: "UPDATED", Status: domain.SalesforceIngestSucceeded})

	h.se.err = salesentity.NotFound("salesentity: invoice not found")
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "DELETED", testInvoiceSfID)); err != nil {
		t.Fatalf("DELETED: %v", err)
	}
	h.se.err = nil
	if got := h.repo.deleteState[0].EventModifiedOn; got.Before(ahead.Truncate(time.Second)) {
		t.Fatalf("DELETED version %v is older than the recorded %v", got, ahead)
	}
	h.states.apply(h.repo.deleteState[0])
	if err := h.svc.HandleEvent(context.Background(), invoiceEvent("Invoice__c", "RESTORED", testInvoiceSfID)); err != nil {
		t.Fatalf("RESTORED: %v", err)
	}
	if len(h.repo.upserts) != 1 {
		t.Errorf("upserts after RESTORED = %d, want 1", len(h.repo.upserts))
	}
}
