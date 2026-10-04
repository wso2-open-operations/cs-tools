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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const testLineItemSfID = "00kE200000EB5HFIA1"

type fakeLineItemSalesEntity struct {
	li     salesentity.SubscriptionLineItem
	err    error
	calls  int
	lastID string
}

func (f *fakeLineItemSalesEntity) GetOpportunityLineItem(_ context.Context, id string) (salesentity.SubscriptionLineItem, error) {
	f.calls++
	f.lastID = id
	return f.li, f.err
}

type lineItemUpsertCall struct {
	oppSfID, oppID string
	row            domain.SalesforceOpportunityLineItemUpsert
	state          domain.UpsertSalesforceIngestStateRequest
}

type fakeLineItemRepo struct {
	upserts     []lineItemUpsertCall
	deletes     []string
	deleteState []domain.UpsertSalesforceIngestStateRequest
	deleteRows  int64
}

func (f *fakeLineItemRepo) UpsertFromSalesforce(_ context.Context, oppSfID, oppID string, li domain.SalesforceOpportunityLineItemUpsert, state domain.UpsertSalesforceIngestStateRequest) (bool, error) {
	f.upserts = append(f.upserts, lineItemUpsertCall{oppSfID, oppID, li, state})
	return true, nil
}

func (f *fakeLineItemRepo) DeleteByLineItemSfID(_ context.Context, id string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	f.deletes = append(f.deletes, id)
	f.deleteState = append(f.deleteState, state)
	return f.deleteRows, nil
}

func sampleStandaloneLineItem() salesentity.SubscriptionLineItem {
	li := sampleOpportunity().SubscriptionLineItems[0]
	li.LastModifiedDate = sampleStr(testInvoiceLastMod)
	return li
}

type lineItemHarness struct {
	se      *fakeLineItemSalesEntity
	repo    *fakeLineItemRepo
	lookup  *fakeOpportunityLookup
	oppSE   *fakeOpportunitySalesEntity
	oppRepo *registeringOpportunityRepo
	states  *fakeIngestStateRepo
	svc     *salesforceEventService
}

func newLineItemHarness(li salesentity.SubscriptionLineItem, opportunities, accounts map[string]string) *lineItemHarness {
	h := &lineItemHarness{
		se:     &fakeLineItemSalesEntity{li: li},
		repo:   &fakeLineItemRepo{deleteRows: 1},
		lookup: &fakeOpportunityLookup{byID: opportunities},
		oppSE:  &fakeOpportunitySalesEntity{opp: sampleOpportunity()},
		states: &fakeIngestStateRepo{},
	}
	h.oppRepo = &registeringOpportunityRepo{lookup: h.lookup}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: &stubSalesforceAccountRepo{accountsBySfID: accounts}, States: h.states})
	svc = WithOpportunityIngest(svc, OpportunityIngest{Opportunities: h.oppRepo, SalesEntity: h.oppSE})
	h.svc = WithOpportunityLineItemIngest(svc, OpportunityLineItemIngest{LineItems: h.repo, Opportunities: h.lookup, SalesEntity: h.se}).(*salesforceEventService)
	return h
}

func lineItemEvent(eventType, ref string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{Entity: "OpportunityLineItem", EventType: eventType, ReferenceID: ref}
}

func TestLineItemIngest_DisabledIgnoresEvents(t *testing.T) {
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}})
	for _, et := range []string{"CREATED", "UPDATED", "DELETED", "RESTORED"} {
		if err := svc.HandleEvent(context.Background(), lineItemEvent(et, testLineItemSfID)); err != nil {
			t.Errorf("%s: err = %v, want nil (acknowledged)", et, err)
		}
	}
	if err := svc.(OpportunityLineItemReingester).RetryOpportunityLineItemIngest(context.Background(), testLineItemSfID); !errors.Is(err, errOpportunityLineItemIngestDisabled) {
		t.Errorf("retry err = %v, want errOpportunityLineItemIngestDisabled", err)
	}
}

