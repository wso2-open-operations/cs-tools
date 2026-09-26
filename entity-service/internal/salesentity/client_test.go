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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

func TestGetCustomer_PostsRealtimeSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	mux.HandleFunc("/customer-search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req customerSearchRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatalf("parse body: %v", err)
		}
		if len(req.IDs) != 1 || req.IDs[0] != "001xx" || !req.IsRealTime || req.Limit != 1 {
			t.Errorf("request = %+v, want ids=[001xx] isRealTime=true limit=1", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "001xx", "name": "Acme", "technicalOwner": "owner@example.com"},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := New(srv.URL, ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "id",
		ClientSecret: "secret",
	})
	got, err := client.GetCustomer(context.Background(), "001xx")
	if err != nil {
		t.Fatalf("GetCustomer: %v", err)
	}
	if got.ID != "001xx" || deref(got.Name) != "Acme" || deref(got.TechnicalOwner) != "owner@example.com" {
		t.Errorf("customer = %+v", got)
	}
}

func TestGetCustomer_EmptyArrayIs503(t *testing.T) {
	client := newTestClient(t, http.StatusOK, []any{})
	_, err := client.GetCustomer(context.Background(), "001xx")
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

func TestGetCustomer_ServerErrorIs503(t *testing.T) {
	client := newTestClient(t, http.StatusInternalServerError, map[string]string{"message": "boom"})
	_, err := client.GetCustomer(context.Background(), "001xx")
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

func TestGetCustomer_MismatchedIDIs503(t *testing.T) {
	client := newTestClient(t, http.StatusOK, []map[string]any{{"id": "001other", "name": "Other"}})
	_, err := client.GetCustomer(context.Background(), "001xx")
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

func TestGetCustomer_EmptyIDIs503(t *testing.T) {
	client := newTestClient(t, http.StatusOK, []map[string]any{{"name": "Acme"}})
	_, err := client.GetCustomer(context.Background(), "001xx")
	var sue *apierror.ServiceUnavailableError
	if !asSvcUnavailable(err, &sue) {
		t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
	}
}

func TestGetCustomer_Accepts15And18CharSalesforceIDs(t *testing.T) {
	want15 := "001xx000000ABC1"
	want18 := want15 + "AAA"
	client := newTestClient(t, http.StatusOK, []map[string]any{{"id": want18, "name": "Acme"}})
	got, err := client.GetCustomer(context.Background(), want15)
	if err != nil {
		t.Fatalf("GetCustomer: %v", err)
	}
	if got.ID != want18 {
		t.Errorf("id = %q, want %q", got.ID, want18)
	}
}

func newTestClient(t *testing.T, searchStatus int, searchBody any) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	mux.HandleFunc("/customer-search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(searchStatus)
		_ = json.NewEncoder(w).Encode(searchBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth2/token",
		ClientID:     "id",
		ClientSecret: "secret",
	})
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func asSvcUnavailable(err error, target **apierror.ServiceUnavailableError) bool {
	if sue, ok := err.(*apierror.ServiceUnavailableError); ok {
		*target = sue
		return true
	}
	return false
}

func newMembershipTestClient(t *testing.T, handle func(mux *http.ServeMux)) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	handle(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, ClientCredentialsConfig{TokenURL: srv.URL + "/oauth2/token", ClientID: "id", ClientSecret: "secret"})
}

func TestGetProjectContact_PostsIDSearch(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/search", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("method=%s auth=%q", r.Method, r.Header.Get("Authorization"))
			}
			var req idSearchRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("parse body: %v", err)
			}
			if req.ID != "a0e000000000001AAA" || req.Limit != 1 {
				t.Errorf("request = %+v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "a0e000000000001", "email": "jane@acme.com", "state": "INVITED", "roles": []string{"Portal user", "Admin"},
				"type":             "OWN CONTACT",
				"contact":          map[string]any{"id": "003xx", "name": "Jane Doe", "email": "jane@acme.com", "customerId": "001xx"},
				"subscription":     map[string]any{"id": "a0pxx", "name": "Acme Prod", "key": "ACMEPROD", "customerId": "001xx"},
				"lastModifiedDate": "2026-09-18T06:37:07.000+0000",
			}})
		})
	})
	got, err := client.GetProjectContact(context.Background(), "a0e000000000001AAA")
	if err != nil {
		t.Fatalf("GetProjectContact: %v", err)
	}
	if got.ID != "a0e000000000001" || deref(got.State) != "INVITED" || len(got.Roles) != 2 || got.Contact == nil || deref(got.Contact.ID) != "003xx" ||
		got.Subscription == nil || deref(got.Subscription.Key) != "ACMEPROD" || deref(got.LastModifiedDate) != "2026-09-18T06:37:07.000+0000" {
		t.Errorf("project contact = %+v", got)
	}
}

func TestGetProjectContact_EmptyOrMismatchedIs503(t *testing.T) {
	for name, rows := range map[string]any{
		"empty":      []any{},
		"mismatched": []map[string]any{{"id": "a0e000000000009"}},
	} {
		t.Run(name, func(t *testing.T) {
			client := newMembershipTestClient(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/project-contacts/search", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(rows)
				})
			})
			_, err := client.GetProjectContact(context.Background(), "a0e000000000001")
			var sue *apierror.ServiceUnavailableError
			if !errors.As(err, &sue) {
				t.Fatalf("err = %v, want ServiceUnavailableError", err)
			}
		})
	}
}

