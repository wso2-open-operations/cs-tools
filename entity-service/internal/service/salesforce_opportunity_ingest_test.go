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
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	testOpportunitySfID = "006E200000aIb6ZIAS"
	testOppAccountSfID  = "001E200000AAAAAIAS"
	testOppAccountRowID = "11111111-2222-3333-4444-555555555555"
)

type fakeOpportunitySalesEntity struct {
	opp    salesentity.Opportunity
	err    error
	calls  int
	lastID string
}

func (f *fakeOpportunitySalesEntity) GetOpportunity(_ context.Context, id string) (salesentity.Opportunity, error) {
	f.calls++
	f.lastID = id
	return f.opp, f.err
}

type fakeOpportunityRepo struct {
	upserts     []domain.SalesforceOpportunityUpsert
	upsertState []domain.UpsertSalesforceIngestStateRequest
	upsertErr   error
	deletes     []string
	deleteState []domain.UpsertSalesforceIngestStateRequest
	deleteRows  int64
}

func (f *fakeOpportunityRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceOpportunityUpsert, state domain.UpsertSalesforceIngestStateRequest) (domain.SalesforceOpportunityUpsertResult, error) {
	f.upserts = append(f.upserts, row)
	f.upsertState = append(f.upsertState, state)
	if f.upsertErr != nil {
		return domain.SalesforceOpportunityUpsertResult{}, f.upsertErr
	}
	return domain.SalesforceOpportunityUpsertResult{OpportunityID: "opp-row", LineItemsWritten: len(row.LineItems)}, nil
}

func (f *fakeOpportunityRepo) DeleteBySfID(_ context.Context, sfID string, state domain.UpsertSalesforceIngestStateRequest) (int64, error) {
	f.deletes = append(f.deletes, sfID)
	f.deleteState = append(f.deleteState, state)
	return f.deleteRows, nil
}

func sampleOpportunity() salesentity.Opportunity {
	won := true
	qty, price := 3.0, 1200.5
	return salesentity.Opportunity{
		ID:                          testOpportunitySfID,
		Name:                        sampleStr("CSM Sync Test Opportunity"),
		CustomerID:                  testOppAccountSfID,
		StageName:                   sampleStr("Closed Won"),
		IsWon:                       &won,
		SupportAccountEndDateRollUp: sampleStr("2027-03-31"),
		EulaVersion:                 salesentity.OptionalString{Present: true, Value: sampleStr("WSO2 EULA v3.5")},
		Type:                        sampleStr("Renewal"),
		EngagementCode:              sampleStr("ENG-9"),
		SubscriptionLineItems: []salesentity.SubscriptionLineItem{{
			ID:               sampleStr("00kE200000EB5HFIA1"),
			OpportunityID:    sampleStr(testOpportunitySfID),
			Name:             sampleStr("CSM Sync Test Opportunity - Development Support"),
			Quantity:         &qty,
			TotalPrice:       &price,
			Environment:      sampleStr("Production"),
			Classification:   sampleStr("Subscription"),
			ServiceStartDate: sampleStr("2026-04-01"),
			ServiceEndDate:   sampleStr("2027-03-31"),
			Product: &salesentity.SubscriptionProduct{
				ID: sampleStr("01tE0000000001"), Name: sampleStr("Development Support - 40 hours"), Description: sampleStr("40 hours"),
				ProductUnit: sampleStr("Hours"), Family: sampleStr("Support"), ProductCode: sampleStr("DS-40"), EngProductCode: sampleStr("DEVSUP"),
			},
		}},
	}
}

// newOpportunityService builds a service with the Opportunity ingest on, the
// Account ingest off (EnsureAccount is a pure lookup) and the given accounts
// already in CSM.
func newOpportunityService(se *fakeOpportunitySalesEntity, repo *fakeOpportunityRepo, states *fakeIngestStateRepo, accounts map[string]string) *salesforceEventService {
	lookup := &stubSalesforceAccountRepo{accountsBySfID: accounts}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{Accounts: lookup, States: states})
	return WithOpportunityIngest(svc, OpportunityIngest{Opportunities: repo, SalesEntity: se}).(*salesforceEventService)
}