// CREATED, UPDATED and RESTORED write the line item under its opportunity,
// with exactly the derived path's mapping and a ledger row of its own.
func TestLineItemIngest_UpsertsUnderOpportunity(t *testing.T) {
	for _, et := range []string{"CREATED", "UPDATED", "RESTORED"} {
		t.Run(et, func(t *testing.T) {
			h := newLineItemHarness(sampleStandaloneLineItem(), map[string]string{testOpportunitySfID: testInvoiceOppRow}, nil)
			if err := h.svc.HandleEvent(context.Background(), lineItemEvent(et, testLineItemSfID[:15])); err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if len(h.repo.upserts) != 1 || h.oppSE.calls != 0 {
				t.Fatalf("upserts %d opportunity fetches %d", len(h.repo.upserts), h.oppSE.calls)
			}
			call := h.repo.upserts[0]
			want := mapSalesEntityLineItem(context.Background(), testLineItemSfID, sampleStandaloneLineItem())
			if call.oppSfID != testOpportunitySfID || call.oppID != testInvoiceOppRow || call.row.LineItemSfID != testLineItemSfID ||
				derefString(call.row.ProductName) != derefString(want.ProductName) || derefString(call.row.EngProductCode) != "DEVSUP" {
				t.Errorf("upsert = %+v", call)
			}
			if call.state.Entity != domain.SalesforceIngestEntityOpportunityLineItem || call.state.SfID != testLineItemSfID ||
				call.state.EventType != et || call.state.Status != domain.SalesforceIngestSucceeded {
				t.Errorf("ledger = %+v", call.state)
			}
		})
	}
}

func TestLineItemIngest_EnsuresMissingOpportunity(t *testing.T) {
	h := newLineItemHarness(sampleStandaloneLineItem(), map[string]string{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	if err := h.svc.HandleEvent(context.Background(), lineItemEvent("CREATED", testLineItemSfID)); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.oppSE.calls != 1 || len(h.oppRepo.upserts) != 1 || len(h.repo.upserts) != 1 || h.repo.upserts[0].oppID != testInvoiceOppRow {
		t.Errorf("opportunity fetches %d upserts %d line item upserts %+v", h.oppSE.calls, len(h.oppRepo.upserts), h.repo.upserts)
	}
}

func TestLineItemIngest_MissingAccountFailsForRetry(t *testing.T) {
	h := newLineItemHarness(sampleStandaloneLineItem(), map[string]string{}, map[string]string{})
	err := h.svc.HandleEvent(context.Background(), lineItemEvent("UPDATED", testLineItemSfID))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) || !strings.HasPrefix(nf.Msg, "account not found") || len(h.repo.upserts) != 0 {
		t.Fatalf("err = %v upserts = %d", err, len(h.repo.upserts))
	}
	if len(h.states.upserts) == 0 || h.states.upserts[len(h.states.upserts)-1].Entity != domain.SalesforceIngestEntityOpportunityLineItem ||
		h.states.upserts[len(h.states.upserts)-1].Status != domain.SalesforceIngestFailed {
		t.Errorf("ledger = %+v, want a FAILED opportunity_line_item row", h.states.upserts)
	}
}

func TestLineItemIngest_NoOpportunityIDIsRetryable(t *testing.T) {
	li := sampleStandaloneLineItem()
	li.OpportunityID = nil
	h := newLineItemHarness(li, nil, nil)
	var su *apierror.ServiceUnavailableError
	if err := h.svc.HandleEvent(context.Background(), lineItemEvent("CREATED", testLineItemSfID)); !errors.As(err, &su) || len(h.repo.upserts) != 0 {
		t.Fatalf("err = %v upserts = %d, want ServiceUnavailableError and no write", err, len(h.repo.upserts))
	}
}

func TestLineItemIngest_DeletedHardDeletesBy18CharID(t *testing.T) {
	h := newLineItemHarness(sampleStandaloneLineItem(), nil, nil)
	h.se.err = salesentity.NotFound("salesentity: opportunity line item not found")
	if err := h.svc.HandleEvent(context.Background(), lineItemEvent("DELETED", testLineItemSfID[:15])); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if h.se.calls != 1 || len(h.repo.deletes) != 1 || h.repo.deletes[0] != testLineItemSfID {
		t.Fatalf("fetches %d deletes %v", h.se.calls, h.repo.deletes)
	}
	if st := h.repo.deleteState[0]; st.Entity != domain.SalesforceIngestEntityOpportunityLineItem || st.EventType != domain.SalesforceEventDeleted {
		t.Errorf("ledger = %+v", st)
	}
	h.repo.deleteRows = 0
	if err := h.svc.HandleEvent(context.Background(), lineItemEvent("DELETED", testLineItemSfID)); err != nil {
		t.Errorf("missing line item delete: %v, want nil", err)
	}
}