func TestGetContact_PostsIDSearchAndRefreshesTokenOn401(t *testing.T) {
	calls := 0
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts/search", func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var req idSearchRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.ID != "003xx" {
				t.Errorf("request = %+v", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "003xx", "email": "jane@acme.com", "firstName": "Jane", "lastName": "Doe", "isCsAdmin": true, "isCsIntegrationUser": false,
				"account":     map[string]any{"id": "001xx", "isPartner": false},
				"memberships": []map[string]any{{"id": "a0e1", "subscriptionId": "a0p1", "state": "INVITED"}},
			}})
		})
	})
	got, err := client.GetContact(context.Background(), "003xx")
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want one retry after 401", calls)
	}
	if deref(got.ID) != "003xx" || got.IsCsAdmin == nil || !*got.IsCsAdmin || len(got.Memberships) != 1 || deref(got.Memberships[0].ID) != "a0e1" {
		t.Errorf("contact = %+v", got)
	}
}

func TestGetContact_ServerErrorIs503(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts/search", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	})
	_, err := client.GetContact(context.Background(), "003xx")
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
}

func TestUpdateContactLockout_PatchesLockoutStatusOnly(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("auth = %q", r.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("parse body: %v", err)
			}
			// PATCH /contacts/{id} answers 200 with no body at all.
			w.WriteHeader(http.StatusOK)
		})
	})
	if err := client.UpdateContactLockout(context.Background(), "003000000000001AAA", false); err != nil {
		t.Fatalf("UpdateContactLockout: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/contacts/003000000000001AAA" {
		t.Errorf("request = %s %s, want PATCH /contacts/003000000000001AAA", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["lockoutStatus"] != false {
		t.Errorf("body = %v, want exactly {lockoutStatus: false}", gotBody)
	}
}

func TestUpdateProjectContactState_ReturnsTheReReadRecord(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/{id}", func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("parse body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "a0e000000000001AAA", "email": "jane@acme.com", "state": "REGISTERED",
				"lastModifiedDate": "2026-09-18T06:37:07.000+0000",
			})
		})
	})
	got, err := client.UpdateProjectContactState(context.Background(), "a0e000000000001AAA", "REGISTERED")
	if err != nil {
		t.Fatalf("UpdateProjectContactState: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/project-contacts/a0e000000000001AAA" {
		t.Errorf("request = %s %s, want PATCH /project-contacts/a0e000000000001AAA", gotMethod, gotPath)
	}
	if len(gotBody) != 1 || gotBody["state"] != "REGISTERED" {
		t.Errorf("body = %v, want exactly {state: REGISTERED}", gotBody)
	}
	if got.ID != "a0e000000000001AAA" || deref(got.State) != "REGISTERED" || deref(got.LastModifiedDate) != "2026-09-18T06:37:07.000+0000" {
		t.Errorf("project contact = %+v", got)
	}
}

// sales-entity-service writes the record, then re-reads it best-effort: a
// failed re-read there is still a 200, with an empty body.
func TestUpdateProjectContactState_EmptyBodyIsAZeroRecordNotAnError(t *testing.T) {
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/project-contacts/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	})
	got, err := client.UpdateProjectContactState(context.Background(), "a0e000000000001AAA", "REGISTERED")
	if err != nil {
		t.Fatalf("UpdateProjectContactState: %v", err)
	}
	if got.ID != "" || got.State != nil {
		t.Errorf("project contact = %+v, want the zero record", got)
	}
}

func TestUpdateContactLockout_RefreshesTokenOn401(t *testing.T) {
	calls := 0
	client := newMembershipTestClient(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/contacts/{id}", func(w http.ResponseWriter, _ *http.Request) {
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
	})
	if err := client.UpdateContactLockout(context.Background(), "003xx000000ABC1", false); err != nil {
		t.Fatalf("UpdateContactLockout: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want one retry after 401", calls)
	}
}

// A write against a record this service has already ingested is a real 404,
// not the "Salesforce has not committed it yet, retry" the read path means.
func TestPatchStatusMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		check  func(error) bool
	}{
		{"404 is not found", http.StatusNotFound, func(err error) bool {
			var nfe *apierror.NotFoundError
			return errors.As(err, &nfe)
		}},
		{"400 is a downstream rejection", http.StatusBadRequest, func(err error) bool {
			var de *apierror.DownstreamError
			return errors.As(err, &de)
		}},
		{"502 is unavailable", http.StatusBadGateway, func(err error) bool {
			var sue *apierror.ServiceUnavailableError
			return errors.As(err, &sue)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newMembershipTestClient(t, func(mux *http.ServeMux) {
				mux.HandleFunc("/project-contacts/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })
			})
			_, err := client.UpdateProjectContactState(context.Background(), "a0e000000000001AAA", "REGISTERED")
			if err == nil || !tc.check(err) {
				t.Fatalf("err = %v (%T), want the mapping for %d", err, err, tc.status)
			}
		})
	}
}