func opportunityEvent(eventType string) domain.SalesforceEventRequest {
	return domain.SalesforceEventRequest{Entity: "Opportunity", EventType: eventType, ReferenceID: testOpportunitySfID}
}

func TestOpportunityIngest_DisabledIgnoresEvents(t *testing.T) {
	se := &fakeOpportunitySalesEntity{opp: sampleOpportunity()}
	svc := NewSalesforceEventService(nil, &stubSalesEntityClient{}, SalesforceIngestSupport{States: &fakeIngestStateRepo{}})
	for _, et := range []string{"CREATED", "UPDATED", "DELETED", "RESTORED"} {
		if err := svc.HandleEvent(context.Background(), opportunityEvent(et)); err != nil {
			t.Errorf("%s: err = %v, want nil (acknowledged)", et, err)
		}
	}
	if se.calls != 0 {
		t.Errorf("sales entity calls = %d, want 0", se.calls)
	}
	var re OpportunityReingester = svc.(*salesforceEventService)
	if err := re.RetryOpportunityIngest(context.Background(), testOpportunitySfID); !errors.Is(err, errOpportunityIngestDisabled) {
		t.Errorf("retry err = %v, want errOpportunityIngestDisabled", err)
	}
}

// TestOpportunityIngest_UpsertsWithAccountAndLineItems: CREATED, UPDATED and
// RESTORED all fetch, resolve the account and write one upsert whose ledger
// row is SUCCEEDED for entity "opportunity".
func TestOpportunityIngest_UpsertsWithAccountAndLineItems(t *testing.T) {
	for _, et := range []string{"CREATED", "UPDATED", "RESTORED", "updated"} {
		t.Run(et, func(t *testing.T) {
			se := &fakeOpportunitySalesEntity{opp: sampleOpportunity()}
			repo := &fakeOpportunityRepo{}
			states := &fakeIngestStateRepo{}
			svc := newOpportunityService(se, repo, states, map[string]string{testOppAccountSfID: testOppAccountRowID})
			err := svc.HandleEvent(context.Background(), opportunityEvent(et))
			if et == "updated" {
				// Event types are case-sensitive, as for every other entity.
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("err = %v, want ValidationError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("HandleEvent: %v", err)
			}
			if se.lastID != testOpportunitySfID || len(repo.upserts) != 1 {
				t.Fatalf("fetched %q, upserts %d", se.lastID, len(repo.upserts))
			}
			row := repo.upserts[0]
			if row.SfID != testOpportunitySfID || derefString(row.AccountID) != testOppAccountRowID || len(row.LineItems) != 1 {
				t.Errorf("row = %+v", row)
			}
			st := repo.upsertState[0]
			if st.Entity != domain.SalesforceIngestEntityOpportunity || st.SfID != testOpportunitySfID || st.EventType != et || st.Status != domain.SalesforceIngestSucceeded {
				t.Errorf("ledger state = %+v", st)
			}
			if len(states.upserts) != 0 {
				t.Errorf("out-of-transaction ledger writes = %+v, want none on success", states.upserts)
			}
		})
	}
}

func TestMapSalesEntityOpportunity(t *testing.T) {
	row := mapSalesEntityOpportunity(context.Background(), sampleOpportunity())
	if row.SfID != testOpportunitySfID || derefString(row.Name) != "CSM Sync Test Opportunity" || derefString(row.Stage) != "Closed Won" ||
		row.IsWon == nil || !*row.IsWon {
		t.Errorf("row = %+v", row)
	}
	// D6: close_date is the support-account end-date roll-up.
	if row.CloseDate == nil || row.CloseDate.Format(time.DateOnly) != "2027-03-31" {
		t.Errorf("close date = %v, want 2027-03-31", row.CloseDate)
	}
	if row.KeepExistingEula || derefString(row.EulaVersion) != "WSO2 EULA v3.5" || derefString(row.EulaVersionDecimal) != "3.5" {
		t.Errorf("eula = keep:%v %v %v", row.KeepExistingEula, derefString(row.EulaVersion), derefString(row.EulaVersionDecimal))
	}
	if len(row.LineItems) != 1 {
		t.Fatalf("line items = %d", len(row.LineItems))
	}
	li := row.LineItems[0]
	if li.LineItemSfID != "00kE200000EB5HFIA1" || derefString(li.Name) != "CSM Sync Test Opportunity - Development Support" ||
		derefString(li.ProductName) != "Development Support - 40 hours" || li.Quantity == nil || *li.Quantity != 3 ||
		li.TotalPrice == nil || *li.TotalPrice != 1200.5 || li.ServiceStartDate == nil || li.ServiceStartDate.Format(time.DateOnly) != "2026-04-01" ||
		li.ServiceEndDate == nil || li.ServiceEndDate.Format(time.DateOnly) != "2027-03-31" ||
		derefString(li.ProductCode) != "DS-40" || derefString(li.ProductDescription) != "40 hours" || derefString(li.ProductFamily) != "Support" ||
		derefString(li.ProductUnit) != "Hours" || derefString(li.EngProductCode) != "DEVSUP" || derefString(li.ProductSfID) != "01tE0000000001" ||
		derefString(li.Classification) != "Subscription" || derefString(li.Environment) != "Production" {
		t.Errorf("line item = %+v", li)
	}
}

// TestMapSalesEntityOpportunity_LineItemEdgeCases: a line item without an id
// is skipped, a repeated id is written once (last wins), a line item without
// a product leaves the product columns NULL.
func TestMapSalesEntityOpportunity_LineItemEdgeCases(t *testing.T) {
	opp := sampleOpportunity()
	opp.SubscriptionLineItems = []salesentity.SubscriptionLineItem{
		{ID: nil, Name: sampleStr("no id")},
		{ID: sampleStr("00k1"), Name: sampleStr("first")},
		{ID: sampleStr("00k1"), Name: sampleStr("second")},
		{ID: sampleStr("00k2")},
	}
	row := mapSalesEntityOpportunity(context.Background(), opp)
	if len(row.LineItems) != 2 || derefString(row.LineItems[0].Name) != "second" || row.LineItems[1].LineItemSfID != "00k2" {
		t.Fatalf("line items = %+v", row.LineItems)
	}
	if row.LineItems[1].ProductName != nil || row.LineItems[1].ProductSfID != nil {
		t.Errorf("product columns = %+v, want NULL without a product", row.LineItems[1])
	}
}

func TestMapSalesEntityOpportunity_TruncatesToColumnLimits(t *testing.T) {
	opp := sampleOpportunity()
	opp.Name = sampleStr(strings.Repeat("é", 250))
	opp.StageName = sampleStr(strings.Repeat("s", 41))
	opp.SubscriptionLineItems[0].Product.Name = sampleStr(strings.Repeat("p", 150))
	row := mapSalesEntityOpportunity(context.Background(), opp)
	if n := utf8.RuneCountInString(derefString(row.Name)); n != 200 {
		t.Errorf("name length = %d, want 200", n)
	}
	if n := len(derefString(row.Stage)); n != 40 {
		t.Errorf("stage length = %d, want 40", n)
	}
	if n := len(derefString(row.LineItems[0].ProductName)); n != 100 {
		t.Errorf("product name length = %d, want 100", n)
	}
}

func TestDeriveEulaVersion(t *testing.T) {
	cases := map[string]struct {
		in          salesentity.OptionalString
		wantKeep    bool
		wantVersion string
		wantDecimal string
	}{
		"key absent keeps stored":   {salesentity.OptionalString{}, true, "", ""},
		"null defaults to 3.4":      {salesentity.OptionalString{Present: true}, false, "", "3.4"},
		"blank defaults to 3.4":     {salesentity.OptionalString{Present: true, Value: sampleStr("  ")}, false, "", "3.4"},
		"first decimal match":       {salesentity.OptionalString{Present: true, Value: sampleStr("EULA 3.5 (rev 2.1)")}, false, "EULA 3.5 (rev 2.1)", "3.5"},
		"multi-digit":               {salesentity.OptionalString{Present: true, Value: sampleStr("v10.12")}, false, "v10.12", "10.12"},
		"no decimal is NULL":        {salesentity.OptionalString{Present: true, Value: sampleStr("Custom EULA")}, false, "Custom EULA", ""},
		"integer only is NULL":      {salesentity.OptionalString{Present: true, Value: sampleStr("EULA 4")}, false, "EULA 4", ""},
		"surrounding space trimmed": {salesentity.OptionalString{Present: true, Value: sampleStr(" 3.6 ")}, false, "3.6", "3.6"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			keep, version, decimal := deriveEulaVersion(tc.in)
			if keep != tc.wantKeep || derefString(version) != tc.wantVersion || derefString(decimal) != tc.wantDecimal {
				t.Errorf("got keep=%v version=%q decimal=%q, want %v %q %q", keep, derefString(version), derefString(decimal), tc.wantKeep, tc.wantVersion, tc.wantDecimal)
			}
		})
	}
}

