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
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestSNProjectService_SearchProjects_MapsAccountRef verifies that the account
// reference newly added to ServiceNow's project search response is mapped into
// domain.ProjectView.Account, and that a project with no linked account (blank
// id/name) maps to a nil Account rather than a zero-valued ref.
func TestSNProjectService_SearchProjects_MapsAccountRef(t *testing.T) {
	const accountSysid = "4a6fc0623b16c31091404c6aa5e45a09"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "With account", "key": "WA",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": accountSysid, "name": "Automation Test Customer Account"},
				},
				{
					"id": "22222222222222222222222222222222", "name": "No account", "key": "NA",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 2, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resp.Projects))
	}

	withAccount := resp.Projects[0]
	if withAccount.Account == nil {
		t.Fatalf("expected non-nil Account for project with a linked account")
	}
	if withAccount.Account.ID != sysidToUUID(accountSysid) || withAccount.Account.Name != "Automation Test Customer Account" {
		t.Fatalf("unexpected Account: %+v", withAccount.Account)
	}

	noAccount := resp.Projects[1]
	if noAccount.Account != nil {
		t.Fatalf("expected nil Account for project with no linked account, got %+v", noAccount.Account)
	}
}

// TestSNProjectService_SearchProjects_MapsOnboardingScopedFields verifies that
// the onboarding-scoped dashboard fields added to ServiceNow's project search
// response are mapped into domain.ProjectView: top-level onboardingStatus and
// onboardingOwner, and the account sub-object's region/subRegion/arrToday.
func TestSNProjectService_SearchProjects_MapsOnboardingScopedFields(t *testing.T) {
	const accountSysid = "4a6fc0623b16c31091404c6aa5e45a09"
	const ownerSysid = "5b6fc0623b16c31091404c6aa5e45a10"

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "Onboarding project", "key": "OBP",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{
						"id": accountSysid, "name": "Customer Portal Account",
						"region": nil, "subRegion": "APAC", "arrToday": "0",
					},
					"onboardingStatus": "In-Progress",
					"onboardingOwner":  map[string]any{"id": ownerSysid, "name": "a a", "email": "ff@ww.com"},
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Projects))
	}

	p := resp.Projects[0]
	if p.OnboardingStatus == nil || *p.OnboardingStatus != "In-Progress" {
		t.Errorf("OnboardingStatus = %v, want \"In-Progress\"", p.OnboardingStatus)
	}
	if p.OnboardingOwner == nil {
		t.Fatalf("OnboardingOwner = nil, want non-nil")
	}
	wantOwnerID := sysidToUUID(ownerSysid)
	if p.OnboardingOwner.ID != wantOwnerID || p.OnboardingOwner.Name != "a a" {
		t.Errorf("OnboardingOwner = %+v, want id=%s name=\"a a\"", p.OnboardingOwner, wantOwnerID)
	}
	if p.OnboardingOwner.Email == nil || *p.OnboardingOwner.Email != "ff@ww.com" {
		t.Errorf("OnboardingOwner.Email = %v, want ff@ww.com", p.OnboardingOwner.Email)
	}

	if p.Account == nil {
		t.Fatalf("Account = nil, want non-nil")
	}
	if p.Account.Region != nil {
		t.Errorf("Account.Region = %v, want nil", *p.Account.Region)
	}
	if p.Account.SubRegion == nil || *p.Account.SubRegion != "APAC" {
		t.Errorf("Account.SubRegion = %v, want APAC", p.Account.SubRegion)
	}
	if p.Account.ArrToday == nil || *p.Account.ArrToday != "0" {
		t.Errorf("Account.ArrToday = %v, want \"0\"", p.Account.ArrToday)
	}
}

// TestSNProjectService_SearchProjects_OnboardingScopedFieldsAbsent verifies
// that a project with none of the onboarding-scoped fields tracked decodes
// cleanly to nil OnboardingStatus/OnboardingOwner and nil account
// region/subRegion/arrToday, rather than empty-string/zero-value
// placeholders.
func TestSNProjectService_SearchProjects_OnboardingScopedFieldsAbsent(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "22222222222222222222222222222222", "name": "Plain project", "key": "PLN",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2026-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 1, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := resp.Projects[0]
	if p.OnboardingStatus != nil {
		t.Errorf("OnboardingStatus = %v, want nil", *p.OnboardingStatus)
	}
	if p.OnboardingOwner != nil {
		t.Errorf("OnboardingOwner = %+v, want nil", p.OnboardingOwner)
	}
	if p.Account != nil {
		t.Errorf("Account = %+v, want nil for a project with no linked account", p.Account)
	}
}

