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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIncidentCreateDefaultsSendsGet(t *testing.T) {
	var gotMethod, gotPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/incidents/create-defaults", func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"defaultServiceId":null,"defaultGroup":null}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "test-client", ClientSecret: "test-secret",
	})
	raw, err := client.GetIncidentCreateDefaults(context.Background())
	if err != nil {
		t.Fatalf("GetIncidentCreateDefaults: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/incidents/create-defaults" {
		t.Errorf("sent %s %s, want GET /incidents/create-defaults", gotMethod, gotPath)
	}
	if string(raw) != `{"defaultServiceId":null,"defaultGroup":null}` {
		t.Errorf("body = %s", raw)
	}
}
