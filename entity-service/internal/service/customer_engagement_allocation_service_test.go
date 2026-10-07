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
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeAllocationStore is an in-memory customer_engagement / allocation_resource pair.
type fakeAllocationStore struct {
	engagementsByEngID map[string]string // engagement_id -> id
	engagementsByLine  map[string]string // line-item sf id -> id
	accountsBySfID     map[string]string
	accountsByName     map[string][]domain.AccountCandidate
	usersByEmail       map[string]string
	allocations        map[string]domain.AllocationResourceFields // engagement/allocation -> row
	allocationIDs      map[string]string
	inserted           []domain.NewCustomerEngagement
	seq                int
}

func newFakeAllocationStore() *fakeAllocationStore {
	return &fakeAllocationStore{
		engagementsByEngID: map[string]string{}, engagementsByLine: map[string]string{},
		accountsBySfID: map[string]string{}, accountsByName: map[string][]domain.AccountCandidate{},
		usersByEmail: map[string]string{}, allocations: map[string]domain.AllocationResourceFields{},
		allocationIDs: map[string]string{},
	}
}

func (f *fakeAllocationStore) InTx(_ context.Context, fn func(repository.AllocationEventStore) error) error {
	return fn(f)
}

func (f *fakeAllocationStore) nextID() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func allocOptional(m map[string]string, k string) *string {
	if v, ok := m[k]; ok {
		return &v
	}
	return nil
}

func (f *fakeAllocationStore) FindEngagementByEngagementID(_ context.Context, id string) (*string, error) {
	return allocOptional(f.engagementsByEngID, id), nil
}
func (f *fakeAllocationStore) FindEngagementByLineItemSfID(_ context.Context, id string) (*string, error) {
	return allocOptional(f.engagementsByLine, id), nil
}
func (f *fakeAllocationStore) FindAccountBySfID(_ context.Context, id string) (*string, error) {
	return allocOptional(f.accountsBySfID, id), nil
}
func (f *fakeAllocationStore) FindAccountsByName(_ context.Context, name string) ([]domain.AccountCandidate, error) {
	return f.accountsByName[name], nil
}
func (f *fakeAllocationStore) FindUserByEmailOrUserName(_ context.Context, email string) (*string, error) {
	return allocOptional(f.usersByEmail, strings.ToLower(email)), nil
}
func (f *fakeAllocationStore) InsertEngagement(_ context.Context, e domain.NewCustomerEngagement) (string, bool, error) {
	if id, ok := f.engagementsByEngID[e.EngagementID]; ok {
		return id, false, nil
	}
	id := f.nextID()
	f.engagementsByEngID[e.EngagementID] = id
	f.inserted = append(f.inserted, e)
	return id, true, nil
}
func (f *fakeAllocationStore) UpdateAllocationResource(_ context.Context, r domain.AllocationResourceFields) (*string, error) {
	key := r.EngagementID + "/" + r.AllocationID
	id, ok := f.allocationIDs[key]
	if !ok {
		return nil, nil
	}
	f.allocations[key] = r
	return &id, nil
}
func (f *fakeAllocationStore) UpsertAllocationResource(ctx context.Context, r domain.AllocationResourceFields, _ string) (string, bool, error) {
	if id, _ := f.UpdateAllocationResource(ctx, r); id != nil {
		return *id, false, nil
	}
	key := r.EngagementID + "/" + r.AllocationID
	id := f.nextID()
	f.allocationIDs[key], f.allocations[key] = id, r
	return id, true, nil
}

func allocStr(s string) *string { return &s }

func allocFirefightingEvent() domain.AllocationEvent {
	return domain.AllocationEvent{
		ID: "A0001", Email: "consultant@wso2.com", AllocationType: 76,
		AllocationTypeName: "Support Related Customer Firefighting",
		StartDate:          "2026-10-01", EndDate: "2026-10-10",
		StartTime: allocStr("09:00:00"), EndTime: allocStr("17:00:00"), TimeZone: allocStr("Asia/Colombo"),
		ClearanceStatus: allocStr("Confirmed"), CustomerCode: allocStr("001000000000001AAA"),
		Engagement: &domain.AllocationEventEngagement{
			EngagementID: "E1001", EngagementCode: "EC-FF-1", CustomerName: "Acme",
			EngagementTypeName: "Non-Paid Post Sale", EngagementNature: "Off-site",
		},
	}
}