// TestSNProjectService_SearchProjects_WiresOnboardingScopedFilters verifies
// that OnboardingStatus/ArrTodayGte/SubRegion on the request are translated
// into the corresponding keys on the Choreo request body's filters object,
// matching digiops-cs's field names exactly.
func TestSNProjectService_SearchProjects_WiresOnboardingScopedFilters(t *testing.T) {
	var gotBody map[string]any
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{}, "totalRecords": 0, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	_, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination:       domain.Pagination{Limit: 10},
		OnboardingStatus: []string{"In-Progress", "Not-Started"},
		ArrTodayGte:      "1000",
		SubRegion:        "APAC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filters, ok := gotBody["filters"].(map[string]any)
	if !ok {
		t.Fatalf("request body missing filters object: %+v", gotBody)
	}
	gotStatuses, ok := filters["onboardingStatus"].([]any)
	if !ok || len(gotStatuses) != 2 || gotStatuses[0] != "In-Progress" || gotStatuses[1] != "Not-Started" {
		t.Errorf("filters.onboardingStatus = %v, want [In-Progress Not-Started]", filters["onboardingStatus"])
	}
	if filters["arrTodayGte"] != "1000" {
		t.Errorf("filters.arrTodayGte = %v, want 1000", filters["arrTodayGte"])
	}
	if filters["subRegion"] != "APAC" {
		t.Errorf("filters.subRegion = %v, want APAC", filters["subRegion"])
	}
}

// TestSNProjectService_SearchProjects_MapsStartDate verifies that the date-only
// startDate from ServiceNow's project search response is parsed into
// domain.ProjectView.StartDate, and that a null or absent startDate maps to a
// nil pointer rather than a zero time (which would serialize as year 0001).
func TestSNProjectService_SearchProjects_MapsStartDate(t *testing.T) {
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{
				{
					"id": "11111111111111111111111111111111", "name": "With start date", "key": "WSD",
					"type":      map[string]any{"name": "Subscription"},
					"startDate": "2026-03-15", "endDate": "2027-03-14",
					"createdOn": "2024-01-01 00:00:00",
					"account":   map[string]any{"id": "", "name": ""},
				},
				{
					"id": "22222222222222222222222222222222", "name": "Null start date", "key": "NSD",
					"type":      map[string]any{"name": "Subscription"},
					"startDate": nil, "endDate": "",
					"createdOn": "2024-01-01 00:00:00",
					"account":   map[string]any{"id": "", "name": ""},
				},
				{
					// startDate key omitted entirely.
					"id": "33333333333333333333333333333333", "name": "Absent start date", "key": "ASD",
					"type":    map[string]any{"name": "Subscription"},
					"endDate": "", "createdOn": "2024-01-01 00:00:00",
					"account": map[string]any{"id": "", "name": ""},
				},
			},
			"totalRecords": 3, "offset": 0, "limit": 10,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	resp, err := svc.SearchProjects(contextWithUserIDToken("token"), domain.SearchProjectsRequest{
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(resp.Projects))
	}

	withStart := resp.Projects[0]
	if withStart.StartDate == nil {
		t.Fatalf("expected non-nil StartDate for project with a start date")
	}
	want := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if !withStart.StartDate.Equal(want) {
		t.Fatalf("unexpected StartDate: got %v, want %v", *withStart.StartDate, want)
	}
	// StartDate must stay distinct from EndDate and CreatedOn.
	if withStart.EndDate == nil || !withStart.EndDate.Equal(time.Date(2027, 3, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected EndDate: %v", withStart.EndDate)
	}
	if !withStart.CreatedOn.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected CreatedOn: %v", withStart.CreatedOn)
	}

	if got := resp.Projects[1].StartDate; got != nil {
		t.Fatalf("expected nil StartDate for null startDate, got %v", *got)
	}
	if got := resp.Projects[2].StartDate; got != nil {
		t.Fatalf("expected nil StartDate for absent startDate, got %v", *got)
	}
}

