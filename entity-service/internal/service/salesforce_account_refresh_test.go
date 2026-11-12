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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// accountAlreadyWritten returns a service whose ledger already holds the
// customer's current version, so an UPDATED event for it is a duplicate.
func accountAlreadyWritten() (SalesforceEventService, *stubSalesforceAccountRepo) {
	cust := fullCustomer()
	cust.ID = testAccountID
	cust.LastModifiedDate = sampleStr("2026-09-29T10:00:00.000+0000")
	states := &fakeIngestStateRepo{rows: map[string]domain.SalesforceIngestState{
		domain.SalesforceIngestEntityAccount + "/" + testAccountID: ingestStateRow(domain.SalesforceIngestSucceeded, domain.SalesforceEventUpdated, "2026-09-29T10:00:00.000+0000"),
	}}
	repo := &stubSalesforceAccountRepo{lookup: fullCustomerLookup(), states: states}
	return NewSalesforceEventService(repo, &stubSalesEntityClient{customer: cust}, SalesforceIngestSupport{States: states}), repo
}

// TestRefreshAccount_WritesAVersionTheGuardSkips: the event for an unchanged
// account is skipped, the refresh writes it anyway (with the primary partner).
func TestRefreshAccount_WritesAVersionTheGuardSkips(t *testing.T) {
	svc, repo := accountAlreadyWritten()

	if err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType: domain.SalesforceEventUpdated, Entity: "Account", ReferenceID: testAccountID,
	}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("event upsert calls = %d, want 0 (duplicate)", repo.upsertCalls)
	}

	if err := svc.(AccountRefresher).RefreshAccount(context.Background(), " "+testAccountID+" "); err != nil {
		t.Fatalf("RefreshAccount: %v", err)
	}
	if repo.upsertCalls != 1 || repo.lastUpsert.SfID != testAccountID {
		t.Fatalf("refresh upsert calls = %d sfId = %q, want 1 / %s", repo.upsertCalls, repo.lastUpsert.SfID, testAccountID)
	}
	if !eqStr(repo.lastUpsert.PrimaryPartnerSfID, "001xx0000009") {
		t.Errorf("primary partner = %v, want 001xx0000009", repo.lastUpsert.PrimaryPartnerSfID)
	}
	if repo.lastState.Status != domain.SalesforceIngestSucceeded || repo.lastState.EventType != domain.SalesforceEventUpdated {
		t.Errorf("ledger row = %+v, want SUCCEEDED UPDATED", repo.lastState)
	}
}

func TestRefreshAccount_RequiresSfIDAndAccountIngest(t *testing.T) {
	svc, repo := accountAlreadyWritten()
	var ve *apierror.ValidationError
	if err := svc.(AccountRefresher).RefreshAccount(context.Background(), "  "); !errors.As(err, &ve) {
		t.Errorf("blank sfId: err = %v, want ValidationError", err)
	}

	off := NewSalesforceEventService(nil, &stubSalesEntityClient{customer: fullCustomer()}, SalesforceIngestSupport{})
	if err := off.(AccountRefresher).RefreshAccount(context.Background(), testAccountID); err == nil {
		t.Error("account ingest off: err = nil, want an error")
	}
	if repo.upsertCalls != 0 {
		t.Errorf("upsert calls = %d, want 0", repo.upsertCalls)
	}
}

func TestAccountRefreshService_InternalCallersOnly(t *testing.T) {
	svc, repo := accountAlreadyWritten()
	refresher := svc.(AccountRefresher)

	err := NewAccountRefreshService(refresher, stubAccess{scope: AccessScope{}}).Refresh(context.Background(), testAccountID)
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || repo.upsertCalls != 0 {
		t.Fatalf("err = %v upsert calls = %d, want Forbidden and no write", err, repo.upsertCalls)
	}
	if err := NewAccountRefreshService(refresher, stubAccess{scope: AccessScope{Unrestricted: true}}).Refresh(context.Background(), testAccountID); err != nil || repo.upsertCalls != 1 {
		t.Fatalf("err = %v upsert calls = %d, want nil / 1", err, repo.upsertCalls)
	}
}