func allocLineItemEvent() domain.AllocationEvent {
	ev := allocFirefightingEvent()
	ev.AllocationType = 12
	ev.AllocationTypeName = "Consulting - Delivery"
	ev.Engagement.EngagementID = "E2001"
	ev.Engagement.ProductID = allocStr("00k000000000001AAA")
	return ev
}

func newAllocationSvc(f *fakeAllocationStore) CustomerEngagementAllocationService {
	return NewCustomerEngagementAllocationService(f)
}

func TestAllocationEvent_FirefightingCreatesEngagement(t *testing.T) {
	f := newFakeAllocationStore()
	f.accountsBySfID["001000000000001AAA"] = "acct-1"
	f.usersByEmail["consultant@wso2.com"] = "user-1"

	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), allocFirefightingEvent())
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != domain.AllocationEventCreated || !res.EngagementCreated || res.EngagementID == nil || res.AllocationResourceID == nil {
		t.Fatalf("result = %+v", res)
	}
	if len(f.inserted) != 1 {
		t.Fatalf("inserted %d engagements", len(f.inserted))
	}
	e := f.inserted[0]
	if e.Name != "Acme - Support Related Customer Firefighting" || e.AccountID != "acct-1" || e.IsPaid ||
		e.EngagementType != "FIREFIGHTING" || e.DeliveryMode == nil || *e.DeliveryMode != "OFFSITE" ||
		*e.PlannedStartDate != "2026-10-01" || *e.PlannedEndDate != "2026-10-10" || *e.EngagementCode != "EC-FF-1" {
		t.Errorf("engagement = %+v", e)
	}
	row := f.allocations[*res.EngagementID+"/A0001"]
	if row.State == nil || *row.State != "CONFIRMED" || *row.TimeZone != "Asia/Colombo" {
		t.Errorf("allocation row = %+v", row)
	}
}

// is_paid matches ServiceNow's exact "Paid" check, after trimming whitespace.
func TestAllocationEvent_PaidIsExactAfterTrim(t *testing.T) {
	for name, want := range map[string]bool{" Paid ": true, "Paid Post Sale": false} {
		f := newFakeAllocationStore()
		f.accountsBySfID["001000000000001AAA"] = "acct-1"
		f.usersByEmail["consultant@wso2.com"] = "user-1"
		ev := allocFirefightingEvent()
		ev.Engagement.EngagementTypeName = name
		if _, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
		if len(f.inserted) != 1 || f.inserted[0].IsPaid != want {
			t.Errorf("%q: is_paid = %+v, want %v", name, f.inserted, want)
		}
	}
}

func TestAllocationEvent_FirefightingUsesExistingEngagement(t *testing.T) {
	f := newFakeAllocationStore()
	f.engagementsByEngID["E1001"] = "eng-existing"
	f.usersByEmail["consultant@wso2.com"] = "user-1"

	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), allocFirefightingEvent())
	if err != nil {
		t.Fatal(err)
	}
	if res.EngagementCreated || *res.EngagementID != "eng-existing" || res.Result != domain.AllocationEventCreated || len(f.inserted) != 0 {
		t.Fatalf("result = %+v, inserted %d", res, len(f.inserted))
	}
}

func TestAllocationEvent_FirefightingWithoutEngagementIDSkips(t *testing.T) {
	ev := allocFirefightingEvent()
	ev.Engagement.EngagementID = " "
	res, err := newAllocationSvc(newFakeAllocationStore()).ProcessAllocationEvent(context.Background(), ev)
	if err != nil || res.Result != domain.AllocationEventSkipped || res.Reason != AllocationSkipNoEngagementID {
		t.Fatalf("result = %+v, err %v", res, err)
	}
}

func TestAllocationEvent_LineItemFound(t *testing.T) {
	f := newFakeAllocationStore()
	f.engagementsByLine["00k000000000001AAA"] = "eng-line"
	// ServiceNow matches non-firefighting by line item only, even when the engagement id matches elsewhere.
	f.engagementsByEngID["E2001"] = "eng-other"
	f.usersByEmail["consultant@wso2.com"] = "user-1"

	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), allocLineItemEvent())
	if err != nil || res.Result != domain.AllocationEventCreated || *res.EngagementID != "eng-line" || res.EngagementCreated {
		t.Fatalf("result = %+v, err %v", res, err)
	}
}