// The contact id is optional upstream: absent on an instance that predates the field, and
// null for a row with no linked contact record. Neither case may produce a bogus id — the
// caller uses emptiness to decide whether the row is clickable.
func TestSNProjectContactService_SearchProjectContacts_OptionalContactID(t *testing.T) {
	projectUUID := sysidToUUID(sysid32('7'))
	contactSysid := sysid32('8')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contacts":[
			{"id":"` + contactSysid + `","name":"Linked","email":"linked@example.com",
			 "registrationState":"REGISTERED","notificationsEnabled":true,"roles":["r"]},
			{"id":null,"name":"Orphaned","email":"orphan@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[]},
			{"name":"OldInstance","email":"old@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[]}
		],"totalRecords":3,"offset":0,"limit":10}`))
	}))

	svc := NewServiceNowProjectContactService(client)

	got, err := svc.SearchProjectContacts(contextWithUserIDToken("token"), projectUUID,
		domain.SearchProjectContactsRequest{Pagination: domain.Pagination{Limit: 10}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Contacts) != 3 {
		t.Fatalf("got %d contacts, want 3", len(got.Contacts))
	}

	want := sysidToUUID(contactSysid)
	if got.Contacts[0].ID == nil || *got.Contacts[0].ID != want {
		t.Errorf("linked contact ID = %v, want %q", got.Contacts[0].ID, want)
	}
	if got.Contacts[1].ID != nil {
		t.Errorf("null upstream id produced %q, want nil", *got.Contacts[1].ID)
	}
	if got.Contacts[2].ID != nil {
		t.Errorf("absent upstream id produced %q, want nil", *got.Contacts[2].ID)
	}

	// A nil id must be omitted from the wire payload, not emitted as null or "": the
	// published contract documents the field as absent when no contact record is linked.
	encoded, err := json.Marshal(got.Contacts[1])
	if err != nil {
		t.Fatalf("marshal contact: %v", err)
	}
	if strings.Contains(string(encoded), `"id"`) {
		t.Errorf("unlinked contact serialized as %s, want no id key", encoded)
	}
}

// TestSNProjectContactService_SearchProjectContacts_MapsAccessStatus verifies that the
// access-status fields ServiceNow computes per contact row (customerContactPresent,
// grantsCaseAccess) flow through into domain.ProjectContact unchanged, covering both a
// linked and an orphaned row. grantsCaseAccess mirrors customerContactPresent directly —
// there is no separate email-match check, since that only ever diverges for
// integration/system accounts, not real customers.
func TestSNProjectContactService_SearchProjectContacts_MapsAccessStatus(t *testing.T) {
	projectUUID := sysidToUUID(sysid32('7'))

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contacts":[
			{"id":"` + sysid32('8') + `","name":"Granted","email":"granted@example.com",
			 "registrationState":"REGISTERED","notificationsEnabled":true,"roles":["r"],
			 "customerContactPresent":true,"grantsCaseAccess":true},
			{"id":null,"name":"Orphaned","email":"orphaned@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[],
			 "customerContactPresent":false,"grantsCaseAccess":false}
		],"totalRecords":2,"offset":0,"limit":10}`))
	}))

	svc := NewServiceNowProjectContactService(client)

	got, err := svc.SearchProjectContacts(contextWithUserIDToken("token"), projectUUID,
		domain.SearchProjectContactsRequest{Pagination: domain.Pagination{Limit: 10}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Contacts) != 2 {
		t.Fatalf("got %d contacts, want 2", len(got.Contacts))
	}

	granted := got.Contacts[0]
	if !granted.CustomerContactPresent || !granted.GrantsCaseAccess {
		t.Errorf("granted row = %+v, want both access-status fields true", granted)
	}

	orphaned := got.Contacts[1]
	if orphaned.CustomerContactPresent || orphaned.GrantsCaseAccess {
		t.Errorf("orphaned row = %+v, want both access-status fields false", orphaned)
	}
}

