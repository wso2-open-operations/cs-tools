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

func TestSearchServices_Success(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody SearchITServicesRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"services":[{"id":"33333333-3333-3333-3333-333333333333","name":"Azure Monitoring"}],"total":1,"limit":50,"offset":0}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	results, err := client.SearchServices(context.Background(), "Azure Monitoring")
	if err != nil {
		t.Fatalf("SearchServices returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/services/search" {
		t.Errorf("path = %q, want /services/search", gotPath)
	}
	if gotBody.Filters == nil || gotBody.Filters.SearchQuery != "Azure Monitoring" {
		t.Errorf("request filters = %+v, want searchQuery=%q", gotBody.Filters, "Azure Monitoring")
	}
	if gotBody.Pagination.Limit != servicesSearchPageSize {
		t.Errorf("request pagination.limit = %d, want %d", gotBody.Pagination.Limit, servicesSearchPageSize)
	}

	if len(results) != 1 || results[0].ID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("results = %+v, want one service with id 33333333-3333-3333-3333-333333333333", results)
	}
}

// TestSearchServices_SubstringMatchIsNotAnExactMatch pins the fix for a real
// bug: entity-service's own SearchITServices does a case-insensitive
// substring match (name ILIKE '%<query>%'), not an exact match. A result
// whose Name merely contains the label as a substring (but isn't equal to
// it) must never be returned — only a case-insensitively exact Name match
// counts, otherwise a label like "Azure" could resolve to an unrelated
// service like "Azure Backup Services" and get cached against it.
func TestSearchServices_SubstringMatchIsNotAnExactMatch(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"services":[{"id":"44444444-4444-4444-4444-444444444444","name":"Azure Backup Services"}],"total":1,"limit":50,"offset":0}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	results, err := client.SearchServices(context.Background(), "Azure")
	if err != nil {
		t.Fatalf("SearchServices returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v, want empty — the only hit is a substring match, not an exact one", results)
	}
}

// TestSearchServices_ExactMatchOnALaterPage pins that SearchServices walks
// every page rather than trusting the first page's ordering (entity-service
// orders by created_on, not match quality, so an exact match is not
// guaranteed to be on page one).
func TestSearchServices_ExactMatchOnALaterPage(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req SearchITServicesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(http.StatusOK)
		if req.Pagination.Offset == 0 {
			_, _ = w.Write([]byte(`{"services":[{"id":"55555555-5555-5555-5555-555555555555","name":"Azure Backup Services"}],"total":2,"limit":1,"offset":0}`))
			return
		}
		_, _ = w.Write([]byte(`{"services":[{"id":"33333333-3333-3333-3333-333333333333","name":"Azure Monitoring"}],"total":2,"limit":1,"offset":1}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	results, err := client.SearchServices(context.Background(), "Azure Monitoring")
	if err != nil {
		t.Fatalf("SearchServices returned error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("results = %+v, want the exact match found on the second page", results)
	}
	if calls != 2 {
		t.Errorf("upstream calls = %d, want 2 (one per page walked)", calls)
	}
}

// TestSearchServices_ZeroResultIsNotAnError pins the contract
// internal/worker.resolveServiceID depends on: a confirmed zero-result
// search is a normal, error-free outcome (an empty slice), not something
// this method itself turns into an error or a fallback decision — that is
// the caller's job.
func TestSearchServices_ZeroResultIsNotAnError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"services":[],"total":0,"limit":1,"offset":0}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	results, err := client.SearchServices(context.Background(), "unknown-label")
	if err != nil {
		t.Fatalf("SearchServices returned error for a zero-result search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v, want empty", results)
	}
}

func TestSearchServices_UpstreamErrorIsReturned(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer upstream.Close()

	tokenSrv := tokenServer(t)
	client := newClient(Config{BaseURL: upstream.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"}, true)

	_, err := client.SearchServices(context.Background(), "svc")
	if err == nil {
		t.Fatal("SearchServices returned no error for a 401 upstream response")
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *apierror.Error", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}