// TestOpportunityIngest_Guard: with a lastModifiedDate, a version the ledger
// already holds is skipped; without one (today's Sales Entity) the guard is
// skipped and the idempotent upsert runs.
func TestOpportunityIngest_Guard(t *testing.T) {
	recorded := domain.SalesforceIngestState{
		Entity: domain.SalesforceIngestEntityOpportunity, SfID: testOpportunitySfID, Status: domain.SalesforceIngestSucceeded,
		EventType: "UPDATED", EventModifiedOn: time.Date(2026, 9, 18, 6, 37, 7, 0, time.UTC),
	}
	key := domain.SalesforceIngestEntityOpportunity + "/" + testOpportunitySfID

	t.Run("same version skipped", func(t *testing.T) {
		opp := sampleOpportunity()
		opp.LastModifiedDate = sampleStr("2026-09-18T06:37:07.000+0000")
		repo := &fakeOpportunityRepo{}
		states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{key: recorded}}
		svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: opp}, repo, states, map[string]string{testOppAccountSfID: testOppAccountRowID})
		if err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED")); err != nil {
			t.Fatalf("HandleEvent: %v", err)
		}
		if len(repo.upserts) != 0 {
			t.Errorf("upserts = %d, want 0 (duplicate)", len(repo.upserts))
		}
	})
	t.Run("newer version written", func(t *testing.T) {
		opp := sampleOpportunity()
		opp.LastModifiedDate = sampleStr("2026-09-19T00:00:00.000+0000")
		repo := &fakeOpportunityRepo{}
		states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{key: recorded}}
		svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: opp}, repo, states, map[string]string{testOppAccountSfID: testOppAccountRowID})
		if err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED")); err != nil {
			t.Fatalf("HandleEvent: %v", err)
		}
		if len(repo.upserts) != 1 || !repo.upsertState[0].EventModifiedOn.Equal(time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("upserts = %d state = %+v, want one write stamped with the record's version", len(repo.upserts), repo.upsertState)
		}
	})
	t.Run("no lastModifiedDate skips the guard", func(t *testing.T) {
		repo := &fakeOpportunityRepo{}
		states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{key: recorded}}
		svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: sampleOpportunity()}, repo, states, map[string]string{testOppAccountSfID: testOppAccountRowID})
		if err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED")); err != nil {
			t.Fatalf("HandleEvent: %v", err)
		}
		if states.getCalls != 0 || len(repo.upserts) != 1 {
			t.Errorf("ledger reads = %d, upserts = %d; want 0 and 1", states.getCalls, len(repo.upserts))
		}
	})
}

