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

package csmclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/apierror"
)

func TestCreateAlertIncidentMapping_Success(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody CreateAlertIncidentMappingRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"map-1","alertNumber":"ALT0000001","source":"azure","uniqueIdentifier":"uid-1","alertStatus":"FIRING","incidentId":"inc-1","incidentNumber":"INC0001","createdAt":"2026-08-27T00:00:00Z"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	uid := "uid-1"
	view, err := client.CreateAlertIncidentMapping(context.Background(), CreateAlertIncidentMappingRequest{
		AlertNumber:      "ALT0000001",
		Source:           "azure",
		UniqueIdentifier: &uid,
		AlertStatus:      "FIRING",
		IncidentID:       "inc-1",
	})
	if err != nil {
		t.Fatalf("CreateAlertIncidentMapping() error = %v, want nil", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/alert-incident-mappings" {
		t.Errorf("request = %s %s, want POST /alert-incident-mappings", gotMethod, gotPath)
	}
	if gotBody.AlertNumber != "ALT0000001" || gotBody.IncidentID != "inc-1" {
		t.Errorf("request body = %+v, missing expected fields", gotBody)
	}
	if view == nil || view.ID != "map-1" || view.IncidentID != "inc-1" {
		t.Errorf("result = %+v, want ID=map-1 IncidentID=inc-1", view)
	}
}

// TestCreateAlertIncidentMapping_409IsTreatedAsAlreadyRecorded verifies a
// 409 (alertNumber already has a recorded mapping) is swallowed into
// (nil, nil) rather than surfaced as an error — see this method's doc
// comment: every call site treats this as "already recorded," never as a
// reason to fail the alert's overall delivery.
func TestCreateAlertIncidentMapping_409IsTreatedAsAlreadyRecorded(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"mapping already exists for this alertNumber"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	view, err := client.CreateAlertIncidentMapping(context.Background(), CreateAlertIncidentMappingRequest{})
	if err != nil {
		t.Fatalf("CreateAlertIncidentMapping() error = %v, want nil (409 treated as already-recorded)", err)
	}
	if view != nil {
		t.Errorf("view = %+v, want nil on a 409", view)
	}
}

func TestCreateAlertIncidentMapping_OtherErrorIsReturned(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Missing or invalid user ID token header."}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	_, err := client.CreateAlertIncidentMapping(context.Background(), CreateAlertIncidentMappingRequest{})
	if err == nil {
		t.Fatal("expected error for a non-409 non-2xx response, got nil")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("error = %v, want *apierror.Error{StatusCode: 401}", err)
	}
}

func TestLookupAlertIncidentMappings_ReturnsMappings(t *testing.T) {
	var gotPath string
	var gotBody LookupAlertIncidentMappingsRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mappings":[{"id":"map-2","alertNumber":"ALT0000002","source":"azure","uniqueIdentifier":"uid-1","alertStatus":"RESOLVED","incidentId":"inc-1","incidentNumber":"INC0001","createdAt":"2026-08-27T01:00:00Z"},{"id":"map-1","alertNumber":"ALT0000001","source":"azure","uniqueIdentifier":"uid-1","alertStatus":"FIRING","incidentId":"inc-1","incidentNumber":"INC0001","createdAt":"2026-08-27T00:00:00Z"}]}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	mappings, err := client.LookupAlertIncidentMappings(context.Background(), "azure", "uid-1")
	if err != nil {
		t.Fatalf("LookupAlertIncidentMappings() error = %v, want nil", err)
	}
	if gotPath != "/alert-incident-mappings/lookup" {
		t.Errorf("path = %q, want /alert-incident-mappings/lookup", gotPath)
	}
	if gotBody.Source != "azure" || gotBody.UniqueIdentifier != "uid-1" {
		t.Errorf("request body = %+v, want Source=azure UniqueIdentifier=uid-1", gotBody)
	}
	if len(mappings) != 2 || mappings[0].ID != "map-2" {
		t.Fatalf("mappings = %+v, want 2 entries with map-2 first (most-recent-first)", mappings)
	}
}

func TestLookupAlertIncidentMappings_EmptyWhenNoneFound(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"mappings":[]}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	mappings, err := client.LookupAlertIncidentMappings(context.Background(), "azure", "uid-none")
	if err != nil {
		t.Fatalf("LookupAlertIncidentMappings() error = %v, want nil", err)
	}
	if len(mappings) != 0 {
		t.Errorf("mappings = %+v, want empty", mappings)
	}
}

func TestSearchOpenIncidentByNumber_SendsNumberAndStateFilter(t *testing.T) {
	var gotReq SearchIncidentsRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"incidents":[{"id":"inc-1","number":"INC0001"}],"total":1,"offset":0,"limit":1}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	result, found, err := client.SearchOpenIncidentByNumber(context.Background(), "INC0001")
	if err != nil {
		t.Fatalf("SearchOpenIncidentByNumber() error = %v, want nil", err)
	}
	if !found || result.IncidentID != "inc-1" {
		t.Fatalf("result = %+v found=%v, want inc-1/true", result, found)
	}
	if gotReq.Filters.Number == nil || *gotReq.Filters.Number != "INC0001" {
		t.Errorf("Filters.Number = %v, want INC0001", gotReq.Filters.Number)
	}
	if len(gotReq.Filters.Filters) != 1 || gotReq.Filters.Filters[0].Field != "state" || gotReq.Filters.Filters[0].Op != "in" {
		t.Fatalf("Filters.Filters = %+v, want a single state/in entry", gotReq.Filters.Filters)
	}
	wantStates := map[string]bool{"NEW": true, "IN_PROGRESS": true, "ON_HOLD": true}
	for _, v := range gotReq.Filters.Filters[0].Values {
		if !wantStates[v] {
			t.Errorf("state filter includes %q, want only NEW/IN_PROGRESS/ON_HOLD", v)
		}
	}
	for want := range wantStates {
		found := false
		for _, v := range gotReq.Filters.Filters[0].Values {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("state filter is missing %q", want)
		}
	}
}

func TestSearchOpenIncidentByNumber_NoMatch(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"incidents":[],"total":0,"offset":0,"limit":1}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := NewClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})

	result, found, err := client.SearchOpenIncidentByNumber(context.Background(), "INC0002")
	if err != nil {
		t.Fatalf("SearchOpenIncidentByNumber() error = %v, want nil", err)
	}
	if found || result != nil {
		t.Errorf("result = %+v found=%v, want nil/false for no match", result, found)
	}
}
