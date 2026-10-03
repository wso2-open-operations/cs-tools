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

package escalation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// entity-service decides whether a caller may read the rota from
// x-jwt-assertion's client_id, not from Authorization. A Choreo gateway
// translates one into the other in a real deployment; nothing does locally, so
// without this header every rota lookup is refused and every rung of a real
// ladder resolves to nobody.
//
// Verified against a running entity-service: the same endpoint answers 401
// without the header and 200 with it.
func TestEntityClient_SendsTheClientAssertion(t *testing.T) {
	var gotAssertion, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAssertion = r.Header.Get("x-jwt-assertion")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"assignments":[],"count":0}`))
	}))
	defer srv.Close()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-assertion-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	c := NewEntityClient(EntityConfig{
		BaseURL: srv.URL, TokenURL: tokenSrv.URL,
		ClientID: "csm-notification-service-dev-client", ClientSecret: "s",
	})
	if _, err := c.OnDutyAt(context.Background(), time.Now()); err != nil {
		t.Fatalf("OnDutyAt: %v", err)
	}
	if gotAssertion != "test-assertion-token" {
		t.Errorf("x-jwt-assertion = %q, want the access token", gotAssertion)
	}
	// Authorization still carries it too, which is what the gateway and every
	// other caller expect.
	if gotAuth == "" {
		t.Error("Authorization was dropped; the OAuth2 transport must still set it")
	}
}