// TestSNProjectContactService_GetProjectContact_ScanLimitIsAccepted pins the scan window to
// a value SearchProjectContacts will accept. A scan limit above maxLimit made every lookup
// fail with a pagination validation error before the upstream call was ever made.
func TestSNProjectContactService_GetProjectContact_ScanLimitIsAccepted(t *testing.T) {
	if projectContactScanLimit > maxLimit {
		t.Fatalf("projectContactScanLimit = %d exceeds maxLimit %d; every lookup would 400",
			projectContactScanLimit, maxLimit)
	}

	projectUUID := sysidToUUID(sysid32('7'))
	contactSysid := sysid32('8')
	contactUUID := sysidToUUID(contactSysid)

	var gotLimit int
	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Pagination struct {
				Limit int `json:"limit"`
			} `json:"pagination"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotLimit = payload.Pagination.Limit
		_, _ = w.Write([]byte(`{"contacts":[
			{"id":"` + contactSysid + `","name":"Linked","email":"linked@example.com",
			 "registrationState":"REGISTERED","notificationsEnabled":true,"roles":["r"]}
		],"totalRecords":1,"offset":0,"limit":` + strconv.Itoa(projectContactScanLimit) + `}`))
	}))

	svc := NewServiceNowProjectContactService(client)

	got, err := svc.GetProjectContact(contextWithUserIDToken("token"), projectUUID, contactUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == nil || *got.ID != contactUUID {
		t.Errorf("GetProjectContact returned ID %v, want %q", got.ID, contactUUID)
	}
	if gotLimit != projectContactScanLimit {
		t.Errorf("upstream saw limit %d, want %d", gotLimit, projectContactScanLimit)
	}
}

// TestSNProjectContactService_GetProjectContact_UnlinkedRowsDoNotMatch checks that a row
// with no linked contact record never matches: nil is "no id", not "any id".
func TestSNProjectContactService_GetProjectContact_UnlinkedRowsDoNotMatch(t *testing.T) {
	projectUUID := sysidToUUID(sysid32('7'))

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"contacts":[
			{"id":null,"name":"Orphaned","email":"orphan@example.com",
			 "registrationState":"INVITED","notificationsEnabled":false,"roles":[]}
		],"totalRecords":1,"offset":0,"limit":10}`))
	}))

	svc := NewServiceNowProjectContactService(client)

	_, err := svc.GetProjectContact(contextWithUserIDToken("token"), projectUUID, sysidToUUID(sysid32('8')))
	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("GetProjectContact error = %v, want NotFoundError", err)
	}
}

// TestSNProjectService_GetProjectByID_MapsHasSr verifies that ServiceNow's own
// precomputed "hasSr" (service-request eligibility) field is parsed from the
// project-detail response and passed through into domain.ProjectDetailsView
// unmodified.
func TestSNProjectService_GetProjectByID_MapsHasSr(t *testing.T) {
	projectSysid := sysid32('9')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "SR Eligible", "key": "SRE", "sfId": "sf-1",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type":    map[string]any{"name": "Subscription"},
			"account": map[string]any{"id": "", "name": ""},
			"hasSr":   true,
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HasSr {
		t.Errorf("GetProjectByID HasSr = false, want true (passthrough of SN's hasSr)")
	}
}

// TestSNProjectService_GetProjectByID_MapsOnboardingFields verifies that the
// detail-only onboardingStatus and onboardingOwner fields are parsed from the
// project-detail response and mapped into domain.ProjectDetailsView, with the
// owner's sys_id converted to the service's UUID form.
func TestSNProjectService_GetProjectByID_MapsOnboardingFields(t *testing.T) {
	projectSysid := sysid32('a')
	ownerSysid := sysid32('b')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "Onboarding Project", "key": "ONB", "sfId": "sf-2",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type":             map[string]any{"name": "Subscription"},
			"account":          map[string]any{"id": "", "name": ""},
			"onboardingStatus": "In-Progress",
			"onboardingOwner":  map[string]any{"id": ownerSysid, "name": "Jane Doe", "email": "jane.doe@example.com"},
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OnboardingStatus == nil || *got.OnboardingStatus != "In-Progress" {
		t.Errorf("GetProjectByID OnboardingStatus = %v, want \"In-Progress\"", got.OnboardingStatus)
	}
	wantOwnerID := sysidToUUID(ownerSysid)
	if got.OnboardingOwner == nil {
		t.Fatalf("GetProjectByID OnboardingOwner = nil, want non-nil")
	}
	if got.OnboardingOwner.ID != wantOwnerID || got.OnboardingOwner.Name != "Jane Doe" {
		t.Errorf("GetProjectByID OnboardingOwner = %+v, want id=%s name=Jane Doe", got.OnboardingOwner, wantOwnerID)
	}
	if got.OnboardingOwner.Email == nil || *got.OnboardingOwner.Email != "jane.doe@example.com" {
		t.Errorf("GetProjectByID OnboardingOwner.Email = %v, want jane.doe@example.com", got.OnboardingOwner.Email)
	}
}

// TestSNProjectService_GetProjectByID_OnboardingFieldsAbsent verifies that
// projects with no onboarding engagement at all (the common case) map to nil
// OnboardingStatus and nil OnboardingOwner, rather than empty-string/zero-value
// placeholders.
func TestSNProjectService_GetProjectByID_OnboardingFieldsAbsent(t *testing.T) {
	projectSysid := sysid32('c')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "No Onboarding", "key": "NOB", "sfId": "sf-3",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type":    map[string]any{"name": "Subscription"},
			"account": map[string]any{"id": "", "name": ""},
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OnboardingStatus != nil {
		t.Errorf("GetProjectByID OnboardingStatus = %v, want nil", *got.OnboardingStatus)
	}
	if got.OnboardingOwner != nil {
		t.Errorf("GetProjectByID OnboardingOwner = %+v, want nil", got.OnboardingOwner)
	}
}

