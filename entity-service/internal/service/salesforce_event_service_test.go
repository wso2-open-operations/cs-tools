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
	"strings"
	"testing"

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
	upsertCalls  int
	lastUpsert   domain.SalesforceAccountUpsert
	upsertErr    error
	deleteCalls  int
	lastDeleteID string
	deleteErr    error
	lookup       map[string]string
	lookupErr    error
}

func (s *stubSalesforceAccountRepo) UpsertFromSalesforce(_ context.Context, row domain.SalesforceAccountUpsert) error {
	s.upsertCalls++
	s.lastUpsert = row
	return s.upsertErr
}

func (s *stubSalesforceAccountRepo) SoftDeleteBySfID(_ context.Context, sfID string) error {
	s.deleteCalls++
	s.lastDeleteID = sfID
	return s.deleteErr
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
	svc := NewSalesforceEventService(repo, se)

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
	se := &stubSalesEntityClient{}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se)

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventDeleted,
		Entity:      "account",
		ReferenceID: "  001xx0000001  ",
	})
	if err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if se.calls != 0 {
		t.Errorf("GetCustomer calls = %d, want 0", se.calls)
	}
	if repo.deleteCalls != 1 || repo.lastDeleteID != "001xx0000001" {
		t.Errorf("soft-delete calls = %d id = %q", repo.deleteCalls, repo.lastDeleteID)
	}
}

func TestHandleEvent_UnknownEntityIgnored(t *testing.T) {
	se := &stubSalesEntityClient{}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se)

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
	svc := NewSalesforceEventService(&stubSalesforceAccountRepo{}, &stubSalesEntityClient{})
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
	svc := NewSalesforceEventService(&stubSalesforceAccountRepo{}, se)

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

func TestHandleEvent_MissingNameIs503(t *testing.T) {
	cust := sampleCustomer()
	cust.Name = sampleStr("  ")
	se := &stubSalesEntityClient{customer: cust}
	svc := NewSalesforceEventService(&stubSalesforceAccountRepo{}, se)

	err := svc.HandleEvent(context.Background(), domain.SalesforceEventRequest{
		EventType:   domain.SalesforceEventCreated,
		Entity:      "Account",
		ReferenceID: "001xx0000001",
	})
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

func TestHandleEvent_MissingOwnerLeavesFKNull(t *testing.T) {
	se := &stubSalesEntityClient{customer: sampleCustomer()}
	repo := &stubSalesforceAccountRepo{lookup: map[string]string{}}
	svc := NewSalesforceEventService(repo, se)

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

func TestMapPhone_LongerThan20OmitsUpdate(t *testing.T) {
	got, omit := mapPhone("+49 40 123456789012345")
	if got != nil || !omit {
		t.Errorf("mapPhone(long) = %v omit = %v, want nil / true", got, omit)
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
	cust.Phone = sampleStr("+49 40 123456789012345")
	se := &stubSalesEntityClient{customer: cust}
	repo := &stubSalesforceAccountRepo{}
	svc := NewSalesforceEventService(repo, se)

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
	got := mapSalesEntityCustomer(salesentity.Customer{ID: "001xx", Name: sampleStr("Acme")})
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