// TestOpportunityIngest_AccountNotFound: with the Account ingest off and the
// account absent, the event fails with a NotFoundError, nothing is written,
// and a FAILED ledger row carries text the retry job recognises.
func TestOpportunityIngest_AccountNotFound(t *testing.T) {
	repo := &fakeOpportunityRepo{}
	states := &fakeIngestStateRepo{}
	svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: sampleOpportunity()}, repo, states, map[string]string{})
	err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED"))
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
	if len(repo.upserts) != 0 {
		t.Errorf("upserts = %d, want 0", len(repo.upserts))
	}
	if len(states.upserts) != 1 {
		t.Fatalf("ledger writes = %+v, want one FAILED row", states.upserts)
	}
	st := states.upserts[0]
	if st.Status != domain.SalesforceIngestFailed || st.Entity != domain.SalesforceIngestEntityOpportunity || st.EventType != "UPDATED" ||
		!repository.IsMissingParentError(derefString(st.LastError)) {
		t.Errorf("ledger row = %+v (lastError %q), want FAILED with a missing-parent error", st, derefString(st.LastError))
	}
}

// TestOpportunityIngest_AccountIngestedFirst: with the Account ingest on, a
// missing account is fetched and written before the opportunity.
func TestOpportunityIngest_AccountIngestedFirst(t *testing.T) {
	accounts := &stubSalesforceAccountRepo{accountsBySfID: map[string]string{}, registerOnUpsert: testOppAccountRowID}
	cust := sampleCustomer()
	cust.ID = testOppAccountSfID
	custSE := &stubSalesEntityClient{customer: cust}
	repo := &fakeOpportunityRepo{}
	svc := WithOpportunityIngest(
		NewSalesforceEventService(accounts, custSE, SalesforceIngestSupport{Accounts: accounts, States: &fakeIngestStateRepo{}}),
		OpportunityIngest{Opportunities: repo, SalesEntity: &fakeOpportunitySalesEntity{opp: sampleOpportunity()}},
	)
	if err := svc.HandleEvent(context.Background(), opportunityEvent("CREATED")); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if custSE.lastID != testOppAccountSfID || accounts.upsertCalls != 1 || len(repo.upserts) != 1 || derefString(repo.upserts[0].AccountID) != testOppAccountRowID {
		t.Errorf("customer fetch %q, account upserts %d, opportunity upserts %d", custSE.lastID, accounts.upsertCalls, len(repo.upserts))
	}
}