// TestSNProjectService_GetProjectByID_MapsStartEndDate verifies that populated
// startDate/endDate values are parsed into non-nil domain.ProjectDetailsView
// dates, so the empty-string handling added below doesn't regress the normal
// case.
func TestSNProjectService_GetProjectByID_MapsStartEndDate(t *testing.T) {
	projectSysid := sysid32('d')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "Dated Project", "key": "DTD", "sfId": "sf-4",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type":    map[string]any{"name": "Subscription"},
			"account": map[string]any{"id": "", "name": ""},
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StartDate == nil || !got.StartDate.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("GetProjectByID StartDate = %v, want 2026-01-01", got.StartDate)
	}
	if got.EndDate == nil || !got.EndDate.Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("GetProjectByID EndDate = %v, want 2026-12-31", got.EndDate)
	}
}

// TestSNProjectService_GetProjectByID_EmptyStartEndDate is the regression guard
// for the bug where ServiceNow legitimately returning "" for a project's
// startDate/endDate (field genuinely not set on that record) crashed
// GetProjectByID with a time.Parse error instead of mapping to nil — a project
// read must return normally with the date fields absent, not 500.
func TestSNProjectService_GetProjectByID_EmptyStartEndDate(t *testing.T) {
	projectSysid := sysid32('e')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "Undated Project", "key": "UND", "sfId": "sf-5",
			"createdOn": "2026-01-01 00:00:00", "startDate": "", "endDate": "",
			"type":    map[string]any{"name": "Subscription"},
			"account": map[string]any{"id": "", "name": ""},
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("GetProjectByID returned error for empty startDate/endDate (should map to nil, not error): %v", err)
	}
	if got.StartDate != nil {
		t.Errorf("GetProjectByID StartDate = %v, want nil for empty upstream startDate", *got.StartDate)
	}
	if got.EndDate != nil {
		t.Errorf("GetProjectByID EndDate = %v, want nil for empty upstream endDate", *got.EndDate)
	}
}

// TestSNProjectService_GetProjectByID_EmptyOptionalDates is an audit-driven
// regression guard for every other optional date on the project-detail
// response — goLiveDate, goLivePlanDate, onboardingExpiryDate, and the
// account's activationDate/deactivationDate. All five already routed through
// optionalSNProjectDate before the startDate/endDate fix, so this pins that
// they actually behave: empty string maps to nil, not a parse error or a
// zero-value timestamp.
func TestSNProjectService_GetProjectByID_EmptyOptionalDates(t *testing.T) {
	projectSysid := sysid32('f')

	client := newTestSNClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": projectSysid, "name": "No Optional Dates", "key": "NOD", "sfId": "sf-6",
			"createdOn": "2026-01-01 00:00:00", "startDate": "2026-01-01", "endDate": "2026-12-31",
			"type": map[string]any{"name": "Subscription"},
			"account": map[string]any{
				"id": "", "name": "", "activationDate": "", "deactivationDate": "",
			},
			"goLiveDate":           "",
			"goLivePlanDate":       "",
			"onboardingExpiryDate": "",
		})
	}))

	svc := NewServiceNowProjectService(client, nil)
	got, err := svc.GetProjectByID(contextWithUserIDToken("token"), sysidToUUID(projectSysid))
	if err != nil {
		t.Fatalf("GetProjectByID returned error for empty optional dates (should map to nil, not error): %v", err)
	}
	if got.GoLiveDate != nil {
		t.Errorf("GetProjectByID GoLiveDate = %v, want nil for empty upstream goLiveDate", *got.GoLiveDate)
	}
	if got.GoLivePlanDate != nil {
		t.Errorf("GetProjectByID GoLivePlanDate = %v, want nil for empty upstream goLivePlanDate", *got.GoLivePlanDate)
	}
	if got.OnboardingExpiryDate != nil {
		t.Errorf("GetProjectByID OnboardingExpiryDate = %v, want nil for empty upstream onboardingExpiryDate", *got.OnboardingExpiryDate)
	}
	if got.Account.ActivationDate != nil {
		t.Errorf("GetProjectByID Account.ActivationDate = %v, want nil for empty upstream activationDate", *got.Account.ActivationDate)
	}
	if got.Account.DeactivationDate != nil {
		t.Errorf("GetProjectByID Account.DeactivationDate = %v, want nil for empty upstream deactivationDate", *got.Account.DeactivationDate)
	}
}