func TestAllocationEvent_LineItemMissingSkips(t *testing.T) {
	f := newFakeAllocationStore()
	f.engagementsByEngID["E2001"] = "eng-other"
	f.accountsBySfID["001000000000001AAA"] = "acct-1"
	noProduct := allocLineItemEvent()
	noProduct.Engagement.ProductID = nil
	for _, ev := range []domain.AllocationEvent{allocLineItemEvent(), noProduct} {
		res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev)
		if err != nil || res.Result != domain.AllocationEventSkipped || res.Reason != AllocationSkipNoLineItem ||
			res.EngagementID != nil || len(f.inserted) != 0 {
			t.Fatalf("result = %+v, err %v", res, err)
		}
	}
}

func TestAllocationEvent_FirefightingByTypeIDNotName(t *testing.T) {
	f := newFakeAllocationStore()
	f.accountsBySfID["001000000000001AAA"] = "acct-1"
	f.usersByEmail["consultant@wso2.com"] = "user-1"
	ev := allocFirefightingEvent()
	ev.AllocationType, ev.AllocationTypeName = 83, "Consulting Related Customer Firefighting"
	ev.Engagement.EngagementTypeName = "Paid"
	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev)
	if err != nil || !res.EngagementCreated || len(f.inserted) != 1 || !f.inserted[0].IsPaid {
		t.Fatalf("type 83: result = %+v, inserted %+v, err %v", res, f.inserted, err)
	}

	// A firefighting-sounding name on another type id takes the line-item path.
	other := allocLineItemEvent()
	other.AllocationTypeName = "Support Related Customer Firefighting"
	res, err = newAllocationSvc(f).ProcessAllocationEvent(context.Background(), other)
	if err != nil || res.Reason != AllocationSkipNoLineItem || len(f.inserted) != 1 {
		t.Fatalf("type 12: result = %+v, err %v", res, err)
	}
}

func TestAllocationEvent_UserMissingSkips(t *testing.T) {
	f := newFakeAllocationStore()
	f.engagementsByLine["00k000000000001AAA"] = "eng-line"

	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), allocLineItemEvent())
	if err != nil || res.Result != domain.AllocationEventSkipped || res.Reason != AllocationSkipUserNotFound ||
		res.AllocationResourceID != nil || *res.EngagementID != "eng-line" {
		t.Fatalf("result = %+v, err %v", res, err)
	}
	if len(f.allocations) != 0 {
		t.Errorf("allocation written without a user")
	}
}

func TestAllocationEvent_AccountResolution(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(f *fakeAllocationStore)
		code       *string
		wantAcct   string
		wantReason string
	}{
		{"by sf_id wins over name", func(f *fakeAllocationStore) {
			f.accountsBySfID["001000000000001AAA"] = "acct-sf"
			f.accountsByName["Acme"] = []domain.AccountCandidate{{ID: "acct-name", Live: true}}
		}, allocStr("001000000000001AAA"), "acct-sf", ""},
		{"unique name fallback", func(f *fakeAllocationStore) {
			f.accountsByName["Acme"] = []domain.AccountCandidate{{ID: "acct-name", Live: true}}
		}, allocStr("001000000000001AAA"), "acct-name", ""},
		{"live row preferred over deleted", func(f *fakeAllocationStore) {
			f.accountsByName["Acme"] = []domain.AccountCandidate{{ID: "acct-dead"}, {ID: "acct-live", Live: true}}
		}, nil, "acct-live", ""},
		{"ambiguous name", func(f *fakeAllocationStore) {
			f.accountsByName["Acme"] = []domain.AccountCandidate{{ID: "a1", Live: true}, {ID: "a2", Live: true}}
		}, nil, "", AllocationSkipAmbiguousAccount},
		{"no account", func(*fakeAllocationStore) {}, nil, "", AllocationSkipAccountNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAllocationStore()
			f.usersByEmail["consultant@wso2.com"] = "user-1"
			tc.setup(f)
			ev := allocFirefightingEvent()
			ev.CustomerCode = tc.code
			res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantReason != "" {
				if res.Result != domain.AllocationEventSkipped || res.Reason != tc.wantReason || len(f.inserted) != 0 {
					t.Fatalf("result = %+v", res)
				}
				return
			}
			if len(f.inserted) != 1 || f.inserted[0].AccountID != tc.wantAcct {
				t.Fatalf("inserted = %+v", f.inserted)
			}
		})
	}
}

