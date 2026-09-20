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
