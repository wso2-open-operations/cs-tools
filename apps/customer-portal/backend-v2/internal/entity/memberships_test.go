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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient serves both the OAuth2 token endpoint and entity-service's
// contact search from one test server.
func newTestClient(t *testing.T, search http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("POST /projects/{id}/contacts/search", search)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
}

// TestListProjectContacts_PagesPastTheLimit: entity-service caps a page at
// 50 rows, so a project with more contacts must be read in several pages
// and returned whole.
func TestListProjectContacts_PagesPastTheLimit(t *testing.T) {
	const total = 87
	var offsets []int
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req searchProjectContactsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if req.Pagination.Limit != projectContactsPageLimit {
			t.Errorf("limit = %d, want %d", req.Pagination.Limit, projectContactsPageLimit)
		}
		offsets = append(offsets, req.Pagination.Offset)
		var out searchProjectContactsResponse
		out.Total = total
		for i := req.Pagination.Offset; i < total && i < req.Pagination.Offset+req.Pagination.Limit; i++ {
			out.Contacts = append(out.Contacts, ProjectContact{Email: fmt.Sprintf("c%d@acme.com", i)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	got, err := c.ListProjectContacts(context.Background(), "11111111-2222-3333-4444-555555555555")

	if err != nil {
		t.Fatalf("ListProjectContacts: %v", err)
	}
	if len(got) != total || got[total-1].Email != "c86@acme.com" {
		t.Errorf("got %d contacts (last %q), want %d ending c86@acme.com", len(got), got[len(got)-1].Email, total)
	}
	if fmt.Sprint(offsets) != "[0 50]" {
		t.Errorf("offsets = %v, want [0 50]", offsets)
	}
}

// TestListProjectContacts_StopsOnAShortPage guards against a total that
// entity-service never reaches: a short page ends the listing.
func TestListProjectContacts_StopsOnAShortPage(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contacts":[{"email":"a@acme.com"}],"total":500}`))
	})

	got, err := c.ListProjectContacts(context.Background(), "11111111-2222-3333-4444-555555555555")

	if err != nil || len(got) != 1 || calls != 1 {
		t.Errorf("got %d contacts in %d calls, err %v; want 1 contact in 1 call", len(got), calls, err)
	}
}