func TestOpportunityIngest_UpsertErrorRecordsFailed(t *testing.T) {
	repo := &fakeOpportunityRepo{upsertErr: errors.New("boom")}
	states := &fakeIngestStateRepo{}
	svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: sampleOpportunity()}, repo, states, map[string]string{testOppAccountSfID: testOppAccountRowID})
	if err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the repository error", err)
	}
	if len(states.upserts) != 1 || states.upserts[0].Status != domain.SalesforceIngestFailed || derefString(states.upserts[0].LastError) != "boom" {
		t.Errorf("ledger writes = %+v", states.upserts)
	}
}

func TestOpportunityIngest_FetchErrorPropagates(t *testing.T) {
	repo := &fakeOpportunityRepo{}
	states := &fakeIngestStateRepo{}
	se := &fakeOpportunitySalesEntity{err: &apierror.ServiceUnavailableError{Msg: "salesentity: opportunity not found"}}
	svc := newOpportunityService(se, repo, states, nil)
	var sue *apierror.ServiceUnavailableError
	if err := svc.HandleEvent(context.Background(), opportunityEvent("UPDATED")); !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError (Service Bus retries)", err)
	}
	if len(repo.upserts) != 0 || len(states.upserts) != 0 {
		t.Errorf("writes after a failed fetch: upserts=%d ledger=%d", len(repo.upserts), len(states.upserts))
	}
}

// TestOpportunityIngest_Deleted: DELETED is a hard delete by sf_id with a
// DELETED ledger row and no Sales Entity call; a missing row is acknowledged.
func TestOpportunityIngest_Deleted(t *testing.T) {
	for _, rows := range []int64{1, 0} {
		se := &fakeOpportunitySalesEntity{err: salesentity.NotFound("salesentity: opportunity not found")}
		repo := &fakeOpportunityRepo{deleteRows: rows}
		svc := newOpportunityService(se, repo, &fakeIngestStateRepo{}, nil)
		if err := svc.HandleEvent(context.Background(), opportunityEvent("DELETED")); err != nil {
			t.Fatalf("rows=%d: err = %v", rows, err)
		}
		if se.calls != 1 || len(repo.deletes) != 1 || repo.deletes[0] != testOpportunitySfID {
			t.Errorf("rows=%d: fetches %d, deletes %v", rows, se.calls, repo.deletes)
		}
		st := repo.deleteState[0]
		if st.EventType != domain.SalesforceEventDeleted || st.Status != domain.SalesforceIngestSucceeded || st.Entity != domain.SalesforceIngestEntityOpportunity {
			t.Errorf("rows=%d: ledger state = %+v", rows, st)
		}
	}
}

