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

package salesentity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// newWriteTestClient starts a stub sales-entity-service with a working token
// endpoint and whatever handlers the test registers.
func newWriteTestClient(t *testing.T, register func(mux *http.ServeMux)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, ClientCredentialsConfig{TokenURL: srv.URL + "/oauth2/token", ClientID: "id", ClientSecret: "secret"})
}

func TestSearchContactByEmail(t *testing.T) {
	cases := []struct {
		name      string
		rows      []map[string]any
		wantFound bool
	}{
		{"a matching contact is found", []map[string]any{{"id": "003xx", "email": "Jane@Acme.com"}}, true},
		// Absence is an ordinary answer here, not an error: it is exactly
		// what tells the caller to create the contact.
		{"no rows means not found, with no error", nil, false},
		// A row that does not carry the address asked for is never adopted:
		// a duplicate contact is recoverable, a membership written for the
		// wrong person is not.
		{"a row for a different address is not a match", []map[string]any{{"id": "003yy", "email": "bob@acme.com"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody contactEmailSearchRequest
			c := newWriteTestClient(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/contacts/search", func(w http.ResponseWriter, r *http.Request) {
					raw, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(raw, &gotBody)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(tc.rows)
				})
			})
			got, found, err := c.SearchContactByEmail(context.Background(), " Jane@Acme.com ")
			if err != nil {
				t.Fatalf("SearchContactByEmail: %v", err)
			}
			if found != tc.wantFound {
				t.Errorf("found = %v, want %v", found, tc.wantFound)
			}
			if gotBody.Email != "jane@acme.com" || gotBody.Limit != 1 {
				t.Errorf("request body = %+v, want the lower-cased address and limit 1", gotBody)
			}
			if tc.wantFound && (got.ID == nil || *got.ID != "003xx") {
				t.Errorf("contact = %+v", got)
			}
		})
	}
}

func TestCreateContactPostsTheContract(t *testing.T) {
	var gotMethod string
	var gotBody CreateContactInput
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "003xx", "email": "jane@acme.com"})
		})
	})
	got, err := c.CreateContact(context.Background(), CreateContactInput{
		FirstName: "Jane", LastName: "Doe", Email: " Jane@Acme.com ", AccountID: "001xx",
	})
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotBody != (CreateContactInput{FirstName: "Jane", LastName: "Doe", Email: "jane@acme.com", AccountID: "001xx"}) {
		t.Errorf("body = %+v", gotBody)
	}
	if got.ID == nil || *got.ID != "003xx" {
		t.Errorf("contact = %+v", got)
	}
}

func TestCreateProjectContactPostsTheContract(t *testing.T) {
	var gotBody CreateProjectContactInput
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a0exx", "email": "jane@acme.com", "state": "INVITED"})
		})
	})
	got, err := c.CreateProjectContact(context.Background(), CreateProjectContactInput{
		ProjectID: "a0pxx", ContactID: "003xx", State: "INVITED", Role: []string{"Portal user", "Admin"},
	})
	if err != nil {
		t.Fatalf("CreateProjectContact: %v", err)
	}
	if gotBody.ProjectID != "a0pxx" || gotBody.ContactID != "003xx" || gotBody.State != "INVITED" ||
		!reflect.DeepEqual(gotBody.Role, []string{"Portal user", "Admin"}) {
		t.Errorf("body = %+v", gotBody)
	}
	if got.ID != "a0exx" {
		t.Errorf("membership = %+v", got)
	}
}

// TestUpdateProjectContactEmptyBodyIsASuccess pins the one surprising part of
// the PATCH contract: it answers 200 with an EMPTY body when its own re-read
// failed, even though the write itself succeeded. That is a zero record and
// no error — treating it as a parse failure would roll back a transaction
// over a write that actually landed.
func TestUpdateProjectContactEmptyBodyIsASuccess(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody updateProjectContactRequest
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/", func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK) // no body at all
		})
	})
	got, err := c.UpdateProjectContactState(context.Background(), "a0exx", "DEACTIVATED")
	if err != nil {
		t.Fatalf("UpdateProjectContactState: %v", err)
	}
	if got.ID != "" {
		t.Errorf("an empty body must decode to a zero record, got %+v", got)
	}
	if gotMethod != http.MethodPatch || gotPath != "/project-contacts/a0exx" {
		t.Errorf("%s %s, want PATCH /project-contacts/a0exx", gotMethod, gotPath)
	}
	if gotBody.State == nil || *gotBody.State != "DEACTIVATED" || gotBody.Role != nil {
		t.Errorf("body = %+v, want only the state", gotBody)
	}
}

func TestUpdateProjectContactRolesSendsOnlyTheRoles(t *testing.T) {
	var gotBody updateProjectContactRequest
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/", func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "a0exx", "role": "Portal user"})
		})
	})
	got, err := c.UpdateProjectContactRoles(context.Background(), "a0exx", []string{"Portal user"})
	if err != nil {
		t.Fatalf("UpdateProjectContactRoles: %v", err)
	}
	if got.ID != "a0exx" {
		t.Errorf("membership = %+v", got)
	}
	if gotBody.State != nil {
		t.Error("a role change must not restate the state")
	}
	if gotBody.Role == nil || !reflect.DeepEqual(*gotBody.Role, []string{"Portal user"}) {
		t.Errorf("roles = %v", gotBody.Role)
	}
}

func TestUpdateContactLockoutPatchesTheFlag(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody updateContactRequest
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts/", func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusNoContent)
		})
	})
	if err := c.UpdateContactLockout(context.Background(), "003xx", true); err != nil {
		t.Fatalf("UpdateContactLockout: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/contacts/003xx" {
		t.Errorf("%s %s, want PATCH /contacts/003xx", gotMethod, gotPath)
	}
	if !gotBody.LockoutStatus {
		t.Error("lockoutStatus must be sent as true")
	}
}

func TestSearchProjectContactFindsTheContactsMembership(t *testing.T) {
	c := newWriteTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/search", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "a0exx", "contact": map[string]any{"id": "003xx"}},
			})
		})
	})
	got, found, err := c.SearchProjectContact(context.Background(), "a0pxx", "003xx")
	if err != nil || !found || got.ID != "a0exx" {
		t.Fatalf("got %+v, %v, %v", got, found, err)
	}
	// A membership belonging to a different contact is not this contact's.
	_, found, err = c.SearchProjectContact(context.Background(), "a0pxx", "003yy")
	if err != nil || found {
		t.Fatalf("found = %v, err = %v; want not found", found, err)
	}
}