func TestAllocationEvent_CustomerCodeFromEngagement(t *testing.T) {
	f := newFakeAllocationStore()
	f.accountsBySfID["001000000000002AAA"] = "acct-eng"
	f.usersByEmail["consultant@wso2.com"] = "user-1"
	ev := allocFirefightingEvent()
	ev.CustomerCode = nil
	ev.Engagement.CustomerCode = allocStr("001000000000002AAA")
	if _, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(f.inserted) != 1 || f.inserted[0].AccountID != "acct-eng" {
		t.Fatalf("inserted = %+v", f.inserted)
	}
}

func TestAllocationEvent_NameWithoutCustomerName(t *testing.T) {
	f := newFakeAllocationStore()
	f.accountsBySfID["001000000000001AAA"] = "acct-1"
	f.usersByEmail["consultant@wso2.com"] = "user-1"
	ev := allocFirefightingEvent()
	ev.Engagement.CustomerName = ""
	if _, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(f.inserted) != 1 || f.inserted[0].Name != "Support Related Customer Firefighting" {
		t.Fatalf("inserted = %+v, want the allocation type name alone", f.inserted)
	}
}

func TestAllocationEvent_RepeatIsIdempotent(t *testing.T) {
	f := newFakeAllocationStore()
	f.accountsBySfID["001000000000001AAA"] = "acct-1"
	f.usersByEmail["consultant@wso2.com"] = "user-1"
	svc := newAllocationSvc(f)

	first, err := svc.ProcessAllocationEvent(context.Background(), allocFirefightingEvent())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ProcessAllocationEvent(context.Background(), allocFirefightingEvent())
	if err != nil {
		t.Fatal(err)
	}
	if len(f.engagementsByEngID) != 1 || len(f.allocations) != 1 {
		t.Fatalf("engagements %d, allocations %d", len(f.engagementsByEngID), len(f.allocations))
	}
	if second.Result != domain.AllocationEventUpdated || second.EngagementCreated ||
		*second.EngagementID != *first.EngagementID || *second.AllocationResourceID != *first.AllocationResourceID {
		t.Fatalf("first %+v, second %+v", first, second)
	}
}

func TestAllocationEvent_UpdatePath(t *testing.T) {
	f := newFakeAllocationStore()
	f.engagementsByLine["00k000000000001AAA"] = "eng-line"
	f.allocationIDs["eng-line/A0001"] = "alloc-1"

	ev := allocLineItemEvent()
	ev.StartDate, ev.EndDate = "2026-11-01", "2026-11-30T00:00:00"
	ev.TimeZone = allocStr("Europe/London")
	ev.ClearanceStatus = allocStr("Rejected - Visa Issues")
	res, err := newAllocationSvc(f).ProcessAllocationEvent(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	// No user is configured: the update path must not need one.
	if res.Result != domain.AllocationEventUpdated || *res.AllocationResourceID != "alloc-1" {
		t.Fatalf("result = %+v", res)
	}
	row := f.allocations["eng-line/A0001"]
	if *row.StartDate != "2026-11-01" || *row.EndDate != "2026-11-30" || *row.TimeZone != "Europe/London" ||
		*row.State != "REJECTED_VISA_ISSUES" {
		t.Errorf("row = %+v", row)
	}
}

func TestAllocationEvent_Validation(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*domain.AllocationEvent)
	}{
		{"missing id", func(e *domain.AllocationEvent) { e.ID = "" }},
		{"missing email", func(e *domain.AllocationEvent) { e.Email = "  " }},
		{"bad date", func(e *domain.AllocationEvent) { e.StartDate = "01/10/2026" }},
		{"long timezone", func(e *domain.AllocationEvent) { e.TimeZone = allocStr("America/Argentina/Buenos_Aires") }},
		{"missing allocationType", func(e *domain.AllocationEvent) { e.AllocationType = 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := allocFirefightingEvent()
			tc.mod(&ev)
			_, err := newAllocationSvc(newFakeAllocationStore()).ProcessAllocationEvent(context.Background(), ev)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestDeliveryModeFromNature(t *testing.T) {
	for in, want := range map[string]string{"Off-site": "OFFSITE", "On-site": "ONSITE", "Onsite": "ONSITE", "Hybrid": ""} {
		got := deliveryModeFromNature(in)
		if (want == "" && got != nil) || (want != "" && (got == nil || *got != want)) {
			t.Errorf("%q -> %v, want %q", in, got, want)
		}
	}
}