// TestOpportunityIngest_DeletedWidens15CharID: a 15-character referenceId is
// deleted, locked and recorded under the 18-character Id the ingest stored.
func TestOpportunityIngest_DeletedWidens15CharID(t *testing.T) {
	repo := &fakeOpportunityRepo{deleteRows: 1}
	svc := newOpportunityService(&fakeOpportunitySalesEntity{err: salesentity.NotFound("salesentity: opportunity not found")}, repo, &fakeIngestStateRepo{}, nil)
	req := opportunityEvent("DELETED")
	req.ReferenceID = testOpportunitySfID[:15]
	if err := svc.HandleEvent(context.Background(), req); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(repo.deletes) != 1 || repo.deletes[0] != testOpportunitySfID {
		t.Errorf("deletes = %v, want [%s]", repo.deletes, testOpportunitySfID)
	}
	if repo.deleteState[0].SfID != testOpportunitySfID {
		t.Errorf("ledger sfId = %q, want %q", repo.deleteState[0].SfID, testOpportunitySfID)
	}
}

func TestSalesforceID18(t *testing.T) {
	for in, want := range map[string]string{
		"001A0000006Vm9r":    "001A0000006Vm9rIAC",
		"006E200000aIb6Z":    "006E200000aIb6ZIAS",
		" 006E200000aIb6Z ":  "006E200000aIb6ZIAS",
		"006E200000aIb6ZIAS": "006E200000aIb6ZIAS", // already 18
		"006E200000aIb6-":    "006E200000aIb6-",    // not an Id
		"":                   "",
	} {
		if got := salesforceID18(in); got != want {
			t.Errorf("salesforceID18(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRetryOpportunityIngest_ReRunsAsUpdated(t *testing.T) {
	repo := &fakeOpportunityRepo{}
	svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: sampleOpportunity()}, repo, &fakeIngestStateRepo{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	var re OpportunityReingester = svc
	if err := re.RetryOpportunityIngest(context.Background(), testOpportunitySfID); err != nil {
		t.Fatalf("RetryOpportunityIngest: %v", err)
	}
	if len(repo.upsertState) != 1 || repo.upsertState[0].EventType != domain.SalesforceEventUpdated {
		t.Errorf("states = %+v, want one UPDATED write", repo.upsertState)
	}
}

// TestRetryWorker_OpportunityRetrier: a FAILED "account not found"
// opportunity ledger row is handed to the registered retrier.
func TestRetryWorker_OpportunityRetrier(t *testing.T) {
	msg := `account not found for sfId "001E200000AAAAAIAS"`
	states := &fakeIngestStateRepo{failed: []domain.SalesforceIngestState{{
		Entity: domain.SalesforceIngestEntityOpportunity, SfID: testOpportunitySfID, Status: domain.SalesforceIngestFailed, LastError: &msg, AttemptCount: 1,
	}}}
	repo := &fakeOpportunityRepo{}
	svc := newOpportunityService(&fakeOpportunitySalesEntity{opp: sampleOpportunity()}, repo, &fakeIngestStateRepo{}, map[string]string{testOppAccountSfID: testOppAccountRowID})
	w := NewSalesforceIngestRetryWorker(nil, nil, states, time.Minute)
	w.EntityRetriers[domain.SalesforceIngestEntityOpportunity] = svc.RetryOpportunityIngest
	w.RunOnce(context.Background())
	if len(repo.upserts) != 1 {
		t.Errorf("upserts = %d, want the retrier to re-run the opportunity", len(repo.upserts))
	}
}
